//go:build !windows

package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
)

func (svc *Services) catalogShellInstalled(shell string) (bool, error) {
	records := catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{}}
	_, err := readLifecycleFile(svc.catalogInstalledPath(), &records)
	if err != nil {
		return false, err
	}
	for _, record := range records.Records {
		if record.Shell == shell {
			return true, nil
		}
	}
	return false, nil
}

func (svc *Services) catalogAlias(entry neutralcatalog.Entry, shell, state string) Alias {
	command := ""
	kind := "function"
	if implementation, exists := entry.Native[shell]; exists {
		if implementation.AliasValue != nil {
			command = *implementation.AliasValue
			kind = "alias"
		} else if implementation.FunctionBody != nil {
			command = *implementation.FunctionBody
		}
	}
	if command == "" && entry.Portable != nil {
		words := []string{entry.Portable.Program}
		for _, arg := range entry.Portable.Args {
			words = append(words, quoteShadow(arg))
		}
		command = strings.Join(words, " ")
	}
	return Alias{Name: entry.Name, Command: command, Description: entry.Description, Category: entry.Category, Type: kind, Tags: entry.Tags, Platforms: entry.Platforms, Favorite: entry.Favorite, CatalogID: entry.ID, CatalogState: state}
}

func catalogImplementationMatches(a, b neutralcatalog.Entry, shell string) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Kind == b.Kind && reflect.DeepEqual(a.Native[shell], b.Native[shell]) && reflect.DeepEqual(a.Portable, b.Portable)
}

func (svc *Services) catalogEntryFacade(shell string, native []Alias) ([]Alias, error) {
	active, err := svc.catalogShellInstalled(shell)
	if err != nil {
		return nil, err
	}
	if !active {
		return native, nil
	}
	adapter, err := svc.shellAdapter(shell)
	if err != nil {
		return nil, err
	}
	installed, err := svc.readCatalogGeneration(svc.catalogGeneratedRoot(shell), filepath.Join(svc.homeDirectory(), adapter.AliasFilename()), shell)
	if err != nil {
		return nil, fmt.Errorf("installed catalog cannot be verified; use al catalog rollback")
	}
	declared, err := readCatalogFile(svc.localCatalogPath())
	if err != nil {
		return nil, err
	}
	observed, err := svc.observeConfig()
	if err != nil {
		return nil, err
	}
	resolved, problems := neutralcatalog.Resolve(declared, neutralcatalog.ResolveContext{Shell: shell, Platform: svc.currentPlatform(), Profiles: observed.Config.Profiles})
	if len(problems) > 0 {
		return nil, fmt.Errorf("catalog is invalid")
	}
	installedByID := map[string]catalogstore.GenerationEntry{}
	managedNames := map[string]bool{}
	for _, item := range installed.Entries {
		installedByID[item.Entry.ID] = item
		managedNames[item.Entry.Name] = true
	}
	result := []Alias{}
	for _, item := range resolved {
		state := "pending"
		if old, exists := installedByID[item.Entry.ID]; exists && catalogImplementationMatches(item.Entry, old.Entry, shell) {
			state = "installed"
		} else if !item.Available {
			state = "unavailable"
		}
		alias := svc.catalogAlias(item.Entry, shell, state)
		if state != "installed" {
			alias.Issues = append(alias.Issues, "catalog "+state+"; enter al catalog enable --shell "+shell)
		}
		result = append(result, alias)
		managedNames[item.Entry.Name] = true
		delete(installedByID, item.Entry.ID)
	}
	for _, item := range installedByID {
		alias := svc.catalogAlias(item.Entry, shell, "installed-removed")
		alias.Issues = append(alias.Issues, "catalog deletion pending; enter al catalog enable --shell "+shell)
		result = append(result, alias)
	}
	for _, alias := range native {
		if !managedNames[alias.Name] {
			alias.CatalogState = "native-only"
			result = append(result, alias)
		}
	}
	sort.Slice(result, func(i, j int) bool { return strings.ToLower(result[i].Name) < strings.ToLower(result[j].Name) })
	return result, nil
}

func (svc *Services) catalogInstalledDeclaration(name, shell string) (string, bool, error) {
	active, err := svc.catalogShellInstalled(shell)
	if err != nil || !active {
		return "", false, err
	}
	adapter, err := svc.shellAdapter(shell)
	if err != nil {
		return "", true, err
	}
	manifest, err := svc.readCatalogGeneration(svc.catalogGeneratedRoot(shell), filepath.Join(svc.homeDirectory(), adapter.AliasFilename()), shell)
	if err != nil {
		return "", true, err
	}
	value, err := readCatalogFile(svc.localCatalogPath())
	if err != nil {
		return "", true, err
	}
	for _, item := range manifest.Entries {
		if item.Entry.Name == name {
			for _, entry := range value.Entries {
				if entry.ID == item.Entry.ID {
					if !catalogImplementationMatches(entry, item.Entry, shell) {
						return "", true, fmt.Errorf("catalog change is pending; enter al catalog enable --shell %s", shell)
					}
					return catalogRuntimeHandoff(shell, []catalogstore.GenerationEntry{item}), true, nil
				}
			}
			return catalogRuntimeHandoff(shell, []catalogstore.GenerationEntry{item}), true, nil
		}
	}
	for _, entry := range value.Entries {
		if entry.Name == name {
			return "", true, fmt.Errorf("catalog entry is pending; enter al catalog enable --shell %s", shell)
		}
	}
	return "", false, nil
}

func (svc *Services) catalogInstalledCompletionNames(shell string) ([]string, bool, error) {
	active, err := svc.catalogShellInstalled(shell)
	if err != nil || !active {
		return nil, false, err
	}
	adapter, err := svc.shellAdapter(shell)
	if err != nil {
		return nil, true, err
	}
	nativePath := filepath.Join(svc.homeDirectory(), adapter.AliasFilename())
	manifest, err := svc.readCatalogGeneration(svc.catalogGeneratedRoot(shell), nativePath, shell)
	if err != nil {
		return nil, true, err
	}
	names := []string{}
	managed := map[string]bool{}
	for _, item := range manifest.Entries {
		names = append(names, item.Entry.Name)
		managed[item.Entry.Name] = true
	}
	contents, err := readRegularFile(nativePath, ShadowSourceLimit)
	if err != nil && !os.IsNotExist(err) {
		return nil, true, err
	}
	for _, item := range importShadowSource(shell, contents) {
		if item.Entry != nil && !managed[item.Name] {
			names = append(names, item.Name)
		}
	}
	return uniqueCompletionNames(names), true, nil
}

func catalogEntryRunnable(alias Alias) bool {
	return alias.CatalogState == "" || alias.CatalogState == "native-only" || alias.CatalogState == "installed" || alias.CatalogState == "installed-removed"
}
