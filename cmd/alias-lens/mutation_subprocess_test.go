//go:build !windows

package main

import (
	"github.com/alderon07/al/internal/transaction"

	"crypto/sha256"

	"encoding/json"

	"fmt"
	"os"
	"os/exec"

	"path/filepath"
	"strings"
	"testing"
)

func TestMutationCommandProcess(t *testing.T) {
	if os.Getenv("AL_MUTATION_PROCESS") != "1" {
		return
	}
	var args []string
	if e := json.Unmarshal([]byte(os.Getenv("AL_MUTATION_ARGS")), &args); e != nil {
		t.Fatal(e)
	}
	applicationDependencies.FindTrustedShell = func(shell string) (string, error) { return exec.LookPath(shell) }
	os.Args = append([]string{"alias-lens"}, args...)
	os.Exit(runMain())
}
func mutationInventory(t *testing.T, home string) string {
	t.Helper()
	h := sha256.New()
	e := filepath.WalkDir(home, func(path string, entry os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		info, e := os.Lstat(path)
		if e != nil {
			return e
		}
		rel, e := filepath.Rel(home, path)
		if e != nil {
			return e
		}
		fmt.Fprintf(h, "%q %d %d\n", rel, info.Mode(), info.Size())
		if info.Mode().IsRegular() {
			b, e := os.ReadFile(path)
			if e != nil {
				return e
			}
			h.Write(b)
		} else if info.Mode()&os.ModeSymlink != 0 {
			link, e := os.Readlink(path)
			if e != nil {
				return e
			}
			h.Write([]byte(link))
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}

func TestMutationSubprocessContentionLeavesManagedFilesUnchanged(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("SHELL", "/bin/bash")
	if e := saveConfig(defaultConfig()); e != nil {
		t.Fatal(e)
	}
	for _, name := range []string{".bash_aliases", ".bashrc", "notes.txt"} {
		content := "synthetic\n"
		if name == ".bash_aliases" {
			content = "alias fixture='git status'\n"
		}
		if e := os.WriteFile(filepath.Join(home, name), []byte(content), 0o600); e != nil {
			t.Fatal(e)
		}
	}
	state := filepath.Join(home, ".local", "state", "alias-lens")
	lock, e := transaction.AcquireLock(state, filepath.Join(state, "mutation.lock"))
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	cases := map[string][]string{"legacy edit": {"meta", "fixture", "tags=changed"}, "setup": {"setup", "bash"}, "catalog apply": {"catalog", "import", "--from", "bash"}, "config": {"config", "footer-message", "changed"}, "theme": {"theme", "phosphor"}, "context": {"context", "add", "fixture", "--directory"}, "usage": {"record-use", "fixture"}, "completion": {"completion", "install", "bash"}, "tracked": {"track", filepath.Join(home, "notes.txt")}, "autosync": {"autosync", "disable"}, "watch": {"watch"}}
	for name, args := range cases {
		t.Run(name, func(t *testing.T) {
			before := mutationInventory(t, home)
			data, _ := json.Marshal(args)
			cmd := exec.Command(os.Args[0], "-test.run=^TestMutationCommandProcess$")
			cmd.Env = append(os.Environ(), "AL_MUTATION_PROCESS=1", "AL_MUTATION_ARGS="+string(data))
			cmd.Dir = home
			out, e := cmd.CombinedOutput()
			if e == nil || !strings.Contains(string(out), "lock") {
				t.Fatalf("command did not refuse held mutation lock: %v %s", e, out)
			}
			if after := mutationInventory(t, home); after != before {
				t.Fatal("blocked subprocess changed managed files")
			}
		})
	}
}
