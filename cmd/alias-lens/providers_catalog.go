package main

import (
	"alias-lens/internal/catalogstore"
	"alias-lens/internal/managedgit"
	"alias-lens/internal/providers"
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

type remoteCatalogPreview struct {
	Provider           string
	Host               string
	Repository         string
	CloneURL           string
	Revision           string
	Blob               string
	CatalogPath        string
	Bytes              []byte
	Transport          string
	KnownHostsSHA      string
	KnownHostsIdentity string
	Branch             string
}

var catalogProviderTransport http.RoundTripper = http.DefaultTransport
var catalogGitObject = regexp.MustCompile(`^[0-9a-f]{40}$`)

func parseCatalogRemoteLocator(source string, config AppConfig) (providers.Locator, error) {
	return providerRegistry(config).ParseLocator(source)
}
func gitBlobSHA(data []byte) string { return providers.BlobSHA(data) }
func catalogProviderToken(ctx context.Context, provider, host string) (string, error) {
	return (providers.Runtime{}).Token(ctx, provider, host)
}
func proveCatalogRemoteAncestry(ctx context.Context, config AppConfig, record catalogstore.CatalogSyncRecord, remote remoteCatalogPreview) error {
	return providerRegistry(config).ProveAncestry(ctx, record.HEAD, providers.Object{Provider: record.Provider, Host: remote.Host, Repository: remote.Repository, Revision: remote.Revision})
}
func discoverRemoteCatalog(ctx context.Context, config AppConfig, source, path string) (remoteCatalogPreview, error) {
	return discoverRemoteCatalogAtRef(ctx, config, source, path, "")
}
func discoverRemoteCatalogAtRef(ctx context.Context, config AppConfig, source, path, ref string) (remoteCatalogPreview, error) {
	if _, err := cleanRepositoryRelativePath(path, "catalog path"); err != nil || strings.ContainsAny(path, "\x00\r\n\\?#%:") {
		return remoteCatalogPreview{}, errors.New("unsafe provider catalog path")
	}
	locator, err := parseCatalogRemoteLocator(source, config)
	if err != nil {
		return remoteCatalogPreview{}, err
	}
	if ref != "" {
		if !catalogstore.ValidRef(ref) {
			return remoteCatalogPreview{}, errors.New("invalid catalog remote ref")
		}
		locator.Ref = strings.TrimPrefix(ref, "refs/heads/")
	}
	provider := providerByID(config, locator.Provider)
	immutable, ok := provider.(providers.ImmutableProvider)
	if !ok {
		return remoteCatalogPreview{}, fmt.Errorf("provider cannot prove immutable catalog content; enroll a local checkout with al init")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	object, err := immutable.PreviewCatalog(ctx, locator, path)
	preview := remoteCatalogPreview{Provider: object.Provider, Host: object.Host, Repository: object.Repository, CloneURL: object.CloneURL, Revision: object.Revision, Blob: object.Blob, CatalogPath: object.CatalogPath, Bytes: object.Bytes, Branch: object.Branch}
	if err != nil {
		return preview, err
	}
	value, err := decodeSyncCatalog(preview.Bytes)
	if err != nil {
		return remoteCatalogPreview{}, err
	}
	if err := scanCatalogForSync(value, preview.Bytes); err != nil {
		return remoteCatalogPreview{}, err
	}
	preview.Transport = "https"
	protocol := strings.ToLower(config.Providers[locator.Provider].Protocol)
	if protocol == "ssh" || protocol == "auto" {
		_, sha, hostErr := managedgit.ReadKnownHosts(homeDirectory())
		if hostErr == nil && os.Getenv("SSH_AUTH_SOCK") != "" {
			preview.Transport = "ssh"
			preview.KnownHostsSHA = sha
			preview.KnownHostsIdentity = managedgit.KnownHostsIdentity(homeDirectory())
		} else if protocol == "ssh" {
			return preview, errors.New("configured SSH needs safe known hosts and an agent; enroll a local repository instead")
		}
	}
	return preview, nil
}
func catalogInitRemoteSource(source string) bool {
	return strings.Contains(source, "://") || strings.Contains(source, ":")
}
func (preview remoteCatalogPreview) managedTransport() managedgit.Transport {
	return managedgit.Transport{CloneURL: preview.CloneURL, Transport: preview.Transport, KnownHostsSHA: preview.KnownHostsSHA, KnownHostsIdentity: preview.KnownHostsIdentity, Home: homeDirectory(), AgentSocket: os.Getenv("SSH_AUTH_SOCK")}
}
