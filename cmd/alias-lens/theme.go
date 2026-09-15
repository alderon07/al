package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Theme struct {
	Preset     string `json:"preset"`
	Name       string `json:"name"`
	Accent     string `json:"accent"`
	Secondary  string `json:"secondary"`
	Git        string `json:"git"`
	Docker     string `json:"docker"`
	Files      string `json:"files"`
	Dev        string `json:"dev"`
	Text       string `json:"text"`
	Muted      string `json:"muted"`
	Background string `json:"background"`
	Panel      string `json:"panel"`
	Selected   string `json:"selected"`
	Border     string `json:"border"`
}

func defaultTheme() Theme {
	return builtInTheme("phosphor")
}

var themeOrder = []string{
	"phosphor", "absolutely", "ayu", "catppuccin", "codex", "darcula", "dracula",
	"everforest", "github", "gruvbox", "linear", "lobster", "material", "matrix",
	"monokai", "night-owl", "nord", "notion", "one", "oscurange", "raycast",
	"rose-pine", "sentry", "solarized", "temple", "tokyo-night", "vercel",
	"vscode-plus", "xcode",
}

// Codex palettes use the app's dark chrome seed for accent, text, background,
// and semantic colors. The remaining fields come from each shipped editor
// theme. completeTheme derives contrast only when that theme omits a value.
var builtInThemes = map[string]Theme{
	"phosphor": {
		Preset: "phosphor", Name: "Phosphor", Accent: "#B8FF6A", Secondary: "#72DDF7",
		Git: "#FF8A65", Docker: "#72DDF7", Files: "#C6A0F6", Dev: "#FFD166",
		Text: "#F3F6EE", Muted: "#8EA6A2", Background: "#0D1211", Panel: "#151C1A",
		Selected: "#21302A", Border: "#33443F",
	},
	"absolutely": {
		Preset: "absolutely", Name: "Absolutely", Accent: "#CC7D5E", Secondary: "#00C853",
		Git: "#FF5F38", Docker: "#00C853", Files: "#CC7D5E", Dev: "#CC7D5E",
		Text: "#F9F9F7", Muted: "#F9F9F7", Background: "#2D2D2B", Panel: "#373735",
		Border: "#CC7D5E",
	},
	"ayu": {
		Preset: "ayu", Name: "Ayu", Accent: "#E6B450", Secondary: "#93E2C8",
		Git: "#F06B73", Docker: "#4FBFFF", Files: "#D0A1FF", Dev: "#FDB04C",
		Text: "#BFBDB6", Muted: "#5A6378", Background: "#10141C", Panel: "#0D1017",
		Selected: "#475266", Border: "#1B1F29",
	},
	"catppuccin": {
		Preset: "catppuccin", Name: "Catppuccin", Accent: "#CBA6F7", Secondary: "#94E2D5",
		Git: "#FAB387", Docker: "#89B4FA", Files: "#F5C2E7", Dev: "#F9E2AF",
		Text: "#CDD6F4", Muted: "#9399B2", Background: "#1E1E2E", Panel: "#181825",
		Selected: "#313244", Border: "#585B70",
	},
	"codex": {
		Preset: "codex", Name: "Codex", Accent: "#0169CC", Secondary: "#00A240",
		Git: "#F67576", Docker: "#00A240", Files: "#B06DFF", Dev: "#FA994C",
		Text: "#FCFCFC", Muted: "#8F8F8F", Background: "#111111", Panel: "#131313",
		Border: "#0169CC",
	},
	"darcula": {
		Preset: "darcula", Name: "Darcula", Accent: "#FFC66D", Secondary: "#6897BB",
		Git: "#CC7832", Docker: "#6A8759", Files: "#9876AA", Dev: "#BBB529",
		Text: "#A9B7C6", Muted: "#808080", Background: "#2B2B2B", Panel: "#313335",
		Selected: "#214283", Border: "#555555",
	},
	"dracula": {
		Preset: "dracula", Name: "Dracula", Accent: "#FF79C6", Secondary: "#8BE9FD",
		Git: "#FF5555", Docker: "#BD93F9", Files: "#FF79C6", Dev: "#F1FA8C",
		Text: "#F8F8F2", Muted: "#6272A4", Background: "#282A36", Panel: "#21222C",
		Selected: "#44475A", Border: "#BD93F9",
	},
	"everforest": {
		Preset: "everforest", Name: "Everforest", Accent: "#A7C080", Secondary: "#83C092",
		Git: "#E69875", Docker: "#7FBBB3", Files: "#D699B6", Dev: "#DBBC7F",
		Text: "#D3C6AA", Muted: "#859289", Background: "#2D353B", Panel: "#2D353B",
		Selected: "#475258",
	},
	"github": {
		Preset: "github", Name: "GitHub", Accent: "#1F6FEB", Secondary: "#39C5CF",
		Git: "#FF7B72", Docker: "#58A6FF", Files: "#BC8CFF", Dev: "#D29922",
		Text: "#E6EDF3", Muted: "#8B949E", Background: "#0D1117", Panel: "#010409",
		Selected: "#6E7681", Border: "#30363D",
	},
	"gruvbox": {
		Preset: "gruvbox", Name: "Gruvbox", Accent: "#458588", Secondary: "#689D6A",
		Git: "#CC241D", Docker: "#458588", Files: "#B16286", Dev: "#D79921",
		Text: "#EBDBB2", Muted: "#A89984", Background: "#282828", Panel: "#282828",
		Selected: "#3C3836", Border: "#3C3836",
	},
	"linear": {
		Preset: "linear", Name: "Linear", Accent: "#606ACC", Secondary: "#69C967",
		Git: "#FF7E78", Docker: "#69C967", Files: "#C2A1FF", Dev: "#606ACC",
		Text: "#E3E4E6", Muted: "#9CA0AA", Background: "#0F0F11", Panel: "#080A0F",
		Border: "#5E6AD2",
	},
	"lobster": {
		Preset: "lobster", Name: "Lobster", Accent: "#FF5C5C", Secondary: "#22C55E",
		Git: "#FF5C5C", Docker: "#22C55E", Files: "#3B82F6", Dev: "#F59E0B",
		Text: "#E4E4E7", Muted: "#A1A1AA", Background: "#111827", Panel: "#111827",
		Border: "#FF5C5C",
	},
	"material": {
		Preset: "material", Name: "Material", Accent: "#80CBC4", Secondary: "#89DDFF",
		Git: "#F07178", Docker: "#82AAFF", Files: "#C792EA", Dev: "#FFCB6B",
		Text: "#EEFFFF", Muted: "#676767", Background: "#212121", Panel: "#212121",
	},
	"matrix": {
		Preset: "matrix", Name: "Matrix", Accent: "#1EFF5A", Secondary: "#1EFF5A",
		Git: "#FA423E", Docker: "#1EFF5A", Files: "#1EFF5A", Dev: "#7DFF95",
		Text: "#B8FFCA", Muted: "#9BFFB8", Background: "#040805", Panel: "#020402",
		Border: "#1EFF5A",
	},
	"monokai": {
		Preset: "monokai", Name: "Monokai", Accent: "#99947C", Secondary: "#56ADBC",
		Git: "#C4265E", Docker: "#6A7EC8", Files: "#8C6BC8", Dev: "#B3B42B",
		Text: "#F8F8F2", Muted: "#90908A", Background: "#272822", Panel: "#1E1F1C",
		Selected: "#75715E", Border: "#414339",
	},
	"night-owl": {
		Preset: "night-owl", Name: "Night Owl", Accent: "#44596B", Secondary: "#21C7A8",
		Git: "#EF5350", Docker: "#82AAFF", Files: "#C792EA", Dev: "#C5E478",
		Text: "#D6DEEB", Muted: "#89A4BB", Background: "#011627", Panel: "#011627",
		Selected: "#234D70", Border: "#5F7E97",
	},
	"nord": {
		Preset: "nord", Name: "Nord", Accent: "#88C0D0", Secondary: "#88C0D0",
		Git: "#D08770", Docker: "#81A1C1", Files: "#B48EAD", Dev: "#EBCB8B",
		Text: "#D8DEE9", Muted: "#8FBCBB", Background: "#2E3440", Panel: "#2E3440",
		Selected: "#434C5E", Border: "#3B4252",
	},
	"notion": {
		Preset: "notion", Name: "Notion", Accent: "#3183D8", Secondary: "#4EC9B0",
		Git: "#FA423E", Docker: "#4EC9B0", Files: "#569CD6", Dev: "#DCDCAA",
		Text: "#D9D9D8", Muted: "#9B9B9A", Background: "#191919", Panel: "#151515",
		Border: "#3183D8",
	},
	"one": {
		Preset: "one", Name: "One", Accent: "#4D78CC", Secondary: "#42B3C2",
		Git: "#E05561", Docker: "#4AA5F0", Files: "#C162DE", Dev: "#D18F52",
		Text: "#ABB2BF", Muted: "#7F848E", Background: "#282C34", Panel: "#21252B",
		Selected: "#2C313A", Border: "#3E4452",
	},
	"oscurange": {
		Preset: "oscurange", Name: "Oscurange", Accent: "#F9B98C", Secondary: "#40C977",
		Git: "#FA423E", Docker: "#40C977", Files: "#479FFA", Dev: "#F9B98C",
		Text: "#E6E6E6", Muted: "#94949A", Background: "#0B0B0F", Panel: "#0B0B0F",
		Border: "#F9B98C",
	},
	"raycast": {
		Preset: "raycast", Name: "Raycast", Accent: "#FF6363", Secondary: "#56C2FF",
		Git: "#FF6363", Docker: "#56C2FF", Files: "#CF2F98", Dev: "#FFC531",
		Text: "#FEFEFE", Muted: "#A3A3A3", Background: "#101010", Panel: "#101010",
		Border: "#5C5C5C",
	},
	"rose-pine": {
		Preset: "rose-pine", Name: "Rose Pine", Accent: "#EA9A97", Secondary: "#9CCFD8",
		Git: "#EA9A97", Docker: "#9CCFD8", Files: "#C4A7E7", Dev: "#F6C177",
		Text: "#E0DEF4", Muted: "#908CAA", Background: "#232136", Panel: "#232136",
		Selected: "#393552", Border: "#817C9C",
	},
	"sentry": {
		Preset: "sentry", Name: "Sentry", Accent: "#7055F6", Secondary: "#8EE6D7",
		Git: "#FA423E", Docker: "#8EE6D7", Files: "#C39BFF", Dev: "#F4C46A",
		Text: "#E6DFF9", Muted: "#A69CBF", Background: "#2D2935", Panel: "#26222D",
		Border: "#7055F6",
	},
	"solarized": {
		Preset: "solarized", Name: "Solarized", Accent: "#D30102", Secondary: "#2AA198",
		Git: "#DC322F", Docker: "#268BD2", Files: "#D33682", Dev: "#B58900",
		Text: "#839496", Muted: "#586E75", Background: "#002B36", Panel: "#00212B",
		Selected: "#005A6F", Border: "#2B2B4A",
	},
	"temple": {
		Preset: "temple", Name: "Temple", Accent: "#E4F222", Secondary: "#40C977",
		Git: "#FA423E", Docker: "#40C977", Files: "#E4F222", Dev: "#E4F222",
		Text: "#C7E6DA", Muted: "#84A99A", Background: "#02120C", Panel: "#1D2D0F",
		Border: "#E4F222",
	},
	"tokyo-night": {
		Preset: "tokyo-night", Name: "Tokyo Night", Accent: "#3D59A1", Secondary: "#7DCFFF",
		Git: "#FF9E64", Docker: "#7AA2F7", Files: "#BB9AF7", Dev: "#E0AF68",
		Text: "#A9B1D6", Muted: "#787C99", Background: "#1A1B26", Panel: "#16161E",
		Selected: "#202330", Border: "#101014",
	},
	"vercel": {
		Preset: "vercel", Name: "Vercel", Accent: "#006EFE", Secondary: "#00AD3A",
		Git: "#F13342", Docker: "#00AD3A", Files: "#9540D5", Dev: "#52A8FF",
		Text: "#EDEDED", Muted: "#A1A1A1", Background: "#000000", Panel: "#000000",
		Border: "#006EFE",
	},
	"vscode-plus": {
		Preset: "vscode-plus", Name: "VS Code Plus", Accent: "#007ACC", Secondary: "#4EC9B0",
		Git: "#F44747", Docker: "#569CD6", Files: "#C586C0", Dev: "#DCDCAA",
		Text: "#D4D4D4", Muted: "#A6A6A6", Background: "#1E1E1E", Panel: "#181818",
		Selected: "#3A3D41", Border: "#007ACC",
	},
	"xcode": {
		Preset: "xcode", Name: "Xcode", Accent: "#5482FF", Secondary: "#67B7A4",
		Git: "#FC6A5D", Docker: "#67B7A4", Files: "#FC5FA3", Dev: "#D0BF69",
		Text: "#FFFFFF", Muted: "#A8A8AD", Background: "#1F1F24", Panel: "#1F1F24",
		Selected: "#23252B", Border: "#5482FF",
	},
}

