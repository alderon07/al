package app

import "fmt"

type ContextAssociation struct {
	Shell         string
	Name          string
	Type          string
	CommandSHA256 string
	Kind          string
	Path          string
}

type ContextChange struct {
	Kind    string
	Removed int
}

type ContextToggle struct {
	Ranking ContextRanking
	Kind    string
	Path    string
	Marked  bool
}

func (s *Services) ContextAssociations() ([]ContextAssociation, error) {
	path, err := s.contextPath()
	if err != nil {
		return nil, err
	}
	saved, err := s.loadContextFile(path)
	if err != nil {
		return nil, err
	}
	result := make([]ContextAssociation, 0, len(saved.Bindings))
	for _, binding := range saved.Bindings {
		result = append(result, ContextAssociation{Shell: binding.Shell, Name: binding.Name, Type: binding.Type, CommandSHA256: binding.CommandSHA256, Kind: binding.Kind, Path: binding.Path})
	}
	return result, nil
}

func (s *Services) AddContext(name, target string) (ContextChange, error) {
	if err := validateContextEntryName("add", name); err != nil {
		return ContextChange{}, err
	}
	working, err := s.currentWorkingContext()
	if err != nil {
		return ContextChange{}, err
	}
	kind, path, err := contextTarget(working, target)
	if err != nil {
		return ContextChange{}, err
	}
	aliases, err := s.loadAliases()
	if err != nil {
		return ContextChange{}, err
	}
	alias, err := uniqueAliasByName(aliases, name)
	if err != nil {
		return ContextChange{}, err
	}
	if err := s.addContextBinding(alias, s.activeShellAdapter().Name(), kind, path); err != nil {
		return ContextChange{}, err
	}
	return ContextChange{Kind: kind}, nil
}

func (s *Services) RemoveContext(name, target string) (ContextChange, error) {
	if err := validateContextEntryName("remove", name); err != nil {
		return ContextChange{}, err
	}
	shell := s.activeShellAdapter().Name()
	if target == "all" {
		removed, err := s.removeContextBindings(shell, name, "", "")
		return ContextChange{Removed: removed}, err
	}
	working, err := s.currentWorkingContext()
	if err != nil {
		return ContextChange{}, err
	}
	kind, path, err := contextTarget(working, target)
	if err != nil {
		return ContextChange{}, err
	}
	removed, err := s.removeContextBindings(shell, name, kind, path)
	return ContextChange{Kind: kind, Removed: removed}, err
}

func (s *Services) ToggleEntryContext(selected Alias, ranking ContextRanking) (ContextToggle, error) {
	if err := validateContextEntryName("add", selected.Name); err != nil {
		return ContextToggle{}, err
	}
	kind, path, err := contextToggleTarget(ranking, selected)
	if err != nil {
		return ContextToggle{}, err
	}
	if ranking.HasBinding(selected, kind, path) {
		_, err = s.removeContextBindings(ranking.Shell, selected.Name, kind, path)
	} else {
		err = s.addContextBinding(selected, ranking.Shell, kind, path)
	}
	if err != nil {
		return ContextToggle{}, fmt.Errorf("Could not change context: %w", err)
	}
	updated, err := s.loadContextRanking(ranking.Shell, ranking.Working)
	if err != nil {
		return ContextToggle{}, fmt.Errorf("Context changed, but could not reload it: %w", err)
	}
	return ContextToggle{Ranking: updated, Kind: kind, Path: path, Marked: updated.HasBinding(selected, kind, path)}, nil
}

func validateContextEntryName(operation, name string) error {
	if aliasName.MatchString(name) {
		return nil
	}
	return fmt.Errorf("usage: al context %s NAME [--repo|--directory%s]", operation, map[bool]string{true: "|--all", false: ""}[operation == "remove"])
}
