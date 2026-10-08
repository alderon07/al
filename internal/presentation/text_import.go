package presentation

import (
	"fmt"

	"strings"
	"unicode"
)

func TerminalSafeText(value string) string {
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
