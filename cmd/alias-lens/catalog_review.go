//go:build !windows

package main

import (
	"github.com/alderon07/al/internal/app"

	"bufio"
	"errors"
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
	return bufio.NewReader(catalogReviewByteReader{catalogReviewInput})
}

var catalogReviewTerminal = func() bool { return fileIsTerminal(os.Stdin) && fileIsTerminal(os.Stdout) }

var errCatalogReviewCanceled = errors.New("catalog review canceled")
var catalogReviewInput io.Reader = os.Stdin

func writeCatalogReviewOutput(output string) error {
	if len(output) > app.ShadowReportLimit {
		return fmt.Errorf("catalog review output exceeds 16 MiB")
	}
	written, err := io.WriteString(catalogWorkflowStdout, output)
	if err != nil {
		return fmt.Errorf("write catalog review: %w", err)
	}
	if written != len(output) {
		return fmt.Errorf("write catalog review: %w", io.ErrShortWrite)
	}
	return nil
}

func catalogReviewAnswer(reader *bufio.Reader, prompt string) (string, error) {
	if err := writeCatalogReviewOutput(prompt); err != nil {
		return "", err
	}
	answer, err := reader.ReadString('\n')
	if errors.Is(err, io.EOF) || strings.ContainsAny(answer, "\x03\x04") {
		return "", errCatalogReviewCanceled
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(answer), nil
}

func confirmCatalogReview(reader *bufio.Reader, prompt string) (bool, error) {
	answer, err := catalogReviewAnswer(reader, prompt+" [y/N] ")
	if errors.Is(err, errCatalogReviewCanceled) {
		return false, nil
	}
	return answer == "y", err
}

func guidedCatalogLifecycleReview(value neutralcatalog.Catalog, shell string, adopt bool) (app.CatalogLifecycleDecisions, error) {
	return guidedCatalogLifecycleReviewMode(value, shell, adopt, false)
}

func guidedCatalogLifecycleReviewMode(value neutralcatalog.Catalog, shell string, adopt, individual bool) (app.CatalogLifecycleDecisions, error) {
	if !catalogReviewTerminal() {
		return app.CatalogLifecycleDecisions{}, fmt.Errorf("native approval and ownership review need an interactive terminal")
	}
	review, err := applicationServices().PrepareCatalogReview(value, shell)
	if err != nil {
		return app.CatalogLifecycleDecisions{}, err
	}
	items, err := applicationServices().CatalogPendingReview(review, value, adopt)
	if err != nil {
		return app.CatalogLifecycleDecisions{}, err
	}
	return reviewCatalogItems(newCatalogReviewReader(), review.Decisions, items, individual)
}

func reviewCatalogItems(reader *bufio.Reader, base app.CatalogLifecycleDecisions, items []app.CatalogReviewItem, individual bool) (app.CatalogLifecycleDecisions, error) {
	if len(items) == 0 {
		return base, nil
	}
	if !individual {
		if err := writeCatalogReviewOutput(app.CatalogReviewBatchText(items)); err != nil {
			return base, err
		}
		answer, err := catalogReviewAnswer(reader, "Approve this exact batch? y approve/enroll all displayed; i review individually; n cancel [y/i/N] ")
		if err != nil {
			return base, err
		}
		switch answer {
		case "y":
			return app.StageCatalogReviewItems(base, items), nil
		case "i":
			individual = true
		default:
			return base, errCatalogReviewCanceled
		}
	}
	decisions := base
	for _, item := range items {
		if err := writeCatalogReviewOutput("\n" + app.CatalogReviewItemText(item)); err != nil {
			return base, err
		}
		prompt := "Approve this exact native implementation?"
		if item.Adoption != nil {
			prompt = "Enroll this exact native fallback?"
		}
		answer, err := catalogReviewAnswer(reader, prompt+" [y/N] ")
		if err != nil {
			return base, err
		}
		if answer == "y" {
			decisions = app.StageCatalogReviewItems(decisions, []app.CatalogReviewItem{item})
		}
	}
	return decisions, nil
}
