package codexplugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"github.com/Strelizialeomon/sop-better/internal/releasemanager"
	"github.com/Strelizialeomon/sop-better/internal/state"
)

var (
	pluginNamePattern      = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)
	marketplaceNamePattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
)

type Controller struct {
	StateHome string
	Runner    Runner
}

var _ releasemanager.PluginController = Controller{}
var _ releasemanager.PluginHealthController = Controller{}

func (controller Controller) CheckActive(ctx context.Context, plugin releasemanager.PluginRef) error {
	if err := controller.validate(plugin); err != nil {
		return err
	}
	marketplaceRoot, err := filepath.Abs(filepath.Join(controller.StateHome, "versions", plugin.Version, "marketplace"))
	if err != nil {
		return fmt.Errorf("resolve verified release marketplace root: %w", err)
	}
	marketplaces, err := controller.listMarketplaces(ctx, "marketplace health list")
	if err != nil {
		return err
	}
	foundMarketplace := 0
	for _, candidate := range marketplaces {
		if candidate.Name != plugin.Marketplace {
			continue
		}
		foundMarketplace++
		if candidate.MarketplaceSource == nil || candidate.MarketplaceSource.SourceType != "local" ||
			!samePath(candidate.Root, marketplaceRoot) || !samePath(candidate.MarketplaceSource.Source, marketplaceRoot) {
			return fmt.Errorf("codex marketplace %q is configured at a different root", plugin.Marketplace)
		}
	}
	if foundMarketplace == 0 {
		return fmt.Errorf("codex marketplace %q is not configured", plugin.Marketplace)
	}
	if foundMarketplace > 1 {
		return fmt.Errorf("codex marketplace %q is configured more than once", plugin.Marketplace)
	}
	plugins, err := controller.listPlugins(ctx, "plugin health list")
	if err != nil {
		return err
	}
	candidate, count := findInstalled(plugins.Installed, plugin)
	if count == 0 {
		return fmt.Errorf("codex plugin %q is not installed", plugin.Selector())
	}
	if count > 1 {
		return fmt.Errorf("codex plugin %q appears more than once", plugin.Selector())
	}
	return verifyActivePlugin(candidate, plugin, marketplaceRoot)
}

func (controller Controller) EnsureActive(ctx context.Context, plugin releasemanager.PluginRef) error {
	if err := controller.validate(plugin); err != nil {
		return err
	}
	marketplaceRoot, err := filepath.Abs(filepath.Join(controller.StateHome, "versions", plugin.Version, "marketplace"))
	if err != nil {
		return fmt.Errorf("resolve marketplace root: %w", err)
	}

	marketplaces, err := controller.listMarketplaces(ctx, "marketplace list")
	if err != nil {
		return err
	}
	found := false
	for _, candidate := range marketplaces {
		if candidate.Name != plugin.Marketplace {
			continue
		}
		if found {
			return fmt.Errorf("codex marketplace %q is configured more than once", plugin.Marketplace)
		}
		found = true
		if candidate.MarketplaceSource == nil ||
			candidate.MarketplaceSource.SourceType != "local" ||
			!samePath(candidate.Root, marketplaceRoot) ||
			!samePath(candidate.MarketplaceSource.Source, marketplaceRoot) {
			return fmt.Errorf("codex marketplace %q is configured at a different root", plugin.Marketplace)
		}
	}
	if !found {
		if err := controller.addMarketplace(ctx, plugin, marketplaceRoot); err != nil {
			return err
		}
	}
	if err := controller.addPlugin(ctx, plugin); err != nil {
		return err
	}

	plugins, err := controller.listPlugins(ctx, "plugin verification list")
	if err != nil {
		return err
	}
	candidate, count := findInstalled(plugins.Installed, plugin)
	if count == 0 {
		return fmt.Errorf("codex plugin %q is not installed after add", plugin.Selector())
	}
	if count > 1 {
		return fmt.Errorf("codex plugin %q appears more than once", plugin.Selector())
	}
	return verifyActivePlugin(candidate, plugin, marketplaceRoot)
}

func verifyActivePlugin(candidate listedPlugin, plugin releasemanager.PluginRef, marketplaceRoot string) error {
	if err := verifyOwnedPlugin(candidate, plugin, marketplaceRoot); err != nil {
		return err
	}
	if candidate.Enabled == nil || !*candidate.Enabled {
		return fmt.Errorf("codex plugin %q is not enabled", plugin.Selector())
	}
	return nil
}

