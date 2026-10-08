package app

type settingsWriter interface {
	WriteSettings([]byte) error
}

type settingsDependencies struct {
	HomeDir                func() (string, error)
	ReadPrivate            func(string, int64) ([]byte, error)
	ReadObserved           func(string, int64) ([]byte, error)
	Mutate                 func(func(settingsWriter) error) error
	GuardTrackedSource     func(string) error
	ContextPath            func() (string, error)
	ValidateRepositoryPath func(string, string) (string, error)
	StartAutoSync          func() error
}

type SettingsService struct{ dependencies settingsDependencies }

type Services struct {
	Settings     *SettingsService
	dependencies Dependencies
}

func (s *SettingsService) Load() (AppConfig, error) { return s.loadConfig() }
func (s *SettingsService) Defaults() AppConfig      { return s.defaultConfig() }
func (s *SettingsService) FillDefaults(config AppConfig) AppConfig {
	return s.ensureConfigDefaults(config)
}
func (s *SettingsService) Validate(config AppConfig) error            { return s.validateAppConfig(config) }
func (s *SettingsService) Save(config AppConfig) error                { return s.saveConfig(config) }
func (s *SettingsService) Update(change func(*AppConfig) error) error { return s.updateConfig(change) }

func (s *SettingsService) Path() (string, error) { return s.configPath() }

func (s *SettingsService) Observe() (ObservedConfig, error) { return s.observeConfig() }
