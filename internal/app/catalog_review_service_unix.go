//go:build !windows

package app

import (
	"encoding/json"
	"fmt"
	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"github.com/alderon07/al/internal/presentation"
	"os"
	"path/filepath"
	"strings"
)

type CatalogReview struct {
	Decisions  CatalogLifecycleDecisions
	Shell      string
	nativePath string
	native     []byte
	approved   map[catalogstore.ApprovalKey]bool
}
type CatalogNativeReview struct {
	Entry       neutralcatalog.Entry
	Declaration string
	Approval    catalogstore.ApprovalRecord
}
type CatalogAdoptionReview struct {
	Entry       neutralcatalog.Entry
	DisplayPath string
	Adoption    catalogstore.Adoption
}
type CatalogReviewItem struct {
	Native   *CatalogNativeReview
	Adoption *CatalogAdoptionReview
}
type CatalogConflictSummary struct {
	ID     string
	Fields []neutralcatalog.MergeConflict
}
type CatalogReviewSnapshot struct {
	Decisions CatalogLifecycleDecisions
	Entries   []Alias
	Items     []CatalogReviewItem
	Conflicts []CatalogConflictSummary
	Warning   string
}

func (s *Services) PrepareCatalogReview(value neutralcatalog.Catalog, shell string) (CatalogReview, error) {
	return s.prepareCatalogReview(value, shell, true)
}
func (s *Services) prepareCatalogReview(value neutralcatalog.Catalog, shell string, strict bool) (CatalogReview, error) {
	encoded, _ := neutralcatalog.Encode(value)
	review := CatalogReview{Shell: shell, Decisions: CatalogLifecycleDecisions{ConfirmedAt: s.lifecycleTimestamp(), SourcePath: s.localCatalogPath(), SourceSHA256: hashBytes(encoded)}, approved: map[catalogstore.ApprovalKey]bool{}}
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	if _, err := readLifecycleFile(s.catalogApprovalsPath(), &approvals); err != nil {
		return review, err
	}
	for _, record := range approvals.Records {
		review.approved[record.Key] = true
	}
	adapter, err := s.shellAdapter(shell)
	if err != nil {
		return review, err
	}
	review.nativePath = filepath.Join(s.homeDirectory(), adapter.AliasFilename())
	review.native, err = readRegularFile(review.nativePath, ShadowSourceLimit)
	if os.IsNotExist(err) {
		review.native = []byte{}
	} else if err != nil && strict {
		return review, err
	}
	return review, nil
}
func (s *Services) CatalogNativeReview(review CatalogReview, entry neutralcatalog.Entry) (*CatalogNativeReview, error) {
	implementation, exists := entry.Native[review.Shell]
	if !exists {
		return nil, nil
	}
	declaration, err := catalogstore.Declaration(entry, review.Shell, "")
	if err != nil {
		return nil, err
	}
	if err := s.validateCatalogDeclaration(review.Shell, entry, []byte(declaration)); err != nil {
		return nil, err
	}
	key := catalogstore.NativeApproval(entry, review.Shell, implementation)
	if review.approved[key] {
		return nil, nil
	}
	return &CatalogNativeReview{Entry: entry, Declaration: declaration, Approval: catalogstore.ApprovalRecord{Key: key, ApprovedAt: review.Decisions.ConfirmedAt}}, nil
}
func (s *Services) CatalogAdoptionReview(review CatalogReview, entry neutralcatalog.Entry) (CatalogAdoptionReview, error) {
	record, err := s.catalogAdoptionForEntry(entry, review.Shell, review.nativePath, review.native)
	return CatalogAdoptionReview{Entry: entry, DisplayPath: s.displayPrivatePath(review.nativePath), Adoption: record}, err
}
func (s *Services) LoadCatalogReview(shell string) (CatalogReviewSnapshot, error) {
	result := CatalogReviewSnapshot{Decisions: CatalogLifecycleDecisions{ConfirmedAt: s.lifecycleTimestamp()}}
	value, err := readCatalogFile(s.localCatalogPath())
	if err != nil {
		return result, err
	}
	review, err := s.prepareCatalogReview(value, shell, false)
	if err != nil {
		return result, err
	}
	review.Decisions.ConfirmedAt = result.Decisions.ConfirmedAt
	result.Decisions = review.Decisions
	result.Items, result.Warning, err = s.catalogPendingReview(review, value, true, true)
	if err != nil {
		return result, err
	}
	result.Entries, err = s.loadAliases()
	if err != nil && result.Warning == "" {
		result.Warning = err.Error()
	}

	conflictRoot := filepath.Join(filepath.Dir(s.localCatalogPath()), "catalog-conflicts")
	if directories, err := os.ReadDir(conflictRoot); err == nil {
		for _, directory := range directories {
			if !directory.IsDir() || !catalogstore.ValidHash(directory.Name()) {
				continue
			}
			path := filepath.Join(conflictRoot, directory.Name(), "conflicts.json")
			data, err := catalogstore.ReadPrivateBytes(path, catalogstore.MaxDocumentBytes)
			if err != nil {
				result.Warning = "Saved conflict metadata needs attention; enter al doctor"
				continue
			}
			var metadata struct {
				Version      int                            `json:"version"`
				BaseSHA256   string                         `json:"base_sha256"`
				LocalSHA256  string                         `json:"local_sha256"`
				RemoteSHA256 string                         `json:"remote_sha256"`
				Conflicts    []neutralcatalog.MergeConflict `json:"conflicts"`
			}
			if json.Unmarshal(data, &metadata) != nil || metadata.Version != 1 {
				result.Warning = "Saved conflict metadata is invalid"
				continue
			}
			result.Conflicts = append(result.Conflicts, CatalogConflictSummary{ID: directory.Name(), Fields: metadata.Conflicts})
		}
	}
	return result, nil
}

