//go:build !windows

package tui

import "alias-lens/internal/presentation"

import "alias-lens/internal/app"

import (
	"fmt"
	"os"
	"strings"

	"alias-lens/internal/catalogstore"
	workflowplan "alias-lens/internal/plan"
	tea "alias-lens/internal/tea"
)

type catalogTUIReviewItem struct {
	Text     string
	Approval *catalogstore.ApprovalRecord
	Adoption *catalogstore.Adoption
}
type catalogTUIView struct {
	Shell       string
	Text        string
	Err         string
	Loading     bool
	Offset      int
	Review      []catalogTUIReviewItem
	Index       int
	Decisions   app.CatalogLifecycleDecisions
	Plan        *workflowplan.OperationPlan
	ConflictIDs []string
	Conflict    *app.CatalogConflictResolution
	Choices     map[string]string
	Stage       string
}
type catalogTUILoadedMsg struct{ View *catalogTUIView }
type catalogTUIAppliedMsg struct{ Err error }

func loadCatalogTUI(services *app.Services, shell string) tea.Cmd {
	return func() tea.Msg {
		view := &catalogTUIView{Shell: shell, Stage: "status"}
		snapshot, err := services.LoadCatalogReview(shell)
		view.Decisions = snapshot.Decisions
		if err != nil {
			view.Err = err.Error()
			return catalogTUILoadedMsg{view}
		}
		view.Err = snapshot.Warning
		var text strings.Builder
		fmt.Fprintf(&text, "Catalog for %s\n\n", presentation.FriendlyShellName(shell))
		for _, alias := range snapshot.Entries {
			fmt.Fprintf(&text, "%s  %s\n", presentation.EscapePlainText(alias.Name), alias.CatalogState)
		}
		for _, item := range snapshot.Items {
			if candidate := item.Native; candidate != nil {
				record := candidate.Approval
				entry := candidate.Entry
				key := record.Key
				view.Review = append(view.Review, catalogTUIReviewItem{Text: fmt.Sprintf("Approve %s (%s, %s)\nID %s\nHash %s\nExact declaration %q", entry.Name, entry.Kind, shell, entry.ID, key.ImplementationSHA256, candidate.Declaration), Approval: &record})
			}
			if candidate := item.Adoption; candidate != nil {
				record := candidate.Adoption
				entry := candidate.Entry
				view.Review = append(view.Review, catalogTUIReviewItem{Text: fmt.Sprintf("Enroll exact fallback %s\nSource %s\nBytes %d-%d\nHash %s\nExact fallback %q\nCandidate %s", entry.Name, candidate.DisplayPath, record.Start, record.End, record.DefinitionSHA256, record.Definition, presentation.QuotedCatalogValue(entry)), Adoption: &record})
			}
		}
		for _, conflict := range snapshot.Conflicts {
			view.ConflictIDs = append(view.ConflictIDs, conflict.ID)
			fmt.Fprintf(&text, "\nSaved conflict %s\n", presentation.ShortFingerprint(conflict.ID))
			for _, field := range conflict.Fields {
				fmt.Fprintf(&text, "%s  %s  %s\n", presentation.EscapePlainText(field.Name), presentation.EscapePlainText(field.Path), presentation.EscapePlainText(field.Kind))
			}
			text.WriteString("Choose x to review semantic conflict fields.\n")
		}
		text.WriteString("\nReview and install with e. Decisions remain staged until final confirmation.\n")
		view.Text = text.String()
		return catalogTUILoadedMsg{view}
	}
}

func catalogTUIPlan(services *app.Services, shell string, decisions app.CatalogLifecycleDecisions) tea.Cmd {
	return func() tea.Msg {
		plan, err := services.BuildCatalogEnablePlan(shell, decisions)
		view := &catalogTUIView{Shell: shell, Decisions: decisions, Stage: "plan", Plan: &plan}
		if err != nil {
			view.Err = err.Error()
		} else {
			view.Text = workflowplan.RenderPlain(plan)
		}
		return catalogTUILoadedMsg{view}
	}
}

