package app

import (
	"github.com/alderon07/al/internal/presentation"
	"github.com/alderon07/al/internal/shortcuts"
)

type Theme = presentation.Theme

type settingsSession struct {
	session  *mutationSession
	services *Services
}

func (writer settingsSession) WriteSettings(contents []byte) error {
	svc := writer.services
	if svc == nil {
		svc = DefaultServices()
	}

	path, err := svc.configPath()
	if err != nil {
		return err
	}
	return writer.session.WritePrivate(path, contents)
}
func (svc *Services) loadConfig() (AppConfig, error) { return svc.internalSettings().Load() }
func (svc *Services) observeConfig() (ObservedConfig, error) {
	return svc.internalSettings().Observe()
}
func (svc *Services) defaultConfig() AppConfig { return svc.internalSettings().Defaults() }
func (svc *Services) ensureConfigDefaults(config AppConfig) AppConfig {
	return svc.internalSettings().FillDefaults(config)
}
func (svc *Services) validateAppConfig(config AppConfig) error {
	return svc.internalSettings().Validate(config)
}
func (svc *Services) saveConfig(config AppConfig) error {
	return svc.internalSettings().Save(config)
}
func (svc *Services) updateConfig(change func(*AppConfig) error) error {
	return svc.internalSettings().Update(change)
}
func (svc *Services) saveConfigInSession(session *mutationSession, config AppConfig) error {
	return svc.internalSettings().saveConfigInSession(settingsSession{session: session, services: svc}, config)
}
func (svc *Services) updateConfigInSession(session *mutationSession, change func(*AppConfig) error) error {
	return svc.internalSettings().updateConfigInSession(settingsSession{session: session, services: svc}, change)
}
func (svc *Services) saveConfigFile(path string, config AppConfig) error {
	return svc.internalSettings().saveConfigFile(path, config)
}
func (svc *Services) configPath() (string, error) { return svc.internalSettings().Path() }
func (svc *Services) expandUserPath(path string) (string, error) {
	return svc.internalSettings().expandUserPath(path)
}
func (svc *Services) validateTrackedFileConfig(config TrackedFileConfig) error {
	return svc.internalSettings().validateTrackedFileConfig(config)
}
func (svc *Services) sensitiveConfigPath(path string) bool {
	return svc.internalSettings().sensitiveConfigPath(path)
}
func (svc *Services) sameFilePath(left, right string) bool {
	return svc.internalSettings().sameFilePath(left, right)
}
func launcherLabel(config AppConfig) string         { return shortcuts.LauncherLabel(config.Shortcuts) }
func defaultTheme() Theme                           { return presentation.DefaultTheme() }
func builtInTheme(name string) Theme                { return presentation.BuiltInTheme(name) }
func canonicalThemeName(name string) (string, bool) { return presentation.CanonicalThemeName(name) }
func completeTheme(theme Theme) Theme               { return presentation.CompleteTheme(theme) }

func (svc *Services) internalSettings() *SettingsService { return svc.Settings }
