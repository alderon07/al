package app

import (
	workflowplan "alias-lens/internal/plan"
	"path/filepath"
)

func (s *Services) ApplyProfile(preview workflowplan.OperationPlan, action, name string) error {
	path, err := s.configPath()
	if err != nil {
		return err
	}
	return s.applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) { return s.buildProfilePlan(action, name) })
}
func (s *Services) EnsureAliasFile(path string) ([]OperationNotice, error) {
	result := SetupResult{}
	err := s.withMutation(func(session *mutationSession) error {
		return s.ensureAliasFileExistsInSession(session, path, &result)
	})
	return result.Notices, err
}
