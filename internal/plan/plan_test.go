package plan

import (
	"bytes"
	"errors"
	"go/parser"
	"go/token"
	"math/rand/v2"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

func testPlan() OperationPlan {
	inputs := []Input{
		{Role: "config", DisplayPath: "~/.config/alias-lens/config.json", SHA256: "config", Internal: InternalInput{Path: "/private/home/.config/alias-lens/config.json", Identity: Identity{FileType: "regular", Device: 1, Inode: 2}}},
		{Role: "catalog", DisplayPath: "~/.config/alias-lens/catalog.json", SHA256: "catalog", Internal: InternalInput{Path: "/private/home/.config/alias-lens/catalog.json", Identity: Identity{FileType: "regular", Device: 1, Inode: 3}}},
	}
	actions := []Action{
		{Sequence: 2, Kind: ActionActivate, TargetRole: "active_pointer", DisplayPath: "~/.local/state/alias-lens/active", Reason: "activate the rendered definitions", Risk: RiskReview, PlannedSHA256: "pointer", Backup: true, Reversible: true, Target: Target{Path: "/private/home/.local/state/alias-lens/active", PlannedBytes: []byte("private pointer bytes"), Inverse: &InverseEdit{ExpectedSHA256: "old", Bytes: []byte("private inverse")}}},
		{Sequence: 1, Kind: ActionCreate, TargetRole: "generated_file", DisplayPath: "~/.config/alias-lens/generated/bash/hash.sh", Reason: "the resolved Bash definitions changed", Risk: RiskLow, PlannedSHA256: "generated", Reversible: true, Target: Target{Path: "/private/home/.config/alias-lens/generated/bash/hash.sh", PlannedBytes: []byte("secret implementation")}},
	}
	approvals := []Approval{{Kind: ApprovalNativeCode, EntryID: "b", Shell: "zsh", ImplementationSHA256: "implementation", State: ApprovalRequired}, {Kind: ApprovalNativeCode, EntryID: "a", Shell: "bash", ImplementationSHA256: "other", State: ApprovalApproved}}
	return Build("catalog.enable", inputs, actions, approvals, nil)
}

func TestPlanMatrix(t *testing.T) {
	plan := testPlan()
	if plan.SchemaVersion != 1 || plan.Summary.ActionCount != 2 || plan.Summary.Blocked {
		t.Fatalf("bad summary: %#v", plan.Summary)
	}
	if plan.Actions[0].Sequence != 1 || plan.Inputs[0].Role != "catalog" || plan.Approvals[0].Shell != "bash" {
		t.Fatalf("plan is not canonical: %#v", plan)
	}
	for _, kind := range []ActionKind{ActionCreate, ActionReplace, ActionRemove, ActionEditOwnedRange, ActionRemoveOwnedRange, ActionActivate, ActionDeactivate, ActionRecordApproval, ActionClone, ActionConfigure} {
		value := Build("matrix", nil, []Action{{Sequence: 1, Kind: kind, Risk: RiskLow}}, nil, nil)
		if value.Actions[0].Kind != kind {
			t.Fatalf("lost action kind %q", kind)
		}
	}
	blocked := Build("blocked", nil, []Action{{Sequence: 1, Kind: ActionConfigure, Risk: RiskBlocked}}, nil, nil)
	if !blocked.Summary.Blocked {
		t.Fatal("blocked action did not block plan")
	}
}

func TestPlanDeterministic(t *testing.T) {
	want, err := EncodeJSON(testPlan())
	if err != nil {
		t.Fatal(err)
	}
	base := testPlan()
	for iteration := range 100 {
		inputs := append([]Input(nil), base.Inputs...)
		actions := append([]Action(nil), base.Actions...)
		approvals := append([]Approval(nil), base.Approvals...)
		random := rand.New(rand.NewPCG(uint64(iteration), uint64(iteration+1)))
		random.Shuffle(len(inputs), func(i, j int) { inputs[i], inputs[j] = inputs[j], inputs[i] })
		random.Shuffle(len(actions), func(i, j int) { actions[i], actions[j] = actions[j], actions[i] })
		random.Shuffle(len(approvals), func(i, j int) { approvals[i], approvals[j] = approvals[j], approvals[i] })
		got, err := EncodeJSON(Build(base.Operation, inputs, actions, approvals, nil))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("iteration %d changed report", iteration)
		}
	}
}

func TestPlanReportRedaction(t *testing.T) {
	value := testPlan()
	encoded, err := EncodeJSON(value)
	if err != nil {
		t.Fatal(err)
	}
	plain := RenderPlain(value)
	for _, sentinel := range []string{"/private/home", "secret implementation", "private inverse", "private pointer bytes"} {
		if bytes.Contains(encoded, []byte(sentinel)) || strings.Contains(plain, sentinel) {
			t.Errorf("report leaked %q", sentinel)
		}
	}
	if !bytes.HasSuffix(encoded, []byte{'\n'}) {
		t.Fatal("JSON report needs one final line feed")
	}
	value.Actions[0].Reason = "safe\x1b[2Jtext"
	if rendered := RenderPlain(value); strings.ContainsRune(rendered, '\x1b') || !strings.Contains(rendered, `\x1b`) {
		t.Fatalf("terminal control was not escaped: %q", rendered)
	}
}

