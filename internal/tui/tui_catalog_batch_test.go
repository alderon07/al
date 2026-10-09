//go:build !windows

package tui

import (
	"strings"
	"testing"

	"github.com/alderon07/al/internal/catalogstore"
	workflowplan "github.com/alderon07/al/internal/plan"
	tea "github.com/alderon07/al/internal/tea"
	"github.com/charmbracelet/lipgloss"
)

func TestCatalogCompleteWrapPreservesAllContent(t *testing.T) {
	text := "Exact declaration: " + strings.Repeat("x", 500) + " \\t  \"synthetic\"\nHash: " + strings.Repeat("2", 64)
	for _, width := range []int{40, 108} {
		lines := catalogCompleteLines(text, width)
		if strings.Join(lines, "") != strings.ReplaceAll(text, "\n", "") {
			t.Fatal("wrapping dropped exact review content")
		}
		for _, line := range lines {
			if lipgloss.Width(line) > width {
				t.Fatal("complete review exceeded viewport width")
			}
		}
	}
}

func TestCatalogBatchAndPlanRequireCompleteRenderedCoverage(t *testing.T) {
	key := catalogstore.ApprovalKey{EntryID: strings.Repeat("1", 32), Name: "fixture", Shell: "bash", Kind: "command", ImplementationSHA256: strings.Repeat("2", 64)}
	record := catalogstore.ApprovalRecord{Key: key}
	text := strings.Repeat("Exact declaration: "+strings.Repeat("x", 160)+"\n", 20) + "END_OF_EXACT_REVIEW"
	view := &catalogTUIView{Stage: "batch", Shell: "bash", Text: text, BatchText: text, Review: []catalogTUIReviewItem{{Text: text, Approval: &record}}}
	m := model{catalogView: view, width: 52, height: 24}
	render := func() { m.catalogTUIView(newMainTUIFrame(m.width, m.height), "Catalog") }
	render()
	result, cmd := m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(model)
	if cmd != nil || len(view.Decisions.Approvals) != 0 {
		t.Fatal("unread batch granted approval")
	}
	view.Offset = 100000
	render()
	if view.Read {
		t.Fatal("jumping to clamped end bypassed complete coverage")
	}
	view.Offset = 0
	view.SeenThrough = 0
	for attempts := 0; !view.Read && attempts < 1000; attempts++ {
		render()
		if view.Read {
			break
		}
		result, cmd = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyPgDown})
		m = result.(model)
		if cmd != nil {
			t.Fatal("scrolling staged decisions")
		}
	}
	if !view.Read {
		t.Fatal("complete scrolling did not enable batch")
	}
	result, _ = m.Update(tea.WindowSizeMsg{Width: 140, Height: 24})
	m = result.(model)
	if view.Read || view.Offset != 0 {
		t.Fatal("resize retained stale coverage")
	}
	result, cmd = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(model)
	if cmd != nil || len(view.Decisions.Approvals) != 0 {
		t.Fatal("resize bypassed review gate")
	}
	result, cmd = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'i'}})
	m = result.(model)
	if cmd != nil || view.Stage != "review" || len(view.Decisions.Approvals) != 0 {
		t.Fatal("individual selection staged batch")
	}
	for attempts := 0; !view.Read && attempts < 1000; attempts++ {
		render()
		if view.Read {
			break
		}
		result, _ = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyPgDown})
		m = result.(model)
	}
	if !view.Read {
		t.Fatal("individual review remained unavailable")
	}
	result, cmd = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(model)
	if cmd == nil || len(view.Decisions.Approvals) != 1 {
		t.Fatal("complete individual review did not stage exact item")
	}
	view.Loading = false
	result, cmd = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyEsc})
	if result.(model).catalogView != nil || cmd != nil {
		t.Fatal("escape did not discard staged batch")
	}
}

func TestCatalogBatchEntryResetsStatusScroll(t *testing.T) {
	view := &catalogTUIView{Stage: "status", Offset: 100000, Read: true, SeenThrough: 100000, BatchText: strings.Repeat("exact\n", 50), Review: []catalogTUIReviewItem{{Text: "exact"}}}
	m := model{catalogView: view, width: 140, height: 24}
	result, cmd := m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = result.(model)
	if cmd != nil || view.Stage != "batch" || view.Offset != 0 || view.Read || view.SeenThrough != 0 {
		t.Fatal("entering batch reused status coverage")
	}
	m.catalogTUIView(newMainTUIFrame(52, 24), "Catalog")
	if view.Read {
		t.Fatal("narrow batch was approved from status offset")
	}
}

func TestCatalogFinalPlanRequiresCompleteDisplay(t *testing.T) {
	view := &catalogTUIView{Stage: "plan", Shell: "bash", Text: strings.Repeat("Full exact plan content\n", 50), Plan: &workflowplan.OperationPlan{}}
	m := model{catalogView: view, width: 52, height: 24}
	result, cmd := m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	m = result.(model)
	if cmd != nil {
		t.Fatal("unread final plan applied")
	}
	for attempts := 0; !view.Read && attempts < 100; attempts++ {
		m.catalogTUIView(newMainTUIFrame(m.width, m.height), "Catalog")
		if view.Read {
			break
		}
		result, _ = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyPgDown})
		m = result.(model)
	}
	if !view.Read {
		t.Fatal("full plan scrolling did not enable apply")
	}
	result, cmd = m.updateCatalogTUI(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	if cmd == nil || !result.(model).catalogView.Loading {
		t.Fatal("read final plan did not allow final confirmation")
	}
}
