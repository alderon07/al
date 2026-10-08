//go:build !windows

package main

import (
	"fmt"
	"os"
	"path/filepath"

	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogrender"
	"alias-lens/internal/catalogstore"
)

func runCatalogCheck(strict bool) (int, error) {
	value, err := readCatalogFile(localCatalogPath())
	if err != nil {
		return 2, err
	}
	if err := validateCatalogNames(value); err != nil {
		return 1, err
	}
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	_, err = readLifecycleFile(catalogApprovalsPath(), &approvals)
	if err != nil {
		return 2, err
	}
	approved := map[catalogstore.ApprovalKey]bool{}
	for _, record := range approvals.Records {
		approved[record.Key] = true
	}
	config, err := observeConfig()
	if err != nil {
		return 2, err
	}
	shell := activeShellAdapter().Name()
	rendered, problems := catalogrender.RenderV2(value, catalogrender.RenderV2Context{Shell: shell, Platform: currentPlatform(), Profiles: config.Config.Profiles, Approvals: approved, ValidateNative: validateCatalogNativeDeclaration, ResolveExecutable: func(program string) (string, error) { return resolveCatalogExecutable(program, os.Getenv("PATH")) }})
	if len(problems) > 0 {
		return 1, fmt.Errorf("catalog has unsafe declarations; enter al catalog review --shell %s", shell)
	}
	entries := []catalogstore.GenerationEntry{}
	entries = append(entries, rendered.Entries...)
	entryList := []neutralcatalog.Entry{}
	declarations := [][]byte{}
	for _, item := range entries {
		entryList = append(entryList, item.Entry)
		declarations = append(declarations, []byte(item.Declaration))
	}
	adapter := activeShellAdapter()
	native, err := readRegularFile(filepath.Join(homeDirectory(), adapter.AliasFilename()), shadowSourceLimit)
	if err != nil {
		return 2, err
	}
	if err := validateCatalogDeclarations(shell, entryList, declarations, native); err != nil {
		return 1, err
	}
	warnings := len(rendered.PendingApprovals) + len(rendered.Unavailable)
	fmt.Printf("Catalog checked: %d installed candidates, %d pending approvals, %d unavailable entries.\n", len(rendered.IncludedEntryIDs), len(rendered.PendingApprovals), len(rendered.Unavailable))
	if strict && warnings > 0 {
		return 1, nil
	}
	return 0, nil
}

func catalogDoctorRuntime(shell string) (bool, string) {
	active, err := catalogShellInstalled(shell)
	if err != nil {
		return false, "catalog state is invalid; run al doctor"
	}
	if !active {
		return false, ""
	}
	adapter, _ := shellAdapter(shell)
	_, err = readCatalogGeneration(catalogGeneratedRoot(shell), filepath.Join(homeDirectory(), adapter.AliasFilename()), shell)
	if err != nil {
		return false, "catalog runtime declined; run al catalog enable --shell " + shell
	}
	return true, "verified catalog package; ready for new shells"
}
