package main

import "github.com/alderon07/al/internal/presentation"

import (
	"fmt"
	"io"
	"os"
)

func showRepositoryDiff() error {
	return showRepositoryDiffTo(os.Stdout)
}

func showRepositoryDiffTo(output io.Writer) error {
	preview, err := applicationServices().PreviewRepositoryDiff()
	if err != nil {
		return err
	}
	if preview.Equal {
		fmt.Fprintln(output, "The local and tracked alias files match.")
		return nil
	}
	source, target := preview.Source, preview.Target
	localOnly, remoteOnly, conflicts := preview.LocalOnly, preview.RemoteOnly, preview.Conflicts
	if len(localOnly)+len(remoteOnly)+len(conflicts) == 0 {
		fmt.Fprintln(output, "FILES DIFFER outside parsed alias commands")
		fmt.Fprintln(output, "The commands and functions match, but comments, metadata, ordering, whitespace, or unparsed syntax differ.")
		fmt.Fprintf(output, "  local:   %s\n", source)
		fmt.Fprintf(output, "  tracked: %s\n", target)
		fmt.Fprintln(output, "Neither file was changed. Run al diff --tui to review every changed line before choosing what to keep.")
		fmt.Fprintln(output, "Run al sync --push only when the active file contains everything you want to publish.")
		return nil
	}
	fmt.Fprintln(output, "Alias files differ. Neither file was changed.")
	fmt.Fprintf(output, "  active:     %s\n", source)
	fmt.Fprintf(output, "  repository: %s\n", target)
	fmt.Fprintf(output, "Summary: %d local-only, %d repository-only, %d changed.\n\n", len(localOnly), len(remoteOnly), len(conflicts))
	for _, name := range localOnly {
		fmt.Fprintln(output, "LOCAL ONLY ", name)
	}
	for _, name := range remoteOnly {
		fmt.Fprintln(output, "REMOTE ONLY", name)
	}
	for _, conflict := range conflicts {
		fmt.Fprintf(output, "CHANGED    %s\n  local:  %s\n  remote: %s\n", conflict.Name, presentation.TerminalSafeText(conflict.Local), presentation.TerminalSafeText(conflict.Remote))
	}
	fmt.Fprintln(output, "\nThis summary covers parsed alias and function names and commands only. Comments, metadata, ordering, whitespace, or other shell lines may also differ.")
	fmt.Fprintln(output, "Run al diff --tui to review every changed line before replacing the repository copy.")
	writeRepositoryDiffGuidance(output, len(localOnly), len(remoteOnly), len(conflicts))
	return nil
}

func writeRepositoryDiffGuidance(output io.Writer, localOnly, remoteOnly, changed int) {
	fmt.Fprintln(output)
	if changed > 0 {
		fmt.Fprintf(output, "Alias Lens cannot choose between commands for %d changed alias", changed)
		if changed != 1 {
			fmt.Fprint(output, "es")
		}
		fmt.Fprintln(output, ".")
		fmt.Fprintln(output, "Edit the active alias file to keep the command you want for each CHANGED alias.")
		if remoteOnly > 0 {
			fmt.Fprintln(output, "Copy any REMOTE ONLY aliases you want to keep into the active file.")
		}
		fmt.Fprintln(output, "Run al diff again to review the result, then run al sync --push to publish the resolved active file.")
		return
	}
	if localOnly > 0 && remoteOnly > 0 {
		fmt.Fprintln(output, "To keep aliases from both files:")
		fmt.Fprintln(output, "  1. Run al sync --pull to import repository-only aliases into the active file.")
		fmt.Fprintln(output, "  2. Run al diff again. Copy any remaining REMOTE ONLY functions or entries you want into the active file.")
		fmt.Fprintln(output, "  3. Run al sync --push to publish the resolved active file.")
		return
	}
	if remoteOnly > 0 {
		fmt.Fprintln(output, "To bring repository entries into the active file:")
		fmt.Fprintln(output, "  1. Run al sync --pull to import repository-only aliases.")
		fmt.Fprintln(output, "  2. Run al diff again. Copy any remaining REMOTE ONLY functions or entries you want into the active file.")
		fmt.Fprintln(output, "  3. Run al sync --push to publish the resolved active file.")
		return
	}
	fmt.Fprintln(output, "Run al sync --push to publish the local-only aliases from the active file.")
}
