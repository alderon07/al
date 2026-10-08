//go:build !windows

package app

import (
	"bytes"
	"context"
	shellapi "github.com/alderon07/al/internal/shell"

	"encoding/json"
	"errors"
	"fmt"

	"os"
	"os/exec"
	"path/filepath"

	"sort"
	"strings"
	"syscall"
	"time"
	"unicode/utf8"
)

var shadowValidatorPath = trustedShadowShell
var shadowSyntaxValidator = validateShadowSyntax

const (
	shadowConfigLimit = 1 << 20
	ShadowSourceLimit = 8 << 20
	shadowLineLimit   = 1 << 20
	ShadowReportLimit = 16 << 20
	shadowMaxResults  = 20000
	shadowMaxEntries  = 10000
	shadowMaxLaunches = 256
	shadowMaxLines    = 100000
)

type shadowDiagnostic = shellapi.Diagnostic
type shadowResult = shellapi.Result
type shadowSummary struct {
	Equivalent  int `json:"equivalent"`
	Unsupported int `json:"unsupported"`
	Different   int `json:"different"`
	Duplicate   int `json:"duplicate"`
	Invalid     int `json:"invalid"`
	Blocked     int `json:"blocked"`
}
type ShadowReport struct {
	SchemaVersion int                `json:"schema_version"`
	Shell         string             `json:"shell"`
	Summary       shadowSummary      `json:"summary"`
	Diagnostics   []shadowDiagnostic `json:"diagnostics"`
	Results       []shadowResult     `json:"results"`
}

func (svc *Services) inspectCatalogShadow(explicitShell string) (ShadowReport, error) {
	adapter, err := svc.shadowShellAdapter(explicitShell)
	if err != nil {
		return ShadowReport{}, err
	}
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return ShadowReport{}, fmt.Errorf("find home directory: %w", err)
	}
	path := filepath.Join(home, adapter.AliasFilename())
	source, err := readShadowRegular(path, ShadowSourceLimit)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return ShadowReport{}, fmt.Errorf("selected alias file is missing; run al setup %s", adapter.Name())
		}
		return ShadowReport{}, fmt.Errorf("selected alias file is unreadable; check its type, size, and permissions")
	}
	if bytes.IndexByte(source, 0) >= 0 || !utf8.Valid(source) {
		return ShadowReport{}, fmt.Errorf("alias file is not valid UTF-8 text")
	}
	if sourceLineCount(source) > shadowMaxLines {
		return ShadowReport{}, fmt.Errorf("alias file contains more than 100000 lines")
	}
	if lineTooLong(source) {
		return ShadowReport{}, fmt.Errorf("alias file contains a line over 1 MiB")
	}
	findings := findSecretFindings(source)
	results := importShadowSource(adapter.Name(), source)
	definitions := 0
	for _, result := range results {
		if result.Entry != nil {
			definitions++
		}
	}
	if definitions > shadowMaxEntries {
		return ShadowReport{}, fmt.Errorf("alias file contains more than 10000 proven definitions")
	}
	if len(results) > shadowMaxResults {
		return ShadowReport{}, fmt.Errorf("alias file produces more than 20000 inspection ranges")
	}
	results = blockShadowSecrets(results, findings, source)
	if len(results) > shadowMaxResults {
		return ShadowReport{}, fmt.Errorf("alias file produces more than 20000 inspection ranges")
	}
	assignShadowUnits(results)
	markShadowDuplicates(results)
	validateShadowCandidates(results)
	svc.validateShadowRendered(adapter.Name(), results)
	report := ShadowReport{SchemaVersion: 1, Shell: adapter.Name(), Diagnostics: []shadowDiagnostic{}, Results: results}
	finalizeShadowReport(&report)
	return report, nil
}

func assignShadowUnits(results []shadowResult) {
	indices := make([]int, len(results))
	for index := range results {
		indices[index] = index
	}
	sort.SliceStable(indices, func(i, j int) bool {
		left, right := results[indices[i]], results[indices[j]]
		if left.StartByte != right.StartByte {
			return left.StartByte < right.StartByte
		}
		return left.EndByte < right.EndByte
	})
	for unit, index := range indices {
		results[index].Unit = unit
	}
}

