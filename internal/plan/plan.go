// Package plan defines the shared, side-effect-free operation plan contract.
package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

const (
	SchemaVersion        = 1
	MaxInputs            = 256
	MaxActions           = 20000
	MaxApprovals         = 10000
	MaxDiagnostics       = 100
	MaxPublicReportBytes = 8 << 20
)

var ErrStalePlan = errors.New("operation plan changed after preview")

type ActionKind string

const (
	ActionCreate           ActionKind = "create"
	ActionReplace          ActionKind = "replace"
	ActionRemove           ActionKind = "remove"
	ActionEditOwnedRange   ActionKind = "edit_owned_range"
	ActionRemoveOwnedRange ActionKind = "remove_owned_range"
	ActionActivate         ActionKind = "activate"
	ActionDeactivate       ActionKind = "deactivate"
	ActionRecordApproval   ActionKind = "record_approval"
	ActionClone            ActionKind = "clone"
	ActionConfigure        ActionKind = "configure"
)

type Risk string

const (
	RiskLow     Risk = "low"
	RiskReview  Risk = "review"
	RiskBlocked Risk = "blocked"
)

type ApprovalKind string
type ApprovalState string

const (
	ApprovalNativeCode ApprovalKind  = "native_code"
	ApprovalRequired   ApprovalState = "required"
	ApprovalApproved   ApprovalState = "approved"
)

// Identity records the exact object observed during planning. Callers can add
// ordered metadata fields for platform-specific ACL and extended attributes.
type Identity struct {
	FileType   string
	Device     uint64
	Inode      uint64
	LinkTarget string
	LinkCount  uint64
	Owner      uint64
	Group      uint64
	Mode       uint32
	Metadata   []MetadataField
}

type MetadataField struct {
	Name   string
	SHA256 string
}

type InternalInput struct {
	Path     string
	Identity Identity
	Metadata []MetadataField
}

type Input struct {
	Role        string        `json:"role"`
	DisplayPath string        `json:"display_path"`
	SHA256      string        `json:"sha256"`
	Internal    InternalInput `json:"-"`
}

type InverseEdit struct {
	ExpectedSHA256 string
	Offset         int64
	Length         int64
	Bytes          []byte
}

type Target struct {
	Path             string
	ExpectedIdentity Identity
	ExpectedSHA256   string
	PlannedBytes     []byte
	Inverse          *InverseEdit
	Metadata         []MetadataField
	Validation       []ValidationCommand
}

// ValidationCommand is data for the future transactional writer. This package
// never invokes it.
type ValidationCommand struct {
	Program string
	Args    []string
}

type Action struct {
	Sequence      int        `json:"sequence"`
	Kind          ActionKind `json:"kind"`
	TargetRole    string     `json:"target_role"`
	DisplayPath   string     `json:"display_path"`
	Reason        string     `json:"reason"`
	Risk          Risk       `json:"risk"`
	PlannedSHA256 string     `json:"planned_sha256,omitempty"`
	Backup        bool       `json:"backup"`
	Reversible    bool       `json:"reversible"`
	Target        Target     `json:"-"`
}

type Approval struct {
	Kind                 ApprovalKind  `json:"kind"`
	EntryID              string        `json:"entry_id"`
	Shell                string        `json:"shell"`
	ImplementationSHA256 string        `json:"implementation_sha256"`
	State                ApprovalState `json:"state"`
}

type Diagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Blocked bool   `json:"blocked,omitempty"`
}

type Summary struct {
	ActionCount int  `json:"action_count"`
	Blocked     bool `json:"blocked"`
}

