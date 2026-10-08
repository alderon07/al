package app

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"

	"strings"
	"testing"
)

func TestContextRankingKeepsTextRelevanceAndBoostsSuggestions(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	working := workingContext{Directory: filepath.Join(project, "src"), Repository: project}
	local := Alias{Name: "deploy", Command: "go run ./deploy", Type: "alias"}
	global := Alias{Name: "depot", Command: "echo depot", Type: "alias"}
	ranking := ContextRanking{
		Shell:   "bash",
		Working: working,
		bindings: map[string][]contextBinding{
			aliasContextKey("bash", local): {{Shell: "bash", Name: local.Name, Type: "alias", CommandSHA256: commandDigest(local.Command), Kind: ContextRepository, Path: project}},
		},
	}
	aliases := []Alias{global, local}
	if got := suggestedAliasesForContext(aliases, ranking); got[0].Name != "deploy" {
		t.Fatalf("project suggestion = %s, want deploy", got[0].Name)
	}
	favorite := Alias{Name: "fav", Command: "echo favorite", Type: "alias", Favorite: true}
	if got := suggestedAliasesForContext(append(aliases, favorite), ranking); got[0].Name != "fav" {
		t.Fatalf("favorite lost precedence to project mark: %#v", got)
	}
	dangerous := Alias{Name: "wipe", Command: "git reset --hard", Type: "alias"}
	ranking.bindings[aliasContextKey("bash", dangerous)] = []contextBinding{{Shell: "bash", Name: dangerous.Name, Type: "alias", CommandSHA256: commandDigest(dangerous.Command), Kind: ContextRepository, Path: project}}
	for _, suggestion := range suggestedAliasesForContext(append(aliases, dangerous), ranking) {
		if suggestion.Name == "wipe" {
			t.Fatal("dangerous alias entered default suggestions through a context mark")
		}
	}
	if got := DefaultServices().filterAliasesForContext(aliases, "dep", ranking); got[0].Name != "deploy" {
		t.Fatalf("equally relevant search match = %s, want deploy", got[0].Name)
	}
	if got := DefaultServices().filterAliasesForContext(aliases, "depot", ranking); got[0].Name != "depot" {
		t.Fatalf("exact global match lost to project alias: %s", got[0].Name)
	}
	if got := DefaultServices().filterAliasesForContext(aliases, "d", ranking); got[0].Name != "deploy" || len(got) != 2 {
		t.Fatalf("single-letter contextual search = %#v", got)
	}
	if got := DefaultServices().filterAliasesForContext(aliases, "dep", ContextRanking{}); got[0].Name != "depot" {
		t.Fatalf("global ordering unexpectedly changed: %#v", got)
	}
	outside := ranking
	outside.Working = workingContext{Directory: t.TempDir()}
	if outside.Match(local) != 0 {
		t.Fatal("project alias was boosted outside its project")
	}
	directoryOnly := ranking
	directoryOnly.bindings = map[string][]contextBinding{
		aliasContextKey("bash", local): {{Shell: "bash", Name: local.Name, Type: "alias", CommandSHA256: commandDigest(local.Command), Kind: ContextDirectory, Path: working.Directory}},
	}
	if directoryOnly.Match(local) != 2 {
		t.Fatal("exact folder mark did not outrank project mark")
	}
	directoryOnly.Working.Directory = filepath.Join(project, "other")
	if directoryOnly.Match(local) != 0 {
		t.Fatal("folder mark matched a sibling directory")
	}
	changed := local
	changed.Command = "go run ./other"
	if ranking.Match(changed) != 0 {
		t.Fatal("edited command inherited an old context mark")
	}
	unsupported := local
	unsupported.Platforms = []string{"unavailable-platform"}
	if ranking.Match(unsupported) != 0 {
		t.Fatal("unsupported platform received a context boost")
	}
}

