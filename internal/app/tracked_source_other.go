//go:build !linux && !darwin

package app

import (
	"fmt"
	"os"
)

func openTrustedTrackedSource(path string, validate func(string) error) (*os.File, string, error) {
	return nil, "", fmt.Errorf("safe tracked source observation is unavailable on this platform; run al untrack FILE")
}
