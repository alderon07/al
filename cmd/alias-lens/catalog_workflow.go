package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
	workflowplan "alias-lens/internal/plan"
)

const catalogReportLimit = 8 << 20

var catalogWorkflowStdout io.Writer = os.Stdout
var catalogWorkflowTerminal = func() bool { return fileIsTerminal(os.Stdout) }

func catalogCommandUsageError() error {
	return fmt.Errorf("usage: al catalog preview|import|shadow|diff|migrate [OPTIONS]")
}

func runCatalogWorkflowCommand(arguments []string) (bool, int, error) {
	if len(arguments) == 0 {
		return false, 0, nil
	}
	switch arguments[0] {
	case "diff":
		code, err := runCatalogDiff(arguments[1:])
		return true, code, err
	case "migrate":
		code, err := runCatalogMigrationCommand(arguments[1:])
		return true, code, err
	default:
		return false, 0, nil
	}
}

func runCatalogMigrationCommand(arguments []string) (int, error) {
	if len(arguments) != 2 || arguments[0] != "--to" || arguments[1] != "2" {
		return 2, fmt.Errorf("usage: al catalog migrate --to 2")
	}
	preview, err := buildCatalogMigrationPlan()
	if err != nil {
		return 1, err
	}
	fmt.Fprint(catalogWorkflowStdout, workflowplan.RenderPlain(preview))
	if len(preview.Actions) == 0 {
		fmt.Fprintln(catalogWorkflowStdout, "Nothing needed to change.")
		return 0, nil
	}
	root := filepath.Dir(localCatalogPath())
	if err := applyPrivatePlan(root, preview, buildCatalogMigrationPlan); err != nil {
		return 1, err
	}
	fmt.Fprintln(catalogWorkflowStdout, "The catalog now uses data format 2. Your entries did not change.")
	return 0, nil
}

func runCatalogDiff(arguments []string) (int, error) {
	jsonOutput := false
	showCode := false
	webOutput := false
	source := ""
	shell := ""
	for index := 0; index < len(arguments); index++ {
		switch arguments[index] {
		case "--json":
			jsonOutput = true
		case "--show-code":
			showCode = true
		case "--web":
			webOutput = true
		case "--from":
			if index+1 >= len(arguments) {
				return 2, fmt.Errorf("--from needs repository or installed")
			}
			index++
			source = arguments[index]
		case "--shell":
			if index+1 >= len(arguments) {
				return 2, fmt.Errorf("--shell needs bash or zsh")
			}
			index++
			shell = arguments[index]
		default:
			return 2, fmt.Errorf("unknown catalog diff option %q", arguments[index])
		}
	}
	if (jsonOutput && showCode) || (webOutput && (jsonOutput || showCode)) {
		return 2, fmt.Errorf("choose one of --json, --show-code, or --web")
	}
	if showCode && !catalogWorkflowTerminal() {
		return 2, fmt.Errorf("--show-code needs an interactive terminal because it can display private command text")
	}
	observed, err := observeConfig()
	if err != nil {
		return 1, fmt.Errorf("Alias Lens could not read its settings: %w", err)
	}
	config := observed.Config
	if shell == "" {
		shell = config.Shell
	}
	if source == "" {
		if config.Repository != "" {
			source = "repository"
		} else {
			source = "installed"
		}
	}
	local, err := readCatalogFile(localCatalogPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 1, fmt.Errorf("no catalog is ready to compare; enter al catalog shadow to inspect your current aliases first")
		}
		return 1, fmt.Errorf("Alias Lens cannot use the local catalog: %w", err)
	}
	var other neutralcatalog.Catalog
	switch source {
	case "repository":
		if config.Repository == "" {
			return 1, fmt.Errorf("no repository is connected; enter al repo to choose one")
		}
		repositoryPath, pathErr := observedCatalogRepositoryPath()
		if pathErr != nil {
			return 1, pathErr
		}
		repositoryCatalog, pathErr := repositoryFilePath(config.Repository, repositoryPath)
		if pathErr != nil {
			return 1, pathErr
		}
		other, err = readCatalogFile(repositoryCatalog)
	case "installed":
		other, err = readInstalledCatalogSnapshot(shell)
	default:
		return 2, fmt.Errorf("--from needs repository or installed")
	}
	if err != nil {
		return 1, err
	}
	reportShell := ""
	if source == "installed" {
		reportShell = shell
	}
	report := neutralcatalog.SemanticDiff(other, local, source, reportShell)
	if webOutput {
		if err := runCatalogDiffWeb(other, local, report); err != nil {
			return 1, err
		}
		return 0, nil
	}
	var output []byte
	if jsonOutput {
		output, err = encodeCatalogJSON(report)
	} else if showCode {
		output = renderCatalogDiffCode(report, other, local)
	} else {
		output = renderCatalogDiffPlain(report, other, local)
	}
	if err != nil {
		return 1, err
	}
	if len(output) > catalogReportLimit {
		return 1, fmt.Errorf("The comparison is too large to show safely")
	}
	if _, err := catalogWorkflowStdout.Write(output); err != nil {
		return 1, fmt.Errorf("show catalog comparison: %w", err)
	}
	if len(report.Diagnostics) > 0 || len(report.Changes) > 0 {
		return 1, nil
	}
	return 0, nil
}

