package app

import (
	"bytes"
	"fmt"

	"os"
	"path/filepath"
	"sort"
	"strings"
)

var repositoryWriteBeforeOpen func()

func (svc *Services) configureRepository(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if output, err := gitOutput("-C", absolute, "rev-parse", "--is-inside-work-tree"); err != nil {
		return fmt.Errorf("%s is not a Git repository: %s", absolute, strings.TrimSpace(string(output)))
	}
	config, err := svc.loadConfig()
	if err != nil {
		return err
	}
	config.Repository = absolute
	config.AutoSync.Enabled = true
	if err := svc.withMutation(func(session *mutationSession) error {
		fresh, err := svc.loadConfig()
		if err != nil {
			return err
		}
		fresh.Repository = absolute
		fresh.AutoSync.Enabled = true
		return svc.saveConfigInSession(session, fresh)
	}); err != nil {
		return err
	}
	return svc.ensureWatchProcess()
}

func (svc *Services) syncRepository(push bool) (string, error) {
	var result string
	err := svc.withMutation(func(session *mutationSession) error {
		var err error
		result, err = svc.syncRepositoryInSession(session, push)
		return err
	})
	return result, err
}
func (svc *Services) syncRepositoryInSession(session *mutationSession, push bool) (string, error) {
	config, err := svc.loadConfig()
	if err != nil {
		return "", err
	}
	if err := svc.rejectCatalogFallbackSync(config); err != nil {
		return "", err
	}
	if config.Repository == "" {
		return "", fmt.Errorf("configure a repo first: al repo /path/to/dotfiles")
	}
	sourcePath, err := svc.aliasesPath()
	if err != nil {
		return "", err
	}
	if !push {
		state, stateErr := svc.loadSyncState()
		if stateErr != nil {
			return "", fmt.Errorf("read sync status: %w", stateErr)
		}
		if state.Status == "conflict" {
			return "", fmt.Errorf("sync conflict is unresolved; repository was not changed; run al diff, then use al sync --pull to import remote-only aliases or al sync --push to keep the local alias file")
		}
	}
	return svc.syncRepositoryFilesInSession(session, config, sourcePath, push)
}

type AliasConflict struct {
	Name   string
	Local  string
	Remote string
}

func (svc *Services) repositoryPaths() (AppConfig, string, string, error) {
	config, err := svc.loadConfig()
	if err != nil {
		return config, "", "", err
	}
	if config.Repository == "" {
		return config, "", "", fmt.Errorf("configure a repo first: al repo /path/to/dotfiles")
	}
	source, err := svc.aliasesPath()
	if err != nil {
		return config, "", "", err
	}
	target, err := repositoryFilePath(config.Repository, config.AliasFile)
	if err != nil {
		return config, "", "", err
	}
	return config, source, target, nil
}

func cleanRepositoryRelativePath(path, field string) (string, error) {
	cleaned := filepath.Clean(path)
	if cleaned == "." || filepath.IsAbs(cleaned) || cleaned == ".." || strings.HasPrefix(cleaned, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("%s must name a file inside the configured repository", field)
	}
	return cleaned, nil
}

func repositoryFilePath(repository, relative string) (string, error) {
	cleaned, err := cleanRepositoryRelativePath(relative, "repository path")
	if err != nil {
		return "", err
	}
	root, err := filepath.Abs(repository)
	if err != nil {
		return "", fmt.Errorf("resolve configured repository: %w", err)
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve configured repository: %w", err)
	}
	rootInfo, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect configured repository: %w", err)
	}
	if !rootInfo.IsDir() {
		return "", fmt.Errorf("configured repository is not a directory")
	}

	current := root
	parts := strings.Split(cleaned, string(filepath.Separator))
	for index, part := range parts {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if os.IsNotExist(statErr) {
			break
		}
		if statErr != nil {
			return "", fmt.Errorf("inspect repository path %s: %w", cleaned, statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return "", fmt.Errorf("repository path %s contains a symbolic link; choose a regular path inside the repository", cleaned)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return "", fmt.Errorf("repository path %s crosses a non-directory component", cleaned)
		}
		if index == len(parts)-1 && !info.Mode().IsRegular() {
			return "", fmt.Errorf("repository path %s is not a regular file", cleaned)
		}
	}
	return filepath.Join(root, cleaned), nil
}

func aliasCommandMap(contents []byte) map[string]string {
	commands := aliasDefinitionMap(contents)
	for _, function := range parseFunctions(string(contents)) {
		commands[function.Name] = function.Command
	}
	return commands
}

func aliasDefinitionMap(contents []byte) map[string]string {
	commands := map[string]string{}
	for _, line := range strings.Split(string(contents), "\n") {
		if name, command, ok := parseAliasDefinition(line); ok {
			commands[name] = command
		}
	}
	return commands
}

