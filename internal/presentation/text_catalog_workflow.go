package presentation

import (
	"encoding/json"

	"strings"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
)

func CatalogFieldValue(entry neutralcatalog.Entry, path string) any {
	switch path {
	case "entry":
		return entry
	case "name":
		return entry.Name
	case "kind":
		return entry.Kind
	case "description":
		return entry.Description
	case "category":
		return entry.Category
	case "tags":
		return entry.Tags
	case "platforms":
		return entry.Platforms
	case "favorite":
		return entry.Favorite
	case "portable":
		return entry.Portable
	case "native.bash":
		return entry.Native["bash"]
	case "native.zsh":
		return entry.Native["zsh"]
	case "when.profiles_any":
		if entry.When != nil {
			return entry.When.ProfilesAny
		}
	case "when.profiles_none":
		if entry.When != nil {
			return entry.When.ProfilesNone
		}
	case "when.shells":
		if entry.When != nil {
			return entry.When.Shells
		}
	}
	return nil
}

func QuotedCatalogValue(value any) string {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "(could not display)"
	}
	return EscapePlainText(string(encoded))
}

func FriendlyShellName(shell string) string {
	if shell == "bash" {
		return "Bash"
	}
	if shell == "zsh" {
		return "Zsh"
	}
	return shell
}

func ShortFingerprint(value string) string {
	if len(value) > 12 {
		return value[:12]
	}
	return value
}

func FriendlyCatalogField(path string) string {
	labels := map[string]string{
		"schema_version":     "data format",
		"name":               "name",
		"kind":               "type",
		"description":        "description",
		"category":           "category",
		"tags":               "tags",
		"platforms":          "supported computers",
		"favorite":           "favorite setting",
		"portable":           "portable command",
		"native.bash":        "Bash command",
		"native.zsh":         "Zsh command",
		"when.profiles_any":  "machine profiles",
		"when.profiles_none": "excluded machine profiles",
		"when.shells":        "supported shells",
	}
	if label := labels[path]; label != "" {
		return label
	}
	return "details"
}

func EscapePlainText(value string) string {
	return strings.Map(func(character rune) rune {
		if character < 0x20 || character == 0x7f {
			return '?'
		}
		return character
	}, value)
}
