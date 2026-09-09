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

func TestFilterGitHubRepos(t *testing.T) {
	repositories := []GitHubRepo{
		{FullName: "naqi/al", Description: "Alias manager"},
		{FullName: "naqi/dotfiles", Description: "Shell configuration"},
	}

	if got := filterGitHubRepos(repositories, "dot"); len(got) != 1 || got[0].FullName != "naqi/dotfiles" {
		t.Fatalf("name filter returned %#v", got)
	}
	if got := filterGitHubRepos(repositories, "manager"); len(got) != 1 || got[0].FullName != "naqi/al" {
		t.Fatalf("description filter returned %#v", got)
	}
	if got := filterGitHubRepos(repositories, ""); len(got) != len(repositories) {
		t.Fatalf("empty filter returned %d repositories", len(got))
	}
}