func (s *Services) CatalogPendingReview(review CatalogReview, value neutralcatalog.Catalog, adopt bool) ([]CatalogReviewItem, error) {
	items, _, err := s.catalogPendingReview(review, value, adopt, false)
	return items, err
}

func (s *Services) catalogPendingReview(review CatalogReview, value neutralcatalog.Catalog, adopt, tolerant bool) ([]CatalogReviewItem, string, error) {
	items := []CatalogReviewItem{}
	warning := ""
	ledger := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	if adopt {
		if _, err := readLifecycleFile(s.catalogAdoptionsPath(), &ledger); err != nil {
			return nil, "", err
		}
	}
	for _, entry := range neutralcatalog.Normalize(value).Entries {
		candidate, err := s.CatalogNativeReview(review, entry)
		if err != nil {
			if !tolerant {
				return nil, "", err
			}
			warning = "Unsafe native declaration; edit the catalog before review"
			continue
		}
		if candidate != nil {
			items = append(items, CatalogReviewItem{Native: candidate})
		}
		if !adopt {
			continue
		}
		ownership, err := s.CatalogAdoptionReview(review, entry)
		if err != nil {
			continue
		}
		current := ownership.Adoption
		matched := false
		for _, record := range ledger.Records {
			if catalogOwnershipMatches(record, current) {
				matched = true
				break
			}
		}
		if !matched {
			items = append(items, CatalogReviewItem{Adoption: &ownership})
		}
	}
	return items, warning, nil
}

func CatalogReviewItemText(item CatalogReviewItem) string {
	if candidate := item.Native; candidate != nil {
		entry, key := candidate.Entry, candidate.Approval.Key
		return fmt.Sprintf("Review %s (%s, %s)\nID: %s\nImplementation: %s\nExact declaration: %q\n", presentation.TerminalSafeText(entry.Name), entry.Kind, key.Shell, entry.ID, key.ImplementationSHA256, candidate.Declaration)
	}
	if candidate := item.Adoption; candidate != nil {
		record := candidate.Adoption
		return fmt.Sprintf("Ownership %s\nID: %s\nSource: %s\nBytes: %d-%d\nFile: %s\nRange: %s\nExact fallback: %q\nCandidate: %s\n", presentation.TerminalSafeText(candidate.Entry.Name), candidate.Entry.ID, presentation.TerminalSafeText(candidate.DisplayPath), record.Start, record.End, record.FileSHA256, record.DefinitionSHA256, record.Definition, presentation.QuotedCatalogValue(candidate.Entry))
	}
	return ""
}

func CatalogReviewBatchText(items []CatalogReviewItem) string {
	approvals, adoptions := 0, 0
	var text strings.Builder
	for _, item := range items {
		if item.Native != nil {
			approvals++
		}
		if item.Adoption != nil {
			adoptions++
		}
	}
	fmt.Fprintf(&text, "Pending exact review: %d native approvals and %d fallback enrollments\n", approvals, adoptions)
	text.WriteString("Decisions remain staged until final save or apply confirmation.\n")
	for _, item := range items {
		text.WriteByte('\n')
		text.WriteString(CatalogReviewItemText(item))
	}
	return text.String()
}

func StageCatalogReviewItems(decisions CatalogLifecycleDecisions, items []CatalogReviewItem) CatalogLifecycleDecisions {
	for _, item := range items {
		if item.Native != nil {
			decisions.Approvals = append(decisions.Approvals, item.Native.Approval)
		}
		if item.Adoption != nil {
			decisions.Adoptions = append(decisions.Adoptions, item.Adoption.Adoption)
		}
	}
	return decisions
}

func catalogOwnershipMatches(record, current catalogstore.Adoption) bool {
	return record.EntryID == current.EntryID && record.Shell == current.Shell && record.Name == current.Name && record.Kind == current.Kind && record.Path == current.Path && record.Start == current.Start && record.End == current.End && record.Definition == current.Definition && record.DefinitionSHA256 == current.DefinitionSHA256 && record.FileSHA256 == current.FileSHA256
}
