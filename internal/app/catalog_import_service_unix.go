//go:build !windows

package app

import (
	workflowplan "github.com/alderon07/al/internal/plan"
	"path/filepath"
)

func (s *Services) ApplyCatalogImport(preview workflowplan.OperationPlan, shell string) error {
	return s.applyPrivatePlan(filepath.Dir(s.localCatalogPath()), preview, func() (workflowplan.OperationPlan, error) { return s.buildCatalogImportPlan(shell) })
}