func compareAliasFiles(local, remote []byte) (localOnly, remoteOnly []string, conflicts []AliasConflict) {
	localCommands, remoteCommands := aliasCommandMap(local), aliasCommandMap(remote)
	for name, localCommand := range localCommands {
		remoteCommand, exists := remoteCommands[name]
		if !exists {
			localOnly = append(localOnly, name)
		} else if localCommand != remoteCommand {
			conflicts = append(conflicts, AliasConflict{Name: name, Local: localCommand, Remote: remoteCommand})
		}
	}
	for name := range remoteCommands {
		if _, exists := localCommands[name]; !exists {
			remoteOnly = append(remoteOnly, name)
		}
	}
	sort.Strings(localOnly)
	sort.Strings(remoteOnly)
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].Name < conflicts[j].Name })
	return
}

func (svc *Services) pullRepository() (string, error) {
	var result string
	err := svc.withMutation(func(session *mutationSession) error {
		var err error
		result, err = svc.pullRepositoryInSession(session)
		return err
	})
	return result, err
}
func (svc *Services) pullRepositoryInSession(session *mutationSession) (string, error) {
	config, source, target, err := svc.repositoryPaths()
	if err != nil {
		return "", err
	}
	if err := svc.rejectCatalogFallbackSync(config); err != nil {
		return "", err
	}
	if output, err := gitOutput("-C", config.Repository, "pull", "--ff-only"); err != nil {
		return "", fmt.Errorf("git pull failed without changing aliases: %s; retry with al sync --pull", strings.TrimSpace(string(output)))
	}
	target, err = repositoryFilePath(config.Repository, config.AliasFile)
	if err != nil {
		return "", err
	}
	local, err := readFileLimited(source, AliasFileLimit)
	localMissing := os.IsNotExist(err)
	if err != nil && !localMissing {
		return "", err
	}
	if localMissing {
		local = nil
	}
	remote, err := readFileLimited(target, AliasFileLimit)
	if os.IsNotExist(err) {
		return "Repository updated; it does not contain an alias file yet", nil
	}
	if err != nil {
		return "", err
	}
	if err := protectRepositoryAliasCopy(config.Repository, config.AliasFile, target, remote); err != nil {
		return "", err
	}
	_, remoteOnly, conflicts := compareAliasFiles(local, remote)
	if len(conflicts) > 0 {
		var names []string
		for _, conflict := range conflicts {
			names = append(names, conflict.Name)
		}
		return "", fmt.Errorf("alias conflicts: %s; run al diff to inspect both commands", strings.Join(names, ", "))
	}
	remoteAliases := aliasDefinitionMap(remote)
	if localMissing && len(remoteOnly) > 0 {
		if err := svc.writeNewAliasFile(source, nil); err != nil {
			return "", err
		}
	}
	var additions []aliasAddition
	skippedFunctions := 0
	for _, name := range remoteOnly {
		command, isAlias := remoteAliases[name]
		if !isAlias {
			skippedFunctions++
			continue
		}
		additions = append(additions, aliasAddition{Name: name, Command: command, Description: "Imported from the configured repository"})
	}
	if len(additions) > 0 {
		if err := svc.addAliasesToFileInSession(session, source, additions); err != nil {
			return "", err
		}
	}
	updated := local
	if len(additions) > 0 {
		updated, err = readFileLimited(source, AliasFileLimit)
		if err != nil {
			return "", fmt.Errorf("read imported aliases: %w", err)
		}
	}
	message := fmt.Sprintf("Repository pulled; imported %d aliases", len(additions))
	if skippedFunctions > 0 {
		message += fmt.Sprintf("; left %d remote functions unchanged for manual review", skippedFunctions)
	}
	if bytes.Equal(updated, remote) {
		message += "; active and repository alias files match"
	} else {
		message += "; active and repository alias files still differ. Run al diff to review what remains before pushing"
	}
	return message, nil
}

