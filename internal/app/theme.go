package app

import (
	"encoding/json"
	"fmt"

	"os"
	"path/filepath"
)

func (svc *Services) loadTheme() (Theme, error) {
	theme := defaultTheme()
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return theme, err
	}
	path := filepath.Join(home, ".config", "alias-lens", "theme.json")
	contents, err := readManagedPrivateFile(path, 1<<20)
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

func (svc *Services) saveTheme(theme Theme) error {
	return svc.withMutation(func(session *mutationSession) error { return saveThemeInSession(session, theme) })
}
func saveThemeInSession(session *mutationSession, theme Theme) error {
	selection := struct {
		Preset string `json:"preset"`
	}{theme.Preset}
	contents, e := json.MarshalIndent(selection, "", "  ")
	if e != nil {
		return e
	}
	return session.WritePrivate(filepath.Join(session.configRoot, "theme.json"), append(contents, '\n'))
}
