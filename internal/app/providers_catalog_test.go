package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"github.com/alderon07/al/internal/catalog"
	"io"
	"net/http"
	"strings"
	"testing"
)

type catalogProviderRoundTrip func(*http.Request) (*http.Response, error)

func (f catalogProviderRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func syntheticCatalogProviderBytes(t *testing.T) []byte {
	t.Helper()
	data, d := catalog.Encode(catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{{ID: "11111111111111111111111111111111", Name: "demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"synthetic"}}}}})
	if len(d) > 0 {
		t.Fatal(d)
	}
	return data
}
func TestGitHubImmutableCatalogPreview(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("SSH_AUTH_SOCK", "")
	data := syntheticCatalogProviderBytes(t)
	revision := strings.Repeat("a", 40)
	t.Setenv("GH_TOKEN", "synthetic-credential")
	original := catalogProviderTransport
	t.Cleanup(func() { catalogProviderTransport = original })
	calls := 0
	catalogProviderTransport = catalogProviderRoundTrip(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.Host != "api.github.com" || r.Header.Get("Authorization") != "Bearer synthetic-credential" {
			t.Fatal("credential authority mismatch")
		}
		var response any
		switch {
		case strings.Contains(r.URL.Path, "/contents/"):
			if r.URL.Query().Get("ref") != revision {
				t.Fatal("mutable file ref")
			}
			response = map[string]any{"type": "file", "path": "catalog.json", "sha": gitBlobSHA(data), "size": len(data), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)}
		case strings.Contains(r.URL.Path, "/commits/"):
			response = map[string]any{"sha": revision}
		default:
			response = map[string]any{"default_branch": "main", "permissions": map[string]bool{"push": true}}
		}
		encoded, _ := json.Marshal(response)
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(encoded))), Header: http.Header{}}, nil
	})
	preview, err := DefaultServices().discoverRemoteCatalog(context.Background(), DefaultServices().defaultConfig(), "https://github.com/synthetic/repository.git", "catalog.json")
	if err != nil || preview.Revision != revision || preview.Blob != gitBlobSHA(data) || calls != 3 {
		t.Fatal("immutable preview", err)
	}
}

func TestProviderPreviewKeepsAppCatalogValidation(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	t.Setenv("GH_TOKEN", "synthetic")
	t.Setenv("SSH_AUTH_SOCK", "")
	for _, invalid := range []string{"schema", "secret"} {
		t.Run(invalid, func(t *testing.T) {
			data := []byte(`{"schema_version":900,"entries":[]}`)
			if invalid == "secret" {
				var problems []catalog.Diagnostic
				data, problems = catalog.Encode(catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{{ID: "11111111111111111111111111111111", Name: "demo", Kind: "command", Portable: &catalog.Portable{Program: "printf", Args: []string{"api_key=synthetic"}}}}})
				if len(problems) > 0 {
					t.Fatal(problems)
				}
			}
			original := catalogProviderTransport
			t.Cleanup(func() { catalogProviderTransport = original })
			catalogProviderTransport = catalogProviderRoundTrip(func(request *http.Request) (*http.Response, error) {
				var response any
				switch {
				case strings.Contains(request.URL.Path, "/contents/"):
					response = map[string]any{"type": "file", "path": "catalog.json", "sha": gitBlobSHA(data), "size": len(data), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)}
				case strings.Contains(request.URL.Path, "/commits/"):
					response = map[string]string{"sha": strings.Repeat("a", 40)}
				default:
					response = map[string]any{"default_branch": "main", "permissions": map[string]bool{"push": true}}
				}
				encoded, _ := json.Marshal(response)
				return &http.Response{StatusCode: 200, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(string(encoded)))}, nil
			})
			preview, err := DefaultServices().discoverRemoteCatalog(context.Background(), DefaultServices().defaultConfig(), "github:synthetic/repo", "catalog.json")
			if err == nil || len(preview.Bytes) > 0 {
				t.Fatal("invalid app catalog returned usable provider bytes")
			}
			if invalid == "secret" && !strings.Contains(err.Error(), "likely secrets") {
				t.Fatal("whole-catalog secret scan was skipped", err)
			}
		})
	}
}
