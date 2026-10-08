//go:build windows

package transaction

func InspectWorkflowTarget(path string, limit int64, preserveSymlink bool) (FileIdentity, error) {
	return FileIdentity{}, ErrUnsafePath
}
func ApplyWorkflow(spec WorkflowSpec, hooks ...func(WorkflowBoundary) error) error {
	return ErrUnsafePath
}
func RecoverWorkflows(stateRoot string, hooks ...func(WorkflowBoundary) error) error {
	return ErrUnsafePath
}
func ReadWorkflowJournal(stateRoot, path string) (WorkflowManifest, []WorkflowProgress, error) {
	return WorkflowManifest{}, nil, ErrUnsafePath
}
func EnsureWorkflowDirectory(path string, private bool) error { return ErrUnsafePath }

func InspectWorkflowDirectory(path string) (FileIdentity, error) {
	return FileIdentity{}, ErrUnsafePath
}
func ReadWorkflowTarget(path string, limit int64) (FileIdentity, []byte, error) {
	return FileIdentity{}, nil, ErrUnsafePath
}
func AppendWorkflowPrivateFile(path string, contents []byte) error { return ErrUnsafePath }
func RemoveWorkflowPrivateFile(path string) error                  { return ErrUnsafePath }

func InspectWorkflowDirectoryMetadata(path string) (FileIdentity, error) {
	return FileIdentity{}, ErrUnsafePath
}
