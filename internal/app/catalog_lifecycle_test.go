//go:build !windows

package app

import (
	"bytes"
	"fmt"

	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	workflowplan "github.com/alderon07/al/internal/plan"
	workflowstate "github.com/alderon07/al/internal/state"
)

func TestCatalogStructuralBoundaryNeverExecutes(t *testing.T) {
	sentinel := filepath.Join(t.TempDir(), "executed")
	for _, body := range []string{"\n}; printf danger > " + sentinel + "\nfunction escaped {\n", "\n:;\n} > " + sentinel + "\nfunction escaped {\n"} {
		entry := neutralcatalog.Entry{ID: strings.Repeat("1", 32), Name: "safe", Kind: "function", Native: map[string]neutralcatalog.NativeImplementation{"bash": {FunctionBody: &body}}}
		declaration, err := catalogstore.Declaration(entry, "bash", "")
		if err != nil {
			t.Fatal(err)
		}
		if DefaultServices().validateCatalogDeclaration("bash", entry, []byte(declaration)) == nil {
			t.Fatal("escaped function accepted")
		}
	}
	if _, err := os.Stat(sentinel); !os.IsNotExist(err) {
		t.Fatal("structural inspection executed user code")
	}
	for _, name := range []string{"al", "alias-lens", "_alias_lens_control", "eval", "readonly", "if"} {
		if !catalogProtectedName(name) {
			t.Fatalf("protected name %s accepted", name)
		}
	}
}

func TestCatalogExecutableResolutionRejectsRelativePATH(t *testing.T) {
	for _, path := range []string{"", ":/bin", "/bin:", "relative:/bin"} {
		if _, err := resolveCatalogExecutable("printf", path); err == nil {
			t.Fatalf("unsafe PATH %q accepted", path)
		}
	}
	resolved, err := resolveCatalogExecutable("printf", "/usr/bin:/bin")
	if err != nil || !filepath.IsAbs(resolved) {
		t.Fatalf("pin=%q, %v", resolved, err)
	}
}

func TestCatalogStartupGrammarPreservesGuardAndRejectsAmbiguity(t *testing.T) {
	home := t.TempDir()
	native := filepath.Join(home, ".bash_aliases")
	guard := []byte("if [ -f \"$HOME/.bash_aliases\" ]; then\n . \"$HOME/.bash_aliases\"\nfi\n")
	preserved, load, err := catalogStartupPlacement(guard, "bash", home, native, nil)
	if err != nil || load || !bytes.Equal(preserved, guard) {
		t.Fatalf("static guard=%q load=%v err=%v", preserved, load, err)
	}
	for _, text := range []string{string(guard) + string(guard), "source \"$native\"\n", "return\n", "if true; then\n . \"$HOME/.bash_aliases\"\nfi\n"} {
		if _, _, err := catalogStartupPlacement([]byte(text), "bash", home, native, nil); err == nil {
			t.Fatalf("ambiguous route accepted: %q", text)
		}
	}
}

func TestCatalogEnableRepeatEditAndOfflineRollback(t *testing.T) {
	priorValidator := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = priorValidator })
	home := t.TempDir()
	if err := os.Chmod(home, 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("1", 32), Name: "pinned", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"installed"}, PassArguments: true}}}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	nativePath := filepath.Join(home, ".bash_aliases")
	native := []byte("alias nativeonly='printf native'\n")
	if err := os.WriteFile(nativePath, native, 0600); err != nil {
		t.Fatal(err)
	}
	decisions := CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T00:00:00Z", Executable: "/bin/false"}

	preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}

	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	repeated, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if len(repeated.Actions) != 0 {
		for _, action := range repeated.Actions {
			t.Log(action.TargetRole, action.DisplayPath)
		}
		t.Fatal("repeat enable changed state")
	}
	declaration, handled, err := DefaultServices().catalogInstalledDeclaration("pinned", "bash")
	if err != nil || !handled || !strings.Contains(declaration, "'/usr/bin/printf'") {
		t.Fatalf("installed declaration=%q %v %v", declaration, handled, err)
	}
	if err := DefaultServices().editCatalogAlias("pinned", "pinned", "printf changed", "changed", EntryMetadata{}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := DefaultServices().catalogInstalledDeclaration("pinned", "bash"); err == nil {
		t.Fatal("pending candidate ran prior installed body")
	}
	if err := os.Remove(DefaultServices().localCatalogPath()); err != nil {
		t.Fatal(err)
	}
	rollback, err := DefaultServices().buildCatalogRollbackPlan("bash")
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(rollback, func() (workflowplan.OperationPlan, error) { return DefaultServices().buildCatalogRollbackPlan("bash") }); err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(nativePath)
	if !bytes.Equal(after, native) {
		t.Fatal("offline rollback changed native baseline")
	}
}

func TestCatalogLoaderGuardedHandoffPTY(t *testing.T) {
	bash, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	body := "  printf generated\\n"
	entry := neutralcatalog.Entry{ID: strings.Repeat("1", 32), Name: "collision", Kind: "function", Native: map[string]neutralcatalog.NativeImplementation{"bash": {FunctionBody: &body}}}
	declaration, err := catalogstore.Declaration(entry, "bash", "")
	if err != nil {
		t.Fatal(err)
	}
	session := startShellPTY(t, bash, []string{"--noprofile", "--norc", "-i"}, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "PS1=" + ptyPrompt})
	session.run("alias collision='printf fallback'")
	command := "if eval " + quoteShadow(declaration) + "; then builtin unalias -- collision; fi"
	output := session.run(command)
	if strings.Contains(output, "syntax error") {
		t.Fatal(output)
	}
	output = session.run("collision; printf '\\n'")
	if !strings.Contains(output, "generated") {
		t.Fatal(output)
	}
	session.run("readonly -f collision")
	output = session.run("if eval " + quoteShadow(strings.ReplaceAll(declaration, "generated", "unsafe")) + "; then builtin unalias -- collision; fi")
	if !strings.Contains(output, "readonly") {
		t.Fatal("readonly function was replaced")
	}
}

