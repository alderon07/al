package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

type DoctorCheck struct {
	Name    string
	OK      bool
	Message string
}

func runDoctor() error {
	checks := doctorChecks()
	failures := 0
	for _, check := range checks {
		marker := "OK"
		if !check.OK {
			marker = "FIX"
			failures++
		}
		fmt.Printf("%-3s  %-18s %s\n", marker, check.Name, check.Message)
	}
	if failures > 0 {
		return fmt.Errorf("%d checks need attention", failures)
	}
	fmt.Println("Alias Lens is ready.")
	return nil
}

func doctorChecks() []DoctorCheck {
	var checks []DoctorCheck
	adapter := activeShellAdapter()
	executable, executableErr := os.Executable()
	checks = append(checks, DoctorCheck{Name: "binary", OK: executableErr == nil, Message: defaultString(executable, "not found")})
	pathExecutable, pathErr := exec.LookPath("alias-lens")
	pathMessage := pathExecutable
	if pathErr != nil {
		pathMessage = "run " + defaultString(executable, "alias-lens") + " setup " + adapter.Name() + ", then start a new shell"
	}
	checks = append(checks, DoctorCheck{Name: "binary on PATH", OK: pathErr == nil, Message: pathMessage})
	aliasPath, aliasErr := aliasesPath()
	_, aliasStatErr := os.Stat(aliasPath)
	checks = append(checks, DoctorCheck{Name: adapter.AliasFilename(), OK: aliasErr == nil && aliasStatErr == nil, Message: aliasPath})
	if aliasErr == nil && aliasStatErr == nil {
		checks = append(checks, aliasSyntaxDoctorCheck(aliasPath, adapter.Name()))
	}

	home, _ := os.UserHomeDir()
	startupOK, startupMessage := adapter.StartupStatus(home, runtime.GOOS)
	checks = append(checks, DoctorCheck{Name: adapter.DisplayName() + " loading", OK: startupOK, Message: startupMessage})
	aliases, _ := os.ReadFile(aliasPath)
	hasIntegration := strings.Contains(string(aliases), "shell-init "+adapter.Name())
	setupCommand := "run al setup " + adapter.Name()
	actionsMessage := setupCommand
	if hasIntegration {
		actionsMessage = shellActionsMessage(adapter)
	}
	checks = append(checks, DoctorCheck{Name: "shell actions", OK: hasIntegration, Message: actionsMessage})

	_, gitErr := exec.LookPath("git")
	checks = append(checks, DoctorCheck{Name: "Git", OK: gitErr == nil, Message: map[bool]string{true: "installed", false: "install Git"}[gitErr == nil]})
	config, configErr := loadConfig()
	repoOK := configErr == nil && config.Repository != ""
	repoMessage := "run al repo"
	if repoOK {
		if output, err := gitOutput("-C", config.Repository, "rev-parse", "--is-inside-work-tree"); err != nil {
			repoOK = false
			repoMessage = cleanCommandOutput(output)
		} else {
			repoMessage = config.Repository
		}
	}
	checks = append(checks, DoctorCheck{Name: "sync repository", OK: repoOK, Message: repoMessage})
	if config.AutoSync.Enabled {
		state, _ := loadSyncState()
		syncOK := state.Status != "conflict" && state.Status != "offline" && state.Status != ""
		message := defaultString(state.Status, "run al watch")
		if state.Message != "" {
			message += ": " + state.Message
		}
		checks = append(checks, DoctorCheck{Name: "automatic sync", OK: syncOK, Message: message})
	} else {
		checks = append(checks, DoctorCheck{Name: "automatic sync", OK: false, Message: "run al autosync enable"})
	}

	for _, provider := range configuredProviders(config) {
		ok, message := providerCredentialStatus(provider)
		checks = append(checks, DoctorCheck{Name: provider.Label(), OK: ok, Message: message})
		sshOK := checkProviderSSH(provider)
		sshMessage := "HTTPS fallback will be used"
		if sshOK {
			sshMessage = "authenticated SSH connection"
		}
		checks = append(checks, DoctorCheck{Name: provider.Label() + " SSH", OK: true, Message: sshMessage})
	}
	return checks
}

