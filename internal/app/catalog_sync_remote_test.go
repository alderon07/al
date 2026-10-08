//go:build !windows

package app

import (
	"alias-lens/internal/managedgit"
	"context"

	"encoding/base64"
	"encoding/json"

	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"

	shellapi "alias-lens/internal/shell"
)

func TestRemotePullEditPushUncertainReconciliation(t *testing.T) {
	actualGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	capturedPATH := os.Getenv("PATH")
	for _, outcome := range []string{"advanced", "unchanged"} {
		t.Run(outcome, func(t *testing.T) {
			repository, value := setupCatalogSyncFixture(t)
			service := DefaultServices()
			push := func() error {
				preview, err := service.prepareCatalogPush()
				if err != nil {
					return err
				}
				return service.ApplyCatalogPush(preview)
			}
			record, _, err := service.readCatalogSyncRecord()
			if err != nil {
				t.Fatal(err)
			}
			branch := strings.TrimSpace(runGit(t, repository, "symbolic-ref", "--short", "HEAD"))
			oldHEAD := record.HEAD
			remoteURL := "https://github.com/synthetic/repo.git"
			runGit(t, repository, "remote", "add", "origin", remoteURL)
			bare := filepath.Join(service.homeDirectory(), "remote.git")
			runGit(t, repository, "clone", "--bare", repository, bare)
			runGit(t, bare, "config", "uploadpack.allowFilter", "true")
			record.Provider = "github"
			record.RemoteURL = remoteURL
			record.Remote = "origin"
			record.RemoteRef = "refs/heads/" + branch
			record.Transport = "https"
			encoded, err := catalogstore.Encode(record)
			if err != nil {
				t.Fatal(err)
			}
			os.WriteFile(service.catalogSyncPath(), encoded, 0600)
			value.Entries[0].Category = "remote semantic edit"
			remoteBytes, problems := catalog.Encode(value)
			if len(problems) > 0 {
				t.Fatal(problems)
			}
			runner, cleanup, err := managedgit.New()
			if err != nil {
				t.Fatal(err)
			}
			remoteRecord := record
			remoteRecord.Repository = bare
			remoteCommit, _, err := createCatalogPathCommit(context.Background(), runner, CatalogPushPreview{Record: remoteRecord, HEAD: oldHEAD, Local: remoteBytes, Date: "2026-10-03T12:00:00Z"})
			cleanup()
			if err != nil {
				t.Fatal(err)
			}
			runGit(t, bare, "update-ref", record.RemoteRef, remoteCommit, oldHEAD)
			t.Setenv("GH_TOKEN", "synthetic-credential")
			original := service.dependencies.Transport
			t.Cleanup(func() { service.dependencies.Transport = original })
			service.dependencies.Transport = catalogProviderRoundTrip(func(r *http.Request) (*http.Response, error) {
				if r.URL.Host != "api.github.com" || r.Header.Get("Authorization") != "Bearer synthetic-credential" {
					t.Fatal("credential authority mismatch")
				}
				current := strings.TrimSpace(runGit(t, bare, "rev-parse", record.RemoteRef))
				var response any
				switch {
				case strings.Contains(r.URL.Path, "/compare/"):
					comparison := strings.Split(strings.Split(r.URL.Path, "/compare/")[1], "...")
					response = map[string]any{"status": "ahead", "base_commit": map[string]string{"sha": comparison[0]}, "merge_base_commit": map[string]string{"sha": comparison[0]}}
				case strings.Contains(r.URL.Path, "/contents/"):
					commit := r.URL.Query().Get("ref")
					data := []byte(runGit(t, bare, "show", commit+":catalog.json"))
					response = map[string]any{"type": "file", "path": "catalog.json", "sha": gitBlobSHA(data), "size": len(data), "encoding": "base64", "content": base64.StdEncoding.EncodeToString(data)}
				case strings.Contains(r.URL.Path, "/commits/"):
					response = map[string]string{"sha": current}
				default:
					response = map[string]any{"default_branch": branch, "permissions": map[string]bool{"push": true}}
				}
				data, _ := json.Marshal(response)
				return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(string(data))), Header: http.Header{}}, nil
			})
			preview, err := service.buildCatalogSyncPullPlan()
			if err != nil {
				t.Fatal(err)
			}
			if err := service.applyMutationPlan(preview, service.BuildCatalogSyncPullPlan); err != nil {
				t.Fatal(err)
			}
			if strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD")) != oldHEAD {
				t.Fatal("pull changed physical local branch")
			}
			combined, err := readCatalogFile(service.localCatalogPath())
			if err != nil {
				t.Fatal(err)
			}
			combined.Entries[0].Description = "local semantic edit"
			writeCatalogFixture(t, service.localCatalogPath(), combined)
			os.WriteFile(filepath.Join(repository, "unrelated"), []byte("staged"), 0600)
			runGit(t, repository, "add", "unrelated")
			os.WriteFile(filepath.Join(repository, "unrelated"), []byte("dirty"), 0600)
			beforeIndex := runGit(t, repository, "ls-files", "--stage")
			runGit(t, repository, "tag", "-am", "synthetic private tag message", "synthetic-private")
			runGit(t, repository, "config", "push.followTags", "true")
			signing := "true"
			if outcome == "unchanged" {
				signing = "if-asked"
			}
			runGit(t, repository, "config", "push.gpgSign", signing)
			runGit(t, repository, "config", "user.signingKey", "synthetic-key")
			runGit(t, bare, "config", "receive.certNonceSeed", "synthetic-seed")
			signerMarker := filepath.Join(service.homeDirectory(), "signer-called")
			signer := filepath.Join(service.homeDirectory(), "synthetic-signer")
			if err := os.WriteFile(signer, []byte("#!/bin/sh\nprintf called > "+shellapi.Quote(signerMarker)+"\nexit 1\n"), 0700); err != nil {
				t.Fatal(err)
			}
			runGit(t, repository, "config", "gpg.program", signer)
			bin := filepath.Join(service.homeDirectory(), "bin")
			os.Mkdir(bin, 0700)
			attempted := filepath.Join(service.homeDirectory(), "attempted")
			failure := "exit 1"
			if outcome == "advanced" {
				failure = shellapi.Quote(actualGit) + " \"$@\"; exit 1"
			}
			script := "#!/bin/sh\noperation=\nfor arg do case \"$arg\" in fetch|push) operation=$arg ;; esac; done\nfor arg do shift; case \"$arg\" in " + shellapi.Quote(remoteURL) + ") set -- \"$@\" " + shellapi.Quote("file://"+bare) + " ;; fetch|push) set -- \"$@\" -c protocol.file.allow=always \"$arg\" ;; *) set -- \"$@\" \"$arg\" ;; esac; done\ncase \"$operation\" in\nfetch) " + shellapi.Quote(actualGit) + " \"$@\" || exit $?\n " + shellapi.Quote(actualGit) + " -C " + shellapi.Quote(repository) + " config --unset-all " + shellapi.Quote("remote.file://"+bare+".promisor") + "\n " + shellapi.Quote(actualGit) + " -C " + shellapi.Quote(repository) + " config --unset-all " + shellapi.Quote("remote.file://"+bare+".partialclonefilter") + "\n " + shellapi.Quote(actualGit) + " -C " + shellapi.Quote(repository) + " config " + shellapi.Quote("remote."+remoteURL+".promisor") + " true\n " + shellapi.Quote(actualGit) + " -C " + shellapi.Quote(repository) + " config " + shellapi.Quote("remote."+remoteURL+".partialclonefilter") + " blob:none\n exit 0 ;;\npush) printf '%s\\n' \"$@\" > " + shellapi.Quote(filepath.Join(service.homeDirectory(), "push-args")) + "; if [ ! -f " + shellapi.Quote(attempted) + " ]; then printf attempted > " + shellapi.Quote(attempted) + "; " + failure + "; fi\n exec " + shellapi.Quote(actualGit) + " \"$@\" ;;\n*) exec " + shellapi.Quote(actualGit) + " \"$@\" ;;\nesac\n"
			script = strings.Replace(script, "#!/bin/sh\n", "#!/bin/sh\nexec 2>"+shellapi.Quote(filepath.Join(service.homeDirectory(), "git-debug-stderr"))+"\n", 1)
			os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0700)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+capturedPATH)
			if err := push(); err == nil || !strings.Contains(err.Error(), "uncertain") {
				debug, _ := os.ReadFile(filepath.Join(service.homeDirectory(), "git-debug-stderr"))
				t.Fatalf("uncertain push did not preserve intent: %v %s", err, debug)
			}
			pending, _, err := service.readCatalogSyncRecord()
			if err != nil || pending.PushIntent == nil {
				t.Fatal("missing durable intent", err)
			}
			source := pending.PushIntent.SourceCommit
			if err := push(); err != nil {
				t.Fatal("uncertain intent reconciliation", err)
			}
			if strings.TrimSpace(runGit(t, bare, "for-each-ref", "refs/tags")) != "" {
				t.Fatal("catalog push published an unrelated tag")
			}
			if _, err := os.Stat(signerMarker); !os.IsNotExist(err) {
				t.Fatal("catalog push executed a configured signer")
			}
			pushArgs, err := os.ReadFile(filepath.Join(service.homeDirectory(), "push-args"))
			if err != nil || !strings.Contains(string(pushArgs), "--no-follow-tags\n") || !strings.Contains(string(pushArgs), "--signed=false\n") || !strings.Contains(string(pushArgs), "--no-replace-objects\n") {
				t.Fatal("catalog transport lost explicit push controls", err)
			}
			complete, _, err := service.readCatalogSyncRecord()
			if err != nil || complete.PushIntent != nil || complete.HEAD != source {
				t.Fatal("intent was not reconciled", err)
			}
			if beforeIndex != runGit(t, repository, "ls-files", "--stage") || strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD")) != oldHEAD {
				t.Fatal("unmanaged branch/index changed")
			}
			dirty, _ := os.ReadFile(filepath.Join(repository, "unrelated"))
			if string(dirty) != "dirty" {
				t.Fatal("unrelated worktree changed")
			}
			pushed, problems := catalog.Decode([]byte(runGit(t, bare, "show", source+":catalog.json")))
			if len(problems) > 0 || pushed.Entries[0].Category != "remote semantic edit" || pushed.Entries[0].Description != "local semantic edit" {
				t.Fatal("semantic pull/edit/push lost edits")
			}
		})
	}
}
