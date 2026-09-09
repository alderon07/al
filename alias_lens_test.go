package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseAliasDefinitionRoundTripsShellQuotes(t *testing.T) {
	command := `awk '{print $1}'`
	name, parsed, ok := parseAliasDefinition("alias first=" + shellQuote(command))
	if !ok || name != "first" || parsed != command {
		t.Fatalf("round trip failed: ok=%v name=%q command=%q", ok, name, parsed)
	}
}

func TestAddAliasPlacesRelatedCommandsTogetherAndCreatesBackup(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".bash_aliases")
	original := "alias gst='git stash'\n\nalias gp='git push'\n"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}

	if err := addAliasToFile(path, "gstc", "git stash clear", "Clear every stash"); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	updated := string(contents)
	if strings.Index(updated, "alias gst=") > strings.Index(updated, "alias gstc=") || strings.Index(updated, "alias gstc=") > strings.Index(updated, "alias gp=") {
		t.Fatalf("related alias was not inserted beside git stash:\n%s", updated)
	}
	backup, err := os.ReadFile(path + ".alias-lens.bak")
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatal("backup did not preserve the original file")
	}
}

func TestWrapTextKeepsDescriptionWithinWidth(t *testing.T) {
	wrapped := wrapText("Long listing view with permissions owner size and date", 18)
	for _, line := range strings.Split(wrapped, "\n") {
		if len([]rune(line)) > 18 {
			t.Fatalf("line %q exceeds width", line)
		}
	}
}

func TestBuiltInThemesCanCycle(t *testing.T) {
	darcula := nextTheme("phosphor")
	dracula := nextTheme(darcula.Preset)
	catppuccin := nextTheme(dracula.Preset)
	if darcula.Preset != "darcula" || dracula.Preset != "dracula" || catppuccin.Preset != "catppuccin" {
		t.Fatalf("unexpected theme cycle: %q, %q, then %q", darcula.Preset, dracula.Preset, catppuccin.Preset)
	}
	if darcula.Background != "#2B2B2B" || darcula.Text != "#A9B7C6" || darcula.Selected != "#214283" {
		t.Fatalf("Darcula preset drifted from JetBrains values: %+v", darcula)
	}
}

func TestOfficialThemePaletteValues(t *testing.T) {
	dracula := builtInTheme("dracula")
	if dracula.Background != "#282A36" || dracula.Text != "#F8F8F2" || dracula.Selected != "#44475A" || dracula.Accent != "#50FA7B" {
		t.Fatalf("Dracula preset drifted from its official palette: %+v", dracula)
	}
	mocha := builtInTheme("catppuccin")
	if mocha.Background != "#1E1E2E" || mocha.Text != "#CDD6F4" || mocha.Panel != "#313244" || mocha.Selected != "#45475A" || mocha.Border != "#585B70" {
		t.Fatalf("Catppuccin Mocha preset drifted from its official palette: %+v", mocha)
	}
}

func TestEditAndDeleteAliasRemainRecoverable(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".bash_aliases")
	original := "# Show status\nalias gs='git status'\n\n# Push changes\nalias gp='git push'\n"
	if err := os.WriteFile(path, []byte(original), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := editAliasInFile(path, "gs", "gss", "git status -sb", "Show concise status"); err != nil {
		t.Fatal(err)
	}
	edited, _ := os.ReadFile(path)
	if !strings.Contains(string(edited), "alias gss='git status -sb'") || strings.Contains(string(edited), "alias gs=") {
		t.Fatalf("unexpected edited file:\n%s", edited)
	}
	if err := deleteAliasFromFile(path, "gss"); err != nil {
		t.Fatal(err)
	}
	deleted, _ := os.ReadFile(path)
	if strings.Contains(string(deleted), "gss") || !strings.Contains(string(deleted), "alias gp=") {
		t.Fatalf("delete changed the wrong content:\n%s", deleted)
	}
	backup, _ := os.ReadFile(path + ".alias-lens.bak")
	if !strings.Contains(string(backup), "alias gss=") {
		t.Fatal("delete backup should contain the last editable version")
	}
}

func TestRankedSearchFindsMeaningAndTypos(t *testing.T) {
	aliases := []Alias{
		{Name: "dps", Command: "docker ps", Description: "List containers"},
		{Name: "gs", Command: "git status -sb", Description: "Show status"},
		{Name: "gb", Command: "git branch", Description: "List branches"},
	}
	if result := filterAliases(aliases, "status"); len(result) == 0 || result[0].Name != "gs" {
		t.Fatalf("semantic search did not rank gs first: %+v", result)
	}
	if result := filterAliases(aliases, "gss"); len(result) == 0 || result[0].Name != "gs" {
		t.Fatalf("fuzzy search did not correct gss: %+v", result)
	}
}

