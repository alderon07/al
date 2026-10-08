package presentation

import (
	"strings"
	"testing"

	"github.com/charmbracelet/lipgloss"
)

func TestPixelIconsHaveOneFixedWidthRow(t *testing.T) {
	icons := []PixelIcon{
		iconAlias,
		iconBrand,
		iconCommand,
		iconContext,
		iconEdit,
		iconFavorite,
		iconFunction,
		iconHealth,
		iconHeart,
		iconHelp,
		iconHistory,
		iconRepository,
		iconSearch,
		iconSpark,
		iconStats,
		iconSync,
		iconTheme,
	}
	for index, icon := range icons {
		rendered := RenderPixelIcon(icon)
		if strings.ContainsRune(rendered, '\n') {
			t.Errorf("icon %d contains a newline: %q", index, rendered)
		}
		if width := lipgloss.Width(rendered); width != 4 {
			t.Errorf("icon %d width = %d, want 4: %q", index, width, rendered)
		}
	}
}
