package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMainHelpDescribesGuidedCatalogWorkflow(t *testing.T) {
	output, err := os.Create(filepath.Join(t.TempDir(), "help.txt"))
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	originalArgs, originalOutput := os.Args, os.Stdout
	t.Cleanup(func() { os.Args, os.Stdout = originalArgs, originalOutput })
	os.Args = []string{"alias-lens", "help"}
	os.Stdout = output
	if code := runMain(); code != 0 {
		t.Fatalf("help exit %d", code)
	}
	bytes, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	for _, phrase := range []string{"init       Enroll", "al catalog enable", "approval and ownership review before final apply", "al sync --catalog --pull", "al catalog rollback"} {
		if !strings.Contains(string(bytes), phrase) {
			t.Errorf("global help missing %q", phrase)
		}
	}
}

func TestCommandRegistryHelpAndCompletionParity(t *testing.T) {
	rules := completionRules()
	root := map[string]bool{}
	for _, rule := range rules {
		if len(rule.Path) == 0 {
			for _, name := range rule.Values {
				root[name] = true
			}
		}
	}
	for _, spec := range publicCommandSpecs {
		if !root[spec.Name] {
			t.Errorf("%s missing root completion", spec.Name)
		}
		if spec.Usage != commandUsage[spec.Name] || !strings.HasPrefix(spec.Usage, "Usage:") {
			t.Errorf("%s help differs from shared registry", spec.Name)
		}
	}
	valuesFor := func(path string) map[string]bool {
		result := map[string]bool{}
		for _, rule := range rules {
			if strings.Join(rule.Path, " ") == path {
				for _, value := range rule.Values {
					result[value] = true
				}
			}
		}
		return result
	}
	for _, test := range []struct {
		path  string
		flags []string
	}{
		{"init", []string{"--shell", "--catalog-path", "--startup-path", "--apply"}},
		{"catalog enable", []string{"--shell", "--startup-path", "--apply"}},
		{"catalog rollback", []string{"--shell", "--apply"}},
		{"sync", []string{"--catalog", "--pull", "--push", "--apply", "--resolve"}},
	} {
		values := valuesFor(test.path)
		help := commandUsage[strings.Fields(test.path)[0]]
		for _, flag := range test.flags {
			if !values[flag] || !strings.Contains(help, flag) {
				t.Errorf("%s %s differs between help/completion", test.path, flag)
			}
		}
	}
	for _, path := range []string{"plan init", "plan catalog enable", "plan catalog rollback"} {
		if valuesFor(path)["--apply"] {
			t.Errorf("preview %s offers apply", path)
		}
	}
}