var themeAliases = map[string]string{
	"catppuccin-mocha": "catppuccin",
	"one-dark":         "one",
	"one-dark-pro":     "one",
	"rose-pine-moon":   "rose-pine",
	"tokyonight":       "tokyo-night",
	"vs-code-plus":     "vscode-plus",
}

func builtInTheme(name string) Theme {
	name, ok := canonicalThemeName(name)
	if !ok {
		name = "phosphor"
	}
	theme := builtInThemes[name]
	return completeTheme(theme)
}

func canonicalThemeName(name string) (string, bool) {
	name = strings.ToLower(strings.TrimSpace(name))
	if alias := themeAliases[name]; alias != "" {
		name = alias
	}
	_, ok := builtInThemes[name]
	return name, ok
}

func availableThemes() []Theme {
	themes := make([]Theme, 0, len(themeOrder))
	for _, name := range themeOrder {
		themes = append(themes, builtInTheme(name))
	}
	return themes
}

func completeTheme(theme Theme) Theme {
	if theme.Panel == "" {
		theme.Panel = theme.Background
	}
	if theme.Muted == "" || strings.EqualFold(theme.Muted, theme.Text) {
		theme.Muted = blendThemeColor(theme.Background, theme.Text, 55)
	}
	if theme.Selected == "" || strings.EqualFold(theme.Selected, theme.Background) || strings.EqualFold(theme.Selected, theme.Text) {
		theme.Selected = blendThemeColor(theme.Background, theme.Accent, 22)
	}
	if theme.Border == "" || strings.EqualFold(theme.Border, theme.Background) || strings.EqualFold(theme.Border, theme.Text) {
		theme.Border = blendThemeColor(theme.Background, theme.Accent, 42)
	}
	return theme
}