func TestCatalogActualStartupPTY(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "alias-lens")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/alias-lens")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	for _, route := range []string{".bash_login", ".profile"} {
		t.Run("login"+route, func(t *testing.T) {
			executable, err := exec.LookPath("bash")
			if err != nil {
				t.Fatal(err)
			}
			home := privateTestHome(t)
			t.Setenv("HOME", home)
			t.Setenv("ALIAS_LENS_SHELL", "bash")
			t.Setenv("PATH", "/usr/bin:/bin")
			prior := shadowValidatorPath
			shadowValidatorPath = func(string) (string, error) { return executable, nil }
			t.Cleanup(func() { shadowValidatorPath = prior })
			if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".bashrc"), []byte("PS1="+quoteShadow(ptyPrompt)+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, route), []byte("source \"$HOME/.bashrc\"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), nil, 0600); err != nil {
				t.Fatal(err)
			}
			value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("5", 32), Name: "loginready", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"loginready"}}}}}
			writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
			decisions := CatalogLifecycleDecisions{Executable: binary, ConfirmedAt: DefaultServices().lifecycleTimestamp()}
			preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
			if err != nil {
				t.Fatal(err)
			}
			if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
				return DefaultServices().buildCatalogEnablePlan("bash", decisions)
			}); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(home, ".bash_profile")); !os.IsNotExist(err) {
				t.Fatal("created higher priority startup file")
			}
			session := startShellPTY(t, executable, []string{"--login", "-i"}, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "HISTFILE=/dev/null", "PS1=" + ptyPrompt, "ALIAS_LENS_SHELL=bash"})
			output := session.run("al --version; loginready; printf '\\n'")
			if !strings.Contains(output, "alias-lens") || !strings.Contains(output, "loginready") {
				t.Fatal(output)
			}
		})
	}
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			executable, err := exec.LookPath(shell)
			if err != nil {
				t.Fatal(err)
			}
			home := t.TempDir()
			if err := os.Chmod(home, 0700); err != nil {
				t.Fatal(err)
			}
			t.Setenv("HOME", home)
			t.Setenv("ALIAS_LENS_SHELL", shell)
			t.Setenv("ZDOTDIR", "")
			priorValidator := shadowValidatorPath
			shadowValidatorPath = func(string) (string, error) { return executable, nil }
			t.Cleanup(func() { shadowValidatorPath = priorValidator })
			if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
				t.Fatal(err)
			}
			startupName := ".bashrc"
			if shell == "zsh" {
				startupName = ".zshrc"
			}
			if err := os.WriteFile(filepath.Join(home, startupName), []byte("PS1="+quoteShadow(ptyPrompt)+"\n"), 0600); err != nil {
				t.Fatal(err)
			}
			entry := neutralcatalog.Entry{ID: strings.Repeat("2", 32), Name: "collision", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"generated"}, PassArguments: true}}
			value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{entry}}
			writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
			adapter, _ := DefaultServices().shellAdapter(shell)
			nativePath := filepath.Join(home, adapter.AliasFilename())
			native := []byte("alias collision='printf fallback'\nalias al='printf legacy-al-mask'\n")
			if err := os.WriteFile(nativePath, native, 0600); err != nil {
				t.Fatal(err)
			}
			ownership, err := DefaultServices().catalogAdoptionForEntry(entry, shell, nativePath, native)
			if err != nil {
				t.Fatal(err)
			}
			decisions := CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T00:00:00Z", Executable: binary, Adoptions: []catalogstore.Adoption{ownership}}
			preview, err := DefaultServices().buildCatalogEnablePlan(shell, decisions)
			if err != nil {
				t.Fatal(err)
			}
			if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
				return DefaultServices().buildCatalogEnablePlan(shell, decisions)
			}); err != nil {
				t.Fatal(err)
			}
			arguments := []string{"--noprofile", "-i"}
			if shell == "zsh" {
				arguments = []string{"-d", "-i"}
			}
			environment := []string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "PS1=" + ptyPrompt, "HISTFILE=/dev/null", "ALIAS_LENS_SHELL=" + shell}
			if shell == "zsh" {
				environment = append(environment, "ZDOTDIR="+home)
			}
			start := func() *shellPTY {
				session := startShellPTY(t, executable, arguments, environment)

				return session
			}
			session := start()
			output := session.run("al --version")
			if !strings.Contains(output, "alias-lens") {
				t.Fatal(output)
			}
			output = session.run("collision; printf '\\n'")
			if !strings.Contains(output, "generated") {
				t.Fatal(output)
			}
			useCollision := func() string {
				mark := session.mark()
				session.write("al use collision; printf 'handoff-status:%s\\n' $?\n")
				session.waitFor(mark, "Choose an alias to use.")
				session.write("\r")
				return session.waitFor(mark, ptyPrompt)
			}
			session.run("alias collision='printf old-mask'")
			output = useCollision()
			if !strings.Contains(output, "generated") || strings.Contains(output, "old-mask") {
				t.Fatal(output)
			}
			if shell == "bash" {
				session.run("readonly -f collision; alias collision='printf readonly-mask'")
				output = useCollision()
				if !strings.Contains(output, "handoff-status:1") {
					t.Fatal("readonly replacement did not decline")
				}
				output = session.run("collision; printf '\\n'")
				if !strings.Contains(output, "readonly-mask") {
					t.Fatal("readonly conflict removed alias mask")
				}
			}
			pointer := filepath.Join(DefaultServices().catalogGeneratedRoot(shell), "active")
			before, err := os.ReadFile(pointer)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(pointer, []byte("invalid\n"), 0600); err != nil {
				t.Fatal(err)
			}
			session = start()
			output = session.run("collision; printf '\\n'")
			if !strings.Contains(output, "fallback") {
				t.Fatal(output)
			}
			if err := os.WriteFile(pointer, before, 0600); err != nil {
				t.Fatal(err)
			}
			id, _ := catalogstore.DecodePointer(before)
			bodyPath := filepath.Join(DefaultServices().catalogGeneratedRoot(shell), id+".sh")
			body, _ := os.ReadFile(bodyPath)
			if err := os.WriteFile(bodyPath, append(body, []byte("printf forbidden\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			session = start()
			output = session.run("collision; printf '\\n'")
			if !strings.Contains(output, "fallback") || strings.Contains(output, "forbidden") {
				t.Fatal(output)
			}
			if err := os.WriteFile(bodyPath, body, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(nativePath, append(native, []byte("alias untouched='printf drift'\n")...), 0600); err != nil {
				t.Fatal(err)
			}
			session = start()
			output = session.run("collision; printf '\\n'")
			if !strings.Contains(output, "fallback") {
				t.Fatal(output)
			}
		})
	}
}

