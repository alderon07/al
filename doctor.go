package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
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
	aliasPath, aliasErr := aliasesPath()
	_, aliasStatErr := os.Stat(aliasPath)
	checks = append(checks, DoctorCheck{Name: adapter.AliasFilename(), OK: aliasErr == nil && aliasStatErr == nil, Message: aliasPath})

	home, _ := os.UserHomeDir()
	startupOK, startupMessage := adapter.StartupStatus(home, runtime.GOOS)
	checks = append(checks, DoctorCheck{Name: adapter.DisplayName() + " loading", OK: startupOK, Message: startupMessage})
	aliases, _ := os.ReadFile(aliasPath)
	hasIntegration := strings.Contains(string(aliases), "shell-init "+adapter.Name())
	setupCommand := "run al setup " + adapter.Name()
	checks = append(checks, DoctorCheck{Name: "shell actions", OK: hasIntegration, Message: map[bool]string{true: "Enter, al use, and Ctrl+G are enabled", false: setupCommand}[hasIntegration]})

	_, gitErr := exec.LookPath("git")
	checks = append(checks, DoctorCheck{Name: "Git", OK: gitErr == nil, Message: map[bool]string{true: "installed", false: "install Git"}[gitErr == nil]})
	config, configErr := loadConfig()
	repoOK := configErr == nil && config.Repository != ""
	repoMessage := "run al repo"
	if repoOK {
		if output, err := exec.Command("git", "-C", config.Repository, "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil {
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
		if err := exec.Command("gh", "auth", "status").Run(); err != nil {
			return false, "run gh auth login"
		}
		return true, "GitHub CLI authenticated"
	case "bitbucket":
		if os.Getenv("BITBUCKET_API_TOKEN") == "" {
			return false, "set BITBUCKET_API_TOKEN"
		}
		return true, "API token available"
	case "gitlab":
		if os.Getenv("GITLAB_TOKEN") == "" {
			return false, "set GITLAB_TOKEN"
		}
		return true, "API token available"
	default:
		return false, "unknown provider"
	}
}

func runSetup(shellName string) error {
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
		fmt.Printf("Alias Lens %s integration is already installed.\n", adapter.DisplayName())
	} else {
		if err := writeAliasFile(aliasPath, contents, updated, 0o600); err != nil {
			return err
		}
		fmt.Printf("Installed Alias Lens %s integration. Start a new %s shell to use Enter, al use, and Ctrl+G.\n", adapter.DisplayName(), adapter.Name())
	}
	if err := adapter.ConfigureStartup(filepath.Dir(aliasPath), runtime.GOOS); err != nil {
		return err
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

func withShellIntegration(lines []string, adapter ShellAdapter) []string {
	var kept []string
	for _, line := range lines {
		name, _, ok := parseAliasDefinition(line)
		trimmed := strings.TrimSpace(line)
		if (ok && name == "al") || strings.Contains(line, "shell-init ") || (strings.HasPrefix(trimmed, "# Alias Lens ") && strings.HasSuffix(trimmed, " integration")) {
			continue
		}
		kept = append(kept, line)
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	kept = append(kept, "", "# Alias Lens "+adapter.DisplayName()+" integration", `eval "$(command alias-lens shell-init `+adapter.Name()+`)"`)
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
	if config.Repository != "" {
		remotePath := filepath.Join(config.Repository, filepath.Clean(config.AliasFile))
		if remote, readErr := os.ReadFile(remotePath); readErr == nil {
			if err := writeNewAliasFile(aliasPath, remote); err != nil {
				return err
			}
			fmt.Println("Restored", aliasDisplayPath(), "from the configured repository.")
			return nil
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
	}
	if err := writeNewAliasFile(aliasPath, nil); err != nil {
		return err
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