func (m model) updateCatalogTUI(message tea.KeyMsg) (tea.Model, tea.Cmd) {
	view := m.catalogView
	if message.Type == tea.KeyEsc {
		m.catalogView = nil
		if m.catalogStandalone {
			return m, tea.Quit
		}
		return m, nil
	}
	if message.Type == tea.KeyCtrlC {
		return m, tea.Quit
	}
	if view.Loading {
		return m, nil
	}
	if view.Stage == "conflict-select" {
		if message.Type == tea.KeyUp {
			view.Index = max(0, view.Index-1)
			view.Text = catalogConflictListText(view.ConflictIDs, view.Index)
			return m, nil
		}
		if message.Type == tea.KeyDown {
			view.Index = min(len(view.ConflictIDs)-1, view.Index+1)
			view.Text = catalogConflictListText(view.ConflictIDs, view.Index)
			return m, nil
		}
		if message.Type == tea.KeyEnter {
			view.Loading = true
			id, shell := view.ConflictIDs[view.Index], view.Shell
			return m, func() tea.Msg {
				resolution, err := m.service().LoadCatalogConflictResolution(id)
				next := &catalogTUIView{Shell: shell, Stage: "conflict", Conflict: &resolution, Choices: map[string]string{}}
				if err != nil {
					next.Err = err.Error()
				} else {
					next.Text = catalogConflictReviewText(resolution, 0)
				}
				return catalogTUILoadedMsg{next}
			}
		}
	}
	if message.Type == tea.KeyUp {
		view.Offset = max(0, view.Offset-1)
		return m, nil
	}
	if message.Type == tea.KeyDown {
		view.Offset++
		return m, nil
	}
	key := ""
	if message.Type == tea.KeyRunes {
		key = string(message.Runes)
	}
	if key == "x" && view.Stage == "status" && len(view.ConflictIDs) > 0 {
		view.Stage = "conflict-select"
		view.Index = 0
		view.Offset = 0
		view.Text = catalogConflictListText(view.ConflictIDs, view.Index)
		return m, nil
	}
	if view.Stage == "conflict" && view.Err == "" && (key == "l" || key == "r") {
		item := view.Conflict.Conflicts[view.Index]
		choice := "local"
		if key == "r" {
			choice = "remote"
		}
		view.Choices[item.EntryID+":"+item.Path] = choice
		view.Index++
		view.Offset = 0
		if view.Index < len(view.Conflict.Conflicts) {
			view.Text = catalogConflictReviewText(*view.Conflict, view.Index)
			return m, nil
		}
		view.Loading = true
		id, shell, choices := view.Conflict.ID, view.Shell, view.Choices
		return m, func() tea.Msg {
			preview, err := m.service().BuildCatalogConflictResolutionPlan(id, choices)
			next := &catalogTUIView{Shell: shell, Stage: "conflict-plan", Conflict: view.Conflict, Choices: choices, Plan: &preview}
			if err != nil {
				next.Err = err.Error()
			} else {
				next.Text = workflowplan.RenderPlain(preview)
			}
			return catalogTUILoadedMsg{next}
		}
	}
	if view.Stage == "conflict-plan" && key == "y" && view.Err == "" && view.Plan != nil {
		view.Loading = true
		preview, id, choices := *view.Plan, view.Conflict.ID, view.Choices
		return m, func() tea.Msg {
			return catalogTUIAppliedMsg{m.service().ApplyCatalogConflictResolution(preview, id, choices)}
		}
	}
	if key == "r" && view.Stage == "status" {
		view.Loading = true
		return m, loadCatalogTUI(m.service(),

			view.Shell)
	}
	if key == "e" && view.Stage == "status" {
		view.Err = ""
		view.Offset = 0
		if len(view.Review) > 0 {
			view.Stage = "review"
			view.Text = view.Review[0].Text
			return m, nil
		}
		view.Loading = true
		return m, catalogTUIPlan(m.service(),

			view.Shell, view.Decisions)
	}
	if view.Stage == "review" && (key == "y" || key == "n") {
		item := view.Review[view.Index]
		if key == "y" {
			if item.Approval != nil {
				view.Decisions.Approvals = append(view.Decisions.Approvals, *item.Approval)
			}
			if item.Adoption != nil {
				view.Decisions.Adoptions = append(view.Decisions.Adoptions, *item.Adoption)
			}
		}
		view.Index++
		view.Offset = 0
		if view.Index < len(view.Review) {
			view.Text = view.Review[view.Index].Text
			return m, nil
		}
		view.Loading = true
		return m, catalogTUIPlan(m.service(),

			view.Shell, view.Decisions)
	}
	if view.Stage == "plan" && key == "y" && view.Err == "" && view.Plan != nil {
		view.Loading = true
		preview := *view.Plan
		shell := view.Shell
		decisions := view.Decisions
		return m, func() tea.Msg {
			return catalogTUIAppliedMsg{m.service().ApplyCatalogInstallation(preview, shell, decisions)}
		}
	}
	return m, nil
}

