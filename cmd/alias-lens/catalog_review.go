//go:build !windows

package main

import "github.com/alderon07/al/internal/presentation"

import (
	"github.com/alderon07/al/internal/app"

	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
)

type catalogReviewByteReader struct{ reader io.Reader }

func (r catalogReviewByteReader) Read(data []byte) (int, error) {
	if len(data) > 1 {
		data = data[:1]
	}
	return r.reader.Read(data)
}
func newCatalogReviewReader() *bufio.Reader {
	return bufio.NewReader(catalogReviewByteReader{os.Stdin})
}

var catalogReviewTerminal = func() bool { return fileIsTerminal(os.Stdin) && fileIsTerminal(os.Stdout) }

func confirmCatalogReview(reader *bufio.Reader, prompt string) (bool, error) {
	fmt.Fprint(catalogWorkflowStdout, prompt+" [y/N] ")
	answer, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(answer) == "y", nil
}

func guidedCatalogLifecycleReview(value neutralcatalog.Catalog, shell string, adopt bool) (app.CatalogLifecycleDecisions, error) {
	if !catalogReviewTerminal() {
		return app.CatalogLifecycleDecisions{}, fmt.Errorf("native approval and ownership review need an interactive terminal")
	}
	review, err := applicationServices().PrepareCatalogReview(value, shell)
	decisions := review.Decisions
	if err != nil {
		return decisions, err
	}
	reader := newCatalogReviewReader()
	for _, entry := range neutralcatalog.Normalize(value).Entries {
		if candidate, err := applicationServices().CatalogNativeReview(review, entry); err != nil {
			return decisions, err
		} else if candidate != nil {
			key := candidate.Approval.Key
			declaration := candidate.Declaration
			fmt.Fprintf(catalogWorkflowStdout, "\nReview %s (%s, %s)\nID: %s\nImplementation: %s\nExact declaration: %q\n", presentation.EscapePlainText(entry.Name), entry.Kind, shell, entry.ID, key.ImplementationSHA256, declaration)
			yes, err := confirmCatalogReview(reader, "Approve this exact native implementation?")
			if err != nil {
				return decisions, err
			}
			if yes {
				decisions.Approvals = append(decisions.Approvals, candidate.Approval)
			}
		}
		if adopt {
			candidate, err := applicationServices().CatalogAdoptionReview(review, entry)
			if err != nil {
				continue
			}
			record := candidate.Adoption
			fmt.Fprintf(catalogWorkflowStdout, "\nOwnership %s\nSource: %s\nBytes: %d-%d\nRange: %s\nExact fallback: %q\nCandidate: %s\n", presentation.EscapePlainText(entry.Name), candidate.DisplayPath, record.Start, record.End, record.DefinitionSHA256, record.Definition, presentation.QuotedCatalogValue(entry))
			yes, err := confirmCatalogReview(reader, "Enroll this exact native fallback?")
			if err != nil {
				return decisions, err
			}
			if yes {
				decisions.Adoptions = append(decisions.Adoptions, record)
			}
		}
	}
	return decisions, nil
}