func TestSingleLetterSearchReturnsEveryMatchingPrefix(t *testing.T) {
	aliases := []Alias{
		{Name: "ll", Command: "eza -l"},
		{Name: "gs", Command: "git status"},
		{Name: "gco", Command: "git checkout"},
		{Name: "ag", Command: "silver searcher"},
	}
	result := filterAliases(aliases, "g")
	if len(result) != 2 || result[0].Name != "gs" || result[1].Name != "gco" {
		t.Fatalf("single-letter prefix search returned the wrong aliases: %+v", result)
	}
}

func TestHealthChecksFlagDuplicatesDangerAndMissingTools(t *testing.T) {
	aliases := []Alias{
		{Name: "wipe", Command: "git reset --hard"},
		{Name: "same", Command: "definitely-not-a-real-alias-lens-tool run"},
		{Name: "same", Command: "echo safe"},
	}
	annotateHealth(aliases)
	if len(aliases[0].Issues) == 0 || !strings.Contains(strings.Join(aliases[0].Issues, " "), "review") {
		t.Fatal("dangerous command was not flagged")
	}
	if !strings.Contains(strings.Join(aliases[1].Issues, " "), "missing executable") || !strings.Contains(strings.Join(aliases[1].Issues, " "), "duplicate") {
		t.Fatal("missing executable and duplicate were not flagged")
	}
}

