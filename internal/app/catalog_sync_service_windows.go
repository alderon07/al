//go:build windows

package app

import (
	"fmt"
	workflowplan "github.com/alderon07/al/internal/plan"
)

func (svc *Services) ApplyCatalogPull(workflowplan.OperationPlan) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}
func (svc *Services) ApplyCatalogPush(CatalogPushPreview) error {
	return fmt.Errorf("catalog activation is unavailable on Windows")
}
