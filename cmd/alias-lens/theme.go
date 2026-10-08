package main

import "fmt"
import "github.com/alderon07/al/internal/presentation"

func runThemeCommand(arguments []string) error {
	if len(arguments) > 1 {
		return fmt.Errorf("usage: al theme [PRESET|--check]")
	}
	if len(arguments) == 1 {
		if arguments[0] == "--check" {
			theme, err := applicationServices().LoadTheme()
			if err != nil {
				return err
			}
			ratios := presentation.ThemeContrastRatios(theme)
			textRatio, controlRatio := ratios.Text, ratios.Controls
			fmt.Printf("%s: text %.2f:1, controls %.2f:1\n", theme.Name, textRatio, controlRatio)
			if textRatio < 4.5 || controlRatio < 3 {
				return fmt.Errorf("theme contrast is below WCAG AA; run al theme phosphor")
			}
			return nil
		}
		name, ok := canonicalThemeName(arguments[0])
		if !ok {
			return fmt.Errorf("unknown theme %q; run al theme to list available presets", arguments[0])
		}
		theme := builtInTheme(name)
		if err := applicationServices().SaveTheme(theme); err != nil {
			return err
		}
		cliResult(fmt.Sprintf("Theme set to %s.", theme.Name))
		return nil
	}
	current, err := applicationServices().LoadTheme()
	if err != nil {
		return err
	}
	for _, theme := range availableThemes() {
		marker := " "
		if theme.Preset == current.Preset {
			marker = "*"
		}
		fmt.Printf("%s %s %s\n", cliPositive(marker), cliAccent(fmt.Sprintf("%-14s", theme.Preset)), theme.Name)
	}
	return nil
}
