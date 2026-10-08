package tui

import (
	"alias-lens/internal/app"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

var benchmarkPickerView string

func BenchmarkPickerInitialView(b *testing.B) {
	b.Setenv("NO_COLOR", "")
	b.Setenv("TERM", "xterm-256color")
	previousRenderer := lipgloss.DefaultRenderer()
	renderer := lipgloss.NewRenderer(io.Discard)
	renderer.SetColorProfile(termenv.TrueColor)
	lipgloss.SetDefaultRenderer(renderer)
	b.Cleanup(func() { lipgloss.SetDefaultRenderer(previousRenderer) })
	for _, n := range []int{100, 1000, 10000} {
		for _, width := range []int{60, 140} {
			b.Run(fmt.Sprintf("%d/width-%d", n, width), func(b *testing.B) {
				home, err := filepath.EvalSymlinks(b.TempDir())
				if err != nil {
					b.Fatal(err)
				}
				if err = os.Chmod(home, 0700); err != nil {
					b.Fatal(err)
				}
				services := app.NewServices(app.Dependencies{HomeDir: func() (string, error) { return home, nil }, WorkingDir: func() (string, error) { return home, nil }, Environment: func(key string) string {
					switch key {
					case "ALIAS_LENS_SHELL":
						return "bash"
					case "PATH":
						return "/usr/bin:/bin"
					}
					return ""
				}})
				config := services.Settings.Defaults()
				if err = services.Settings.Save(config); err != nil {
					b.Fatal(err)
				}
				ranking, err := services.CurrentContextRanking()
				if err != nil {
					b.Fatal(err)
				}
				aliases := make([]aliasEntry, n)
				for i := range aliases {
					aliases[i] = aliasEntry{Name: fmt.Sprintf("tool%05d", i), Command: fmt.Sprintf("printf synthetic-%05d", i), Description: "Inspect synthetic repository status", Category: "dev", Type: "alias", Tags: []string{"repository", "status"}, Favorite: i%10 == 0, Usage: i % 26}
				}
				theme := builtInTheme("phosphor")
				applyTheme(theme)
				applyFooterConfig(config.Footer)
				applyAppearanceConfig(config.Appearance)
				current := model{services: services, aliases: aliases, context: ranking, width: width, height: 32, theme: theme, selectMode: true, shortcutProfile: resolvedShortcutProfile(config), shortcutLauncher: launcherLabel(config), aliasMode: aliasModeSearch}
				ordered := current.currentAliases()
				if len(ordered) != n || !ordered[0].Favorite {
					b.Fatal("picker suggestion ranking invalid")
				}
				view := current.View()
				if view == "" || !strings.Contains(view, ordered[0].Name) || strings.Contains(view, "Terminal is too small") {
					b.Fatal("picker view omitted ranked entries")
				}
				b.SetBytes(int64(len(view)))
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					benchmarkPickerView = current.View()
				}
				b.StopTimer()
				b.ReportMetric(float64(n), "entries")
				b.ReportMetric(float64(width), "columns")
			})
		}
	}
}