func verifyOwnedPlugin(candidate listedPlugin, plugin releasemanager.PluginRef, marketplaceRoot string) error {
	if candidate.PluginID != plugin.Selector() {
		return fmt.Errorf("codex plugin id is %q, want %q", candidate.PluginID, plugin.Selector())
	}
	if candidate.Name != plugin.Name {
		return fmt.Errorf("codex plugin %q name is %q, want %q", plugin.Selector(), candidate.Name, plugin.Name)
	}
	if candidate.MarketplaceName != plugin.Marketplace {
		return fmt.Errorf("codex plugin %q marketplace is %q, want %q", plugin.Selector(), candidate.MarketplaceName, plugin.Marketplace)
	}
	if candidate.Version != plugin.Version {
		return fmt.Errorf("codex plugin %q version is %q, want %q", plugin.Selector(), candidate.Version, plugin.Version)
	}
	if candidate.Installed == nil || !*candidate.Installed {
		return fmt.Errorf("codex plugin %q is not marked installed", plugin.Selector())
	}
	if candidate.MarketplaceSource == nil ||
		candidate.MarketplaceSource.SourceType != "local" ||
		!samePath(candidate.MarketplaceSource.Source, marketplaceRoot) {
		return fmt.Errorf("codex plugin %q marketplace source is not the verified release root", plugin.Selector())
	}
	expectedPluginRoot := filepath.Join(marketplaceRoot, "plugins", plugin.Name)
	if candidate.Source.Source != "local" || !samePath(candidate.Source.Path, expectedPluginRoot) {
		return fmt.Errorf("codex plugin %q source is not the verified release plugin", plugin.Selector())
	}
	return nil
}

func (controller Controller) EnsureAbsent(ctx context.Context, plugin releasemanager.PluginRef) error {
	if err := controller.validate(plugin); err != nil {
		return err
	}
	marketplaceRoot, err := filepath.Abs(filepath.Join(controller.StateHome, "versions", plugin.Version, "marketplace"))
	if err != nil {
		return fmt.Errorf("resolve verified release marketplace root: %w", err)
	}
	plugins, err := controller.listPlugins(ctx, "plugin list")
	if err != nil {
		return err
	}
	candidate, count := findInstalled(plugins.Installed, plugin)
	if count > 1 {
		return fmt.Errorf("codex plugin %q appears more than once", plugin.Selector())
	}
	if count == 1 {
		if err := verifyOwnedPlugin(candidate, plugin, marketplaceRoot); err != nil {
			return fmt.Errorf("refuse to remove unowned plugin: %w", err)
		}
		if err := controller.removePlugin(ctx, plugin); err != nil {
			return err
		}
		plugins, err = controller.listPlugins(ctx, "plugin verification list")
		if err != nil {
			return err
		}
		if _, count := findInstalled(plugins.Installed, plugin); count != 0 {
			return fmt.Errorf("codex plugin %q is still installed after remove", plugin.Selector())
		}
	}
	return controller.ensureMarketplaceAbsent(ctx, plugin, marketplaceRoot)
}

func (controller Controller) ensureMarketplaceAbsent(ctx context.Context, plugin releasemanager.PluginRef, marketplaceRoot string) error {
	marketplaces, err := controller.listMarketplaces(ctx, "marketplace cleanup list")
	if err != nil {
		return err
	}
	found := 0
	for _, candidate := range marketplaces {
		if candidate.Name != plugin.Marketplace {
			continue
		}
		found++
		if candidate.MarketplaceSource == nil || candidate.MarketplaceSource.SourceType != "local" ||
			!samePath(candidate.Root, marketplaceRoot) || !samePath(candidate.MarketplaceSource.Source, marketplaceRoot) {
			return fmt.Errorf("refuse to remove marketplace %q from a different root", plugin.Marketplace)
		}
	}
	if found == 0 {
		return nil
	}
	if found > 1 {
		return fmt.Errorf("codex marketplace %q is configured more than once", plugin.Marketplace)
	}
	if err := controller.removeMarketplace(ctx, plugin); err != nil {
		return err
	}
	marketplaces, err = controller.listMarketplaces(ctx, "marketplace cleanup verification list")
	if err != nil {
		return err
	}
	for _, candidate := range marketplaces {
		if candidate.Name == plugin.Marketplace {
			return fmt.Errorf("codex marketplace %q remains after remove", plugin.Marketplace)
		}
	}
	return nil
}

func (controller Controller) validate(plugin releasemanager.PluginRef) error {
	if strings.TrimSpace(controller.StateHome) == "" {
		return errors.New("state home is required")
	}
	if !pluginNamePattern.MatchString(plugin.Name) {
		return fmt.Errorf("plugin name %q is invalid", plugin.Name)
	}
	if !state.ValidVersion(plugin.Version) {
		return fmt.Errorf("plugin version %q must be strict semver", plugin.Version)
	}
	if !marketplaceNamePattern.MatchString(plugin.Marketplace) {
		return fmt.Errorf("marketplace name %q is invalid", plugin.Marketplace)
	}
	return nil
}

func (controller Controller) runner() Runner {
	if controller.Runner != nil {
		return controller.Runner
	}
	return CommandRunner{}
}

