package main

import (
	"github.com/alderon07/al/internal/app"

	"github.com/alderon07/al/internal/shell"

	"fmt"
)

func printShellEntry(name string) error {
	definition, err := applicationServices().LoadShellEntry(name)
	if err != nil {
		return err
	}
	fmt.Println(definition)
	return nil
}

func renderLegacyEntryDefinition(adapter app.ShellAdapter, alias Alias) (string, error) {
	return shell.RenderLegacyEntryDefinition(adapter, alias)
}
