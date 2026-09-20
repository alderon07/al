// Package state derives Alias Lens status from observations supplied by a caller.
// It does not inspect the machine itself.
package state

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

const SchemaVersion = 1

type Mode string

const (
	ModeLegacy  Mode = "legacy"
	ModeCatalog Mode = "catalog"
	ModeMixed   Mode = "mixed"
)

type ConfigState string

const (
	ConfigCurrent           ConfigState = "current"
	ConfigMigrationRequired ConfigState = "migration_required"
	ConfigInvalid           ConfigState = "invalid"
	ConfigUnreadable        ConfigState = "unreadable"
)

type CatalogState string

const (
	CatalogAbsent     CatalogState = "absent"
	CatalogValid      CatalogState = "valid"
	CatalogInvalid    CatalogState = "invalid"
	CatalogUnreadable CatalogState = "unreadable"
)

type ShellState string

const (
	ShellNotInstalled     ShellState = "not_installed"
	ShellCurrent          ShellState = "current"
	ShellRenderRequired   ShellState = "render_required"
	ShellApprovalRequired ShellState = "approval_required"
	ShellIntegrationDrift ShellState = "integration_drift"
	ShellUnreadable       ShellState = "unreadable"
	ShellBlocked          ShellState = "blocked"
)

type SyncState string

const (
	SyncUnconfigured  SyncState = "unconfigured"
	SyncClean         SyncState = "clean"
	SyncLocalChanges  SyncState = "local_changes"
	SyncRemoteChanges SyncState = "remote_changes"
	SyncDiverged      SyncState = "diverged"
	SyncConflict      SyncState = "conflict"
	SyncOffline       SyncState = "offline"
	SyncInvalid       SyncState = "invalid"
)

type RecoveryState string

const (
	RecoveryNotRequired RecoveryState = "not_required"
	RecoveryRequired    RecoveryState = "recovery_required"
	RecoveryBlocked     RecoveryState = "blocked"
)

type SummaryState string

const (
	SummaryHealthy   SummaryState = "healthy"
	SummaryAttention SummaryState = "attention"
	SummaryBlocked   SummaryState = "blocked"
)

// Inputs contains observations gathered by an observational inspector. Resolve
// never opens a path or consults ambient process state.
type Inputs struct {
	Mode     Mode
	Config   ConfigObservation
	Catalog  CatalogObservation
	Shells   []ShellObservation
	Sync     SyncObservation
	Recovery RecoveryObservation
}

type ConfigObservation struct {
	Present              bool
	SchemaVersion        int
	CurrentSchemaVersion int
	Invalid              bool
	Unreadable           bool
}

type CatalogObservation struct {
	Present       bool
	SchemaVersion int
	EntryCount    int
	SHA256        string
	Invalid       bool
	Unreadable    bool
}

// ResolvedSummary contains only the information needed to compare resolved and
// installed state. It must not contain command arguments or implementation text.
type ResolvedSummary struct {
	SHA256           string
	EligibleEntries  int
	PendingApprovals int
}

type InstalledObservation struct {
	GenerationSHA256       string
	ResolvedStateSHA256    string
	RecordedLoaderSHA256   string
	ActiveGenerationSHA256 string
	OnDiskGenerationSHA256 string
	OnDiskLoaderSHA256     string
}

type ShellObservation struct {
	Name       string
	Installed  *InstalledObservation
	Resolved   ResolvedSummary
	Unreadable bool
	Blocked    bool
}

type SyncObservation struct {
	Configured   bool
	BaseSHA256   string
	LocalSHA256  string
	RemoteSHA256 string
	Conflict     bool
	Offline      bool
	Invalid      bool
}

type RecoveryObservation struct {
	Required bool
	Blocked  bool
}

type Report struct {
	SchemaVersion int            `json:"schema_version"`
	Mode          Mode           `json:"mode"`
	Config        ConfigReport   `json:"config"`
	Catalog       CatalogReport  `json:"catalog"`
	Shells        []ShellReport  `json:"shells"`
	Sync          SyncReport     `json:"sync"`
	Recovery      RecoveryReport `json:"recovery"`
	Summary       SummaryReport  `json:"summary"`
}