func TestContextAssociationStaysPrivateAndCanBeRemoved(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	alias := Alias{Name: "deploy", Command: "printf 'private command'", Type: "alias"}
	project := filepath.Join(t.TempDir(), "project")
	if err := DefaultServices().addContextBinding(alias, "bash", ContextRepository, project); err != nil {
		t.Fatal(err)
	}
	path, err := DefaultServices().contextPath()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("context file permissions = %v, %v", info, err)
	}
	parent, err := os.Stat(filepath.Dir(path))
	if err != nil || parent.Mode().Perm() != 0o700 {
		t.Fatalf("context directory permissions = %v, %v", parent, err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(contents, []byte(alias.Command)) {
		t.Fatal("context file stored command text")
	}
	if err := DefaultServices().validateTrackedFileConfig(TrackedFileConfig{Source: path, RepositoryPath: "contexts.json"}); err == nil {
		t.Fatal("private context marks could be enrolled for synchronization")
	}
	ranking, err := DefaultServices().loadContextRanking("bash", workingContext{Directory: project, Repository: project})
	if err != nil || ranking.Match(alias) != 1 {
		t.Fatalf("saved context did not match: rank=%d error=%v", ranking.Match(alias), err)
	}
	alias.Command = "printf 'edited command'"
	if err := DefaultServices().addContextBinding(alias, "bash", ContextRepository, project); err != nil {
		t.Fatal(err)
	}
	saved, err := DefaultServices().loadContextFile(path)
	if err != nil || len(saved.Bindings) != 1 || saved.Bindings[0].CommandSHA256 != commandDigest(alias.Command) {
		t.Fatalf("edited alias kept stale association: %#v, %v", saved, err)
	}
	removed, err := DefaultServices().removeContextBindings("bash", alias.Name, "", "")
	if err != nil || removed != 1 {
		t.Fatalf("removed %d associations: %v", removed, err)
	}
}

func TestContextUsesNearestGitRootAndExactFolder(t *testing.T) {
	parent := t.TempDir()
	outer := filepath.Join(parent, "outer")
	inner := filepath.Join(outer, "nested", "inner")
	folder := filepath.Join(inner, "src")
	if err := os.MkdirAll(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(outer, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(inner, ".git"), []byte("gitdir: elsewhere\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	canonicalInner, err := filepath.EvalSymlinks(inner)
	if err != nil {
		t.Fatal(err)
	}
	canonicalFolder, err := filepath.EvalSymlinks(folder)
	if err != nil {
		t.Fatal(err)
	}
	working, err := contextForDirectory(folder)
	if err != nil || working.Repository != canonicalInner {
		t.Fatalf("nearest Git root = %q, %v", working.Repository, err)
	}
	kind, path, err := contextTarget(working, "auto")
	if err != nil || kind != ContextRepository || path != canonicalInner {
		t.Fatalf("default context = %s %s, %v", kind, path, err)
	}
	kind, path, err = contextTarget(working, ContextDirectory)
	if err != nil || kind != ContextDirectory || path != canonicalFolder {
		t.Fatalf("exact folder context = %s %s, %v", kind, path, err)
	}
	if _, _, err := contextTarget(workingContext{Directory: parent}, ContextRepository); err == nil {
		t.Fatal("repository target succeeded outside Git")
	}
}

func TestContextFileRejectsUnsafeOrUnknownData(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	path, err := DefaultServices().contextPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"bindings":[],"unexpected":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().loadContextFile(path); err == nil {
		t.Fatal("unknown context data was accepted")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"version":1,"bindings":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().loadContextFile(path); err == nil {
		t.Fatal("duplicate context fields were accepted")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().loadContextFile(path); err == nil {
		t.Fatal("public context file was accepted")
	}
}

func TestContextDataDoesNotEnterAliasJSON(t *testing.T) {
	alias := Alias{Name: "deploy", Command: "go run ./deploy", Type: "alias"}
	contents, err := json.Marshal(alias)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "context") || strings.Contains(string(contents), "directory") || strings.Contains(string(contents), "repository") {
		t.Fatalf("alias JSON exposed context: %s", contents)
	}
}
