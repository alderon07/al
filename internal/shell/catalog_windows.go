//go:build windows

package shell

func CatalogZshStartupPath(home string) (string, error) { return ZshStartupPath(home) }
func (bashShellAdapter) CatalogStartupRoute(path string, contents []byte, home string) (string, bool) {
	return "", false
}
func (zshShellAdapter) CatalogStartupRoute(path string, contents []byte, home string) (string, bool) {
	return "", false
}
