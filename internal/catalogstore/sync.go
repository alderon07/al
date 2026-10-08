package catalogstore

import (
	"errors"
	"github.com/alderon07/al/internal/catalog"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
)

type CatalogSyncRecord struct {
	Version            int                `json:"version"`
	Repository         string             `json:"repository"`
	CatalogPath        string             `json:"catalog_path"`
	BaseBytes          string             `json:"base_bytes"`
	SourceSHA256       string             `json:"source_sha256"`
	BaseSHA256         string             `json:"base_sha256"`
	HEAD               string             `json:"head"`
	Blob               string             `json:"blob"`
	Provider           string             `json:"provider,omitempty"`
	RemoteURL          string             `json:"remote_url,omitempty"`
	Remote             string             `json:"remote,omitempty"`
	RemoteRef          string             `json:"remote_ref,omitempty"`
	KnownHostsSHA      string             `json:"known_hosts_sha256,omitempty"`
	KnownHostsIdentity string             `json:"known_hosts_identity,omitempty"`
	Transport          string             `json:"transport,omitempty"`
	Managed            bool               `json:"managed"`
	PushIntent         *CatalogPushIntent `json:"push_intent,omitempty"`
}
type CatalogPushIntent struct {
	LocalHEAD      string `json:"local_head"`
	SourceCommit   string `json:"source_commit"`
	DestinationRef string `json:"destination_ref"`
	ExpectedRemote string `json:"expected_remote"`
	CatalogSHA256  string `json:"catalog_sha256"`
}

var gitObjectPattern = regexp.MustCompile(`^(?:[0-9a-f]{40}|[0-9a-f]{64})$`)
var gitRefPattern = regexp.MustCompile(`^refs/heads/[A-Za-z0-9][A-Za-z0-9_./-]*$`)

func ValidateSyncRecord(v CatalogSyncRecord) error {
	if v.Transport != "" && v.RemoteURL == "" {
		return errors.New("catalog transport requires a remote")
	}
	if v.Transport != "ssh" && (v.KnownHostsSHA != "" || v.KnownHostsIdentity != "") {
		return errors.New("SSH proof requires SSH transport")
	}
	if v.Transport == "ssh" && (!ValidHash(v.KnownHostsSHA) || !regexp.MustCompile(`^[0-9]{1,20}:[0-9]{1,20}:[0-9]{1,20}:[0-9]{1,10}$`).MatchString(v.KnownHostsIdentity)) {
		return errors.New("invalid SSH known host proof")
	}
	if v.Transport != "" && v.Transport != "https" && v.Transport != "ssh" {
		return errors.New("invalid catalog transport")
	}
	bad := errors.New("invalid catalog sync enrollment; run al init with a local repository")
	path := filepath.ToSlash(v.CatalogPath)
	if v.Version != 1 || !absolute(v.Repository) || path == "" || path == "." || strings.HasPrefix(path, "/") || filepath.ToSlash(filepath.Clean(v.CatalogPath)) != path || path == ".." || strings.HasPrefix(path, "../") || strings.ContainsAny(path, "\x00\r\n\\:") || len(path) > 4096 || !ValidHash(v.SourceSHA256) || !ValidHash(v.BaseSHA256) || Hash([]byte(v.BaseBytes)) != v.BaseSHA256 || !gitObjectPattern.MatchString(v.HEAD) || !gitObjectPattern.MatchString(v.Blob) {
		return bad
	}
	if _, diagnostics := catalog.Decode([]byte(v.BaseBytes)); len(diagnostics) > 0 {
		return bad
	}
	if v.RemoteURL != "" {
		u, err := url.Parse(v.RemoteURL)
		if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
			return bad
		}
		if v.Provider != "github" && v.Provider != "gitlab" && v.Provider != "bitbucket" {
			return bad
		}
	}
	if (v.RemoteURL == "") != (v.Remote == "") {
		return bad
	}
	if v.Remote != "" && v.Remote != "origin" || v.RemoteRef != "" && !validRef(v.RemoteRef) {
		return bad
	}
	if p := v.PushIntent; p != nil {
		if p.DestinationRef != v.RemoteRef || p.ExpectedRemote != v.HEAD || !gitObjectPattern.MatchString(p.LocalHEAD) || !gitObjectPattern.MatchString(p.SourceCommit) || !validRef(p.DestinationRef) || !gitObjectPattern.MatchString(p.ExpectedRemote) || !ValidHash(p.CatalogSHA256) || v.Remote == "" {
			return bad
		}
	}
	return nil
}
func validRef(s string) bool {
	return gitRefPattern.MatchString(s) && !strings.Contains(s, "..") && !strings.Contains(s, "//") && !strings.HasSuffix(s, "/") && !strings.HasSuffix(s, ".lock") && !strings.Contains(s, "@{")
}

func ValidRef(s string) bool { return validRef(s) }