func (svc *Services) syncRepositoryFiles(config AppConfig, sourcePath string, push bool) (string, error) {
	var result string
	err := svc.withMutation(func(session *mutationSession) error {
		var err error
		result, err = svc.syncRepositoryFilesInSession(session, config, sourcePath, push)
		return err
	})
	return result, err
}
func (svc *Services) syncRepositoryFilesInSession(session *mutationSession, config AppConfig, sourcePath string, push bool) (string, error) {
	relative, err := cleanRepositoryRelativePath(config.AliasFile, "alias_file")
	if err != nil {
		return "", err
	}
	if output, err := gitOutput("-C", config.Repository, "rev-parse", "--is-inside-work-tree"); err != nil {
		return "", fmt.Errorf("configured repository is unavailable: %s", strings.TrimSpace(string(output)))
	}

	contents, err := readFileLimited(sourcePath, AliasFileLimit)
	if err != nil {
		return "", err
	}
	if err := svc.secretFindingsError(findSecretFindings(contents)); err != nil {
		return "", err
	}
	target, err := repositoryFilePath(config.Repository, relative)
	if err != nil {
		return "", err
	}
	existing, readErr := readFileLimited(target, AliasFileLimit)
	if readErr != nil && !os.IsNotExist(readErr) {
		return "", readErr
	}
	changed := !bytes.Equal(contents, existing)
	if changed {
		if err := writeRepositoryFile(config.Repository, relative, contents, 0o600); err != nil {
			return "", err
		}
	} else if err := protectRepositoryAliasCopy(config.Repository, relative, target, existing); err != nil {
		return "", err
	}
	if changed {
		if output, err := gitOutput("-C", config.Repository, "add", "--", relative); err != nil {
			return "", fmt.Errorf("git add failed: %s", strings.TrimSpace(string(output)))
		}
		if output, err := gitOutput("-C", config.Repository, "commit", "--only", "-m", "Update shell aliases", "--", relative); err != nil {
			return "", fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(output)))
		}
	}
	if push {
		if err := pushRepository(config); err != nil {
			return "", fmt.Errorf("push aliases: %w; retry with al sync --push", err)
		}
		return "Aliases committed and pushed", nil
	}
	if changed {
		return "Aliases committed locally", nil
	}
	return "Repository already matches " + svc.aliasDisplayPath(), nil
}

func protectRepositoryAliasCopy(repository, relative, target string, contents []byte) error {
	info, err := os.Stat(target)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if info.Mode().Perm() == 0o600 {
		return nil
	}
	return writeRepositoryFile(repository, relative, contents, 0o600)
}

func scanOutgoingAliasHistory(repository, relative string) error {
	return scanOutgoingFileHistory(repository, relative, "alias")
}

func scanOutgoingFileHistory(repository, relative, label string) error {
	output, err := gitOutput("-C", repository, "rev-list", "HEAD", "--not", "--remotes", "--", relative)
	if err != nil {
		return fmt.Errorf("inspect outgoing %s history before push: %s", label, cleanCommandOutput(output))
	}
	for _, revision := range strings.Fields(string(output)) {
		pathAtRevision := revision + ":" + filepath.ToSlash(relative)
		contents, showErr := gitOutput("-C", repository, "show", pathAtRevision)
		if showErr != nil {
			continue
		}
		if findings := findSecretFindings(contents); len(findings) > 0 {
			return fmt.Errorf("push blocked because an outgoing %s commit may contain a secret; remove the commit from local history, then run al sync --push", label)
		}
	}
	return nil
}

type repositoryPushPlan struct {
	Remote      string
	Destination string
	Snapshot    string
	RemoteBase  string
}

func pushRepository(config AppConfig) error {
	plan, err := prepareRepositoryPush(config)
	if err != nil {
		return err
	}
	refspec := plan.Snapshot + ":" + plan.Destination
	lease := "--force-with-lease=" + plan.Destination + ":" + plan.RemoteBase
	if output, err := gitOutput("-C", config.Repository, "push", "--porcelain", "--no-follow-tags", lease, "--", plan.Remote, refspec); err != nil {
		return fmt.Errorf("git push failed: %s", cleanCommandOutput(output))
	}
	return nil
}

