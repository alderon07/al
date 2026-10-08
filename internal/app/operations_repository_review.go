package app

import (
	"bytes"
	"fmt"
	"os"
)

type RepositoryDiffPreview struct {
	Source     string
	Target     string
	Local      []byte
	Remote     []byte
	Equal      bool
	LocalOnly  []string
	RemoteOnly []string
	Conflicts  []AliasConflict
}

func (s *Services) PreviewRepositoryDiff() (RepositoryDiffPreview, error) {
	_, source, target, err := s.repositoryPaths()
	if err != nil {
		return RepositoryDiffPreview{}, err
	}
	local, err := readFileLimited(source, AliasFileLimit)
	if err != nil {
		return RepositoryDiffPreview{}, err
	}
	remote, err := readFileLimited(target, AliasFileLimit)
	if os.IsNotExist(err) {
		remote = nil
	} else if err != nil {
		return RepositoryDiffPreview{}, err
	}
	result := RepositoryDiffPreview{Source: source, Target: target, Local: local, Remote: remote, Equal: bytes.Equal(local, remote)}
	if result.Equal {
		hash := contentHash(local)
		if err := s.writeSyncStatus("synced", "files match", hash, hash); err != nil {
			return RepositoryDiffPreview{}, fmt.Errorf("alias files match, but refresh sync status: %w; run al diff again", err)
		}
	} else {
		result.LocalOnly, result.RemoteOnly, result.Conflicts = compareAliasFiles(local, remote)
	}
	return result, nil
}
