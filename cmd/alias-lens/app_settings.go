package main

import (
	"alias-lens/internal/app"
	"fmt"
)

type AppConfig = app.AppConfig
type AutoSyncConfig = app.AutoSyncConfig
type TrackedFileConfig = app.TrackedFileConfig
type ProviderConfig = app.ProviderConfig

const currentConfigVersion = 2

var applicationDependencies app.Dependencies

func applicationServices() *app.Services {
	dependencies := applicationDependencies
	if dependencies.Notice == nil {
		dependencies.Notice = func(notice app.OperationNotice) { fmt.Println(notice.Message) }
	}
	return app.NewServices(dependencies)
}

func loadConfig() (AppConfig, error) { return applicationServices().Settings.Load() }
func defaultConfig() AppConfig       { return applicationServices().Settings.Defaults() }
func ensureConfigDefaults(config AppConfig) AppConfig {
	return applicationServices().Settings.FillDefaults(config)
}
func validateAppConfig(config AppConfig) error {
	return applicationServices().Settings.Validate(config)
}
func saveConfig(config AppConfig) error { return applicationServices().Settings.Save(config) }
func updateConfig(change func(*AppConfig) error) error {
	return applicationServices().Settings.Update(change)
}

func configPath() (string, error) { return applicationServices().Settings.Path() }

type observedConfig = app.ObservedConfig

func observeConfig() (observedConfig, error) { return applicationServices().Settings.Observe() }
