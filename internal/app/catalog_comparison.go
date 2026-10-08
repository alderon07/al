package app

import (
	neutralcatalog "alias-lens/internal/catalog"
	"errors"
	"fmt"
	"os"
)

type CatalogComparison struct {
	Before, After neutralcatalog.Catalog
	Report        neutralcatalog.SemanticDiffReport
}
type CatalogComparisonInputError struct{}

func (*CatalogComparisonInputError) Error() string { return "--from needs repository or installed" }
func (s *Services) CompareCatalog(source, shell string) (CatalogComparison, error) {
	observed, err := s.observeConfig()
	if err != nil {
		return CatalogComparison{}, fmt.Errorf("Alias Lens could not read its settings: %w", err)
	}
	config := observed.Config
	if shell == "" {
		shell = config.Shell
	}
	if source == "" {
		if config.Repository != "" {
			source = "repository"
		} else {
			source = "installed"
		}
	}
	local, err := readCatalogFile(s.localCatalogPath())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return CatalogComparison{}, fmt.Errorf("no catalog is ready to compare; enter al catalog shadow to inspect your current aliases first")
		}
		return CatalogComparison{}, fmt.Errorf("Alias Lens cannot use the local catalog: %w", err)
	}
	var other neutralcatalog.Catalog
	switch source {
	case "repository":
		enrolled, handled, readErr := s.readEnrolledRepositoryCatalog()
		if readErr != nil {
			return CatalogComparison{}, readErr
		}
		if handled {
			other = enrolled
			break
		}
		if config.Repository == "" {
			return CatalogComparison{}, fmt.Errorf("no repository is connected; enter al repo to choose one")
		}
		repositoryPath, pathErr := s.observedCatalogRepositoryPath()
		if pathErr != nil {
			return CatalogComparison{}, pathErr
		}
		repositoryCatalog, pathErr := repositoryFilePath(config.Repository, repositoryPath)
		if pathErr != nil {
			return CatalogComparison{}, pathErr
		}
		other, err = readCatalogFile(repositoryCatalog)
	case "installed":
		other, err = s.readInstalledCatalogSnapshot(shell)
	default:
		return CatalogComparison{}, &CatalogComparisonInputError{}
	}
	if err != nil {
		return CatalogComparison{}, err
	}
	reportShell := ""
	if source == "installed" {
		reportShell = shell
	}
	report := neutralcatalog.SemanticDiff(other, local, source, reportShell)
	return CatalogComparison{Before: other, After: local, Report: report}, nil

}
