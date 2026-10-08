package app

import (
	"strings"
	"testing"
)

func TestShellIntegrationLoadsOnlyInstalledCompletionFile(t *testing.T) {
	for _, test := range []struct {
		name        string
		integration string
		path        string
	}{
		{name: "bash", integration: bashIntegration, path: "$HOME/.config/alias-lens/completion.bash"},
		{name: "zsh", integration: zshIntegration, path: "$HOME/.config/alias-lens/completion.zsh"},
	} {
		t.Run(test.name, func(t *testing.T) {
			if !strings.Contains(test.integration, test.path) || !strings.Contains(test.integration, "-s \"") {
				t.Fatalf("integration does not guard and load %s", test.path)
			}
		})
	}
}
