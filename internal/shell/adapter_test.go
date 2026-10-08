package shell_test

import (
	"alias-lens/internal/entry"
	"alias-lens/internal/shell"
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestAdapterParsingRenderingAndHistory(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		t.Run(name, func(t *testing.T) {
			adapter, err := shell.New(name)
			if err != nil {
				t.Fatal(err)
			}
			parsed, command, ok := adapter.ParseAliasDefinition(`alias demo='printf synthetic'`)
			if !ok || parsed != "demo" || command != "printf synthetic" {
				t.Fatalf("parse: %q %q %v", parsed, command, ok)
			}
			rendered, err := adapter.RenderEntryDefinition(entry.Alias{Name: parsed, Command: command})
			if err != nil || rendered != `alias demo='printf synthetic'` {
				t.Fatalf("render: %q %v", rendered, err)
			}
			functions := adapter.ParseFunctions("# Synthetic\n# al: tags=one category=test\nfn() {\n printf synthetic\n}\n")
			if len(functions) != 1 || functions[0].Name != "fn" || functions[0].Category != "test" {
				t.Fatalf("functions: %#v", functions)
			}
			var state shell.HistoryState
			if name == "bash" {
				_, _, skip := adapter.HistoryUsageLine("#1700000000", &state)
				command, stamp, skip2 := adapter.HistoryUsageLine("printf synthetic", &state)
				if !skip || skip2 || command != "printf synthetic" || !stamp.Equal(time.Unix(1700000000, 0)) {
					t.Fatal("bash history")
				}
			} else {
				command, stamp, skip := adapter.HistoryUsageLine(": 1700000000:0;printf synthetic", &state)
				if skip || command != "printf synthetic" || !stamp.Equal(time.Unix(1700000000, 0)) {
					t.Fatal("zsh history")
				}
			}
		})
	}
}

func TestStartupPlansReadOnlyPrecedenceAndFreshInputs(t *testing.T) {
	home := t.TempDir()
	bash, _ := shell.New("bash")
	original := []byte("printf synthetic\n")
	for _, name := range []string{".bashrc", ".bash_login", ".profile"} {
		if err := os.WriteFile(filepath.Join(home, name), original, 0600); err != nil {
			t.Fatal(err)
		}
	}
	before := startupTree(t, home)
	plans, err := bash.PlanConfigureStartup(home, "linux", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(plans) != 2 || plans[1].Path != filepath.Join(home, ".bash_login") {
		t.Fatalf("precedence: %#v", plans)
	}
	for _, edit := range plans {
		actual, err := os.ReadFile(edit.Path)
		if err != nil || !bytes.Equal(actual, original) || !bytes.Equal(edit.Before, original) || bytes.Equal(edit.After, original) {
			t.Fatalf("plan mutated: %#v %v", edit, err)
		}
	}
	if !reflect.DeepEqual(before, startupTree(t, home)) {
		t.Fatal("planner changed filesystem manifest")
	}
	if _, err := os.Stat(filepath.Join(home, ".bash_profile")); !os.IsNotExist(err) {
		t.Fatal("created higher precedence startup")
	}
	fresh := []byte("printf fresh\n")
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), fresh, 0600); err != nil {
		t.Fatal(err)
	}
	plans, err = bash.PlanConfigureStartup(home, "linux", "")
	if err != nil || !bytes.Equal(plans[0].Before, fresh) {
		t.Fatalf("stale plan: %#v %v", plans, err)
	}
	zsh, _ := shell.New("zsh")
	for _, directory := range []string{t.TempDir(), t.TempDir()} {
		t.Setenv("ZDOTDIR", directory)
		before := startupTree(t, directory)
		plans, err = zsh.PlanConfigureStartup(home, "linux", "")
		if err != nil || plans[0].Path != filepath.Join(directory, ".zshrc") {
			t.Fatalf("ZDOTDIR: %#v %v", plans, err)
		}
		if !reflect.DeepEqual(before, startupTree(t, directory)) {
			t.Fatal("zsh planner changed filesystem manifest")
		}
		if _, err := os.Stat(plans[0].Path); !os.IsNotExist(err) {
			t.Fatal("planner wrote file")
		}
	}
}

func startupTree(t *testing.T, root string) map[string]string {
	t.Helper()
	result := map[string]string{}
	err := filepath.WalkDir(root, func(path string, object fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := object.Info()
		if err != nil {
			return err
		}
		value := info.Mode().String()
		if info.Mode().IsRegular() {
			contents, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			value += string(contents)
		}
		result[relative] = value
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return result
}
