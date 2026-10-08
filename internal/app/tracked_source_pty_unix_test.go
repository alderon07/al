//go:build linux || darwin

package app

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestCompiledUntrackInvalidatedSourcePTY(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "alias-lens")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/alias-lens")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	source := filepath.Join(home, "notes")
	if err := os.WriteFile(source, []byte("ordinary data"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings := DefaultServices().internalSettings()
	if err := settings.Track(source, nil); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(source, 0o666); err != nil {
		t.Fatal(err)
	}
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	session := startShellPTY(t, bash, []string{"--noprofile", "--norc", "-i"}, []string{"HOME=" + home, "PATH=" + os.Getenv("PATH"), "TERM=xterm-256color", "HISTFILE=/dev/null", "PS1=" + ptyPrompt})
	offset := session.mark()
	session.write(quoteShadow(binary) + " untrack " + quoteShadow(source) + "; printf 'UNTRACK_RESULT=%s\\n' \"$?\"\n")
	session.waitFor(offset, "UNTRACK_RESULT=0")
	config, err := settings.loadConfig()
	if err != nil || len(config.TrackedFiles) != 0 {
		t.Fatalf("compiled untrack failed to remove source: %v", err)
	}
}
