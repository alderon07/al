package main

import "alias-lens/internal/presentation"

import (
	"fmt"
	"io"
	"os"

	"strings"
)

func runTUI() {
	if err := runTUIWithDiff(false); err != nil {
		fmt.Fprintln(os.Stderr, "Alias Lens:", err)
	}
}

// Legacy terminals cannot mark key repeats or releases, so hold off toggling
// the same non-text shortcut until its press stream has gone quiet.

// The empty state above replaces the normal search result list.

func noColorRequested() bool { return os.Getenv("NO_COLOR") != "" }

func terminalIsDumb() bool { return strings.EqualFold(strings.TrimSpace(os.Getenv("TERM")), "dumb") }

func printPlainAliasList(output io.Writer, aliases []Alias, query string) {
	needle := strings.ToLower(strings.TrimSpace(query))
	for _, alias := range aliases {
		if needle != "" && !strings.Contains(strings.ToLower(alias.Name+" "+alias.Command+" "+alias.Description), needle) {
			continue
		}
		fmt.Fprintf(output, "%s\t%s\n", presentation.TerminalSafeText(alias.Name), presentation.TerminalSafeText(alias.Command))
	}
	fmt.Fprintln(output, "Use 'al search QUERY' for non-interactive search.")
}
