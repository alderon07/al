package main

import "fmt"

func addAliasDescriptions() error {
	update, err := applicationServices().UpdateDescriptions()
	if err != nil {
		return err
	}
	if update.Catalog {
		return nil
	}
	if update.Count == 0 {
		fmt.Println("Every alias already has a useful description.")
		return nil
	}
	fmt.Printf("Added or improved descriptions for %d aliases. Backup and private revision saved.\n", update.Count)
	return nil
}
