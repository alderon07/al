package catalogstore

import (
	"alias-lens/internal/catalog"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

func Hash(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func FramedHash(frames ...[]byte) string {
	h := sha256.New()
	var size [8]byte
	for _, f := range frames {
		binary.BigEndian.PutUint64(size[:], uint64(len(f)))
		h.Write(size[:])
		h.Write(f)
	}
	return hex.EncodeToString(h.Sum(nil))
}
func NativeApproval(entry catalog.Entry, shell string, implementation catalog.NativeImplementation) ApprovalKey {
	field, contents := "alias_value", ""
	if implementation.AliasValue != nil {
		contents = *implementation.AliasValue
	} else {
		field = "function_body"
		if implementation.FunctionBody != nil {
			contents = *implementation.FunctionBody
		}
	}
	return ApprovalKey{EntryID: entry.ID, Shell: shell, Name: entry.Name, Kind: entry.Kind, ImplementationSHA256: FramedHash([]byte(entry.Kind), []byte(field), []byte(contents)), Renderer: shell + "/v2"}
}
func literal(s string) string { return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'" }
func Declaration(entry catalog.Entry, shell, executable string) (string, error) {
	if len(catalog.Validate(catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{entry}})) > 0 || !validShell(shell) {
		return "", errors.New("invalid declaration")
	}
	if n, ok := entry.Native[shell]; ok {
		if entry.Kind == "command" {
			return "alias " + entry.Name + "=" + literal(*n.AliasValue) + "\n", nil
		}
		body := *n.FunctionBody
		if !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		return "function " + entry.Name + " {\n" + body + "}\n", nil
	}
	if entry.Portable == nil || !absolute(executable) {
		return "", errors.New("absolute executable resolution required")
	}
	words := []string{literal(executable)}
	for _, a := range entry.Portable.Args {
		words = append(words, literal(a))
	}
	s := "function " + entry.Name + " {\n  command " + strings.Join(words, " ")
	if entry.Portable.PassArguments {
		s += " \"$@\""
	}
	return s + "\n}\n", nil
}
func canonical(m GenerationManifest) GenerationManifest {
	m.Profiles = append([]string{}, m.Profiles...)
	sort.Strings(m.Profiles)
	m.Entries = append([]GenerationEntry{}, m.Entries...)
	sort.Slice(m.Entries, func(i, j int) bool { return m.Entries[i].Entry.Name < m.Entries[j].Entry.Name })
	m.IncludedEntryIDs = []string{}
	for i := range m.Entries {
		m.Entries[i].Entry = catalog.Normalize(catalog.Catalog{SchemaVersion: 2, Entries: []catalog.Entry{m.Entries[i].Entry}}).Entries[0]
		m.IncludedEntryIDs = append(m.IncludedEntryIDs, m.Entries[i].Entry.ID)
	}
	m.Confirmations = append([]ApprovalKey{}, m.Confirmations...)
	sort.Slice(m.Confirmations, func(i, j int) bool {
		a, _ := json.Marshal(m.Confirmations[i])
		b, _ := json.Marshal(m.Confirmations[j])
		return bytes.Compare(a, b) < 0
	})
	m.ExecutableResolutions = append([]ExecutableResolution{}, m.ExecutableResolutions...)
	sort.Slice(m.ExecutableResolutions, func(i, j int) bool { return m.ExecutableResolutions[i].EntryID < m.ExecutableResolutions[j].EntryID })
	return m
}
func identity(m GenerationManifest) string {
	m.ID = ""
	m.FileSHA256 = ""
	b, _ := json.Marshal(canonical(m))
	return FramedHash(b)
}
func BuildGeneration(m GenerationManifest, body []byte) (GenerationManifest, []byte, error) {
	m = canonical(m)
	m.FileSHA256 = Hash(body)
	m.ID = identity(m)
	if err := Validate(m); err != nil {
		return GenerationManifest{}, nil, err
	}
	if err := verifyDeclarations(m, body, nil); err != nil {
		return GenerationManifest{}, nil, err
	}
	b, err := Encode(m)
	return m, b, err
}
func EncodePointer(id string) ([]byte, error) {
	if !ValidHash(id) {
		return nil, errors.New("invalid generation pointer")
	}
	return []byte(id + "\n"), nil
}
func DecodePointer(data []byte) (string, error) {
	if len(data) != 65 || data[64] != '\n' || !ValidHash(string(data[:64])) {
		return "", errors.New("invalid generation pointer")
	}
	return string(data[:64]), nil
}
func VerifyGeneration(manifest, body, pointer, native []byte, validateNative func(string, []byte) error) (GenerationManifest, error) {
	id, err := DecodePointer(pointer)
	if err != nil {
		return GenerationManifest{}, err
	}
	var m GenerationManifest
	if err := Decode(manifest, &m); err != nil {
		return GenerationManifest{}, err
	}
	if id != m.ID || identity(m) != m.ID || Hash(body) != m.FileSHA256 || Hash(native) != m.NativeInputSHA256 {
		return GenerationManifest{}, errors.New("generation integrity mismatch")
	}
	expected, err := Encode(canonical(m))
	if err != nil || !bytes.Equal(expected, manifest) {
		return GenerationManifest{}, errors.New("noncanonical generation manifest")
	}
	for _, entry := range m.Entries {
		if _, native := entry.Entry.Native[m.Shell]; native && validateNative == nil {
			return GenerationManifest{}, errors.New("native structural validator required")
		}
	}
	if err := verifyDeclarations(m, body, validateNative); err != nil {
		return GenerationManifest{}, err
	}
	return m, nil
}
func verifyDeclarations(m GenerationManifest, body []byte, validate func(string, []byte) error) error {
	resolutions := map[string]ExecutableResolution{}
	for _, r := range m.ExecutableResolutions {
		resolutions[r.EntryID] = r
	}
	confirmations := map[ApprovalKey]bool{}
	for _, k := range m.Confirmations {
		confirmations[k] = true
	}
	var expected strings.Builder
	usedApprovals, usedExecutables := 0, 0
	for _, r := range m.Entries {
		path := ""
		if n, ok := r.Entry.Native[m.Shell]; ok {
			k := NativeApproval(r.Entry, m.Shell, n)
			if !confirmations[k] {
				return errors.New("generation missing native confirmation")
			}
			usedApprovals++
			if validate != nil {
				if err := validate(m.Shell, []byte(r.Declaration)); err != nil {
					return errors.New("invalid native generation declaration")
				}
			}
		} else {
			resolution, ok := resolutions[r.Entry.ID]
			if !ok || r.Entry.Portable == nil || resolution.Program != r.Entry.Portable.Program {
				return errors.New("generation missing executable resolution")
			}
			path = resolution.Path
			usedExecutables++
		}
		declaration, err := Declaration(r.Entry, m.Shell, path)
		if err != nil || declaration != r.Declaration {
			return errors.New("generation declaration mismatch")
		}
		expected.WriteString(declaration)
	}
	if usedApprovals != len(m.Confirmations) || usedExecutables != len(m.ExecutableResolutions) || !bytes.Equal([]byte(expected.String()), body) {
		return errors.New("generation output mismatch")
	}
	return nil
}
