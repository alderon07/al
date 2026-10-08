//go:build windows

package app

import (
	neutralcatalog "github.com/alderon07/al/internal/catalog"

	workflowstate "github.com/alderon07/al/internal/state"

	"fmt"
)

func (svc *Services) catalogEntryFacade(_ string, native []Alias) ([]Alias, error) {
	return native, nil
}
func (svc *Services) catalogInstalledDeclaration(string, string) (string, bool, error) {
	return "", false, nil
}
func (svc *Services) catalogInstalledCompletionNames(string) ([]string, bool, error) {
	return nil, false, nil
}
func catalogEntryRunnable(Alias) bool                      { return true }
func (svc *Services) catalogManagedEditing() (bool, error) { return false, nil }
func (svc *Services) addCatalogAlias(string, string, string, EntryMetadata) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}
func (svc *Services) editCatalogAlias(string, string, string, string, EntryMetadata) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}
func (svc *Services) deleteCatalogAlias(string) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}
func (svc *Services) setCatalogMetadata(string, EntryMetadata) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}

func (svc *Services) readLifecycleInstalledSnapshot(string) (neutralcatalog.Catalog, bool, error) {
	return neutralcatalog.Catalog{}, false, nil
}
func (svc *Services) observeLifecycleStatus(*workflowstate.Inputs) {}

func (svc *Services) editableEntriesPath() (string, error)           { return svc.aliasesPath() }
func (svc *Services) catalogRevisionDirectory(string) (string, bool) { return "", false }
func (svc *Services) restoreCatalogBytes([]byte) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}

func catalogZshStartupPath(home string) (string, error) { return zshStartupPath(home) }

func (svc *Services) catalogDoctorRuntime(string) (bool, string) {
	return false, "catalog activation is unavailable"
}

func (svc *Services) addCatalogDescriptions() error {
	return fmt.Errorf("catalog editing is unavailable on Windows")
}
func (svc *Services) importCatalogAliases(aliases []Alias) error {
	return fmt.Errorf("catalog editing is unavailable on Windows")
}
func (svc *Services) catalogImportCurrent() ([]byte, error) {
	return nil, fmt.Errorf("catalog editing is unavailable on Windows")
}

func (svc *Services) homeDirectory() string { home, _ := svc.dependencies.HomeDir(); return home }

func (svc *Services) readEnrolledRepositoryCatalog() (neutralcatalog.Catalog, bool, error) {
	return neutralcatalog.Catalog{}, false, nil
}

func (svc *Services) restoreCatalogBytesInSession(_ *mutationSession, contents []byte) error {
	return svc.restoreCatalogBytes(contents)
}
