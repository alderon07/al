//go:build windows

package tui

import (
	"alias-lens/internal/app"
	"fmt"
)

func CatalogConflict(_ *app.Services, _ string) error {
	return fmt.Errorf("catalog activation needs Bash or Zsh on Linux, WSL, or macOS")
}
