package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "alias-lens/internal/catalog"
)

func TestCatalogDiffWebKeepsCodeBehindAuthenticatedDetailRequest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	before := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: "0123456789abcdef0123456789abcdef", Name: "deploy", Kind: "command", Native: map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: stringPointer("private-command-before")}}}}}
	after := before
	after.Entries = append([]neutralcatalog.Entry(nil), before.Entries...)
	after.Entries[0].Native = map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: stringPointer("private-command-after")}}
	report := neutralcatalog.SemanticDiff(before, after, "repository", "")
	view, err := newCatalogDiffWebView(before, after, report)
	if err != nil {
		t.Fatal(err)
	}
	handler, err := newWebHandlerWithCatalogDiff("secret-token", "127.0.0.1:8787", view)
	if err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787/api/catalog-diff", nil)
	request.Host = "127.0.0.1:8787"
	request.Header.Set("Authorization", "Bearer secret-token")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("summary status = %d; body=%s", response.Code, response.Body.String())
	}
	if strings.Contains(response.Body.String(), "private-command") {
		t.Fatalf("summary disclosed command text: %s", response.Body.String())
	}

	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787/api/catalog-diff/details", nil)
	request.Host = "127.0.0.1:8787"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated detail status = %d, want %d", response.Code, http.StatusUnauthorized)
	}

	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787/api/catalog-diff/details", nil)
	request.Host = "127.0.0.1:8787"
	request.Header.Set("Authorization", "Bearer secret-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "private-command-after") {
		t.Fatalf("authenticated details were unavailable: status=%d body=%s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}

	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787/api/aliases", nil)
	request.Host = "127.0.0.1:8787"
	request.Header.Set("Authorization", "Bearer secret-token")
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("catalog viewer exposed alias API: status=%d body=%s", response.Code, response.Body.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".bash_aliases")); !os.IsNotExist(err) {
		t.Fatalf("catalog viewer created an alias file: %v", err)
	}

	request = httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787/diff.html", nil)
	request.Host = "127.0.0.1:8787"
	response = httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusOK || strings.Contains(response.Body.String(), "private-command") {
		t.Fatalf("initial page disclosed exact command text: status=%d body=%s", response.Code, response.Body.String())
	}
}

func TestCatalogDiffDetailUsesInternalFilename(t *testing.T) {
	entry := neutralcatalog.Entry{ID: "0123456789abcdef0123456789abcdef", Name: "private.name", Kind: "command", Native: map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: stringPointer("true")}}}
	after := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{entry}}
	report := neutralcatalog.SemanticDiffReport{SchemaVersion: 1, Source: "repository", Changes: []neutralcatalog.SemanticChange{{Scope: "entry", EntryID: entry.ID, Name: entry.Name, Kind: "added", Path: "entry"}}, Summary: neutralcatalog.SemanticSummary{Added: 1}}
	view, err := newCatalogDiffWebView(neutralcatalog.Catalog{SchemaVersion: 2}, after, report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(view.detail), `"filename":"private.name.json"`) {
		t.Fatalf("detail used the display name as a filename: %s", view.detail)
	}
	if !strings.Contains(string(view.detail), `"filename":"entry-0123456789abcdef0123456789abcdef.json"`) {
		t.Fatalf("detail filename is not stable: %s", view.detail)
	}
}

func TestCatalogDiffWebRejectsHostOriginAndUnrelatedPages(t *testing.T) {
	view := &catalogDiffWebView{summary: []byte(`{"changes":[]}`), detail: []byte(`{"files":[]}`)}
	handler, err := newWebHandlerWithCatalogDiff("secret-token", "127.0.0.1:8787", view)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		name   string
		host   string
		origin string
		path   string
		want   int
	}{
		{name: "host", host: "example.com", path: "/diff.html", want: http.StatusForbidden},
		{name: "origin", host: "127.0.0.1:8787", origin: "https://example.com", path: "/diff.html", want: http.StatusForbidden},
		{name: "legacy page", host: "127.0.0.1:8787", path: "/index.html", want: http.StatusNotFound},
	} {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787"+test.path, nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.want {
				t.Fatalf("status = %d, want %d", response.Code, test.want)
			}
		})
	}
}

func TestCatalogDiffWebRequestsDoNotChangeUserFiles(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	paths := []string{
		filepath.Join(home, ".config", "alias-lens", "catalog.json"),
		filepath.Join(home, ".config", "alias-lens", "config.json"),
		filepath.Join(home, ".local", "state", "alias-lens", "catalog-state.json"),
		filepath.Join(home, "dotfiles", "alias-lens", "catalog.json"),
		filepath.Join(home, ".bash_aliases"),
		filepath.Join(home, ".bashrc"),
	}
	before := map[string][]byte{}
	for index, path := range paths {
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		contents := []byte(fmt.Sprintf("private-canary-%d\n", index))
		if err := os.WriteFile(path, contents, 0o600); err != nil {
			t.Fatal(err)
		}
		before[path] = contents
	}
	view := &catalogDiffWebView{summary: []byte(`{"changes":[]}`), detail: []byte(`{"files":[]}`)}
	handler, err := newWebHandlerWithCatalogDiff("secret-token", "127.0.0.1:8787", view)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/", "/diff.css", "/api/catalog-diff", "/api/catalog-diff/details"} {
		request := httptest.NewRequest(http.MethodGet, "http://127.0.0.1:8787"+path, nil)
		request.Host = "127.0.0.1:8787"
		request.Header.Set("Authorization", "Bearer secret-token")
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", path, response.Code)
		}
	}
	for path, wanted := range before {
		contents, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(contents, wanted) {
			t.Fatalf("viewer changed %s: %q, %v", path, contents, err)
		}
	}
}

func stringPointer(value string) *string { return &value }
