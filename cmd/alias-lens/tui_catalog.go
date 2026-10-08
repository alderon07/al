//go:build !windows

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
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
	Decisions   catalogLifecycleDecisions
	Plan        *workflowplan.OperationPlan
	ConflictIDs []string
	Conflict    *catalogConflictResolution
	Choices     map[string]string
	Stage       string
}
type catalogTUILoadedMsg struct{ View *catalogTUIView }
type catalogTUIAppliedMsg struct{ Err error }

func loadCatalogTUI(shell string) tea.Cmd {
	return func() tea.Msg {
		view := &catalogTUIView{Shell: shell, Stage: "status", Decisions: catalogLifecycleDecisions{ConfirmedAt: lifecycleTimestamp()}}
		value, err := readCatalogFile(localCatalogPath())
		if err != nil {
			view.Err = err.Error()
			return catalogTUILoadedMsg{view}
		}
		canonical, _ := neutralcatalog.Encode(value)
		view.Decisions.SourcePath = localCatalogPath()
		view.Decisions.SourceSHA256 = hashBytes(canonical)
		approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
		_, err = readLifecycleFile(catalogApprovalsPath(), &approvals)
		if err != nil {
			view.Err = err.Error()
			return catalogTUILoadedMsg{view}
		}
		approved := map[catalogstore.ApprovalKey]bool{}
		for _, record := range approvals.Records {
			approved[record.Key] = true
		}
		adapter, _ := shellAdapter(shell)
		nativePath := filepath.Join(homeDirectory(), adapter.AliasFilename())
		native, _ := readRegularFile(nativePath, shadowSourceLimit)
		adoptions := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
		_, err = readLifecycleFile(catalogAdoptionsPath(), &adoptions)
		if err != nil {
			view.Err = err.Error()
			return catalogTUILoadedMsg{view}
		}
		owned := map[string]bool{}
		for _, record := range adoptions.Records {
			if record.Shell == shell && record.FileSHA256 == hashBytes(native) {
				owned[record.EntryID] = true
			}
		}
		var text strings.Builder
		fmt.Fprintf(&text, "Catalog for %s\n\n", friendlyShellName(shell))
		aliases, err := loadAliases()
		if err != nil {
			view.Err = err.Error()
		} else {
			for _, alias := range aliases {
				fmt.Fprintf(&text, "%s  %s\n", escapePlainText(alias.Name), alias.CatalogState)
			}
		}
		for _, entry := range neutralcatalog.Normalize(value).Entries {
			if implementation, exists := entry.Native[shell]; exists {
				key := catalogstore.NativeApproval(entry, shell, implementation)
				if !approved[key] {
					declaration, e := catalogstore.Declaration(entry, shell, "")
					if e != nil || validateCatalogDeclaration(shell, entry, []byte(declaration)) != nil {
						view.Err = "Unsafe native declaration; edit the catalog before review"
						continue
					}
					record := catalogstore.ApprovalRecord{Key: key, ApprovedAt: view.Decisions.ConfirmedAt}
					view.Review = append(view.Review, catalogTUIReviewItem{Text: fmt.Sprintf("Approve %s (%s, %s)\nID %s\nHash %s\nExact declaration %q", entry.Name, entry.Kind, shell, entry.ID, key.ImplementationSHA256, declaration), Approval: &record})
				}
			}
			if !owned[entry.ID] {
				if record, e := catalogAdoptionForEntry(entry, shell, nativePath, native); e == nil {
					copy := record
					view.Review = append(view.Review, catalogTUIReviewItem{Text: fmt.Sprintf("Enroll exact fallback %s\nSource %s\nBytes %d-%d\nHash %s\nExact fallback %q\nCandidate %s", entry.Name, displayPrivatePath(nativePath), record.Start, record.End, record.DefinitionSHA256, record.Definition, quotedCatalogValue(entry)), Adoption: &copy})
				}
			}
		}
		conflictRoot := filepath.Join(filepath.Dir(localCatalogPath()), "catalog-conflicts")
		if directories, e := os.ReadDir(conflictRoot); e == nil {
			for _, directory := range directories {
				if !directory.IsDir() || !catalogstore.ValidHash(directory.Name()) {
					continue
				}
				path := filepath.Join(conflictRoot, directory.Name(), "conflicts.json")
				data, e := catalogstore.ReadPrivateBytes(path, catalogstore.MaxDocumentBytes)
				if e != nil {
					view.Err = "Saved conflict metadata needs attention; enter al doctor"
					continue
				}
				var metadata struct {
					Version      int                            `json:"version"`
					BaseSHA256   string                         `json:"base_sha256"`
					LocalSHA256  string                         `json:"local_sha256"`
					RemoteSHA256 string                         `json:"remote_sha256"`
					Conflicts    []neutralcatalog.MergeConflict `json:"conflicts"`
				}
				if json.Unmarshal(data, &metadata) != nil || metadata.Version != 1 {
					view.Err = "Saved conflict metadata is invalid"
					continue
				}
				view.ConflictIDs = append(view.ConflictIDs, directory.Name())
				fmt.Fprintf(&text, "\nSaved conflict %s\n", shortFingerprint(directory.Name()))
				for _, conflict := range metadata.Conflicts {
					fmt.Fprintf(&text, "%s  %s  %s\n", escapePlainText(conflict.Name), escapePlainText(conflict.Path), escapePlainText(conflict.Kind))
				}
				text.WriteString("Choose x to review semantic conflict fields.\n")
			}
		}
		text.WriteString("\nReview and install with e. Decisions remain staged until final confirmation.\n")
		view.Text = text.String()
		return catalogTUILoadedMsg{view}
	}
}