func (controller Controller) listMarketplaces(ctx context.Context, label string) ([]marketplace, error) {
	output, err := controller.run(ctx, label, "plugin", "marketplace", "list", "--json")
	if err != nil {
		return nil, err
	}
	var payload marketplaceListPayload
	if err := decodeStrictJSON(output, &payload); err != nil {
		return nil, fmt.Errorf("decode codex %s JSON: %w", label, err)
	}
	if payload.Marketplaces == nil {
		return nil, fmt.Errorf("decode codex %s JSON: marketplaces must be an array", label)
	}
	for index, candidate := range payload.Marketplaces {
		if candidate.Name == "" || candidate.Root == "" {
			return nil, fmt.Errorf("decode codex %s JSON: marketplace %d is incomplete", label, index)
		}
		if candidate.MarketplaceSource != nil && (candidate.MarketplaceSource.SourceType == "" || candidate.MarketplaceSource.Source == "") {
			return nil, fmt.Errorf("decode codex %s JSON: marketplace %d source is incomplete", label, index)
		}
	}
	return payload.Marketplaces, nil
}

func (controller Controller) addMarketplace(ctx context.Context, plugin releasemanager.PluginRef, marketplaceRoot string) error {
	output, err := controller.run(ctx, "marketplace add", "plugin", "marketplace", "add", marketplaceRoot, "--json")
	if err != nil {
		return err
	}
	var result marketplaceAddResult
	if err := decodeStrictJSON(output, &result); err != nil {
		return fmt.Errorf("decode codex marketplace add JSON: %w", err)
	}
	if result.MarketplaceName != plugin.Marketplace {
		return fmt.Errorf("codex marketplace add returned name %q, want %q", result.MarketplaceName, plugin.Marketplace)
	}
	if !samePath(result.InstalledRoot, marketplaceRoot) {
		return fmt.Errorf("codex marketplace add returned a different root")
	}
	if result.AlreadyAdded == nil {
		return errors.New("codex marketplace add omitted alreadyAdded")
	}
	return nil
}

func (controller Controller) addPlugin(ctx context.Context, plugin releasemanager.PluginRef) error {
	output, err := controller.run(ctx, "plugin add", "plugin", "add", plugin.Selector(), "--json")
	if err != nil {
		return err
	}
	var result pluginAddResult
	if err := decodeStrictJSON(output, &result); err != nil {
		return fmt.Errorf("decode codex plugin add JSON: %w", err)
	}
	if result.PluginID != plugin.Selector() || result.Name != plugin.Name || result.MarketplaceName != plugin.Marketplace {
		return fmt.Errorf("codex plugin add returned a different plugin identity")
	}
	if result.Version != plugin.Version {
		return fmt.Errorf("codex plugin add returned version %q, want %q", result.Version, plugin.Version)
	}
	if result.InstalledPath == "" || result.AuthPolicy == "" {
		return errors.New("codex plugin add result is incomplete")
	}
	return nil
}

func (controller Controller) removePlugin(ctx context.Context, plugin releasemanager.PluginRef) error {
	output, err := controller.run(ctx, "plugin remove", "plugin", "remove", plugin.Selector(), "--json")
	if err != nil {
		return err
	}
	var result pluginRemoveResult
	if err := decodeStrictJSON(output, &result); err != nil {
		return fmt.Errorf("decode codex plugin remove JSON: %w", err)
	}
	if result.PluginID != plugin.Selector() || result.Name != plugin.Name || result.MarketplaceName != plugin.Marketplace {
		return errors.New("codex plugin remove returned a different plugin identity")
	}
	return nil
}

func (controller Controller) removeMarketplace(ctx context.Context, plugin releasemanager.PluginRef) error {
	output, err := controller.run(ctx, "marketplace remove", "plugin", "marketplace", "remove", plugin.Marketplace, "--json")
	if err != nil {
		return err
	}
	var result marketplaceRemoveResult
	if err := decodeStrictJSON(output, &result); err != nil {
		return fmt.Errorf("decode codex marketplace remove JSON: %w", err)
	}
	if result.MarketplaceName != plugin.Marketplace || result.InstalledRoot != nil {
		return errors.New("codex marketplace remove returned a different marketplace identity")
	}
	return nil
}

func (controller Controller) listPlugins(ctx context.Context, label string) (pluginListPayload, error) {
	output, err := controller.run(ctx, label, "plugin", "list", "--json")
	if err != nil {
		return pluginListPayload{}, err
	}
	var payload pluginListPayload
	if err := decodeStrictJSON(output, &payload); err != nil {
		return pluginListPayload{}, fmt.Errorf("decode codex %s JSON: %w", label, err)
	}
	if payload.Installed == nil || payload.Available == nil {
		return pluginListPayload{}, fmt.Errorf("decode codex %s JSON: installed and available must be arrays", label)
	}
	for index, candidate := range append(append([]listedPlugin(nil), payload.Installed...), payload.Available...) {
		if err := validateListedPlugin(candidate); err != nil {
			return pluginListPayload{}, fmt.Errorf("decode codex %s JSON: plugin %d: %w", label, index, err)
		}
	}
	return payload, nil
}

