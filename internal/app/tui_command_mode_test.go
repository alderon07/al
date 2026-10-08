package app

import (
	"testing"
)

func TestSlashCannotReplaceGlobalAction(t *testing.T) {
	config := DefaultServices().defaultConfig()
	config.Shortcuts = map[string]string{"help": "/"}
	if err := DefaultServices().validateAppConfig(config); err == nil {
		t.Fatal("reserved search key was accepted as a global action")
	}
}
