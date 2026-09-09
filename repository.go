package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func configureRepository(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if output, err := exec.Command("git", "-C", absolute, "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil {
		return fmt.Errorf("%s is not a Git repository: %s", absolute, strings.TrimSpace(string(output)))
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	config.Repository = absolute
	return saveConfig(config)
}

func syncRepository(push bool) (string, error) {
	config, err := loadConfig()
	if err != nil {
		return "", err
	}
	if config.Repository == "" {
		return "", fmt.Errorf("configure a repo first: al repo /path/to/dotfiles")
	}
	sourcePath, err := aliasesPath()
	if err != nil {
		return "", err
	}
	return syncRepositoryFiles(config, sourcePath, push)
}

func syncRepositoryFiles(config AppConfig, sourcePath string, push bool) (string, error) {
	relative := filepath.Clean(config.AliasFile)
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("alias_file must stay inside the configured repository")
	}
	if output, err := exec.Command("git", "-C", config.Repository, "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil {
		return "", fmt.Errorf("configured repository is unavailable: %s", strings.TrimSpace(string(output)))
	}

	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}
	target := filepath.Join(config.Repository, relative)
	existing, _ := os.ReadFile(target)
	changed := !bytes.Equal(contents, existing)
	if changed {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, contents, 0o644); err != nil {
			return "", err
		}
		if output, err := exec.Command("git", "-C", config.Repository, "add", "--", relative).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git add failed: %s", strings.TrimSpace(string(output)))
		}
		if output, err := exec.Command("git", "-C", config.Repository, "commit", "--only", "-m", "Update shell aliases", "--", relative).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(output)))
		}
	}
	if push {
		if output, err := exec.Command("git", "-C", config.Repository, "push").CombinedOutput(); err != nil {
			return "", fmt.Errorf("git push failed: %s", strings.TrimSpace(string(output)))
		}
		return "Aliases committed and pushed", nil
	}
	if changed {
		return "Aliases committed locally", nil
	}
	return "Repository already matches ~/.bash_aliases", nil
}
