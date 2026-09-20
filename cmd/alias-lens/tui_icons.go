package main

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type pixelIcon struct {
	top    string
	bottom string
	symbol string
	ascii  string
}

const aliasLensFullMark = "  ▗▄▄▄▖\n ▐▛ ◉ ▜▌\n  ▝▀▀▀▘▖"

var (
	iconAlias      = pixelIcon{"#..#", ".##.", "›", ">"}
	iconBrand      = pixelIcon{".##.", "####", "◉", "*"}
	iconCommand    = pixelIcon{"#...", ".###", "$", "$"}
	iconEdit       = pixelIcon{"...#", ".##.", "✎", "~"}
	iconFavorite   = pixelIcon{".#.#", "###.", "♥", "*"}
	iconFunction   = pixelIcon{"#..#", ".##.", "ƒ", "f"}
	iconHealth     = pixelIcon{".##.", "####", "!", "!"}
	iconHeart      = pixelIcon{"#..#", ".##.", "♥", "*"}
	iconHelp       = pixelIcon{".##.", "..#.", "?", "?"}
	iconHistory    = pixelIcon{"###.", "#.##", "↶", "<"}
	iconRepository = pixelIcon{"##..", "####", "◇", "#"}
	iconSearch     = pixelIcon{".##.", "..##", "⌕", "/"}
	iconSpark      = pixelIcon{".#.#", "###.", "✦", "*"}
	iconStats      = pixelIcon{"...#", "#.##", "▥", "#"}
	iconSync       = pixelIcon{"##..", "..##", "↻", "~"}
	iconTheme      = pixelIcon{"##..", ".##.", "◐", "o"}
)

func parsePixelIcon(value string) (pixelIcon, bool) {
	named := map[string]pixelIcon{
		"alias":      iconAlias,
		"brand":      iconBrand,
		"command":    iconCommand,
		"edit":       iconEdit,
		"favorite":   iconFavorite,
		"function":   iconFunction,
		"health":     iconHealth,
		"heart":      iconHeart,
		"help":       iconHelp,
		"history":    iconHistory,
		"repository": iconRepository,
		"search":     iconSearch,
		"spark":      iconSpark,
		"stats":      iconStats,
		"sync":       iconSync,
		"theme":      iconTheme,
	}
	if icon, ok := named[strings.ToLower(value)]; ok {
		return icon, true
	}
	rows := strings.Split(value, "/")
	if len(rows) != 2 || len(rows[0]) != 4 || len(rows[1]) != 4 {
		return pixelIcon{}, false
	}
	for _, row := range rows {
		for _, cell := range row {
			if cell != '#' && cell != '.' {
				return pixelIcon{}, false
			}
		}
	}
	return pixelIcon{top: rows[0], bottom: rows[1]}, true
}

func renderPixelIcon(icon pixelIcon) string {
	var rendered strings.Builder
	for index := range icon.top {
		top := icon.top[index] == '#'
		bottom := icon.bottom[index] == '#'
		switch {
		case top && bottom:
			rendered.WriteRune('█')
		case top:
			rendered.WriteRune('▀')
		case bottom:
			rendered.WriteRune('▄')
		default:
			rendered.WriteByte(' ')
		}
	}
	return rendered.String()
}

func interfaceMarker(icon pixelIcon) string {
	switch activeAppearance.MarkerStyle {
	case "symbols":
		return icon.symbol
	case "ascii":
		return icon.ascii
	default:
		return ""
	}
}

func markerPrefix(icon pixelIcon) string {
	marker := interfaceMarker(icon)
	if marker == "" {
		return ""
	}
	return marker + " "
}

func pixelIconLabel(icon pixelIcon, label string, style lipgloss.Style) string {
	return style.Render(markerPrefix(icon) + label)
}

func compactBrandLabel() string {
	switch activeAppearance.ArtStyle {
	case "none":
		return ""
	case "compact":
		return "▗◉▖ " + activeAppearance.Brand
	default:
		return activeAppearance.Brand
	}
}

func fullBrandMark() string {
	if activeAppearance.ArtStyle != "full" {
		return ""
	}
	return lipgloss.NewStyle().Bold(true).Foreground(acidColor).Render(aliasLensFullMark)
}
