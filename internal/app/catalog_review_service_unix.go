//go:build !windows

package app

import (
	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	"encoding/json"
	"os"
	"path/filepath"
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
	adoptions := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	if _, err := readLifecycleFile(s.catalogAdoptionsPath(), &adoptions); err != nil {
		return result, err
	}
	owned := map[string]bool{}
	for _, record := range adoptions.Records {
		if record.Shell == shell && record.FileSHA256 == hashBytes(review.native) {
			owned[record.EntryID] = true
		}
	}
	result.Entries, err = s.loadAliases()
	if err != nil {
		result.Warning = err.Error()
	}
	for _, entry := range neutralcatalog.Normalize(value).Entries {
		if implementation, exists := entry.Native[shell]; exists {
			key := catalogstore.NativeApproval(entry, shell, implementation)
			if !review.approved[key] {
				candidate, err := s.CatalogNativeReview(review, entry)
				if err != nil {
					result.Warning = "Unsafe native declaration; edit the catalog before review"
					continue
				}
				result.Items = append(result.Items, CatalogReviewItem{Native: candidate})
			}
		}
		if !owned[entry.ID] {
			if candidate, err := s.CatalogAdoptionReview(review, entry); err == nil {
				result.Items = append(result.Items, CatalogReviewItem{Adoption: &candidate})
			}
		}
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
