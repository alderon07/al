package main

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	workflowplan "alias-lens/internal/plan"
	"alias-lens/internal/transaction"
)

type planBuilder func() (workflowplan.OperationPlan, error)

func applyPrivatePlan(root string, preview workflowplan.OperationPlan, rebuild planBuilder) error {
	return applyMutationPlan(preview, rebuild)
}

func plannedIdentityMatches(expected workflowplan.Identity, actual transaction.FileIdentity) bool {
	if expected.FileType == "missing" {
		return !actual.Exists
	}
	if expected.FileType != "regular" || !actual.Exists {
		return false
	}
	if expected.Device != 0 || expected.Inode != 0 {
		platformID := fmt.Sprintf("%d:%d", expected.Device, expected.Inode)
		return platformID == actual.PlatformID && expected.Mode == actual.Mode && expected.Owner == actual.Owner &&
			expected.Group == actual.Group && expected.LinkCount == actual.Links
	}
	return expected.Mode == actual.Mode && (expected.LinkCount == 0 || expected.LinkCount == actual.Links)
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() || info.Mode().Perm() != 0o700 {
		return fmt.Errorf("Alias Lens cannot use %s because it is not a private directory with mode 0700", displayPrivatePath(path))
	}
	return nil
}

func recoverPrivateTransactions(root, directory string) error {
	entries, err := os.ReadDir(directory)
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".journal" {
			continue
		}
		if err := transaction.RecoverJournal(root, filepath.Join(directory, entry.Name())); err != nil {
			return err
		}
	}
	return nil
}

func newOperationID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("create operation ID: %w", err)
	}
	return "op-" + hex.EncodeToString(buffer), nil
}
