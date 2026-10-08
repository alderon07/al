//go:build !windows

package app

import (
	shellapi "alias-lens/internal/shell"
	"context"
)

func (svc *Services) validateSyntax(ctx context.Context, name string, contents []byte) error {
	if svc.dependencies.ValidateSyntax != nil {
		return svc.dependencies.ValidateSyntax(ctx, name, contents)
	}
	if svc.dependencies.FindTrustedShell != nil {
		return shellapi.ValidateSyntax(ctx, name, contents, svc.dependencies.FindTrustedShell)
	}
	return shadowSyntaxValidator(ctx, name, contents)
}
