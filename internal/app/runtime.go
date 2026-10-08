package app

import (
	"context"
	"net/http"
	"os"
	"time"
)

type Dependencies struct {
	HomeDir          func() (string, error)
	WorkingDir       func() (string, error)
	Environment      func(string) string
	Executable       func() (string, error)
	Now              func() time.Time
	Transport        http.RoundTripper
	FindTrustedShell func(string) (string, error)
	ValidateSyntax   func(context.Context, string, []byte) error
	StartWatcher     func() error
	Notice           func(OperationNotice)
}

func DefaultServices() *Services { return NewServices(Dependencies{}) }
func NewServices(dependencies Dependencies) *Services {
	if dependencies.HomeDir == nil {
		dependencies.HomeDir = os.UserHomeDir
	}
	if dependencies.WorkingDir == nil {
		dependencies.WorkingDir = os.Getwd
	}
	if dependencies.Environment == nil {
		dependencies.Environment = os.Getenv
	}
	if dependencies.Executable == nil {
		dependencies.Executable = os.Executable
	}
	if dependencies.Now == nil {
		dependencies.Now = time.Now
	}
	if dependencies.Transport == nil {
		dependencies.Transport = catalogProviderTransport
	}
	service := &Services{dependencies: dependencies}
	service.Settings = service.newSettings()
	return service
}
func (svc *Services) newSettings() *SettingsService {
	return &SettingsService{dependencies: settingsDependencies{
		HomeDir:                svc.dependencies.HomeDir,
		ReadPrivate:            readManagedPrivateFile,
		ReadObserved:           svc.readObservedPrivateFile,
		GuardTrackedSource:     svc.rejectCatalogFallbackTracking,
		ContextPath:            svc.contextPath,
		ValidateRepositoryPath: cleanRepositoryRelativePath,
		StartAutoSync:          svc.ensureWatchProcess,
		Mutate: func(work func(settingsWriter) error) error {
			return svc.withMutation(func(session *mutationSession) error { return work(settingsSession{session: session, services: svc}) })
		},
	}}
}
