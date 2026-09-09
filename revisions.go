package main

import (
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
	if len(contents) == 0 {
		return nil
	}
	directory, err := revisionDirectory(aliasPath)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return err
	}
	id := time.Now().UTC().Format("20060102T150405.000000000Z")
	return os.WriteFile(filepath.Join(directory, id+".bash_aliases"), contents, 0o600)
}

func revisionDirectory(aliasPath string) (string, error) {
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
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var revisions []Revision
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".bash_aliases") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		revisions = append(revisions, Revision{ID: strings.TrimSuffix(entry.Name(), ".bash_aliases"), Path: filepath.Join(directory, entry.Name()), Time: info.ModTime(), Size: info.Size()})
	}
	sort.Slice(revisions, func(i, j int) bool { return revisions[i].ID > revisions[j].ID })
	return revisions, nil
}

func runRevisionHistory() error {
	path, err := aliasesPath()
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
	path, err := aliasesPath()
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
	restored, err := os.ReadFile(selected.Path)
	if err != nil {
		return err
	}
	current, mode, _, err := readAliasFile(path)
	if err != nil {
		return err
	}
	if err := writeAliasFile(path, current, restored, mode); err != nil {
		return err
	}
	fmt.Println("Restored revision", selected.ID)
	return nil
}
