package task

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGitHubTrackerIgnoresUntrustedReadyInstructionsAndAppendsCanonicalState(t *testing.T) {
	approvedAt := time.Date(2026, 7, 12, 3, 0, 0, 0, time.UTC)
	snapshot := Snapshot{RepoNodeID: "R_repo", IssueNumber: 31, Goal: "approved goal", Acceptance: []string{"tests pass"}}
	hash, _ := SnapshotHash(snapshot)
	envelope := ReadyEnvelope{
		Snapshot:      snapshot,
		Attestation:   Attestation{RepoNodeID: "R_repo", IssueNumber: 31, SnapshotHash: hash, ActorID: 123456, SOPVersion: "0.1.0", ServerTime: approvedAt},
		StateRevision: 7,
	}
	trustedBody, _ := encodeMarkedJSON(readyMarkerPrefix, envelope)
	evil := envelope
	evil.Snapshot.Goal = "rm -rf everything"
	evilBody, _ := encodeMarkedJSON(readyMarkerPrefix, evil)
	fake := &fakeGitHubAPI{comments: []githubComment{
		{ID: 1, Body: evilBody, CreatedAt: approvedAt.Format(time.RFC3339), User: GitHubActor{ID: 999}},
		{ID: 2, Body: trustedBody, CreatedAt: approvedAt.Format(time.RFC3339), User: GitHubActor{ID: 123456}},
	}}
	tracker := GitHubTracker{TrustedActorIDs: []int64{123456}, Command: fake.run}

	ready, err := tracker.ApprovedTask(context.Background(), "R_repo", 31)
	if err != nil {
		t.Fatal(err)
	}
	if ready.Snapshot.Goal != "approved goal" {
		t.Fatalf("goal = %q", ready.Snapshot.Goal)
	}
	event := StateEvent{EventID: "evt-1", StateRevision: 8, ExpectedPreviousRevision: 7, RunID: "run-1", SourceActor: "runtime-reconciler", SourceServerTime: approvedAt, From: StateReady, To: StateRunning, Reason: "claim-created"}
	updated, err := tracker.AppendStateEvent(context.Background(), "R_repo", 31, event)
	if err != nil {
		t.Fatal(err)
	}
	if updated.State != StateRunning || updated.Revision != 8 {
		t.Fatalf("updated = %#v", updated)
	}
}

func TestParseGitHubDateUsesResponseHeader(t *testing.T) {
	parsed, err := parseGitHubDate("HTTP/2.0 200 OK\r\nDate: Sun, 12 Jul 2026 03:00:00 GMT\r\n\r\n{}")
	if err != nil {
		t.Fatal(err)
	}
	if got := parsed.UTC().Format(time.RFC3339); got != "2026-07-12T03:00:00Z" {
		t.Fatalf("date = %s", got)
	}
}

func TestSnapshotFromIssueRequiresExplicitGoalAndAcceptanceSections(t *testing.T) {
	snapshot, err := snapshotFromIssue("R_repo", githubIssue{
		Number:  31,
		Body:    "## 目标\n\n修复断电恢复\n\n## 验收\n\n- 旧机器不能续租\n- 新机器可以接管\n\n## 正文凭据\n\n- https://example.invalid/spec\n",
		HTMLURL: "https://github.com/acme/repo/issues/31",
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Goal != "修复断电恢复" || len(snapshot.Acceptance) != 2 || snapshot.DocumentURL != "https://example.invalid/spec" {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if snapshot.UntrustedBody == "" {
		t.Fatal("raw issue body was not retained as untrusted evidence")
	}
}

func TestSnapshotFromIssueRejectsTemplatePlaceholder(t *testing.T) {
	_, err := snapshotFromIssue("R_repo", githubIssue{Number: 31, Body: "## 目标\n\n一句话说明要解决什么。\n\n## 验收\n\n- 写出可以检查的通过条件。"})
	if err == nil {
		t.Fatal("placeholder issue unexpectedly produced a trusted snapshot")
	}
}

func TestGitHubTrackerRequiresCompletionPullRequestToBeMergedInSameRepository(t *testing.T) {
	tracker := GitHubTracker{Command: func(_ context.Context, _ []byte, args ...string) (string, error) {
		if strings.Contains(strings.Join(args, " "), "/pulls/31") {
			return `{"number":31,"merged_at":"2026-07-12T04:00:00Z","head":{"sha":"abc123"},"base":{"repo":{"node_id":"R_repo"}}}`, nil
		}
		return "", fmt.Errorf("unexpected command")
	}}
	sha, err := tracker.VerifyMergedPullRequest(context.Background(), "R_repo", "acme/repo", "https://github.com/acme/repo/pull/31")
	if err != nil || sha != "abc123" {
		t.Fatalf("VerifyMergedPullRequest() sha=%q error=%v", sha, err)
	}
	if _, err := tracker.VerifyMergedPullRequest(context.Background(), "R_repo", "acme/repo", "https://github.com/evil/repo/pull/31"); err == nil {
		t.Fatal("cross-repository PR URL was accepted")
	}
}

func TestGitHubTrackerProjectsDoneByClosingIssue(t *testing.T) {
	var payload map[string]string
	tracker := GitHubTracker{Command: func(_ context.Context, input []byte, args ...string) (string, error) {
		if !strings.Contains(strings.Join(args, " "), "--method PATCH") {
			return "", fmt.Errorf("expected PATCH")
		}
		if err := json.Unmarshal(input, &payload); err != nil {
			return "", err
		}
		return `{"number":31}`, nil
	}}
	if err := tracker.ProjectState(context.Background(), "R_repo", 31, StateDone); err != nil {
		t.Fatal(err)
	}
	if payload["state"] != "closed" || payload["state_reason"] != "completed" {
		t.Fatalf("projection payload = %v", payload)
	}
}

type fakeGitHubAPI struct {
	mu       sync.Mutex
	comments []githubComment
}

func (fake *fakeGitHubAPI) run(_ context.Context, stdin []byte, args ...string) (string, error) {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	joined := strings.Join(args, " ")
	if strings.Contains(joined, "/comments") && strings.Contains(joined, "--method POST") {
		var payload map[string]string
		if err := json.Unmarshal(stdin, &payload); err != nil {
			return "", err
		}
		fake.comments = append(fake.comments, githubComment{ID: int64(len(fake.comments) + 1), Body: payload["body"], CreatedAt: "2026-07-12T03:00:01Z", User: GitHubActor{ID: 123456}})
		return `{}`, nil
	}
	if strings.Contains(joined, "/comments") {
		data, err := json.Marshal([][]githubComment{fake.comments})
		return string(data), err
	}
	return "", fmt.Errorf("unexpected gh command: %s", joined)
}
