package catalogrender

import (
	"bytes"
	"errors"
	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"testing"
)

func TestRenderV2NativeNameBinding(t *testing.T) {
	body := "printf synthetic"
	entry := catalog.Entry{ID: "11111111111111111111111111111111", Name: "demo", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &body}}}
	value := catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{entry}}
	context := RenderV2Context{Shell: "bash", Platform: "linux", Approvals: map[catalogstore.ApprovalKey]bool{catalogstore.NativeApproval(entry, "bash", entry.Native["bash"]): true}, ValidateNative: func(string, []byte) error { return nil }}
	result, d := RenderV2(value, context)
	if len(d) > 0 || len(result.Entries) != 1 {
		t.Fatal(d)
	}
	value.Entries[0].Name = "renamed"
	result, d = RenderV2(value, context)
	if len(d) > 0 || len(result.Entries) != 0 || len(result.PendingApprovals) != 1 {
		t.Fatal("rename reused approval")
	}
	context.ValidateNative = func(string, []byte) error { return errors.New("synthetic invalid") }
	if _, d := RenderV2(value, context); len(d) == 0 {
		t.Fatal("unsafe declaration accepted")
	}
}
func TestRenderV2ExternalPath(t *testing.T) {
	value := catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{{ID: "11111111111111111111111111111111", Name: "demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"synthetic"}, PassArguments: true}}}}
	context := RenderV2Context{Shell: "bash", Platform: "linux", ResolveExecutable: func(string) (string, error) { return "/synthetic/bin/printf", nil }}
	r, d := RenderV2(value, context)
	if len(d) > 0 || !bytes.Contains(r.Body, []byte("command '/synthetic/bin/printf'")) {
		t.Fatal("path resolution")
	}
	context.ResolveExecutable = func(string) (string, error) { return "relative", nil }
	if _, d := RenderV2(value, context); len(d) == 0 {
		t.Fatal("relative executable accepted")
	}
}
