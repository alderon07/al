package main

import (
	"errors"
	"os"
)

type executableWatch struct {
	path string
	info os.FileInfo
}

func watchRunningExecutable() executableWatch {
	path, err := os.Executable()
	if err != nil {
		return executableWatch{}
	}
	info, err := os.Stat(path)
	if err != nil {
		return executableWatch{}
	}
	return executableWatch{path: path, info: info}
}

func (watch executableWatch) changed() bool {
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
