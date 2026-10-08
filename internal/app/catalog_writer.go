//go:build !windows

package app

import (
	"fmt"
	"path/filepath"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
	workflowplan "alias-lens/internal/plan"
)

func (svc *Services) catalogManagedEditing() (bool, error) {
	return svc.catalogShellInstalled(svc.activeShellAdapter().Name())
}

func (svc *Services) writeCatalogEdit(edit func(*neutralcatalog.Catalog) error) error {
	return svc.writeCatalogEditInSession(nil, edit)
}

func (svc *Services) writeCatalogEditInSession(session *mutationSession, edit func(*neutralcatalog.Catalog) error) error {
	stamp := svc.lifecycleTimestamp()
	build := func() (workflowplan.OperationPlan, error) {
		path := svc.localCatalogPath()
		before, err := readRegularFile(path, neutralcatalog.MaxDocumentBytes)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		value, problems := neutralcatalog.Decode(before)
		if len(problems) > 0 {
			return workflowplan.OperationPlan{}, fmt.Errorf("catalog is invalid; enter al doctor")
		}
		if err := edit(&value); err != nil {
			return workflowplan.OperationPlan{}, err
		}
		after, problems := neutralcatalog.Encode(value)
		if len(problems) > 0 {
			return workflowplan.OperationPlan{}, fmt.Errorf("catalog edit is invalid; check the entry name and implementation")
		}
		revision := filepath.Join(svc.catalogStateRoot(), "catalog-revisions", strings.NewReplacer(":", "", "-", "", ".", "").Replace(stamp)+"catalog.json")
		save, err := svc.lifecycleAction(revision, "catalog_revision", before)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		change, err := svc.lifecycleAction(path, "catalog", after)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		save.Sequence = 1
		change.Sequence = 2
		return workflowplan.Build("catalog.edit", []workflowplan.Input{svc.planInput("catalog", path, before)}, []workflowplan.Action{save, change}, nil, nil), nil
	}
	preview, err := build()
	if err != nil {
		return err
	}
	if session != nil {
		return session.ApplyPlan(preview, build)
	}
	return svc.applyMutationPlan(preview, build)
}

func (svc *Services) applyCatalogMetadata(entry *neutralcatalog.Entry, metadata EntryMetadata) {
	entry.Tags = append([]string(nil), metadata.Tags...)
	entry.Platforms = append([]string(nil), metadata.Platforms...)
	entry.Favorite = metadata.Favorite
	entry.Category = metadata.Category
}

func (svc *Services) addCatalogAlias(name, command, description string, metadata EntryMetadata) error {
	id, err := newOperationID()
	if err != nil {
		return err
	}
	shell := svc.activeShellAdapter().Name()
	return svc.writeCatalogEdit(func(value *neutralcatalog.Catalog) error {
		for _, entry := range value.Entries {
			if entry.Name == name {
				return fmt.Errorf("catalog name already exists")
			}
		}
		if catalogProtectedName(name) {
			return fmt.Errorf("catalog name is reserved for shell control")
		}
		entry := neutralcatalog.Entry{ID: id, Name: name, Kind: "command", Description: description, Native: map[string]neutralcatalog.NativeImplementation{shell: {AliasValue: &command}}}
		svc.applyCatalogMetadata(&entry, metadata)
		value.Entries = append(value.Entries, entry)
		return nil
	})
}

func (svc *Services) editCatalogAlias(originalName, name, command, description string, metadata EntryMetadata) error {
	shell := svc.activeShellAdapter().Name()
	return svc.writeCatalogEdit(func(value *neutralcatalog.Catalog) error {
		for i := range value.Entries {
			entry := &value.Entries[i]
			if entry.Name != originalName {
				continue
			}
			if catalogProtectedName(name) {
				return fmt.Errorf("catalog name is reserved for shell control")
			}
			entry.Name = name
			entry.Description = description
			svc.applyCatalogMetadata(entry, metadata)
			if entry.Native == nil {
				entry.Native = map[string]neutralcatalog.NativeImplementation{}
			}
			implementation := neutralcatalog.NativeImplementation{}
			if entry.Kind == "function" {
				implementation.FunctionBody = &command
			} else {
				implementation.AliasValue = &command
			}
			entry.Native[shell] = implementation
			return nil
		}
		return fmt.Errorf("native-only entry needs enrollment; use al catalog import --from %s, then review it with al catalog enable", svc.activeShellAdapter().Name())
	})
}

