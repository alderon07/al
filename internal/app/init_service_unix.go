//go:build !windows

package app

import (
	"bytes"
	"context"
	"fmt"
	workflowplan "github.com/alderon07/al/internal/plan"
)

func (s *Services) ApplyRemoteCatalogInit(ctx context.Context, config AppConfig, options CatalogInitOptions, remote RemoteCatalogPreview, decisions CatalogLifecycleDecisions, preview workflowplan.OperationPlan) error {
	return s.withMutation(func(session *mutationSession) error {
		freshRemote, err := s.discoverRemoteCatalog(ctx, config, options.Source, options.CatalogPath)
		if err != nil {
			return err
		}
		if freshRemote.Revision != remote.Revision || freshRemote.Blob != remote.Blob || freshRemote.Branch != remote.Branch || !bytes.Equal(freshRemote.Bytes, remote.Bytes) {
			return fmt.Errorf("remote catalog changed during review; restart al init")
		}
		fresh, err := s.buildRemoteCatalogInitPlan(options, freshRemote, decisions, "")
		if err != nil {
			return err
		}
		if err := workflowplan.CheckFresh(preview, fresh); err != nil {
			return fmt.Errorf("catalog installation inputs changed after review")
		}
		intent, stage, err := s.stageRemoteCatalog(ctx, session, remote)
		if err != nil {
			return err
		}
		defer cleanupManagedCatalogStage(intent)
		concrete, err := s.buildRemoteCatalogInitPlan(options, remote, decisions, stage)
		if err != nil {
			return err
		}
		if err := workflowplan.CheckFresh(remoteInitSemanticPlan(preview), remoteInitSemanticPlan(concrete)); err != nil {
			return fmt.Errorf("staged catalog changes differ from the reviewed plan")
		}
		if err := session.ApplyPlan(concrete, func() (workflowplan.OperationPlan, error) {
			return s.buildRemoteCatalogInitPlan(options, remote, decisions, stage)
		}); err != nil {
			return err
		}
		if err := cleanupManagedCatalogStage(intent); err != nil {
			return err
		}
		if err := session.RemovePrivate(s.managedStageIntentPath(intent.ID)); err != nil {
			return err
		}
		return nil
	})
}
func (s *Services) ApplyLocalCatalogInit(preview workflowplan.OperationPlan, options CatalogInitOptions, decisions CatalogLifecycleDecisions) error {
	return s.applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return s.buildLocalCatalogInitPlan(options, decisions) })
}
func (s *Services) ApplyCatalogDecisions(preview workflowplan.OperationPlan, decisions CatalogLifecycleDecisions) error {
	return s.applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return s.buildCatalogDecisionPlan(decisions) })
}
func (s *Services) ApplyCatalogInstallation(preview workflowplan.OperationPlan, shell string, decisions CatalogLifecycleDecisions) error {
	return s.applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return s.buildCatalogEnablePlan(shell, decisions) })
}
func (s *Services) ApplyCatalogConflictResolution(preview workflowplan.OperationPlan, id string, choices map[string]string) error {
	return s.applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return s.buildCatalogConflictResolutionPlan(id, choices) })
}
func (s *Services) ApplyCatalogPull(preview workflowplan.OperationPlan) error {
	return s.applyMutationPlan(preview, s.buildCatalogSyncPullPlan)
}
func (s *Services) ApplyCatalogPush(preview CatalogPushPreview) error {
	return s.withMutation(func(session *mutationSession) error { return s.runCatalogSyncPushInSession(session, preview) })
}
func (s *Services) ApplyCatalogRollback(preview workflowplan.OperationPlan, shell string) error {
	return s.applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return s.buildCatalogRollbackPlan(shell) })
}
