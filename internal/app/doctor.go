package app

import (
	"alias-lens/internal/providers"
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

func (svc *Services) doctorChecks() []DoctorCheck {
	var checks []DoctorCheck
	adapter := svc.activeShellAdapter()
	executable, executableErr := svc.dependencies.Executable()
	checks = append(checks, DoctorCheck{Name: "binary", OK: executableErr == nil, Message: defaultString(executable, "not found")})
	pathExecutable, pathErr := exec.LookPath("alias-lens")
	pathMessage := pathExecutable
	if pathErr != nil {
		pathMessage = "run " + defaultString(executable, "alias-lens") + " setup " + adapter.Name() + ", then start a new shell"
	}
	checks = append(checks, DoctorCheck{Name: "binary on PATH", OK: pathErr == nil, Message: pathMessage})
	aliasPath, aliasErr := svc.aliasesPath()
	_, aliasStatErr := os.Stat(aliasPath)
	checks = append(checks, DoctorCheck{Name: adapter.AliasFilename(), OK: aliasErr == nil && aliasStatErr == nil, Message: aliasPath})
	if aliasErr == nil && aliasStatErr == nil {
		checks = append(checks, svc.aliasSyntaxDoctorCheck(aliasPath, adapter.Name()))
	}

	home, _ := svc.dependencies.HomeDir()
	startupOK, startupMessage := adapter.StartupStatus(home, runtime.GOOS)
	checks = append(checks, DoctorCheck{Name: adapter.DisplayName() + " loading", OK: startupOK, Message: startupMessage})
	aliases, _ := os.ReadFile(aliasPath)
	hasIntegration := strings.Contains(string(aliases), "shell-init "+adapter.Name())
	if active, _ := svc.catalogManagedEditing(); active {
		ok, message := svc.catalogDoctorRuntime(adapter.Name())
		hasIntegration = ok
		checks = append(checks, DoctorCheck{Name: "catalog runtime", OK: ok, Message: message})
		for i := range checks {
			if checks[i].Name == "binary on PATH" {
				checks[i].OK = true
				checks[i].Message = "catalog loader uses its pinned executable"
			}
		}
	}
	setupCommand := "run al setup " + adapter.Name()
	actionsMessage := setupCommand
	if hasIntegration {
		actionsMessage = svc.shellActionsMessage(adapter)
	}
	checks = append(checks, DoctorCheck{Name: "shell actions", OK: hasIntegration, Message: actionsMessage})

	_, gitErr := exec.LookPath("git")
	checks = append(checks, DoctorCheck{Name: "Git", OK: gitErr == nil, Message: map[bool]string{true: "installed", false: "install Git"}[gitErr == nil]})
	config, configErr := svc.loadConfig()
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
		state, _ := svc.loadSyncState()
		syncOK := state.Status != "conflict" && state.Status != "offline" && state.Status != ""
		message := defaultString(state.Status, "run al watch")
		if state.Message != "" {
			message += ": " + state.Message
		}
		checks = append(checks, DoctorCheck{Name: "automatic sync", OK: syncOK, Message: message})
	} else {
		checks = append(checks, DoctorCheck{Name: "automatic sync", OK: false, Message: "run al autosync enable"})
	}

	for _, provider := range svc.configuredProviders(config) {
		ok, message := svc.providerCredentialStatus(provider)
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

func (svc *Services) providerCredentialStatus(provider providers.RepoProvider) (bool, string) {
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
		if svc.dependencies.Environment("BITBUCKET_API_TOKEN") == "" {
			return false, "run al repo bitbucket"
		}
		return true, "API token available"
	case "gitlab":
		if svc.dependencies.Environment("GITLAB_TOKEN") != "" {
			return true, "API token available"
		}
		glab, err := exec.LookPath("glab")
		if err != nil {
			return false, "install glab, then run al repo gitlab"
		}
		host := provider.Host()
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
	SetupInstall setupAction = iota
	SetupRepair
	SetupRemove
)

func (svc *Services) performSetup(request SetupRequest) (SetupResult, error) {
	result := SetupResult{}
	err := svc.withMutation(func(session *mutationSession) error { return svc.runSetupInSession(session, request, &result) })
	return result, err
}

func (svc *Services) runSetupInSession(session *mutationSession, request SetupRequest, result *SetupResult) error {
	shellName, repair := request.Shell, request.Repair
	adapter, err := svc.requestedShellAdapter(shellName)
	if err != nil {
		return err
	}
	config, err := svc.loadConfig()
	if err != nil {
		return err
	}
	oldAdapter, _ := svc.shellAdapter(config.Shell)
	if oldAdapter != nil && config.AliasFile == oldAdapter.AliasFilename() {
		config.AliasFile = adapter.AliasFilename()
	}
	config.Shell = adapter.Name()
	if err := svc.saveConfigInSession(session, config); err != nil {
		return err
	}
	if shellName == "" {
		detected := filepath.Base(strings.TrimSpace(svc.dependencies.Environment(activeShellEnvironment)))
		if detected == "." || detected == "" {
			detected = filepath.Base(strings.TrimSpace(svc.dependencies.Environment("SHELL")))
		}
		if detected == adapter.Name() {
			result.Notices = append(result.Notices, OperationNotice{Message: fmt.Sprintf("Detected %s from the current shell environment.", adapter.DisplayName())})
		} else {
			result.Notices = append(result.Notices, OperationNotice{Message: fmt.Sprintf("Using configured shell %s. Pass bash or zsh to override it.", adapter.DisplayName())})
		}
	}
	aliasPath, err := svc.aliasPathFor(adapter)
	if err != nil {
		return err
	}
	if err := svc.ensureAliasFileExistsInSession(session, aliasPath, result); err != nil {
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
		if repair {
			result.Notices = append(result.Notices, OperationNotice{Message: fmt.Sprintf("Verified Alias Lens %s alias integration.", adapter.DisplayName()), Styled: true})
		} else {
			result.Notices = append(result.Notices, OperationNotice{Message: fmt.Sprintf("Alias Lens %s integration is already installed.", adapter.DisplayName()), Styled: true})
		}
	} else {
		if err := svc.writeAliasFileInSession(session, aliasPath, contents, updated, 0o600); err != nil {
			return err
		}
		verb := "Installed"
		if repair {
			verb = "Repaired"
		}
		result.Notices = append(result.Notices, OperationNotice{Message: fmt.Sprintf("%s Alias Lens %s integration.", verb, adapter.DisplayName()), Styled: true})
	}
	result.Notices = append(result.Notices, OperationNotice{Message: svc.shellSetupInstruction(adapter)})
	home := filepath.Dir(aliasPath)
	if err := adapter.ConfigureStartupInSession(session, home, runtime.GOOS, svc.userExecutableDirectory(home)); err != nil {
		return err
	}
	if repair {
		result.Notices = append(result.Notices, OperationNotice{Message: "Alias Lens kept your aliases and checked only its generated integration."})
		return nil
	}
	if err := svc.scheduleTourInSession(session); err != nil {
		return fmt.Errorf("save first-run tour state: %w", err)
	}
	if !request.Interactive {
		result.Notices = append(result.Notices, OperationNotice{Message: "Optional developer aliases were not reviewed because input is not interactive. Run al setup in a terminal to review them."})
		return nil
	}
	result.AliasPath = aliasPath
	return nil
}

func (svc *Services) shellActionsMessage(adapter ShellAdapter) string {
	launcher := svc.configuredLauncherLabel()
	if adapter.Name() == "bash" {
		if supported, known := bashCtrlGSupport(); known && !supported {
			return "Enter and al use are enabled; run al because " + launcher + " requires Bash 4 or newer"
		}
	}
	return "Enter, al use, and " + launcher + " are enabled"
}

func (svc *Services) shellSetupInstruction(adapter ShellAdapter) string {
	launcher := svc.configuredLauncherLabel()
	if adapter.Name() == "bash" {
		if supported, known := bashCtrlGSupport(); known && !supported {
			return "Start a new bash shell, then run al. " + launcher + " requires Bash 4 or newer."
		}
	}
	return "Start a new " + adapter.Name() + " shell, then press " + launcher + " on an empty prompt."
}

func (svc *Services) configuredLauncherLabel() string {
	config, err := svc.loadConfig()
	if err != nil {
		return "Ctrl+G"
	}
	return launcherLabel(config)
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

func (svc *Services) performRemoveSetup(shellName string) (SetupRemovalResult, error) {
	result := SetupRemovalResult{}
	err := svc.withMutation(func(session *mutationSession) error { return svc.removeSetupInSession(session, shellName, &result) })
	return result, err
}

func (svc *Services) removeSetupInSession(session *mutationSession, shellName string, result *SetupRemovalResult) error {
	adapter, err := svc.requestedShellAdapter(shellName)
	if err != nil {
		return err
	}
	aliasPath, err := svc.aliasPathFor(adapter)
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
			if err := svc.writeAliasFileInSession(session, aliasPath, contents, updated, 0o600); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(readErr) {
		return readErr
	}
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return err
	}
	if err := adapter.RemoveStartupInSession(session, home, runtime.GOOS); err != nil {
		return err
	}
	result.DisplayName, result.Shell = adapter.DisplayName(), adapter.Name()
	return nil
}

func (svc *Services) userExecutableDirectory(home string) string {
	executable, err := svc.dependencies.Executable()
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

func (svc *Services) ensureAliasFileExistsInSession(session *mutationSession, aliasPath string, result *SetupResult) error {
	if _, err := os.Stat(aliasPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	config, err := svc.loadConfig()
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
	if err := session.writeUserFile(aliasPath, nil, nil, 0o600, true, 0, false); err != nil {
		return err
	}
	if remoteAvailable {
		result.Notices = append(result.Notices, OperationNotice{Message: "Created " + svc.aliasDisplayPath() + " with mode 0600. Remote aliases were not activated; review them with al diff, then run al sync --pull."})
		return nil
	}
	result.Notices = append(result.Notices, OperationNotice{Message: "Created " + svc.aliasDisplayPath() + " with mode 0600."})
	return nil
}

func checkProviderSSH(provider providers.RepoProvider) bool {
	return provider.SSHAvailable(context.Background())
}
