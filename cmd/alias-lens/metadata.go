package main

import "github.com/alderon07/al/internal/entry"

import (
	"github.com/alderon07/al/internal/app"

	"fmt"

	"strings"
)

func runMetadataCommand(arguments []string) error {
	if len(arguments) < 2 {
		return fmt.Errorf("usage: al meta ALIAS key=value [key=value]")
	}
	aliases, err := applicationServices().Entries()
	if err != nil {
		return err
	}
	metadata := app.EntryMetadata{}
	foundEntry := false
	for _, alias := range aliases {
		if alias.Name == arguments[0] {
			metadata = entry.MetadataForAlias(alias)
			foundEntry = true
			break
		}
	}
	if !foundEntry {
		return fmt.Errorf("entry %q was not found", arguments[0])
	}
	for _, argument := range arguments[1:] {
		key, value, found := strings.Cut(argument, "=")
		if !found {
			return fmt.Errorf("metadata must use key=value")
		}
		switch strings.ToLower(key) {
		case "tags", "collections":
			metadata.Tags = entry.SplitMetadataValues(value)
		case "platforms":
			metadata.Platforms = entry.SplitMetadataValues(value)
		case "favorite":
			metadata.Favorite = strings.EqualFold(value, "true") || value == "1" || strings.EqualFold(value, "yes")
		case "category":
			metadata.Category = entry.NormalizeCategory(value)
			if metadata.Category != "" && metadataInvalid(metadata) {
				return fmt.Errorf("category may only use letters, numbers, dot, dash, and underscore")
			}
		default:
			return fmt.Errorf("unknown metadata field %q", key)
		}
	}
	return applicationServices().EditMetadata(arguments[0], metadata)
}

func metadataInvalid(metadata app.EntryMetadata) bool {
	_, err := applicationServices().ValidateMetadata(metadata)
	return err != nil
}
