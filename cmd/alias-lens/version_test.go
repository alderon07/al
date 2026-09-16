package main

import "testing"

func TestResolveVersionPrefersReleaseBuildValue(t *testing.T) {
	if got := resolveVersion("1.2.3", "v9.9.9"); got != "1.2.3" {
		t.Fatalf("resolveVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestResolveVersionUsesGoModuleVersion(t *testing.T) {
	if got := resolveVersion("dev", "v1.2.3"); got != "1.2.3" {
		t.Fatalf("resolveVersion() = %q, want %q", got, "1.2.3")
	}
}

func TestResolveVersionKeepsDevelopmentFallback(t *testing.T) {
	for _, moduleVersion := range []string{"", "(devel)"} {
		if got := resolveVersion("dev", moduleVersion); got != "dev" {
			t.Fatalf("resolveVersion(%q) = %q, want %q", moduleVersion, got, "dev")
		}
	}
}
