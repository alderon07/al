package usagelog

import (
	"alias-lens/internal/transaction"
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Event struct {
	Name string
	Time time.Time
}

func Path(home string) string {
	return filepath.Join(home, ".local", "share", "alias-lens", "usage.tsv")
}

func Record(home, name string, at time.Time) error {
	for _, path := range []string{filepath.Join(home, ".local"), filepath.Join(home, ".local", "state")} {
		if e := transaction.EnsureWorkflowDirectory(path, false); e != nil {
			return e
		}
	}
	state := filepath.Join(home, ".local", "state", "alias-lens")
	if e := transaction.EnsureWorkflowDirectory(state, true); e != nil {
		return e
	}
	lock, e := transaction.AcquireLock(state, filepath.Join(state, "mutation.lock"))
	if e != nil {
		return e
	}
	defer lock.Close()
	if e = transaction.RecoverWorkflows(state); e != nil {
		return e
	}
	for _, path := range []string{filepath.Join(home, ".local", "share"), filepath.Join(home, ".local", "share", "alias-lens")} {
		if e = transaction.EnsureWorkflowDirectory(path, path == filepath.Dir(Path(home))); e != nil {
			return e
		}
	}
	return RecordUnlocked(home, name, at)
}
func RecordUnlocked(home, name string, at time.Time) error {
	if strings.TrimSpace(name) == "" || strings.ContainsAny(name, "\t\r\n") {
		return fmt.Errorf("invalid usage name")
	}
	return transaction.AppendWorkflowPrivateFile(Path(home), []byte(fmt.Sprintf("%d\t%s\n", at.Unix(), name)))
}

func Load(home string) ([]Event, error) {
	path := Path(home)
	if _, e := os.Lstat(path); errors.Is(e, os.ErrNotExist) {
		return nil, nil
	}
	id, contents, err := transaction.ReadWorkflowTarget(path, transaction.MaxPrivateFileSize)
	if err != nil {
		return nil, err
	}
	if !id.Exists {
		return nil, nil
	}
	if id.Mode != 0o600 {
		return nil, transaction.ErrUnsafePath
	}

	var events []Event
	scanner := bufio.NewScanner(bytes.NewReader(contents))
	for scanner.Scan() {
		stamp, name, found := strings.Cut(scanner.Text(), "\t")
		unix, parseErr := strconv.ParseInt(stamp, 10, 64)
		if !found || parseErr != nil || strings.TrimSpace(name) == "" {
			continue
		}
		events = append(events, Event{Name: name, Time: time.Unix(unix, 0)})
	}
	return events, scanner.Err()
}
