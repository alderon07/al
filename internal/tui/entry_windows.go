//go:build windows

package tui

import (
	"fmt"
	"github.com/alderon07/al/internal/app"
)

func CatalogConflict(_ *app.Services, _ string) error {
	return fmt.Errorf("catalog activation needs Bash or Zsh on Linux, WSL, or macOS")
}