func prepareRepositoryPush(config AppConfig) (repositoryPushPlan, error) {
	branchOutput, err := gitOutput("-C", config.Repository, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err != nil {
		return repositoryPushPlan{}, fmt.Errorf("push requires a checked-out branch")
	}
	branch := strings.TrimSpace(string(branchOutput))
	remoteOutput, err := gitOutput("-C", config.Repository, "config", "--get", "branch."+branch+".remote")
	if err != nil || strings.TrimSpace(string(remoteOutput)) == "" {
		return repositoryPushPlan{}, fmt.Errorf("branch %s has no upstream remote; publish it manually, then retry", branch)
	}
	remote := strings.TrimSpace(string(remoteOutput))
	mergeOutput, err := gitOutput("-C", config.Repository, "config", "--get", "branch."+branch+".merge")
	if err != nil {
		return repositoryPushPlan{}, fmt.Errorf("branch %s has no upstream branch; publish it manually, then retry", branch)
	}
	destination := strings.TrimSpace(string(mergeOutput))
	if !strings.HasPrefix(destination, "refs/heads/") {
		return repositoryPushPlan{}, fmt.Errorf("branch %s has an unsupported upstream ref", branch)
	}
	if output, err := gitOutput("-C", config.Repository, "fetch", "--no-tags", "--force", "--", remote, destination); err != nil {
		return repositoryPushPlan{}, fmt.Errorf("refresh upstream %s before push: %s", destination, cleanCommandOutput(output))
	}
	snapshotOutput, err := gitReviewOutputBounded(1024, config.Repository, "rev-parse", "--verify", "HEAD^{commit}")
	if err != nil {
		return repositoryPushPlan{}, fmt.Errorf("resolve current commit: %w", err)
	}
	snapshot := strings.TrimSpace(string(snapshotOutput))
	upstreamOutput, err := gitReviewOutputBounded(1024, config.Repository, "rev-parse", "--verify", "FETCH_HEAD^{commit}")
	if err != nil {
		return repositoryPushPlan{}, fmt.Errorf("resolve refreshed upstream for branch %s: %w", branch, err)
	}
	upstream := strings.TrimSpace(string(upstreamOutput))
	if _, err := gitReviewOutputBounded(1024, config.Repository, "merge-base", "--is-ancestor", upstream, snapshot); err != nil {
		return repositoryPushPlan{}, fmt.Errorf("push blocked because the remote branch diverged from the local branch; reconcile the branch, then retry")
	}
	if err := validateOutgoingRepositoryHistory(config, upstream, snapshot); err != nil {
		return repositoryPushPlan{}, err
	}
	return repositoryPushPlan{Remote: remote, Destination: destination, Snapshot: snapshot, RemoteBase: upstream}, nil
}

func validateOutgoingRepositoryHistory(config AppConfig, upstream, snapshot string) error {
	allowed, err := repositoryPushAllowlist(config)
	if err != nil {
		return err
	}
	revisionsOutput, err := gitReviewOutputBounded(8<<20, config.Repository, "rev-list", "--reverse", upstream+".."+snapshot)
	if err != nil {
		return fmt.Errorf("inspect outgoing commits: %w", err)
	}
	for _, revision := range strings.Fields(string(revisionsOutput)) {
		pathsOutput, err := gitReviewOutputBounded(8<<20, config.Repository, "diff-tree", "--root", "-m", "--no-commit-id", "--name-only", "--no-renames", "-r", "-z", revision)
		if err != nil {
			return fmt.Errorf("inspect outgoing commit %s: %w", revision, err)
		}
		for _, rawPath := range bytes.Split(pathsOutput, []byte{0}) {
			if len(rawPath) == 0 {
				continue
			}
			path := string(rawPath)
			limit, ok := allowed[path]
			if !ok {
				return fmt.Errorf("push blocked because outgoing commit %s changes untracked path %s; publish that commit separately, then retry", revision, terminalSafeText(path))
			}
			treeOutput, treeErr := gitReviewOutputBounded(8<<20, config.Repository, "ls-tree", "-z", revision, "--", ":(literal)"+path)
			if treeErr != nil {
				return fmt.Errorf("inspect outgoing file %s: %w", terminalSafeText(path), treeErr)
			}
			if len(treeOutput) == 0 {
				continue
			}
			entry := bytes.TrimSuffix(treeOutput, []byte{0})
			metadata, entryPath, found := bytes.Cut(entry, []byte{'\t'})
			fields := strings.Fields(string(metadata))
			if !found || string(entryPath) != path || len(fields) != 3 || fields[1] != "blob" || (fields[0] != "100644" && fields[0] != "100755") {
				return fmt.Errorf("push blocked because %s is not a regular file in outgoing commit %s", terminalSafeText(path), revision)
			}
			object := revision + ":" + path
			contents, showErr := gitReviewOutputBounded(limit, config.Repository, "show", object)
			if showErr != nil {
				return fmt.Errorf("inspect outgoing file %s: %w", terminalSafeText(path), showErr)
			}
			if findings := findSecretFindings(contents); len(findings) > 0 {
				return fmt.Errorf("push blocked because outgoing commit %s may contain a secret in %s; remove it from local history, then retry", revision, terminalSafeText(path))
			}
		}
	}
	return nil
}

func gitReviewOutputBounded(limit int, repository string, arguments ...string) ([]byte, error) {
	commandArguments := []string{"--no-replace-objects", "-C", repository}
	commandArguments = append(commandArguments, arguments...)
	return gitOutputBounded(limit, commandArguments...)
}

func repositoryPushAllowlist(config AppConfig) (map[string]int, error) {
	allowed := make(map[string]int)
	aliasPath, err := cleanRepositoryRelativePath(config.AliasFile, "alias_file")
	if err != nil {
		return nil, err
	}
	allowed[filepath.ToSlash(aliasPath)] = AliasFileLimit
	for _, tracked := range config.TrackedFiles {
		path, err := cleanRepositoryRelativePath(tracked.RepositoryPath, "tracked repository path")
		if err != nil {
			return nil, err
		}
		allowed[filepath.ToSlash(path)] = trackedFileLimit
	}
	return allowed, nil
}
