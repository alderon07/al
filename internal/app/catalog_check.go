//go:build !windows

package app

import "path/filepath"

func (svc *Services) catalogDoctorRuntime(shell string) (bool, string) {
	active, err := svc.catalogShellInstalled(shell)
	if err != nil {
		return false, "catalog state is invalid; run al doctor"
	}
	if !active {
		return false, ""
	}
	adapter, _ := svc.shellAdapter(shell)
	_, err = svc.readCatalogGeneration(svc.catalogGeneratedRoot(shell), filepath.Join(svc.homeDirectory(), adapter.AliasFilename()), shell)
	if err != nil {
		return false, "catalog runtime declined; run al catalog enable --shell " + shell
	}
	return true, "verified catalog package; ready for new shells"
}
