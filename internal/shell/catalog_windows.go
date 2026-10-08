//go:build windows

package shell

func CatalogZshStartupPath(home string) (string, error) {
	return catalogZshStartupPath(home, defaultRuntime(Runtime{}))
}
func catalogZshStartupPath(home string, runtime Runtime) (string, error) {
	return zshStartupPath(home, runtime)
}
func (bashShellAdapter) CatalogStartupRoute(path string, contents []byte, home string) (string, bool) {
	return "", false
}
func (zshShellAdapter) CatalogStartupRoute(path string, contents []byte, home string) (string, bool) {
	return "", false
}
