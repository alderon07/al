package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "alias-lens/cmd/alias-lens/internal/tea"
)

func TestContextRankingKeepsTextRelevanceAndBoostsSuggestions(t *testing.T) {
	project := filepath.Join(t.TempDir(), "project")
	working := workingContext{Directory: filepath.Join(project, "src"), Repository: project}
	local := Alias{Name: "deploy", Command: "go run ./deploy", Type: "alias"}
	global := Alias{Name: "depot", Command: "echo depot", Type: "alias"}
	ranking := contextRanking{
		Shell:   "bash",
		Working: working,
		bindings: map[string][]contextBinding{
			aliasContextKey("bash", local): {{Shell: "bash", Name: local.Name, Type: "alias", CommandSHA256: commandDigest(local.Command), Kind: contextRepository, Path: project}},
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
	ranking.bindings[aliasContextKey("bash", dangerous)] = []contextBinding{{Shell: "bash", Name: dangerous.Name, Type: "alias", CommandSHA256: commandDigest(dangerous.Command), Kind: contextRepository, Path: project}}
	for _, suggestion := range suggestedAliasesForContext(append(aliases, dangerous), ranking) {
		if suggestion.Name == "wipe" {
			t.Fatal("dangerous alias entered default suggestions through a context mark")
		}
	}
	if got := filterAliasesForContext(aliases, "dep", ranking); got[0].Name != "deploy" {
		t.Fatalf("equally relevant search match = %s, want deploy", got[0].Name)
	}
	if got := filterAliasesForContext(aliases, "depot", ranking); got[0].Name != "depot" {
		t.Fatalf("exact global match lost to project alias: %s", got[0].Name)
	}
	if got := filterAliasesForContext(aliases, "d", ranking); got[0].Name != "deploy" || len(got) != 2 {
		t.Fatalf("single-letter contextual search = %#v", got)
	}
	if got := filterAliasesForContext(aliases, "dep", contextRanking{}); got[0].Name != "depot" {
		t.Fatalf("global ordering unexpectedly changed: %#v", got)
	}
	outside := ranking
	outside.Working = workingContext{Directory: t.TempDir()}
	if outside.match(local) != 0 {
		t.Fatal("project alias was boosted outside its project")
	}
	directoryOnly := ranking
	directoryOnly.bindings = map[string][]contextBinding{
		aliasContextKey("bash", local): {{Shell: "bash", Name: local.Name, Type: "alias", CommandSHA256: commandDigest(local.Command), Kind: contextDirectory, Path: working.Directory}},
	}
	if directoryOnly.match(local) != 2 {
		t.Fatal("exact folder mark did not outrank project mark")
	}
	directoryOnly.Working.Directory = filepath.Join(project, "other")
	if directoryOnly.match(local) != 0 {
		t.Fatal("folder mark matched a sibling directory")
	}
	changed := local
	changed.Command = "go run ./other"
	if ranking.match(changed) != 0 {
		t.Fatal("edited command inherited an old context mark")
	}
	unsupported := local
	unsupported.Platforms = []string{"unavailable-platform"}
	if ranking.match(unsupported) != 0 {
		t.Fatal("unsupported platform received a context boost")
	}
}

func TestContextAssociationStaysPrivateAndCanBeRemoved(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	alias := Alias{Name: "deploy", Command: "printf 'private command'", Type: "alias"}
	project := filepath.Join(t.TempDir(), "project")
	if err := addContextBinding(alias, "bash", contextRepository, project); err != nil {
		t.Fatal(err)
	}
	path, err := contextPath()
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
	if err := validateTrackedFileConfig(TrackedFileConfig{Source: path, RepositoryPath: "contexts.json"}); err == nil {
		t.Fatal("private context marks could be enrolled for synchronization")
	}
	ranking, err := loadContextRanking("bash", workingContext{Directory: project, Repository: project})
	if err != nil || ranking.match(alias) != 1 {
		t.Fatalf("saved context did not match: rank=%d error=%v", ranking.match(alias), err)
	}
	alias.Command = "printf 'edited command'"
	if err := addContextBinding(alias, "bash", contextRepository, project); err != nil {
		t.Fatal(err)
	}
	saved, err := loadContextFile(path)
	if err != nil || len(saved.Bindings) != 1 || saved.Bindings[0].CommandSHA256 != commandDigest(alias.Command) {
		t.Fatalf("edited alias kept stale association: %#v, %v", saved, err)
	}
	removed, err := removeContextBindings("bash", alias.Name, "", "")
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
	working, err := contextForDirectory(folder)
	if err != nil || working.Repository != inner {
		t.Fatalf("nearest Git root = %q, %v", working.Repository, err)
	}
	kind, path, err := contextTarget(working, "auto")
	if err != nil || kind != contextRepository || path != inner {
		t.Fatalf("default context = %s %s, %v", kind, path, err)
	}
	kind, path, err = contextTarget(working, contextDirectory)
	if err != nil || kind != contextDirectory || path != folder {
		t.Fatalf("exact folder context = %s %s, %v", kind, path, err)
	}
	if _, _, err := contextTarget(workingContext{Directory: parent}, contextRepository); err == nil {
		t.Fatal("repository target succeeded outside Git")
	}
}

func TestContextFileRejectsUnsafeOrUnknownData(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path, err := contextPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"bindings":[],"unexpected":true}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadContextFile(path); err == nil {
		t.Fatal("unknown context data was accepted")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"version":1,"bindings":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := loadContextFile(path); err == nil {
		t.Fatal("duplicate context fields were accepted")
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := loadContextFile(path); err == nil {
		t.Fatal("public context file was accepted")
	}
}

func TestTUIContextShortcutTogglesSelectedAlias(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	project := t.TempDir()
	alias := Alias{Name: "deploy", Command: "go run ./deploy", Type: "alias"}
	ranking, err := loadContextRanking("bash", workingContext{Directory: project, Repository: project})
	if err != nil {
		t.Fatal(err)
	}
	m := model{aliases: []Alias{alias}, context: ranking, width: 90, height: 24, shortcutProfile: shortcutLinux}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	marked := updated.(model)
	if marked.context.match(alias) != 1 || !strings.Contains(marked.View(), "HERE") {
		t.Fatalf("TUI did not show context mark: %s", marked.View())
	}
	updated, _ = marked.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if updated.(model).context.match(alias) != 0 {
		t.Fatal("TUI shortcut did not remove context mark")
	}
}

func TestTUIContextShortcutPrefersActiveDirectoryMark(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	project := t.TempDir()
	folder := filepath.Join(project, "src")
	if err := os.Mkdir(folder, 0o700); err != nil {
		t.Fatal(err)
	}
	alias := Alias{Name: "deploy", Command: "go run ./deploy", Type: "alias"}
	if err := addContextBinding(alias, "bash", contextRepository, project); err != nil {
		t.Fatal(err)
	}
	if err := addContextBinding(alias, "bash", contextDirectory, folder); err != nil {
		t.Fatal(err)
	}
	ranking, err := loadContextRanking("bash", workingContext{Directory: folder, Repository: project})
	if err != nil {
		t.Fatal(err)
	}
	m := model{aliases: []Alias{alias}, context: ranking, width: 90, height: 24, shortcutProfile: shortcutLinux}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	withoutFolder := updated.(model)
	if withoutFolder.context.hasBinding(alias, contextDirectory, folder) || !withoutFolder.context.hasBinding(alias, contextRepository, project) {
		t.Fatal("TUI shortcut did not remove the active folder mark while preserving the project mark")
	}
	updated, _ = withoutFolder.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	withoutProject := updated.(model)
	if withoutProject.context.match(alias) != 0 {
		t.Fatal("TUI shortcut did not remove the remaining project mark")
	}
}

func TestTUIContextShortcutRejectsDuplicateNames(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	alias := Alias{Name: "same", Command: "echo one", Type: "alias"}
	m := model{
		aliases: []Alias{alias, {Name: "same", Command: "echo two", Type: "alias"}},
		context: contextRanking{Shell: "bash", Working: workingContext{Directory: t.TempDir()}},
	}
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyCtrlB})
	if !strings.Contains(updated.(model).status, "Duplicate alias name") {
		t.Fatalf("duplicate name was marked: %q", updated.(model).status)
	}
	path, _ := contextPath()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("duplicate mark wrote context data: %v", err)
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

func TestContextCommandAndSearchUseCurrentProject(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias depot='echo depot'\nalias deploy='go run ./deploy'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	project := filepath.Join(t.TempDir(), "project")
	if err := os.MkdirAll(filepath.Join(project, "src"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(project, ".git"), 0o700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(filepath.Join(project, "src"))
	if err := runContextCommand([]string{"add", "deploy"}); err != nil {
		t.Fatal(err)
	}
	marked := captureSearchOutput(t, []string{"dep"})
	global := captureSearchOutput(t, []string{"--global", "dep"})
	if !strings.HasPrefix(marked, "deploy\t") || !strings.HasPrefix(global, "depot\t") {
		t.Fatalf("context search order = %q; global order = %q", marked, global)
	}
	var jsonResults []Alias
	jsonOutput := captureSearchOutput(t, []string{"--json", "dep"})
	if err := json.Unmarshal([]byte(jsonOutput), &jsonResults); err != nil || len(jsonResults) != 2 || jsonResults[0].Name != "deploy" {
		t.Fatalf("context JSON results = %#v, %v", jsonResults, err)
	}
	if strings.Contains(jsonOutput, `"context"`) || strings.Contains(jsonOutput, `"repository"`) || strings.Contains(jsonOutput, `"directory"`) {
		t.Fatalf("search JSON exposed local context: %s", jsonOutput)
	}
	if err := runContextCommand([]string{"remove", "deploy"}); err != nil {
		t.Fatal(err)
	}
	if got := captureSearchOutput(t, []string{"dep"}); got != global {
		t.Fatalf("search order after removing mark = %q, want %q", got, global)
	}
}

func captureSearchOutput(t *testing.T, arguments []string) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	previous := os.Stdout
	os.Stdout = writer
	defer func() {
		os.Stdout = previous
		reader.Close()
	}()
	searchErr := runSearchCommand(arguments)
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if searchErr != nil {
		t.Fatal(searchErr)
	}
	contents := new(bytes.Buffer)
	if _, err := contents.ReadFrom(reader); err != nil {
		t.Fatal(err)
	}
	return contents.String()
}