func (svc *Services) shadowShellAdapter(explicit string) (ShellAdapter, error) {
	if explicit != "" {
		return svc.shellAdapter(explicit)
	}
	home, err := svc.dependencies.HomeDir()
	if err != nil {
		return nil, err
	}
	path := filepath.Join(home, ".config", "alias-lens", "config.json")
	contents, err := readShadowRegular(path, shadowConfigLimit)
	if errors.Is(err, os.ErrNotExist) {
		return svc.mustShellAdapter("bash"), nil
	}
	if err != nil {
		return nil, fmt.Errorf("configuration unreadable; check config.json type, size, and permissions")
	}
	var raw map[string]json.RawMessage
	if err := decodeUniqueJSON(contents, &raw); err != nil {
		if errors.Is(err, errShadowDuplicateConfigField) {
			return nil, errShadowDuplicateConfigField
		}
		return nil, fmt.Errorf("invalid configuration")
	}
	value, ok := raw["version"]
	if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return nil, fmt.Errorf("invalid configuration version")
	}
	var version int
	if err := json.Unmarshal(value, &version); err != nil {
		return nil, fmt.Errorf("invalid configuration version")
	}
	if version != currentConfigVersion {
		return nil, fmt.Errorf("configuration version unsupported")
	}
	shellName := "bash"
	if value, ok := raw["shell"]; ok {
		if bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return nil, fmt.Errorf("invalid configuration")
		}
		if err := json.Unmarshal(value, &shellName); err != nil {
			return nil, fmt.Errorf("invalid configuration")
		}
		if strings.TrimSpace(shellName) == "" {
			shellName = "bash"
		}
	}
	adapter, err := svc.shellAdapter(shellName)
	if err != nil {
		return nil, fmt.Errorf("unsupported configured shell %q", shellName)
	}
	return adapter, nil
}

func readShadowRegular(path string, limit int64) ([]byte, error) {
	return readRegularFile(path, limit)
}
func lineTooLong(source []byte) bool { return shellapi.LineTooLong(source) }

func sourceLineCount(source []byte) int { return shellapi.SourceLineCount(source) }

func importShadowSource(name string, source []byte) []shadowResult {
	return shellapi.ImportShadowSource(name, source)
}

type sourceLine = shellapi.SourceLine

func sourceLines(source []byte) []sourceLine { return shellapi.SourceLines(source) }

func blockShadowSecrets(results []shadowResult, findings []SecretFinding, source []byte) []shadowResult {
	lines := sourceLines(source)
	owners := make([]int, len(lines)+1)
	for line := range owners {
		owners[line] = -1
	}
	indices := make([]int, len(results))
	for index := range results {
		indices[index] = index
	}
	sort.SliceStable(indices, func(left, right int) bool {
		leftResult, rightResult := results[indices[left]], results[indices[right]]
		if leftResult.StartLine != rightResult.StartLine {
			return leftResult.StartLine < rightResult.StartLine
		}
		return leftResult.EndLine < rightResult.EndLine
	})
	resultOffset := 0
	for line := 1; line <= len(lines) && resultOffset < len(indices); line++ {
		for resultOffset < len(indices) && results[indices[resultOffset]].EndLine < line {
			resultOffset++
		}
		if resultOffset < len(indices) {
			candidate := results[indices[resultOffset]]
			if candidate.StartLine <= line && line <= candidate.EndLine {
				owners[line] = indices[resultOffset]
			}
		}
	}
	for _, finding := range findings {
		if finding.Line <= 0 || finding.Line > len(lines) {
			continue
		}
		index := owners[finding.Line]
		if index < 0 {
			line := lines[finding.Line-1]
			results = append(results, shadowResult{Unit: len(results), Status: "blocked", StartByte: line.Start, EndByte: line.End, StartLine: finding.Line, EndLine: finding.Line, Diagnostics: []shadowDiagnostic{{Code: "secret_detected", Message: fmt.Sprintf("possible %s on line %d", finding.Kind, finding.Line)}}})
			index = len(results) - 1
			owners[finding.Line] = index
			continue
		}
		results[index].Status = "blocked"
		results[index].Entry = nil
		results[index].Diagnostics = append(results[index].Diagnostics, shadowDiagnostic{Code: "secret_detected", Message: fmt.Sprintf("possible %s on line %d", finding.Kind, finding.Line)})
	}
	return results
}
func markShadowDuplicates(results []shadowResult)     { shellapi.MarkShadowDuplicates(results) }
func validateShadowCandidates(results []shadowResult) { shellapi.ValidateShadowCandidates(results) }

func quoteShadow(value string) string { return shellapi.QuoteShadow(value) }