func TestCatalogRollbackPreservesOriginalAbsenceAndInvalidPointer(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	oldValidator := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = oldValidator })
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("3", 32), Name: "portable", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", PassArguments: true}}}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	decisions := CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T00:00:00Z", Executable: "/bin/false"}
	plan, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(plan, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(DefaultServices().catalogGeneratedRoot("bash"), "active"), []byte("corrupt\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(DefaultServices().localCatalogPath()); err != nil {
		t.Fatal(err)
	}
	rollback, err := DefaultServices().buildCatalogRollbackPlan("bash")
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(rollback, func() (workflowplan.OperationPlan, error) { return DefaultServices().buildCatalogRollbackPlan("bash") }); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{".bash_aliases", ".bashrc", ".bash_profile"} {
		if _, err := os.Lstat(filepath.Join(home, name)); !os.IsNotExist(err) {
			t.Fatalf("originally absent %s was retained", name)
		}
	}
}

func TestCatalogEditorPreservesIdentityFieldsAndInstalledMembership(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	oldValidator := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = oldValidator })
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	entry := neutralcatalog.Entry{ID: strings.Repeat("4", 32), Name: "installed", Kind: "command", When: &neutralcatalog.Conditions{ProfilesNone: []string{"paused"}}, Portable: &neutralcatalog.Portable{Program: "printf", PassArguments: true}}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{entry}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	native := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(native, []byte("alias nativeonly='printf native'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	decisions := CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T00:00:00Z", Executable: "/bin/false"}
	plan, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(plan, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	originalNative, _ := os.ReadFile(native)
	if err := DefaultServices().editAliasWithMetadata("installed", "renamed", "printf edited", "edited", EntryMetadata{Favorite: true}); err != nil {
		t.Fatal(err)
	}
	changed, err := readCatalogFile(DefaultServices().localCatalogPath())
	if err != nil {
		t.Fatal(err)
	}
	if changed.Entries[0].ID != entry.ID || changed.Entries[0].When == nil || changed.Entries[0].Portable == nil || !changed.Entries[0].Favorite {
		t.Fatal("typed edit lost stable identity or unrelated fields")
	}
	afterNative, _ := os.ReadFile(native)
	if !bytes.Equal(afterNative, originalNative) {
		t.Fatal("catalog editing changed native input")
	}
	if _, _, err := DefaultServices().catalogInstalledDeclaration("renamed", "bash"); err == nil {
		t.Fatal("new label silently ran old installed body")
	}
	names, handled, err := DefaultServices().catalogInstalledCompletionNames("bash")
	if err != nil || !handled || strings.Join(names, ",") != "installed,nativeonly" {
		t.Fatalf("installed completion membership=%v handled=%v err=%v", names, handled, err)
	}
	revisions, err := DefaultServices().listRevisions(DefaultServices().localCatalogPath())
	if err != nil || len(revisions) != 1 {
		t.Fatalf("catalog revisions=%v err=%v", revisions, err)
	}
}

func TestCatalogProtectedNamesBeforeEligibility(t *testing.T) {
	for _, name := range []string{"al", "return", "_alias_lens_mask", "bad-name"} {
		value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("9", 32), Name: name, Kind: "command", Portable: &neutralcatalog.Portable{Program: "nonexistent-program"}}}}
		if err := DefaultServices().validateCatalogNames(value); err == nil {
			t.Fatalf("accepted %s", name)
		}
	}
}

func TestCatalogRenewedOwnershipAfterNativeDrift(t *testing.T) {
	prior := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = prior })
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("8", 32), Name: "renewed", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"installed"}}}}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	nativePath := filepath.Join(home, ".bash_aliases")
	original := []byte("alias renewed='printf original'\n")
	if err := os.WriteFile(nativePath, original, 0600); err != nil {
		t.Fatal(err)
	}
	adoption, err := DefaultServices().catalogAdoptionForEntry(value.Entries[0], "bash", nativePath, original)
	if err != nil {
		t.Fatal(err)
	}
	decisions := CatalogLifecycleDecisions{Adoptions: []catalogstore.Adoption{adoption}, Executable: "/bin/false", ConfirmedAt: DefaultServices().lifecycleTimestamp()}
	apply := func(decisions CatalogLifecycleDecisions) {
		t.Helper()
		preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
		if err != nil {
			t.Fatal(err)
		}
		if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
			return DefaultServices().buildCatalogEnablePlan("bash", decisions)
		}); err != nil {
			t.Fatal(err)
		}
	}
	apply(decisions)
	changed := []byte("alias renewed='printf intentional'\nalias unrelated='printf unrelated'\n")
	if err := os.WriteFile(nativePath, changed, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().buildCatalogEnablePlan("bash", CatalogLifecycleDecisions{Executable: "/bin/false"}); err == nil {
		t.Fatal("unreviewed drift accepted")
	}
	value.Entries[0].Name = "renewedname"
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	renewed, err := DefaultServices().catalogAdoptionForEntry(value.Entries[0], "bash", nativePath, changed)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.Name != "renewed" || renewed.Kind != "command" {
		t.Fatal("renewal lost exact enrolled source identity")
	}
	decisions.Adoptions = []catalogstore.Adoption{renewed}
	apply(decisions)
	if _, err := DefaultServices().readCatalogGeneration(DefaultServices().catalogGeneratedRoot("bash"), nativePath, "bash"); err != nil {
		t.Fatal(err)
	}
	refreshedNative, _ := os.ReadFile(nativePath)
	if !bytes.Contains(refreshedNative, []byte("function renewedname {")) || !bytes.Contains(refreshedNative, []byte("alias unrelated=")) || bytes.Contains(refreshedNative, []byte("alias renewed=")) {
		t.Fatal("owned fallback rename did not preserve unrelated declaration")
	}
	record := catalogstore.RollbackRecord{}
	installed := catalogstore.InstalledFile{Version: 1, Records: []catalogstore.InstalledState{}}
	if _, err := readLifecycleFile(DefaultServices().catalogInstalledPath(), &installed); err != nil {
		t.Fatal(err)
	}
	if _, err := readLifecycleFile(filepath.Join(DefaultServices().catalogStateRoot(), "rollback", installed.Records[0].RollbackID+".json"), &record); err != nil {
		t.Fatal(err)
	}
	baseline, err := catalogstore.ReadPrivateBytes(record.OriginalPath, catalogstore.MaxDocumentBytes)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(baseline, original) {
		t.Fatal("renewal replaced original rollback baseline")
	}
}