func (controller Controller) run(ctx context.Context, label string, args ...string) ([]byte, error) {
	output, err := controller.runner().Run(ctx, args...)
	if err != nil {
		return nil, fmt.Errorf("codex %s: %w", label, err)
	}
	return output, nil
}

func decodeStrictJSON(data []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return fmt.Errorf("trailing data: %w", err)
	}
	return nil
}

func samePath(left, right string) bool {
	return samePathForOS(runtime.GOOS, left, right)
}

func samePathForOS(goos, left, right string) bool {
	leftInfo, leftErr := os.Stat(left)
	rightInfo, rightErr := os.Stat(right)
	if leftErr == nil && rightErr == nil && os.SameFile(leftInfo, rightInfo) {
		return true
	}
	left = canonicalPath(left)
	right = canonicalPath(right)
	if goos == "windows" {
		return strings.EqualFold(left, right)
	}
	return left == right
}

func canonicalPath(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return filepath.Clean(path)
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err == nil {
		return filepath.Clean(resolved)
	}
	return filepath.Clean(absolute)
}

func findInstalled(plugins []listedPlugin, target releasemanager.PluginRef) (listedPlugin, int) {
	var found listedPlugin
	count := 0
	for _, candidate := range plugins {
		if candidate.PluginID == target.Selector() {
			found = candidate
			count++
		}
	}
	return found, count
}

func validateListedPlugin(plugin listedPlugin) error {
	if plugin.PluginID == "" || plugin.Name == "" || plugin.MarketplaceName == "" || plugin.Version == "" {
		return errors.New("identity and version are required")
	}
	if plugin.PluginID != plugin.Name+"@"+plugin.MarketplaceName {
		return errors.New("pluginId does not match name and marketplaceName")
	}
	if plugin.Installed == nil || plugin.Enabled == nil {
		return errors.New("installed and enabled flags are required")
	}
	if plugin.Source.Source == "" || plugin.Source.Path == "" {
		return errors.New("source is incomplete")
	}
	if plugin.MarketplaceSource != nil && (plugin.MarketplaceSource.SourceType == "" || plugin.MarketplaceSource.Source == "") {
		return errors.New("marketplaceSource is incomplete")
	}
	if plugin.InstallPolicy == "" || plugin.AuthPolicy == "" {
		return errors.New("plugin policy is incomplete")
	}
	return nil
}

type marketplaceListPayload struct {
	Marketplaces []marketplace `json:"marketplaces"`
}

type marketplace struct {
	Name              string             `json:"name"`
	Root              string             `json:"root"`
	MarketplaceSource *marketplaceSource `json:"marketplaceSource,omitempty"`
}

type marketplaceSource struct {
	SourceType string `json:"sourceType"`
	Source     string `json:"source"`
}

type marketplaceAddResult struct {
	MarketplaceName string `json:"marketplaceName"`
	InstalledRoot   string `json:"installedRoot"`
	AlreadyAdded    *bool  `json:"alreadyAdded"`
}

type marketplaceRemoveResult struct {
	MarketplaceName string  `json:"marketplaceName"`
	InstalledRoot   *string `json:"installedRoot"`
}

type pluginListPayload struct {
	Installed []listedPlugin `json:"installed"`
	Available []listedPlugin `json:"available"`
}

type listedPlugin struct {
	PluginID          string             `json:"pluginId"`
	Name              string             `json:"name"`
	MarketplaceName   string             `json:"marketplaceName"`
	Version           string             `json:"version"`
	Installed         *bool              `json:"installed"`
	Enabled           *bool              `json:"enabled"`
	Source            pluginSource       `json:"source"`
	MarketplaceSource *marketplaceSource `json:"marketplaceSource,omitempty"`
	InstallPolicy     string             `json:"installPolicy"`
	AuthPolicy        string             `json:"authPolicy"`
}

type pluginSource struct {
	Source string `json:"source"`
	Path   string `json:"path"`
}

type pluginAddResult struct {
	PluginID        string `json:"pluginId"`
	Name            string `json:"name"`
	MarketplaceName string `json:"marketplaceName"`
	Version         string `json:"version"`
	InstalledPath   string `json:"installedPath"`
	AuthPolicy      string `json:"authPolicy"`
}

type pluginRemoveResult struct {
	PluginID        string `json:"pluginId"`
	Name            string `json:"name"`
	MarketplaceName string `json:"marketplaceName"`
}
