package releasemanager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"

	"github.com/Strelizialeomon/sop-better/internal/platform"
	"github.com/Strelizialeomon/sop-better/internal/releasebundle"
	"github.com/Strelizialeomon/sop-better/internal/state"
)

const journalFormat = 1

var journalCommitPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var pluginIdentifierPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

type PluginRef struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Marketplace string `json:"marketplace"`
}

func (plugin PluginRef) Selector() string {
	return plugin.Name + "@" + plugin.Marketplace
}

type PluginController interface {
	EnsureActive(context.Context, PluginRef) error
	EnsureAbsent(context.Context, PluginRef) error
}

type PluginHealthController interface {
	CheckActive(context.Context, PluginRef) error
}

type Confirmer interface {
	Confirm(context.Context, string) (bool, error)
}

type Journal struct {
	Format     int           `json:"format"`
	Operation  string        `json:"operation"`
	Phase      string        `json:"phase"`
	Before     state.Current `json:"before"`
	After      state.Current `json:"after"`
	From       PluginRef     `json:"from_plugin"`
	To         PluginRef     `json:"to_plugin"`
	FromCommit string        `json:"from_commit"`
	ToCommit   string        `json:"to_commit"`
}

func journalPath(stateHome string) string {
	return filepath.Join(stateHome, "transactions", "release.json")
}

func writeJournal(stateHome string, journal Journal) error {
	journal.Format = journalFormat
	if err := validateJournal(journal); err != nil {
		return err
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return platform.AtomicWrite(journalPath(stateHome), data, 0o600)
}

func readJournal(stateHome string) (Journal, bool, error) {
	file, err := os.Open(journalPath(stateHome))
	if errors.Is(err, os.ErrNotExist) {
		return Journal{}, false, nil
	}
	if err != nil {
		return Journal{}, false, err
	}
	defer file.Close()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var journal Journal
	if err := decoder.Decode(&journal); err != nil {
		return Journal{}, false, fmt.Errorf("parse release journal: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			err = errors.New("multiple JSON values are not allowed")
		}
		return Journal{}, false, fmt.Errorf("parse release journal: %w", err)
	}
	if err := validateJournal(journal); err != nil {
		return Journal{}, false, err
	}
	return journal, true, nil
}

func validateJournal(journal Journal) error {
	if journal.Format != journalFormat {
		return errors.New("release journal format is unsupported")
	}
	if journal.Operation != "upgrade" && journal.Operation != "rollback" {
		return errors.New("release journal operation is invalid")
	}
	switch journal.Phase {
	case "prepared", "target_plugin_ready", "old_plugin_removed", "current_committed":
	default:
		return errors.New("release journal phase is invalid")
	}
	if err := journal.Before.Validate(); err != nil {
		return fmt.Errorf("release journal before state: %w", err)
	}
	if err := journal.After.Validate(); err != nil {
		return fmt.Errorf("release journal after state: %w", err)
	}
	if journal.After.Previous != journal.Before.Version || journal.After.Version == journal.Before.Version {
		return errors.New("release journal current transition is inconsistent")
	}
	if err := validatePluginRef(journal.From, journal.Before.Version); err != nil {
		return fmt.Errorf("release journal previous plugin: %w", err)
	}
	if err := validatePluginRef(journal.To, journal.After.Version); err != nil {
		return fmt.Errorf("release journal target plugin: %w", err)
	}
	if !journalCommitPattern.MatchString(journal.FromCommit) || !journalCommitPattern.MatchString(journal.ToCommit) {
		return errors.New("release journal commit pin is invalid")
	}
	return nil
}

func validatePluginRef(plugin PluginRef, expectedVersion string) error {
	if plugin.Name != "sop-better" || plugin.Version != expectedVersion {
		return errors.New("name/version is inconsistent")
	}
	if !pluginIdentifierPattern.MatchString(plugin.Marketplace) {
		return errors.New("marketplace is invalid")
	}
	return nil
}

func clearJournal(stateHome string) error {
	err := os.Remove(journalPath(stateHome))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func (manager Manager) Recover(ctx context.Context) error {
	journal, exists, err := readJournal(manager.StateHome)
	if err != nil || !exists {
		return err
	}
	if manager.Plugin == nil {
		return errors.New("plugin controller is required for release recovery")
	}
	if err := manager.verifyJournalPins(journal); err != nil {
		return err
	}
	current, err := state.ReadCurrent(manager.StateHome)
	if err != nil {
		return err
	}
	switch current {
	case journal.Before:
		if err := manager.Plugin.EnsureActive(ctx, journal.From); err != nil {
			return fmt.Errorf("restore previous plugin: %w", err)
		}
		if err := manager.Plugin.EnsureAbsent(ctx, journal.To); err != nil {
			return fmt.Errorf("remove target plugin during recovery: %w", err)
		}
	case journal.After:
		if err := manager.Plugin.EnsureActive(ctx, journal.To); err != nil {
			return fmt.Errorf("restore target plugin: %w", err)
		}
		if err := manager.Plugin.EnsureAbsent(ctx, journal.From); err != nil {
			return fmt.Errorf("remove previous plugin during recovery: %w", err)
		}
	default:
		return errors.New("current.json conflicts with the pending release journal")
	}
	return clearJournal(manager.StateHome)
}

func (manager Manager) verifyJournalPins(journal Journal) error {
	for label, pin := range map[string]struct {
		version string
		commit  string
		plugin  PluginRef
	}{
		"previous": {version: journal.Before.Version, commit: journal.FromCommit, plugin: journal.From},
		"target":   {version: journal.After.Version, commit: journal.ToCommit, plugin: journal.To},
	} {
		manifest, err := releasebundle.Inspect(filepath.Join(manager.StateHome, "versions", pin.version))
		if err != nil {
			return fmt.Errorf("verify %s release pin: %w", label, err)
		}
		if manifest.GitCommit != pin.commit || pluginRefFromManifest(manifest) != pin.plugin {
			return fmt.Errorf("verify %s release pin: installed metadata differs from the transaction journal", label)
		}
	}
	return nil
}
