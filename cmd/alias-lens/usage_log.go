package main

import (
	"fmt"
	"os"
	"time"

	"alias-lens/cmd/alias-lens/internal/usagelog"
)

func recordAliasUse(name string) error {
	if !aliasName.MatchString(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	return usagelog.Record(home, name, time.Now())
}