func TestCatalogStartupInsertionFollowsNativeSource(t *testing.T) {
	native := filepath.Join("/home/example", ".bash_aliases")
	source := []byte("source \"$HOME/.bash_aliases\"\nprintf after-source\n")
	preserved, load, position, err := catalogStartupPlacementAt(source, "bash", "/home/example", native, map[string]bool{})
	if err != nil || load || !bytes.Equal(preserved, source) || position != len("source \"$HOME/.bash_aliases\"\n") {
		t.Fatalf("placement load=%v position=%d err=%v", load, position, err)
	}
}

func TestCatalogRuntimeHandoffMasksReadonlyHelperPTY(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		t.Run(shell, func(t *testing.T) {
			executable, err := exec.LookPath(shell)
			if err != nil {
				t.Skip(err)
			}
			args := []string{"--noprofile", "--norc", "-i"}
			if shell == "zsh" {
				args = []string{"-d", "-f", "-i"}
			}
			home := privateTestHome(t)
			session := startShellPTY(t, executable, args, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "PS1=" + ptyPrompt})
			output := session.run("builtin eval " + quoteShadow(catalogRuntimeHandoff(shell, nil)) + "; printf 'empty-ok\\n'")
			if !strings.Contains(output, "empty-ok") || strings.Contains(output, "parse error") || strings.Contains(output, "syntax error") {
				t.Fatal(output)
			}
			if shell == "bash" {
				session.run("function _alias_lens_catalog_handoff { printf stale-helper; }; readonly -f _alias_lens_catalog_handoff")
				output = session.run("builtin eval " + quoteShadow(catalogRuntimeHandoff(shell, nil)))
				if strings.Contains(output, "stale-helper") {
					t.Fatal("stale readonly helper executed")
				}
			}
			body := "  printf replacement\\n"
			entry := neutralcatalog.Entry{ID: strings.Repeat("7", 32), Name: "masked", Kind: "function", Native: map[string]neutralcatalog.NativeImplementation{shell: {FunctionBody: &body}}}
			declaration, err := catalogstore.Declaration(entry, shell, "")
			if err != nil {
				t.Fatal(err)
			}
			if shell == "bash" {
				session.run("builtin unset -f _alias_lens_catalog_handoff 2>/dev/null")
				return
			}
			session.run("alias masked='printf fallback'; alias _alias_lens_catalog_handoff='printf stale-alias'")
			output = session.run("builtin eval " + quoteShadow(catalogRuntimeHandoff(shell, []catalogstore.GenerationEntry{{Entry: entry, Declaration: declaration}})))
			if strings.Contains(output, "stale-alias") || strings.Contains(output, "parse error") {
				t.Fatal(output)
			}
			output = session.run("masked; printf '\\n'")
			if !strings.Contains(output, "replacement") {
				t.Fatal(output)
			}
		})
	}
}

func TestCatalogStatusTracksProfileResolutionAndInstalledMembership(t *testing.T) {
	prior := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = prior })
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	config := DefaultServices().defaultConfig()
	if err := DefaultServices().saveConfig(config); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("6", 32), Name: "profiled", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"profiled"}}, When: &neutralcatalog.Conditions{ProfilesAny: []string{"work"}}}}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	decisions := CatalogLifecycleDecisions{Executable: "/bin/false", ConfirmedAt: DefaultServices().lifecycleTimestamp()}
	apply := func() {
		t.Helper()
		preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
		if err != nil {
			t.Fatal(err)
		}
		if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
			return DefaultServices().buildCatalogEnablePlan("bash", decisions)
		}); err != nil {
			t.Fatal(err)
		}
	}
	apply()
	before := workflowstate.Inputs{}
	DefaultServices().observeLifecycleStatus(&before)
	if len(before.Shells) != 1 || before.Shells[0].Resolved.SHA256 != before.Shells[0].Installed.ResolvedStateSHA256 {
		t.Fatal("new installed resolution was not current")
	}
	config.Profiles = []string{"work"}
	if err := DefaultServices().saveConfig(config); err != nil {
		t.Fatal(err)
	}
	changed := workflowstate.Inputs{}
	DefaultServices().observeLifecycleStatus(&changed)
	if changed.Shells[0].Resolved.SHA256 == changed.Shells[0].Installed.ResolvedStateSHA256 {
		t.Fatal("profile activation did not require rendering")
	}
	candidates, handled, err := DefaultServices().catalogInstalledCompletionNames("bash")
	if err != nil || !handled || len(candidates) != 0 {
		t.Fatalf("pending profile leaked installed completion %v %v %v", candidates, handled, err)
	}
	apply()
	after := workflowstate.Inputs{}
	DefaultServices().observeLifecycleStatus(&after)
	if after.Shells[0].Resolved.SHA256 != after.Shells[0].Installed.ResolvedStateSHA256 {
		t.Fatal("enable did not refresh resolved status")
	}
	candidates, _, err = DefaultServices().catalogInstalledCompletionNames("bash")
	if err != nil || len(candidates) != 1 || candidates[0] != "profiled" {
		t.Fatalf("installed completion %v %v", candidates, err)
	}
}

