package app

func (svc *Services) ensureAliasFileExists(path string) error {
	result := SetupResult{}
	err := svc.withMutation(func(session *mutationSession) error {
		return svc.ensureAliasFileExistsInSession(session, path, &result)
	})
	if svc.dependencies.Notice != nil {
		for _, notice := range result.Notices {
			svc.dependencies.Notice(notice)
		}
	}
	return err
}
