package app

import (
	"context"
	"errors"
	"fmt"
	"github.com/alderon07/al/internal/catalogstore"
	"github.com/alderon07/al/internal/managedgit"
	"github.com/alderon07/al/internal/providers"
	"net/http"

	"regexp"
	"strings"
	"time"
)

type RemoteCatalogPreview struct {
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
	services           *Services
}

var catalogProviderTransport http.RoundTripper = http.DefaultTransport
var catalogGitObject = regexp.MustCompile(`^[0-9a-f]{40}$`)

func (svc *Services) parseCatalogRemoteLocator(source string, config AppConfig) (providers.Locator, error) {
	return svc.providerRegistry(config).ParseLocator(source)
}
func gitBlobSHA(data []byte) string { return providers.BlobSHA(data) }
func (svc *Services) catalogProviderToken(ctx context.Context, provider, host string) (string, error) {
	return (providers.Runtime{Getenv: svc.dependencies.Environment}).Token(ctx, provider, host)
}
func (svc *Services) proveCatalogRemoteAncestry(ctx context.Context, config AppConfig, record catalogstore.CatalogSyncRecord, remote RemoteCatalogPreview) error {
	return svc.providerRegistry(config).ProveAncestry(ctx, record.HEAD, providers.Object{Provider: record.Provider, Host: remote.Host, Repository: remote.Repository, Revision: remote.Revision})
}
func (svc *Services) discoverRemoteCatalog(ctx context.Context, config AppConfig, source, path string) (RemoteCatalogPreview, error) {
	return svc.discoverRemoteCatalogAtRef(ctx, config, source, path, "")
}
func (svc *Services) discoverRemoteCatalogAtRef(ctx context.Context, config AppConfig, source, path, ref string) (RemoteCatalogPreview, error) {
	if _, err := cleanRepositoryRelativePath(path, "catalog path"); err != nil || strings.ContainsAny(path, "\x00\r\n\\?#%:") {
		return RemoteCatalogPreview{services: svc}, errors.New("unsafe provider catalog path")
	}
	locator, err := svc.parseCatalogRemoteLocator(source, config)
	if err != nil {
		return RemoteCatalogPreview{services: svc}, err
	}
	if ref != "" {
		if !catalogstore.ValidRef(ref) {
			return RemoteCatalogPreview{services: svc}, errors.New("invalid catalog remote ref")
		}
		locator.Ref = strings.TrimPrefix(ref, "refs/heads/")
	}
	provider := svc.providerByID(config, locator.Provider)
	immutable, ok := provider.(providers.ImmutableProvider)
	if !ok {
		return RemoteCatalogPreview{services: svc}, fmt.Errorf("provider cannot prove immutable catalog content; enroll a local checkout with al init")
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	object, err := immutable.PreviewCatalog(ctx, locator, path)
	preview := RemoteCatalogPreview{Provider: object.Provider, Host: object.Host, Repository: object.Repository, CloneURL: object.CloneURL, Revision: object.Revision, Blob: object.Blob, CatalogPath: object.CatalogPath, Bytes: object.Bytes, Branch: object.Branch, services: svc}
	if err != nil {
		return preview, err
	}
	value, err := decodeSyncCatalog(preview.Bytes)
	if err != nil {
		return RemoteCatalogPreview{services: svc}, err
	}
	if err := svc.scanCatalogForSync(value, preview.Bytes); err != nil {
		return RemoteCatalogPreview{services: svc}, err
	}
	preview.Transport = "https"
	protocol := strings.ToLower(config.Providers[locator.Provider].Protocol)
	if protocol == "ssh" || protocol == "auto" {
		_, sha, hostErr := managedgit.ReadKnownHosts(svc.homeDirectory())
		if hostErr == nil && svc.dependencies.Environment("SSH_AUTH_SOCK") != "" {
			preview.Transport = "ssh"
			preview.KnownHostsSHA = sha
			preview.KnownHostsIdentity = managedgit.KnownHostsIdentity(svc.homeDirectory())
		} else if protocol == "ssh" {
			return preview, errors.New("configured SSH needs safe known hosts and an agent; enroll a local repository instead")
		}
	}
	return preview, nil
}
func catalogInitRemoteSource(source string) bool {
	return strings.Contains(source, "://") || strings.Contains(source, ":")
}
func (preview RemoteCatalogPreview) managedTransport() managedgit.Transport {
	svc := preview.services
	if svc == nil {
		svc =
			DefaultServices()
	}

	return managedgit.Transport{CloneURL: preview.CloneURL, Transport: preview.Transport, KnownHostsSHA: preview.KnownHostsSHA, KnownHostsIdentity: preview.KnownHostsIdentity, Home: svc.homeDirectory(), AgentSocket: svc.dependencies.Environment("SSH_AUTH_SOCK")}
}
