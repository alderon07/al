//go:build !windows

package main

import (
	"alias-lens/internal/catalog"
	"alias-lens/internal/catalogstore"
	"alias-lens/internal/managedgit"
	workflowplan "alias-lens/internal/plan"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestManagedFilterProof(t *testing.T) {
	for _, test := range []struct {
		trace string
		want  bool
	}{{"packet: clone< fetch=shallow wait-for-done filter\npacket: clone> filter blob:none\n", true}, {"packet: clone< fetch=shallow\npacket: clone> filter blob:none\n", false}, {"warning: filtering ignored\nremote.origin.partialclonefilter=blob:none\n", false}, {"packet: clone< filter\n", false}, {"packet: clone< nofilter\npacket: clone> filter blob:none\n", false}} {
		if managedFilterProof([]byte(test.trace)) != test.want {
			t.Fatal("incorrect capability proof")
		}
	}
}
func TestCatalogStageIntentRecoveryAndReplacementRefusal(t *testing.T) {
	setupCatalogSyncFixture(t)
	preview := remoteCatalogPreview{Provider: "github", Host: "github.com", Repository: "synthetic/repo", Revision: strings.Repeat("a", 40), Blob: strings.Repeat("b", 40)}
	if err := withMutation(func(session *mutationSession) error {
		intent, err := beginManagedCatalogStage(session, preview)
		if err != nil {
			return err
		}
		if _, err := os.Stat(managedStageIntentPath(intent.ID)); err != nil {
			t.Fatal("intent not durable")
		}
		old := intent.Root + "-old"
		os.Rename(intent.Root, old)
		os.Mkdir(intent.Root, 0700)
		if cleanupManagedCatalogStage(intent) == nil {
			t.Fatal("replacement stage deleted")
		}
		os.Remove(intent.Root)
		os.Rename(old, intent.Root)
		return recoverManagedCatalogStages(session)
	}); err != nil {
		t.Fatal(err)
	}
}
func TestCatalogPathCommitPreservesUnrelatedIndexAndWorktree(t *testing.T) {
	repo, _ := setupCatalogSyncFixture(t)
	other := filepath.Join(repo, "unrelated")
	os.WriteFile(other, []byte("committed"), 0600)
	runGit(t, repo, "add", "unrelated")
	runGit(t, repo, "commit", "-qm", "unrelated committed")
	os.WriteFile(other, []byte("staged"), 0600)
	runGit(t, repo, "add", "unrelated")
	os.WriteFile(other, []byte("dirty"), 0600)
	beforeIndex := runGit(t, repo, "ls-files", "--stage")
	beforeWork, _ := os.ReadFile(other)
	head := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	record, _, err := readCatalogSyncRecord()
	if err != nil {
		t.Fatal(err)
	}
	local, _ := os.ReadFile(localCatalogPath())
	preview := catalogPushPreview{Record: record, Local: local, HEAD: head, Date: "2026-10-03T12:00:00Z"}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	commit, _, err := createCatalogPathCommit(context.Background(), runner, preview)
	if err != nil {
		t.Fatal(err)
	}
	if commit == "" {
		t.Fatal("empty commit")
	}
	afterWork, _ := os.ReadFile(other)
	if string(beforeWork) != string(afterWork) || beforeIndex != runGit(t, repo, "ls-files", "--stage") {
		t.Fatal("unrelated paths/index modified")
	}
	changed := strings.TrimSpace(runGit(t, repo, "diff-tree", "--no-commit-id", "--name-only", "-r", head, commit))
	if changed != "" && changed != record.CatalogPath {
		t.Fatal("commit changed unrelated path")
	}
	if err := scanCatalogOutgoingHistory(context.Background(), runner, record, commit); err != nil {
		t.Fatal(err)
	}
}
func TestCatalogSyncCodecStrictRecord(t *testing.T) {
	repo, _ := setupCatalogSyncFixture(t)
	record, _, err := readCatalogSyncRecord()
	if err != nil {
		t.Fatal(err)
	}
	record.Repository = repo
	encoded, err := catalogstore.Encode(record)
	if err != nil {
		t.Fatal(err)
	}
	var decoded catalogstore.CatalogSyncRecord
	if err := catalogstore.Decode(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	record.CatalogPath = "../outside"
	if catalogstore.Validate(record) == nil {
		t.Fatal("unsafe enrollment path")
	}
}

func TestManagedIndexPreservesHEADMetadataWithoutCheckout(t *testing.T) {
	repository, _ := setupCatalogSyncFixture(t)
	os.WriteFile(filepath.Join(repository, "unrelated"), []byte("synthetic"), 0600)
	runGit(t, repository, "add", "unrelated")
	runGit(t, repository, "commit", "-qm", "synthetic unrelated")
	os.Remove(filepath.Join(repository, "unrelated"))
	runner, cleanup, err := managedgit.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if err := initializeManagedCatalogIndex(context.Background(), runner, repository, "catalog.json"); err != nil {
		t.Fatal(err)
	}
	flags := runGit(t, repository, "ls-files", "-v")
	if !strings.Contains(flags, "S unrelated\n") || !strings.Contains(flags, "H catalog.json\n") {
		t.Fatal("incorrect managed index membership")
	}
	if runGit(t, repository, "diff", "--cached", "--name-only") != "" {
		t.Fatal("managed index staged unrelated deletions")
	}
	if _, err := os.Stat(filepath.Join(repository, "unrelated")); !os.IsNotExist(err) {
		t.Fatal("unrelated path materialized")
	}
}

func TestCatalogPathCommitUsesPulledRemoteBase(t *testing.T) {
	repository, value := setupCatalogSyncFixture(t)
	oldHEAD := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	value.Entries[0].Description = "remote semantic edit"
	writeCatalogFixture(t, filepath.Join(repository, "catalog.json"), value)
	runGit(t, repository, "add", "catalog.json")
	runGit(t, repository, "commit", "-qm", "remote synthetic")
	remoteHEAD := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	record, _, err := readCatalogSyncRecord()
	if err != nil {
		t.Fatal(err)
	}
	record.HEAD = remoteHEAD
	record.RemoteURL = "https://github.com/synthetic/repo.git"
	runGit(t, repository, "reset", "--hard", oldHEAD)
	value.Entries[0].Category = "local semantic edit"
	local, problems := catalog.Encode(value)
	if len(problems) > 0 {
		t.Fatal(problems)
	}
	beforeIndex := runGit(t, repository, "ls-files", "--stage")
	runner, cleanup, err := managedgit.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	commit, _, err := createCatalogPathCommit(context.Background(), runner, catalogPushPreview{Record: record, HEAD: oldHEAD, Local: local, Date: "2026-10-03T12:00:00Z"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(runGit(t, repository, "rev-parse", commit+"^")) != remoteHEAD {
		t.Fatal("path commit did not use pulled remote parent")
	}
	if beforeIndex != runGit(t, repository, "ls-files", "--stage") || strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD")) != oldHEAD {
		t.Fatal("unmanaged branch/index modified")
	}
}

func TestRemoteStageActualFilteredGitAndPromotion(t *testing.T) {
	actualGit, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	capturedPATH := os.Getenv("PATH")
	for _, name := range []string{"supported", "ignored", "stale-revision", "stale-blob", "cancel", "existing", "byte-limit", "deadline-limit", "packet-limit"} {
		filter := name != "ignored"
		t.Run(name, func(t *testing.T) {
			repository, _ := setupCatalogSyncFixture(t)
			catalogBytes, _ := os.ReadFile(filepath.Join(repository, "catalog.json"))
			os.WriteFile(filepath.Join(repository, ".gitattributes"), []byte("* filter=sentinel\n*.json filter=lfs\n"), 0600)
			os.WriteFile(filepath.Join(repository, "unrelated"), []byte("unrelated synthetic blob"), 0600)
			runGit(t, repository, "add", ".gitattributes", "unrelated")
			os.WriteFile(filepath.Join(repository, ".gitmodules"), []byte("[submodule \"synthetic\"]\n path = synthetic-module\n url = ext::synthetic-danger\n"), 0600)
			runGit(t, repository, "add", ".gitmodules")
			runGit(t, repository, "update-index", "--add", "--cacheinfo", "160000,"+strings.Repeat("c", 40)+",synthetic-module")
			runGit(t, repository, "commit", "-qm", "synthetic malicious attributes")
			bare := filepath.Join(homeDirectory(), "remote.git")
			runGit(t, repository, "clone", "--bare", repository, bare)
			runGit(t, bare, "config", "uploadpack.allowFilter", strings.ToLower(strconv.FormatBool(filter)))
			maliciousSentinel := filepath.Join(homeDirectory(), "hook-filter-executed")
			os.WriteFile(filepath.Join(bare, "hooks", "post-checkout"), []byte("#!/bin/sh\nprintf danger > "+quoteShadow(maliciousSentinel)+"\n"), 0700)
			runGit(t, bare, "config", "filter.sentinel.smudge", "printf danger > "+quoteShadow(maliciousSentinel))
			runGit(t, bare, "config", "filter.lfs.process", "printf danger > "+quoteShadow(maliciousSentinel))
			revision := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
			branch := strings.TrimSpace(runGit(t, repository, "symbolic-ref", "--short", "HEAD"))
			preview := remoteCatalogPreview{Provider: "github", Host: "github.com", Repository: "synthetic/repository", CloneURL: "https://github.com/synthetic/repository.git", Revision: revision, Blob: gitBlobSHA(catalogBytes), CatalogPath: "catalog.json", Branch: branch, Bytes: catalogBytes}
			if name == "stale-revision" {
				preview.Revision = strings.Repeat("a", 40)
			}
			if name == "stale-blob" {
				preview.Blob = strings.Repeat("b", 40)
			}
			bin := filepath.Join(homeDirectory(), "bin")
			os.Mkdir(bin, 0700)
			wrapper := "#!/bin/sh\nclone=false\ntemplate=\nfor arg do\n case \"$arg\" in clone) clone=true ;; --template=*) template=$arg ;; esac\n destination=$arg\ndone\nif [ \"$clone\" = true ]; then\n " + quoteShadow(actualGit) + " -c protocol.file.allow=always clone --no-checkout --depth=1 --filter=blob:none --single-branch --branch " + quoteShadow(branch) + " \"$template\" -- " + quoteShadow("file://"+bare) + " \"$destination\" || exit $?\n " + quoteShadow(actualGit) + " -C \"$destination\" config remote.origin.url " + quoteShadow(preview.CloneURL) + "\nelse\n exec " + quoteShadow(actualGit) + " \"$@\"\nfi\n"
			sentinel := filepath.Join(homeDirectory(), "child-executed")
			if name == "cancel" || name == "deadline-limit" {
				wrapper = strings.Replace(wrapper, "if [ \"$clone\" = true ]; then\n", "if [ \"$clone\" = true ]; then\n (sleep 1; printf danger > "+quoteShadow(sentinel)+") &\n wait\n exit 0\n", 1)
			}
			wrapper = strings.Replace(wrapper, " || exit $?", " 2>"+quoteShadow(filepath.Join(homeDirectory(), "git-test-stderr"))+" || exit $?", 1)
			os.WriteFile(filepath.Join(bin, "git"), []byte(wrapper), 0700)
			t.Setenv("PATH", bin+string(os.PathListSeparator)+capturedPATH)
			t.Setenv("GH_TOKEN", "synthetic-credential")
			if name == "existing" {
				os.MkdirAll(managedCatalogDestination(preview), 0700)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if name == "cancel" {
				ctx, cancel = context.WithTimeout(ctx, 150*time.Millisecond)
				defer cancel()
			}
			err := withMutation(func(session *mutationSession) error {
				limits := managedStageLimits{5 * time.Minute, managedCloneTreeLimit, catalogstore.MaxDocumentBytes}
				if name == "byte-limit" {
					limits.TreeBytes = 1024
				}
				if name == "packet-limit" {
					limits.PacketBytes = 16
				}
				if name == "deadline-limit" {
					limits.Deadline = 100 * time.Millisecond
				}
				intent, stage, e := stageRemoteCatalogWithLimits(ctx, session, preview, limits)
				if name != "supported" {
					if e == nil {
						t.Fatal("ignored filtering accepted")
					}
					if intent.ID != "" {
						return session.removePrivate(managedStageIntentPath(intent.ID))
					}
					return nil
				}
				if e != nil {
					return fmt.Errorf("stage: %w", e)
				}
				if filepath.Dir(intent.Root) != filepath.Dir(intent.Destination) {
					t.Fatal("stage uses different destination filesystem parent")
				}
				for _, path := range []string{"unrelated", ".gitattributes", ".gitmodules", "synthetic-module"} {
					if _, e := os.Stat(filepath.Join(stage, path)); !os.IsNotExist(e) {
						t.Fatal("unapproved blob materialized")
					}
				}
				flags := runGit(t, stage, "ls-files", "-v")
				if !strings.Contains(flags, "S unrelated") {
					t.Fatal("unrelated metadata missing")
				}
				action, e := managedPromotionAction(stage, intent.Destination)
				if e != nil {
					return fmt.Errorf("promotion inspect: %w", e)
				}
				action.Sequence = 1
				plan := workflowplan.Build("init.stage.test", nil, []workflowplan.Action{action}, nil, nil)
				if e := session.ApplyPlan(plan, func() (workflowplan.OperationPlan, error) { return plan, nil }); e != nil {
					return fmt.Errorf("promotion apply: %w", e)
				}
				data, e := os.ReadFile(filepath.Join(intent.Destination, "catalog.json"))
				if e != nil || string(data) != string(catalogBytes) {
					t.Fatal("promotion changed catalog")
				}
				if e := cleanupManagedCatalogStage(intent); e != nil {
					return e
				}
				return session.removePrivate(managedStageIntentPath(intent.ID))
			})
			if err != nil {
				debug, _ := os.ReadFile(filepath.Join(homeDirectory(), "git-test-stderr"))
				t.Fatalf("%v: %s", err, debug)
			}
			if _, e := os.Stat(maliciousSentinel); !os.IsNotExist(e) {
				t.Fatal("untrusted hook/filter/LFS executed")
			}
			if name == "cancel" || name == "deadline-limit" {
				time.Sleep(1100 * time.Millisecond)
				if _, e := os.Stat(sentinel); !os.IsNotExist(e) {
					t.Fatal("cancelled clone child survived")
				}
			}
		})
	}
}

func TestCatalogPushIntentRejectsUnenrolledTreeChanges(t *testing.T) {
	repository, _ := setupCatalogSyncFixture(t)
	record, _, err := readCatalogSyncRecord()
	if err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(repository, "unrelated"), []byte("synthetic"), 0600)
	runGit(t, repository, "add", "unrelated")
	runGit(t, repository, "commit", "-qm", "synthetic extra path")
	source := strings.TrimSpace(runGit(t, repository, "rev-parse", "HEAD"))
	record.PushIntent = &catalogstore.CatalogPushIntent{LocalHEAD: record.HEAD, SourceCommit: source, ExpectedRemote: record.HEAD, DestinationRef: "refs/heads/main", CatalogSHA256: record.BaseSHA256}
	runner, cleanup, err := managedgit.New()
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	if verifyCatalogPushIntent(context.Background(), runner, record) == nil {
		t.Fatal("restored arbitrary tree intent accepted")
	}
	record.PushIntent.SourceCommit = strings.Repeat("a", 40)
	if verifyCatalogPushIntent(context.Background(), runner, record) == nil {
		t.Fatal("lost source object intent accepted")
	}
}

func TestCatalogSyncTransportAndIntentBindingsStrict(t *testing.T) {
	setupCatalogSyncFixture(t)
	base, _, err := readCatalogSyncRecord()
	if err != nil {
		t.Fatal(err)
	}
	base.Provider = "github"
	base.RemoteURL = "https://github.com/synthetic/repo.git"
	base.Remote = "origin"
	base.RemoteRef = "refs/heads/main"
	base.Transport = "https"
	base.PushIntent = &catalogstore.CatalogPushIntent{LocalHEAD: base.HEAD, SourceCommit: strings.Repeat("a", 40), DestinationRef: base.RemoteRef, ExpectedRemote: base.HEAD, CatalogSHA256: base.BaseSHA256}
	if err := catalogstore.Validate(base); err != nil {
		t.Fatal(err)
	}
	for _, damage := range []string{"ref", "base", "ssh-missing-proof", "https-unused-proof", "transport-without-remote"} {
		record := base
		intent := *base.PushIntent
		record.PushIntent = &intent
		switch damage {
		case "ref":
			intent.DestinationRef = "refs/heads/other"
		case "base":
			intent.ExpectedRemote = strings.Repeat("b", 40)
		case "ssh-missing-proof":
			record.Transport = "ssh"
		case "https-unused-proof":
			record.KnownHostsSHA = strings.Repeat("a", 64)
		case "transport-without-remote":
			record.RemoteURL = ""
			record.Remote = ""
			record.RemoteRef = ""
			record.Provider = ""
			record.PushIntent = nil
		}
		if catalogstore.Validate(record) == nil {
			t.Fatal("malformed transport/intent accepted")
		}
	}
}

func TestCatalogStageCleanupResumesAfterRootRemoval(t *testing.T) {
	for _, replaceParent := range []bool{false, true} {
		name := "repeated"
		if replaceParent {
			name = "replaced-parent"
		}
		t.Run(name, func(t *testing.T) {
			setupCatalogSyncFixture(t)
			preview := remoteCatalogPreview{Provider: "github", Host: "github.com", Repository: "synthetic/repo", Revision: strings.Repeat("a", 40), Blob: strings.Repeat("b", 40)}
			if err := withMutation(func(session *mutationSession) error {
				intent, err := beginManagedCatalogStage(session, preview)
				if err != nil {
					return err
				}
				if len(intent.CreatedParents) == 0 {
					t.Fatal("fixture did not create owned parents")
				}
				if err := os.RemoveAll(intent.Root); err != nil {
					return err
				}
				if replaceParent {
					parent := intent.CreatedParents[len(intent.CreatedParents)-1]
					old := parent + "-original"
					if err := os.Rename(parent, old); err != nil {
						return err
					}
					if err := os.Mkdir(parent, 0700); err != nil {
						return err
					}
					replacement := observePlanIdentity(parent)
					if err := recoverManagedCatalogStages(session); err == nil {
						t.Fatal("replacement parent accepted during missing-root recovery")
					}
					if observePlanIdentity(parent).Inode != replacement.Inode {
						t.Fatal("replacement parent removed")
					}
					if _, err := os.Stat(managedStageIntentPath(intent.ID)); err != nil {
						t.Fatal("failed recovery discarded intent")
					}
					if err := os.Remove(parent); err != nil {
						return err
					}
					if err := os.Rename(old, parent); err != nil {
						return err
					}
				}
				if err := cleanupManagedCatalogStage(intent); err != nil {
					return err
				}
				if err := cleanupManagedCatalogStage(intent); err != nil {
					return err
				}
				for _, parent := range intent.CreatedParents {
					if _, err := os.Lstat(parent); !os.IsNotExist(err) {
						t.Fatal("owned empty parent remains")
					}
				}
				if err := recoverManagedCatalogStages(session); err != nil {
					return err
				}
				if err := recoverManagedCatalogStages(session); err != nil {
					return err
				}
				if _, err := os.Stat(managedStageIntentPath(intent.ID)); !os.IsNotExist(err) {
					t.Fatal("recovered intent remains")
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCatalogStageStatusUsesCanonicalParentSchemaReadOnly(t *testing.T) {
	setupCatalogSyncFixture(t)
	preview := remoteCatalogPreview{Provider: "github", Host: "github.com", Repository: "synthetic/repo", Revision: strings.Repeat("a", 40), Blob: strings.Repeat("b", 40)}
	if err := withMutation(func(session *mutationSession) error {
		intent, err := beginManagedCatalogStage(session, preview)
		if err != nil {
			return err
		}
		before := mutationInventory(t, homeDirectory())
		status := inspectStageRecovery(homeDirectory())
		if !status.Required || status.Blocked {
			t.Fatal("valid parent identities blocked stage status")
		}
		if before != mutationInventory(t, homeDirectory()) {
			t.Fatal("stage status changed inventory")
		}
		if err := os.RemoveAll(intent.Root); err != nil {
			return err
		}
		before = mutationInventory(t, homeDirectory())
		status = inspectStageRecovery(homeDirectory())
		if !status.Required || status.Blocked {
			t.Fatal("missing-root cleanup status blocked valid parents")
		}
		if before != mutationInventory(t, homeDirectory()) {
			t.Fatal("missing-root status changed inventory")
		}
		parent := intent.CreatedParents[len(intent.CreatedParents)-1]
		old := parent + "-original"
		if err := os.Rename(parent, old); err != nil {
			return err
		}
		if err := os.Mkdir(parent, 0700); err != nil {
			return err
		}
		before = mutationInventory(t, homeDirectory())
		status = inspectStageRecovery(homeDirectory())
		if !status.Required || !status.Blocked {
			t.Fatal("replacement parent not reported blocked")
		}
		if before != mutationInventory(t, homeDirectory()) {
			t.Fatal("blocked stage status changed inventory")
		}
		if err := os.Remove(parent); err != nil {
			return err
		}
		if err := os.Rename(old, parent); err != nil {
			return err
		}
		return recoverManagedCatalogStages(session)
	}); err != nil {
		t.Fatal(err)
	}
}
