package main

import (
	"testing"
)

func TestFooterConfigCommandsWriteAndResetSettings(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	if err := runConfigCommand([]string{"footer-message", "Built {icon} by Sam"}); err != nil {
		t.Fatal(err)
	}
	if err := runConfigCommand([]string{"footer-icon", "spark"}); err != nil {
		t.Fatal(err)
	}
	config, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Footer.Message != "Built {icon} by Sam" || config.Footer.Icon != "spark" {
		t.Fatalf("saved footer config = %#v", config.Footer)
	}
	if err := runConfigCommand([]string{"footer-reset"}); err != nil {
		t.Fatal(err)
	}
	config, err = loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if config.Footer != defaultFooterConfig() {
		t.Fatalf("reset footer config = %#v", config.Footer)
	}
}
