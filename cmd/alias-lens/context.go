package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"alias-lens/internal/transaction"
)

const (
	contextFileVersion = 1
	contextFileLimit   = 1 << 20
	contextMaxBindings = 10000
	contextRepository  = "repository"
	contextDirectory   = "directory"
)

var contextDigestPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type contextBinding struct {
	Shell         string `json:"shell"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	CommandSHA256 string `json:"command_sha256"`
	Kind          string `json:"kind"`
	Path          string `json:"path"`
}

type contextFile struct {
	Version  int              `json:"version"`
	Bindings []contextBinding `json:"bindings"`
}

type workingContext struct {
	Directory  string
	Repository string
}

type contextRanking struct {
	Shell    string
	Working  workingContext
	bindings map[string][]contextBinding
}

func contextPath() (string, error) {
	config, err := configPath()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(config), "contexts.json"), nil
}

func commandDigest(command string) string {
	sum := sha256.Sum256([]byte(command))
	return hex.EncodeToString(sum[:])
}

func aliasContextKey(shell string, alias Alias) string {
	entryType := alias.Type
	if entryType == "" {
		entryType = "alias"
	}
	return shell + "\x00" + entryType + "\x00" + alias.Name + "\x00" + commandDigest(alias.Command)
}

func bindingKey(binding contextBinding) string {
	return binding.Shell + "\x00" + binding.Type + "\x00" + binding.Name + "\x00" + binding.CommandSHA256
}

func contextForDirectory(directory string) (workingContext, error) {
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return workingContext{}, err
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return workingContext{}, err
	}
	current := workingContext{Directory: filepath.Clean(canonical)}
	for candidate := current.Directory; ; candidate = filepath.Dir(candidate) {
		marker, err := os.Lstat(filepath.Join(candidate, ".git"))
		if err == nil && (marker.IsDir() || marker.Mode().IsRegular()) {
			current.Repository = candidate
			break
		}
		if candidate == filepath.Dir(candidate) {
			break
		}
	}
	return current, nil
}

func currentWorkingContext() (workingContext, error) {
	directory, err := os.Getwd()
	if err != nil {
		return workingContext{}, err
	}
	return contextForDirectory(directory)
}

func loadContextFile(path string) (contextFile, error) {
	contents, err := readObservedPrivateFile(path, contextFileLimit)
	if errors.Is(err, os.ErrNotExist) {
		return contextFile{Version: contextFileVersion, Bindings: []contextBinding{}}, nil
	}
	if err != nil {
		return contextFile{}, fmt.Errorf("read private context associations: %w", err)
	}
	var saved contextFile
	if err := decodeUniqueJSON(contents, &saved); err != nil {
		return contextFile{}, fmt.Errorf("parse private context associations: %w", err)
	}
	if saved.Version != contextFileVersion || saved.Bindings == nil || len(saved.Bindings) > contextMaxBindings {
		return contextFile{}, fmt.Errorf("unsupported or oversized private context associations")
	}
	seen := make(map[string]bool, len(saved.Bindings))
	for _, binding := range saved.Bindings {
		if (binding.Shell != "bash" && binding.Shell != "zsh") || !aliasName.MatchString(binding.Name) ||
			(binding.Type != "alias" && binding.Type != "function") || !contextDigestPattern.MatchString(binding.CommandSHA256) ||
			(binding.Kind != contextRepository && binding.Kind != contextDirectory) || !filepath.IsAbs(binding.Path) ||
			binding.Path != filepath.Clean(binding.Path) || strings.ContainsRune(binding.Path, 0) {
			return contextFile{}, fmt.Errorf("private context associations contain an invalid entry")
		}
		key := bindingKey(binding) + "\x00" + binding.Kind + "\x00" + binding.Path
		if seen[key] {
			return contextFile{}, fmt.Errorf("private context associations contain a duplicate entry")
		}
		seen[key] = true
	}
	return saved, nil
}

func loadContextRanking(shell string, working workingContext) (contextRanking, error) {
	path, err := contextPath()
	if err != nil {
		return contextRanking{}, err
	}
	saved, err := loadContextFile(path)
	if err != nil {
		return contextRanking{}, err
	}
	ranking := contextRanking{Shell: shell, Working: working, bindings: make(map[string][]contextBinding)}
	for _, binding := range saved.Bindings {
		ranking.bindings[bindingKey(binding)] = append(ranking.bindings[bindingKey(binding)], binding)
	}
	return ranking, nil
}

func currentContextRanking() (contextRanking, error) {
	working, err := currentWorkingContext()
	if err != nil {
		return contextRanking{}, err
	}
	return loadContextRanking(activeShellAdapter().Name(), working)
}

func (ranking contextRanking) match(alias Alias) int {
	if !platformSupported(alias.Platforms) {
		return 0
	}
	best := 0
	for _, binding := range ranking.bindings[aliasContextKey(ranking.Shell, alias)] {
		switch {
		case binding.Kind == contextDirectory && binding.Path == ranking.Working.Directory:
			return 2
		case binding.Kind == contextRepository && binding.Path == ranking.Working.Repository:
			best = 1
		}
	}
	return best
}

func (ranking contextRanking) hasBinding(alias Alias, kind, path string) bool {
	for _, binding := range ranking.bindings[aliasContextKey(ranking.Shell, alias)] {
		if binding.Kind == kind && binding.Path == path {
			return true
		}
	}
	return false
}

func contextTarget(working workingContext, requested string) (string, string, error) {
	if working.Directory == "" {
		return "", "", fmt.Errorf("current directory is unavailable; reopen Alias Lens from a directory")
	}
	switch requested {
	case "", "auto":
		if working.Repository != "" {
			return contextRepository, working.Repository, nil
		}
		return contextDirectory, working.Directory, nil
	case contextRepository:
		if working.Repository == "" {
			return "", "", fmt.Errorf("this directory is outside a Git repository; use --directory")
		}
		return contextRepository, working.Repository, nil
	case contextDirectory:
		return contextDirectory, working.Directory, nil
	default:
		return "", "", fmt.Errorf("choose --repo or --directory")
	}
}

func contextToggleTarget(ranking contextRanking, alias Alias) (string, string, error) {
	working := ranking.Working
	if working.Directory != "" && ranking.hasBinding(alias, contextDirectory, working.Directory) {
		return contextDirectory, working.Directory, nil
	}
	return contextTarget(working, "auto")
}

func updateContextFile(change func(*contextFile) (bool, error)) error {
	path, err := contextPath()
	if err != nil {
		return err
	}
	directory := filepath.Dir(path)
	if err := ensurePrivateDirectory(directory); err != nil {
		return err
	}
	lock, err := transaction.AcquireLock(directory, filepath.Join(directory, "contexts.lock"))
	if err != nil {
		return fmt.Errorf("change context associations: %w", err)
	}
	defer lock.Close()
	saved, err := loadContextFile(path)
	if err != nil {
		return err
	}
	changed, err := change(&saved)
	if err != nil || !changed {
		return err
	}
	if len(saved.Bindings) > contextMaxBindings {
		return fmt.Errorf("context associations exceed %d entries", contextMaxBindings)
	}
	sort.Slice(saved.Bindings, func(i, j int) bool {
		left, right := saved.Bindings[i], saved.Bindings[j]
		return left.Shell+"\x00"+left.Name+"\x00"+left.Kind+"\x00"+left.Path < right.Shell+"\x00"+right.Name+"\x00"+right.Kind+"\x00"+right.Path
	})
	contents, err := json.MarshalIndent(saved, "", "  ")
	if err != nil {
		return err
	}
	if len(contents) > contextFileLimit {
		return fmt.Errorf("context associations exceed %d bytes", contextFileLimit)
	}
	return writeFileAtomically(path, append(contents, '\n'), 0o600)
}

func addContextBinding(alias Alias, shell, kind, path string) error {
	binding := contextBinding{Shell: shell, Name: alias.Name, Type: alias.Type, CommandSHA256: commandDigest(alias.Command), Kind: kind, Path: path}
	if binding.Type == "" {
		binding.Type = "alias"
	}
	return updateContextFile(func(saved *contextFile) (bool, error) {
		kept := saved.Bindings[:0]
		for _, existing := range saved.Bindings {
			if existing == binding {
				return false, nil
			}
			if existing.Shell == shell && existing.Name == alias.Name && existing.Kind == kind && existing.Path == path {
				continue
			}
			kept = append(kept, existing)
		}
		saved.Bindings = append(kept, binding)
		return true, nil
	})
}

func removeContextBindings(shell, name, kind, path string) (int, error) {
	removed := 0
	err := updateContextFile(func(saved *contextFile) (bool, error) {
		kept := saved.Bindings[:0]
		for _, binding := range saved.Bindings {
			if binding.Shell == shell && binding.Name == name && (kind == "" || (binding.Kind == kind && binding.Path == path)) {
				removed++
				continue
			}
			kept = append(kept, binding)
		}
		saved.Bindings = kept
		return removed > 0, nil
	})
	return removed, err
}

func uniqueAliasByName(aliases []Alias, name string) (Alias, error) {
	var found Alias
	count := 0
	for _, alias := range aliases {
		if alias.Name == name {
			found = alias
			count++
		}
	}
	if count == 0 {
		return Alias{}, fmt.Errorf("alias %q was not found; run al search %s", name, name)
	}
	if count > 1 {
		return Alias{}, fmt.Errorf("alias %q has duplicate definitions; run al check", name)
	}
	return found, nil
}

func runContextCommand(arguments []string) error {
	if len(arguments) == 0 {
		return fmt.Errorf("usage: al context list|add NAME [--repo|--directory]|remove NAME [--all|--repo|--directory]")
	}
	shell := activeShellAdapter().Name()
	switch arguments[0] {
	case "list":
		if len(arguments) != 1 {
			return fmt.Errorf("usage: al context list")
		}
		path, err := contextPath()
		if err != nil {
			return err
		}
		saved, err := loadContextFile(path)
		if err != nil {
			return err
		}
		if len(saved.Bindings) == 0 {
			fmt.Println("No aliases are marked for a project or folder. In one, run: al context add NAME")
			return nil
		}
		for _, binding := range saved.Bindings {
			fmt.Printf("%s\t%s\t%s\t%s\n", terminalSafeText(binding.Shell), terminalSafeText(binding.Name), terminalSafeText(binding.Kind), terminalSafeText(binding.Path))
		}
		return nil
	case "add", "remove":
		if len(arguments) < 2 || len(arguments) > 3 || !aliasName.MatchString(arguments[1]) {
			return fmt.Errorf("usage: al context %s NAME [--repo|--directory%s]", arguments[0], map[bool]string{true: "|--all", false: ""}[arguments[0] == "remove"])
		}
		option := "auto"
		if len(arguments) == 3 {
			switch arguments[2] {
			case "--repo":
				option = contextRepository
			case "--directory":
				option = contextDirectory
			case "--all":
				if arguments[0] != "remove" {
					return fmt.Errorf("--all is only valid with al context remove")
				}
				option = "all"
			default:
				return fmt.Errorf("choose --repo, --directory, or --all")
			}
		}
		name := arguments[1]
		if option == "all" {
			removed, err := removeContextBindings(shell, name, "", "")
			if err != nil {
				return err
			}
			fmt.Printf("Removed %d local context association(s) for %s.\n", removed, name)
			return nil
		}
		working, err := currentWorkingContext()
		if err != nil {
			return err
		}
		kind, target, err := contextTarget(working, option)
		if err != nil {
			return err
		}
		if arguments[0] == "remove" {
			removed, err := removeContextBindings(shell, name, kind, target)
			if err != nil {
				return err
			}
			fmt.Printf("Removed %d local context association(s) for %s.\n", removed, name)
			return nil
		}
		aliases, err := loadAliases()
		if err != nil {
			return err
		}
		alias, err := uniqueAliasByName(aliases, name)
		if err != nil {
			return err
		}
		if err := addContextBinding(alias, shell, kind, target); err != nil {
			return err
		}
		fmt.Printf("Marked %s for this %s. It remains available everywhere.\n", name, map[string]string{contextRepository: "project", contextDirectory: "folder"}[kind])
		return nil
	default:
		return fmt.Errorf("usage: al context list|add NAME [--repo|--directory]|remove NAME [--all|--repo|--directory]")
	}
}
