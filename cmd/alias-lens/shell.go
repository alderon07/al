package main

import (
	"alias-lens/internal/shell"
	"alias-lens/internal/transaction"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const activeShellEnvironment = "ALIAS_LENS_SHELL"
const historyFileEnvironment = "ALIAS_LENS_HISTORY_FILE"

type ShellAdapter interface {
	shell.Adapter
	ConfigureStartup(home, platform, executableDir string) error
	ConfigureStartupInSession(session *mutationSession, home, platform, executableDir string) error
	RemoveStartup(home, platform string) error
	RemoveStartupInSession(session *mutationSession, home, platform string) error
}
type shellHistoryState = shell.HistoryState
type shellExecutionSpec = shell.ExecutionSpec
type shellPromptSpec = shell.PromptSpec
type shellBindingSpec = shell.BindingSpec
type appShellAdapter struct{ shell.Adapter }

func shellAdapter(name string) (ShellAdapter, error) {
	adapter, err := shell.New(name)
	if err != nil {
		return nil, err
	}
	return appShellAdapter{Adapter: adapter}, nil
}
func mustShellAdapter(name string) ShellAdapter {
	adapter, err := shellAdapter(name)
	if err != nil {
		panic(err)
	}
	return adapter
}
func activeShellAdapter() ShellAdapter {
	if adapter, err := shellAdapter(os.Getenv(activeShellEnvironment)); err == nil {
		return adapter
	}
	if config, err := loadConfig(); err == nil {
		if adapter, adapterErr := shellAdapter(config.Shell); adapterErr == nil {
			return adapter
		}
	}
	return mustShellAdapter("bash")
}
func requestedShellAdapter(name string) (ShellAdapter, error) {
	if name != "" {
		return shellAdapter(name)
	}
	if configured := os.Getenv(activeShellEnvironment); configured != "" {
		return shellAdapter(configured)
	}
	if detected := filepath.Base(strings.TrimSpace(os.Getenv("SHELL"))); detected != "." && detected != "" {
		return shellAdapter(detected)
	}
	return activeShellAdapter(), nil
}
func printShellEntry(name string) error {
	definition, err := loadShellEntry(name)
	if err != nil {
		return err
	}
	fmt.Println(definition)
	return nil
}
func loadShellEntry(name string) (string, error) {
	if definition, handled, err := catalogInstalledDeclaration(name, activeShellAdapter().Name()); handled || err != nil {
		return definition, err
	}
	alias, err := loadAliasEntry(name)
	if err != nil {
		return "", err
	}
	return legacyShellEntryHandoff(activeShellAdapter(), alias)
}
func loadAliasEntry(name string) (Alias, error) {
	if !aliasName.MatchString(name) {
		return Alias{}, fmt.Errorf("invalid alias name %q", name)
	}
	aliases, err := loadAliases()
	if err != nil {
		return Alias{}, err
	}
	for _, alias := range aliases {
		if alias.Name == name {
			return alias, nil
		}
	}
	return Alias{}, fmt.Errorf("alias %q is no longer in %s", name, aliasDisplayPath())
}
func shellEntryDefinition(alias Alias) (string, error) {
	if alias.CatalogID != "" {
		definition, handled, err := catalogInstalledDeclaration(alias.Name, activeShellAdapter().Name())
		if handled || err != nil {
			return definition, err
		}
	}
	return legacyShellEntryHandoff(activeShellAdapter(), alias)
}
func aliasPathFor(adapter ShellAdapter) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, adapter.AliasFilename()), nil
}
func aliasDisplayPath() string {
	return "~/" + activeShellAdapter().AliasFilename()
}
func historyPathFor(adapter ShellAdapter, home string) string {
	if configured := strings.TrimSpace(os.Getenv(historyFileEnvironment)); configured != "" {
		return configured
	}
	return filepath.Join(home, adapter.HistoryFilename())
}
func ensureStartupFileLoads(path, aliasFilename, block string) error {
	return configureStartupFile(path, aliasFilename, block, "", "")
}
func configureStartupFile(path, aliasFilename, aliasBlock, home, executableDir string) error {
	return withMutation(func(session *mutationSession) error {
		return configureStartupFileInSession(session, path, aliasFilename, aliasBlock, home, executableDir)
	})
}
func configureStartupFileInSession(session *mutationSession, path, aliasFilename, aliasBlock, home, executableDir string) error {
	edit, err := shell.PlanStartupFile(path, aliasFilename, aliasBlock, home, executableDir)
	if err != nil {
		return err
	}
	return applyStartupEdits(session, []shell.StartupEdit{edit})
}

