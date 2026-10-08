package app

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

type ShellAdapter = shell.Adapter

type shellMutationAdapter interface {
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
type appShellAdapter struct {
	shell.Adapter
	services *Services
}

func (svc *Services) shellAdapter(name string) (shellMutationAdapter, error) {
	adapter, err := shell.NewWithRuntime(name, shell.Runtime{Environment: svc.dependencies.Environment, WorkingDir: svc.dependencies.WorkingDir})
	if err != nil {
		return nil, err
	}
	return appShellAdapter{Adapter: adapter, services: svc}, nil
}
func (svc *Services) mustShellAdapter(name string) shellMutationAdapter {
	adapter, err := svc.shellAdapter(name)
	if err != nil {
		panic(err)
	}
	return adapter
}
func (svc *Services) activeShellAdapter() shellMutationAdapter {
	if adapter, err := svc.shellAdapter(svc.dependencies.Environment(activeShellEnvironment)); err == nil {
		return adapter
	}
	if config, err := svc.loadConfig(); err == nil {
		if adapter, adapterErr := svc.shellAdapter(config.Shell); adapterErr == nil {
			return adapter
		}
	}
	return svc.mustShellAdapter("bash")
}
func (svc *Services) requestedShellAdapter(name string) (shellMutationAdapter, error) {
	if name != "" {
		return svc.shellAdapter(name)
	}
	if configured := svc.dependencies.Environment(activeShellEnvironment); configured != "" {
		return svc.shellAdapter(configured)
	}
	if detected := filepath.Base(strings.TrimSpace(svc.dependencies.Environment("SHELL"))); detected != "." && detected != "" {
		return svc.shellAdapter(detected)
	}
	return svc.activeShellAdapter(), nil
}

func (svc *Services) loadShellEntry(name string) (string, error) {
	if definition, handled, err := svc.catalogInstalledDeclaration(name, svc.activeShellAdapter().Name()); handled || err != nil {
		return definition, err
	}
	alias, err := svc.loadAliasEntry(name)
	if err != nil {
		return "", err
	}
	return legacyShellEntryHandoff(svc.activeShellAdapter(), alias)
}
func (svc *Services) loadAliasEntry(name string) (Alias, error) {
	if !aliasName.MatchString(name) {
		return Alias{}, fmt.Errorf("invalid alias name %q", name)
	}
	aliases, err := svc.loadAliases()
	if err != nil {
		return Alias{}, err
	}
	for _, alias := range aliases {
		if alias.Name == name {
			return alias, nil
		}
	}
	return Alias{}, fmt.Errorf("alias %q is no longer in %s", name, svc.aliasDisplayPath())
}
func (svc *Services) shellEntryDefinition(alias Alias) (string, error) {
	if alias.CatalogID != "" {
		definition, handled, err := svc.catalogInstalledDeclaration(alias.Name, svc.activeShellAdapter().Name())
		if handled || err != nil {
			return definition, err
		}
	}
	return legacyShellEntryHandoff(svc.activeShellAdapter(), alias)
}
func (svc *Services) aliasPathFor(adapter ShellAdapter) (string, error) {
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, adapter.AliasFilename()), nil
}
func (svc *Services) aliasDisplayPath() string {
	return "~/" + svc.activeShellAdapter().AliasFilename()
}
func (svc *Services) historyPathFor(adapter ShellAdapter, home string) string {
	if configured := strings.TrimSpace(svc.dependencies.Environment(historyFileEnvironment)); configured != "" {
		return configured
	}
	return filepath.Join(home, adapter.HistoryFilename())
}
func (svc *Services) ensureStartupFileLoads(path, aliasFilename, block string) error {
	return svc.configureStartupFile(path, aliasFilename, block, "", "")
}
func (svc *Services) configureStartupFile(path, aliasFilename, aliasBlock, home, executableDir string) error {
	return svc.withMutation(func(session *mutationSession) error {
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

func (svc *Services) removeStartupFileBlocks(path string, blocks ...string) error {
	return svc.withMutation(func(session *mutationSession) error {
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

func (svc *Services) writeStartupFile(path string, contents, updated []byte) error {
	return svc.withMutation(func(session *mutationSession) error {
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
	svc := adapter.services
	if svc == nil {
		svc =
			DefaultServices()
	}

	return svc.withMutation(func(session *mutationSession) error {
		return adapter.ConfigureStartupInSession(session, home, platform, executableDir)
	})
}
func (adapter appShellAdapter) ConfigureStartupInSession(session *mutationSession, home, platform, executableDir string) error {
	svc := adapter.services
	if svc == nil {
		svc =
			DefaultServices()
	}

	edits, err := adapter.PlanConfigureStartup(home, platform, executableDir)
	if err != nil {
		return err
	}
	return applyStartupEdits(session, edits)
}
func (adapter appShellAdapter) RemoveStartup(home, platform string) error {
	svc := adapter.services
	if svc == nil {
		svc =
			DefaultServices()
	}

	return svc.withMutation(func(session *mutationSession) error { return adapter.RemoveStartupInSession(session, home, platform) })
}
func (adapter appShellAdapter) RemoveStartupInSession(session *mutationSession, home, platform string) error {
	svc := adapter.services
	if svc == nil {
		svc =
			DefaultServices()
	}

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
