package tui

import "alias-lens/internal/app"

var applicationDependencies app.Dependencies

func applicationServices() *app.Services          { return app.NewServices(applicationDependencies) }
func loadConfig() (appConfig, error)              { return applicationServices().Settings.Load() }
func defaultConfig() appConfig                    { return applicationServices().Settings.Defaults() }
func validateAppConfig(c appConfig) error         { return applicationServices().Settings.Validate(c) }
func saveConfig(c appConfig) error                { return applicationServices().Settings.Save(c) }
func updateConfig(f func(*appConfig) error) error { return applicationServices().Settings.Update(f) }