type ConfigReport struct {
	State         ConfigState `json:"state"`
	SchemaVersion int         `json:"schema_version,omitempty"`
	Action        string      `json:"action"`
}

type CatalogReport struct {
	State         CatalogState `json:"state"`
	SchemaVersion int          `json:"schema_version,omitempty"`
	EntryCount    int          `json:"entry_count,omitempty"`
	CatalogSHA256 string       `json:"catalog_sha256,omitempty"`
}

type ShellReport struct {
	Name             string     `json:"name"`
	State            ShellState `json:"state"`
	Installed        bool       `json:"installed"`
	GenerationSHA256 string     `json:"generation_sha256,omitempty"`
	Action           string     `json:"action"`
}

type SyncReport struct {
	State  SyncState `json:"state"`
	Action string    `json:"action"`
}

type RecoveryReport struct {
	State  RecoveryState `json:"state"`
	Action string        `json:"action"`
}

type SummaryReport struct {
	State       SummaryState `json:"state"`
	ActionCount int          `json:"action_count"`
}

func Resolve(input Inputs) Report {
	report := Report{
		SchemaVersion: SchemaVersion,
		Mode:          resolveMode(input.Mode),
		Config:        resolveConfig(input.Config),
		Catalog:       resolveCatalog(input.Mode, input.Catalog),
		Shells:        make([]ShellReport, len(input.Shells)),
		Sync:          resolveSync(input.Sync),
		Recovery:      resolveRecovery(input.Recovery),
	}

	shells := append([]ShellObservation(nil), input.Shells...)
	sort.SliceStable(shells, func(i, j int) bool { return shells[i].Name < shells[j].Name })
	for index, shell := range shells {
		report.Shells[index] = resolveShell(shell)
	}
	report.Summary = summarize(report)
	return report
}

func resolveMode(value Mode) Mode {
	switch value {
	case ModeLegacy, ModeCatalog, ModeMixed:
		return value
	default:
		return ModeLegacy
	}
}

func resolveConfig(value ConfigObservation) ConfigReport {
	report := ConfigReport{Action: ""}
	switch {
	case value.Unreadable:
		report.State, report.Action = ConfigUnreadable, "al doctor"
	case value.Invalid || value.CurrentSchemaVersion < 1 || value.SchemaVersion > value.CurrentSchemaVersion:
		report.State, report.Action = ConfigInvalid, "al doctor"
	case !value.Present:
		report.State, report.SchemaVersion = ConfigCurrent, value.CurrentSchemaVersion
	case value.SchemaVersion < value.CurrentSchemaVersion:
		report.State, report.SchemaVersion, report.Action = ConfigMigrationRequired, value.SchemaVersion, "al config migrate"
	default:
		report.State, report.SchemaVersion = ConfigCurrent, value.SchemaVersion
	}
	return report
}

func resolveCatalog(mode Mode, value CatalogObservation) CatalogReport {
	report := CatalogReport{}
	if mode == ModeLegacy && !value.Present {
		report.State = CatalogAbsent
		return report
	}
	switch {
	case value.Unreadable:
		report.State = CatalogUnreadable
	case !value.Present:
		report.State = CatalogAbsent
	case value.Invalid:
		report.State = CatalogInvalid
	default:
		report.State = CatalogValid
		report.SchemaVersion = value.SchemaVersion
		report.EntryCount = value.EntryCount
		report.CatalogSHA256 = value.SHA256
	}
	return report
}

