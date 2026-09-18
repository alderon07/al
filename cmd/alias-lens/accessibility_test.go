package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestUnicodeLayoutUsesTerminalCellWidth(t *testing.T) {
	for _, test := range []struct {
		name  string
		value string
		width int
	}{
		{name: "wide", value: "部署-preview-command", width: 10},
		{name: "combining", value: "cafe\u0301-preview", width: 8},
		{name: "emoji", value: "工具🔧-preview", width: 9},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := lipgloss.Width(truncate(test.value, test.width)); got > test.width {
				t.Fatalf("truncate width = %d, want <= %d", got, test.width)
			}
			if got := lipgloss.Width(padRight(test.value, test.width)); got != test.width {
				t.Fatalf("pad width = %d, want %d", got, test.width)
			}
			for _, line := range strings.Split(wrapText(test.value+" next", test.width), "\n") {
				if got := lipgloss.Width(line); got > test.width {
					t.Fatalf("wrap width = %d, want <= %d: %q", got, test.width, line)
				}
			}
		})
	}
}

func TestTruncateCapsZeroWidthInputWork(t *testing.T) {
	value := strings.Repeat("\u200b", 1_000_000)
	got := truncate(value, 10)
	if len(got) > 4100 {
		t.Fatalf("truncated zero-width output retained %d bytes", len(got))
	}
	if width := lipgloss.Width(got); width > 10 {
		t.Fatalf("truncated zero-width width = %d", width)
	}
}

func TestTruncateCapsSingleCombiningCluster(t *testing.T) {
	input := "a" + strings.Repeat("\u0301", 1_000_000)
	result := truncate(input, 20)
	if !strings.HasSuffix(result, "…") {
		t.Fatalf("truncate did not signal clipped combining cluster: %q", result)
	}
	if len(result) > 4100 {
		t.Fatalf("truncate returned %d bytes after applying its work cap", len(result))
	}
}

func TestSmallTerminalShowsRequirement(t *testing.T) {
	view := (model{width: 40, height: 12}).View()
	if !strings.Contains(view, "at least 48 columns and 18 rows") || !strings.Contains(view, "40x12") {
		t.Fatalf("small terminal view = %q", view)
	}
}

func TestDumbTerminalPlainListHasNoControlSequences(t *testing.T) {
	var output bytes.Buffer
	printPlainAliasList(&output, []Alias{{Name: "部署", Command: "kubectl apply"}}, "")
	if strings.Contains(output.String(), "\x1b[") || !strings.Contains(output.String(), "部署\tkubectl apply") {
		t.Fatalf("plain output = %q", output.String())
	}
}

func TestDefaultThemeMeetsContrastThresholds(t *testing.T) {
	theme := defaultTheme()
	if got := themeContrast(theme.Text, theme.Background); got < 4.5 {
		t.Fatalf("default text contrast = %.2f", got)
	}
	if got := themeContrast(theme.Accent, theme.Background); got < 3 {
		t.Fatalf("default control contrast = %.2f", got)
	}
}
