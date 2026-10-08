//go:build !windows

package app

import (
	catalog "github.com/alderon07/al/internal/catalog"

	catalogstore "github.com/alderon07/al/internal/catalogstore"
	workflowplan "github.com/alderon07/al/internal/plan"
)

func (s *Services) BuildCatalogDecisionPlan(decisions CatalogLifecycleDecisions) (workflowplan.OperationPlan, error) {
	return s.buildCatalogDecisionPlan(decisions)
}
func (s *Services) BuildCatalogEnablePlan(shell string, decisions CatalogLifecycleDecisions) (workflowplan.OperationPlan, error) {
	return s.buildCatalogEnablePlan(shell, decisions)
}
func (s *Services) BuildCatalogImportPlan(shell string) (workflowplan.OperationPlan, error) {
	return s.buildCatalogImportPlan(shell)
}
func (s *Services) BuildCatalogRollbackPlan(shell string) (workflowplan.OperationPlan, error) {
	return s.buildCatalogRollbackPlan(shell)
}
func (s *Services) BuildLocalCatalogInitPlan(options CatalogInitOptions, decisions CatalogLifecycleDecisions) (workflowplan.OperationPlan, error) {
	return s.buildLocalCatalogInitPlan(options, decisions)
}
func (s *Services) BuildRemoteCatalogInitPlan(options CatalogInitOptions, preview RemoteCatalogPreview, decisions CatalogLifecycleDecisions, stage string) (workflowplan.OperationPlan, error) {
	return s.buildRemoteCatalogInitPlan(options, preview, decisions, stage)
}

func (s *Services) CatalogEntryRunnable(alias Alias) bool { return catalogEntryRunnable(alias) }

func (s *Services) CatalogManagedEditing() (bool, error) { return s.catalogManagedEditing() }

func (s *Services) InspectCatalogShadow(explicitShell string) (ShadowReport, error) {
	return s.inspectCatalogShadow(explicitShell)
}
func (s *Services) LifecycleTimestamp() string { return s.lifecycleTimestamp() }
func (s *Services) ObserveLocalCatalogInit(options CatalogInitOptions) (catalogstore.CatalogSyncRecord, catalog.Catalog, []byte, error) {
	return s.observeLocalCatalogInit(options)
}