func resolveShell(value ShellObservation) ShellReport {
	report := ShellReport{Name: value.Name, Installed: value.Installed != nil, Action: ""}
	if value.Installed != nil {
		report.GenerationSHA256 = value.Installed.GenerationSHA256
	}
	review := "al catalog diff --from installed --shell " + value.Name
	switch {
	case value.Unreadable:
		report.State, report.Action = ShellUnreadable, "al doctor"
	case value.Blocked:
		report.State, report.Action = ShellBlocked, "al doctor"
	case value.Installed == nil:
		report.State, report.Action = ShellNotInstalled, "catalog activation is not available yet"
	case integrationChanged(*value.Installed):
		report.State, report.Action = ShellIntegrationDrift, "al setup "+value.Name
	case value.Resolved.PendingApprovals > 0:
		report.State, report.Action = ShellApprovalRequired, review
	case value.Resolved.SHA256 != value.Installed.ResolvedStateSHA256:
		report.State, report.Action = ShellRenderRequired, review
	default:
		report.State = ShellCurrent
	}
	return report
}

func integrationChanged(value InstalledObservation) bool {
	return value.ActiveGenerationSHA256 != value.GenerationSHA256 ||
		value.OnDiskGenerationSHA256 != value.GenerationSHA256 ||
		value.OnDiskLoaderSHA256 != value.RecordedLoaderSHA256
}

func resolveSync(value SyncObservation) SyncReport {
	report := SyncReport{}
	switch {
	case !value.Configured:
		report.State = SyncUnconfigured
	case value.Invalid:
		report.State, report.Action = SyncInvalid, "al doctor"
	case value.Conflict:
		report.State, report.Action = SyncConflict, "al catalog diff"
	case value.Offline:
		report.State, report.Action = SyncOffline, "retry al sync when the network is available"
	case value.LocalSHA256 == value.RemoteSHA256:
		report.State = SyncClean
	case value.BaseSHA256 == "":
		report.State, report.Action = SyncConflict, "al catalog diff"
	case value.LocalSHA256 != value.BaseSHA256 && value.RemoteSHA256 == value.BaseSHA256:
		report.State, report.Action = SyncLocalChanges, "al catalog diff"
	case value.LocalSHA256 == value.BaseSHA256 && value.RemoteSHA256 != value.BaseSHA256:
		report.State, report.Action = SyncRemoteChanges, "al catalog diff"
	default:
		report.State, report.Action = SyncDiverged, "al catalog diff"
	}
	return report
}

func resolveRecovery(value RecoveryObservation) RecoveryReport {
	switch {
	case value.Blocked:
		return RecoveryReport{State: RecoveryBlocked, Action: "al doctor"}
	case value.Required:
		return RecoveryReport{State: RecoveryRequired, Action: "enter the command again to finish recovery"}
	default:
		return RecoveryReport{State: RecoveryNotRequired, Action: ""}
	}
}

func summarize(report Report) SummaryReport {
	actions, blocked := 0, false
	count := func(action string) {
		if action != "" {
			actions++
		}
	}
	count(report.Config.Action)
	if report.Mode != ModeLegacy && report.Catalog.State != CatalogValid {
		actions++
	}
	for _, shell := range report.Shells {
		count(shell.Action)
		blocked = blocked || shell.State == ShellBlocked || shell.State == ShellUnreadable
	}
	count(report.Sync.Action)
	count(report.Recovery.Action)
	blocked = blocked || report.Config.State == ConfigInvalid || report.Config.State == ConfigUnreadable ||
		report.Catalog.State == CatalogInvalid || report.Catalog.State == CatalogUnreadable ||
		report.Sync.State == SyncInvalid || report.Recovery.State == RecoveryBlocked
	state := SummaryHealthy
	if blocked {
		state = SummaryBlocked
	} else if actions > 0 {
		state = SummaryAttention
	}
	return SummaryReport{State: state, ActionCount: actions}
}

