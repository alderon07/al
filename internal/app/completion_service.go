package app

import (
	"bytes"
	"fmt"
	workflowplan "github.com/alderon07/al/internal/plan"
	"path/filepath"
	"strings"
	"unicode/utf8"
)

func (s *Services) PreviewCompletion(action, shell string, script []byte) (workflowplan.OperationPlan, error) {
	return s.previewCompletionPlan(action, shell, script)
}
func (s *Services) ApplyCompletion(preview workflowplan.OperationPlan, action, shell string, script []byte) error {
	path, err := s.completionInstallPath(shell)
	if err != nil {
		return err
	}
	return s.applyPrivatePlan(filepath.Dir(path), preview, func() (workflowplan.OperationPlan, error) { return s.previewCompletionPlan(action, shell, script) })
}
func (s *Services) CompletionIntegrationState(shell string) string {
	adapter, err := s.shellAdapter(shell)
	if err != nil {
		return "missing"
	}
	return s.completionIntegrationState(adapter)
}
func (s *Services) CompletionCandidates(shell, kind, prefix string) ([]byte, error) {
	adapter, err := s.shellAdapter(shell)
	if err != nil {
		return nil, err
	}
	if len(prefix) > 256 || !utf8.ValidString(prefix) {
		return nil, fmt.Errorf("invalid completion prefix")
	}
	var candidates []string
	switch kind {
	case "entries":
		candidates, err = s.completionEntryCandidates(adapter)
	case "profiles":
		candidates, err = s.completionProfileCandidates()
	default:
		return nil, fmt.Errorf("invalid completion kind")
	}
	if err != nil {
		return nil, err
	}
	filtered := candidates[:0]
	for _, candidate := range candidates {
		if strings.HasPrefix(candidate, prefix) {
			filtered = append(filtered, candidate)
		}
		if len(filtered) == completionMaxResults {
			break
		}
	}
	var buffer bytes.Buffer
	for _, candidate := range filtered {
		if !completionCandidateName.MatchString(candidate) || buffer.Len()+len(candidate)+1 > completionOutputLimit {
			return nil, fmt.Errorf("invalid completion result")
		}
		buffer.WriteString(candidate)
		buffer.WriteByte('\n')
	}
	return buffer.Bytes(), nil
}
