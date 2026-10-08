//go:build !windows

package app

import (
	"fmt"

	"path/filepath"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	workflowplan "github.com/alderon07/al/internal/plan"
	shellapi "github.com/alderon07/al/internal/shell"
)

func (svc *Services) catalogAdoptionForEntry(entry neutralcatalog.Entry, shell, path string, native []byte) (catalogstore.Adoption, error) {
	sourceName := entry.Name
	ledger := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	if _, err := readLifecycleFile(svc.catalogAdoptionsPath(), &ledger); err != nil {
		return catalogstore.Adoption{}, err
	}
	for _, record := range ledger.Records {
		if record.Shell == shell && record.EntryID == entry.ID && record.Path == path {
			sourceName = record.Name
		}
	}
	var match *shadowResult
	adapter, err := svc.shellAdapter(shell)
	if err != nil {
		return catalogstore.Adoption{}, err
	}
	reviewSource, err := shellapi.CatalogNativeReviewSource(adapter, native)
	if err != nil {
		return catalogstore.Adoption{}, err
	}
	for _, result := range importShadowSource(shell, reviewSource) {
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
	return catalogstore.Adoption{EntryID: entry.ID, Shell: shell, Name: sourceName, Kind: match.Kind, Path: path, Start: match.StartByte, End: match.EndByte, Definition: definition, DefinitionSHA256: hashBytes([]byte(definition)), FileSHA256: hashBytes(native), OriginalPath: filepath.Join(svc.catalogStateRoot(), "rollback", hashBytes([]byte(shell+path+hashBytes(native)))+".native"), OriginalSHA256: hashBytes(native)}, nil
}

func (svc *Services) buildCatalogDecisionPlan(decisions CatalogLifecycleDecisions) (workflowplan.OperationPlan, error) {
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
		inputs = append(inputs, svc.planInput("catalog", decisions.SourcePath, canonical))
	}
	actions := []workflowplan.Action{}
	approvals := catalogstore.ApprovalFile{Version: 2, Records: []catalogstore.ApprovalRecord{}}
	before, err := readLifecycleFile(svc.catalogApprovalsPath(), &approvals)
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
	inputs = append(inputs, svc.planInput("approvals", svc.catalogApprovalsPath(), before))
	if len(decisions.Approvals) > 0 {
		action, e := svc.lifecycleAction(svc.catalogApprovalsPath(), "approvals", data)
		if e != nil {
			return workflowplan.OperationPlan{}, e
		}
		actions = append(actions, action)
	}
	adoptions := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	before, err = readLifecycleFile(svc.catalogAdoptionsPath(), &adoptions)
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
		native, err := readRegularFile(record.Path, ShadowSourceLimit)
		if err != nil {
			return workflowplan.OperationPlan{}, err
		}
		if hashBytes(native) != record.FileSHA256 {
			return workflowplan.OperationPlan{}, fmt.Errorf("native source changed; review al catalog adopt again")
		}
		inputs = append(inputs, svc.planInput("native_alias_file", record.Path, native))
	}
	data, err = catalogstore.Encode(adoptions)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, svc.planInput("adoptions", svc.catalogAdoptionsPath(), before))
	if len(decisions.Adoptions) > 0 {
		action, e := svc.lifecycleAction(svc.catalogAdoptionsPath(), "adoptions", data)
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
