package presentation

import "strings"

type PixelIcon struct {
	Top    string
	Bottom string
	Symbol string
	ASCII  string
}

var (
	iconAlias      = PixelIcon{"#..#", ".##.", "›", ">"}
	iconBrand      = PixelIcon{".##.", "####", "◉", "*"}
	iconCommand    = PixelIcon{"#...", ".###", "$", "$"}
	iconContext    = PixelIcon{".##.", "#..#", "⌖", "@"}
	iconEdit       = PixelIcon{"...#", ".##.", "✎", "~"}
	iconFavorite   = PixelIcon{".#.#", "###.", "♥︎", "*"}
	iconFunction   = PixelIcon{"#..#", ".##.", "ƒ", "f"}
	iconHealth     = PixelIcon{".##.", "####", "!", "!"}
	iconHeart      = PixelIcon{"#..#", ".##.", "♥︎", "*"}
	iconHelp       = PixelIcon{".##.", "..#.", "?", "?"}
	iconHistory    = PixelIcon{"###.", "#.##", "↶", "<"}
	iconRepository = PixelIcon{"##..", "####", "◇", "#"}
	iconSearch     = PixelIcon{".##.", "..##", "⌕", "/"}
	iconSpark      = PixelIcon{".#.#", "###.", "✦", "*"}
	iconStats      = PixelIcon{"...#", "#.##", "▥", "#"}
	iconSync       = PixelIcon{"##..", "..##", "↻", "~"}
	iconTheme      = PixelIcon{"##..", ".##.", "◐", "o"}
)

func ParsePixelIcon(value string) (PixelIcon, bool) {
	if icon, ok := NamedPixelIcon(value); ok {
		return icon, true
	}
	rows := strings.Split(value, "/")
	if len(rows) != 2 || len(rows[0]) != 4 || len(rows[1]) != 4 {
		return PixelIcon{}, false
	}
	for _, row := range rows {
		for _, cell := range row {
			if cell != '#' && cell != '.' {
				return PixelIcon{}, false
			}
		}
	}
	return PixelIcon{Top: rows[0], Bottom: rows[1]}, true
}

func NamedPixelIcon(value string) (PixelIcon, bool) {
	named := map[string]PixelIcon{
		"alias":      iconAlias,
		"brand":      iconBrand,
		"command":    iconCommand,
		"context":    iconContext,
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
	return PixelIcon{}, false
}

func RenderPixelIcon(icon PixelIcon) string {
	var rendered strings.Builder
	for index := range icon.Top {
		Top := icon.Top[index] == '#'
		Bottom := icon.Bottom[index] == '#'
		switch {
		case Top && Bottom:
			rendered.WriteRune('█')
		case Top:
			rendered.WriteRune('▀')
		case Bottom:
			rendered.WriteRune('▄')
		default:
			rendered.WriteByte(' ')
		}
	}
	return rendered.String()
}
