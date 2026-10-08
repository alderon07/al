//go:build !windows

package app

import (
	"fmt"

	"path/filepath"

	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogrender"
	"alias-lens/internal/catalogstore"
)

func (s *Services) CheckCatalog() (CatalogCheck, error) {
	value, err := readCatalogFile(s.localCatalogPath())
	if err != nil {
		return CatalogCheck{}, err
	}
	if err := s.validateCatalogNames(value); err != nil {
		return CatalogCheck{}, &CatalogCheckError{Cause: err}
	}
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	_, err = readLifecycleFile(s.catalogApprovalsPath(), &approvals)
	if err != nil {
		return CatalogCheck{}, err
	}
	approved := map[catalogstore.ApprovalKey]bool{}
	for _, record := range approvals.Records {
		approved[record.Key] = true
	}
	config, err := s.observeConfig()
	if err != nil {
		return CatalogCheck{}, err
	}
	shell := s.activeShellAdapter().Name()
	rendered, problems := catalogrender.RenderV2(value, catalogrender.RenderV2Context{Shell: shell, Platform: s.currentPlatform(), Profiles: config.Config.Profiles, Approvals: approved, ValidateNative: s.validateCatalogNativeDeclaration, ResolveExecutable: func(program string) (string, error) {
		return resolveCatalogExecutable(program, s.dependencies.Environment("PATH"))
	}})
	if len(problems) > 0 {
		return CatalogCheck{}, &CatalogCheckError{Cause: fmt.Errorf("catalog has unsafe declarations; enter al catalog review --shell %s", shell)}
	}
	entries := []catalogstore.GenerationEntry{}
	entries = append(entries, rendered.Entries...)
	entryList := []neutralcatalog.Entry{}
	declarations := [][]byte{}
	for _, item := range entries {
		entryList = append(entryList, item.Entry)
		declarations = append(declarations, []byte(item.Declaration))
	}
	adapter := s.activeShellAdapter()
	native, err := readRegularFile(filepath.Join(s.homeDirectory(), adapter.AliasFilename()), ShadowSourceLimit)
	if err != nil {
		return CatalogCheck{}, err
	}
	if err := s.validateCatalogDeclarations(shell, entryList, declarations, native); err != nil {
		return CatalogCheck{}, &CatalogCheckError{Cause: err}
	}
	return CatalogCheck{Installed: len(rendered.IncludedEntryIDs), Pending: len(rendered.PendingApprovals), Unavailable: len(rendered.Unavailable)}, nil
}

type CatalogCheck struct{ Installed, Pending, Unavailable int }
type CatalogCheckError struct{ Cause error }

func (e *CatalogCheckError) Error() string { return e.Cause.Error() }
func (e *CatalogCheckError) Unwrap() error { return e.Cause }