func removeStartupFileBlocks(path string, blocks ...string) error {
	return withMutation(func(session *mutationSession) error {
		return removeStartupFileBlocksInSession(session, path, blocks...)
	})
}
func removeStartupFileBlocksInSession(session *mutationSession, path string, blocks ...string) error {
	edit, err := shell.PlanRemoveStartupFile(path, blocks...)
	if err != nil {
		return err
	}
	return applyStartupEdits(session, []shell.StartupEdit{edit})
}

func writeStartupFile(path string, contents, updated []byte) error {
	return withMutation(func(session *mutationSession) error {
		return writeStartupFileInSession(session, path, contents, updated)
	})
}
func writeStartupFileInSession(session *mutationSession, path string, contents, updated []byte) error {
	identity, err := transaction.InspectWorkflowTarget(path, transaction.MaxPrivateFileSize, true)
	if err != nil {
		return err
	}
	mode := os.FileMode(0o644)
	if identity.Exists {
		mode = os.FileMode(identity.Mode)
	}
	if len(contents) > 0 {
		if err := writePrivateBackupInSession(session, path+".alias-lens.bak", contents); err != nil {
			return fmt.Errorf("create startup backup: %w", err)
		}
	}
	return session.writeUserFile(path, contents, updated, mode, true, 2, true)
}
func (adapter appShellAdapter) ConfigureStartup(home, platform, executableDir string) error {
	return withMutation(func(session *mutationSession) error {
		return adapter.ConfigureStartupInSession(session, home, platform, executableDir)
	})
}
func (adapter appShellAdapter) ConfigureStartupInSession(session *mutationSession, home, platform, executableDir string) error {
	edits, err := adapter.PlanConfigureStartup(home, platform, executableDir)
	if err != nil {
		return err
	}
	return applyStartupEdits(session, edits)
}
func (adapter appShellAdapter) RemoveStartup(home, platform string) error {
	return withMutation(func(session *mutationSession) error { return adapter.RemoveStartupInSession(session, home, platform) })
}
func (adapter appShellAdapter) RemoveStartupInSession(session *mutationSession, home, platform string) error {
	edits, err := adapter.PlanRemoveStartup(home, platform)
	if err != nil {
		return err
	}
	return applyStartupEdits(session, edits)
}
func applyStartupEdits(session *mutationSession, edits []shell.StartupEdit) error {
	for _, edit := range edits {
		if string(edit.Before) == string(edit.After) {
			continue
		}
		if err := writeStartupFileInSession(session, edit.Path, edit.Before, edit.After); err != nil {
			return err
		}
	}
	return nil
}

func parseHistoryUnix(value string) (int64, error) { return strconv.ParseInt(value, 10, 64) }
func validateLegacyEntryName(name, kind string) error {
	return shell.ValidateLegacyEntryName(name, kind)
}
func renderLegacyEntryDefinition(adapter ShellAdapter, alias Alias) (string, error) {
	return shell.RenderLegacyEntryDefinition(adapter, alias)
}
func zshStartupPath(home string) (string, error) { return shell.ZshStartupPath(home) }
func bashLoginPath(home string) (string, error)  { return shell.BashLoginPath(home) }
func bashLoginStartupSupported(platform string) bool {
	return shell.BashLoginStartupSupported(platform)
}
func removeStartupPathBlocks(contents []byte) []byte { return shell.RemoveStartupPathBlocks(contents) }
func startupPathReady(contents []byte, aliasFilename, home, directory string) bool {
	return shell.StartupPathReady(contents, aliasFilename, home, directory)
}
func startupPathBlock(home, directory string) string { return shell.StartupPathBlock(home, directory) }
func hasActiveShellReference(contents []byte, filename string) bool {
	return shell.HasActiveShellReference(contents, filename)
}

const bashAliasLoader = shell.BashAliasLoader
const bashLoginLoader = shell.BashLoginLoader
const zshAliasLoader = shell.ZshAliasLoader
const bashIntegration = shell.BashIntegration
const zshIntegration = shell.ZshIntegration
