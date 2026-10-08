//go:build windows

package app

func (s *Services) CatalogEntryRunnable(alias Alias) bool { return catalogEntryRunnable(alias) }

func (s *Services) CatalogManagedEditing() (bool, error) { return s.catalogManagedEditing() }