func providerCredentialStatus(provider RepoProvider) (bool, string) {
	switch provider.ID() {
	case "github":
		if _, err := exec.LookPath("gh"); err != nil {
			return false, "install gh"
		}
		ctx, cancel := interruptContext()
		_, err := commandOutput(ctx, repositoryCommandTimeout, "gh", "auth", "status")
		cancel()
		if err != nil {
			return false, "run al repo github"
		}
		return true, "GitHub CLI authenticated"
	case "bitbucket":
		if os.Getenv("BITBUCKET_API_TOKEN") == "" {
			return false, "run al repo bitbucket"
		}
		return true, "API token available"
	case "gitlab":
		if os.Getenv("GITLAB_TOKEN") != "" {
			return true, "API token available"
		}
		glab, err := exec.LookPath("glab")
		if err != nil {
			return false, "install glab, then run al repo gitlab"
		}
		host := "gitlab.com"
		switch configured := provider.(type) {
		case gitlabProvider:
			host, _ = configured.hostname()
		case *gitlabProvider:
			host, _ = configured.hostname()
		}
		ctx, cancel := interruptContext()
		_, err = commandOutput(ctx, repositoryCommandTimeout, glab, "auth", "status", "--hostname", host)
		cancel()
		if err != nil {
			return false, "run al repo gitlab"
		}
		return true, "GitLab CLI authenticated"
	default:
		return false, "unknown provider"
	}
}

type setupAction int

const (
	setupInstall setupAction = iota
	setupRepair
	setupRemove
)

func runSetupCommand(arguments []string) error {
	action := setupInstall
	shellName := ""
	for _, argument := range arguments {
		switch argument {
		case "--repair":
			if action != setupInstall {
				return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
			}
			action = setupRepair
		case "--remove":
			if action != setupInstall {
				return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
			}
			action = setupRemove
		case "bash", "zsh":
			if shellName != "" {
				return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
			}
			shellName = argument
		default:
			return fmt.Errorf("usage: al setup [--repair|--remove] [bash|zsh]")
		}
	}
	if action == setupRemove {
		return removeSetup(shellName)
	}
	return runSetup(shellName, action == setupRepair)
}