func blendThemeColor(background, foreground string, foregroundPercent int) string {
	parse := func(value string) [3]int64 {
		var channels [3]int64
		if len(value) != 7 || value[0] != '#' {
			return channels
		}
		for index := range channels {
			channels[index], _ = strconv.ParseInt(value[1+index*2:3+index*2], 16, 16)
		}
		return channels
	}
	base, overlay := parse(background), parse(foreground)
	return fmt.Sprintf("#%02X%02X%02X",
		(base[0]*int64(100-foregroundPercent)+overlay[0]*int64(foregroundPercent))/100,
		(base[1]*int64(100-foregroundPercent)+overlay[1]*int64(foregroundPercent))/100,
		(base[2]*int64(100-foregroundPercent)+overlay[2]*int64(foregroundPercent))/100,
	)
}

func loadTheme() (Theme, error) {
	theme := defaultTheme()
	home, err := os.UserHomeDir()
	if err != nil {
		return theme, err
	}
	path := filepath.Join(home, ".config", "alias-lens", "theme.json")
	contents, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return theme, nil
	}
	if err != nil {
		return theme, fmt.Errorf("read theme: %w", err)
	}
	var selection struct {
		Preset string `json:"preset"`
	}
	if err := json.Unmarshal(contents, &selection); err != nil {
		return theme, fmt.Errorf("parse %s: %w", path, err)
	}
	if selection.Preset != "" {
		theme = builtInTheme(selection.Preset)
	}
	if err := json.Unmarshal(contents, &theme); err != nil {
		return defaultTheme(), fmt.Errorf("parse %s: %w", path, err)
	}
	canonical, ok := canonicalThemeName(theme.Preset)
	if !ok {
		canonical = "phosphor"
	}
	theme.Preset = canonical
	return completeTheme(theme), nil
}

