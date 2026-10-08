package app

import (
	"fmt"
	"github.com/alderon07/al/internal/transaction"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

type Revision struct {
	ID   string
	Path string
	Time time.Time
	Size int64
}

func (svc *Services) saveRevision(aliasPath string, contents []byte) error {
	return svc.withMutation(func(session *mutationSession) error {
		return svc.saveRevisionInSession(session, aliasPath, contents)
	})
}
func (svc *Services) saveRevisionInSession(session *mutationSession, aliasPath string, contents []byte) error {
	directory, e := svc.revisionDirectory(aliasPath)
	if e != nil {
		return e
	}
	id := svc.dependencies.Now().UTC().Format("20060102T150405.000000000Z")
	path := filepath.Join(directory, id+filepath.Base(aliasPath))
	home, e := svc.dependencies.HomeDir()
	if e != nil {
		return e
	}
	if directory == filepath.Join(home, ".local", "share", "alias-lens", "revisions") || directory == filepath.Join(session.stateRoot, "catalog-revisions") {
		return session.WritePrivate(path, contents)
	}
	if e = transaction.EnsureWorkflowDirectory(directory, true); e != nil {
		return e
	}
	operation, e := newOperationID()
	if e != nil {
		return e
	}
	spec := transaction.WorkflowSpec{OperationID: operation, StateRoot: session.stateRoot, PrivateRoots: []string{directory}, Targets: []transaction.WorkflowTarget{{Path: path, Role: transaction.WorkflowPrivate, Planned: contents, Mode: 0o600, RecoveryOrder: 3}}}
	return session.applySpec(spec)
}

func (svc *Services) revisionDirectory(aliasPath string) (string, error) {
	if directory, ok := svc.catalogRevisionDirectory(aliasPath); ok {
		return directory, nil
	}
	configuredPath, err := svc.aliasesPath()
	if err == nil {
		configuredPath, _ = filepath.Abs(configuredPath)
		candidate, _ := filepath.Abs(aliasPath)
		if candidate == configuredPath {
			home, homeErr := svc.dependencies.HomeDir()
			if homeErr != nil {
				return "", homeErr
			}
			return filepath.Join(home, ".local", "share", "alias-lens", "revisions"), nil
		}
	}
	return filepath.Join(filepath.Dir(aliasPath), ".alias-lens-history"), nil
}

func (svc *Services) listRevisions(aliasPath string) ([]Revision, error) {
	directory, err := svc.revisionDirectory(aliasPath)
	if err != nil {
		return nil, err
	}
	if _, e := os.Lstat(directory); os.IsNotExist(e) {
		return nil, nil
	}
	directoryIdentity, e := transaction.InspectWorkflowDirectory(directory)
	if e != nil {
		return nil, e
	}
	if directoryIdentity.Mode != 0o700 {
		return nil, transaction.ErrUnsafePath
	}
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var revisions []Revision
	suffix := filepath.Base(aliasPath)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), suffix) {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		revisions = append(revisions, Revision{ID: strings.TrimSuffix(entry.Name(), suffix), Path: filepath.Join(directory, entry.Name()), Time: info.ModTime(), Size: info.Size()})
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].ID > revisions[j].ID })
	return revisions, nil
}

func (svc *Services) restoreNamedRevision(id string) (Revision, error) {
	path, err := svc.editableEntriesPath()
	if err != nil {
		return Revision{}, err
	}
	revisions, err := svc.listRevisions(path)
	if err != nil {
		return Revision{}, err
	}
	if len(revisions) == 0 {
		return Revision{}, fmt.Errorf("no revisions are available")
	}
	selected := revisions[0]
	if id != "" && id != "latest" {
		found := false
		for _, revision := range revisions {
			if revision.ID == id {
				selected = revision
				found = true
				break
			}
		}
		if !found {
			return Revision{}, fmt.Errorf("revision %q was not found", id)
		}
	}
	if err := svc.restoreRevisionFile(path, selected); err != nil {
		return Revision{}, err
	}
	return selected, nil
}

func (svc *Services) restoreRevisionFile(aliasPath string, revision Revision) error {
	restored, err := readManagedPrivateFile(revision.Path, AliasFileLimit)
	if err != nil {
		return err
	}
	if _, ok := svc.catalogRevisionDirectory(aliasPath); ok {
		return svc.restoreCatalogBytes(restored)
	}
	return svc.withMutation(func(session *mutationSession) error {
		current, mode, _, err := readAliasFile(aliasPath)
		if err != nil {
			return err
		}
		return svc.writeAliasFileInSession(session, aliasPath, current, restored, mode)
	})
}
