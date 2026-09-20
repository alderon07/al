// Package catalog defines the pure, shell-neutral Alias Lens catalog model.
package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

const (
	SchemaVersion    = 2
	MaxDocumentBytes = 8 << 20
	MaxEntries       = 10000
	MaxDiagnostics   = 100
)

type Catalog struct {
	SchemaVersion int     `json:"schema_version"`
	Entries       []Entry `json:"entries"`
}
type Entry struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	Kind        string                          `json:"kind"`
	Description string                          `json:"description,omitempty"`
	Category    string                          `json:"category,omitempty"`
	Tags        []string                        `json:"tags,omitempty"`
	Platforms   []string                        `json:"platforms,omitempty"`
	When        *Conditions                     `json:"when,omitempty"`
	Favorite    bool                            `json:"favorite,omitempty"`
	Portable    *Portable                       `json:"portable,omitempty"`
	Native      map[string]NativeImplementation `json:"native,omitempty"`
}
type Conditions struct {
	ProfilesAny  []string `json:"profiles_any,omitempty"`
	ProfilesNone []string `json:"profiles_none,omitempty"`
	Shells       []string `json:"shells,omitempty"`
}
type Portable struct {
	Program       string   `json:"program"`
	Args          []string `json:"args"`
	PassArguments bool     `json:"pass_arguments"`
}
type NativeImplementation struct {
	AliasValue   *string `json:"alias_value,omitempty"`
	FunctionBody *string `json:"function_body,omitempty"`
}

type wireCatalog struct {
	SchemaVersion int         `json:"schema_version"`
	Entries       []wireEntry `json:"entries"`
}

type wireEntry struct {
	ID          string                          `json:"id"`
	Name        string                          `json:"name"`
	Kind        string                          `json:"kind"`
	Description string                          `json:"description,omitempty"`
	Category    string                          `json:"category,omitempty"`
	Tags        []string                        `json:"tags,omitempty"`
	Platforms   []string                        `json:"platforms,omitempty"`
	When        *Conditions                     `json:"when,omitempty"`
	Favorite    bool                            `json:"favorite,omitempty"`
	Portable    *Portable                       `json:"portable,omitempty"`
	Native      map[string]NativeImplementation `json:"native,omitempty"`
}
type Diagnostic struct {
	Code       string `json:"code"`
	EntryIndex int    `json:"entry_index,omitempty"`
	Field      string `json:"field"`
	Message    string `json:"message"`
	Omitted    int    `json:"omitted,omitempty"`
}
type Change struct {
	ID     string
	Fields []string
}

var idPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)
var commandNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.-]*$`)
var functionNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
var programPattern = regexp.MustCompile(`^[A-Za-z0-9_][A-Za-z0-9_.+-]*$`)
var profileNamePattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)
var deniedPrograms = makeSet(strings.Fields("alias bg break builtin cd command compdef continue declare dirs disown enable eval exec exit export false fc fg functions getopts hash jobs kill let local logout popd pushd read readonly return set shift source suspend times trap true typeset ulimit umask unalias unset wait whence where which . :"))
var fieldRank = makeRank([]string{"root", "schema_version", "entries", "id", "name", "kind", "description", "category", "tags", "platforms", "when", "favorite", "portable", "native"})

func makeSet(values []string) map[string]bool {
	result := map[string]bool{}
	for _, value := range values {
		result[value] = true
	}
	return result
}
func makeRank(values []string) map[string]int {
	result := map[string]int{}
	for index, value := range values {
		result[value] = index
	}
	return result
}
func diagnostic(code string, index int, field, message string) Diagnostic {
	return Diagnostic{Code: code, EntryIndex: index, Field: field, Message: message}
}

func Validate(value Catalog) []Diagnostic {
	var result []Diagnostic
	if value.SchemaVersion != SchemaVersion {
		result = append(result, diagnostic("invalid_schema_version", -1, "schema_version", "schema_version must be 2"))
	}
	if value.Entries == nil {
		result = append(result, diagnostic("missing_entries", -1, "entries", "entries is required"))
	}
	if len(value.Entries) > MaxEntries {
		result = append(result, diagnostic("too_many_entries", -1, "entries", "entries exceeds 10000 items"))
	}
	ids, names := map[string]bool{}, map[string]bool{}
	for index, entry := range value.Entries {
		if !idPattern.MatchString(entry.ID) {
			result = append(result, diagnostic("invalid_id", index, "id", "id must be 32 lowercase hexadecimal characters"))
		} else if ids[entry.ID] {
			result = append(result, diagnostic("duplicate_id", index, "id", "id must be unique"))
		}
		ids[entry.ID] = true
		namePattern := commandNamePattern
		if entry.Kind == "function" {
			namePattern = functionNamePattern
		}
		if !namePattern.MatchString(entry.Name) {
			result = append(result, diagnostic("invalid_name", index, "name", "name is invalid for its kind"))
		} else if names[entry.Name] {
			result = append(result, diagnostic("duplicate_name", index, "name", "name must be unique"))
		}
		names[entry.Name] = true
		if entry.Kind != "command" && entry.Kind != "function" {
			result = append(result, diagnostic("invalid_kind", index, "kind", "kind must be command or function"))
		}
		checkText := func(field, text string, limit int, newline bool) {
			if !utf8.ValidString(text) || strings.ContainsRune(text, 0) || (!newline && strings.ContainsAny(text, "\r\n")) || len(text) > limit {
				result = append(result, diagnostic("invalid_"+field, index, field, field+" is invalid or exceeds its byte limit"))
			}
		}
		checkText("description", entry.Description, 1024, true)
		checkText("category", entry.Category, 64, false)
		if len(entry.Tags) > 64 {
			result = append(result, diagnostic("too_many_tags", index, "tags", "tags exceeds 64 input values"))
		}
		for _, item := range entry.Tags {
			checkText("tags", item, 64, false)
			if item == "" {
				result = append(result, diagnostic("empty_tag", index, "tags", "tags cannot contain an empty value"))
			}
		}
		if len(entry.Platforms) > 4 {
			result = append(result, diagnostic("too_many_platforms", index, "platforms", "platforms exceeds four input values"))
		}
		for _, item := range entry.Platforms {
			if !makeSet([]string{"linux", "macos", "wsl", "windows"})[item] {
				result = append(result, diagnostic("invalid_platform", index, "platforms", "platform is not supported"))
			}
		}
		if entry.When != nil {
			validateConditions(&result, index, entry.When)
		}
		if entry.Portable != nil {
			if entry.Kind != "command" {
				result = append(result, diagnostic("portable_function", index, "portable", "portable is valid only for command entries"))
			}
			validatePortable(&result, index, entry.Portable)
		}
		if len(entry.Native) == 0 && entry.Portable == nil {
			result = append(result, diagnostic("missing_implementation", index, "native", "entry needs a portable or native implementation"))
		}
		for shell, implementation := range entry.Native {
			if shell != "bash" && shell != "zsh" {
				result = append(result, diagnostic("invalid_native_shell", index, "native", "native shell must be bash or zsh"))
			}
			validateNative(&result, index, entry.Kind, implementation)
		}
	}
	return orderAndLimit(result)
}

func validateConditions(result *[]Diagnostic, index int, value *Conditions) {
	if len(value.ProfilesAny) == 0 && len(value.ProfilesNone) == 0 && len(value.Shells) == 0 {
		*result = append(*result, diagnostic("empty_when", index, "when", "when needs at least one condition"))
	}
	validateProfiles := func(field string, values []string) {
		if len(values) > 32 {
			*result = append(*result, diagnostic("too_many_"+field, index, "when", field+" exceeds 32 values"))
		}
		seen := map[string]bool{}
		for _, item := range values {
			if !profileNamePattern.MatchString(item) {
				*result = append(*result, diagnostic("invalid_"+field, index, "when", field+" contains an invalid profile name"))
			}
			if seen[item] {
				*result = append(*result, diagnostic("duplicate_"+field, index, "when", field+" contains a duplicate profile"))
			}
			seen[item] = true
		}
	}
	validateProfiles("profiles_any", value.ProfilesAny)
	validateProfiles("profiles_none", value.ProfilesNone)
	any := makeSet(value.ProfilesAny)
	for _, item := range value.ProfilesNone {
		if any[item] {
			*result = append(*result, diagnostic("overlapping_profiles", index, "when", "a profile cannot appear in both profile lists"))
		}
	}
	seenShells := map[string]bool{}
	for _, shell := range value.Shells {
		if shell != "bash" && shell != "zsh" {
			*result = append(*result, diagnostic("invalid_condition_shell", index, "when", "shells can contain only bash or zsh"))
		}
		if seenShells[shell] {
			*result = append(*result, diagnostic("duplicate_condition_shell", index, "when", "shells contains a duplicate value"))
		}
		seenShells[shell] = true
	}
}

func validatePortable(result *[]Diagnostic, index int, value *Portable) {
	if len(value.Program) > 255 || !programPattern.MatchString(value.Program) || deniedPrograms[value.Program] {
		*result = append(*result, diagnostic("invalid_program", index, "portable", "portable program is not an allowed executable token"))
	}
	if value.Args == nil {
		*result = append(*result, diagnostic("missing_args", index, "portable", "portable args is required"))
	}
	if len(value.Args) > 256 {
		*result = append(*result, diagnostic("too_many_args", index, "portable", "portable args exceeds 256 values"))
	}
	for _, argument := range value.Args {
		if !utf8.ValidString(argument) || strings.ContainsRune(argument, 0) || len(argument) > 65536 {
			*result = append(*result, diagnostic("invalid_argument", index, "portable", "portable argument is invalid or exceeds its byte limit"))
		}
	}
}

func validateNative(result *[]Diagnostic, index int, kind string, value NativeImplementation) {
	valid := false
	if kind == "command" && value.AliasValue != nil && value.FunctionBody == nil {
		valid = *value.AliasValue != "" && utf8.ValidString(*value.AliasValue) && !strings.ContainsRune(*value.AliasValue, 0) && len(*value.AliasValue) <= 65536
	}
	if kind == "function" && value.FunctionBody != nil && value.AliasValue == nil {
		valid = *value.FunctionBody != "" && utf8.ValidString(*value.FunctionBody) && !strings.ContainsRune(*value.FunctionBody, 0) && len(*value.FunctionBody) <= 262144
	}
	if !valid {
		*result = append(*result, diagnostic("invalid_native_implementation", index, "native", "native implementation does not match entry kind or byte limits"))
	}
}

func orderAndLimit(values []Diagnostic) []Diagnostic {
	sort.SliceStable(values, func(i, j int) bool {
		a, b := values[i], values[j]
		if a.EntryIndex != b.EntryIndex {
			return a.EntryIndex < b.EntryIndex
		}
		if fieldRank[a.Field] != fieldRank[b.Field] {
			return fieldRank[a.Field] < fieldRank[b.Field]
		}
		return a.Code < b.Code
	})
	if len(values) <= MaxDiagnostics {
		return values
	}
	omitted := len(values) - MaxDiagnostics
	values = append([]Diagnostic(nil), values[:MaxDiagnostics]...)
	return append(values, Diagnostic{Code: "diagnostics_truncated", EntryIndex: -1, Field: "root", Message: "additional diagnostics omitted", Omitted: omitted})
}

func Normalize(value Catalog) Catalog {
	result := Catalog{SchemaVersion: value.SchemaVersion, Entries: make([]Entry, len(value.Entries))}
	for index, entry := range value.Entries {
		result.Entries[index] = entry
		result.Entries[index].Native = nil
		result.Entries[index].Portable = nil
		result.Entries[index].When = nil
		result.Entries[index].Tags = uniqueSorted(entry.Tags)
		result.Entries[index].Platforms = orderedPlatforms(entry.Platforms)
		if len(result.Entries[index].Tags) == 0 {
			result.Entries[index].Tags = nil
		}
		if len(result.Entries[index].Platforms) == 0 {
			result.Entries[index].Platforms = nil
		}
		if len(entry.Native) > 0 {
			result.Entries[index].Native = map[string]NativeImplementation{}
			for key, item := range entry.Native {
				result.Entries[index].Native[key] = copyNative(item)
			}
		}
		if entry.Portable != nil {
			result.Entries[index].Portable = &Portable{Program: entry.Portable.Program, Args: append([]string{}, entry.Portable.Args...), PassArguments: entry.Portable.PassArguments}
		}
		if entry.When != nil {
			result.Entries[index].When = &Conditions{
				ProfilesAny:  uniqueSorted(entry.When.ProfilesAny),
				ProfilesNone: uniqueSorted(entry.When.ProfilesNone),
				Shells:       uniqueSorted(entry.When.Shells),
			}
		}
	}
	sort.SliceStable(result.Entries, func(i, j int) bool {
		if result.Entries[i].Name == result.Entries[j].Name {
			return result.Entries[i].ID < result.Entries[j].ID
		}
		return result.Entries[i].Name < result.Entries[j].Name
	})
	return result
}
func copyNative(value NativeImplementation) NativeImplementation {
	result := NativeImplementation{}
	if value.AliasValue != nil {
		v := *value.AliasValue
		result.AliasValue = &v
	}
	if value.FunctionBody != nil {
		v := *value.FunctionBody
		result.FunctionBody = &v
	}
	return result
}
func uniqueSorted(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	output := result[:0]
	for _, v := range result {
		if len(output) == 0 || output[len(output)-1] != v {
			output = append(output, v)
		}
	}
	return output
}
func orderedPlatforms(values []string) []string {
	seen := makeSet(values)
	result := []string{}
	for _, v := range []string{"linux", "macos", "wsl", "windows"} {
		if seen[v] {
			result = append(result, v)
		}
	}
	return result
}

func Encode(value Catalog) ([]byte, []Diagnostic) {
	value = Normalize(value)
	diagnostics := Validate(value)
	if len(diagnostics) > 0 {
		return nil, diagnostics
	}
	var output bytes.Buffer
	encoder := json.NewEncoder(&output)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(toWireCatalog(value)); err != nil {
		return nil, []Diagnostic{diagnostic("encode_failed", -1, "root", "catalog encoding failed")}
	}
	if output.Len() > MaxDocumentBytes {
		return nil, []Diagnostic{diagnostic("encoded_document_too_large", -1, "root", "encoded catalog exceeds 8 MiB")}
	}
	return output.Bytes(), nil
}

func Decode(data []byte) (Catalog, []Diagnostic) {
	if len(data) > MaxDocumentBytes {
		return Catalog{}, []Diagnostic{diagnostic("document_too_large", -1, "root", "catalog exceeds 8 MiB")}
	}
	if !utf8.Valid(data) {
		return Catalog{}, []Diagnostic{diagnostic("invalid_utf8", -1, "root", "catalog is not valid UTF-8")}
	}
	if err := checkJSON(data); err != nil {
		return Catalog{}, []Diagnostic{diagnostic("invalid_json", -1, "root", err.Error())}
	}
	if err := checkRequiredFields(data); err != nil {
		return Catalog{}, []Diagnostic{diagnostic("invalid_structure", -1, "root", err.Error())}
	}
	var header struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &header); err != nil {
		return Catalog{}, []Diagnostic{diagnostic("invalid_structure", -1, "root", "catalog structure is invalid")}
	}
	if header.SchemaVersion != SchemaVersion {
		return Catalog{}, Validate(Catalog{SchemaVersion: header.SchemaVersion, Entries: []Entry{}})
	}
	var wire wireCatalog
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&wire); err != nil {
		return Catalog{}, []Diagnostic{diagnostic("invalid_structure", -1, "root", "catalog structure is invalid")}
	}
	value := fromWireCatalog(wire)
	value = Normalize(value)
	diagnostics := Validate(value)
	if len(diagnostics) > 0 {
		return Catalog{}, diagnostics
	}
	return value, nil
}

func toWireCatalog(value Catalog) wireCatalog {
	result := wireCatalog{SchemaVersion: SchemaVersion, Entries: make([]wireEntry, len(value.Entries))}
	for index, entry := range value.Entries {
		result.Entries[index] = wireEntry{ID: entry.ID, Name: entry.Name, Kind: entry.Kind, Description: entry.Description, Category: entry.Category, Tags: entry.Tags, Platforms: entry.Platforms, When: entry.When, Favorite: entry.Favorite, Portable: entry.Portable, Native: entry.Native}
	}
	return result
}

func fromWireCatalog(value wireCatalog) Catalog {
	result := Catalog{SchemaVersion: SchemaVersion, Entries: make([]Entry, len(value.Entries))}
	for index, entry := range value.Entries {
		result.Entries[index] = Entry{ID: entry.ID, Name: entry.Name, Kind: entry.Kind, Description: entry.Description, Category: entry.Category, Tags: entry.Tags, Platforms: entry.Platforms, When: entry.When, Favorite: entry.Favorite, Portable: entry.Portable, Native: entry.Native}
	}
	return result
}

func checkRequiredFields(data []byte) error {
	var root map[string]json.RawMessage
	if err := json.Unmarshal(data, &root); err != nil {
		return fmt.Errorf("catalog structure is invalid")
	}
	if _, ok := root["schema_version"]; !ok {
		return fmt.Errorf("schema_version is required")
	}
	var schemaVersion int
	if err := json.Unmarshal(root["schema_version"], &schemaVersion); err != nil {
		return fmt.Errorf("schema_version must be a number")
	}
	entriesRaw, ok := root["entries"]
	if !ok {
		return fmt.Errorf("entries is required")
	}
	var entries []map[string]json.RawMessage
	if err := json.Unmarshal(entriesRaw, &entries); err != nil {
		return fmt.Errorf("entries must be an array")
	}
	for _, entry := range entries {
		for _, field := range []string{"id", "name", "kind"} {
			if _, ok := entry[field]; !ok {
				return fmt.Errorf("entry required field is missing")
			}
		}
		if raw, ok := entry["portable"]; ok {
			var portable map[string]json.RawMessage
			if err := json.Unmarshal(raw, &portable); err != nil {
				return fmt.Errorf("portable must be an object")
			}
			for _, field := range []string{"program", "args", "pass_arguments"} {
				if _, ok := portable[field]; !ok {
					return fmt.Errorf("portable required field is missing")
				}
			}
		}
		if raw, ok := entry["when"]; ok {
			var conditions map[string]json.RawMessage
			if err := json.Unmarshal(raw, &conditions); err != nil {
				return fmt.Errorf("when must be an object")
			}
			for _, field := range []string{"profiles_any", "profiles_none", "shells"} {
				list, present := conditions[field]
				if !present {
					continue
				}
				var values []string
				if err := json.Unmarshal(list, &values); err != nil || len(values) == 0 {
					return fmt.Errorf("when.%s must be a non-empty list", field)
				}
			}
		}
	}
	return nil
}

func checkJSON(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := walkJSON(decoder, 0); err != nil {
		return err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fmt.Errorf("catalog has trailing JSON data")
	}
	return nil
}
func walkJSON(decoder *json.Decoder, depth int) error {
	if depth > 32 {
		return fmt.Errorf("catalog JSON is too deeply nested")
	}
	token, err := decoder.Token()
	if err != nil {
		return fmt.Errorf("catalog JSON is malformed")
	}
	if token == nil {
		return fmt.Errorf("catalog JSON cannot contain null")
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		seen := map[string]bool{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return fmt.Errorf("catalog JSON is malformed")
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("catalog object key is invalid")
			}
			if seen[key] {
				return fmt.Errorf("catalog contains a duplicate field")
			}
			seen[key] = true
			if err := walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim('}') {
			return fmt.Errorf("catalog JSON is malformed")
		}
	case '[':
		for decoder.More() {
			if err := walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil || end != json.Delim(']') {
			return fmt.Errorf("catalog JSON is malformed")
		}
	default:
		return fmt.Errorf("catalog JSON is malformed")
	}
	return nil
}

func Compare(left, right Catalog) ([]Change, []Diagnostic) {
	diagnostics := append(Validate(left), Validate(right)...)
	if len(diagnostics) > 0 {
		return nil, orderAndLimit(diagnostics)
	}
	left = Normalize(left)
	right = Normalize(right)
	rightByID := map[string]Entry{}
	for _, entry := range right.Entries {
		rightByID[entry.ID] = entry
	}
	var changes []Change
	for _, entry := range left.Entries {
		other, ok := rightByID[entry.ID]
		fields := []string{}
		if !ok {
			fields = []string{"missing"}
		} else {
			fields = changedFields(entry, other)
			delete(rightByID, entry.ID)
		}
		if len(fields) > 0 {
			changes = append(changes, Change{ID: entry.ID, Fields: fields})
		}
	}
	for id := range rightByID {
		changes = append(changes, Change{ID: id, Fields: []string{"added"}})
	}
	sort.Slice(changes, func(i, j int) bool { return changes[i].ID < changes[j].ID })
	return changes, nil
}
func changedFields(a, b Entry) []string {
	fields := []string{}
	if a.Name != b.Name {
		fields = append(fields, "name")
	}
	if a.Kind != b.Kind {
		fields = append(fields, "kind")
	}
	if a.Description != b.Description {
		fields = append(fields, "description")
	}
	if a.Category != b.Category {
		fields = append(fields, "category")
	}
	if strings.Join(a.Tags, "\x00") != strings.Join(b.Tags, "\x00") {
		fields = append(fields, "tags")
	}
	if strings.Join(a.Platforms, "\x00") != strings.Join(b.Platforms, "\x00") {
		fields = append(fields, "platforms")
	}
	aj, _ := json.Marshal(a.When)
	bj, _ := json.Marshal(b.When)
	if !bytes.Equal(aj, bj) {
		fields = append(fields, "when")
	}
	if a.Favorite != b.Favorite {
		fields = append(fields, "favorite")
	}
	aj, _ = json.Marshal(a.Portable)
	bj, _ = json.Marshal(b.Portable)
	if !bytes.Equal(aj, bj) {
		fields = append(fields, "portable")
	}
	aj, _ = json.Marshal(a.Native)
	bj, _ = json.Marshal(b.Native)
	if !bytes.Equal(aj, bj) {
		fields = append(fields, "native")
	}
	return fields
}
