package app

type OperationNotice struct {
	Message string
	Styled  bool
}
type SetupRequest struct {
	Shell       string
	Repair      bool
	Interactive bool
}
type SetupResult struct {
	AliasPath string
	Notices   []OperationNotice
}

type SetupRemovalResult struct {
	Shell       string
	DisplayName string
}
