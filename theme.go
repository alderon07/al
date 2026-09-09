package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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

func builtInTheme(name string) Theme {
	switch strings.ToLower(name) {
	case "darcula":
		return Theme{
			Preset: "darcula", Name: "Darcula", Accent: "#FFC66D", Secondary: "#6897BB",
			Git: "#CC7832", Docker: "#6A8759", Files: "#9876AA", Dev: "#BBB529",
			Text: "#A9B7C6", Muted: "#808080", Background: "#2B2B2B", Panel: "#313335",
			Selected: "#214283", Border: "#555555",
		}
	case "dracula":
		return Theme{
			Preset: "dracula", Name: "Dracula", Accent: "#50FA7B", Secondary: "#8BE9FD",
			Git: "#FFB86C", Docker: "#8BE9FD", Files: "#BD93F9", Dev: "#FF79C6",
			Text: "#F8F8F2", Muted: "#6272A4", Background: "#282A36", Panel: "#343746",
			Selected: "#44475A", Border: "#6272A4",
		}
	case "catppuccin", "catppuccin-mocha":
		return Theme{
			Preset: "catppuccin", Name: "Catppuccin Mocha", Accent: "#A6E3A1", Secondary: "#89DCEB",
			Git: "#FAB387", Docker: "#89DCEB", Files: "#CBA6F7", Dev: "#F9E2AF",
			Text: "#CDD6F4", Muted: "#9399B2", Background: "#1E1E2E", Panel: "#313244",
			Selected: "#45475A", Border: "#585B70",
		}
	default:
		return Theme{
			Preset:     "phosphor",
			Name:       "Phosphor",
			Accent:     "#B8FF6A",
			Secondary:  "#72DDF7",
			Git:        "#FF8A65",
			Docker:     "#72DDF7",
			Files:      "#C6A0F6",
			Dev:        "#FFD166",
			Text:       "#F3F6EE",
			Muted:      "#8EA6A2",
			Background: "#0D1211",
			Panel:      "#151C1A",
			Selected:   "#21302A",
			Border:     "#33443F",
		}
	}
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
	return theme, nil
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
	switch strings.ToLower(current) {
	case "phosphor":
		return builtInTheme("darcula")
	case "darcula":
		return builtInTheme("dracula")
	case "dracula":
		return builtInTheme("catppuccin")
	default:
		return builtInTheme("phosphor")
	}
}