func renderCatalogDiffCode(report neutralcatalog.SemanticDiffReport, before, after neutralcatalog.Catalog) []byte {
	var output strings.Builder
	output.Write(renderCatalogDiffPlain(report, before, after))
	if len(report.Diagnostics) > 0 || len(report.Changes) == 0 {
		return []byte(output.String())
	}
	beforeEntries := map[string]neutralcatalog.Entry{}
	afterEntries := map[string]neutralcatalog.Entry{}
	for _, entry := range before.Entries {
		beforeEntries[entry.ID] = entry
	}
	for _, entry := range after.Entries {
		afterEntries[entry.ID] = entry
	}
	output.WriteString("\nExact changes:\n")
	for _, change := range report.Changes {
		if change.Scope != "entry" {
			continue
		}
		oldEntry, oldOK := beforeEntries[change.EntryID]
		newEntry, newOK := afterEntries[change.EntryID]
		fmt.Fprintf(&output, "\n%s · %s\n", escapePlainText(change.Name), friendlyCatalogField(change.Path))
		if oldOK {
			fmt.Fprintf(&output, "before: %s\n", quotedCatalogValue(catalogFieldValue(oldEntry, change.Path)))
		}
		if newOK {
			fmt.Fprintf(&output, "after:  %s\n", quotedCatalogValue(catalogFieldValue(newEntry, change.Path)))
		}
	}
	return []byte(output.String())
}

func catalogFieldValue(entry neutralcatalog.Entry, path string) any {
	switch path {
	case "entry":
		return entry
	case "name":
		return entry.Name
	case "kind":
		return entry.Kind
	case "description":
		return entry.Description
	case "category":
		return entry.Category
	case "tags":
		return entry.Tags
	case "platforms":
		return entry.Platforms
	case "favorite":
		return entry.Favorite
	case "portable":
		return entry.Portable
	case "native.bash":
		return entry.Native["bash"]
	case "native.zsh":
		return entry.Native["zsh"]
	case "when.profiles_any":
		if entry.When != nil {
			return entry.When.ProfilesAny
		}
	case "when.profiles_none":
		if entry.When != nil {
			return entry.When.ProfilesNone
		}
	case "when.shells":
		if entry.When != nil {
			return entry.When.Shells
		}
	}
	return nil
}

func quotedCatalogValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "(could not display)"
	}
	return escapePlainText(string(encoded))
}

func localCatalogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "alias-lens", "catalog.json")
}

func readCatalogFile(path string) (neutralcatalog.Catalog, error) {
	contents, err := readRegularFile(path, neutralcatalog.MaxDocumentBytes)
	if err != nil {
		return neutralcatalog.Catalog{}, err
	}
	value, diagnostics := neutralcatalog.Decode(contents)
	if len(diagnostics) > 0 {
		return neutralcatalog.Catalog{}, fmt.Errorf("the catalog data format is not valid")
	}
	return value, nil
}

func readInstalledCatalogSnapshot(shell string) (neutralcatalog.Catalog, error) {
	if shell != "bash" && shell != "zsh" {
		return neutralcatalog.Catalog{}, fmt.Errorf("choose Bash or Zsh with --shell")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return neutralcatalog.Catalog{}, err
	}
	statePath := filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json")
	contents, err := readObservedPrivateFile(statePath, 1<<20)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return neutralcatalog.Catalog{}, fmt.Errorf("No saved catalog is installed for %s yet", friendlyShellName(shell))
		}
		return neutralcatalog.Catalog{}, fmt.Errorf("Alias Lens cannot read the installed catalog record")
	}
	var state catalogStateFile
	if err := json.Unmarshal(contents, &state); err != nil || state.SchemaVersion != 1 {
		return neutralcatalog.Catalog{}, fmt.Errorf("Alias Lens cannot use the installed catalog record because its data format is not supported")
	}
	hash := state.InstalledShells[shell].SourceCatalogSHA256
	if len(hash) != 64 || strings.Trim(hash, "0123456789abcdef") != "" {
		return neutralcatalog.Catalog{}, fmt.Errorf("No saved catalog is installed for %s yet", friendlyShellName(shell))
	}
	snapshotPath := filepath.Join(home, ".local", "state", "alias-lens", "catalog-snapshots", hash+".json")
	snapshot, err := readObservedPrivateFile(snapshotPath, neutralcatalog.MaxDocumentBytes)
	if err != nil {
		return neutralcatalog.Catalog{}, fmt.Errorf("Alias Lens cannot read the installed catalog copy")
	}
	value, diagnostics := neutralcatalog.Decode(snapshot)
	if len(diagnostics) > 0 {
		return neutralcatalog.Catalog{}, fmt.Errorf("the installed catalog copy is not valid")
	}
	canonical, diagnostics := neutralcatalog.Encode(value)
	if len(diagnostics) > 0 || hashBytes(canonical) != hash {
		return neutralcatalog.Catalog{}, fmt.Errorf("the installed catalog copy does not match its saved fingerprint")
	}
	return value, nil
}

func observedCatalogRepositoryPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	state, err := inspectCatalogState(filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("Alias Lens cannot read the saved catalog location")
	}
	path := state.CatalogSync.RepositoryPath
	if path == "" {
		path = filepath.Join("alias-lens", "catalog.json")
	}
	path, err = cleanRepositoryRelativePath(path, "catalog repository path")
	if err != nil {
		return "", fmt.Errorf("the saved catalog location is not safe; enter al doctor")
	}
	return path, nil
}

func friendlyShellName(shell string) string {
	if shell == "bash" {
		return "Bash"
	}
	if shell == "zsh" {
		return "Zsh"
	}
	return shell
}

func encodeCatalogJSON(value any) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, fmt.Errorf("prepare JSON output: %w", err)
	}
	return output.Bytes(), nil
}

func renderCatalogDiffPlain(report neutralcatalog.SemanticDiffReport, before, after neutralcatalog.Catalog) []byte {
	var output strings.Builder
	if len(report.Diagnostics) > 0 {
		output.WriteString("Alias Lens could not compare the catalogs. Your files were left unchanged.\n")
		for _, item := range report.Diagnostics {
			fmt.Fprintf(&output, "- %s\n", escapePlainText(item.Message))
		}
		return []byte(output.String())
	}
	if len(report.Changes) == 0 {
		output.WriteString("Your catalog matches the saved copy. Nothing needs to change.\n")
		return []byte(output.String())
	}
	fmt.Fprintf(&output, "Found %d added, %d removed, and %d changed entries. Nothing has been changed yet.\n\n", report.Summary.Added, report.Summary.Deleted, report.Summary.Changed)
	for _, change := range report.Changes {
		name := escapePlainText(change.Name)
		if name == "" {
			name = "catalog"
		}
		switch change.Kind {
		case "added":
			fmt.Fprintf(&output, "+ Add %s\n", name)
		case "deleted":
			fmt.Fprintf(&output, "- Remove %s\n", name)
		default:
			fmt.Fprintf(&output, "~ Change %s: %s\n", name, friendlyCatalogField(change.Path))
		}
		writeCatalogChangeDetails(&output, change, before, after)
	}
	output.WriteString("\nReview these changes before combining them.\n")
	return []byte(output.String())
}

func writeCatalogChangeDetails(output *strings.Builder, change neutralcatalog.SemanticChange, before, after neutralcatalog.Catalog) {
	implementation := change.Path == "portable" || strings.HasPrefix(change.Path, "native.") || change.Path == "entry"
	if implementation {
		if change.BeforeSHA256 != "" {
			fmt.Fprintf(output, "  Before: %d bytes, fingerprint %s\n", change.BeforeBytes, shortFingerprint(change.BeforeSHA256))
		}
		if change.AfterSHA256 != "" {
			fmt.Fprintf(output, "  After: %d bytes, fingerprint %s\n", change.AfterBytes, shortFingerprint(change.AfterSHA256))
		}
		return
	}
	beforeEntries, afterEntries := map[string]neutralcatalog.Entry{}, map[string]neutralcatalog.Entry{}
	for _, entry := range before.Entries {
		beforeEntries[entry.ID] = entry
	}
	for _, entry := range after.Entries {
		afterEntries[entry.ID] = entry
	}
	if entry, ok := beforeEntries[change.EntryID]; ok {
		fmt.Fprintf(output, "  Before: %s\n", quotedCatalogValue(catalogFieldValue(entry, change.Path)))
	}
	if entry, ok := afterEntries[change.EntryID]; ok {
		fmt.Fprintf(output, "  After: %s\n", quotedCatalogValue(catalogFieldValue(entry, change.Path)))
	}
}

func shortFingerprint(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func friendlyCatalogField(path string) string {
	labels := map[string]string{
		"schema_version":     "data format",
		"name":               "name",
		"kind":               "type",
		"description":        "description",
		"category":           "category",
		"tags":               "tags",
		"platforms":          "supported computers",
		"favorite":           "favorite setting",
		"portable":           "portable command",
		"native.bash":        "Bash command",
		"native.zsh":         "Zsh command",
		"when.profiles_any":  "machine profiles",
		"when.profiles_none": "excluded machine profiles",
		"when.shells":        "supported shells",
	}
	if label := labels[path]; label != "" {
		return label
	}
	return "details"
}

func escapePlainText(value string) string {
	return strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return '?'
		}
		return character
	}, value)
}
