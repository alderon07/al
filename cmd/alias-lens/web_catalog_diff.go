package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"

	neutralcatalog "alias-lens/internal/catalog"
)

const catalogDiffDetailLimit = 8 << 20

type catalogDiffWebView struct {
	summary []byte
	detail  []byte
}

type catalogDiffDetailReport struct {
	SchemaVersion int                     `json:"schema_version"`
	Files         []catalogDiffDetailFile `json:"files"`
}

type catalogDiffDetailFile struct {
	EntryID     string  `json:"entry_id,omitempty"`
	DisplayName string  `json:"display_name"`
	Filename    string  `json:"filename"`
	Kind        string  `json:"kind"`
	BeforeText  *string `json:"before_text"`
	AfterText   *string `json:"after_text"`
}

func newCatalogDiffWebView(before, after neutralcatalog.Catalog, report neutralcatalog.SemanticDiffReport) (*catalogDiffWebView, error) {
	summary, err := encodeCatalogJSON(report)
	if err != nil {
		return nil, err
	}
	if len(summary) > catalogReportLimit {
		return nil, fmt.Errorf("the catalog summary is too large to show safely")
	}
	detail, err := buildCatalogDiffDetail(before, after, report)
	if err != nil {
		return nil, err
	}
	return &catalogDiffWebView{summary: summary, detail: detail}, nil
}

func buildCatalogDiffDetail(before, after neutralcatalog.Catalog, report neutralcatalog.SemanticDiffReport) ([]byte, error) {
	before = neutralcatalog.Normalize(before)
	after = neutralcatalog.Normalize(after)
	beforeEntries := catalogEntriesByID(before.Entries)
	afterEntries := catalogEntriesByID(after.Entries)

	type selectedChange struct {
		id   string
		kind string
		name string
	}
	selected := map[string]selectedChange{}
	var order []string
	for _, change := range report.Changes {
		key := change.EntryID
		if change.Scope == "catalog" {
			key = "catalog"
		}
		if _, exists := selected[key]; exists {
			continue
		}
		selected[key] = selectedChange{id: change.EntryID, kind: change.Kind, name: change.Name}
		order = append(order, key)
	}
	sort.Strings(order)

	detail := catalogDiffDetailReport{SchemaVersion: 1, Files: make([]catalogDiffDetailFile, 0, len(order))}
	for _, key := range order {
		change := selected[key]
		if key == "catalog" {
			oldText := fmt.Sprintf("{\n  \"schema_version\": %d\n}\n", before.SchemaVersion)
			newText := fmt.Sprintf("{\n  \"schema_version\": %d\n}\n", after.SchemaVersion)
			detail.Files = append(detail.Files, catalogDiffDetailFile{DisplayName: "Catalog data format", Filename: "catalog.json", Kind: change.kind, BeforeText: &oldText, AfterText: &newText})
			continue
		}
		oldEntry, oldOK := beforeEntries[change.id]
		newEntry, newOK := afterEntries[change.id]
		displayName := change.name
		if displayName == "" && newOK {
			displayName = newEntry.Name
		}
		if displayName == "" && oldOK {
			displayName = oldEntry.Name
		}
		filename := "entry-" + change.id + ".json"
		if displayName == "" {
			displayName = "Catalog entry"
		}
		file := catalogDiffDetailFile{EntryID: change.id, DisplayName: displayName, Filename: filename, Kind: change.kind}
		if oldOK {
			text, err := encodeCatalogDetailValue(oldEntry)
			if err != nil {
				return nil, fmt.Errorf("prepare exact catalog comparison")
			}
			file.BeforeText = &text
		}
		if newOK {
			text, err := encodeCatalogDetailValue(newEntry)
			if err != nil {
				return nil, fmt.Errorf("prepare exact catalog comparison")
			}
			file.AfterText = &text
		}
		detail.Files = append(detail.Files, file)
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(detail); err != nil {
		return nil, fmt.Errorf("prepare exact catalog comparison")
	}
	if output.Len() > catalogDiffDetailLimit {
		return nil, fmt.Errorf("the exact comparison is too large to show safely")
	}
	return output.Bytes(), nil
}

func catalogEntriesByID(entries []neutralcatalog.Entry) map[string]neutralcatalog.Entry {
	result := make(map[string]neutralcatalog.Entry, len(entries))
	for _, entry := range entries {
		result[entry.ID] = entry
	}
	return result
}

func encodeCatalogDetailValue(value any) (string, error) {
	contents, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return "", err
	}
	return string(contents) + "\n", nil
}

func (view *catalogDiffWebView) summaryHandler(w http.ResponseWriter, r *http.Request) {
	view.writeJSON(w, r, view.summary)
}

func (view *catalogDiffWebView) detailHandler(w http.ResponseWriter, r *http.Request) {
	view.writeJSON(w, r, view.detail)
}

func (view *catalogDiffWebView) writeJSON(w http.ResponseWriter, r *http.Request, contents []byte) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", fmt.Sprint(len(contents)))
	_, _ = w.Write(contents)
}

func runCatalogDiffWeb(before, after neutralcatalog.Catalog, report neutralcatalog.SemanticDiffReport) error {
	view, err := newCatalogDiffWebView(before, after, report)
	if err != nil {
		return err
	}
	const address = "127.0.0.1:8787"
	token, err := generateWebToken()
	if err != nil {
		return fmt.Errorf("create web session: %w", err)
	}
	handler, err := newWebHandlerWithCatalogDiff(token, address, view)
	if err != nil {
		return err
	}
	fmt.Printf("Alias Lens catalog review is running at http://%s/diff.html#token=%s\n", address, token)
	fmt.Println("This local page is read-only. Press Ctrl+C to close it.")
	return serveLocalWeb(address, handler)
}
