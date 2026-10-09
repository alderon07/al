//go:build !windows

package app

import shellapi "github.com/alderon07/al/internal/shell"

type catalogStartupEdit struct {
	Path      string
	Before    []byte
	After     []byte
	Route     string
	Reordered bool
}

const catalogLoaderStart = "# >>> Alias Lens catalog v2 >>>"
const catalogLoaderEnd = "# <<< Alias Lens catalog v2 <<<"

func catalogLoaderBlock(shell, nativePath, executable, generatedRoot string, sourceNative bool) string {
	return shellapi.CatalogLoaderBlock(shell, nativePath, executable, generatedRoot, sourceNative)
}
func catalogStaticSource(line, home, path string) bool {
	return shellapi.CatalogStaticSource(line, home, path)
}
func catalogStartupPlacement(contents []byte, shell, home, nativePath string, names map[string]bool) ([]byte, bool, error) {
	return shellapi.CatalogStartupPlacement(contents, shell, home, nativePath, names)
}
func catalogStartupPlacementAt(contents []byte, shell, home, nativePath string, names map[string]bool) ([]byte, bool, int, error) {
	return shellapi.CatalogStartupPlacementAt(contents, shell, home, nativePath, names)
}
func catalogZshStartupPath(home string) (string, error) { return shellapi.CatalogZshStartupPath(home) }
func catalogLoginLoadsBashrc(contents []byte, home string) bool {
	return shellapi.CatalogLoginLoadsBashrc(contents, home)
}
func catalogPinnedIntegration(adapter ShellAdapter, executable string) (string, error) {
	return shellapi.CatalogPinnedIntegration(adapter, executable)
}
func catalogRemoveOwnedIntegration(adapter ShellAdapter, native []byte) ([]byte, int, error) {
	return shellapi.CatalogRemoveOwnedIntegration(adapter, native)
}
