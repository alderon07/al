package app

import (
	"testing"
)

func TestMissingDefaultsSkipExistingNamesAndCommands(t *testing.T) {
	t.Setenv("HOME", privateTestHome(t))
	contents := []byte("alias gs='git status'\nalias stage='git add'\n")
	missing := missingDefaultAliases(contents)
	for _, candidate := range missing {
		if candidate.Name == "gs" {
			t.Fatal("default replaced an existing alias name")
		}
		if candidate.Name == "ga" {
			t.Fatal("default duplicated an existing command")
		}
	}
}
