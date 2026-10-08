package main

import "fmt"

func runRevisionHistory() error {
	revisions, err := applicationServices().RevisionList()
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
	restored, err := applicationServices().RestoreRevision(id)
	if err != nil {
		return err
	}
	fmt.Println("Restored revision", restored.ID)
	return nil
}
