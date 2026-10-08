package catalogstore

import (
	"alias-lens/internal/catalog"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testID = "11111111111111111111111111111111"

func testKey() ApprovalKey {
	return ApprovalKey{EntryID: testID, Shell: "bash", Name: "demo", Kind: "command", ImplementationSHA256: Hash([]byte("synthetic")), Renderer: "bash/v2"}
}
func TestCodecs(t *testing.T) {
	h := Hash(nil)
	startup := StartupRecord{Path: "/synthetic/startup", SHA256: h}
	key := testKey()
	values := []any{ApprovalFile{Version: 2, Records: []ApprovalRecord{{Key: key, ApprovedAt: "2026-10-03T12:00:00Z"}}}, InstalledFile{Version: 1, Records: []InstalledState{{Shell: "bash", GenerationID: h, Renderer: "bash/v2", StartupRecords: []StartupRecord{startup}}}}, AdoptionsFile{Version: 1, Records: []Adoption{{EntryID: testID, Shell: "bash", Name: "demo", Kind: "command", Path: "/synthetic/native", Start: 0, End: 4, Definition: "demo", DefinitionSHA256: Hash([]byte("demo")), FileSHA256: h, OriginalPath: "/synthetic/original", OriginalSHA256: h}}}, RollbackRecord{Version: 1, ID: h, Shell: "bash", CreatedAt: "2026-10-03T12:00:00Z", NativePath: "/synthetic/native", OriginalPath: "/synthetic/original", OriginalSHA256: h, StartupRecords: []StartupRecord{startup}}}
	for _, v := range values {
		data, err := Encode(v)
		if err != nil {
			t.Fatal(err)
		}
		var out any
		switch v.(type) {
		case ApprovalFile:
			out = &ApprovalFile{}
		case InstalledFile:
			out = &InstalledFile{}
		case AdoptionsFile:
			out = &AdoptionsFile{}
		case RollbackRecord:
			out = &RollbackRecord{}
		}
		if err := Decode(data, out); err != nil {
			t.Fatal(err)
		}
		for _, bad := range [][]byte{append(append([]byte{}, data...), []byte("{}")...), bytes.Replace(data, []byte(`"version":`), []byte(`"unknown": 0, "version":`), 1), bytes.Replace(data, []byte(`"version":`), []byte(`"version": 1,"version":`), 1), bytes.Replace(data, []byte(`"version": 1`), []byte(`"version": 99`), 1)} {
			if bytes.Equal(bad, data) {
				continue
			}
			if Decode(bad, out) == nil {
				t.Fatal("accepted invalid codec input")
			}
		}
	}
	var file ApprovalFile
	for _, bad := range []string{`{"version":2,"records":null}`, `{"version":2}`, `{"version":1,"records":[]}`, `{"version":2,"records":[],"version":2}`} {
		if Decode([]byte(bad), &file) == nil {
			t.Fatal("accepted malformed state")
		}
	}
	k := testKey()
	k.Name = "unsafe;"
	if Validate(k) == nil {
		t.Fatal("unsafe name")
	}
	for _, when := range []string{"yesterday", "2026-10-03T12:00:00+00:00", "2026-10-03T12:00:00.000Z"} {
		if Validate(ApprovalFile{Version: 2, Records: []ApprovalRecord{{Key: key, ApprovedAt: when}}}) == nil {
			t.Fatal("invalid timestamp")
		}
	}
	if Validate(ApprovalFile{Version: 2, Records: []ApprovalRecord{{Key: key, ApprovedAt: "2026-10-03T12:00:00Z"}, {Key: key, ApprovedAt: "2026-10-03T12:00:00Z"}}}) == nil {
		t.Fatal("duplicate record")
	}
}
func syntheticGeneration(t *testing.T) (GenerationManifest, []byte, []byte) {
	t.Helper()
	entry := catalog.Entry{ID: testID, Name: "demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"synthetic"}, PassArguments: true}}
	declaration, err := Declaration(entry, "bash", "/synthetic/bin/printf")
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(declaration)
	m := GenerationManifest{Version: 1, Shell: "bash", Renderer: "bash/v2", Platform: "linux", NativePath: "/synthetic/native", NativePolicy: "regular-user", SourceSHA256: Hash([]byte("source")), NativeInputSHA256: Hash([]byte("native")), Profiles: []string{}, Entries: []GenerationEntry{{Entry: entry, Declaration: declaration}}, IncludedEntryIDs: []string{testID}, Confirmations: []ApprovalKey{}, ExecutableResolutions: []ExecutableResolution{{EntryID: testID, Program: "printf", Path: "/synthetic/bin/printf"}}}
	m, data, err := BuildGeneration(m, body)
	if err != nil {
		t.Fatal(err)
	}
	return m, data, body
}
func TestGenerationIntegrity(t *testing.T) {
	m, data, body := syntheticGeneration(t)
	pointer, _ := EncodePointer(m.ID)
	if _, err := VerifyGeneration(data, body, pointer, []byte("native"), nil); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{pointer[:64], append(pointer, '\n'), []byte(strings.TrimSpace(string(pointer)) + "\r\n"), bytes.ToUpper(pointer)} {
		if _, err := DecodePointer(bad); err == nil {
			t.Fatal("invalid pointer accepted")
		}
	}
	if _, err := VerifyGeneration(data, append(body, '\n'), pointer, []byte("native"), nil); err == nil {
		t.Fatal("output corruption accepted")
	}
	if _, err := VerifyGeneration(data, body, pointer, []byte("changed"), nil); err == nil {
		t.Fatal("native drift accepted")
	}
	changed := m
	changed.NativePath = "/synthetic/other"
	changed, _, err := BuildGeneration(changed, body)
	if err != nil || changed.ID == m.ID {
		t.Fatal("identity must bind native path")
	}
	if Hash(nil) != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Fatal("hash known answer")
	}
	var decoded GenerationManifest
	if Decode(data, &decoded) != nil {
		t.Fatal("manifest decode")
	}
	encoded, _ := Encode(decoded)
	if !bytes.Equal(data, encoded) {
		t.Fatal("canonical roundtrip")
	}
}
func TestNativeGenerationRequiresValidator(t *testing.T) {
	m, _, _ := syntheticGeneration(t)
	native := "printf synthetic"
	entry := catalog.Entry{ID: testID, Name: "demo", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &native}}}
	declaration, _ := Declaration(entry, "bash", "")
	m.Entries = []GenerationEntry{{Entry: entry, Declaration: declaration}}
	m.Confirmations = []ApprovalKey{NativeApproval(entry, "bash", entry.Native["bash"])}
	m.ExecutableResolutions = []ExecutableResolution{}
	m, data, err := BuildGeneration(m, []byte(declaration))
	if err != nil {
		t.Fatal(err)
	}
	pointer, _ := EncodePointer(m.ID)
	if _, err := VerifyGeneration(data, []byte(declaration), pointer, []byte("native"), nil); err == nil {
		t.Fatal("validator bypass")
	}
	called := false
	if _, err := VerifyGeneration(data, []byte(declaration), pointer, []byte("native"), func(shell string, b []byte) error {
		called = true
		if shell != "bash" || string(b) != declaration {
			t.Fatal("wrong declaration")
		}
		return nil
	}); err != nil || !called {
		t.Fatal("native validation")
	}
}
func TestPrivateRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state")
	data, _ := Encode(ApprovalFile{Version: 2, Records: []ApprovalRecord{}})
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	var state ApprovalFile
	if err := ReadPrivate(path, &state); err != nil {
		t.Fatal(err)
	}
	os.Chmod(path, 0644)
	if ReadPrivate(path, &state) == nil {
		t.Fatal("public read")
	}
	os.Chmod(path, 0600)
	link := path + "-link"
	os.Symlink(path, link)
	if ReadPrivate(link, &state) == nil {
		t.Fatal("symlink read")
	}
}

