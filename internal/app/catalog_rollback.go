//go:build !windows

package app

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/alderon07/al/internal/catalogstore"
	workflowplan "github.com/alderon07/al/internal/plan"
)

const catalogRollbackBaselineWarning = "Restore enrollment baseline; aliases renamed or deleted since enrollment can return"
const CatalogRollbackConfirmation = "Restore the enrollment baseline and deactivate this catalog? Aliases renamed or deleted since enrollment can return."

func (svc *Services) buildCatalogRollbackPlan(shell string) (workflowplan.OperationPlan, error) {
	installed := catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{}}
	stateBytes, err := readLifecycleFile(svc.catalogInstalledPath(), &installed)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	var state *catalogstore.InstalledState
	for i := range installed.Records {
		if installed.Records[i].Shell == shell {
			state = &installed.Records[i]
		}
	}
	if state == nil {
		return workflowplan.Build("catalog.rollback", nil, nil, nil, nil), nil
	}
	recordPath := filepath.Join(svc.catalogStateRoot(), "rollback", state.RollbackID+".json")
	var record catalogstore.RollbackRecord
	recordBytes, err := readLifecycleFile(recordPath, &record)
	if err != nil || record.ID != state.RollbackID || record.Shell != shell {
		return workflowplan.OperationPlan{}, fmt.Errorf("offline rollback record is unavailable; run al doctor")
	}
	native, err := readRegularFile(record.NativePath, ShadowSourceLimit)
	if err != nil || hashBytes(native) != state.NativeInputSHA256 {
		return workflowplan.OperationPlan{}, fmt.Errorf("native file changed; preserve it and review offline rollback before trying again")
	}
	original, err := catalogstore.ReadPrivateBytes(record.OriginalPath, ShadowSourceLimit)
	if err != nil || hashBytes(original) != record.OriginalSHA256 {
		return workflowplan.OperationPlan{}, fmt.Errorf("offline native rollback copy is invalid")
	}
	inputs := []workflowplan.Input{svc.planInput("installed", svc.catalogInstalledPath(), stateBytes), svc.planInput("rollback", recordPath, recordBytes), svc.planInput("native_alias_file", record.NativePath, native), svc.planInput("rollback_native", record.OriginalPath, original)}
	actions := []workflowplan.Action{}
	add := func(path, role string, data []byte) error {
		action, e := svc.lifecycleAction(path, role, data)
		if e != nil {
			return e
		}
		switch role {
		case "native_alias_file":
			action.Reason = catalogRollbackBaselineWarning
		case "startup":
			action.Reason = "Restore startup files to enrollment baseline"
		case "installed":
			action.Reason = "Remove this shell's installed catalog record"
		case "adoptions":
			action.Reason = "Remove this shell's enrolled fallback ownership records"
		}
		actions = append(actions, action)
		return nil
	}
	if err := add(record.NativePath, "native_alias_file", original); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if !record.NativeExists {
		actions[len(actions)-1].Kind = workflowplan.ActionRemove
		actions[len(actions)-1].Reason = "Remove native file absent at enrollment; aliases renamed or deleted since enrollment can return in restored startup files"
		actions[len(actions)-1].Target.PlannedBytes = nil
	}
	pointerPath := filepath.Join(svc.catalogGeneratedRoot(shell), "active")
	pointer, err := readRegularFile(pointerPath, catalogstore.MaxDocumentBytes)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, svc.planInput("active_pointer", pointerPath, pointer))
	if err == nil {
		actions = append(actions, workflowplan.Action{Kind: workflowplan.ActionRemove, TargetRole: "active_pointer", DisplayPath: svc.displayPrivatePath(pointerPath), Reason: "Deactivate catalog after native restore", Risk: workflowplan.RiskReview, Backup: true, Reversible: true, Target: plannedTarget(pointerPath, pointer, nil)})
	}
	for _, startup := range state.StartupRecords {
		current, e := readRegularFile(startup.Path, ShadowSourceLimit)
		if e != nil || hashBytes(current) != startup.SHA256 {
			return workflowplan.OperationPlan{}, fmt.Errorf("startup changed; preserve it and review offline rollback")
		}
		old, e := catalogstore.ReadPrivateBytes(startup.OriginalPath, ShadowSourceLimit)
		if e != nil || hashBytes(old) != startup.OriginalSHA256 {
			return workflowplan.OperationPlan{}, fmt.Errorf("offline startup rollback copy is invalid")
		}
		inputs = append(inputs, svc.planInput("startup", startup.Path, current), svc.planInput("rollback_startup", startup.OriginalPath, old))
		if err := add(startup.Path, "startup", old); err != nil {
			return workflowplan.OperationPlan{}, err
		}
		if !startup.OriginalExists {
			actions[len(actions)-1].Kind = workflowplan.ActionRemove
			actions[len(actions)-1].Reason = "Remove startup file absent at enrollment and restore startup precedence"
			actions[len(actions)-1].Target.PlannedBytes = nil
		}
	}
	kept := []catalogstore.InstalledState{}
	for _, other := range installed.Records {
		if other.Shell != shell {
			kept = append(kept, other)
		}
	}
	installed.Records = kept
	data, err := catalogstore.Encode(installed)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	if err := add(svc.catalogInstalledPath(), "installed", data); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	adoptions := catalogstore.AdoptionsFile{Version: 1, Records: []catalogstore.Adoption{}}
	before, err := readLifecycleFile(svc.catalogAdoptionsPath(), &adoptions)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	owned := adoptions.Records[:0]
	for _, adoption := range adoptions.Records {
		if adoption.Shell != shell {
			owned = append(owned, adoption)
		}
	}
	adoptions.Records = owned
	data, err = catalogstore.Encode(adoptions)
	if err != nil {
		return workflowplan.OperationPlan{}, err
	}
	inputs = append(inputs, svc.planInput("adoptions", svc.catalogAdoptionsPath(), before))
	if err := add(svc.catalogAdoptionsPath(), "adoptions", data); err != nil {
		return workflowplan.OperationPlan{}, err
	}
	for i := range actions {
		actions[i].Sequence = i + 1
	}
	return workflowplan.Build("catalog.rollback", inputs, actions, nil, nil), nil
}
