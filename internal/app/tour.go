package app

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const (
	tourPending = "pending"
	tourSeen    = "seen"
)

func (svc *Services) tourStatePath() (string, error) {
	home, e := svc.dependencies.HomeDir()
	if e != nil {
		return "", e
	}
	return filepath.Join(home, ".local", "state", "alias-lens", "tour-state"), nil
}

func (svc *Services) scheduleTour() error { return svc.withMutation(svc.scheduleTourInSession) }
func (svc *Services) scheduleTourInSession(session *mutationSession) error {
	path, e := svc.tourStatePath()
	if e != nil {
		return e
	}
	state, e := readManagedPrivateFile(path, 1024)
	if e == nil && strings.TrimSpace(string(state)) == tourSeen {
		return nil
	}
	if e != nil && !errors.Is(e, os.ErrNotExist) {
		return e
	}
	return session.WritePrivate(path, []byte(tourPending+"\n"))
}

func (svc *Services) tourShouldShow() bool {
	path, err := svc.tourStatePath()
	if err != nil {
		return false
	}
	state, err := readManagedPrivateFile(path, 1024)
	return err == nil && strings.TrimSpace(string(state)) == tourPending
}

func (svc *Services) markTourSeen() error {
	return svc.withMutation(func(session *mutationSession) error {
		path, e := svc.tourStatePath()
		if e != nil {
			return e
		}
		return session.WritePrivate(path, []byte(tourSeen+"\n"))
	})
}