func (svc *Services) deleteCatalogAlias(name string) error {
	return svc.writeCatalogEdit(func(value *neutralcatalog.Catalog) error {
		for i, entry := range value.Entries {
			if entry.Name == name {
				value.Entries = append(value.Entries[:i], value.Entries[i+1:]...)
				return nil
			}
		}
		return fmt.Errorf("native-only entry needs enrollment; use al catalog import --from %s, then review it with al catalog enable", svc.activeShellAdapter().Name())
	})
}

func (svc *Services) setCatalogMetadata(name string, metadata EntryMetadata) error {
	return svc.writeCatalogEdit(func(value *neutralcatalog.Catalog) error {
		for i := range value.Entries {
			if value.Entries[i].Name == name {
				svc.applyCatalogMetadata(&value.Entries[i], metadata)
				return nil
			}
		}
		return fmt.Errorf("native-only entry needs enrollment; use al catalog import --from %s, then review it with al catalog enable", svc.activeShellAdapter().Name())
	})
}

func (svc *Services) editableEntriesPath() (string, error) {
	active, err := svc.catalogManagedEditing()
	if err != nil {
		return "", err
	}
	if active {
		return svc.localCatalogPath(), nil
	}
	return svc.aliasesPath()
}
func (svc *Services) catalogRevisionDirectory(path string) (string, bool) {
	if filepath.Clean(path) == filepath.Clean(svc.localCatalogPath()) {
		return filepath.Join(svc.catalogStateRoot(), "catalog-revisions"), true
	}
	return "", false
}
func (svc *Services) restoreCatalogBytes(contents []byte) error {
	return svc.restoreCatalogBytesInSession(nil, contents)
}
func (svc *Services) restoreCatalogBytesInSession(session *mutationSession, contents []byte) error {
	value, problems := neutralcatalog.Decode(contents)
	if len(problems) > 0 {
		return fmt.Errorf("catalog revision is invalid; select another revision")
	}
	return svc.writeCatalogEditInSession(session, func(current *neutralcatalog.Catalog) error { *current = value; return nil })
}

func (svc *Services) addCatalogDescriptions() error {
	return svc.writeCatalogEdit(func(value *neutralcatalog.Catalog) error {
		for i := range value.Entries {
			entry := &value.Entries[i]
			implementation, ok := entry.Native[svc.activeShellAdapter().Name()]
			if !ok || implementation.AliasValue == nil {
				continue
			}
			command := *implementation.AliasValue
			if entry.Description == "" || entry.Description == legacyDescription(entry.Name, command) {
				entry.Description = describe(entry.Name, command)
			}
		}
		return nil
	})
}

func (svc *Services) importCatalogAliases(aliases []Alias) error {
	shell := svc.activeShellAdapter().Name()
	ids := make([]string, len(aliases))
	for i := range ids {
		id, err := newOperationID()
		if err != nil {
			return err
		}
		ids[i] = id
	}
	return svc.writeCatalogEdit(func(value *neutralcatalog.Catalog) error {
		names := map[string]bool{}
		for _, entry := range value.Entries {
			names[entry.Name] = true
		}
		for i, alias := range aliases {
			if names[alias.Name] {
				return fmt.Errorf("catalog changed during import; preview al import again")
			}
			if catalogProtectedName(alias.Name) {
				return fmt.Errorf("catalog import contains a reserved shell name")
			}
			command := alias.Command
			entry := neutralcatalog.Entry{ID: ids[i], Name: alias.Name, Kind: "command", Description: alias.Description, Native: map[string]neutralcatalog.NativeImplementation{shell: {AliasValue: &command}}}
			svc.applyCatalogMetadata(&entry, EntryMetadata{Tags: alias.Tags, Platforms: alias.Platforms, Favorite: alias.Favorite, Category: alias.Category})
			value.Entries = append(value.Entries, entry)
		}
		return nil
	})
}

func (svc *Services) catalogImportCurrent() ([]byte, error) {
	value, err := readCatalogFile(svc.localCatalogPath())
	if err != nil {
		return nil, err
	}
	var contents strings.Builder
	for _, entry := range value.Entries {
		command := ""
		if implementation, ok := entry.Native[svc.activeShellAdapter().Name()]; ok && implementation.AliasValue != nil {
			command = *implementation.AliasValue
		}
		if command == "" {
			command = "__catalog_reserved_name__"
		}
		fmt.Fprintf(&contents, "alias %s=%s\n", entry.Name, shellQuote(command))
	}
	return []byte(contents.String()), nil
}
