package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestSetupDetectsShellFromEnvironment(t *testing.T) {
	t.Setenv(activeShellEnvironment, "")
	t.Setenv("SHELL", "/usr/local/bin/zsh")
	adapter, err := requestedShellAdapter("")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != "zsh" {
		t.Fatalf("detected %q, want zsh", adapter.Name())
	}
}

func TestExplicitSetupShellOverridesEnvironment(t *testing.T) {
	t.Setenv(activeShellEnvironment, "zsh")
	t.Setenv("SHELL", "/bin/zsh")
	adapter, err := requestedShellAdapter("bash")
	if err != nil {
		t.Fatal(err)
	}
	if adapter.Name() != "bash" {
		t.Fatalf("selected %q, want bash", adapter.Name())
	}
}

func TestActiveShellAdapterSelectionTable(t *testing.T) {
	cases := []struct {
		name        string
		integration string
		configured  string
		shell       string
		want        string
	}{
		{name: "integration wins", integration: "bash", configured: "zsh", shell: "/bin/zsh", want: "bash"},
		{name: "invalid integration uses config", integration: "fish", configured: "zsh", shell: "/bin/bash", want: "zsh"},
		{name: "config used", configured: "zsh", shell: "/bin/bash", want: "zsh"},
		{name: "SHELL ignored at runtime", configured: "bash", shell: "/bin/zsh", want: "bash"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv(activeShellEnvironment, test.integration)
			t.Setenv("SHELL", test.shell)
			if err := saveConfig(AppConfig{Shell: test.configured, AliasFile: "." + test.configured + "_aliases"}); err != nil {
				t.Fatal(err)
			}
			if got := activeShellAdapter().Name(); got != test.want {
				t.Fatalf("active adapter = %q, want %q", got, test.want)
			}
		})
	}
}

func TestRequestedShellAdapterSelectionTable(t *testing.T) {
	cases := []struct {
		name        string
		explicit    string
		integration string
		shell       string
		configured  string
		want        string
		wantError   string
	}{
		{name: "explicit", explicit: "bash", integration: "zsh", shell: "/bin/zsh", configured: "zsh", want: "bash"},
		{name: "integration", integration: "zsh", shell: "/bin/bash", configured: "bash", want: "zsh"},
		{name: "detected", shell: "/usr/local/bin/zsh", configured: "bash", want: "zsh"},
		{name: "configured", configured: "zsh", want: "zsh"},
		{name: "unsupported integration", integration: "fish", shell: "/bin/bash", configured: "bash", wantError: "unsupported shell"},
		{name: "unsupported detected", shell: "/usr/bin/fish", configured: "bash", wantError: "unsupported shell"},
		{name: "whitespace integration", integration: " ", configured: "zsh", wantError: "unsupported shell"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Setenv(activeShellEnvironment, test.integration)
			t.Setenv("SHELL", test.shell)
			if err := saveConfig(AppConfig{Shell: test.configured, AliasFile: "." + test.configured + "_aliases"}); err != nil {
				t.Fatal(err)
			}
			adapter, err := requestedShellAdapter(test.explicit)
			if test.wantError != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantError) {
					t.Fatalf("error = %v, want %q", err, test.wantError)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if adapter.Name() != test.want {
				t.Fatalf("requested adapter = %q, want %q", adapter.Name(), test.want)
			}
		})
	}
}

func TestActiveShellSelectionMalformedConfigDoesNotRewriteIt(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "")
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	want := []byte("{not-json}\n")
	if err := os.WriteFile(path, want, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := activeShellAdapter().Name(); got != "bash" {
		t.Fatalf("malformed config fallback = %q", got)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("shell selection rewrote malformed config: %q", got)
	}
}

func TestSetupRejectsUnsupportedDetectedShell(t *testing.T) {
	t.Setenv(activeShellEnvironment, "")
	t.Setenv("SHELL", "/usr/bin/fish")
	if _, err := requestedShellAdapter(""); err == nil || !strings.Contains(err.Error(), "unsupported shell") {
		t.Fatalf("unsupported detected shell returned %v", err)
	}
}