// EncodeJSON produces the canonical public JSON form.
func EncodeJSON(report Report) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(report); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// RenderPlain gives an action for every result that needs attention. It does not
// claim that an already-running shell loaded the installed generation.
func RenderPlain(report Report) string {
	var output strings.Builder
	output.WriteString("Alias Lens status\n")
	fmt.Fprintf(&output, "Setup: %s\n", friendlyMode(report.Mode))
	fmt.Fprintf(&output, "Settings: %s\n", friendlyConfig(report.Config))
	fmt.Fprintf(&output, "Saved aliases: %s\n", friendlyCatalog(report.Mode, report.Catalog))
	for _, shell := range report.Shells {
		fmt.Fprintf(&output, "%s: %s\n", displayShell(shell.Name), friendlyShell(shell))
	}
	fmt.Fprintf(&output, "Repository sync: %s\n", friendlySync(report.Sync))
	fmt.Fprintf(&output, "Recovery: %s\n", friendlyRecovery(report.Recovery))
	return output.String()
}

func friendlyMode(mode Mode) string {
	switch mode {
	case ModeLegacy:
		return "using your shell alias file"
	case ModeCatalog:
		return "using the Alias Lens catalog"
	case ModeMixed:
		return "partly moved to the Alias Lens catalog"
	default:
		return "could not be identified"
	}
}

func friendlyConfig(report ConfigReport) string {
	switch report.State {
	case ConfigCurrent:
		return "ready"
	case ConfigMigrationRequired:
		return "needs an update; enter " + report.Action
	default:
		return "could not be checked; enter " + report.Action
	}
}

func friendlyCatalog(mode Mode, report CatalogReport) string {
	switch report.State {
	case CatalogValid:
		return fmt.Sprintf("ready, %d %s", report.EntryCount, plural(report.EntryCount, "entry", "entries"))
	case CatalogAbsent:
		if mode == ModeLegacy {
			return "not used in legacy mode"
		}
		return "not set up yet"
	case CatalogInvalid:
		return "needs attention; enter al doctor"
	default:
		return "could not be checked; enter al doctor"
	}
}

func friendlyShell(report ShellReport) string {
	prefix := "needs attention"
	switch report.State {
	case ShellCurrent:
		return "installed for new " + displayShell(report.Name) + " shells; current for this machine"
	case ShellNotInstalled:
		prefix = "not installed for new " + displayShell(report.Name) + " shells"
	case ShellUnreadable:
		prefix = "could not be checked"
	case ShellRenderRequired:
		prefix = "the installed definitions need an update"
	case ShellApprovalRequired:
		prefix = "native definitions need approval"
	case ShellIntegrationDrift:
		prefix = "the shell integration changed"
	}
	if report.Action == "" {
		return prefix
	}
	if !strings.HasPrefix(report.Action, "al ") {
		return prefix + "; " + report.Action
	}
	return prefix + "; enter " + report.Action
}

func friendlySync(report SyncReport) string {
	if report.State == SyncClean {
		return "ready; local and repository catalogs match"
	}
	if report.State == SyncUnconfigured {
		return "not configured"
	}
	prefix := "needs attention"
	if report.State == SyncOffline {
		prefix = "could not be checked"
	}
	action := report.Action
	if strings.HasPrefix(action, "al ") {
		action = "enter " + action
	}
	return prefix + "; " + action
}

func friendlyRecovery(report RecoveryReport) string {
	if report.State == RecoveryNotRequired {
		return "not required"
	}
	action := report.Action
	if strings.HasPrefix(action, "al ") {
		action = "enter " + action
	}
	return "needs attention; " + action
}

func displayShell(name string) string {
	if name == "bash" {
		return "Bash"
	}
	if name == "zsh" {
		return "Zsh"
	}
	return safeText(name)
}

func plural(count int, singular, multiple string) string {
	if count == 1 {
		return singular
	}
	return multiple
}

func safeText(value string) string {
	var output strings.Builder
	for _, current := range value {
		if current < 0x20 || current == 0x7f {
			fmt.Fprintf(&output, "\\x%02x", current)
			continue
		}
		output.WriteRune(current)
	}
	return output.String()
}

// ExitCode follows the status contract for a completed inspection.
func ExitCode(report Report) int {
	if report.Summary.State == SummaryHealthy {
		return 0
	}
	return 1
}