func TestExactFieldCases(t *testing.T) {
	key := testKey()
	data, _ := Encode(ApprovalFile{Version: 2, Records: []ApprovalRecord{{Key: key, ApprovedAt: "2026-10-03T12:00:00Z"}}})
	for _, field := range []string{"version", "records", "key", "approved_at", "entry_id", "shell", "name", "kind", "implementation_sha256", "renderer"} {
		needle := []byte(`"` + field + `":`)
		wrong := []byte(`"` + strings.ToUpper(field[:1]) + field[1:] + `": true, "` + field + `":`)
		malformed := bytes.Replace(data, needle, wrong, 1)
		var out ApprovalFile
		if Decode(malformed, &out) == nil {
			t.Fatalf("case variant accepted: %s", field)
		}
		wrong = append([]byte(`"`+strings.ToUpper(field[:1])+field[1:]+`":`), []byte{}...)
		malformed = bytes.Replace(data, needle, wrong, 1)
		if Decode(malformed, &out) == nil {
			t.Fatalf("uppercase accepted: %s", field)
		}
	}
}
func TestPrivateLinkedAndChangingRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "state")
	data, _ := Encode(ApprovalFile{Version: 2, Records: []ApprovalRecord{}})
	os.WriteFile(path, data, 0600)
	var out ApprovalFile
	hard := filepath.Join(dir, "hard")
	if err := os.Link(path, hard); err != nil {
		t.Fatal(err)
	}
	if ReadPrivate(path, &out) == nil {
		t.Fatal("hardlink accepted")
	}
	os.Remove(hard)
	link := filepath.Join(t.TempDir(), "parent")
	os.Symlink(dir, link)
	if ReadPrivate(filepath.Join(link, "state"), &out) == nil {
		t.Fatal("linked parent accepted")
	}
	if readPrivate(path, &out, func() {
		replacement := path + ".new"
		os.WriteFile(replacement, data, 0600)
		os.Rename(replacement, path)
	}) == nil {
		t.Fatal("replaced file accepted")
	}
	if readPrivate(path, &out, func() { os.WriteFile(path, append(data, ' '), 0600) }) == nil {
		t.Fatal("changed file accepted")
	}
}
func TestGenerationIdentityInputs(t *testing.T) {
	baseline, _, body := syntheticGeneration(t)
	changes := []func(*GenerationManifest){func(m *GenerationManifest) { m.SourceSHA256 = Hash([]byte("different source")) }, func(m *GenerationManifest) { m.NativeInputSHA256 = Hash([]byte("different native")) }, func(m *GenerationManifest) { m.Platform = "macos" }, func(m *GenerationManifest) { m.Profiles = []string{"work"} }, func(m *GenerationManifest) { m.NativePath = "/synthetic/other" }, func(m *GenerationManifest) { m.Entries[0].Entry.Description = "different description" }, func(m *GenerationManifest) { m.ExecutableResolutions[0].Path = "/synthetic/other/printf" }}
	for i, change := range changes {
		var m GenerationManifest
		encoded, _ := Encode(baseline)
		Decode(encoded, &m)
		change(&m)
		declaration, _ := Declaration(m.Entries[0].Entry, m.Shell, m.ExecutableResolutions[0].Path)
		m.Entries[0].Declaration = declaration
		altered, _, err := BuildGeneration(m, []byte(declaration))
		if err != nil || altered.ID == baseline.ID {
			t.Fatalf("identity omitted input %d: %v", i, err)
		}
	}
	if baseline.ID != "dcaa3f772b19c8a10a64038ac8e57206f2def6fe865546355eb6c6061fa3292d" || Hash(body) != "7490f9a84039d6d1542dc0b755a99965c9b080d03ea41beb77d5b57b8293efd6" || HashMustEncode(baseline) != "ba23e78fd128685b551194c62e3bc9ccf3c48575df6276c50f2e40f40fc0b7e4" {
		t.Fatal("canonical generation vector changed")
	}
	m := baseline
	m.Profiles = []string{"work", "home"}
	a, _, err := BuildGeneration(m, body)
	if err != nil {
		t.Fatal(err)
	}
	m.Profiles = []string{"home", "work"}
	b, _, err := BuildGeneration(m, body)
	if err != nil || a.ID != b.ID {
		t.Fatal("profile order affected identity")
	}
}
func HashMustEncode(m GenerationManifest) string { data, _ := Encode(m); return Hash(data) }
func TestBoundedListsAndRoutes(t *testing.T) {
	m, _, _ := syntheticGeneration(t)
	m.Profiles = make([]string, 33)
	if Validate(m) == nil {
		t.Fatal("profile limit")
	}
	m.Profiles = []string{"INVALID"}
	if Validate(m) == nil {
		t.Fatal("profile syntax")
	}
	m.Profiles = []string{}
	m.Confirmations = make([]ApprovalKey, 10001)
	if Validate(m) == nil {
		t.Fatal("confirmation limit")
	}
	m.Confirmations = []ApprovalKey{}
	m.ExecutableResolutions = make([]ExecutableResolution, 10001)
	if Validate(m) == nil {
		t.Fatal("resolution limit")
	}
	if startupValid(StartupRecord{Path: "/synthetic/startup", SHA256: Hash(nil), Route: "../unsafe"}) {
		t.Fatal("invalid startup route")
	}
	file := InstalledFile{Version: 1, Records: []InstalledState{{Shell: "bash", Renderer: "bash/v2", GenerationID: Hash(nil), StartupRecords: make([]StartupRecord, 65)}}}
	if Validate(file) == nil {
		t.Fatal("startup limit")
	}
}
func TestGenerationConfirmationIdentity(t *testing.T) {
	m, _, _ := syntheticGeneration(t)
	m.Confirmations = []ApprovalKey{testKey()}
	base := identity(m)
	mutations := []func(*ApprovalKey){func(k *ApprovalKey) { k.EntryID = "22222222222222222222222222222222" }, func(k *ApprovalKey) { k.Shell = "zsh" }, func(k *ApprovalKey) { k.Name = "other" }, func(k *ApprovalKey) { k.Kind = "function" }, func(k *ApprovalKey) { k.ImplementationSHA256 = Hash([]byte("other")) }, func(k *ApprovalKey) { k.Renderer = "zsh/v2" }}
	for i, mutate := range mutations {
		key := testKey()
		mutate(&key)
		other := m
		other.Confirmations = []ApprovalKey{key}
		if identity(other) == base {
			t.Fatalf("approval identity omitted field %d", i)
		}
	}
}
