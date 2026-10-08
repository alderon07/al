//go:build windows

package app

import (
	"context"
	"fmt"
)

func (svc *Services) validateSyntax(ctx context.Context, name string, contents []byte) error {
	if svc.dependencies.ValidateSyntax != nil {
		return svc.dependencies.ValidateSyntax(ctx, name, contents)
	}
	return fmt.Errorf("shell syntax validation is unavailable on Windows")
}
