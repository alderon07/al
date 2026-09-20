package catalogrender

import (
	"strings"
	"testing"

	"alias-lens/internal/catalog"
)

func TestRenderSkipsUnapprovedNativeAndUsesApprovedHash(t *testing.T) {
	nativeText := "git status --short"
	value := catalog.Catalog{SchemaVersion: catalog.SchemaVersion2, Entries: []catalog.Entry{
		{ID: "00000000000000000000000000000001", Name: "gs", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &nativeText}}},
		{ID: "00000000000000000000000000000002", Name: "gl", Kind: "command", Portable: &catalog.Portable{Program: "git", Args: []string{"log", "*", "two words"}, PassArguments: true}},
	}}
	result, diagnostics := Render(value, RenderContext{Shell: "bash", Platform: "linux"})
	if len(diagnostics) != 0 || len(result.PendingApprovals) != 1 {
		t.Fatalf("render diagnostics=%+v result=%+v", diagnostics, result)
	}
	if strings.Contains(string(result.Body), nativeText) {
		t.Fatal("unapproved native text was rendered")
	}
	if !strings.Contains(string(result.Body), `'git' 'log' '*' 'two words' "$@"`) {
		t.Fatalf("portable arguments were not literal: %s", result.Body)
	}
	key := result.PendingApprovals[0]
	result, diagnostics = Render(value, RenderContext{Shell: "bash", Platform: "linux", Approvals: map[NativeApprovalKey]bool{key: true}})
	if len(diagnostics) != 0 || len(result.PendingApprovals) != 0 || !strings.Contains(string(result.Body), nativeText) {
		t.Fatalf("approved native implementation was not rendered: diagnostics=%+v body=%s", diagnostics, result.Body)
	}
}

func TestRenderIsDeterministicAcrossProfileOrder(t *testing.T) {
	value := catalog.Catalog{SchemaVersion: catalog.SchemaVersion2, Entries: []catalog.Entry{{ID: "00000000000000000000000000000001", Name: "work", Kind: "command", Portable: &catalog.Portable{Program: "git", Args: []string{"status"}, PassArguments: false}, When: &catalog.Conditions{ProfilesAny: []string{"work"}}}}}
	left, leftDiagnostics := Render(value, RenderContext{Shell: "zsh", Platform: "linux", Profiles: []string{"work", "laptop"}})
	right, rightDiagnostics := Render(value, RenderContext{Shell: "zsh", Platform: "linux", Profiles: []string{"laptop", "work"}})
	if len(leftDiagnostics) != 0 || len(rightDiagnostics) != 0 || left.ResolvedSHA256 != right.ResolvedSHA256 || string(left.Body) != string(right.Body) {
		t.Fatalf("render changed with profile order: left=%+v right=%+v", left, right)
	}
}
