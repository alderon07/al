//go:build windows

package app

import (
	workflowplan "alias-lens/internal/plan"
	"fmt"
)

func (svc *Services) ApplyCatalogPull(workflowplan.OperationPlan) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}
func (svc *Services) ApplyCatalogPush(CatalogPushPreview) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}