func (svc *Services) validateShadowRendered(shell string, results []shadowResult) {
	indices := []int{}
	for index := range results {
		if results[index].Entry != nil {
			indices = append(indices, index)
		}
	}
	if len(indices) == 0 {
		return
	}
	overall, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	launches := 0
	contentsFor := func(batch []int) []byte {
		var body bytes.Buffer
		for _, index := range batch {
			body.Write(results[index].Rendered)
		}
		return body.Bytes()
	}
	blockBatch := func(batch []int) {
		for _, index := range batch {
			blockShadowValidationResult(&results[index])
		}
	}
	var validateBatch func([]int)
	validateBatch = func(batch []int) {
		if len(batch) == 0 {
			return
		}
		if overall.Err() != nil || launches >= shadowMaxLaunches {
			blockBatch(batch)
			return
		}
		launches++
		err := svc.validateSyntax(overall, shell, contentsFor(batch))
		if err == nil {
			for _, index := range batch {
				compareShadowRoundTrip(shell, &results[index])
			}
			return
		}
		if shadowValidationBlocked(err) {
			blockBatch(batch)
			return
		}
		if len(batch) == 1 {
			index := batch[0]
			results[index].Status = "invalid"
			results[index].Diagnostics = append(results[index].Diagnostics, shadowDiagnostic{Code: "native_syntax", Message: "rendered definition failed native syntax validation"})
			return
		}
		middle := len(batch) / 2
		validateBatch(batch[:middle])
		validateBatch(batch[middle:])
	}
	validateBatch(indices)
}

func compareShadowRoundTrip(name string, result *shadowResult) {
	shellapi.CompareShadowRoundTrip(name, result)
}
func shadowValidationBlocked(err error) bool {
	var exitError *exec.ExitError
	if !errors.As(err, &exitError) {
		return true
	}
	if exitError.ProcessState == nil {
		return false
	}
	status, ok := exitError.Sys().(syscall.WaitStatus)
	return !ok || status.Signaled()
}
func blockShadowValidationResult(result *shadowResult) {
	result.Status = "blocked"
	result.Diagnostics = append(result.Diagnostics, shadowDiagnostic{Code: "validator_blocked", Message: "native syntax validation could not complete"})
}
func validateShadowSyntax(ctx context.Context, name string, contents []byte) error {
	return shellapi.ValidateSyntax(ctx, name, contents, shadowValidatorPath)
}

func trustedShadowShell(name string) (string, error) { return shellapi.TrustedShadowShell(name) }

func finalizeShadowReport(report *ShadowReport) {
	report.Summary = shadowSummary{}
	sort.SliceStable(report.Results, func(i, j int) bool {
		rank := map[string]int{"invalid": 0, "blocked": 1, "duplicate": 2, "different": 3, "unsupported": 4, "equivalent": 5}
		if rank[report.Results[i].Status] != rank[report.Results[j].Status] {
			return rank[report.Results[i].Status] < rank[report.Results[j].Status]
		}
		if report.Results[i].StartByte != report.Results[j].StartByte {
			return report.Results[i].StartByte < report.Results[j].StartByte
		}
		return report.Results[i].Unit < report.Results[j].Unit
	})
	for index := range report.Results {
		sort.SliceStable(report.Results[index].Diagnostics, func(i, j int) bool {
			return report.Results[index].Diagnostics[i].Code < report.Results[index].Diagnostics[j].Code
		})
		result := report.Results[index]
		switch result.Status {
		case "equivalent":
			report.Summary.Equivalent++
		case "unsupported":
			report.Summary.Unsupported++
		case "different":
			report.Summary.Different++
		case "duplicate":
			report.Summary.Duplicate++
		case "invalid":
			report.Summary.Invalid++
		case "blocked":
			report.Summary.Blocked++
		}
	}
	limitShadowDiagnostics(report)
}

func limitShadowDiagnostics(report *ShadowReport) {
	sort.SliceStable(report.Diagnostics, func(i, j int) bool { return report.Diagnostics[i].Code < report.Diagnostics[j].Code })
	remaining := 100
	omitted := 0
	if len(report.Diagnostics) > remaining {
		omitted += len(report.Diagnostics) - remaining
		report.Diagnostics = append([]shadowDiagnostic(nil), report.Diagnostics[:remaining]...)
		remaining = 0
	} else {
		remaining -= len(report.Diagnostics)
	}
	for index := range report.Results {
		diagnostics := report.Results[index].Diagnostics
		if len(diagnostics) > remaining {
			omitted += len(diagnostics) - remaining
			report.Results[index].Diagnostics = append([]shadowDiagnostic(nil), diagnostics[:remaining]...)
			remaining = 0
		} else {
			remaining -= len(diagnostics)
		}
	}
	if omitted > 0 {
		report.Diagnostics = append(report.Diagnostics, shadowDiagnostic{Code: "diagnostics_truncated", Message: fmt.Sprintf("%d additional diagnostics omitted", omitted)})
	}
}
