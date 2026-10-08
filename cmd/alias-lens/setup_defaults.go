package main

import (
	"bufio"
	"fmt"
	"io"
	"strings"
)

func offerDefaultAliases(path string, input io.Reader, output io.Writer) error {
	missing, err := applicationServices().DefaultAliasOffer(path)
	if err != nil {
		return err
	}
	if len(missing) == 0 {
		fmt.Fprintln(output, "No optional developer aliases are missing.")
		return nil
	}

	fmt.Fprintln(output)
	fmt.Fprintf(output, "Alias Lens can add these optional shortcuts to %s:\n", applicationServices().AliasDisplayPath())
	for _, candidate := range missing {
		fmt.Fprintf(output, "  %-3s  %-43s %s\n", candidate.Name, candidate.Command, candidate.Description)
	}
	fmt.Fprintln(output)
	fmt.Fprintln(output, "Nothing above runs during setup. The aliases only run when you type their names later.")
	fmt.Fprintln(output, "Existing alias names and equivalent commands will be skipped. Before writing, Alias Lens creates a backup and private revision.")
	fmt.Fprint(output, "Add these optional aliases? [y/N] ")

	answer, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && err != io.EOF {
		return err
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	if answer != "y" && answer != "yes" {
		fmt.Fprintln(output, "Skipped optional developer aliases.")
		return nil
	}

	added, err := applicationServices().AddDefaultAliasesToFile(path)
	if err != nil {
		return err
	}
	fmt.Fprintf(output, "Added %d optional developer aliases. Start a new %s shell before using them.\n", added, applicationServices().ActiveShellAdapter().Name())
	return nil
}
