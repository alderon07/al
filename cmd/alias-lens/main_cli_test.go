package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMainFailureExitCodes(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	missingImport := filepath.Join(home, "missing.aliases")

	tests := []struct {
		name string
		args []string
		want int
	}{
		{name: "unsupported shell", args: []string{"alias-lens", "shell-init", "fish"}, want: 1},
		{name: "missing import", args: []string{"alias-lens", "import", missingImport}, want: 1},
		{name: "unknown command", args: []string{"alias-lens", "no-such-command"}, want: 2},
	}

	output, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer output.Close()
	originalArgs, originalStdout, originalStderr := os.Args, os.Stdout, os.Stderr
	t.Cleanup(func() {
		os.Args, os.Stdout, os.Stderr = originalArgs, originalStdout, originalStderr
	})
	os.Stdout, os.Stderr = output, output

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Args = test.args
			if got := runMain(); got != test.want {
				t.Fatalf("runMain() = %d, want %d", got, test.want)
			}
		})
	}
}