func TestCatalogZshStartupRouteProofBounded(t *testing.T) {
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ZDOTDIR", "")
	path := filepath.Join(home, ".zshenv")
	for _, source := range []string{"if false; then\nZDOTDIR='/other'\nfi\n", "return\nZDOTDIR='/other'\n", strings.Repeat(" ", int(ShadowSourceLimit)+1)} {
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := catalogZshStartupPath(home); err == nil {
			t.Fatal("unproven or oversized route accepted")
		}
	}
	target := filepath.Join(home, "custom")
	if err := os.WriteFile(path, []byte("ZDOTDIR="+quoteShadow(target)+"\n"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := catalogZshStartupPath(home)
	if err != nil || got != filepath.Join(target, ".zshrc") {
		t.Fatalf("static route %s %v", got, err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ZDOTDIR", target)
	got, err = catalogZshStartupPath(home)
	if err != nil || got != filepath.Join(target, ".zshrc") {
		t.Fatalf("inherited route %s %v", got, err)
	}
}

func TestCatalogEnrollmentPreservesUserLeafSymlinks(t *testing.T) {
	prior := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = prior })
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	root := filepath.Join(home, "dotfiles")
	if err := os.Mkdir(root, 0700); err != nil {
		t.Fatal(err)
	}
	native := filepath.Join(root, "aliases")
	startup := filepath.Join(root, "startup")
	if err := os.WriteFile(native, []byte("alias fallback='printf fallback'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(startup, []byte("source \"$HOME/.bash_aliases\"\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(native, filepath.Join(home, ".bash_aliases")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(startup, filepath.Join(home, ".bashrc")); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("4", 32), Name: "pinnedlink", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"link"}}}}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	decisions := CatalogLifecycleDecisions{Executable: "/bin/false", ConfirmedAt: DefaultServices().lifecycleTimestamp()}
	preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{filepath.Join(home, ".bash_aliases"), filepath.Join(home, ".bashrc")} {
		info, err := os.Lstat(path)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("leaf link lost %s %v", path, err)
		}
	}
	manifest, err := DefaultServices().readCatalogGeneration(DefaultServices().catalogGeneratedRoot("bash"), filepath.Join(home, ".bash_aliases"), "bash")
	if err != nil || manifest.NativePath != native {
		t.Fatalf("native referent %s %v", manifest.NativePath, err)
	}
	preview, err = DefaultServices().buildCatalogRollbackPlan("bash")
	if err != nil {
		t.Fatal(err)
	}
	replacement := filepath.Join(root, "replacement")
	if err := os.WriteFile(replacement, []byte("alias fallback='printf fallback'\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(home, ".bash_aliases")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(replacement, filepath.Join(home, ".bash_aliases")); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().readCatalogGeneration(DefaultServices().catalogGeneratedRoot("bash"), native, "bash"); err == nil {
		t.Fatal("runtime accepted retargeted source while original referent remained unchanged")
	}
	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) { return DefaultServices().buildCatalogRollbackPlan("bash") }); err == nil {
		t.Fatal("retargeted reviewed link accepted")
	}
}

func TestCatalogGuardedLegacyIntegrationReadonlyAlPTY(t *testing.T) {
	executable, err := exec.LookPath("bash")
	if err != nil {
		t.Fatal(err)
	}
	home := privateTestHome(t)
	session := startShellPTY(t, executable, []string{"--noprofile", "--norc", "-i"}, []string{"HOME=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "PS1=" + ptyPrompt})
	session.run("function al { printf readonly-function; }; readonly -f al; alias al='printf preserved-mask'")
	adapter := DefaultServices().mustShellAdapter("bash")
	definition, err := legacyShellEntryHandoff(adapter, Alias{Name: "al", Type: "function", Command: "printf replacement"})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []struct{ name, contents string }{{"integration", adapter.Integration()}, {"shell-entry", definition}} {
		t.Run(input.name, func(t *testing.T) {
			path := filepath.Join(home, input.name+".bash")
			if err := os.WriteFile(path, []byte(input.contents), 0600); err != nil {
				t.Fatal(err)
			}
			output := session.run("builtin source " + quoteShadow(path))
			if strings.Contains(output, "syntax error") || strings.Contains(output, "replacement") {
				t.Fatal(output)
			}
			output = session.run(`al; builtin printf '\n'`)
			if !strings.Contains(output, "preserved-mask") {
				t.Fatal("readonly " + input.name + " removed alias before function replacement")
			}
			output = session.run(`\al; builtin printf '\n'`)
			if !strings.Contains(output, "readonly-function") || strings.Contains(output, "replacement") {
				t.Fatal("readonly " + input.name + " replaced or executed the function: " + output)
			}
			output = session.run("builtin readonly -f")
			if !strings.Contains(output, "declare -fr al") && !strings.Contains(output, "declare -rf al") {
				t.Fatal("readonly " + input.name + " lost the function attribute: " + output)
			}
		})
	}
}

func TestCatalogInitStatusUsesEnrollmentWithoutLegacyRepository(t *testing.T) {
	for _, different := range []bool{false, true} {
		t.Run(fmt.Sprintf("different-%v", different), func(t *testing.T) {
			pinInitTestValidator(t)
			repo, _ := setupCatalogSyncFixture(t)
			if err := os.Remove(DefaultServices().localCatalogPath()); err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(DefaultServices().catalogSyncPath()); err != nil {
				t.Fatal(err)
			}
			config := DefaultServices().defaultConfig()
			if different {
				legacy := filepath.Join(DefaultServices().homeDirectory(), "legacy")
				if err := os.Mkdir(legacy, 0700); err != nil {
					t.Fatal(err)
				}
				config.Repository = legacy
			}
			if err := DefaultServices().saveConfig(config); err != nil {
				t.Fatal(err)
			}
			options := CatalogInitOptions{Source: repo, Shell: "bash", CatalogPath: "catalog.json"}
			decisions := CatalogLifecycleDecisions{ConfirmedAt: DefaultServices().lifecycleTimestamp(), Executable: "/bin/false"}
			preview, err := DefaultServices().buildLocalCatalogInitPlan(options, decisions)
			if err != nil {
				t.Fatal(err)
			}
			if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
				return DefaultServices().buildLocalCatalogInitPlan(options, decisions)
			}); err != nil {
				t.Fatal(err)
			}
			report := DefaultServices().inspectWorkflowStatus()
			if report.Sync.State != workflowstate.SyncClean {
				t.Fatalf("enrolled status=%s", report.Sync.State)
			}
			record, _, err := DefaultServices().readCatalogSyncRecord()
			if err != nil {
				t.Fatal(err)
			}
			observed := workflowstate.Inputs{}
			DefaultServices().observeCatalogSyncStatus(&observed)
			if observed.Sync.BaseSHA256 != record.BaseSHA256 || observed.Sync.LocalSHA256 != record.BaseSHA256 || observed.Sync.RemoteSHA256 != record.BaseSHA256 {
				t.Fatal("status used a legacy repository or base")
			}
		})
	}
}

func TestCatalogInstalledDiffUsesExactIncludedMembership(t *testing.T) {
	prior := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = prior })
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".bash_aliases"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	body := "printf pending"
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{
		{ID: strings.Repeat("a", 32), Name: "included", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"included"}}},
		{ID: strings.Repeat("b", 32), Name: "pendingnative", Kind: "command", Native: map[string]neutralcatalog.NativeImplementation{"bash": {AliasValue: &body}}},
	}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	decisions := CatalogLifecycleDecisions{Executable: "/bin/false", ConfirmedAt: DefaultServices().lifecycleTimestamp()}
	preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	snapshot, handled, err := DefaultServices().readLifecycleInstalledSnapshot("bash")
	if err != nil || !handled || len(snapshot.Entries) != 1 || snapshot.Entries[0].Name != "included" {
		t.Fatalf("installed membership %v %v", handled, err)
	}
	report := neutralcatalog.SemanticDiff(snapshot, value, "installed", "bash")
	if report.Summary.Added != 1 || report.Summary.Changed != 0 || report.Summary.Deleted != 0 {
		t.Fatalf("installed diff %#v", report.Summary)
	}
	completions, handled, err := DefaultServices().catalogInstalledCompletionNames("bash")
	if err != nil || !handled || len(completions) != 1 || completions[0] != "included" {
		t.Fatalf("completions %v %v", completions, err)
	}
	entries, err := DefaultServices().loadAliases()
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, entry := range entries {
		if entry.Name == "pendingnative" {
			found = true
			if entry.CatalogState != "pending" || catalogEntryRunnable(entry) {
				t.Fatal("omitted native candidate marked installed")
			}
		}
	}
	if !found {
		t.Fatal("pending candidate missing from facade")
	}
}

