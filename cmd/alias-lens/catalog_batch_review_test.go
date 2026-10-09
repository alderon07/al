//go:build !windows

package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/alderon07/al/internal/app"
	"github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
)

func syntheticBatchItems() []app.CatalogReviewItem {
	entry := catalog.Entry{ID: strings.Repeat("1", 32), Name: "fixture", Kind: "command"}
	approval := catalogstore.ApprovalRecord{Key: catalogstore.ApprovalKey{EntryID: entry.ID, Shell: "bash", Name: entry.Name, Kind: entry.Kind, ImplementationSHA256: strings.Repeat("2", 64)}}
	adoption := catalogstore.Adoption{EntryID: entry.ID, Shell: "bash", Name: entry.Name, Kind: entry.Kind, Start: 0, End: 31, Definition: "alias fixture='echo synthetic'\n", DefinitionSHA256: strings.Repeat("3", 64), FileSHA256: strings.Repeat("4", 64)}
	return []app.CatalogReviewItem{{Native: &app.CatalogNativeReview{Entry: entry, Declaration: adoption.Definition, Approval: approval}}, {Adoption: &app.CatalogAdoptionReview{Entry: entry, DisplayPath: "~/.bash_aliases", Adoption: adoption}}}
}

func TestCatalogBatchStagesCapturedItemsOrCancels(t *testing.T) {
	base := app.CatalogLifecycleDecisions{SourceSHA256: strings.Repeat("5", 64), ConfirmedAt: "2026-10-09T00:00:00Z"}
	items := syntheticBatchItems()
	previous := catalogWorkflowStdout
	defer func() { catalogWorkflowStdout = previous }()
	for _, test := range []struct {
		answer               string
		approvals, adoptions int
		cancel               bool
	}{
		{"y\n", 1, 1, false}, {"n\n", 0, 0, true}, {"", 0, 0, true}, {"y", 0, 0, true}, {"i\ny\nn\n", 1, 0, false}, {"i\ny\n", 0, 0, true},
	} {
		var output bytes.Buffer
		catalogWorkflowStdout = &output
		decisions, err := reviewCatalogItems(bufio.NewReader(strings.NewReader(test.answer)), base, items, false)
		if errors.Is(err, errCatalogReviewCanceled) != test.cancel {
			t.Fatalf("answer=%q error=%v", test.answer, err)
		}
		if len(decisions.Approvals) != test.approvals || len(decisions.Adoptions) != test.adoptions || decisions.SourceSHA256 != base.SourceSHA256 {
			t.Fatal("batch did not preserve captured scope or source proof")
		}
		for _, required := range []string{"1 native approvals and 1 fallback enrollments", "Implementation:", "Exact declaration:", "Bytes: 0-31", "File:", "Range:", "Exact fallback:", "Candidate:"} {
			if !strings.Contains(output.String(), required) {
				t.Fatalf("missing exact review field %q", required)
			}
		}
		if strings.Count(output.String(), "Approve this exact batch?") != 1 {
			t.Fatal("batch confirmation repeated")
		}
	}
}

type batchFailWriter struct {
	calls  int
	failAt int
	short  bool
}

func (writer *batchFailWriter) Write(contents []byte) (int, error) {
	writer.calls++
	if writer.calls == writer.failAt {
		if writer.short {
			return len(contents) - 1, nil
		}
		return 0, io.ErrClosedPipe
	}
	return len(contents), nil
}

func TestCatalogBatchOutputFailuresStageNothing(t *testing.T) {
	previous := catalogWorkflowStdout
	defer func() { catalogWorkflowStdout = previous }()
	for _, failAt := range []int{1, 2, 3, 4} {
		for _, short := range []bool{false, true} {
			writer := &batchFailWriter{failAt: failAt, short: short}
			catalogWorkflowStdout = writer
			answer := "y\n"
			if failAt > 2 {
				answer = "i\ny\ny\n"
			}
			decisions, err := reviewCatalogItems(bufio.NewReader(strings.NewReader(answer)), app.CatalogLifecycleDecisions{}, syntheticBatchItems(), false)
			expected := io.ErrClosedPipe
			if short {
				expected = io.ErrShortWrite
			}
			if !errors.Is(err, expected) || len(decisions.Approvals) != 0 || len(decisions.Adoptions) != 0 {
				t.Fatalf("failed output staged items: %v", err)
			}
		}
	}
	catalogWorkflowStdout = &batchFailWriter{failAt: 1, short: true}
	input := bufio.NewReader(strings.NewReader("y\n"))
	yes, err := confirmCatalogReview(input, "Apply this catalog installation?")
	remaining, readErr := io.ReadAll(input)
	if yes || !errors.Is(err, io.ErrShortWrite) || readErr != nil || string(remaining) != "y\n" {
		t.Fatal("failed final prompt allowed confirmation")
	}
}
