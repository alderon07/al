//go:build !windows

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	neutralcatalog "alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	workflowplan "alias-lens/internal/plan"
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

func guidedCatalogLifecycleReview(value neutralcatalog.Catalog, shell string, adopt bool) (catalogLifecycleDecisions, error) {
	encoded, _ := neutralcatalog.Encode(value)
	decisions := catalogLifecycleDecisions{ConfirmedAt: lifecycleTimestamp(), SourcePath: localCatalogPath(), SourceSHA256: hashBytes(encoded)}
	if !catalogReviewTerminal() {
		return decisions, fmt.Errorf("native approval and ownership review need an interactive terminal")
	}
	reader := newCatalogReviewReader()
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	if _, err := readLifecycleFile(catalogApprovalsPath(), &approvals); err != nil {
		return decisions, err
	}
	approved := map[catalogstore.ApprovalKey]bool{}
	for _, record := range approvals.Records {
		approved[record.Key] = true
	}
	adapter, err := shellAdapter(shell)
	if err != nil {
		return decisions, err
	}
	path := filepath.Join(homeDirectory(), adapter.AliasFilename())
	native, err := readRegularFile(path, shadowSourceLimit)
	if os.IsNotExist(err) {
		native = []byte{}
	} else if err != nil {
		return decisions, err
	}
	for _, entry := range neutralcatalog.Normalize(value).Entries {
		implementation, exists := entry.Native[shell]
		if exists {
			declaration, err := catalogstore.Declaration(entry, shell, "")
			if err != nil {
				return decisions, err
			}
			if err := validateCatalogDeclaration(shell, entry, []byte(declaration)); err != nil {
				return decisions, err
			}
			key := catalogstore.NativeApproval(entry, shell, implementation)
			if !approved[key] {
				fmt.Fprintf(catalogWorkflowStdout, "\nReview %s (%s, %s)\nID: %s\nImplementation: %s\nExact declaration: %q\n", escapePlainText(entry.Name), entry.Kind, shell, entry.ID, key.ImplementationSHA256, declaration)
				yes, err := confirmCatalogReview(reader, "Approve this exact native implementation?")
				if err != nil {
					return decisions, err
				}
				if yes {
					decisions.Approvals = append(decisions.Approvals, catalogstore.ApprovalRecord{Key: key, ApprovedAt: decisions.ConfirmedAt})
				}
			}
		}
		if adopt {
			record, err := catalogAdoptionForEntry(entry, shell, path, native)
			if err != nil {
				continue
			}
			fmt.Fprintf(catalogWorkflowStdout, "\nOwnership %s\nSource: %s\nBytes: %d-%d\nRange: %s\nExact fallback: %q\nCandidate: %s\n", escapePlainText(entry.Name), displayPrivatePath(path), record.Start, record.End, record.DefinitionSHA256, record.Definition, quotedCatalogValue(entry))
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

func catalogAdoptionForEntry(entry neutralcatalog.Entry, shell, path string, native []byte) (catalogstore.Adoption, error) {
	sourceName := entry.Name
	ledger := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	if _, err := readLifecycleFile(catalogAdoptionsPath(), &ledger); err != nil {
		return catalogstore.Adoption{}, err
	}
	for _, record := range ledger.Records {
		if record.Shell == shell && record.EntryID == entry.ID && record.Path == path {
			sourceName = record.Name
		}
	}
	var match *shadowResult
	for _, result := range importShadowSource(shell, native) {
		if result.Name == sourceName {
			if match != nil || result.Entry == nil {
				return catalogstore.Adoption{}, fmt.Errorf("native name is ambiguous")
			}
			copy := result
			match = &copy
		}
	}
	if match == nil {
		return catalogstore.Adoption{}, fmt.Errorf("no native declaration is available to enroll")
	}

	definition := string(native[match.StartByte:match.EndByte])
	return catalogstore.Adoption{EntryID: entry.ID, Shell: shell, Name: sourceName, Kind: match.Kind, Path: path, Start: match.StartByte, End: match.EndByte, Definition: definition, DefinitionSHA256: hashBytes([]byte(definition)), FileSHA256: hashBytes(native), OriginalPath: filepath.Join(catalogStateRoot(), "rollback", hashBytes([]byte(shell+path+hashBytes(native)))+".native"), OriginalSHA256: hashBytes(native)}, nil
}

func buildCatalogDecisionPlan(decisions catalogLifecycleDecisions) (workflowplan.OperationPlan, error) {
	inputs := []workflowplan.Input{}
	if decisions.SourcePath != "" {
		source, err := readRegularFile(decisions.SourcePath, neutralcatalog.MaxDocumentBytes)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		value, problems := neutralcatalog.Decode(source)
		if len(problems) > 0 {
			return workflowplan.OperationPlan{}, fmt.Errorf("review source changed")
		}
		canonical, _ := neutralcatalog.Encode(value)
		if hashBytes(canonical) != decisions.SourceSHA256 {
			return workflowplan.OperationPlan{}, fmt.Errorf("catalog changed; repeat the exact review")
		}
		inputs = append(inputs, planInput("catalog", decisions.SourcePath, canonical))
	}
	actions := []workflowplan.Action{}
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	before, err := readLifecycleFile(catalogApprovalsPath(), &approvals)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	keys := map[catalogstore.ApprovalKey]bool{}
	for _, record := range approvals.Records {
		keys[record.Key] = true
	}
	for _, record := range decisions.Approvals {
		if !keys[record.Key] {
			approvals.Records = append(approvals.Records, record)
			keys[record.Key] = true
		}
	}
	data, err := catalogstore.Encode(approvals)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, planInput("approvals", catalogApprovalsPath(), before))
	if len(decisions.Approvals) > 0 {
		action, e := lifecycleAction(catalogApprovalsPath(), "approvals", data)
		if e != nil {
			return workflowplan.OperationPlan{}, e
		}
		actions = append(actions, action)
	}
	adoptions := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	before, err = readLifecycleFile(catalogAdoptionsPath(), &adoptions)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	for _, record := range decisions.Adoptions {
		kept := adoptions.Records[:0]
		for _, old := range adoptions.Records {
			if old.Shell != record.Shell || old.EntryID != record.EntryID {
				kept = append(kept, old)
			}
		}
		adoptions.Records = append(kept, record)
		native, err := readRegularFile(record.Path, shadowSourceLimit)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		if hashBytes(native) != record.FileSHA256 {
			return workflowplan.OperationPlan{}, fmt.Errorf("native source changed; review al catalog adopt again")
		}
		inputs = append(inputs, planInput("native_alias_file", record.Path, native))
	}
	data, err = catalogstore.Encode(adoptions)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, planInput("adoptions", catalogAdoptionsPath(), before))
	if len(decisions.Adoptions) > 0 {
		action, e := lifecycleAction(catalogAdoptionsPath(), "adoptions", data)
		if e != nil {
			return workflowplan.OperationPlan{}, e
		}
		actions = append(actions, action)
	}
	for i := range actions {
		actions[i].Sequence = i + 1
	}
	return workflowplan.Build("catalog.review", inputs, actions, nil, nil), nil
}