func TestCatalogSurvivingControlMasksBlockBeforeWritesStartupPTY(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "alias-lens")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/alias-lens")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build %v %s", err, output)
	}
	for _, shell := range []string{"bash", "zsh"} {
		for _, definition := range []string{"alias builtin='printf CONTROL_SENTINEL'\n", "alias command='printf CONTROL_SENTINEL'\n", "builtin() { printf CONTROL_SENTINEL; }\n", "command() { printf CONTROL_SENTINEL; }\n",
			"alias demo='printf fallback'; alias builtin='printf CONTROL_SENTINEL'\n",
			"alias demo='printf fallback' builtin='printf CONTROL_SENTINEL'\n",
			"alias demo='printf fallback' \\\n builtin='printf CONTROL_SENTINEL'\n",
			"alias demo='printf fallback'; \\\n alias command='printf CONTROL_SENTINEL'\n"} {
			t.Run(shell+strings.Fields(definition)[0]+strings.Fields(definition)[1], func(t *testing.T) {
				executable, err := exec.LookPath(shell)
				if err != nil {
					t.Fatal(err)
				}
				home := privateTestHome(t)
				t.Setenv("HOME", home)
				t.Setenv("ALIAS_LENS_SHELL", shell)
				t.Setenv("ZDOTDIR", "")
				if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
					t.Fatal(err)
				}
				adapter, _ := DefaultServices().shellAdapter(shell)
				nativePath := filepath.Join(home, adapter.AliasFilename())
				native := []byte(definition + "alias fallbackentry='printf retained-fallback'\n")
				if err := os.WriteFile(nativePath, native, 0600); err != nil {
					t.Fatal(err)
				}
				startupName := ".bashrc"
				if shell == "zsh" {
					startupName = ".zshrc"
				}
				startup := []byte("PS1=" + quoteShadow(ptyPrompt) + "\nsource " + quoteShadow(nativePath) + "\n")
				if err := os.WriteFile(filepath.Join(home, startupName), startup, 0600); err != nil {
					t.Fatal(err)
				}
				value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{{ID: strings.Repeat("c", 32), Name: "candidate", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"candidate"}}}}}
				writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
				command := exec.Command(binary, "catalog", "enable", "--shell", shell, "--apply")
				command.Env = os.Environ()
				output, err := command.CombinedOutput()
				if err == nil || !bytes.Contains(output, []byte("masks catalog handoff control")) || bytes.Contains(output, []byte("CONTROL_SENTINEL")) {
					t.Fatalf("unsafe control validation %v %s", err, output)
				}
				for _, path := range []string{DefaultServices().catalogInstalledPath(), DefaultServices().catalogApprovalsPath(), filepath.Join(DefaultServices().catalogGeneratedRoot(shell), "active")} {
					if _, err := os.Stat(path); !os.IsNotExist(err) {
						t.Fatal("blocked application wrote managed state")
					}
				}
				current, _ := os.ReadFile(nativePath)
				if !bytes.Equal(current, native) {
					t.Fatal("blocked application changed fallback")
				}
				current, _ = os.ReadFile(filepath.Join(home, startupName))
				if !bytes.Equal(current, startup) {
					t.Fatal("blocked application changed startup")
				}
				args := []string{"--noprofile", "-i"}
				if shell == "zsh" {
					args = []string{"-d", "-i"}
				}
				session := startShellPTY(t, executable, args, []string{"HOME=" + home, "ZDOTDIR=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "PS1=" + ptyPrompt, "HISTFILE=/dev/null"})
				result := session.run("fallbackentry; printf '\\n'")
				if !strings.Contains(result, "retained-fallback") || strings.Contains(session.output.stringFrom(0), "CONTROL_SENTINEL") {
					t.Fatal("blocked catalog did not retain fallback without invoking masked control")
				}
			})
		}
	}
}