func (m model) catalogTUIView(frame tuiFrame, header string) string {
	view := m.catalogView
	text := view.Text
	if view.Loading {
		text = "Checking catalog state…"
	}
	if view.Err != "" {
		text += "\n\n" + view.Err
	}
	lines := strings.Split(text, "\n")
	height := max(1, m.height-8)
	offset := min(view.Offset, max(0, len(lines)-height))
	lines = lines[offset:min(len(lines), offset+height)]
	for i := range lines {
		lines[i] = truncate(lines[i], frame.contentWidth)
	}
	footer := "e review and install · x conflicts · r refresh · ↑↓ scroll · esc cancel"
	if view.Stage == "review" {
		footer = "y approve this exact item · n skip · esc cancel all staged decisions"
	}
	if view.Stage == "conflict-select" {
		footer = "↑↓ choose conflict · enter review fields · esc cancel"
	}
	if view.Stage == "conflict" {
		footer = "l choose local field · r choose remote field · esc cancel"
	}
	if view.Stage == "plan" || view.Stage == "conflict-plan" {
		footer = "y apply this plan · esc cancel all staged decisions"
	}
	return frame.renderWithFooter(header+"\n\n"+strings.Join(lines, "\n"), truncate(footer, frame.contentWidth))
}

func catalogConflictReviewText(value app.CatalogConflictResolution, index int) string {
	conflict := value.Conflicts[index]
	local, remote := "(deleted)", "(deleted)"
	for _, entry := range value.Local.Entries {
		if entry.ID == conflict.EntryID {
			local = presentation.QuotedCatalogValue(entry)
		}
	}
	for _, entry := range value.Remote.Entries {
		if entry.ID == conflict.EntryID {
			remote = presentation.QuotedCatalogValue(entry)
		}
	}
	return fmt.Sprintf("Resolve %s field %s (%d/%d)\n\nLocal entry\n%s\n\nRemote entry\n%s\n\nChoose only this semantic field. Final confirmation saves the catalog and base.", presentation.EscapePlainText(conflict.Name), presentation.EscapePlainText(conflict.Path), index+1, len(value.Conflicts), local, remote)
}

func catalogConflictListText(ids []string, index int) string {
	var text strings.Builder
	text.WriteString("Saved catalog conflicts\n\n")
	for i, id := range ids {
		prefix := "  "
		if i == index {
			prefix = "> "
		}
		fmt.Fprintf(&text, "%s%s\n", prefix, presentation.ShortFingerprint(id))
	}
	return text.String()
}

func runCatalogConflictReview(services *app.Services, id string) error {
	if !(fileIsTerminal(os.Stdin) && fileIsTerminal(os.Stdout)) {
		return fmt.Errorf("conflict field review needs a terminal; open al and choose catalog conflicts")
	}
	resolution, err := services.LoadCatalogConflictResolution(id)
	if err != nil {
		return err
	}
	config, err := services.Settings.Load()
	if err != nil {
		return err
	}
	theme, err := services.LoadTheme()
	if err != nil {
		return err
	}
	applyTheme(theme)
	view := &catalogTUIView{Shell: services.ActiveShellAdapter().Name(), Stage: "conflict", Conflict: &resolution, Choices: map[string]string{}, Text: catalogConflictReviewText(resolution, 0)}
	_, err = tea.NewProgram(model{services: services, catalogView: view, catalogStandalone: true, width: 80, height: 24, theme: theme, shortcutProfile: resolvedShortcutProfile(config)}, tea.WithAltScreen(), tea.WithOutput(os.Stderr)).Run()
	return err
}
