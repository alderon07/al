package tui

import (
	"github.com/alderon07/al/internal/app"
	"github.com/alderon07/al/internal/entry"
)

type aliasEntry = entry.Alias
type appConfig = app.AppConfig
type autoSyncConfig = app.AutoSyncConfig
type trackedFileConfig = app.TrackedFileConfig
type providerConfig = app.ProviderConfig

func (m model) service() *app.Services {
	return m.services
}
func (m statsModel) service() *app.Services {
	return m.services
}
func (m repoPickerModel) service() *app.Services {
	return m.services
}
func (m terminalDiff) service() *app.Services {
	return m.services
}

func (m model) metadataInvalid(value app.EntryMetadata) bool {
	_, err := m.service().ValidateMetadata(value)
	return err != nil
}