func saveTheme(theme Theme) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	directory := filepath.Join(home, ".config", "alias-lens")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	selection := struct {
		Preset string `json:"preset"`
	}{Preset: theme.Preset}
	contents, err := json.MarshalIndent(selection, "", "  ")
	if err != nil {
		return err
	}
	contents = append(contents, '\n')
	return os.WriteFile(filepath.Join(directory, "theme.json"), contents, 0o644)
}

func nextTheme(current string) Theme {
	current, ok := canonicalThemeName(current)
	if !ok {
		return defaultTheme()
	}
	for index, name := range themeOrder {
		if name == current {
			return builtInTheme(themeOrder[(index+1)%len(themeOrder)])
		}
	}
	return defaultTheme()
}

func runThemeCommand(arguments []string) error {
	if len(arguments) > 1 {
		return fmt.Errorf("usage: al theme [PRESET]")
	}
	if len(arguments) == 1 {
		name, ok := canonicalThemeName(arguments[0])
		if !ok {
			return fmt.Errorf("unknown theme %q; run al theme to list available presets", arguments[0])
		}
		theme := builtInTheme(name)
		if err := saveTheme(theme); err != nil {
			return err
		}
		fmt.Printf("Theme set to %s.\n", theme.Name)
		return nil
	}
	current, err := loadTheme()
	if err != nil {
		return err
	}
	for _, theme := range availableThemes() {
		marker := " "
		if theme.Preset == current.Preset {
			marker = "*"
		}
		fmt.Printf("%s %-14s %s\n", marker, theme.Preset, theme.Name)
	}
	return nil
}
