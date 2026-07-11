package codexplugin

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/Strelizialeomon/sop-better/internal/releasemanager"
)

func TestEnsureActiveRunsCodexContractAndVerifiesInstalledPlugin(t *testing.T) {
	stateHome := t.TempDir()
	plugin := releasemanager.PluginRef{
		Name:        "sop-better",
		Version:     "1.2.3",
		Marketplace: "sop-better-stable-v1-2-3",
	}
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	runner := &scriptedRunner{outputs: []string{
		`{"marketplaces":[]}`,
		fmt.Sprintf(`{"marketplaceName":%q,"installedRoot":%q,"alreadyAdded":false}`, plugin.Marketplace, marketplaceRoot),
		fmt.Sprintf(`{"pluginId":%q,"name":%q,"marketplaceName":%q,"version":%q,"installedPath":"/tmp/plugin","authPolicy":"ON_INSTALL"}`, plugin.Selector(), plugin.Name, plugin.Marketplace, plugin.Version),
		installedPayload(plugin, marketplaceRoot),
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	if err := controller.EnsureActive(context.Background(), plugin); err != nil {
		t.Fatalf("EnsureActive() error = %v", err)
	}

	want := [][]string{
		{"plugin", "marketplace", "list", "--json"},
		{"plugin", "marketplace", "add", marketplaceRoot, "--json"},
		{"plugin", "add", plugin.Selector(), "--json"},
		{"plugin", "list", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestCheckActiveIsReadOnlyAndRequiresTheExactOwnedPlugin(t *testing.T) {
	plugin := testPlugin()
	stateHome := t.TempDir()
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	runner := &scriptedRunner{outputs: []string{
		marketplacePayload(plugin.Marketplace, marketplaceRoot),
		installedPayload(plugin, marketplaceRoot),
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	if err := controller.CheckActive(context.Background(), plugin); err != nil {
		t.Fatalf("CheckActive() error = %v", err)
	}
	want := [][]string{
		{"plugin", "marketplace", "list", "--json"},
		{"plugin", "list", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("CheckActive() calls = %#v, want read-only calls %#v", runner.calls, want)
	}

	wrongRoot := filepath.Join(t.TempDir(), "wrong")
	runner = &scriptedRunner{outputs: []string{marketplacePayload(plugin.Marketplace, wrongRoot)}}
	controller.Runner = runner
	if err := controller.CheckActive(context.Background(), plugin); err == nil || !strings.Contains(err.Error(), "different root") {
		t.Fatalf("CheckActive() wrong-root error = %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("wrong-root health check issued a mutation: %#v", runner.calls)
	}
}

func TestCheckActiveReportsReadFailuresAndInactivePlugin(t *testing.T) {
	plugin := testPlugin()
	for _, test := range []struct {
		name    string
		outputs []string
		failAt  int
		want    string
	}{
		{name: "marketplace list failure", failAt: 1, want: "marketplace health list"},
		{name: "plugin list failure", outputs: []string{"marketplace"}, failAt: 2, want: "plugin health list"},
		{name: "disabled plugin", want: "enabled"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateHome := t.TempDir()
			root := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
			outputs := test.outputs
			if test.name == "plugin list failure" {
				outputs[0] = marketplacePayload(plugin.Marketplace, root)
			}
			if test.name == "marketplace list failure" {
				outputs = []string{""}
			}
			if test.name == "disabled plugin" {
				outputs = []string{
					marketplacePayload(plugin.Marketplace, root),
					installedPayloadWith(plugin, root, plugin.Version, true, false, plugin.Version),
				}
			}
			runner := &scriptedRunner{outputs: outputs, failAt: test.failAt}
			err := (Controller{StateHome: stateHome, Runner: runner}).CheckActive(context.Background(), plugin)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("CheckActive() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestEnsureActiveSkipsAlreadyConfiguredMarketplace(t *testing.T) {
	stateHome := t.TempDir()
	plugin := testPlugin()
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	runner := &scriptedRunner{outputs: []string{
		marketplacePayload(plugin.Marketplace, marketplaceRoot),
		pluginAddPayload(plugin),
		installedPayload(plugin, marketplaceRoot),
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	if err := controller.EnsureActive(context.Background(), plugin); err != nil {
		t.Fatalf("EnsureActive() error = %v", err)
	}

	want := [][]string{
		{"plugin", "marketplace", "list", "--json"},
		{"plugin", "add", plugin.Selector(), "--json"},
		{"plugin", "list", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnsureActiveAllowsCodexBuiltinEntriesWithoutMarketplaceSource(t *testing.T) {
	stateHome := t.TempDir()
	plugin := testPlugin()
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	target := strings.TrimSuffix(installedPayload(plugin, marketplaceRoot), `],"available":[]}`)
	withBuiltin := target + `,{"pluginId":"superpowers@openai-curated","name":"superpowers","marketplaceName":"openai-curated","version":"2f1a8948","installed":true,"enabled":true,"source":{"source":"local","path":"/tmp/superpowers"},"installPolicy":"AVAILABLE","authPolicy":"ON_INSTALL"}],"available":[]}`
	runner := &scriptedRunner{outputs: []string{
		fmt.Sprintf(`{"marketplaces":[{"name":"openai-curated","root":"/tmp/curated"},{"name":%q,"root":%q,"marketplaceSource":{"sourceType":"local","source":%q}}]}`, plugin.Marketplace, marketplaceRoot, marketplaceRoot),
		pluginAddPayload(plugin),
		withBuiltin,
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	if err := controller.EnsureActive(context.Background(), plugin); err != nil {
		t.Fatalf("EnsureActive() error = %v", err)
	}
}

func TestEnsureActiveReportsEveryCodexCommandFailure(t *testing.T) {
	plugin := testPlugin()
	for _, test := range []struct {
		name   string
		failAt int
	}{
		{name: "marketplace list", failAt: 1},
		{name: "marketplace add", failAt: 2},
		{name: "plugin add", failAt: 3},
		{name: "plugin verification list", failAt: 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateHome := t.TempDir()
			root := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
			runner := &scriptedRunner{
				outputs: []string{
					`{"marketplaces":[]}`,
					fmt.Sprintf(`{"marketplaceName":%q,"installedRoot":%q,"alreadyAdded":false}`, plugin.Marketplace, root),
					pluginAddPayload(plugin),
					installedPayload(plugin, root),
				},
				failAt: test.failAt,
			}
			controller := Controller{StateHome: stateHome, Runner: runner}

			err := controller.EnsureActive(context.Background(), plugin)
			if err == nil || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), test.name) {
				t.Fatalf("EnsureActive() error = %v, want codex %q failure", err, test.name)
			}
		})
	}
}

func TestEnsureActiveRejectsWrongMarketplaceRootBeforeWriting(t *testing.T) {
	stateHome := t.TempDir()
	plugin := testPlugin()
	runner := &scriptedRunner{outputs: []string{
		marketplacePayload(plugin.Marketplace, filepath.Join(t.TempDir(), "other")),
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	err := controller.EnsureActive(context.Background(), plugin)
	if err == nil || !strings.Contains(err.Error(), "different root") {
		t.Fatalf("EnsureActive() error = %v, want different-root rejection", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("calls = %#v, want only marketplace list", runner.calls)
	}
}

func TestEnsureActiveRequiresExactInstalledPlugin(t *testing.T) {
	plugin := testPlugin()
	for _, test := range []struct {
		name    string
		payload func(string) string
		want    string
	}{
		{name: "missing", payload: func(string) string { return `{"installed":[],"available":[]}` }, want: "not installed"},
		{name: "wrong version", payload: func(root string) string {
			return installedPayloadWith(plugin, root, plugin.Version, true, true, "9.9.9")
		}, want: "version"},
		{name: "disabled", payload: func(root string) string {
			return installedPayloadWith(plugin, root, plugin.Version, true, false, plugin.Version)
		}, want: "enabled"},
		{name: "not installed", payload: func(root string) string {
			return installedPayloadWith(plugin, root, plugin.Version, false, false, plugin.Version)
		}, want: "installed"},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateHome := t.TempDir()
			root := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
			runner := &scriptedRunner{outputs: []string{
				marketplacePayload(plugin.Marketplace, root),
				pluginAddPayload(plugin),
				test.payload(root),
			}}
			controller := Controller{StateHome: stateHome, Runner: runner}

			err := controller.EnsureActive(context.Background(), plugin)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("EnsureActive() error = %v, want substring %q", err, test.want)
			}
		})
	}
}

func TestEnsureActiveRejectsAPluginLoadedFromAnotherRoot(t *testing.T) {
	stateHome := t.TempDir()
	plugin := testPlugin()
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	wrongRoot := filepath.Join(t.TempDir(), "other-marketplace")
	runner := &scriptedRunner{outputs: []string{
		marketplacePayload(plugin.Marketplace, marketplaceRoot),
		pluginAddPayload(plugin),
		installedPayload(plugin, wrongRoot),
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	err := controller.EnsureActive(context.Background(), plugin)
	if err == nil || !strings.Contains(err.Error(), "verified release root") {
		t.Fatalf("EnsureActive() error = %v, want wrong plugin source rejection", err)
	}
}

func TestEnsureActiveRequiresExactNameAndMarketplaceFields(t *testing.T) {
	plugin := testPlugin()
	yes := true
	for _, test := range []struct {
		name           string
		reportedName   string
		reportedMarket string
		want           string
	}{
		{
			name:           "name",
			reportedName:   "not-sop-better",
			reportedMarket: plugin.Marketplace,
			want:           "name",
		},
		{
			name:           "marketplace",
			reportedName:   plugin.Name,
			reportedMarket: "not-the-marketplace",
			want:           "marketplace",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := verifyActivePlugin(listedPlugin{
				PluginID:        plugin.Selector(),
				Name:            test.reportedName,
				MarketplaceName: test.reportedMarket,
				Version:         plugin.Version,
				Installed:       &yes,
				Enabled:         &yes,
			}, plugin, t.TempDir())
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("verifyActivePlugin() error = %v, want exact %s rejection", err, test.want)
			}
		})
	}
}

func TestEnsureActiveRejectsMalformedOrTrailingJSON(t *testing.T) {
	plugin := testPlugin()
	for _, payload := range []string{
		`{"marketplaces":[]} trailing`,
		`{"marketplaces":[],"surprise":true}`,
		`{"marketplaces":null}`,
	} {
		t.Run(payload, func(t *testing.T) {
			runner := &scriptedRunner{outputs: []string{payload}}
			controller := Controller{StateHome: t.TempDir(), Runner: runner}
			if err := controller.EnsureActive(context.Background(), plugin); err == nil {
				t.Fatal("EnsureActive() error = nil, want strict JSON rejection")
			}
		})
	}
}

func TestEnsureAbsentRemovesInstalledPluginAndVerifiesAbsence(t *testing.T) {
	plugin := testPlugin()
	stateHome := t.TempDir()
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	runner := &scriptedRunner{outputs: []string{
		installedPayload(plugin, marketplaceRoot),
		fmt.Sprintf(`{"pluginId":%q,"name":%q,"marketplaceName":%q}`, plugin.Selector(), plugin.Name, plugin.Marketplace),
		`{"installed":[],"available":[]}`,
		marketplacePayload(plugin.Marketplace, marketplaceRoot),
		fmt.Sprintf(`{"marketplaceName":%q,"installedRoot":null}`, plugin.Marketplace),
		`{"marketplaces":[]}`,
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	if err := controller.EnsureAbsent(context.Background(), plugin); err != nil {
		t.Fatalf("EnsureAbsent() error = %v", err)
	}

	want := [][]string{
		{"plugin", "list", "--json"},
		{"plugin", "remove", plugin.Selector(), "--json"},
		{"plugin", "list", "--json"},
		{"plugin", "marketplace", "list", "--json"},
		{"plugin", "marketplace", "remove", plugin.Marketplace, "--json"},
		{"plugin", "marketplace", "list", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnsureAbsentRefusesToRemovePluginFromAnotherSourceRoot(t *testing.T) {
	plugin := testPlugin()
	stateHome := t.TempDir()
	wrongRoot := filepath.Join(t.TempDir(), "not-the-verified-release")
	runner := &scriptedRunner{outputs: []string{installedPayload(plugin, wrongRoot)}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	err := controller.EnsureAbsent(context.Background(), plugin)
	if err == nil || !strings.Contains(err.Error(), "verified release root") {
		t.Fatalf("EnsureAbsent() error = %v, want wrong-source refusal", err)
	}
	want := [][]string{{"plugin", "list", "--json"}}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("wrong-source cleanup issued a mutation: calls=%#v", runner.calls)
	}
}

func TestEnsureAbsentIsIdempotentWhenPluginIsMissing(t *testing.T) {
	runner := &scriptedRunner{outputs: []string{`{"installed":[],"available":[]}`, `{"marketplaces":[]}`}}
	controller := Controller{StateHome: t.TempDir(), Runner: runner}

	if err := controller.EnsureAbsent(context.Background(), testPlugin()); err != nil {
		t.Fatalf("EnsureAbsent() error = %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("calls = %#v, want plugin and marketplace verification lists", runner.calls)
	}
}

func TestEnsureAbsentRemovesStaleOwnedMarketplaceWhenPluginIsMissing(t *testing.T) {
	plugin := testPlugin()
	stateHome := t.TempDir()
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	runner := &scriptedRunner{outputs: []string{
		`{"installed":[],"available":[]}`,
		marketplacePayload(plugin.Marketplace, marketplaceRoot),
		fmt.Sprintf(`{"marketplaceName":%q,"installedRoot":null}`, plugin.Marketplace),
		`{"marketplaces":[]}`,
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	if err := controller.EnsureAbsent(context.Background(), plugin); err != nil {
		t.Fatalf("EnsureAbsent() stale marketplace cleanup: %v", err)
	}
	want := [][]string{
		{"plugin", "list", "--json"},
		{"plugin", "marketplace", "list", "--json"},
		{"plugin", "marketplace", "remove", plugin.Marketplace, "--json"},
		{"plugin", "marketplace", "list", "--json"},
	}
	if !reflect.DeepEqual(runner.calls, want) {
		t.Fatalf("stale marketplace cleanup calls = %#v, want %#v", runner.calls, want)
	}
}

func TestEnsureAbsentRefusesToRemoveMarketplaceFromAnotherRoot(t *testing.T) {
	plugin := testPlugin()
	stateHome := t.TempDir()
	wrongRoot := filepath.Join(t.TempDir(), "wrong-marketplace")
	runner := &scriptedRunner{outputs: []string{
		`{"installed":[],"available":[]}`,
		marketplacePayload(plugin.Marketplace, wrongRoot),
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	err := controller.EnsureAbsent(context.Background(), plugin)
	if err == nil || !strings.Contains(err.Error(), "different root") {
		t.Fatalf("EnsureAbsent() wrong marketplace root error = %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("wrong-root marketplace cleanup issued a mutation: %#v", runner.calls)
	}
}

func TestEnsureAbsentReportsEveryCodexCommandFailure(t *testing.T) {
	plugin := testPlugin()
	for _, test := range []struct {
		name   string
		failAt int
	}{
		{name: "plugin list", failAt: 1},
		{name: "plugin remove", failAt: 2},
		{name: "plugin verification list", failAt: 3},
		{name: "marketplace cleanup list", failAt: 4},
		{name: "marketplace remove", failAt: 5},
		{name: "marketplace cleanup verification list", failAt: 6},
	} {
		t.Run(test.name, func(t *testing.T) {
			stateHome := t.TempDir()
			marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
			runner := &scriptedRunner{
				outputs: []string{
					installedPayload(plugin, marketplaceRoot),
					fmt.Sprintf(`{"pluginId":%q,"name":%q,"marketplaceName":%q}`, plugin.Selector(), plugin.Name, plugin.Marketplace),
					`{"installed":[],"available":[]}`,
					marketplacePayload(plugin.Marketplace, marketplaceRoot),
					fmt.Sprintf(`{"marketplaceName":%q,"installedRoot":null}`, plugin.Marketplace),
					`{"marketplaces":[]}`,
				},
				failAt: test.failAt,
			}
			controller := Controller{StateHome: stateHome, Runner: runner}

			err := controller.EnsureAbsent(context.Background(), plugin)
			if err == nil || !strings.Contains(err.Error(), "codex") || !strings.Contains(err.Error(), test.name) {
				t.Fatalf("EnsureAbsent() error = %v, want codex %q failure", err, test.name)
			}
		})
	}
}

func TestEnsureAbsentFailsWhenPluginRemainsInstalled(t *testing.T) {
	plugin := testPlugin()
	stateHome := t.TempDir()
	marketplaceRoot := filepath.Join(stateHome, "versions", plugin.Version, "marketplace")
	runner := &scriptedRunner{outputs: []string{
		installedPayload(plugin, marketplaceRoot),
		fmt.Sprintf(`{"pluginId":%q,"name":%q,"marketplaceName":%q}`, plugin.Selector(), plugin.Name, plugin.Marketplace),
		installedPayload(plugin, marketplaceRoot),
	}}
	controller := Controller{StateHome: stateHome, Runner: runner}

	err := controller.EnsureAbsent(context.Background(), plugin)
	if err == nil || !strings.Contains(err.Error(), "still installed") {
		t.Fatalf("EnsureAbsent() error = %v, want final absence failure", err)
	}
}

func TestCommandRunnerEnvOverridesValueAndInheritsParentEnvironment(t *testing.T) {
	if os.Getenv("GO_WANT_CODEXPLUGIN_HELPER") == "1" {
		fmt.Printf("%s|%t", os.Getenv("CODEX_HOME"), os.Getenv("PATH") != "")
		os.Exit(0)
	}
	wantHome := t.TempDir()
	runner := CommandRunner{
		Binary: os.Args[0],
		Env: []string{
			"GO_WANT_CODEXPLUGIN_HELPER=1",
			"CODEX_HOME=" + wantHome,
		},
	}

	output, err := runner.Run(context.Background(), "-test.run=TestCommandRunnerEnvOverridesValueAndInheritsParentEnvironment")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if got, want := string(output), wantHome+"|true"; got != want {
		t.Fatalf("Run() output = %q, want %q", got, want)
	}
}

func TestSamePathUsesWindowsCaseInsensitiveComparison(t *testing.T) {
	left := filepath.Join(t.TempDir(), "Marketplace")
	if !samePathForOS("windows", left, strings.ToUpper(left)) {
		t.Fatal("samePathForOS(windows) = false for paths differing only by case")
	}
}

func TestSamePathRecognizesCaseAliasOnCaseInsensitiveFilesystem(t *testing.T) {
	root := filepath.Join(t.TempDir(), "Marketplace")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(filepath.Dir(root), "MARKETPLACE")
	if _, err := os.Stat(alias); err != nil {
		t.Skip("test filesystem is case-sensitive")
	}
	if !samePath(root, alias) {
		t.Fatalf("samePath(%q, %q) = false for the same directory", root, alias)
	}
}

type scriptedRunner struct {
	outputs []string
	failAt  int
	calls   [][]string
}

func (runner *scriptedRunner) Run(_ context.Context, args ...string) ([]byte, error) {
	runner.calls = append(runner.calls, append([]string(nil), args...))
	call := len(runner.calls)
	if call == runner.failAt {
		return nil, errors.New("injected command failure")
	}
	if call > len(runner.outputs) {
		return nil, fmt.Errorf("unexpected call %d: %v", call, args)
	}
	return []byte(runner.outputs[call-1]), nil
}

func testPlugin() releasemanager.PluginRef {
	return releasemanager.PluginRef{
		Name:        "sop-better",
		Version:     "2.0.0",
		Marketplace: "sop-better-stable-v2-0-0",
	}
}

func marketplacePayload(name, root string) string {
	return fmt.Sprintf(`{"marketplaces":[{"name":%q,"root":%q,"marketplaceSource":{"sourceType":"local","source":%q}}]}`, name, root, root)
}

func pluginAddPayload(plugin releasemanager.PluginRef) string {
	return fmt.Sprintf(`{"pluginId":%q,"name":%q,"marketplaceName":%q,"version":%q,"installedPath":"/tmp/plugin","authPolicy":"ON_INSTALL"}`, plugin.Selector(), plugin.Name, plugin.Marketplace, plugin.Version)
}

func installedPayload(plugin releasemanager.PluginRef, marketplaceRoots ...string) string {
	marketplaceRoot := "/tmp/marketplace"
	if len(marketplaceRoots) > 0 {
		marketplaceRoot = marketplaceRoots[0]
	}
	return installedPayloadWith(plugin, marketplaceRoot, plugin.Version, true, true, plugin.Version)
}

func installedPayloadWith(plugin releasemanager.PluginRef, marketplaceRoot, sourceVersion string, installed, enabled bool, reportedVersion string) string {
	pluginRoot := filepath.Join(marketplaceRoot, "plugins", plugin.Name)
	return fmt.Sprintf(`{"installed":[{"pluginId":%q,"name":%q,"marketplaceName":%q,"version":%q,"installed":%t,"enabled":%t,"source":{"source":"local","path":%q},"marketplaceSource":{"sourceType":"local","source":%q},"installPolicy":"AVAILABLE","authPolicy":"ON_INSTALL"}],"available":[]}`,
		plugin.Selector(), plugin.Name, plugin.Marketplace, reportedVersion, installed, enabled, pluginRoot, marketplaceRoot)
}
