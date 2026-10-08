package app

import (
	"fmt"
	"os"
	"time"
)

type BrowserUsageState int

const (
	BrowserUsageReady BrowserUsageState = iota
	BrowserUsageUnavailable
	BrowserUsageMissing
)

type BrowserUsage struct {
	State   BrowserUsageState
	Summary map[string]AliasUsageSummary
}

func (s *Services) BrowserUsage(aliases []Alias, at time.Time) BrowserUsage {
	home, err := s.dependencies.HomeDir()
	if err != nil {
		return BrowserUsage{State: BrowserUsageUnavailable}
	}
	adapter := s.activeShellAdapter()
	path := s.historyPathFor(adapter, home)
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return BrowserUsage{State: BrowserUsageMissing}
	} else if err != nil {
		return BrowserUsage{State: BrowserUsageUnavailable}
	}
	events, err := s.historyUsageEventsFromShell(path, adapter.Name(), aliases)
	if err != nil {
		return BrowserUsage{State: BrowserUsageUnavailable}
	}
	return BrowserUsage{State: BrowserUsageReady, Summary: summarizeAliasUses(events, at)}
}
func (s *Services) EditMetadata(name string, metadata EntryMetadata) error {
	path, err := s.aliasesPath()
	if err != nil {
		return err
	}
	return s.setEntryMetadata(path, name, metadata)
}
func (s *Services) ValidateMetadata(metadata EntryMetadata) (EntryMetadata, error) {
	return s.validateEditableMetadata(metadata)
}
func (s *Services) ValidateContextName(operation, name string) error {
	return validateContextEntryName(operation, name)
}
func (s *Services) ScanEntries() ([]SecretFinding, error) {
	path, err := s.aliasesPath()
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return findSecretFindings(contents), nil
}
func (s *Services) CheckEntries() ([]aliasCheckFinding, error) {
	path, err := s.aliasesPath()
	if err != nil {
		return nil, err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	findings := checkAliasContents(contents)
	return appendNativeSyntaxFinding(findings, s.checkNativeShellSyntax(path, s.activeShellAdapter().Name())), nil
}
func (s *Services) DefaultAliasOffer(path string) ([]DefaultAliasOffer, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return missingDefaultAliases(contents), nil
}
func (s *Services) DataPaths() ([]DataPath, error) {
	home, err := s.dependencies.HomeDir()
	if err != nil {
		return nil, err
	}
	return s.localDataPaths(home)
}
func (s *Services) repositoryComparison() (RepositoryDiffPreview, error) {
	_, source, target, err := s.repositoryPaths()
	if err != nil {
		return RepositoryDiffPreview{}, err
	}
	current, err := readFileLimited(source, AliasFileLimit)
	if err != nil {
		return RepositoryDiffPreview{}, &repositoryReadError{Current: true, Cause: err}
	}
	tracked, err := readFileLimited(target, AliasFileLimit)
	if os.IsNotExist(err) {
		tracked = nil
	} else if err != nil {
		return RepositoryDiffPreview{}, &repositoryReadError{Cause: err}
	}
	return RepositoryDiffPreview{Source: source, Target: target, Local: current, Remote: tracked}, nil
}

type repositoryReadError struct {
	Current bool
	Cause   error
}

func (e *repositoryReadError) Error() string { return e.Cause.Error() }
func (e *repositoryReadError) Unwrap() error { return e.Cause }
func (s *Services) RepositoryComparison() (RepositoryDiffPreview, error) {
	result, err := s.repositoryComparison()
	if read, ok := err.(*repositoryReadError); ok {
		if read.Current {
			return result, fmt.Errorf("read current aliases: %w", read.Cause)
		}
		return result, fmt.Errorf("read tracked aliases: %w", read.Cause)
	}
	return result, err
}