func TestZshStartupSetupIsIdempotent(t *testing.T) {
	t.Setenv("ZDOTDIR", "")
	home := t.TempDir()
	adapter := zshShellAdapter{}
	if err := adapter.ConfigureStartup(home, "darwin", ""); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ConfigureStartup(home, "linux", ""); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(contents), ".zsh_aliases") != 2 {
		t.Fatalf("Zsh loader was duplicated or incomplete:\n%s", contents)
	}
}

func TestZshSetupRespectsZdotdir(t *testing.T) {
	home := t.TempDir()
	zdotdir := filepath.Join(home, "zsh")
	if err := os.MkdirAll(zdotdir, 0o700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZDOTDIR", zdotdir)
	if err := (zshShellAdapter{}).ConfigureStartup(home, "darwin", ""); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(zdotdir, ".zshrc"))
	if err != nil || !strings.Contains(string(contents), ".zsh_aliases") {
		t.Fatalf("ZDOTDIR startup file was not configured: %s, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatal("setup modified the home .zshrc despite ZDOTDIR")
	}
}

func TestAdapterPathMatrix(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(historyFileEnvironment, "")
	bash := bashShellAdapter{}
	if path, err := aliasPathFor(bash); err == nil {
		if want := filepath.Join(home, ".bash_aliases"); path != want {
			t.Fatalf("Bash alias path = %q, want %q", path, want)
		}
	} else {
		t.Fatal(err)
	}
	if got := historyPathFor(bash, home); got != filepath.Join(home, ".bash_history") {
		t.Fatalf("Bash history path = %q", got)
	}
	paths, err := bash.StartupPaths(home, "linux")
	if err != nil || len(paths) != 2 || paths[0] != filepath.Join(home, ".bashrc") || paths[1] != filepath.Join(home, ".bash_profile") {
		t.Fatalf("Bash startup paths = %#v, %v", paths, err)
	}

	zdotdir := filepath.Join(home, "zsh")
	t.Setenv("ZDOTDIR", zdotdir)
	zsh := zshShellAdapter{}
	if got := historyPathFor(zsh, home); got != filepath.Join(home, ".zsh_history") {
		t.Fatalf("Zsh history path = %q", got)
	}
	paths, err = zsh.StartupPaths(home, "linux")
	if err != nil || len(paths) != 1 || paths[0] != filepath.Join(zdotdir, ".zshrc") {
		t.Fatalf("Zsh startup paths = %#v, %v", paths, err)
	}

	override := filepath.Join(home, "custom.history")
	t.Setenv(historyFileEnvironment, override)
	if got := historyPathFor(bash, home); got != override {
		t.Fatalf("history override = %q, want %q", got, override)
	}
}

func TestBashSetupKeepsUserBinaryOnPathAfterRestart(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := os.MkdirAll(binDir, 0o700); err != nil {
		t.Fatal(err)
	}
	shim := filepath.Join(binDir, "alias-lens")
	shimContents := "#!/bin/sh\nif [ \"${1-}\" = shell-init ]; then\n  printf 'al() { :; }\\n'\nfi\n"
	if err := os.WriteFile(shim, []byte(shimContents), 0o700); err != nil {
		t.Fatal(err)
	}
	aliases := []byte("eval \"$(command alias-lens shell-init bash)\"\n")
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), aliases, 0o600); err != nil {
		t.Fatal(err)
	}
	bashrc := filepath.Join(home, ".bashrc")
	original := []byte("# existing Bash settings\n" + bashAliasLoader)
	if err := os.WriteFile(bashrc, original, 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := bashShellAdapter{}
	if err := adapter.ConfigureStartup(home, "linux", binDir); err != nil {
		t.Fatal(err)
	}
	if err := adapter.ConfigureStartup(home, "linux", binDir); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(bashrc)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(contents), "Keep the Alias Lens executable available") != 1 {
		t.Fatalf("PATH setup was duplicated or missing:\n%s", contents)
	}
	backup, err := os.ReadFile(bashrc + ".alias-lens.bak")
	if err != nil || string(backup) != string(original) {
		t.Fatalf("startup backup does not contain the original file: %q, %v", backup, err)
	}
	command := exec.Command("bash", "--noprofile", "--norc", "-c", `source "$HOME/.bashrc"; command -v alias-lens; type al`)
	command.Env = append(os.Environ(), "HOME="+home, "PATH=/usr/bin:/bin")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("new WSL-style Bash shell cannot find alias-lens: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), shim) || !strings.Contains(string(output), "al is a function") {
		t.Fatalf("new shell did not load the binary and al function:\n%s", output)
	}
	loginCommand := exec.Command("bash", "--login", "-c", `command -v alias-lens; type al`)
	loginCommand.Env = append(os.Environ(), "HOME="+home, "PATH=/usr/bin:/bin")
	loginOutput, err := loginCommand.CombinedOutput()
	if err != nil {
		t.Fatalf("new Bash login shell cannot find alias-lens: %v\n%s", err, loginOutput)
	}
	if !strings.Contains(string(loginOutput), shim) || !strings.Contains(string(loginOutput), "al is a function") {
		t.Fatalf("new login shell did not load the binary and al function:\n%s", loginOutput)
	}
}