func TestCatalogPortableTemplateChecksUnquotedAliasDependencies(t *testing.T) {
	value := neutralcatalog.Entry{ID: strings.Repeat("d", 32), Name: "portable", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"printf"}}}
	declaration, err := catalogstore.Declaration(value, "bash", "/usr/bin/printf")
	if err != nil {
		t.Fatal(err)
	}
	prior := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = prior })
	if err := DefaultServices().validateCatalogDeclarations("bash", []neutralcatalog.Entry{value}, [][]byte{[]byte(declaration)}, []byte("alias printf='printf harmless'\n")); err == nil {
		t.Fatal("native control alias accepted")
	}
	if err := DefaultServices().validateCatalogDeclarations("bash", []neutralcatalog.Entry{value}, [][]byte{[]byte(declaration)}, []byte("alias portable='printf fallback'\n")); err != nil {
		t.Fatal("intentional entry-name alias incorrectly blocked", err)
	}
}

func TestCatalogExistingSetupUpgradeStartupAndOfflineRollbackPTY(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "alias-lens")
	build := exec.Command("go", "build", "-o", binary, "../../cmd/alias-lens")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v %s", err, output)
	}
	for _, shell := range []string{"bash", "zsh"} {
		for formIndex := 0; formIndex < 2; formIndex++ {
			t.Run(fmt.Sprintf("%s-%d", shell, formIndex), func(t *testing.T) {
				shellPath, err := exec.LookPath(shell)
				if err != nil {
					t.Fatal(err)
				}
				home := privateTestHome(t)
				t.Setenv("HOME", home)
				t.Setenv("ALIAS_LENS_SHELL", shell)
				t.Setenv("ZDOTDIR", "")
				prior := shadowValidatorPath
				shadowValidatorPath = func(string) (string, error) { return shellPath, nil }
				t.Cleanup(func() { shadowValidatorPath = prior })
				if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
					t.Fatal(err)
				}
				adapter, _ := DefaultServices().shellAdapter(shell)
				nativePath := filepath.Join(home, adapter.AliasFilename())
				original := []byte(adapter.KnownIntegrationForms()[formIndex] + "\nalias collision='printf fallback'\n")
				if err := os.WriteFile(nativePath, original, 0600); err != nil {
					t.Fatal(err)
				}
				startupName, args := ".bashrc", []string{"--noprofile", "-i"}
				if shell == "zsh" {
					startupName, args = ".zshrc", []string{"-d", "-i"}
				}
				startupPath := filepath.Join(home, startupName)
				startupOriginal := []byte("source " + quoteShadow(nativePath) + "\nPS1=" + quoteShadow(ptyPrompt) + "\n")
				if err := os.WriteFile(startupPath, startupOriginal, 0600); err != nil {
					t.Fatal(err)
				}
				entry := neutralcatalog.Entry{ID: strings.Repeat("d", 32), Name: "collision", Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"upgraded"}, PassArguments: true}}
				writeCatalogFixture(t, DefaultServices().localCatalogPath(), neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{entry}})
				ownership, err := DefaultServices().catalogAdoptionForEntry(entry, shell, nativePath, original)
				if err != nil {
					t.Fatal(err)
				}
				decisions := CatalogLifecycleDecisions{ConfirmedAt: "2026-10-03T00:00:00Z", Executable: binary, Adoptions: []catalogstore.Adoption{ownership}}
				preview, err := DefaultServices().buildCatalogEnablePlan(shell, decisions)
				if err != nil {
					t.Fatal(err)
				}
				if text := workflowplan.RenderPlain(preview); !strings.Contains(text, "Retain enrolled fallback") || !strings.Contains(text, "Relocate exact owned native integration") {
					t.Fatal("upgrade plan omitted fallback and integration effects")
				}
				if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
					return DefaultServices().buildCatalogEnablePlan(shell, decisions)
				}); err != nil {
					t.Fatal(err)
				}
				_, err = DefaultServices().buildCatalogEnablePlan(shell, CatalogLifecycleDecisions{Executable: binary})
				if err != nil {
					t.Fatalf("repeat upgrade: %v", err)
				}
				native, _ := os.ReadFile(nativePath)
				startup, _ := os.ReadFile(startupPath)
				if bytes.Contains(native, []byte("Alias Lens")) || bytes.Contains(startup, []byte("unalias al 2>/dev/null || true")) {
					t.Fatal("legacy integration survived upgrade")
				}
				session := startShellPTY(t, shellPath, args, []string{"HOME=" + home, "ZDOTDIR=" + home, "PATH=/usr/bin:/bin", "TERM=xterm-256color", "PS1=" + ptyPrompt, "HISTFILE=/dev/null"})
				if output := session.run("al --version"); !strings.Contains(output, "alias-lens ") {
					t.Fatalf("integration did not dispatch: %s", output)
				}
				if output := session.run("collision"); !strings.Contains(output, "upgraded") {
					t.Fatalf("overlay unavailable: %s", output)
				}
				session.write("exit\n")
				if err := os.Remove(DefaultServices().localCatalogPath()); err != nil {
					t.Fatal(err)
				}
				rollback, err := DefaultServices().buildCatalogRollbackPlan(shell)
				if err != nil {
					t.Fatal(err)
				}
				if !strings.Contains(workflowplan.RenderPlain(rollback), catalogRollbackBaselineWarning) {
					t.Fatal("rollback plan omitted baseline warning")
				}
				if err := DefaultServices().applyMutationPlan(rollback, func() (workflowplan.OperationPlan, error) { return DefaultServices().buildCatalogRollbackPlan(shell) }); err != nil {
					t.Fatal(err)
				}
				native, _ = os.ReadFile(nativePath)
				startup, _ = os.ReadFile(startupPath)
				if !bytes.Equal(native, original) || !bytes.Equal(startup, startupOriginal) {
					t.Fatal("rollback did not preserve exact existing setup")
				}
			})
		}
	}
}

