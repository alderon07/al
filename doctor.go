package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
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
	executable, executableErr := os.Executable()
	checks = append(checks, DoctorCheck{Name: "binary", OK: executableErr == nil, Message: defaultString(executable, "not found")})
	aliasPath, aliasErr := aliasesPath()
	_, aliasStatErr := os.Stat(aliasPath)
	checks = append(checks, DoctorCheck{Name: ".bash_aliases", OK: aliasErr == nil && aliasStatErr == nil, Message: aliasPath})

	home, _ := os.UserHomeDir()
	bashrcPath := filepath.Join(home, ".bashrc")
	bashrc, _ := os.ReadFile(bashrcPath)
	sourcesAliases := strings.Contains(string(bashrc), ".bash_aliases")
	checks = append(checks, DoctorCheck{Name: "Bash loading", OK: sourcesAliases, Message: map[bool]string{true: ".bashrc loads .bash_aliases", false: "run al setup"}[sourcesAliases]})
	aliases, _ := os.ReadFile(aliasPath)
	hasIntegration := strings.Contains(string(aliases), "shell-init bash")
	checks = append(checks, DoctorCheck{Name: "shell actions", OK: hasIntegration, Message: map[bool]string{true: "al use and Ctrl+G are enabled", false: "run al setup"}[hasIntegration]})

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

func runSetup() error {
	aliasPath, err := aliasesPath()
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
	kept := withShellIntegration(lines)
	updated := []byte(strings.Join(kept, "\n") + "\n")
	if string(updated) == string(contents) {
		if err := os.Chmod(aliasPath, 0o600); err != nil {
			return err
		}
		fmt.Println("Alias Lens shell integration is already installed.")
		return nil
	}
	if err := writeAliasFile(aliasPath, contents, updated, 0o600); err != nil {
		return err
	}
	if err := ensureBashLoadsAliases(filepath.Join(filepath.Dir(aliasPath), ".bashrc")); err != nil {
		return err
	}
	fmt.Println("Installed Alias Lens shell integration. Start a new Bash shell to use al use and Ctrl+G.")
	return nil
}

func withShellIntegration(lines []string) []string {
	var kept []string
	for _, line := range lines {
		name, _, ok := parseAliasDefinition(line)
		if (ok && name == "al") || strings.Contains(line, "shell-init bash") || strings.TrimSpace(line) == "# Alias Lens shell integration" {
			continue
		}
		kept = append(kept, line)
	}
	for len(kept) > 0 && strings.TrimSpace(kept[len(kept)-1]) == "" {
		kept = kept[:len(kept)-1]
	}
	kept = append(kept, "", "# Alias Lens shell integration", `eval "$(command alias-lens shell-init bash)"`)
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
			fmt.Println("Restored .bash_aliases from the configured repository.")
			return nil
		} else if !os.IsNotExist(readErr) {
			return readErr
		}
	}
	if err := writeNewAliasFile(aliasPath, nil); err != nil {
		return err
	}
	fmt.Println("Created ~/.bash_aliases with mode 0600.")
	return nil
}

func ensureBashLoadsAliases(path string) error {
	contents, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	if strings.Contains(string(contents), ".bash_aliases") {
		return nil
	}
	if len(contents) > 0 {
		if err := os.WriteFile(path+".alias-lens.bak", contents, 0o600); err != nil {
			return err
		}
	}
	block := "\n# Load personal aliases.\nif [ -f \"$HOME/.bash_aliases\" ]; then\n  . \"$HOME/.bash_aliases\"\nfi\n"
	return os.WriteFile(path, append(contents, []byte(block)...), 0o644)
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
