package app

import (
	"strings"
	"testing"
)

func TestDangerousCommandReasonsIdentifyEachRisk(t *testing.T) {
	tests := map[string]string{
		"git push --force-with-lease":  "uses a force option",
		"git clean -fd":                "deletes untracked Git files",
		"rm -rf ./build":               "recursively deletes files",
		"git branch -D old":            "force-deletes a Git branch",
		"sudo systemctl restart nginx": "runs with elevated privileges",
		"docker system prune":          "deletes unused Docker data",
	}
	for command, want := range tests {
		if got := strings.Join(dangerousCommandReasons(command), " "); !strings.Contains(got, want) {
			t.Errorf("%q reason = %q, want %q", command, got, want)
		}
	}
}
