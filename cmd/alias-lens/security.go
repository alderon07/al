package main

import (
	"fmt"
)

func runSecretScan() error {
	findings, err := applicationServices().ScanEntries()
	if err != nil {
		return err
	}
	if len(findings) == 0 {
		cliResult(fmt.Sprintf("No likely secrets found in %s.", applicationServices().AliasDisplayPath()))
		return nil
	}
	cliHeading("Review these lines before syncing. Secret values are hidden:")
	for _, finding := range findings {
		fmt.Printf("  line %d  %s\n", finding.Line, finding.Kind)
	}
	return fmt.Errorf("found %d possible secrets", len(findings))
}
