//go:build !windows

package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alderon07/al/internal/catalog"
)

func TestCatalogOpaqueNativeFunctionCannotAcquireOwnership(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			home := privateTestHome(t)
			t.Setenv("HOME", home)
			svc := NewServices(Dependencies{ValidateSyntax: func(context.Context, string, []byte) error { return nil }})
			body := "printf synthetic"
			entry := catalog.Entry{ID: strings.Repeat("1", 32), Name: "fixture", Kind: "function", Native: map[string]catalog.NativeImplementation{shell: {FunctionBody: &body}}}
			native := []byte("fixture() { printf '%s' \"$(printf '%s' \"${HOME##*/}\")\"; }\n")
			path := filepath.Join(home, "."+shell+"_aliases")
			if err := os.WriteFile(path, native, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := svc.catalogAdoptionForEntry(entry, shell, path, native); err == nil {
				t.Fatal("opaque function acquired ownership")
			}
			source := catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{entry}}
			writeCatalogFixture(t, svc.localCatalogPath(), source)
			review, err := svc.PrepareCatalogReview(source, shell)
			if err != nil {
				t.Fatal(err)
			}
			items, err := svc.CatalogPendingReview(review, source, true)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range items {
				if item.Adoption != nil {
					t.Fatal("opaque native collision became eligible adoption")
				}
			}
		})
	}
}
