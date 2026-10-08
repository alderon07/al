package tui

import "github.com/charmbracelet/lipgloss"

const aliasLensFullMark = "  ▗▄▄▄▖\n ▐▛ ◉ ▜▌\n  ▝▀▀▀▘▖"

func interfaceMarker(icon pixelIcon) string {
	switch activeAppearance.MarkerStyle {
	case "symbols":
		return icon.Symbol
	case "ascii":
		return icon.ASCII
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
