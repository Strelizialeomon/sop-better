package task

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type ServerClock func(context.Context) (time.Time, time.Duration, error)

type GitClaimStore struct {
	RepositoryPath string
	Remote         string
	GitBinary      string
	Clock          ServerClock
}

func (store *GitClaimStore) ServerNow(ctx context.Context) (time.Time, time.Duration, error) {
	if store.Clock == nil {
		return time.Time{}, 0, errors.New("GitHub server clock is required")
	}
	return store.Clock(ctx)
}

func (store *GitClaimStore) Read(ctx context.Context, repoNodeID string, issueNumber int) (StoredClaim, bool, error) {
	ref, err := claimRef(issueNumber)
	if err != nil {
		return StoredClaim{}, false, err
	}
	output, err := store.git(ctx, nil, "ls-remote", "--refs", store.remote(), ref)
	if err != nil {
		return StoredClaim{}, false, fmt.Errorf("read remote claim ref: %w", err)
	}
	fields := strings.Fields(output)
	if len(fields) == 0 {
		return StoredClaim{}, false, nil
	}
	if len(fields) != 2 || fields[1] != ref {
		return StoredClaim{}, false, errors.New("remote claim ref response is malformed")
	}
	oid := fields[0]
	if _, err := store.git(ctx, nil, "cat-file", "-e", oid+"^{commit}"); err != nil {
		if _, fetchErr := store.git(ctx, nil, "fetch", "--quiet", store.remote(), oid); fetchErr != nil {
			return StoredClaim{}, false, fmt.Errorf("fetch remote claim metadata: %w", fetchErr)
		}
	}
	data, err := store.git(ctx, nil, "show", oid+":claim.json")
	if err != nil {
		return StoredClaim{}, false, fmt.Errorf("read remote claim metadata: %w", err)
	}
	var claim Claim
	if err := json.Unmarshal([]byte(data), &claim); err != nil {
		return StoredClaim{}, false, fmt.Errorf("decode remote claim metadata: %w", err)
	}
	if claim.RepoNodeID != repoNodeID || claim.IssueNumber != issueNumber {
		return StoredClaim{}, false, errors.New("remote claim metadata does not match repository and issue")
	}
	return StoredClaim{OID: oid, Claim: claim}, true, nil
}

func (store *GitClaimStore) Create(ctx context.Context, claim Claim) (StoredClaim, error) {
	ref, err := claimRef(claim.IssueNumber)
	if err != nil {
		return StoredClaim{}, err
	}
	oid, err := store.writeClaimCommit(ctx, claim, "")
	if err != nil {
		return StoredClaim{}, err
	}
	if _, err := store.git(ctx, nil, "push", "--porcelain", store.remote(), oid+":"+ref); err != nil {
		if isGitRefConflict(err) {
			return StoredClaim{}, ErrRefConflict
		}
		return StoredClaim{}, fmt.Errorf("create remote claim ref: %w", err)
	}
	return StoredClaim{OID: oid, Claim: claim}, nil
}

func (store *GitClaimStore) CompareAndSwap(ctx context.Context, expectedOID string, claim Claim) (StoredClaim, error) {
	if strings.TrimSpace(expectedOID) == "" {
		return StoredClaim{}, errors.New("expected claim OID is required")
	}
	ref, err := claimRef(claim.IssueNumber)
	if err != nil {
		return StoredClaim{}, err
	}
	oid, err := store.writeClaimCommit(ctx, claim, expectedOID)
	if err != nil {
		return StoredClaim{}, err
	}
	lease := "--force-with-lease=" + ref + ":" + expectedOID
	if _, err := store.git(ctx, nil, "push", "--porcelain", lease, store.remote(), oid+":"+ref); err != nil {
		if isGitRefConflict(err) {
			return StoredClaim{}, ErrRefConflict
		}
		return StoredClaim{}, fmt.Errorf("update remote claim ref: %w", err)
	}
	return StoredClaim{OID: oid, Claim: claim}, nil
}

func (store *GitClaimStore) Delete(ctx context.Context, _ string, issueNumber int, expectedOID string) error {
	if strings.TrimSpace(expectedOID) == "" {
		return errors.New("expected claim OID is required")
	}
	ref, err := claimRef(issueNumber)
	if err != nil {
		return err
	}
	lease := "--force-with-lease=" + ref + ":" + expectedOID
	if _, err := store.git(ctx, nil, "push", "--porcelain", lease, store.remote(), ":"+ref); err != nil {
		if isGitRefConflict(err) {
			return ErrRefConflict
		}
		return fmt.Errorf("delete remote claim ref: %w", err)
	}
	return nil
}

func (store *GitClaimStore) writeClaimCommit(ctx context.Context, claim Claim, parent string) (string, error) {
	data, err := json.Marshal(claim)
	if err != nil {
		return "", fmt.Errorf("encode claim metadata: %w", err)
	}
	data = append(data, '\n')
	blob, err := store.git(ctx, data, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", fmt.Errorf("write claim blob: %w", err)
	}
	treeInput := []byte("100644 blob " + strings.TrimSpace(blob) + "\tclaim.json\n")
	tree, err := store.git(ctx, treeInput, "mktree")
	if err != nil {
		return "", fmt.Errorf("write claim tree: %w", err)
	}
	args := []string{"commit-tree", strings.TrimSpace(tree)}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	args = append(args, "-m", "sop claim #"+strconv.Itoa(claim.IssueNumber))
	commit, err := store.git(ctx, nil, args...)
	if err != nil {
		return "", fmt.Errorf("write claim commit: %w", err)
	}
	return strings.TrimSpace(commit), nil
}

func (store *GitClaimStore) remote() string {
	if store.Remote == "" {
		return "origin"
	}
	return store.Remote
}

func claimRef(issueNumber int) (string, error) {
	if issueNumber <= 0 {
		return "", errors.New("claim issue number must be positive")
	}
	return "refs/heads/sop/claims/" + strconv.Itoa(issueNumber), nil
}

func (store *GitClaimStore) git(ctx context.Context, stdin []byte, args ...string) (string, error) {
	if strings.TrimSpace(store.RepositoryPath) == "" {
		return "", errors.New("claim repository path is required")
	}
	binary := store.GitBinary
	if binary == "" {
		binary = "git"
	}
	command := exec.CommandContext(ctx, binary, args...)
	command.Dir = store.RepositoryPath
	command.Stdin = bytes.NewReader(stdin)
	command.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=sopctl", "GIT_AUTHOR_EMAIL=sopctl@localhost",
		"GIT_COMMITTER_NAME=sopctl", "GIT_COMMITTER_EMAIL=sopctl@localhost",
	)
	output, err := command.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return string(output), nil
}

func isGitRefConflict(err error) bool {
	message := strings.ToLower(err.Error())
	return strings.Contains(message, "non-fast-forward") || strings.Contains(message, "fetch first") ||
		strings.Contains(message, "stale info") || strings.Contains(message, "reference already exists") ||
		strings.Contains(message, "would clobber existing tag")
}
