package app

import "fmt"

type RevisionRestoreRequest struct {
	ID           string
	CurrentHash  string
	RevisionHash string
}
type RevisionPreview struct {
	Revision     Revision
	Current      []byte
	Previous     []byte
	CurrentHash  string
	RevisionHash string
}

func (svc *Services) loadRevisionList() ([]Revision, error) {
	path, err := svc.editableEntriesPath()
	if err != nil {
		return nil, err
	}
	return svc.listRevisions(path)
}
func (svc *Services) selectedRevision(path, id string) (Revision, error) {
	revisions, err := svc.listRevisions(path)
	if err != nil {
		return Revision{}, err
	}
	for _, revision := range revisions {
		if revision.ID == id {
			return revision, nil
		}
	}
	return Revision{}, fmt.Errorf("revision %q was not found", id)
}
func (svc *Services) loadRevisionPreview(id string) (RevisionPreview, error) {
	path, err := svc.editableEntriesPath()
	if err != nil {
		return RevisionPreview{}, err
	}
	revision, err := svc.selectedRevision(path, id)
	if err != nil {
		return RevisionPreview{}, err
	}
	current, err := readFileLimited(path, AliasFileLimit)
	if err != nil {
		return RevisionPreview{}, fmt.Errorf("read current aliases: %w", err)
	}
	previous, err := readManagedPrivateFile(revision.Path, AliasFileLimit)
	if err != nil {
		return RevisionPreview{}, fmt.Errorf("read selected revision: %w", err)
	}
	return RevisionPreview{Revision: revision, Current: current, Previous: previous, CurrentHash: contentHash(current), RevisionHash: contentHash(previous)}, nil
}
func (svc *Services) restoreReviewedRevision(request RevisionRestoreRequest) error {
	return svc.withMutation(func(session *mutationSession) error {
		path, err := svc.editableEntriesPath()
		if err != nil {
			return err
		}
		revision, err := svc.selectedRevision(path, request.ID)
		if err != nil {
			return err
		}
		current, mode, _, err := readAliasFile(path)
		if err != nil {
			return err
		}
		previous, err := readManagedPrivateFile(revision.Path, AliasFileLimit)
		if err != nil {
			return err
		}
		if contentHash(current) != request.CurrentHash || contentHash(previous) != request.RevisionHash {
			return fmt.Errorf("aliases or this revision changed since preview; reopen the diff before restoring")
		}
		if _, ok := svc.catalogRevisionDirectory(path); ok {
			return svc.restoreCatalogBytesInSession(session, previous)
		}
		return svc.writeAliasFileInSession(session, path, current, previous, mode)
	})
}
