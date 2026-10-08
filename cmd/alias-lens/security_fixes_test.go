package main

import (
	"net/http"
	"net/http/httptest"

	"os"
	"path/filepath"

	"strings"

	"testing"
)

func TestMissingAliasFileDoesNotRestoreRemoteBytes(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	repository := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repository, ".bash_aliases"), []byte("touch \"$HOME/remote-code-ran\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := defaultConfig()
	config.Repository = repository
	if err := saveConfig(config); err != nil {
		t.Fatal(err)
	}
	aliasPath := filepath.Join(home, ".bash_aliases")
	if err := ensureAliasFileExists(aliasPath); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(aliasPath)
	if err != nil || len(contents) != 0 {
		t.Fatalf("missing alias file restored remote bytes: %q, %v", contents, err)
	}
}

func TestWebAPIRequiresAuthenticatedLoopbackRequest(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias safe='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	handler, err := newWebHandler("test-session-token", "127.0.0.1:8787")
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		method        string
		host          string
		origin        string
		authorization string
		wantStatus    int
	}{
		{name: "missing token", method: http.MethodGet, host: "127.0.0.1:8787", wantStatus: http.StatusUnauthorized},
		{name: "wrong host", method: http.MethodGet, host: "attacker.example", authorization: "Bearer test-session-token", wantStatus: http.StatusForbidden},
		{name: "cross origin", method: http.MethodGet, host: "127.0.0.1:8787", origin: "https://attacker.example", authorization: "Bearer test-session-token", wantStatus: http.StatusForbidden},
		{name: "wrong method", method: http.MethodPost, host: "127.0.0.1:8787", authorization: "Bearer test-session-token", wantStatus: http.StatusMethodNotAllowed},
		{name: "authenticated", method: http.MethodGet, host: "127.0.0.1:8787", origin: "http://127.0.0.1:8787", authorization: "Bearer test-session-token", wantStatus: http.StatusOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(test.method, "http://127.0.0.1:8787/api/aliases", nil)
			request.Host = test.host
			if test.origin != "" {
				request.Header.Set("Origin", test.origin)
			}
			if test.authorization != "" {
				request.Header.Set("Authorization", test.authorization)
			}
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, request)
			if response.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d; body=%s", response.Code, test.wantStatus, response.Body.String())
			}
			if test.wantStatus == http.StatusOK && !strings.Contains(response.Body.String(), `"name":"safe"`) {
				t.Fatalf("authenticated response did not contain aliases: %s", response.Body.String())
			}
			if response.Header().Get("Cache-Control") != "no-store" {
				t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
			}
		})
	}
}