func TestPlanJSONGolden(t *testing.T) {
	got, err := EncodeJSON(testPlan())
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile("testdata/plan-v1.json")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("plan JSON changed\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestPlanLimits(t *testing.T) {
	inputs := make([]Input, MaxInputs+1)
	limited := Build("too-many-inputs", inputs, nil, nil, nil)
	if !limited.Summary.Blocked || len(limited.Inputs) != 0 || limited.Diagnostics[0].Code != "plan_limit_exceeded" {
		t.Fatalf("input limit returned partial plan: %#v", limited)
	}
	diagnostics := make([]Diagnostic, MaxDiagnostics+17)
	diagnostics[len(diagnostics)-1].Blocked = true
	value := Build("diagnostics", nil, nil, nil, diagnostics)
	if len(value.Diagnostics) != MaxDiagnostics+1 || value.Diagnostics[MaxDiagnostics].Message != "17 additional diagnostics omitted" {
		t.Fatalf("diagnostic limit = %#v", value.Diagnostics[len(value.Diagnostics)-1])
	}
	if !value.Summary.Blocked || !value.Diagnostics[MaxDiagnostics].Blocked {
		t.Fatal("a truncated blocked diagnostic did not block the plan")
	}
	large := Build("large", nil, []Action{{Sequence: 1, Kind: ActionCreate, DisplayPath: strings.Repeat("x", MaxPublicReportBytes), Risk: RiskLow}}, nil, nil)
	if !large.Summary.Blocked || large.Diagnostics[0].Code != "report_too_large" {
		t.Fatalf("report limit did not block: %#v", large.Summary)
	}
	hugeOperation := Build(strings.Repeat("x", MaxPublicReportBytes), nil, nil, nil, nil)
	encoded, err := EncodeJSON(hugeOperation)
	if err != nil || len(encoded) > MaxPublicReportBytes || !hugeOperation.Summary.Blocked {
		t.Fatalf("operation limit returned an invalid report: bytes=%d err=%v", len(encoded), err)
	}
}

func TestRebuildRejectsStaleInputs(t *testing.T) {
	preview := testPlan()
	if err := CheckFresh(preview, testPlan()); err != nil {
		t.Fatalf("unchanged plan is stale: %v", err)
	}
	tests := []struct {
		name string
		edit func(*OperationPlan)
	}{
		{"input hash", func(p *OperationPlan) { p.Inputs[0].SHA256 = "changed" }},
		{"input identity", func(p *OperationPlan) { p.Inputs[0].Internal.Identity.Inode++ }},
		{"remote object metadata", func(p *OperationPlan) {
			p.Inputs[0].Internal.Metadata = []MetadataField{{Name: "remote_revision", SHA256: "changed"}}
		}},
		{"approval", func(p *OperationPlan) { p.Approvals[0].State = ApprovalRequired }},
		{"target identity", func(p *OperationPlan) { p.Actions[0].Target.ExpectedIdentity.Inode++ }},
		{"target bytes", func(p *OperationPlan) { p.Actions[0].Target.PlannedBytes[0] = 'X' }},
		{"inverse", func(p *OperationPlan) { p.Actions[1].Target.Inverse.Bytes[0] = 'X' }},
		{"metadata", func(p *OperationPlan) {
			p.Actions[0].Target.Metadata = []MetadataField{{Name: "mode", SHA256: "changed"}}
		}},
		{"active pointer", func(p *OperationPlan) { p.Actions[1].PlannedSHA256 = "changed" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			rebuilt := testPlan()
			test.edit(&rebuilt)
			if err := CheckFresh(preview, rebuilt); !errors.Is(err, ErrStalePlan) {
				t.Fatalf("CheckFresh error = %v", err)
			}
		})
	}
}

func TestBuildCopiesPrivateData(t *testing.T) {
	value := testPlan()
	rebuilt := Build(value.Operation, value.Inputs, value.Actions, value.Approvals, value.Diagnostics)
	rebuilt.Actions[0].Target.PlannedBytes[0] = 'X'
	if reflect.DeepEqual(rebuilt, value) || value.Actions[0].Target.PlannedBytes[0] == 'X' {
		t.Fatal("Build retained caller-owned private bytes")
	}
}

func TestPlanImportAllowlist(t *testing.T) {
	allowed := map[string]bool{"bytes": true, "encoding/json": true, "errors": true, "fmt": true, "reflect": true, "sort": true, "strings": true}
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info os.FileInfo) bool { return !strings.HasSuffix(info.Name(), "_test.go") }, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, pkg := range packages {
		for _, file := range pkg.Files {
			for _, spec := range file.Imports {
				path, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatal(err)
				}
				if !allowed[path] {
					t.Errorf("production import %q can cause side effects", path)
				}
			}
		}
	}
}
