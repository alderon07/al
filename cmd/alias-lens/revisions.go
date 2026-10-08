package main

import (
	"alias-lens/internal/transaction"
	"fmt"
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

func saveRevision(aliasPath string, contents []byte) error {
	return withMutation(func(session *mutationSession) error { return saveRevisionInSession(session, aliasPath, contents) })
}
func saveRevisionInSession(session *mutationSession, aliasPath string, contents []byte) error {
	directory, e := revisionDirectory(aliasPath)
	if e != nil {
		return e
	}
	id := time.Now().UTC().Format("20060102T150405.000000000Z")
	path := filepath.Join(directory, id+filepath.Base(aliasPath))
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	if directory == filepath.Join(home, ".local", "share", "alias-lens", "revisions") || directory == filepath.Join(session.stateRoot, "catalog-revisions") {
		return session.writePrivate(path, contents)
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

func revisionDirectory(aliasPath string) (string, error) {
	if directory, ok := catalogRevisionDirectory(aliasPath); ok {
		return directory, nil
	}
	configuredPath, err := aliasesPath()
	if err == nil {
		configuredPath, _ = filepath.Abs(configuredPath)
		candidate, _ := filepath.Abs(aliasPath)
		if candidate == configuredPath {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return "", homeErr
			}
			return filepath.Join(home, ".local", "share", "alias-lens", "revisions"), nil
		}
	}
	return filepath.Join(filepath.Dir(aliasPath), ".alias-lens-history"), nil
}

func listRevisions(aliasPath string) ([]Revision, error) {
	directory, err := revisionDirectory(aliasPath)
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

func runRevisionHistory() error {
	path, err := editableEntriesPath()
	if err != nil {
		return err
	}
	revisions, err := listRevisions(path)
	if err != nil {
		return err
	}
	if len(revisions) == 0 {
		fmt.Println("No Alias Lens revisions exist yet.")
		return nil
	}
	for _, revision := range revisions {
		fmt.Printf("%s  %s  %d bytes\n", revision.ID, revision.Time.Local().Format("2006-01-02 15:04:05"), revision.Size)
	}
	return nil
}

func restoreRevision(id string) error {
	path, err := editableEntriesPath()
	if err != nil {
		return err
	}
	revisions, err := listRevisions(path)
	if err != nil {
		return err
	}
	if len(revisions) == 0 {
		return fmt.Errorf("no revisions are available")
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
			return fmt.Errorf("revision %q was not found", id)
		}
	}
	if err := restoreRevisionFile(path, selected); err != nil {
		return err
	}
	fmt.Println("Restored revision", selected.ID)
	return nil
}

func restoreRevisionFile(aliasPath string, revision Revision) error {
	restored, err := readManagedPrivateFile(revision.Path, aliasFileLimit)
	if err != nil {
		return err
	}
	if _, ok := catalogRevisionDirectory(aliasPath); ok {
		return restoreCatalogBytes(restored)
	}
	return withMutation(func(session *mutationSession) error {
		current, mode, _, err := readAliasFile(aliasPath)
		if err != nil {
			return err
		}
		return writeAliasFileInSession(session, aliasPath, current, restored, mode)
	})
}
