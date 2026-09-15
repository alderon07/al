package usagelog

import (
	"bufio"
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
	path := Path(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := file.Chmod(0o600); err != nil {
		return err
	}
	_, err = fmt.Fprintf(file, "%d\t%s\n", at.Unix(), name)
	return err
}

func Load(home string) ([]Event, error) {
	file, err := os.Open(Path(home))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var events []Event
	scanner := bufio.NewScanner(file)
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
