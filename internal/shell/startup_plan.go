package shell

import (
	"os"
	"path/filepath"
	"strings"
)

type StartupEdit struct {
	Path          string
	Before, After []byte
}

func PlanStartupFile(path, aliasFilename, aliasBlock, home, executableDir string) (StartupEdit, error) {
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return StartupEdit{}, err
	}
	updated := RemoveStartupPathBlocks(contents)
	updated = []byte(strings.ReplaceAll(string(updated), aliasBlock, ""))
	if executableDir != "" && !StartupPathReady(updated, aliasFilename, home, executableDir) {
		updated = append([]byte(StartupPathBlock(home, executableDir)), updated...)
	}
	if !HasActiveShellReference(updated, aliasFilename) {
		updated = append(updated, []byte(aliasBlock)...)
	}
	return StartupEdit{Path: path, Before: contents, After: updated}, nil
}

func PlanRemoveStartupFile(path string, blocks ...string) (StartupEdit, error) {
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return StartupEdit{Path: path}, nil
	}
	if err != nil {
		return StartupEdit{}, err
	}
	updated := RemoveStartupPathBlocks(contents)
	for _, block := range blocks {
		updated = []byte(strings.ReplaceAll(string(updated), block, ""))
	}
	updated = []byte(strings.TrimLeft(string(updated), "\n"))
	return StartupEdit{Path: path, Before: contents, After: updated}, nil
}

func (adapter bashShellAdapter) PlanConfigureStartup(home, platform, executableDir string) ([]StartupEdit, error) {
	first, err := PlanStartupFile(filepath.Join(home, ".bashrc"), ".bash_aliases", BashAliasLoader, home, executableDir)
	if err != nil {
		return nil, err
	}
	edits := []StartupEdit{first}
	if !BashLoginStartupSupported(platform) {
		return edits, nil
	}
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(paths[1])
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	updated := []byte(strings.ReplaceAll(string(contents), BashLoginLoader, ""))
	if !HasActiveShellReference(updated, ".bashrc") && !HasActiveShellReference(updated, ".bash_aliases") {
		updated = append(updated, []byte(BashLoginLoader)...)
	}
	return append(edits, StartupEdit{Path: paths[1], Before: contents, After: updated}), nil
}

func (adapter bashShellAdapter) PlanRemoveStartup(home, platform string) ([]StartupEdit, error) {
	edit, err := PlanRemoveStartupFile(filepath.Join(home, ".bashrc"), BashAliasLoader)
	if err != nil {
		return nil, err
	}
	edits := []StartupEdit{edit}
	if !BashLoginStartupSupported(platform) {
		return edits, nil
	}
	for _, name := range []string{".bash_profile", ".bash_login", ".profile"} {
		edit, err := PlanRemoveStartupFile(filepath.Join(home, name), BashLoginLoader)
		if err != nil {
			return nil, err
		}
		edits = append(edits, edit)
	}
	return edits, nil
}

func (adapter zshShellAdapter) PlanConfigureStartup(home, platform, executableDir string) ([]StartupEdit, error) {
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return nil, err
	}
	edit, err := PlanStartupFile(paths[0], ".zsh_aliases", ZshAliasLoader, home, executableDir)
	if err != nil {
		return nil, err
	}
	return []StartupEdit{edit}, nil
}

func (adapter zshShellAdapter) PlanRemoveStartup(home, platform string) ([]StartupEdit, error) {
	paths, err := adapter.StartupPaths(home, platform)
	if err != nil {
		return nil, err
	}
	edit, err := PlanRemoveStartupFile(paths[0], ZshAliasLoader)
	if err != nil {
		return nil, err
	}
	return []StartupEdit{edit}, nil
}
