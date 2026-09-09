package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func configureRepository(path string) error {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if output, err := exec.Command("git", "-C", absolute, "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil {
		return fmt.Errorf("%s is not a Git repository: %s", absolute, strings.TrimSpace(string(output)))
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	config.Repository = absolute
	return saveConfig(config)
}

func syncRepository(push bool) (string, error) {
	config, err := loadConfig()
	if err != nil {
		return "", err
	}
	if config.Repository == "" {
		return "", fmt.Errorf("configure a repo first: al repo /path/to/dotfiles")
	}
	sourcePath, err := aliasesPath()
	if err != nil {
		return "", err
	}
	return syncRepositoryFiles(config, sourcePath, push)
}

type AliasConflict struct {
	Name   string
	Local  string
	Remote string
}

func repositoryPaths() (AppConfig, string, string, error) {
	config, err := loadConfig()
	if err != nil {
		return config, "", "", err
	}
	if config.Repository == "" {
		return config, "", "", fmt.Errorf("configure a repo first: al repo /path/to/dotfiles")
	}
	source, err := aliasesPath()
	if err != nil {
		return config, "", "", err
	}
	relative := filepath.Clean(config.AliasFile)
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return config, "", "", fmt.Errorf("alias_file must stay inside the configured repository")
	}
	return config, source, filepath.Join(config.Repository, relative), nil
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

func showRepositoryDiff() error {
	_, source, target, err := repositoryPaths()
	if err != nil {
		return err
	}
	local, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	remote, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		remote = nil
	} else if err != nil {
		return err
	}
	localOnly, remoteOnly, conflicts := compareAliasFiles(local, remote)
	if len(localOnly)+len(remoteOnly)+len(conflicts) == 0 {
		fmt.Println("The local and tracked alias files match.")
		return nil
	}
	for _, name := range localOnly {
		fmt.Println("LOCAL ONLY ", name)
	}
	for _, name := range remoteOnly {
		fmt.Println("REMOTE ONLY", name)
	}
	for _, conflict := range conflicts {
		fmt.Printf("CHANGED    %s\n  local:  %s\n  remote: %s\n", conflict.Name, conflict.Local, conflict.Remote)
	}
	return nil
}

func pullRepository() (string, error) {
	config, source, target, err := repositoryPaths()
	if err != nil {
		return "", err
	}
	if output, err := exec.Command("git", "-C", config.Repository, "pull", "--ff-only").CombinedOutput(); err != nil {
		return "", fmt.Errorf("git pull failed without changing aliases: %s", strings.TrimSpace(string(output)))
	}
	local, err := os.ReadFile(source)
	if err != nil {
		return "", err
	}
	remote, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		return "Repository updated; it does not contain an alias file yet", nil
	}
	if err != nil {
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
	imported := 0
	skippedFunctions := 0
	for _, name := range remoteOnly {
		command, isAlias := remoteAliases[name]
		if !isAlias {
			skippedFunctions++
			continue
		}
		if err := addAliasToFile(source, name, command, "Imported from the configured repository"); err != nil {
			return "", err
		}
		imported++
	}
	if imported == 0 && skippedFunctions == 0 {
		return "Repository pulled; no new aliases were found", nil
	}
	message := fmt.Sprintf("Repository pulled; imported %d aliases", imported)
	if skippedFunctions > 0 {
		message += fmt.Sprintf("; left %d remote functions unchanged for manual review", skippedFunctions)
	}
	return message, nil
}

func syncRepositoryFiles(config AppConfig, sourcePath string, push bool) (string, error) {
	relative := filepath.Clean(config.AliasFile)
	if filepath.IsAbs(relative) || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("alias_file must stay inside the configured repository")
	}
	if output, err := exec.Command("git", "-C", config.Repository, "rev-parse", "--is-inside-work-tree").CombinedOutput(); err != nil {
		return "", fmt.Errorf("configured repository is unavailable: %s", strings.TrimSpace(string(output)))
	}

	contents, err := os.ReadFile(sourcePath)
	if err != nil {
		return "", err
	}
	if push {
		if err := secretFindingsError(findSecretFindings(contents)); err != nil {
			return "", err
		}
	}
	target := filepath.Join(config.Repository, relative)
	existing, _ := os.ReadFile(target)
	changed := !bytes.Equal(contents, existing)
	if changed {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(target, contents, 0o644); err != nil {
			return "", err
		}
		if output, err := exec.Command("git", "-C", config.Repository, "add", "--", relative).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git add failed: %s", strings.TrimSpace(string(output)))
		}
		if output, err := exec.Command("git", "-C", config.Repository, "commit", "--only", "-m", "Update shell aliases", "--", relative).CombinedOutput(); err != nil {
			return "", fmt.Errorf("git commit failed: %s", strings.TrimSpace(string(output)))
		}
	}
	if push {
		if output, err := exec.Command("git", "-C", config.Repository, "push").CombinedOutput(); err != nil {
			return "", fmt.Errorf("git push failed: %s", strings.TrimSpace(string(output)))
		}
		return "Aliases committed and pushed", nil
	}
	if changed {
		return "Aliases committed locally", nil
	}
	return "Repository already matches ~/.bash_aliases", nil
}
