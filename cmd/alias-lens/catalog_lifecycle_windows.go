//go:build windows

package main

import (
	neutralcatalog "alias-lens/internal/catalog"
	workflowplan "alias-lens/internal/plan"
	workflowstate "alias-lens/internal/state"
	tea "alias-lens/internal/tea"
	"fmt"
	"os"
)

func runCatalogLoader([]string) error {
	return fmt.Errorf("catalog activation needs Bash or Zsh on Linux, WSL, or macOS")
}
func runCatalogLifecycleCommand(arguments []string) (bool, int, error) {
	if len(arguments) > 0 {
		switch arguments[0] {
		case "enable", "plan", "review", "approve", "adopt", "rollback":
			return true, 2, runCatalogLoader(nil)
		}
	}
	return false, 0, nil
}
func catalogEntryFacade(_ string, native []Alias) ([]Alias, error)     { return native, nil }
func catalogInstalledDeclaration(string, string) (string, bool, error) { return "", false, nil }
func catalogInstalledCompletionNames(string) ([]string, bool, error)   { return nil, false, nil }
func catalogEntryRunnable(Alias) bool                                  { return true }
func catalogManagedEditing() (bool, error)                             { return false, nil }
func addCatalogAlias(string, string, string, EntryMetadata) error      { return runCatalogLoader(nil) }
func editCatalogAlias(string, string, string, string, EntryMetadata) error {
	return runCatalogLoader(nil)
}
func deleteCatalogAlias(string) error                { return runCatalogLoader(nil) }
func setCatalogMetadata(string, EntryMetadata) error { return runCatalogLoader(nil) }

func readLifecycleInstalledSnapshot(string) (neutralcatalog.Catalog, bool, error) {
	return neutralcatalog.Catalog{}, false, nil
}
func observeLifecycleStatus(*workflowstate.Inputs) {}

func editableEntriesPath() (string, error)           { return aliasesPath() }
func catalogRevisionDirectory(string) (string, bool) { return "", false }
func restoreCatalogBytes([]byte) error               { return runCatalogLoader(nil) }

type catalogTUIView struct {
	Shell   string
	Loading bool
	Err     string
}
type catalogTUILoadedMsg struct{ View *catalogTUIView }
type catalogTUIAppliedMsg struct{ Err error }

func loadCatalogTUI(shell string) tea.Cmd {
	return func() tea.Msg {
		return catalogTUILoadedMsg{&catalogTUIView{Shell: shell, Err: "Catalog activation needs Bash or Zsh on Linux, WSL, or macOS"}}
	}
}
func (m model) updateCatalogTUI(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	if message.Type == tea.KeyEsc {
		m.catalogView = nil
	}
	return m, nil
}
func (m model) catalogTUIView(frame tuiFrame, header string) string {
	return frame.renderWithFooter(header+"\n"+m.catalogView.Err, "esc back")
}

func catalogZshStartupPath(home string) (string, error) { return zshStartupPath(home) }

func runCatalogCheck(bool) (int, error)          { return 2, runCatalogLoader(nil) }
func catalogDoctorRuntime(string) (bool, string) { return false, "catalog activation is unavailable" }

func buildCatalogRequestedPlan(arguments []string) (workflowplan.OperationPlan, bool, error) {
	if len(arguments) > 0 && (arguments[0] == "init" || arguments[0] == "catalog") {
		return workflowplan.OperationPlan{}, true, runCatalogLoader(nil)
	}
	return workflowplan.OperationPlan{}, false, nil
}

func addCatalogDescriptions() error { return fmt.Errorf("catalog editing is unavailable on Windows") }
func importCatalogAliases(aliases []Alias) error {
	return fmt.Errorf("catalog editing is unavailable on Windows")
}
func catalogImportCurrent() ([]byte, error) {
	return nil, fmt.Errorf("catalog editing is unavailable on Windows")
}

func homeDirectory() string { home, _ := os.UserHomeDir(); return home }

func runCatalogConflictReview(string) error { return runCatalogLoader(nil) }

func readEnrolledRepositoryCatalog() (neutralcatalog.Catalog, bool, error) {
	return neutralcatalog.Catalog{}, false, nil
}
