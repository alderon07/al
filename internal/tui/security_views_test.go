package tui

import (
	"bytes"
	"github.com/alderon07/al/internal/presentation"
	"strings"
	"testing"
)

func TestTerminalSafeViewsEscapeRepositoryControlledText(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	malicious := "name\x1b]52;c;payload\a\u202e"
	alias := aliasEntry{
		Name:        malicious,
		Command:     "printf '\x1b[2J'",
		Description: "description\x00",
		Category:    "custom\u2066",
		Tags:        []string{"tag\x1b"},
		Issues:      []string{"issue\a"},
	}
	for name, rendered := range map[string]string{
		"card":  renderAlias(alias, true, 80),
		"plain": plainAliasOutput(alias),
	} {
		for _, raw := range []string{"\x1b]52;c;payload\a", "\u202e", "\x1b[2J", "\x00", "\u2066"} {
			if strings.Contains(rendered, raw) {
				t.Fatalf("%s output contains raw control text %q: %q", name, raw, rendered)
			}
		}
		if !strings.Contains(rendered, `\x1b`) {
			t.Fatalf("%s output did not visibly escape control text: %q", name, rendered)
		}
	}

	confirmation := model{runConfirm: &alias, width: 80, height: 24}
	view := confirmation.runConfirmationView(newMainTUIFrame(80, 24), "Alias Lens")
	if strings.Contains(view, "\x1b]52;c;payload\a") || strings.Contains(view, "\x1b[2J") {
		t.Fatalf("confirmation contains raw repository-controlled escape: %q", view)
	}
	if got := presentation.TerminalSafeText("safe\u202evalue"); got != `safe\u202evalue` {
		t.Fatalf("bidirectional formatting was not escaped: %q", got)
	}
}

func plainAliasOutput(alias aliasEntry) string {
	var output bytes.Buffer
	printPlainAliasList(&output, []aliasEntry{alias}, "")
	return output.String()
}
