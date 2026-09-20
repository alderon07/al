package catalogrender

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"sort"
	"strings"

	"alias-lens/internal/catalog"
)

type NativeApprovalKey struct {
	EntryID              string `json:"entry_id"`
	Shell                string `json:"shell"`
	Kind                 string `json:"kind"`
	ImplementationSHA256 string `json:"implementation_sha256"`
	Renderer             string `json:"renderer"`
}

type RenderContext struct {
	Shell     string
	Platform  string
	Profiles  []string
	Approvals map[NativeApprovalKey]bool
}

type RenderResult struct {
	Body             []byte
	ResolvedSHA256   string
	ApprovalSHA256   string
	PendingApprovals []NativeApprovalKey
	IncludedEntryIDs []string
	Unavailable      []catalog.ResolvedEntry
}

func RendererID(shell string) string {
	if shell == "bash" || shell == "zsh" {
		return shell + "/v1"
	}
	return ""
}

func Render(value catalog.Catalog, context RenderContext) (RenderResult, []catalog.Diagnostic) {
	resolved, diagnostics := catalog.Resolve(value, catalog.ResolveContext{Shell: context.Shell, Platform: context.Platform, Profiles: context.Profiles})
	if RendererID(context.Shell) == "" {
		diagnostics = append(diagnostics, catalog.Diagnostic{Code: "unsupported_shell", Field: "shell", Message: "shell must be bash or zsh"})
	}
	if len(diagnostics) > 0 {
		return RenderResult{}, diagnostics
	}
	result := RenderResult{PendingApprovals: []NativeApprovalKey{}, IncludedEntryIDs: []string{}, Unavailable: []catalog.ResolvedEntry{}}
	var body bytes.Buffer
	approvedKeys := make([]NativeApprovalKey, 0)
	for _, item := range resolved {
		if !item.Available {
			result.Unavailable = append(result.Unavailable, item)
			continue
		}
		entry := item.Entry
		if native, exists := entry.Native[context.Shell]; exists {
			key := NativeApproval(entry, context.Shell, native)
			if !context.Approvals[key] {
				result.PendingApprovals = append(result.PendingApprovals, key)
				continue
			}
			body.Write(renderNative(entry, native))
			approvedKeys = append(approvedKeys, key)
			result.IncludedEntryIDs = append(result.IncludedEntryIDs, entry.ID)
			continue
		}
		if entry.Portable == nil {
			continue
		}
		body.Write(renderPortable(entry, *entry.Portable))
		result.IncludedEntryIDs = append(result.IncludedEntryIDs, entry.ID)
	}
	sort.Slice(result.PendingApprovals, func(i, j int) bool { return approvalLess(result.PendingApprovals[i], result.PendingApprovals[j]) })
	sort.Slice(approvedKeys, func(i, j int) bool { return approvalLess(approvedKeys[i], approvedKeys[j]) })
	result.Body = body.Bytes()
	result.ResolvedSHA256 = framedHash([][]byte{[]byte(RendererID(context.Shell)), []byte(context.Shell), []byte(context.Platform), []byte(strings.Join(sortedCopy(context.Profiles), "\x00")), result.Body})
	approvalFrames := make([][]byte, 0, len(approvedKeys)*5)
	for _, key := range approvedKeys {
		approvalFrames = append(approvalFrames, []byte(key.EntryID), []byte(key.Shell), []byte(key.Kind), []byte(key.ImplementationSHA256), []byte(key.Renderer))
	}
	result.ApprovalSHA256 = framedHash(approvalFrames)
	return result, nil
}

func NativeApproval(entry catalog.Entry, shell string, implementation catalog.NativeImplementation) NativeApprovalKey {
	field := "alias_value"
	contents := ""
	if implementation.AliasValue != nil {
		contents = *implementation.AliasValue
	} else {
		field = "function_body"
		if implementation.FunctionBody != nil {
			contents = *implementation.FunctionBody
		}
	}
	return NativeApprovalKey{EntryID: entry.ID, Shell: shell, Kind: entry.Kind, ImplementationSHA256: framedHash([][]byte{[]byte(entry.Kind), []byte(field), []byte(contents)}), Renderer: RendererID(shell)}
}

func renderNative(entry catalog.Entry, implementation catalog.NativeImplementation) []byte {
	var output strings.Builder
	writeDescription(&output, entry.Description)
	if entry.Kind == "command" {
		output.WriteString("alias ")
		output.WriteString(entry.Name)
		output.WriteByte('=')
		output.WriteString(shellLiteral(*implementation.AliasValue))
		output.WriteByte('\n')
	} else {
		output.WriteString(entry.Name)
		output.WriteString("() {\n")
		output.WriteString(*implementation.FunctionBody)
		if !strings.HasSuffix(*implementation.FunctionBody, "\n") {
			output.WriteByte('\n')
		}
		output.WriteString("}\n")
	}
	return []byte(output.String())
}

func renderPortable(entry catalog.Entry, portable catalog.Portable) []byte {
	words := make([]string, 0, len(portable.Args)+1)
	words = append(words, shellLiteral(portable.Program))
	for _, argument := range portable.Args {
		words = append(words, shellLiteral(argument))
	}
	var output strings.Builder
	writeDescription(&output, entry.Description)
	output.WriteString(entry.Name)
	output.WriteString("() {\n  command ")
	output.WriteString(strings.Join(words, " "))
	if portable.PassArguments {
		output.WriteString(" \"$@\"")
	}
	output.WriteString("\n}\n")
	return []byte(output.String())
}

func writeDescription(output *strings.Builder, description string) {
	if description != "" {
		output.WriteString("# ")
		output.WriteString(strings.ReplaceAll(description, "\n", " "))
		output.WriteByte('\n')
	}
}

func shellLiteral(value string) string { return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'" }

func framedHash(frames [][]byte) string {
	hash := sha256.New()
	var length [8]byte
	for _, frame := range frames {
		binary.BigEndian.PutUint64(length[:], uint64(len(frame)))
		_, _ = hash.Write(length[:])
		_, _ = hash.Write(frame)
	}
	return hex.EncodeToString(hash.Sum(nil))
}

func approvalLess(left, right NativeApprovalKey) bool {
	if left.Shell != right.Shell {
		return left.Shell < right.Shell
	}
	if left.EntryID != right.EntryID {
		return left.EntryID < right.EntryID
	}
	return left.ImplementationSHA256 < right.ImplementationSHA256
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