func TestSetupRemoveKeepsAliasesAndUnrelatedBashSettings(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	aliasPath := filepath.Join(home, ".bash_aliases")
	aliases := "alias al='echo personal'\nalias ll='ls -al'\n\n# Alias Lens Bash integration\neval \"$(command alias-lens shell-init bash)\"\n"
	if err := os.WriteFile(aliasPath, []byte(aliases), 0o600); err != nil {
		t.Fatal(err)
	}
	bashrcPath := filepath.Join(home, ".bashrc")
	userBashrc := "export EDITOR=vim\n. \"$HOME/.bash_aliases\"\n"
	bashrc := userBashrc + startupPathBlock(home, filepath.Join(home, ".local", "bin")) + bashAliasLoader
	if err := os.WriteFile(bashrcPath, []byte(bashrc), 0o640); err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(home, ".profile")
	userProfile := "export LANG=C\n"
	if err := os.WriteFile(profilePath, []byte(userProfile+bashLoginLoader), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runSetupCommand([]string{"--remove", "bash"}); err != nil {
		t.Fatal(err)
	}
	if err := runSetupCommand([]string{"--remove", "bash"}); err != nil {
		t.Fatal(err)
	}
	updatedAliases, err := os.ReadFile(aliasPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(updatedAliases), "alias al='echo personal'") || !strings.Contains(string(updatedAliases), "alias ll='ls -al'") {
		t.Fatalf("setup removal changed user aliases:\n%s", updatedAliases)
	}
	if strings.Contains(string(updatedAliases), "shell-init") || strings.Contains(string(updatedAliases), "Alias Lens Bash integration") {
		t.Fatalf("setup removal left generated alias integration:\n%s", updatedAliases)
	}
	updatedBashrc, err := os.ReadFile(bashrcPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(updatedBashrc) != userBashrc {
		t.Fatalf("setup removal changed unrelated Bash settings:\n%s", updatedBashrc)
	}
	info, err := os.Stat(bashrcPath)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("setup removal changed Bash file mode: %v", info.Mode().Perm())
	}
	updatedProfile, err := os.ReadFile(profilePath)
	if err != nil || string(updatedProfile) != userProfile {
		t.Fatalf("setup removal changed unrelated Bash login settings: %q, %v", updatedProfile, err)
	}
}

func TestSetupRemoveRespectsZdotdir(t *testing.T) {
	home := t.TempDir()
	zdotdir := filepath.Join(home, "zsh")
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", zdotdir)
	t.Setenv(activeShellEnvironment, "zsh")
	if err := os.MkdirAll(zdotdir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".zsh_aliases"), []byte("# Alias Lens Zsh integration\neval \"$(command alias-lens shell-init zsh)\"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	zshrcPath := filepath.Join(zdotdir, ".zshrc")
	if err := os.WriteFile(zshrcPath, []byte("setopt autocd\n"+zshAliasLoader), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runSetupCommand([]string{"zsh", "--remove"}); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(zshrcPath)
	if err != nil || string(contents) != "setopt autocd\n" {
		t.Fatalf("ZDOTDIR removal result = %q, %v", contents, err)
	}
	if _, err := os.Stat(filepath.Join(home, ".zshrc")); !os.IsNotExist(err) {
		t.Fatalf("setup removal changed the wrong Zsh file: %v", err)
	}
}

func TestSetupRepairNormalizesDuplicateGeneratedBlocks(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	aliasPath := filepath.Join(home, ".bash_aliases")
	integration := "# Alias Lens Bash integration\neval \"$(command alias-lens shell-init bash)\"\n"
	if err := os.WriteFile(aliasPath, []byte("alias ll='ls -al'\n"+integration+integration), 0o600); err != nil {
		t.Fatal(err)
	}
	bashrcPath := filepath.Join(home, ".bashrc")
	if err := os.WriteFile(bashrcPath, []byte(bashAliasLoader+bashAliasLoader), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runSetupCommand([]string{"--repair", "bash"}); err != nil {
		t.Fatal(err)
	}
	updatedAliases, err := os.ReadFile(aliasPath)
	if err != nil || strings.Count(string(updatedAliases), "shell-init bash") != 1 {
		t.Fatalf("repair did not normalize alias integration: %s, %v", updatedAliases, err)
	}
	updatedBashrc, err := os.ReadFile(bashrcPath)
	if err != nil || strings.Count(string(updatedBashrc), ".bash_aliases") != 2 {
		t.Fatalf("repair did not normalize startup loader: %s, %v", updatedBashrc, err)
	}
}

func TestMacBashSetupNormalizesDuplicateLoginLoaders(t *testing.T) {
	home := t.TempDir()
	profilePath := filepath.Join(home, ".bash_profile")
	if err := os.WriteFile(profilePath, []byte(bashLoginLoader+bashLoginLoader), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (bashShellAdapter{}).ConfigureStartup(home, "darwin", ""); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(profilePath)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Count(string(contents), "Alias Lens Bash login loader >>>") != 1 {
		t.Fatalf("Bash login loader was not normalized:\n%s", contents)
	}
}

func TestZshSetupPersistsUserBinaryDirectory(t *testing.T) {
	home := t.TempDir()
	binDir := filepath.Join(home, ".local", "bin")
	if err := (zshShellAdapter{}).ConfigureStartup(home, "linux", binDir); err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(filepath.Join(home, ".zshrc"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(contents), `_alias_lens_bin_dir="$HOME"/'.local/bin'`) {
		t.Fatalf("Zsh startup does not persist the user binary directory:\n%s", contents)
	}
}

func TestUserOwnedExecutableDirectory(t *testing.T) {
	home := t.TempDir()
	want := filepath.Join(home, ".local", "bin")
	if got := userOwnedExecutableDirectory(filepath.Join(want, "alias-lens"), home); got != want {
		t.Fatalf("user executable directory = %q, want %q", got, want)
	}
	if got := userOwnedExecutableDirectory("/usr/local/bin/alias-lens", home); got != "" {
		t.Fatalf("system executable directory should not be persisted, got %q", got)
	}
}

func TestZshExtendedHistoryIsNormalized(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zsh_history")
	contents := ": 1789059600:4;git status --short --branch\n: 1789059610:0;git status --short --branch\n"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	counts, err := historyCountsFromShell(path, "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if counts["git status --short --branch"] != 2 {
		t.Fatalf("unexpected Zsh history counts: %#v", counts)
	}
}

func TestZshIntegrationExecutesAliasName(t *testing.T) {
	if !strings.Contains(zshIntegration, `ALIAS_LENS_SHELL=zsh ALIAS_LENS_HISTORY_FILE=`) {
		t.Fatal("Zsh integration does not pin commands to the Zsh adapter")
	}
	if !strings.Contains(zshIntegration, `builtin eval "$_alias_lens_name"`) {
		t.Fatal("Zsh integration does not execute the selected alias name")
	}
	if !strings.Contains(zshIntegration, `alias-lens shell-entry "$_alias_lens_name"`) {
		t.Fatal("Zsh integration does not load a newly added alias before executing it")
	}
	if strings.Contains(zshIntegration, "entry-summary") || strings.Contains(zshIntegration, "Alias Lens ran") {
		t.Fatal("Zsh integration still prints a post-execution receipt")
	}
	if !strings.Contains(zshIntegration, `_alias_lens_flush_history 2>/dev/null || true`) || !strings.Contains(zshIntegration, `setopt localoptions extendedhistory`) || !strings.Contains(zshIntegration, `print -s -- "$_alias_lens_name"`) || !strings.Contains(zshIntegration, `fc -AI "${HISTFILE:-$HOME/.zsh_history}"`) {
		t.Fatal("Zsh integration does not record picker runs in native history")
	}
	if !strings.Contains(zshIntegration, `bindkey '^G' _alias_lens_launch`) {
		t.Fatal("Zsh integration does not install the ZLE key binding")
	}
	if !strings.Contains(zshIntegration, `ALIAS_LENS_PROMPT_ACCEPT=1`) || !strings.Contains(zshIntegration, `BUFFER="$_alias_lens_name"`) || !strings.Contains(zshIntegration, `zle accept-line`) {
		t.Fatal("Zsh picker selections are not accepted as native command lines")
	}
	if !strings.Contains(zshIntegration, `ALIAS_LENS_NOBIND`) || !strings.Contains(zshIntegration, `[[ -n "$BUFFER" ]]`) {
		t.Fatal("Zsh binding cannot be disabled or preserve Ctrl+G on a non-empty prompt")
	}
}

func TestShellEntryDefinitionReloadsAliasesAndFunctions(t *testing.T) {
	definition, err := shellEntryDefinition(Alias{Name: "cl", Command: "printf '%s\\n' cleared", Type: "alias"})
	if err != nil {
		t.Fatal(err)
	}
	name, command, ok := parseAliasDefinition(definition)
	if !ok || name != "cl" || command != "printf '%s\\n' cleared" {
		t.Fatalf("alias definition did not round trip: %q", definition)
	}

	definition, err = shellEntryDefinition(Alias{Name: "mkcd", Command: "mkdir -p \"$1\"; cd \"$1\"", Type: "function"})
	if err != nil || !strings.Contains(definition, "mkcd() {") || !strings.Contains(definition, "mkdir -p") {
		t.Fatalf("function definition was not reconstructed: %q, %v", definition, err)
	}
}

func TestNewlyWrittenAliasCanBeLoadedIntoCurrentShell(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv(activeShellEnvironment, "bash")
	path := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := addAliasToFile(path, "cl", "printf clear", "Clear the screen"); err != nil {
		t.Fatal(err)
	}
	definition, err := loadShellEntry("cl")
	if err != nil {
		t.Fatal(err)
	}
	name, command, ok := parseAliasDefinition(definition)
	if !ok || name != "cl" || command != "printf clear" {
		t.Fatalf("saved alias could not be loaded: %q", definition)
	}
}

func TestBashIntegrationLoadsNewAliasBeforeRunningIt(t *testing.T) {
	directory := t.TempDir()
	shim := filepath.Join(directory, "alias-lens")
	contents := `#!/bin/sh
if [ "${1-}" = "shell-entry" ]; then
  printf "alias cl='printf newly-loaded'\n"
elif [ "${1-}" = "watch" ]; then
  :
else
  printf 'cl\n'
fi
`
	if err := os.WriteFile(shim, []byte(contents), 0o700); err != nil {
		t.Fatal(err)
	}
	script := "shopt -s expand_aliases\n" + bashIntegration + "\nal\n"
	command := exec.Command("bash", "--noprofile", "--norc", "-c", script)
	command.Env = append(os.Environ(), "PATH="+directory+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("Bash integration failed: %v\n%s", err, output)
	}
	if !strings.Contains(string(output), "newly-loaded") {
		t.Fatalf("new alias did not run in the existing shell:\n%s", output)
	}
	if strings.Contains(string(output), "Alias Lens ran") {
		t.Fatalf("integration printed an unwanted execution receipt:\n%s", output)
	}
}

func TestBashShellIntegrationGolden(t *testing.T) {
	assertShellIntegrationGolden(t, bashShellAdapter{}, "bash-integration.golden")
}

func TestZshShellIntegrationGolden(t *testing.T) {
	assertShellIntegrationGolden(t, zshShellAdapter{}, "zsh-integration.golden")
}

func assertShellIntegrationGolden(t *testing.T, adapter ShellAdapter, filename string) {
	t.Helper()
	want, err := os.ReadFile(filepath.Join("testdata", "phase3", filename))
	if err != nil {
		t.Fatal(err)
	}
	if got := adapter.Integration(); got != string(want) {
		t.Fatalf("%s integration changed from the approved phase 3 baseline", adapter.DisplayName())
	}
}

func TestShellAdapterContractBash(t *testing.T) {
	assertShellAdapterContract(t, bashShellAdapter{}, "builtin history -s", "clear-readline-buffer")
}

func TestShellAdapterContractZsh(t *testing.T) {
	assertShellAdapterContract(t, zshShellAdapter{}, "print -s", "zle-send-break")
}

func assertShellAdapterContract(t *testing.T, adapter ShellAdapter, historyCommand, promptAction string) {
	t.Helper()
	if adapter.ExecutionSpec().HistoryCommand == "" || !strings.Contains(adapter.ExecutionSpec().HistoryCommand, historyCommand) {
		t.Fatalf("%s execution history contract = %#v", adapter.Name(), adapter.ExecutionSpec())
	}
	if adapter.PromptSpec().SelectionPrefix != "$ " || adapter.PromptSpec().NonEmptyPromptAction != promptAction {
		t.Fatalf("%s prompt contract = %#v", adapter.Name(), adapter.PromptSpec())
	}
	if binding := adapter.BindingSpec(); binding.Key != "Ctrl+G" || binding.DisableEnvironment != "ALIAS_LENS_NOBIND" {
		t.Fatalf("%s binding contract = %#v", adapter.Name(), binding)
	}
}

func TestAdapterNameMatrix(t *testing.T) {
	for _, adapter := range []ShellAdapter{bashShellAdapter{}, zshShellAdapter{}} {
		for _, name := range []string{"ll", "g.s", "1x", "-x", "_x"} {
			if err := adapter.ValidateEntryName(name, "alias"); err != nil {
				t.Errorf("%s rejected baseline alias name %q: %v", adapter.Name(), name, err)
			}
		}
		for _, name := range []string{"é", "", "has space", "line\nbreak"} {
			if err := adapter.ValidateEntryName(name, "alias"); err == nil {
				t.Errorf("%s accepted invalid alias name %q", adapter.Name(), name)
			}
		}
		for _, name := range []string{"1x", "-x", "g.s"} {
			if err := adapter.ValidateEntryName(name, "function"); err == nil || !strings.Contains(err.Error(), "invalid function name") {
				t.Errorf("%s function validation for %q = %v", adapter.Name(), name, err)
			}
		}
	}
}

func TestUnknownEntryTypeBaseline(t *testing.T) {
	for _, adapter := range []ShellAdapter{bashShellAdapter{}, zshShellAdapter{}} {
		definition, err := adapter.RenderEntryDefinition(Alias{Name: "ll", Command: "ls -al", Type: "unknown"})
		if err != nil || definition != "alias ll='ls -al'" {
			t.Fatalf("%s unknown entry type baseline = %q, %v", adapter.Name(), definition, err)
		}
	}
}

func TestAdapterParsingDoesNotExecuteContent(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "executed")
	command := "touch " + sentinel
	for _, adapter := range []ShellAdapter{bashShellAdapter{}, zshShellAdapter{}} {
		definition, err := adapter.RenderEntryDefinition(Alias{Name: "unsafe", Command: command, Type: "alias"})
		if err != nil {
			t.Fatal(err)
		}
		name, parsed, ok := adapter.ParseAliasDefinition(definition)
		if !ok || name != "unsafe" || parsed != command {
			t.Fatalf("%s parse round trip = %q, %q, %v", adapter.Name(), name, parsed, ok)
		}
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("adapter parsing executed alias content: %v", err)
	}
}

func TestZshHistoryAdapterPreservesMalformedPrefixBaseline(t *testing.T) {
	adapter := zshShellAdapter{}
	if got := adapter.HistoryCommand(": not-a-time;git status"); got != "git status" {
		t.Fatalf("malformed Zsh prefix baseline = %q", got)
	}
	line, eventTime, skip := adapter.HistoryUsageLine(": not-a-time;ll", &shellHistoryState{})
	if line != "ll" || !eventTime.IsZero() || skip {
		t.Fatalf("malformed Zsh usage baseline = %q, %v, %v", line, eventTime, skip)
	}
}

func TestBashHistoryAdapterPreservesInvalidCommentBaseline(t *testing.T) {
	adapter := bashShellAdapter{}
	state := shellHistoryState{}
	if _, _, skip := adapter.HistoryUsageLine("#1700000000", &state); !skip {
		t.Fatal("valid Bash timestamp was not consumed")
	}
	if line, eventTime, skip := adapter.HistoryUsageLine("#not-a-time", &state); line != "#not-a-time" || !eventTime.IsZero() || skip {
		t.Fatalf("invalid Bash comment baseline = %q, %v, %v", line, eventTime, skip)
	}
	line, eventTime, skip := adapter.HistoryUsageLine("ll", &state)
	if line != "ll" || eventTime.Unix() != 1700000000 || skip {
		t.Fatalf("Bash timestamp after invalid comment = %q, %v, %v", line, eventTime, skip)
	}
}

func TestBashHistoryAdapterFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".bash_history")
	contents := "#1700000000\nll\n#invalid\ngs\nplain command"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	counts, err := historyCountsFromShell(path, "bash")
	if err != nil {
		t.Fatal(err)
	}
	if counts["#1700000000"] != 0 || counts["ll"] != 1 || counts["#invalid"] != 1 || counts["gs"] != 1 || counts["plain command"] != 1 {
		t.Fatalf("Bash history counts = %#v", counts)
	}
	events, err := historyUsageEventsFromShell(path, "bash", []Alias{{Name: "ll"}, {Name: "gs"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Name != "ll" || events[0].Time.Unix() != 1700000000 || events[1].Name != "gs" || !events[1].Time.IsZero() {
		t.Fatalf("Bash history events = %#v", events)
	}
}

func TestZshHistoryAdapterFixture(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zsh_history")
	contents := ": 1700000000:4;ll\n: not-a-time;gs\n: metadata-without-semicolon\nprintf 'a;b'"
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
	counts, err := historyCountsFromShell(path, "zsh")
	if err != nil {
		t.Fatal(err)
	}
	if counts["ll"] != 1 || counts["gs"] != 1 || counts[": metadata-without-semicolon"] != 1 || counts["printf 'a;b'"] != 1 {
		t.Fatalf("Zsh history counts = %#v", counts)
	}
	events, err := historyUsageEventsFromShell(path, "zsh", []Alias{{Name: "ll"}, {Name: "gs"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Name != "ll" || events[0].Time.Unix() != 1700000000 || events[1].Name != "gs" || !events[1].Time.IsZero() {
		t.Fatalf("Zsh history events = %#v", events)
	}
}

func TestHistoryAdapterMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	counts, err := historyCountsFromShell(path, "bash")
	if err != nil || len(counts) != 0 {
		t.Fatalf("missing history counts = %#v, %v", counts, err)
	}
	events, err := historyUsageEventsFromShell(path, "zsh", nil)
	if err != nil || len(events) != 0 {
		t.Fatalf("missing history events = %#v, %v", events, err)
	}
}

func TestHistoryAdapterReadError(t *testing.T) {
	path := t.TempDir()
	if _, err := historyCountsFromShell(path, "bash"); err == nil {
		t.Fatal("history count read error was ignored")
	}
	if _, err := historyUsageEventsFromShell(path, "zsh", nil); err == nil {
		t.Fatal("history event read error was ignored")
	}
}

func TestHistoryAdapterScannerLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "history")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", 1024*1024+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := historyCountsFromShell(path, "bash"); err == nil {
		t.Fatal("history count scanner limit was ignored")
	}
	if _, err := historyUsageEventsFromShell(path, "zsh", nil); err == nil {
		t.Fatal("history event scanner limit was ignored")
	}
}

func TestPureAdapterOperationsAggregateManifest(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", filepath.Join(home, "zsh"))
	if err := os.MkdirAll(filepath.Join(home, "zsh"), 0o700); err != nil {
		t.Fatal(err)
	}
	sentinel := filepath.Join(home, "sentinel")
	command := "touch " + sentinel
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), []byte("alias unsafe='"+command+"'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# user bash settings\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, "zsh", ".zshrc"), []byte("# user zsh settings\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	before := testTreeManifest(t, home)
	for _, adapter := range []ShellAdapter{bashShellAdapter{}, zshShellAdapter{}} {
		adapter.ValidateEntryName("unsafe", "alias")
		adapter.ParseAliasDefinition("alias unsafe='" + command + "'")
		adapter.ParseFunctions("unsafe() {\n" + command + "\n}")
		if _, err := adapter.RenderEntryDefinition(Alias{Name: "unsafe", Command: command, Type: "alias"}); err != nil {
			t.Fatal(err)
		}
		adapter.HistoryCommand(": 1700000000:0;unsafe")
		adapter.HistoryUsageLine("#1700000000", &shellHistoryState{})
		adapter.Integration()
		adapter.ExecutionSpec()
		adapter.PromptSpec()
		adapter.BindingSpec()
		paths, err := adapter.StartupPaths(home, "linux")
		if err != nil {
			t.Fatal(err)
		}
		adapter.CheckSyntax(filepath.Join(home, adapter.AliasFilename()))
		adapter.StartupStatus(home, "linux")
		if len(paths) == 0 {
			t.Fatalf("%s returned no startup path", adapter.Name())
		}
	}
	after := testTreeManifest(t, home)
	if before != after {
		t.Fatalf("pure adapter operations changed the temporary home\nbefore:\n%s\nafter:\n%s", before, after)
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatalf("pure adapter operation executed content: %v", err)
	}
}

func testTreeManifest(t *testing.T, root string) string {
	t.Helper()
	var entries []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		value := ""
		if info.Mode()&os.ModeSymlink != 0 {
			value, err = os.Readlink(path)
		} else if info.Mode().IsRegular() {
			contents, readErr := os.ReadFile(path)
			err = readErr
			value = string(contents)
		}
		if err != nil {
			return err
		}
		entries = append(entries, fmt.Sprintf("%s|%s|%04o|%s", relative, info.Mode().Type(), info.Mode().Perm(), value))
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return strings.Join(entries, "\n")
}

func TestAliasFormAcceptsSpacesInCommandsAndDescriptions(t *testing.T) {
	m := model{adding: true, field: 1}
	m.form[1] = "git"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	command := updated.(model)
	if command.form[1] != "git " {
		t.Fatalf("command field lost its space: %q", command.form[1])
	}

	command.field = 2
	command.form[2] = "Show"
	updated, _ = command.Update(tea.KeyMsg{Type: tea.KeySpace})
	description := updated.(model)
	if description.form[2] != "Show " {
		t.Fatalf("description field lost its space: %q", description.form[2])
	}
}

func TestAliasFormRejectsSpacesInAliasName(t *testing.T) {
	m := model{adding: true, field: 0}
	m.form[0] = "git"
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	result := updated.(model)
	if result.form[0] != "git" || result.status != "Alias names cannot contain spaces" {
		t.Fatalf("alias name accepted a space or lacked guidance: value=%q status=%q", result.form[0], result.status)
	}
}

func TestZshRevisionsUseAliasFilename(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".zsh_aliases")
	if err := saveRevision(path, []byte("alias gs='git status'\n")); err != nil {
		t.Fatal(err)
	}
	revisions, err := listRevisions(path)
	if err != nil || len(revisions) != 1 {
		t.Fatalf("expected one Zsh revision: %#v, %v", revisions, err)
	}
	if !strings.HasSuffix(revisions[0].Path, ".zsh_aliases") {
		t.Fatalf("unexpected revision path %q", revisions[0].Path)
	}
}
