package app

import (
	"bytes"
	"fmt"
	"github.com/alderon07/al/internal/usagelog"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

type ImportPreview struct {
	Plan       ImportPlan
	plan       ImportPlan
	sourcePath string
	aliasPath  string
	current    []byte
	managed    bool
}

type ImportResult struct {
	Added   int
	Catalog bool
}

type ExportRequest struct {
	Kind       string
	Format     string
	Period     string
	At         time.Time
	OutputPath string
}

func (s *Services) PrepareImport(sourcePath string) (ImportPreview, error) {
	source, err := readRegularFile(sourcePath, ImportFileLimit)
	if err != nil {
		return ImportPreview{}, fmt.Errorf("read import file: %s; choose a regular file no larger than 8 MiB", terminalSafeText(err.Error()))
	}
	if bytes.IndexByte(source, 0) >= 0 || !utf8.Valid(source) {
		return ImportPreview{}, fmt.Errorf("read import file: expected UTF-8 text without NUL bytes")
	}
	aliasPath, err := s.aliasesPath()
	if err != nil {
		return ImportPreview{}, err
	}
	current, err := readFileLimited(aliasPath, AliasFileLimit)
	if err != nil {
		return ImportPreview{}, err
	}
	managed, err := s.catalogManagedEditing()
	if err != nil {
		return ImportPreview{}, err
	}
	if managed {
		current, err = s.catalogImportCurrent()
		if err != nil {
			return ImportPreview{}, err
		}
	}
	plan := s.buildImportPlan(source, current, s.activeShellAdapter())
	native, err := checkImportSyntax(source, s.activeShellAdapter())
	if err != nil {
		return ImportPreview{}, err
	}
	if native != nil {
		plan.Issues = append(plan.Issues, ImportIssue{Line: native.Line, Kind: strings.ToLower(string(native.Severity)), Message: native.Message, Fatal: native.Severity == CheckError})
	}
	return ImportPreview{Plan: copyImportPlan(plan), plan: plan, sourcePath: sourcePath, aliasPath: aliasPath, current: current, managed: managed}, nil
}

func (s *Services) ApplyImport(preview ImportPreview) (ImportResult, error) {
	plan := preview.plan
	aliasPath, current := preview.aliasPath, preview.current
	for _, issue := range plan.Issues {
		if issue.Fatal {
			return ImportResult{}, fmt.Errorf("import has blocking problems; fix them and rerun al import %s", terminalSafeText(preview.sourcePath))
		}
	}
	if len(plan.Add) == 0 {
		return ImportResult{}, nil
	}
	if preview.managed {
		if err := s.importCatalogAliases(plan.Add); err != nil {
			return ImportResult{}, err
		}
		return ImportResult{Added: len(plan.Add), Catalog: true}, nil
	}
	info, err := os.Stat(aliasPath)
	if err != nil {
		return ImportResult{}, err
	}
	lines := strings.Split(strings.TrimSuffix(string(current), "\n"), "\n")
	for _, alias := range plan.Add {
		metadata := EntryMetadata{Tags: alias.Tags, Platforms: alias.Platforms, Favorite: alias.Favorite}
		if alias.Category != s.category(alias.Command) {
			metadata.Category = alias.Category
		}
		lines = s.insertAliasLinesWithMetadata(lines, alias.Name, alias.Command, alias.Description, metadata)
	}
	updated := []byte(strings.TrimLeft(strings.Join(lines, "\n"), "\n") + "\n")
	if err := s.writeAliasFile(aliasPath, current, updated, info.Mode().Perm()); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{Added: len(plan.Add)}, nil
}

func (s *Services) ExportData(request ExportRequest) ([]byte, error) {
	contents, err := s.buildExport(request.Kind, request.Format, request.Period, request.At)
	if err != nil {
		return nil, err
	}
	if request.OutputPath != "" {
		if err := writePrivateExport(request.OutputPath, contents); err != nil {
			return nil, err
		}
	}
	return contents, nil
}

func (s *Services) RecoverWorkflows() error {
	return s.withMutation(func(*mutationSession) error { return nil })
}

func (s *Services) ClearUsageData() error {
	home, err := s.dependencies.HomeDir()
	if err != nil {
		return err
	}
	return s.withMutation(func(*mutationSession) error { return clearManagedPrivateFile(usagelog.Path(home)) })
}

func (s *Services) ClearRevisionData() error {
	home, err := s.dependencies.HomeDir()
	if err != nil {
		return err
	}
	return s.withMutation(func(*mutationSession) error {
		if err := clearManagedRevisions(filepath.Join(home, ".local", "share", "alias-lens", "revisions")); err != nil {
			return err
		}
		return clearManagedRevisions(filepath.Join(home, ".local", "state", "alias-lens", "catalog-revisions"))
	})
}

func (s *Services) SetAutomaticSync(enabled bool) error {
	return s.withMutation(func(session *mutationSession) error {
		fresh, err := s.loadConfig()
		if err != nil {
			return err
		}
		if enabled {
			if _, _, err = s.autosyncUnits(fresh); err != nil {
				return err
			}
		}
		fresh.AutoSync.Enabled = enabled
		if err = s.saveConfigInSession(session, fresh); err != nil {
			return err
		}
		if enabled {
			if s.dependencies.StartWatcher != nil {
				return s.dependencies.StartWatcher()
			}
			return watchProcessLaunch(session)
		}
		return nil
	})
}

func copyImportPlan(plan ImportPlan) ImportPlan {
	result := ImportPlan{Add: append([]Alias(nil), plan.Add...), Skip: append([]string(nil), plan.Skip...), Issues: append([]ImportIssue(nil), plan.Issues...)}
	for i := range result.Add {
		result.Add[i].Tags = append([]string(nil), plan.Add[i].Tags...)
		result.Add[i].Platforms = append([]string(nil), plan.Add[i].Platforms...)
		result.Add[i].Issues = append([]string(nil), plan.Add[i].Issues...)
	}
	return result
}
