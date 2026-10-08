package providers

import (
	"fmt"
	"path/filepath"
	"strings"
	"unicode"
)

func cleanRepositoryRelativePath(path, field string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s must name a file inside the configured repository", field)
	}
	return cleaned, nil
}

func cleanRepositoryPathComponent(component string) (string, error) {
	if component == "" || component == "." || component == ".." || strings.ContainsAny(component, `/\`) {
		return "", fmt.Errorf("unsafe path component %q", component)
	}
	return component, nil
}

func cleanRemoteRepositoryParts(fullName string) ([]string, error) {
	if fullName == "" || strings.HasPrefix(fullName, "/") || strings.Contains(fullName, `\`) {
		return nil, fmt.Errorf("invalid remote repository name %q", fullName)
	}
	parts := strings.Split(fullName, "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("remote repository name %q must include its namespace", fullName)
	}
	for _, part := range parts {
		if _, err := cleanRepositoryPathComponent(part); err != nil {
			return nil, fmt.Errorf("invalid remote repository name %q: %w", fullName, err)
		}
	}
	return parts, nil
}

func terminalSafeText(value string) string {
	var output strings.Builder
	for _, character := range value {
		if unicode.IsControl(character) || isBidirectionalFormatting(character) {
			if character <= 0xff {
				fmt.Fprintf(&output, `\x%02x`, character)
			} else {
				fmt.Fprintf(&output, `\u%04x`, character)
			}
			continue
		}
		output.WriteRune(character)
	}
	return output.String()
}

func isBidirectionalFormatting(character rune) bool {
	switch character {
	case '\u061c', '\u200e', '\u200f', '\u202a', '\u202b', '\u202c', '\u202d', '\u202e', '\u2066', '\u2067', '\u2068', '\u2069':
		return true
	default:
		return false
	}
}