type OperationPlan struct {
	SchemaVersion int          `json:"schema_version"`
	Operation     string       `json:"operation"`
	Inputs        []Input      `json:"inputs"`
	Actions       []Action     `json:"actions"`
	Approvals     []Approval   `json:"approvals"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
	Summary       Summary      `json:"summary"`
}

// Build copies and canonicalizes a complete plan. Limits return a blocked plan
// with no partial action or input list.
func Build(operation string, inputs []Input, actions []Action, approvals []Approval, diagnostics []Diagnostic) OperationPlan {
	if detail := exceededLimit(inputs, actions, approvals); detail != "" {
		return blockedPlan(operation, "plan_limit_exceeded", detail)
	}
	if detail := invalidPlan(actions, approvals); detail != "" {
		return blockedPlan(operation, "invalid_plan", detail)
	}
	result := OperationPlan{
		SchemaVersion: SchemaVersion,
		Operation:     operation,
		Inputs:        copyInputs(inputs),
		Actions:       copyActions(actions),
		Approvals:     append([]Approval{}, approvals...),
		Diagnostics:   limitDiagnostics(diagnostics),
	}
	sort.SliceStable(result.Inputs, func(i, j int) bool {
		if result.Inputs[i].Role != result.Inputs[j].Role {
			return result.Inputs[i].Role < result.Inputs[j].Role
		}
		if result.Inputs[i].DisplayPath != result.Inputs[j].DisplayPath {
			return result.Inputs[i].DisplayPath < result.Inputs[j].DisplayPath
		}
		return result.Inputs[i].SHA256 < result.Inputs[j].SHA256
	})
	sort.SliceStable(result.Actions, func(i, j int) bool { return result.Actions[i].Sequence < result.Actions[j].Sequence })
	sort.SliceStable(result.Approvals, func(i, j int) bool {
		if result.Approvals[i].Shell != result.Approvals[j].Shell {
			return result.Approvals[i].Shell < result.Approvals[j].Shell
		}
		if result.Approvals[i].EntryID != result.Approvals[j].EntryID {
			return result.Approvals[i].EntryID < result.Approvals[j].EntryID
		}
		return result.Approvals[i].ImplementationSHA256 < result.Approvals[j].ImplementationSHA256
	})
	normalizeInternal(&result)
	result.Summary.ActionCount = len(result.Actions)
	for _, action := range result.Actions {
		result.Summary.Blocked = result.Summary.Blocked || action.Risk == RiskBlocked
	}
	for _, diagnostic := range result.Diagnostics {
		result.Summary.Blocked = result.Summary.Blocked || diagnostic.Blocked
	}
	if encoded, err := encode(result); err != nil || len(encoded) > MaxPublicReportBytes {
		return blockedPlan(operation, "report_too_large", "the plan report exceeds 8 MiB")
	}
	return result
}

func exceededLimit(inputs []Input, actions []Action, approvals []Approval) string {
	switch {
	case len(inputs) > MaxInputs:
		return fmt.Sprintf("the plan has %d inputs; the limit is %d", len(inputs), MaxInputs)
	case len(actions) > MaxActions:
		return fmt.Sprintf("the plan has %d actions; the limit is %d", len(actions), MaxActions)
	case len(approvals) > MaxApprovals:
		return fmt.Sprintf("the plan has %d approvals; the limit is %d", len(approvals), MaxApprovals)
	default:
		return ""
	}
}

func invalidPlan(actions []Action, approvals []Approval) string {
	seenSequence := make(map[int]bool, len(actions))
	for _, action := range actions {
		if !validActionKind(action.Kind) {
			return fmt.Sprintf("action %d has unsupported kind %q", action.Sequence, action.Kind)
		}
		if action.Risk != RiskLow && action.Risk != RiskReview && action.Risk != RiskBlocked {
			return fmt.Sprintf("action %d has unsupported risk %q", action.Sequence, action.Risk)
		}
		if action.Sequence < 1 || seenSequence[action.Sequence] {
			return "action sequences must be unique positive integers"
		}
		seenSequence[action.Sequence] = true
	}
	for _, approval := range approvals {
		if approval.Kind != ApprovalNativeCode || (approval.State != ApprovalRequired && approval.State != ApprovalApproved) {
			return "the plan contains an unsupported approval kind or state"
		}
	}
	return ""
}

func validActionKind(value ActionKind) bool {
	switch value {
	case ActionCreate, ActionReplace, ActionRemove, ActionEditOwnedRange, ActionRemoveOwnedRange,
		ActionActivate, ActionDeactivate, ActionRecordApproval, ActionClone, ActionConfigure:
		return true
	default:
		return false
	}
}

func blockedPlan(operation, code, message string) OperationPlan {
	if len(operation) > 1024 {
		operation = "invalid"
	}
	return OperationPlan{
		SchemaVersion: SchemaVersion,
		Operation:     operation,
		Inputs:        []Input{},
		Actions:       []Action{},
		Approvals:     []Approval{},
		Diagnostics:   []Diagnostic{{Code: code, Message: message, Blocked: true}},
		Summary:       Summary{Blocked: true},
	}
}

func copyInputs(values []Input) []Input {
	result := append([]Input{}, values...)
	for index := range result {
		result[index].Internal.Identity = copyIdentity(result[index].Internal.Identity)
		result[index].Internal.Metadata = append([]MetadataField(nil), result[index].Internal.Metadata...)
	}
	return result
}

func copyActions(values []Action) []Action {
	result := append([]Action{}, values...)
	for index := range result {
		target := &result[index].Target
		target.ExpectedIdentity = copyIdentity(target.ExpectedIdentity)
		target.PlannedBytes = append([]byte(nil), target.PlannedBytes...)
		target.Metadata = append([]MetadataField(nil), target.Metadata...)
		target.Validation = append([]ValidationCommand(nil), target.Validation...)
		for command := range target.Validation {
			target.Validation[command].Args = append([]string(nil), target.Validation[command].Args...)
		}
		if target.Inverse != nil {
			inverse := *target.Inverse
			inverse.Bytes = append([]byte(nil), target.Inverse.Bytes...)
			target.Inverse = &inverse
		}
	}
	return result
}

func copyIdentity(value Identity) Identity {
	value.Metadata = append([]MetadataField(nil), value.Metadata...)
	return value
}

func normalizeInternal(result *OperationPlan) {
	metadataLess := func(values []MetadataField) {
		sort.SliceStable(values, func(i, j int) bool {
			if values[i].Name != values[j].Name {
				return values[i].Name < values[j].Name
			}
			return values[i].SHA256 < values[j].SHA256
		})
	}
	for index := range result.Inputs {
		metadataLess(result.Inputs[index].Internal.Identity.Metadata)
		metadataLess(result.Inputs[index].Internal.Metadata)
	}
	for index := range result.Actions {
		metadataLess(result.Actions[index].Target.ExpectedIdentity.Metadata)
		metadataLess(result.Actions[index].Target.Metadata)
	}
}

func limitDiagnostics(values []Diagnostic) []Diagnostic {
	if len(values) <= MaxDiagnostics {
		return append([]Diagnostic{}, values...)
	}
	result := append([]Diagnostic(nil), values[:MaxDiagnostics]...)
	omitted := len(values) - MaxDiagnostics
	blocked := false
	for _, diagnostic := range values[MaxDiagnostics:] {
		blocked = blocked || diagnostic.Blocked
	}
	return append(result, Diagnostic{Code: "diagnostics_truncated", Message: fmt.Sprintf("%d additional diagnostics omitted", omitted), Blocked: blocked})
}

// EncodeJSON emits only the public report. Internal paths, bytes, identities,
// inverse edits, metadata, and validation commands are excluded.
func EncodeJSON(value OperationPlan) ([]byte, error) {
	encoded, err := encode(value)
	if err != nil {
		return nil, err
	}
	if len(encoded) > MaxPublicReportBytes {
		return nil, fmt.Errorf("plan report exceeds %d bytes", MaxPublicReportBytes)
	}
	return encoded, nil
}

func encode(value OperationPlan) ([]byte, error) {
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(value); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}

// RenderPlain explains the preview in everyday language. It escapes terminal
// control characters and never includes private action bytes.
func RenderPlain(value OperationPlan) string {
	var output strings.Builder
	if len(value.Actions) == 0 {
		output.WriteString("No changes are needed.\n")
	} else {
		output.WriteString("Planned changes\n")
		for index, action := range value.Actions {
			fmt.Fprintf(&output, "%d. %s %s\n", index+1, friendlyAction(action.Kind), safeText(action.DisplayPath))
			fmt.Fprintf(&output, "   %s · %s\n", friendlyRisk(action.Risk), safeText(action.Reason))
		}
	}
	required := countApprovals(value.Approvals, ApprovalRequired)
	approved := countApprovals(value.Approvals, ApprovalApproved)
	if required > 0 {
		fmt.Fprintf(&output, "Review needed: %d native shell %s need your approval.\n", required, plural(required, "command", "commands"))
	} else if approved > 0 {
		fmt.Fprintf(&output, "Approved native shell commands: %d.\n", approved)
	}
	if value.Summary.Blocked {
		output.WriteString("Alias Lens cannot make these changes yet.\n")
	}
	for _, diagnostic := range value.Diagnostics {
		fmt.Fprintf(&output, "- %s\n", safeText(diagnostic.Message))
	}
	if len(value.Actions) > 0 {
		output.WriteString("Nothing has been changed.\n")
	}
	return output.String()
}

func friendlyAction(kind ActionKind) string {
	switch kind {
	case ActionCreate:
		return "Create"
	case ActionReplace:
		return "Update"
	case ActionRemove:
		return "Remove"
	case ActionEditOwnedRange:
		return "Update the Alias Lens section in"
	case ActionRemoveOwnedRange:
		return "Remove the Alias Lens section from"
	case ActionActivate:
		return "Turn on"
	case ActionDeactivate:
		return "Turn off"
	case ActionRecordApproval:
		return "Save approval for"
	case ActionClone:
		return "Download"
	case ActionConfigure:
		return "Set up"
	default:
		return "Change"
	}
}

func friendlyRisk(risk Risk) string {
	switch risk {
	case RiskLow:
		return "Low risk"
	case RiskReview:
		return "Review this first"
	case RiskBlocked:
		return "Cannot continue"
	default:
		return "Review this first"
	}
}

func plural(count int, singular, multiple string) string {
	if count == 1 {
		return singular
	}
	return multiple
}

func countApprovals(values []Approval, state ApprovalState) int {
	count := 0
	for _, value := range values {
		if value.State == state {
			count++
		}
	}
	return count
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

// CheckFresh compares a preview with a newly rebuilt plan. It includes private
// identities, planned bytes, inverse edits, metadata, approvals, and validation
// commands. Public report equality alone is not sufficient.
func CheckFresh(preview, rebuilt OperationPlan) error {
	if !reflect.DeepEqual(preview, rebuilt) {
		return ErrStalePlan
	}
	return nil
}
