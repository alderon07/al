package presentation

import (
	"fmt"
	"github.com/charmbracelet/x/ansi"

	"github.com/rivo/uniseg"
	"strings"
	"unicode"
)

type FooterConfig struct {
	Message   string `json:"message"`
	Icon      string `json:"icon"`
	Alignment string `json:"alignment,omitempty"`
	Tone      string `json:"tone,omitempty"`
	Rule      string `json:"rule,omitempty"`
}

type AppearanceConfig struct {
	Brand       string `json:"brand"`
	ArtStyle    string `json:"art_style"`
	MarkerStyle string `json:"marker_style"`
}

func DefaultAppearanceConfig() AppearanceConfig {
	return AppearanceConfig{Brand: "ALIAS LENS", ArtStyle: "full", MarkerStyle: "symbols"}
}

func DefaultFooterConfig() FooterConfig {
	return FooterConfig{Message: "Made by Naqi", Icon: "none", Alignment: "center", Tone: "quiet", Rule: "none"}
}

func ValidateAppearanceConfig(config AppearanceConfig) error {
	if config.ArtStyle != "full" && config.ArtStyle != "compact" && config.ArtStyle != "text" && config.ArtStyle != "none" {
		return fmt.Errorf("choose full, compact, text, or none for Brand art")
	}
	if config.MarkerStyle != "symbols" && config.MarkerStyle != "ascii" && config.MarkerStyle != "none" {
		return fmt.Errorf("choose symbols, ascii, or none for Markers")
	}
	return nil
}

func ValidateFooterConfig(config FooterConfig) error {
	if terminalWidth(config.Message) > 80 {
		return fmt.Errorf("footer message must be 80 terminal cells or fewer")
	}
	for _, character := range config.Message {
		if unicode.IsControl(character) {
			return fmt.Errorf("footer message cannot contain control characters or newlines")
		}
	}
	if strings.Count(config.Message, "{icon}") > 1 {
		return fmt.Errorf("footer message may contain {icon} once")
	}
	if _, err := RenderFooterIcon(config.Icon); err != nil {
		return err
	}
	if config.Alignment != "" && config.Alignment != "left" && config.Alignment != "center" && config.Alignment != "right" {
		return fmt.Errorf("choose left, center, or right for footer alignment")
	}
	if config.Tone != "" && config.Tone != "quiet" && config.Tone != "accent" && config.Tone != "bright" {
		return fmt.Errorf("choose quiet, accent, or bright for footer tone")
	}
	if config.Rule != "" && config.Rule != "none" && config.Rule != "thin" && config.Rule != "dots" {
		return fmt.Errorf("choose none, thin, or dots for the footer rule")
	}
	return nil
}

func RenderFooterIcon(value string) (string, error) {
	if value == "none" {
		return "", nil
	}
	if strings.HasPrefix(value, "emoji:") {
		glyph := strings.TrimPrefix(value, "emoji:")
		graphemes := uniseg.NewGraphemes(glyph)
		count := 0
		for graphemes.Next() {
			count++
		}
		if count != 1 || terminalWidth(glyph) < 1 || terminalWidth(glyph) > 2 {
			return "", fmt.Errorf("enter one visible emoji after emoji:, for example emoji:🚀")
		}
		for _, character := range glyph {
			if unicode.IsControl(character) {
				return "", fmt.Errorf("footer emoji cannot contain control characters")
			}
		}
		return glyph, nil
	}
	if icon, ok := NamedPixelIcon(value); ok {
		return icon.Symbol, nil
	}
	icon, ok := ParsePixelIcon(value)
	if !ok {
		return "", fmt.Errorf("choose a named footer icon, none, or one emoji written as emoji:VALUE")
	}
	return RenderPixelIcon(icon), nil
}

func terminalWidth(value string) (width int) {
	for _, line := range strings.Split(value, "\n") {
		width = max(width, ansi.StringWidth(line))
	}
	return width
}
