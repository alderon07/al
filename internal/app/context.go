package app

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
)

const (
	contextFileVersion = 1
	contextFileLimit   = 1 << 20
	contextMaxBindings = 10000
	ContextRepository  = "repository"
	ContextDirectory   = "directory"
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

type ContextRanking struct {
	Shell    string
	Working  workingContext
	bindings map[string][]contextBinding
	services *Services
}

func (svc *Services) contextPath() (string, error) {
	config, err := svc.configPath()
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

func (svc *Services) currentWorkingContext() (workingContext, error) {
	directory, err := svc.dependencies.WorkingDir()
	if err != nil {
		return workingContext{}, err
	}
	return contextForDirectory(directory)
}

func (svc *Services) loadContextFile(path string) (contextFile, error) {
	contents, err := svc.readObservedPrivateFile(path, contextFileLimit)
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
			(binding.Kind != ContextRepository && binding.Kind != ContextDirectory) || !filepath.IsAbs(binding.Path) ||
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

func (svc *Services) loadContextRanking(shell string, working workingContext) (ContextRanking, error) {
	path, err := svc.contextPath()
	if err != nil {
		return ContextRanking{services: svc}, err
	}
	saved, err := svc.loadContextFile(path)
	if err != nil {
		return ContextRanking{services: svc}, err
	}
	ranking := ContextRanking{Shell: shell, Working: working, bindings: make(map[string][]contextBinding), services: svc}
	for _, binding := range saved.Bindings {
		ranking.bindings[bindingKey(binding)] = append(ranking.bindings[bindingKey(binding)], binding)
	}
	return ranking, nil
}

func (svc *Services) currentContextRanking() (ContextRanking, error) {
	working, err := svc.currentWorkingContext()
	if err != nil {
		return ContextRanking{services: svc}, err
	}
	return svc.loadContextRanking(svc.activeShellAdapter().Name(), working)
}

func (ranking ContextRanking) Match(alias Alias) int {
	svc := ranking.services
	if svc == nil {
		svc =
			DefaultServices()
	}

	if !svc.platformSupported(alias.Platforms) {
		return 0
	}
	best := 0
	for _, binding := range ranking.bindings[aliasContextKey(ranking.Shell, alias)] {
		switch {
		case binding.Kind == ContextDirectory && binding.Path == ranking.Working.Directory:
			return 2
		case binding.Kind == ContextRepository && binding.Path == ranking.Working.Repository:
			best = 1
		}
	}
	return best
}

func (ranking ContextRanking) HasBinding(alias Alias, kind, path string) bool {
	svc := ranking.services
	if svc == nil {
		svc =
			DefaultServices()
	}

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
			return ContextRepository, working.Repository, nil
		}
		return ContextDirectory, working.Directory, nil
	case ContextRepository:
		if working.Repository == "" {
			return "", "", fmt.Errorf("this directory is outside a Git repository; use --directory")
		}
		return ContextRepository, working.Repository, nil
	case ContextDirectory:
		return ContextDirectory, working.Directory, nil
	default:
		return "", "", fmt.Errorf("choose --repo or --directory")
	}
}

func contextToggleTarget(ranking ContextRanking, alias Alias) (string, string, error) {
	working := ranking.Working
	if working.Directory != "" && ranking.HasBinding(alias, ContextDirectory, working.Directory) {
		return ContextDirectory, working.Directory, nil
	}
	return contextTarget(working, "auto")
}

func (svc *Services) updateContextFile(change func(*contextFile) (bool, error)) error {
	return svc.withMutation(func(session *mutationSession) error { return svc.updateContextFileInSession(session, change) })
}
func (svc *Services) updateContextFileInSession(session *mutationSession, change func(*contextFile) (bool, error)) error {
	path, err := svc.contextPath()
	if err != nil {
		return err
	}
	saved, err := svc.loadContextFile(path)
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
	return session.WritePrivate(path, append(contents, '\n'))
}

func (svc *Services) addContextBinding(alias Alias, shell, kind, path string) error {
	binding := contextBinding{Shell: shell, Name: alias.Name, Type: alias.Type, CommandSHA256: commandDigest(alias.Command), Kind: kind, Path: path}
	if binding.Type == "" {
		binding.Type = "alias"
	}
	return svc.updateContextFile(func(saved *contextFile) (bool, error) {
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

func (svc *Services) removeContextBindings(shell, name, kind, path string) (int, error) {
	removed := 0
	err := svc.updateContextFile(func(saved *contextFile) (bool, error) {
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
