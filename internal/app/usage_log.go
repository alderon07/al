package app

import (
	"fmt"

	"github.com/alderon07/al/internal/usagelog"
)

func (svc *Services) recordAliasUse(name string) error {
	if !aliasName.MatchString(name) {
		return fmt.Errorf("invalid alias name %q", name)
	}
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return err
	}
	return svc.withMutation(func(session *mutationSession) error {
		if _, e := session.dataRoot(); e != nil {
			return e
		}
		return usagelog.RecordUnlocked(home, name, svc.dependencies.Now())
	})
}
