package app

import (
	"fmt"

	"testing"
)

func TestValidateAppConfigChecksProfiles(t *testing.T) {
	settings := &SettingsService{dependencies: settingsDependencies{ValidateRepositoryPath: func(path, field string) (string, error) { return path, nil }}}
	tests := [][]string{
		{"Work"},
		{"work", "work"},
		{"work", "laptop"},
	}
	tooMany := make([]string, 33)
	for index := range tooMany {
		tooMany[index] = fmt.Sprintf("p%02d", index)
	}
	tests = append(tests, tooMany)
	for _, profiles := range tests {
		config := settings.defaultConfig()
		config.Profiles = profiles
		if err := settings.validateAppConfig(config); err == nil {
			t.Errorf("invalid profiles were accepted: %#v", profiles)
		}
	}
	config := settings.defaultConfig()
	config.Profiles = []string{"laptop", "work"}
	if err := settings.validateAppConfig(config); err != nil {
		t.Fatalf("valid profiles were rejected: %v", err)
	}
}
