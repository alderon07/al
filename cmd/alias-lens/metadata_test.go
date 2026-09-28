package main

import (
	"path/filepath"
	"testing"
)

func TestPlatformNameUsesCatalogIdentifiers(t *testing.T) {
	noEnvironment := func(string) string { return "" }
	tests := []struct {
		name   string
		goos   string
		getenv func(string) string
		want   string
	}{
		{name: "macOS", goos: "darwin", getenv: noEnvironment, want: "macos"},
		{name: "Linux", goos: "linux", getenv: noEnvironment, want: "linux"},
		{name: "Windows", goos: "windows", getenv: noEnvironment, want: "windows"},
		{name: "WSL distribution", goos: "linux", getenv: func(name string) string {
			if name == "WSL_DISTRO_NAME" {
				return "Ubuntu"
			}
			return ""
		}, want: "wsl"},
		{name: "WSL interop", goos: "linux", getenv: func(name string) string {
			if name == "WSL_INTEROP" {
				return "/run/WSL/1_interop"
			}
			return ""
		}, want: "wsl"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := platformName(test.goos, test.getenv); got != test.want {
				t.Fatalf("platformName(%q) = %q, want %q", test.goos, got, test.want)
			}
		})
	}
}

func TestCanonicalMacOSPlatformKeepsBashLoginStartup(t *testing.T) {
	home := t.TempDir()
	paths, err := (bashShellAdapter{}).StartupPaths(home, "macos")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 || paths[0] != filepath.Join(home, ".bashrc") || paths[1] != filepath.Join(home, ".bash_profile") {
		t.Fatalf("macOS Bash startup paths = %#v", paths)
	}
}