func catalogTUIPlan(shell string, decisions catalogLifecycleDecisions) tea.Cmd {
	return func() tea.Msg {
		plan, err := buildCatalogEnablePlan(shell, decisions)
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
				resolution, err := loadCatalogConflictResolution(id)
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
			preview, err := buildCatalogConflictResolutionPlan(id, choices)
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
			return catalogTUIAppliedMsg{applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return buildCatalogConflictResolutionPlan(id, choices) })}
		}
	}
	if key == "r" && view.Stage == "status" {
		view.Loading = true
		return m, loadCatalogTUI(view.Shell)
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
		return m, catalogTUIPlan(view.Shell, view.Decisions)
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
		return m, catalogTUIPlan(view.Shell, view.Decisions)
	}
	if view.Stage == "plan" && key == "y" && view.Err == "" && view.Plan != nil {
		view.Loading = true
		preview := *view.Plan
		shell := view.Shell
		decisions := view.Decisions
		return m, func() tea.Msg {
			return catalogTUIAppliedMsg{applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return buildCatalogEnablePlan(shell, decisions) })}
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

func catalogConflictReviewText(value catalogConflictResolution, index int) string {
	conflict := value.Conflicts[index]
	local, remote := "(deleted)", "(deleted)"
	for _, entry := range value.Local.Entries {
		if entry.ID == conflict.EntryID {
			local = quotedCatalogValue(entry)
		}
	}
	for _, entry := range value.Remote.Entries {
		if entry.ID == conflict.EntryID {
			remote = quotedCatalogValue(entry)
		}
	}
	return fmt.Sprintf("Resolve %s field %s (%d/%d)\n\nLocal entry\n%s\n\nRemote entry\n%s\n\nChoose only this semantic field. Final confirmation saves the catalog and base.", escapePlainText(conflict.Name), escapePlainText(conflict.Path), index+1, len(value.Conflicts), local, remote)
}

func catalogConflictListText(ids []string, index int) string {
	var text strings.Builder
	text.WriteString("Saved catalog conflicts\n\n")
	for i, id := range ids {
		prefix := "  "
		if i == index {
			prefix = "> "
		}
		fmt.Fprintf(&text, "%s%s\n", prefix, shortFingerprint(id))
	}
	return text.String()
}

func runCatalogConflictReview(id string) error {
	if !catalogReviewTerminal() {
		return fmt.Errorf("conflict field review needs a terminal; open al and choose catalog conflicts")
	}
	resolution, err := loadCatalogConflictResolution(id)
	if err != nil {
		return err
	}
	config, err := loadConfig()
	if err != nil {
		return err
	}
	theme, err := loadTheme()
	if err != nil {
		return err
	}
	applyTheme(theme)
	view := &catalogTUIView{Shell: activeShellAdapter().Name(), Stage: "conflict", Conflict: &resolution, Choices: map[string]string{}, Text: catalogConflictReviewText(resolution, 0)}
	_, err = tea.NewProgram(model{catalogView: view, catalogStandalone: true, width: 80, height: 24, theme: theme, shortcutProfile: resolvedShortcutProfile(config)}, tea.WithAltScreen(), tea.WithOutput(os.Stderr)).Run()
	return err
}
