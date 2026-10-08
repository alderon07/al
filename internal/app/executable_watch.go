package app

import (
	"errors"
	"os"
)

type ExecutableWatch struct {
	path string
	info os.FileInfo
}

func (svc *Services) watchRunningExecutable() ExecutableWatch {
	path, err := svc.dependencies.Executable()
	if err != nil {
		return ExecutableWatch{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return ExecutableWatch{}
	}
	return ExecutableWatch{path: path, info: info}
}

func (watch ExecutableWatch) Changed() bool {
	if watch.path == "" || watch.info == nil {
		return false
	}
	current, err := os.Stat(watch.path)
	if errors.Is(err, os.ErrNotExist) {
		return true
	}
	if err != nil {
		return false
	}
	return !os.SameFile(watch.info, current)
}
