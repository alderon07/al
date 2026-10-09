//go:build !windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
)

func TestCatalogImmutableLoaderChecksAliasDependenciesWithoutValidator(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	validatorCalled := false
	svc := NewServices(Dependencies{ValidateSyntax: func(context.Context, string, []byte) error { validatorCalled = true; return nil }})
	root := svc.catalogGeneratedRoot("bash")
	if err := os.MkdirAll(root, 0700); err != nil {
		t.Fatal(err)
	}
	for path := root; filepath.Base(path) != "alias-lens"; path = filepath.Dir(path) {
		if err := os.Chmod(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	native := []byte("alias other='printf synthetic'\n")
	path := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(path, native, 0600); err != nil {
		t.Fatal(err)
	}
	for _, scenario := range []struct {
		value string
		safe  bool
	}{{"printf --other other", true}, {"other --fixture", false}} {
		entry := catalog.Entry{ID: strings.Repeat("1", 32), Name: "fixture", Kind: "command", Native: map[string]catalog.NativeImplementation{"bash": {AliasValue: &scenario.value}}}
		declaration, err := catalogstore.Declaration(entry, "bash", "")
		if err != nil {
			t.Fatal(err)
		}
		manifest := catalogstore.GenerationManifest{Version: 1, Shell: "bash", Renderer: "bash/v2", Platform: "linux", NativePath: path, NativePolicy: "regular-user", SourceSHA256: hashBytes([]byte("synthetic-source")), NativeInputSHA256: hashBytes(native), Profiles: []string{}, Entries: []catalogstore.GenerationEntry{{Entry: entry, Declaration: declaration}}, IncludedEntryIDs: []string{entry.ID}, Confirmations: []catalogstore.ApprovalKey{catalogstore.NativeApproval(entry, "bash", entry.Native["bash"])}, ExecutableResolutions: []catalogstore.ExecutableResolution{}}
		manifest, encoded, err := catalogstore.BuildGeneration(manifest, []byte(declaration))
		if err != nil {
			t.Fatal(err)
		}
		for suffix, data := range map[string][]byte{".json": encoded, ".sh": []byte(declaration)} {
			if err := os.WriteFile(filepath.Join(root, manifest.ID+suffix), data, 0600); err != nil {
				t.Fatal(err)
			}
		}
		_, err = svc.readCatalogImmutableGeneration(root, path, "bash", manifest.ID, native)
		if scenario.safe && err != nil || !scenario.safe && err == nil {
			t.Fatalf("runtime policy safe=%v err=%v", scenario.safe, err)
		}
		if validatorCalled {
			t.Fatal("read-only generation loader ran syntax validator")
		}
	}
}
