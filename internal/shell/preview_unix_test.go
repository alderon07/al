//go:build !windows

package shell

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestPreviewDiagnosticReasonsAndShadowContract(t *testing.T) {
	cases := []struct{ name, source, code, message string }{
		{"double quoting", "alias a=\"echo $HOME\"\n", "alias_quoting", "expansion timing"},
		{"unquoted", "alias a=echo\n", "alias_quoting", "single-quoted"},
		{"name", "alias ..='echo test'\n", "alias_name", "ASCII letter"},
		{"missing assignment", "alias a\n", "alias_assignment", "assignment"},
		{"trailing command", "alias a='echo test'; echo trailing\n", "alias_trailing", "Additional text"},
		{"attached comment", "alias a='echo test'#comment\n", "alias_trailing", "separated comment"},
		{"metadata", "# al: favorite=false\nalias a='echo test'\n", "unsupported_metadata", "Metadata"},
		{"function keyword tab", "function\tfoo { echo synthetic; }\n", "function_header", "header is outside"},
		{"top level array", "items=()\n", "top_level_syntax", "shell configuration"},
		{"top level array append", "items+=()\n", "top_level_syntax", "shell configuration"},
		{"top level array comment", "items=() # helper()\n", "top_level_syntax", "shell configuration"},
		{"top level export comment", "export EXAMPLE=synthetic # ()\n", "top_level_syntax", "shell configuration"},
		{"top level printf comment", "printf synthetic # helper()\n", "top_level_syntax", "shell configuration"},
		{"top level quoted comment", "printf '# helper()'\n", "top_level_syntax", "shell configuration"},
		{"top level escaped comment", "printf \\# helper()\n", "top_level_syntax", "shell configuration"},
		{"function keyword parentheses", "function foo() { echo synthetic; }\n", "function_header", "header is outside"},
		{"function invalid name", "bad-name() { echo synthetic; }\n", "function_name", "function name"},
		{"function keyword invalid name", "function bad-name { echo synthetic; }\n", "function_name", "function name"},
		{"function brace", "a()\n{ echo test; }\n", "function_opening_brace", "first line"},
		{"substitution", "a() {\n echo $(pwd)\n}\nalias b='echo test'\n", "function_command_substitution", "Scanner stopped at line 2"},
		{"parameter expansion", "a() { echo ${HOME}; }\n", "function_parameter_expansion", "parameter expansion"},
		{"arithmetic expression", "a() { echo $((value + 1)); }\n", "function_arithmetic_expression", "nonnumeric"},
		{"backticks", "a() { echo `pwd`; }\n", "function_backticks", "Backtick"},
		{"grouping", "a() { (echo test); }\n", "function_grouping", "parentheses"},
		{"nested braces", "a() { { echo test; }; }\n", "function_grouping", "Nested braces"},
		{"process substitution", "a() { cat <(echo test); }\n", "function_grouping", "process substitution"},
		{"here string", "a() { cat <<< test; }\n", "function_here_string", "Here-strings"},
		{"continuation", "a() { echo \\\n test; }\n", "function_continuation", "line continuation"},
		{"function tail", "a() { echo test; }; echo trailing\n", "function_trailing", "closing brace"},
		{"function boundary", "a() { echo test\n", "function_boundary", "closing boundary"},
		{"top level", "export EXAMPLE=synthetic\n", "top_level_syntax", "shell configuration"},
		{"top level boundary", "echo \"synthetic\n", "top_level_boundary", "no boundary"},
	}
	for _, name := range []string{"bash", "zsh"} {
		for _, test := range cases {
			t.Run(name+"/"+test.name, func(t *testing.T) {
				source := []byte(test.source)
				original := ImportShadowSource(name, source)
				results := ImportShadowSource(name, source)
				ExplainPreviewDiagnostics(source, results)
				if len(results) != 1 || len(results[0].Diagnostics) != 1 {
					t.Fatalf("unexpected results: %+v", results)
				}
				diagnostic := results[0].Diagnostics[0]
				if diagnostic.Code != test.code || !strings.Contains(diagnostic.Message, test.message) || !strings.Contains(diagnostic.Message, "Next:") {
					t.Fatalf("unexpected detail: %+v", diagnostic)
				}
				if original[0].Diagnostics[0].Code == "ambiguous_definition" && !strings.Contains(diagnostic.Message, "remaining lines are grouped") {
					t.Fatalf("missing range explanation: %+v", diagnostic)
				}
				results[0].Diagnostics = original[0].Diagnostics
				if !reflect.DeepEqual(results, original) {
					t.Fatal("preview changed parser acceptance, ranges or stable identity")
				}
			})
		}
	}
}

func TestPreviewRespectsQuotedAndCommentBoundaries(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		for _, source := range []string{
			"alias a='echo test' # ${HOME} $(pwd)\n",
			"a() { echo '${HOME} $(pwd)'; }\n",
			"a() {\n # ${HOME} $(pwd)\n echo test\n}\n",
			"a() { echo $((1 + 2)); }\n",
			"a() {\n cat <<'END'\n ${HOME} $(pwd)\nEND\n}\n",
		} {
			results := ImportShadowSource(name, []byte(source))
			ExplainPreviewDiagnostics([]byte(source), results)
			if len(results) != 1 || results[0].Status != "equivalent" || len(results[0].Diagnostics) != 0 {
				t.Fatalf("quoted or commented syntax rejected: %+v", results)
			}
		}
	}
}

func TestPreviewDiagnosticsContainNoSourceText(t *testing.T) {
	source := []byte("# Synthetic private description\nalias a=\"SYNTHETIC_PRIVATE_MARKER\\033[31m\"\na() { echo $(SYNTHETIC_PRIVATE_MARKER); }\n")
	results := ImportShadowSource("bash", source)
	ExplainPreviewDiagnostics(source, results)
	encoded, err := json.Marshal(results)
	if err != nil {
		t.Fatal(err)
	}
	for _, private := range []string{"SYNTHETIC_PRIVATE_MARKER", "Synthetic private description", "\\033", "\x1b"} {
		if strings.Contains(string(encoded), private) {
			t.Fatalf("preview revealed source text %q", private)
		}
	}
}

func TestPreviewReservedMarkerDoesNotInspectFollowingRange(t *testing.T) {
	for _, name := range []string{"bash", "zsh"} {
		for _, following := range []string{"alias a='echo synthetic'\n", "a() { echo $(pwd); }\n"} {
			source := []byte("# al-shadow-origin: synthetic\n" + following)
			original := ImportShadowSource(name, source)
			results := ImportShadowSource(name, source)
			ExplainPreviewDiagnostics(source, results)
			if len(results) != 2 || len(results[0].Diagnostics) != 1 {
				t.Fatalf("unexpected results: %+v", results)
			}
			diagnostic := results[0].Diagnostics[0]
			if diagnostic.Code != "reserved_origin_marker" || !strings.Contains(diagnostic.Message, "reserved internal origin marker") || !strings.Contains(diagnostic.Message, "Next:") {
				t.Fatalf("unexpected marker diagnostic: %+v", diagnostic)
			}
			if strings.Contains(diagnostic.Message, "Scanner stopped") || strings.Contains(diagnostic.Message, "Command substitution") {
				t.Fatalf("marker borrowed detail from following range: %+v", diagnostic)
			}
			for index := range results {
				results[index].Diagnostics = original[index].Diagnostics
			}
			if !reflect.DeepEqual(results, original) {
				t.Fatal("preview changed parser membership or source ranges")
			}
		}
	}
}
