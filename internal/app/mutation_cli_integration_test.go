package app

import (
	"alias-lens/internal/transaction"
	"errors"
	"path/filepath"
	"testing"
)

func TestManagedWritersShareStateLock(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	if e := DefaultServices().withMutation(func(session *mutationSession) error { return nil }); e != nil {
		t.Fatal(e)
	}
	state := filepath.Join(home, ".local", "state", "alias-lens")
	lock, e := transaction.AcquireLock(state, filepath.Join(state, "mutation.lock"))
	if e != nil {
		t.Fatal(e)
	}
	defer lock.Close()
	for name, work := range map[string]func() error{"config": func() error { return DefaultServices().Settings.Save(DefaultServices().Settings.Defaults()) }, "theme": func() error { return DefaultServices().saveTheme(defaultTheme()) }, "tour": DefaultServices().scheduleTour, "usage": func() error { return DefaultServices().recordAliasUse("fixture") }, "context": func() error {
		return DefaultServices().updateContextFile(func(*contextFile) (bool, error) { return false, nil })
	}, "clear": func() error { return DefaultServices().ClearUsageData() }} {
		if e := work(); !errors.Is(e, transaction.ErrLocked) {
			t.Fatalf("%s ignored state lock", name)
		}
	}
}