func runSetup(shellName string, repair bool) error {
	adapter, err := requestedShellAdapter(shellName)
	if err != nil {
		return err
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	oldAdapter, _ := shellAdapter(config.Shell)
	if oldAdapter != nil && config.AliasFile == oldAdapter.AliasFilename() {
		config.AliasFile = adapter.AliasFilename()
	}
	config.Shell = adapter.Name()
	if err := saveConfig(config); err != nil {
		return err
	}
	if shellName == "" {
		detected := filepath.Base(strings.TrimSpace(os.Getenv(activeShellEnvironment)))
		if detected == "." || detected == "" {
			detected = filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
		}
		if detected == adapter.Name() {
			fmt.Printf("Detected %s from the current shell environment.\n", adapter.DisplayName())
		} else {
			fmt.Printf("Using configured shell %s. Pass bash or zsh to override it.\n", adapter.DisplayName())
		}
	}
	aliasPath, err := aliasPathFor(adapter)
	if err != nil {
		return err
	}
	if err := ensureAliasFileExists(aliasPath); err != nil {
		return err
	}
	contents, err := os.ReadFile(aliasPath)
	if err != nil {
		return err
	}
	lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
	kept := withShellIntegration(lines, adapter)
	updated := []byte(strings.Join(kept, "\n") + "\n")
	if string(updated) == string(contents) {
		if err := os.Chmod(aliasPath, 0o600); err != nil {
			return err
		}
		if repair {
			fmt.Printf("Verified Alias Lens %s alias integration.\n", adapter.DisplayName())
		} else {
			fmt.Printf("Alias Lens %s integration is already installed.\n", adapter.DisplayName())
		}
	} else {
		if err := writeAliasFile(aliasPath, contents, updated, 0o600); err != nil {
			return err
		}
		verb := "Installed"
		if repair {
			verb = "Repaired"
		}
		fmt.Printf("%s Alias Lens %s integration.\n", verb, adapter.DisplayName())
	}
	fmt.Println(shellSetupInstruction(adapter))
	home := filepath.Dir(aliasPath)
	if err := adapter.ConfigureStartup(home, runtime.GOOS, userExecutableDirectory(home)); err != nil {
		return err
	}
	if repair {
		fmt.Println("Alias Lens kept your aliases and checked only its generated integration.")
		return nil
	}
	if err := scheduleTour(); err != nil {
		return fmt.Errorf("save first-run tour state: %w", err)
	}
	if !interactiveInput(os.Stdin) {
		fmt.Println("Optional developer aliases were not reviewed because input is not interactive. Run al setup in a terminal to review them.")
		return nil
	}
	return offerDefaultAliases(aliasPath, os.Stdin, os.Stdout)
}

func shellActionsMessage(adapter ShellAdapter) string {
	if adapter.Name() == "bash" {
		if supported, known := bashCtrlGSupport(); known && !supported {
			return "Enter and al use are enabled; run al because Ctrl+G requires Bash 4 or newer"
		}
	}
	return "Enter, al use, and Ctrl+G are enabled"
}

func shellSetupInstruction(adapter ShellAdapter) string {
	if adapter.Name() == "bash" {
		if supported, known := bashCtrlGSupport(); known && !supported {
			return "Start a new bash shell, then run al. Ctrl+G requires Bash 4 or newer."
		}
	}
	return "Start a new " + adapter.Name() + " shell, then press Ctrl+G on an empty prompt."
}

func bashCtrlGSupport() (supported, known bool) {
	executable, err := exec.LookPath("bash")
	if err != nil {
		return false, false
	}
	output, err := commandOutput(context.Background(), 2*time.Second, executable, "--version")
	if err != nil {
		return false, false
	}
	fields := strings.Fields(string(output))
	for index, field := range fields {
		if field != "version" || index+1 >= len(fields) {
			continue
		}
		majorText, _, _ := strings.Cut(fields[index+1], ".")
		major, parseErr := strconv.Atoi(majorText)
		if parseErr == nil {
			return major >= 4, true
		}
	}
	return false, false
}

func removeSetup(shellName string) error {
	adapter, err := requestedShellAdapter(shellName)
	if err != nil {
		return err
	}
	aliasPath, err := aliasPathFor(adapter)
	if err != nil {
		return err
	}
	if contents, readErr := os.ReadFile(aliasPath); readErr == nil {
		lines := strings.Split(strings.TrimSuffix(string(contents), "\n"), "\n")
		updated := []byte(strings.Join(withoutShellIntegration(lines), "\n"))
		if len(updated) > 0 {
			updated = append(updated, '\n')
		}
		if string(updated) != string(contents) {
			if err := writeAliasFile(aliasPath, contents, updated, 0o600); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	if err := adapter.RemoveStartup(home, runtime.GOOS); err != nil {
		return err
	}
	fmt.Printf("Removed Alias Lens %s shell integration. Start a new %s shell to finish. Your aliases, configuration, revisions, and repositories were kept.\n", adapter.DisplayName(), adapter.Name())
	return nil
}

func userExecutableDirectory(home string) string {
	executable, err := os.Executable()
	if err != nil {
		return ""
	}
	return userOwnedExecutableDirectory(executable, home)
}

func userOwnedExecutableDirectory(executable, home string) string {
	executable, err := filepath.Abs(executable)
	if err != nil {
		return ""
	}
	home, err = filepath.Abs(home)
	if err != nil {
		return ""
	}
	directory := filepath.Dir(executable)
	relative, err := filepath.Rel(home, directory)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return ""
	}
	return directory
}

func withShellIntegration(lines []string, adapter ShellAdapter) []string {
	kept := withoutShellIntegration(lines)
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	kept = append(kept, "", "# Alias Lens "+adapter.DisplayName()+" integration", `eval "$(command alias-lens shell-init `+adapter.Name()+`)"`)
	return kept
}

func withoutShellIntegration(lines []string) []string {
	var kept []string
	for _, line := range lines {
		name, command, ok := parseAliasDefinition(line)
		trimmed := strings.TrimSpace(line)
		legacyAlias := ok && name == "al" && strings.Contains(command, "alias-lens")
		integration := trimmed == `eval "$(command alias-lens shell-init bash)"` || trimmed == `eval "$(command alias-lens shell-init zsh)"`
		if legacyAlias || integration || (strings.HasPrefix(trimmed, "# Alias Lens ") && strings.HasSuffix(trimmed, " integration")) {
			continue
		}
		kept = append(kept, line)
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	return kept
}

func ensureAliasFileExists(aliasPath string) error {
	if _, err := os.Stat(aliasPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	remoteAvailable := false
	if config.Repository != "" {
		remotePath, pathErr := repositoryFilePath(config.Repository, config.AliasFile)
		if pathErr != nil {
			return pathErr
		}
		if _, statErr := os.Stat(remotePath); statErr == nil {
			remoteAvailable = true
		} else if !os.IsNotExist(statErr) {
			return statErr
		}
	}
	if err := writeNewAliasFile(aliasPath, nil); err != nil {
		return err
	}
	if remoteAvailable {
		fmt.Println("Created", aliasDisplayPath(), "with mode 0600. Remote aliases were not activated; review them with al diff, then run al sync --pull.")
		return nil
	}
	fmt.Println("Created", aliasDisplayPath(), "with mode 0600.")
	return nil
}

func checkProviderSSH(provider RepoProvider) bool {
	ctx := context.Background()
	switch concrete := provider.(type) {
	case githubProvider:
		return useSSH(ctx, "auto", concrete.host)
	case bitbucketProvider:
		return useSSH(ctx, "auto", "bitbucket.org")
	case gitlabProvider:
		host := strings.TrimPrefix(strings.TrimPrefix(strings.TrimRight(concrete.host, "/"), "https://"), "http://")
		return useSSH(ctx, "auto", host)
	}
	return false
}