func TestCatalogKnownNativeIntegrationRequiresProvenBoundary(t *testing.T) {
	for _, shell := range []string{"bash", "zsh"} {
		adapter, _ := DefaultServices().shellAdapter(shell)
		for _, form := range adapter.KnownIntegrationForms() {
			for _, source := range []string{"alias trapped='\n" + form + "'\n", "if false; then\n" + form + "fi\n", form + form, strings.Replace(form, "Alias Lens", "Modified Lens", 1)} {
				if validateCatalogNativeControls(shell, []byte(source)) == nil {
					t.Fatal("ambiguous integration accepted")
				}
			}
		}
	}
}

func TestCatalogFallbackMutationPlanExplainsNames(t *testing.T) {
	prior := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = prior })
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	t.Setenv("PATH", "/usr/bin:/bin")
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	value := neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{}}
	native := []byte{}
	for index, name := range []string{"before", "deleted", "excluded", "refreshed"} {
		value.Entries = append(value.Entries, neutralcatalog.Entry{ID: strings.Repeat(fmt.Sprintf("%x", index+1), 32), Name: name, Kind: "command", Portable: &neutralcatalog.Portable{Program: "printf", Args: []string{"installed"}}})
		native = append(native, []byte("alias "+name+"='printf original'\n")...)
	}
	nativePath := filepath.Join(home, ".bash_aliases")
	if err := os.WriteFile(nativePath, native, 0600); err != nil {
		t.Fatal(err)
	}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	decisions := CatalogLifecycleDecisions{Executable: "/bin/false", ConfirmedAt: DefaultServices().lifecycleTimestamp()}
	for _, entry := range value.Entries {
		record, err := DefaultServices().catalogAdoptionForEntry(entry, "bash", nativePath, native)
		if err != nil {
			t.Fatal(err)
		}
		decisions.Adoptions = append(decisions.Adoptions, record)
	}
	preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	value.Entries[0].Name = "after"
	value.Entries[2].When = &neutralcatalog.Conditions{ProfilesAny: []string{"inactive"}}
	value.Entries = append(value.Entries[:1], value.Entries[2:]...)
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), value)
	preview, err = DefaultServices().buildCatalogEnablePlan("bash", CatalogLifecycleDecisions{Executable: "/bin/false"})
	if err != nil {
		t.Fatal(err)
	}
	text := workflowplan.RenderPlain(preview)
	for _, expected := range []string{`Rename enrolled fallback "before" to "after"`, `Remove enrolled fallback "deleted" for deleted catalog entry`, `Remove enrolled fallback "excluded" excluded by shell/profile/platform conditions`, `Refresh enrolled fallback "refreshed"`} {
		if !strings.Contains(text, expected) {
			t.Fatalf("missing plan effect %s", expected)
		}
	}
	if !strings.Contains(CatalogRollbackConfirmation, "enrollment baseline") || !strings.Contains(CatalogRollbackConfirmation, "renamed or deleted since enrollment can return") {
		t.Fatal("confirmation omitted rollback caveat")
	}
	for _, action := range preview.Actions {
		if action.Reason == "Install reviewed catalog state" {
			t.Fatal("generic lifecycle reason survived")
		}
	}
}

func TestCatalogInaccessibleExecutableOmittedAndReplacementBlocked(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root execute permissions")
	}
	prior := shadowValidatorPath
	shadowValidatorPath = func(shell string) (string, error) { return exec.LookPath(shell) }
	t.Cleanup(func() { shadowValidatorPath = prior })
	home := privateTestHome(t)
	t.Setenv("HOME", home)
	t.Setenv("ALIAS_LENS_SHELL", "bash")
	bin := filepath.Join(home, "bin")
	if err := os.Mkdir(bin, 0700); err != nil {
		t.Fatal(err)
	}
	program := filepath.Join(bin, "owned-program")
	if err := os.WriteFile(program, []byte("#!/bin/sh\nprintf unused\n"), 0640); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(program, 0650); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+":/usr/bin:/bin")
	if _, err := resolveCatalogExecutable("owned-program", os.Getenv("PATH")); err == nil {
		t.Fatal("owner cannot execute group-only candidate")
	}
	if err := DefaultServices().saveConfig(DefaultServices().defaultConfig()); err != nil {
		t.Fatal(err)
	}
	entry := neutralcatalog.Entry{ID: strings.Repeat("e", 32), Name: "inaccessible", Kind: "command", Portable: &neutralcatalog.Portable{Program: "owned-program", Args: []string{}}}
	writeCatalogFixture(t, DefaultServices().localCatalogPath(), neutralcatalog.Catalog{SchemaVersion: 2, Entries: []neutralcatalog.Entry{entry}})
	decisions := CatalogLifecycleDecisions{Executable: "/bin/false", ConfirmedAt: DefaultServices().lifecycleTimestamp()}
	preview, err := DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	manifest, err := DefaultServices().readCatalogGeneration(DefaultServices().catalogGeneratedRoot("bash"), filepath.Join(home, ".bash_aliases"), "bash")
	if err != nil || len(manifest.IncludedEntryIDs) != 0 {
		t.Fatalf("inaccessible new program installed: %v", err)
	}
	if err := os.Chmod(program, 0700); err != nil {
		t.Fatal(err)
	}
	preview, err = DefaultServices().buildCatalogEnablePlan("bash", decisions)
	if err != nil {
		t.Fatal(err)
	}
	if err := DefaultServices().applyMutationPlan(preview, func() (workflowplan.OperationPlan, error) {
		return DefaultServices().buildCatalogEnablePlan("bash", decisions)
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(program, 0650); err != nil {
		t.Fatal(err)
	}
	if _, err := DefaultServices().buildCatalogEnablePlan("bash", decisions); err == nil {
		t.Fatal("inaccessible installed replacement did not block")
	}
}

func TestCatalogIntentionalDisableIncludesNativeFunctions(t *testing.T) {
	body := "\n:\n"
	entry := neutralcatalog.Entry{ID: strings.Repeat("f", 32), Name: "disabled_function", Kind: "function", Native: map[string]neutralcatalog.NativeImplementation{"bash": {FunctionBody: &body}}, When: &neutralcatalog.Conditions{Shells: []string{"zsh"}}}
	if !DefaultServices().catalogIntentionallyDisabled(entry, "bash", "linux", nil) {
		t.Fatal("intentional function exclusion was treated as unavailable replacement")
	}
}