func TestRepositorySyncCommitsOnlyAliasFile(t *testing.T) {
	directory := t.TempDir()
	repository := filepath.Join(directory, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	runGit(t, repository, "config", "user.email", "alias-lens@example.test")
	runGit(t, repository, "config", "user.name", "Alias Lens Test")
	unrelated := filepath.Join(repository, "editor.conf")
	if err := os.WriteFile(unrelated, []byte("keep me\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "add", "editor.conf")
	runGit(t, repository, "commit", "-m", "Add unrelated config")
	source := filepath.Join(directory, ".bash_aliases")
	if err := os.WriteFile(source, []byte("alias gs='git status'\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	message, err := syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: "shell/.bash_aliases"}, source, false)
	if err != nil || message != "Aliases committed locally" {
		t.Fatalf("sync failed: %q %v", message, err)
	}
	if contents, _ := os.ReadFile(unrelated); string(contents) != "keep me\n" {
		t.Fatal("sync modified an unrelated config file")
	}
	changed := strings.TrimSpace(runGit(t, repository, "show", "--pretty=", "--name-only", "HEAD"))
	if changed != "shell/.bash_aliases" {
		t.Fatalf("sync commit contained unrelated paths: %q", changed)
	}
}

func runGit(t *testing.T, directory string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", directory}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v failed: %v\n%s", args, err, output)
	}
	return string(output)
}

func TestFilterRemoteRepos(t *testing.T) {
	repositories := []RemoteRepo{
		{Provider: "github", FullName: "naqi/al", Description: "Alias manager"},
		{Provider: "gitlab", FullName: "naqi/dotfiles", Description: "Shell configuration"},
	}

	if got := filterRemoteRepos(repositories, "dot"); len(got) != 1 || got[0].FullName != "naqi/dotfiles" {
		t.Fatalf("name filter returned %#v", got)
	}
	if got := filterRemoteRepos(repositories, "manager"); len(got) != 1 || got[0].FullName != "naqi/al" {
		t.Fatalf("description filter returned %#v", got)
	}
	if got := filterRemoteRepos(repositories, "gitlab"); len(got) != 1 || got[0].FullName != "naqi/dotfiles" {
		t.Fatalf("provider filter returned %#v", got)
	}
	if got := filterRemoteRepos(repositories, ""); len(got) != len(repositories) {
		t.Fatalf("empty filter returned %d repositories", len(got))
	}
}

func TestTrustedNextURLRejectsCredentialRedirects(t *testing.T) {
	if got := trustedNextURL("https://api.bitbucket.org/2.0/example?page=2", "api.bitbucket.org"); got == "" {
		t.Fatal("trusted Bitbucket pagination URL was rejected")
	}
	if got := trustedNextURL("https://evil.example/steal", "api.bitbucket.org"); got != "" {
		t.Fatalf("untrusted pagination URL was accepted: %s", got)
	}
}

func TestLegacyConfigGetsDefaultProviderLayer(t *testing.T) {
	config := ensureConfigDefaults(AppConfig{Repository: "/tmp/dotfiles", AliasFile: "shell/.bash_aliases"})
	github, exists := config.Providers["github"]
	if !exists || !github.Enabled || github.Protocol != "auto" {
		t.Fatalf("legacy configuration was not migrated in memory: %#v", config)
	}
}

func TestHistorySuggestionsUseRepeatedLongCommands(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".bash_history")
	history := "#1700000000\ngo test ./...\ngo test ./...\ngo test ./...\ngit status\n"
	if err := os.WriteFile(path, []byte(history), 0o600); err != nil {
		t.Fatal(err)
	}
	counts, err := historyCountsFrom(path)
	if err != nil {
		t.Fatal(err)
	}
	suggestions := historySuggestions(nil, counts)
	if len(suggestions) != 1 || suggestions[0].Command != "go test ./..." || suggestions[0].Count != 3 {
		t.Fatalf("unexpected history suggestions: %#v", suggestions)
	}
}

func TestSecretScanReportsLocationWithoutValue(t *testing.T) {
	secret := "github_" + "pat_ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	findings := findSecretFindings([]byte("alias safe='git status'\nalias leak='echo " + secret + "'\n"))
	if len(findings) != 1 || findings[0].Line != 2 || findings[0].Kind != "GitHub token" {
		t.Fatalf("unexpected findings: %#v", findings)
	}
	message := secretFindingsError(findings).Error()
	if strings.Contains(message, secret) || !strings.Contains(message, "line 2") {
		t.Fatalf("finding message leaked or omitted context: %s", message)
	}
}

func TestPushWithSecretStopsBeforeRepositoryWrite(t *testing.T) {
	directory := t.TempDir()
	repository := filepath.Join(directory, "dotfiles")
	if err := os.Mkdir(repository, 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, repository, "init")
	source := filepath.Join(directory, ".bash_aliases")
	secret := "ghp_" + "ABCDEFGHIJKLMNOPQRSTUVWXYZ123456"
	if err := os.WriteFile(source, []byte("alias leak='echo "+secret+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := syncRepositoryFiles(AppConfig{Repository: repository, AliasFile: ".bash_aliases"}, source, true)
	if err == nil || !strings.Contains(err.Error(), "push blocked") {
		t.Fatalf("secret push was not blocked: %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(repository, ".bash_aliases")); !os.IsNotExist(statErr) {
		t.Fatal("blocked push wrote the alias file into the repository")
	}
}

func TestFunctionsAndMetadataAreDiscoverable(t *testing.T) {
	contents := `# al: tags=git,work platforms=linux,wsl favorite=true
# Open the current branch
function gopen() {
  gh browse
}
`
	functions := parseFunctions(contents)
	if len(functions) != 1 {
		t.Fatalf("expected one function, got %#v", functions)
	}
	function := functions[0]
	if function.Name != "gopen" || function.Command != "gh browse" || function.Type != "function" || !function.Favorite {
		t.Fatalf("function fields were not parsed: %#v", function)
	}
	if strings.Join(function.Tags, ",") != "git,work" || strings.Join(function.Platforms, ",") != "linux,wsl" {
		t.Fatalf("function metadata was not parsed: %#v", function)
	}
}

func TestMetadataEditPreservesUnchangedFields(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".bash_aliases")
	original := "# al: tags=git platforms=linux\nalias gs='git status'\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := setEntryMetadata(path, "gs", EntryMetadata{Tags: []string{"git"}, Platforms: []string{"linux"}, Favorite: true}); err != nil {
		t.Fatal(err)
	}
	updated, _ := os.ReadFile(path)
	if !strings.Contains(string(updated), "tags=git") || !strings.Contains(string(updated), "platforms=linux") || !strings.Contains(string(updated), "favorite=true") {
		t.Fatalf("metadata update lost fields:\n%s", updated)
	}
}

func TestAliasComparisonReportsBothSides(t *testing.T) {
	local := []byte("alias gs='git status -sb'\nalias ll='ls -la'\n")
	remote := []byte("alias gs='git status'\nalias gp='git push'\n")
	localOnly, remoteOnly, conflicts := compareAliasFiles(local, remote)
	if strings.Join(localOnly, ",") != "ll" || strings.Join(remoteOnly, ",") != "gp" {
		t.Fatalf("wrong unique entries: local=%v remote=%v", localOnly, remoteOnly)
	}
	if len(conflicts) != 1 || conflicts[0].Name != "gs" || conflicts[0].Local != "git status -sb" || conflicts[0].Remote != "git status" {
		t.Fatalf("wrong conflict: %#v", conflicts)
	}
}

func TestAliasDefinitionMapDoesNotConvertFunctions(t *testing.T) {
	contents := []byte("alias gs='git status'\nfunction gopen() { gh browse; }\n")
	aliases := aliasDefinitionMap(contents)
	if len(aliases) != 1 || aliases["gs"] != "git status" {
		t.Fatalf("plain alias map was incorrect: %#v", aliases)
	}
	if _, exists := aliases["gopen"]; exists {
		t.Fatal("function was treated as a plain alias")
	}
}

func TestTimestampedRevisionCanBeListed(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, ".bash_aliases")
	if err := os.WriteFile(path, []byte("alias gs='git status'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := addAliasToFile(path, "gp", "git push", "Push commits"); err != nil {
		t.Fatal(err)
	}
	revisions, err := listRevisions(path)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("expected one revision: %#v, %v", revisions, err)
	}
	revision, err := os.ReadFile(revisions[0].Path)
	if err != nil || !strings.Contains(string(revision), "alias gs=") || strings.Contains(string(revision), "alias gp=") {
		t.Fatalf("revision did not preserve the prior version: %s, %v", revision, err)
	}
}

func TestBashLoaderSetupIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bashrc")
	if err := ensureBashLoadsAliases(path); err != nil {
		t.Fatal(err)
	}
	if err := ensureBashLoadsAliases(path); err != nil {
		t.Fatal(err)
	}
	contents, _ := os.ReadFile(path)
	if strings.Count(string(contents), ".bash_aliases") != 2 {
		t.Fatalf("loader was duplicated or incomplete:\n%s", contents)
	}
}

func TestShellIntegrationDoesNotAccumulateBlankLines(t *testing.T) {
	initial := []string{"alias gs='git status'", "", "", "# Alias Lens shell integration", `eval "$(command alias-lens shell-init bash)"`}
	once := withShellIntegration(initial)
	twice := withShellIntegration(once)
	if strings.Join(once, "\n") != strings.Join(twice, "\n") {
		t.Fatalf("shell integration was not idempotent:\n%q\n%q", once, twice)
	}
	if strings.Count(strings.Join(once, "\n"), "\n\n") != 1 {
		t.Fatalf("integration spacing was not normalized: %q", once)
	}
}

func TestAutoSyncDecisionNeverOverwritesConcurrentChanges(t *testing.T) {
	base := contentHash([]byte("base"))
	local := contentHash([]byte("local"))
	remote := contentHash([]byte("remote"))
	state := SyncState{LocalHash: base, RemoteHash: base, Status: "synced"}
	if action := decideSyncAction(state, local, base); action != actionPush {
		t.Fatalf("local-only change chose %s", action)
	}
	if action := decideSyncAction(state, base, remote); action != actionPull {
		t.Fatalf("remote-only change chose %s", action)
	}
	if action := decideSyncAction(state, local, remote); action != actionConflict {
		t.Fatalf("concurrent change chose %s", action)
	}
	if action := decideSyncAction(SyncState{}, local, remote); action != actionConflict {
		t.Fatalf("different first-sync files chose %s", action)
	}
	if action := decideSyncAction(SyncState{Status: "conflict", LocalHash: local, RemoteHash: remote}, local, remote); action != actionConflict {
		t.Fatalf("unresolved conflict chose %s", action)
	}
	if action := decideSyncAction(state, local, local); action != actionNoop {
		t.Fatalf("matching files chose %s", action)
	}
}

func TestNewAliasFileUsesPrivatePermissions(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_aliases")
	if err := writeNewAliasFile(path, []byte("alias gs='git status'\n")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("new alias file mode is %o", info.Mode().Perm())
	}
}

func TestCredentialShapedFilesCannotBeTracked(t *testing.T) {
	rejected := []string{".env", ".env.production", "credentials.json", "private.key", "client.p12", "my-secrets.toml"}
	for _, path := range rejected {
		if !sensitiveConfigPath(path) {
			t.Errorf("expected %s to be rejected", path)
		}
	}
	for _, path := range []string{".gitconfig", "starship.toml", "settings.json"} {
		if sensitiveConfigPath(path) {
			t.Errorf("expected %s to be allowed", path)
		}
	}
}

func TestTrackedFilesViewShowsSourceDestinationAndState(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	m := model{
		width:       100,
		height:      30,
		trackedOnly: true,
		trackedRepo: "/tmp/dotfiles",
		tracked: []trackedFileItem{{
			Config: TrackedFileConfig{Source: "/tmp/starship.toml", RepositoryPath: "shell/starship.toml"},
			State:  SyncState{Status: "synced", Message: "files match"},
		}},
	}
	view := m.View()
	for _, expected := range []string{"TRACKED CONFIG FILES", "/tmp/starship.toml", "repo/shell/starship.toml", "SYNCED", "files match"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("tracked-files view does not contain %q:\n%s", expected, view)
		}
	}
}

func TestTrackedFilesViewExplainsEmptyRegistry(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	applyTheme(builtInTheme("phosphor"))
	view := (model{width: 90, height: 24, trackedOnly: true}).View()
	if !strings.Contains(view, "No extra config files are tracked.") || !strings.Contains(view, "al track PATH") {
		t.Fatalf("empty tracked-files view is not actionable:\n%s", view)
	}
}
