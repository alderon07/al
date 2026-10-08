package providers

import (
	"context"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type Locator struct{ Provider, Host, Repository, Ref string }
type Object struct {
	Provider, Host, Repository, CloneURL, Revision, Blob, CatalogPath string
	Bytes                                                             []byte
	Branch                                                            string
}
type ImmutableProvider interface {
	PreviewCatalog(context.Context, Locator, string) (Object, error)
}

var catalogGitObject = regexp.MustCompile(`^[0-9a-f]{40}$`)

func (registry *Registry) ParseLocator(source string) (Locator, error) {
	var err error
	source, err = registry.ExpandSource(source)
	if err != nil {
		return Locator{}, err
	}

	u, err := url.Parse(source)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || strings.Contains(u.Path, "%") || strings.HasSuffix(u.Path, "/") {
		return Locator{}, fmt.Errorf("SOURCE must be a configured credential-free HTTPS repository URL")
	}
	parts, err := cleanRemoteRepositoryParts(strings.TrimSuffix(strings.TrimPrefix(u.Path, "/"), ".git"))
	if err != nil || len(parts) < 2 {
		return Locator{}, fmt.Errorf("SOURCE must identify a repository")
	}
	for _, provider := range registry.Configured() {
		host := provider.Host()
		host = strings.TrimPrefix(strings.TrimSuffix(host, "/"), "https://")
		if strings.EqualFold(u.Host, host) {
			if provider.ID() != "gitlab" && len(parts) != 2 {
				return Locator{}, fmt.Errorf("provider locator needs owner/repository")
			}
			return Locator{Provider: provider.ID(), Host: u.Host, Repository: strings.Join(parts, "/")}, nil
		}
	}
	return Locator{}, fmt.Errorf("configure this HTTPS provider host with al config provider before init")
}
func (runtime Runtime) catalogAPIGet(ctx context.Context, endpoint, header, credential string, destination any) error {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.Host == "" || u.User != nil {
		return errors.New("provider endpoint must use configured HTTPS authority")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return errors.New("invalid provider request")
	}
	request.Header.Set("Accept", "application/json")
	if credential != "" {
		request.Header.Set(header, credential)
	}
	client := http.Client{Transport: runtime.Transport, Timeout: 20 * time.Second, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if next.URL.Scheme != "https" || next.URL.Host != u.Host || next.URL.User != nil || len(via) > 5 {
			return errors.New("provider redirect refused")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return errors.New("provider discovery failed; verify al config provider and credentials")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("provider returned HTTP %d; reconnect with al repo", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, providerResponseLimit+1))
	if err != nil || len(data) > providerResponseLimit {
		return errors.New("provider response exceeds catalog discovery limit")
	}
	if err := json.Unmarshal(data, destination); err != nil {
		return errors.New("invalid provider discovery response")
	}
	return nil
}
func BlobSHA(data []byte) string {
	hash := sha1.New()
	fmt.Fprintf(hash, "blob %d\x00", len(data))
	hash.Write(data)
	return hex.EncodeToString(hash.Sum(nil))
}
func decodeRemoteCatalog(preview Object, encoded string, size int64) (Object, error) {
	if !catalogGitObject.MatchString(preview.Revision) || !catalogGitObject.MatchString(preview.Blob) || size < 0 || size > providerResponseLimit {
		return Object{}, errors.New("provider did not supply bounded immutable Git objects")
	}
	data, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(encoded, "\n", ""))
	if err != nil || int64(len(data)) != size || BlobSHA(data) != preview.Blob {
		return Object{}, errors.New("provider catalog does not match its pinned Git blob")
	}
	preview.Bytes = data
	return preview, nil
}
func (runtime Runtime) Token(ctx context.Context, provider, host string) (string, error) {
	if runtime.Credential != nil {
		return runtime.Credential(ctx, provider, host)
	}
	environment := map[string]string{"github": "GH_TOKEN", "gitlab": "GITLAB_TOKEN", "bitbucket": "BITBUCKET_API_TOKEN"}
	if token := strings.TrimSpace(runtime.getenv(environment[provider])); token != "" {
		return token, nil
	}
	if provider == "github" {
		stdout, _, err := runtime.outputBounded(ctx, 65536, "gh", "auth", "token", "--hostname", host)
		if err == nil && len(strings.TrimSpace(string(stdout))) > 0 {
			return strings.TrimSpace(string(stdout)), nil
		}
	}
	return "", fmt.Errorf("connect %s credentials with al repo %s before remote init", provider, provider)
}
func (p githubProvider) PreviewCatalog(ctx context.Context, locator Locator, path string) (Object, error) {
	if err := validateLocator(locator, p.ID(), p.Host(), path); err != nil {
		return Object{}, err
	}
	token, err := p.runtime.Token(ctx, p.ID(), locator.Host)
	if err != nil {
		return Object{}, err
	}
	base := "https://" + locator.Host + "/api/v3"
	if locator.Host == "github.com" {
		base = "https://api.github.com"
	}
	repositoryEndpoint := base + "/repos/" + locator.Repository
	var repository struct {
		Archived      bool   `json:"archived"`
		DefaultBranch string `json:"default_branch"`
		Permissions   struct {
			Push bool `json:"push"`
		} `json:"permissions"`
	}
	if err := p.runtime.catalogAPIGet(ctx, repositoryEndpoint, "Authorization", "Bearer "+token, &repository); err != nil {
		return Object{}, err
	}
	if repository.Archived || !repository.Permissions.Push || repository.DefaultBranch == "" {
		return Object{}, errors.New("repository needs write access and a default branch")
	}
	if locator.Ref != "" {
		repository.DefaultBranch = locator.Ref
	}
	var commit struct {
		SHA string `json:"sha"`
	}
	if err := p.runtime.catalogAPIGet(ctx, repositoryEndpoint+"/commits/"+url.PathEscape(repository.DefaultBranch), "Authorization", "Bearer "+token, &commit); err != nil {
		return Object{}, err
	}
	if !catalogGitObject.MatchString(commit.SHA) {
		return Object{}, errors.New("provider revision is not immutable")
	}
	var file struct {
		Type     string `json:"type"`
		SHA      string `json:"sha"`
		Size     int64  `json:"size"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
		Path     string `json:"path"`
	}
	if err := p.runtime.catalogAPIGet(ctx, repositoryEndpoint+"/contents/"+path+"?ref="+commit.SHA, "Authorization", "Bearer "+token, &file); err != nil {
		return Object{}, err
	}
	if file.Type != "file" || file.Encoding != "base64" || file.Path != path {
		return Object{}, errors.New("provider catalog is not a regular exact-path file")
	}
	return decodeRemoteCatalog(Object{Provider: p.ID(), Host: locator.Host, Repository: locator.Repository, CloneURL: "https://" + locator.Host + "/" + locator.Repository + ".git", Revision: commit.SHA, Branch: repository.DefaultBranch, Blob: file.SHA, CatalogPath: path}, file.Content, file.Size)
}
func (p gitlabProvider) PreviewCatalog(ctx context.Context, locator Locator, path string) (Object, error) {
	if err := validateLocator(locator, p.ID(), p.Host(), path); err != nil {
		return Object{}, err
	}
	token, err := p.runtime.Token(ctx, p.ID(), locator.Host)
	if err != nil {
		return Object{}, err
	}
	base := "https://" + locator.Host + "/api/v4/projects/" + url.PathEscape(locator.Repository)
	var project struct {
		DefaultBranch string `json:"default_branch"`
		Archived      bool   `json:"archived"`
		Permissions   struct {
			ProjectAccess *struct {
				AccessLevel int `json:"access_level"`
			} `json:"project_access"`
			GroupAccess *struct {
				AccessLevel int `json:"access_level"`
			} `json:"group_access"`
		} `json:"permissions"`
	}
	if err := p.runtime.catalogAPIGet(ctx, base, "PRIVATE-TOKEN", token, &project); err != nil {
		return Object{}, err
	}
	level := 0
	if project.Permissions.ProjectAccess != nil {
		level = project.Permissions.ProjectAccess.AccessLevel
	}
	if project.Permissions.GroupAccess != nil && project.Permissions.GroupAccess.AccessLevel > level {
		level = project.Permissions.GroupAccess.AccessLevel
	}
	if project.Archived || level < 30 || project.DefaultBranch == "" {
		return Object{}, errors.New("repository needs write access and a default branch")
	}
	if locator.Ref != "" {
		project.DefaultBranch = locator.Ref
	}
	var commit struct {
		ID string `json:"id"`
	}
	if err := p.runtime.catalogAPIGet(ctx, base+"/repository/commits/"+url.PathEscape(project.DefaultBranch), "PRIVATE-TOKEN", token, &commit); err != nil {
		return Object{}, err
	}
	var file struct {
		BlobID   string `json:"blob_id"`
		CommitID string `json:"commit_id"`
		FilePath string `json:"file_path"`
		Size     int64  `json:"size"`
		Encoding string `json:"encoding"`
		Content  string `json:"content"`
	}
	if err := p.runtime.catalogAPIGet(ctx, base+"/repository/files/"+url.PathEscape(path)+"?ref="+commit.ID, "PRIVATE-TOKEN", token, &file); err != nil {
		return Object{}, err
	}
	if file.CommitID != commit.ID || file.FilePath != path || file.Encoding != "base64" {
		return Object{}, errors.New("provider file revision or path changed during discovery")
	}
	return decodeRemoteCatalog(Object{Provider: p.ID(), Host: locator.Host, Repository: locator.Repository, CloneURL: "https://" + locator.Host + "/" + locator.Repository + ".git", Revision: commit.ID, Branch: project.DefaultBranch, Blob: file.BlobID, CatalogPath: path}, file.Content, file.Size)
}
func (p bitbucketProvider) PreviewCatalog(ctx context.Context, locator Locator, path string) (Object, error) {
	if err := validateLocator(locator, p.ID(), p.Host(), path); err != nil {
		return Object{}, err
	}
	token, err := p.runtime.Token(ctx, p.ID(), locator.Host)
	if err != nil {
		return Object{}, err
	}
	writable := false
	repos, err := p.List(ctx)
	if err != nil {
		return Object{}, err
	}
	for _, repo := range repos {
		if repo.FullName == locator.Repository {
			writable = true
			break
		}
	}
	if !writable {
		return Object{}, errors.New("repository needs write access")
	}
	base := "https://api.bitbucket.org/2.0/repositories/" + locator.Repository
	var repository struct {
		Mainbranch struct {
			Name string `json:"name"`
		} `json:"mainbranch"`
	}
	if err := p.runtime.catalogAPIGet(ctx, base, "Authorization", "Bearer "+token, &repository); err != nil {
		return Object{}, err
	}
	if repository.Mainbranch.Name == "" {
		return Object{}, errors.New("repository needs a default branch")
	}
	if locator.Ref != "" {
		repository.Mainbranch.Name = locator.Ref
	}
	var commit struct {
		Hash string `json:"hash"`
	}
	if err := p.runtime.catalogAPIGet(ctx, base+"/commit/"+url.PathEscape(repository.Mainbranch.Name), "Authorization", "Bearer "+token, &commit); err != nil {
		return Object{}, err
	}
	if !catalogGitObject.MatchString(commit.Hash) {
		return Object{}, errors.New("provider revision is not immutable")
	}
	endpoint := base + "/src/" + commit.Hash + "/" + path
	var metadata struct {
		Type       string   `json:"type"`
		Path       string   `json:"path"`
		Size       int64    `json:"size"`
		Attributes []string `json:"attributes"`
		Commit     struct {
			Hash string `json:"hash"`
		} `json:"commit"`
	}
	if err := p.runtime.catalogAPIGet(ctx, endpoint+"?format=meta", "Authorization", "Bearer "+token, &metadata); err != nil {
		return Object{}, err
	}
	if metadata.Type != "commit_file" || metadata.Path != path || metadata.Commit.Hash != commit.Hash || metadata.Size < 0 || metadata.Size > providerResponseLimit {
		return Object{}, errors.New("provider file metadata does not match its pinned revision")
	}
	for _, attribute := range metadata.Attributes {
		if attribute != "executable" {
			return Object{}, errors.New("provider catalog has unsupported file attributes")
		}
	}
	data, err := p.runtime.catalogAPIRaw(ctx, endpoint, "Authorization", "Bearer "+token)
	if err != nil {
		return Object{}, err
	}
	if int64(len(data)) != metadata.Size {
		return Object{}, errors.New("provider catalog size changed during discovery")
	}
	return Object{Provider: p.ID(), Host: locator.Host, Repository: locator.Repository, CloneURL: "https://bitbucket.org/" + locator.Repository + ".git", Revision: commit.Hash, Branch: repository.Mainbranch.Name, Blob: BlobSHA(data), CatalogPath: path, Bytes: data}, nil
}
func (runtime Runtime) catalogAPIRaw(ctx context.Context, endpoint, header, credential string) ([]byte, error) {
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.Host == "" {
		return nil, errors.New("invalid provider raw endpoint")
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, errors.New("invalid provider request")
	}
	request.Header.Set(header, credential)
	client := http.Client{Transport: runtime.Transport, Timeout: 20 * time.Second, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if next.URL.Scheme != "https" || next.URL.Host != u.Host || next.URL.User != nil || len(via) > 5 {
			return errors.New("provider raw redirect refused")
		}
		return nil
	}}
	response, err := client.Do(request)
	if err != nil {
		return nil, errors.New("provider raw discovery failed")
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return nil, errors.New("provider raw discovery failed")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, providerResponseLimit+1))
	if err != nil || len(data) > providerResponseLimit {
		return nil, errors.New("provider raw response exceeds catalog limit")
	}
	return data, nil
}
func (registry *Registry) ProveAncestry(ctx context.Context, baseRevision string, remote Object) error {
	provider := registry.ByID(remote.Provider)
	if provider == nil || !catalogGitObject.MatchString(baseRevision) || !catalogGitObject.MatchString(remote.Revision) {
		return errors.New("invalid remote ancestry identity")
	}
	if err := validateLocator(Locator{Provider: remote.Provider, Host: remote.Host, Repository: remote.Repository}, provider.ID(), provider.Host(), "catalog.json"); err != nil {
		return err
	}
	if baseRevision == remote.Revision {
		return nil
	}
	token, err := registry.runtime.Token(ctx, remote.Provider, remote.Host)
	if err != nil {
		return err
	}
	if remote.Provider == "github" {
		base := "https://" + remote.Host + "/api/v3"
		if remote.Host == "github.com" {
			base = "https://api.github.com"
		}
		var comparison struct {
			Status     string `json:"status"`
			BaseCommit struct {
				SHA string `json:"sha"`
			} `json:"base_commit"`
			MergeBaseCommit struct {
				SHA string `json:"sha"`
			} `json:"merge_base_commit"`
		}
		if err := registry.runtime.catalogAPIGet(ctx, base+"/repos/"+remote.Repository+"/compare/"+baseRevision+"..."+remote.Revision, "Authorization", "Bearer "+token, &comparison); err != nil {
			return err
		}
		if comparison.BaseCommit.SHA != baseRevision || comparison.MergeBaseCommit.SHA != baseRevision || (comparison.Status != "ahead" && comparison.Status != "identical") {
			return errors.New("remote catalog history is not a proven fast-forward; preserve enrollment and inspect the repository")
		}
		return nil
	}
	queue := []string{remote.Revision}
	seen := map[string]bool{}
	for len(queue) > 0 {
		if len(seen) >= 256 {
			return errors.New("remote ancestry proof exceeds bounded discovery; enroll a complete local repository")
		}
		current := queue[0]
		queue = queue[1:]
		if current == baseRevision {
			return nil
		}
		if seen[current] {
			continue
		}
		seen[current] = true
		parents := []string{}
		switch remote.Provider {
		case "gitlab":
			var commit struct {
				ID        string   `json:"id"`
				ParentIDs []string `json:"parent_ids"`
			}
			endpoint := "https://" + remote.Host + "/api/v4/projects/" + url.PathEscape(remote.Repository) + "/repository/commits/" + current
			if err := registry.runtime.catalogAPIGet(ctx, endpoint, "PRIVATE-TOKEN", token, &commit); err != nil {
				return err
			}
			if commit.ID != current {
				return errors.New("remote ancestry object changed")
			}
			parents = commit.ParentIDs
		case "bitbucket":
			var commit struct {
				Hash    string `json:"hash"`
				Parents []struct {
					Hash string `json:"hash"`
				} `json:"parents"`
			}
			if err := registry.runtime.catalogAPIGet(ctx, "https://api.bitbucket.org/2.0/repositories/"+remote.Repository+"/commit/"+current, "Authorization", "Bearer "+token, &commit); err != nil {
				return err
			}
			if commit.Hash != current {
				return errors.New("remote ancestry object changed")
			}
			for _, parent := range commit.Parents {
				parents = append(parents, parent.Hash)
			}
		default:
			return errors.New("provider cannot prove remote ancestry")
		}
		if len(parents) > 16 {
			return errors.New("remote ancestry parent bound exceeded")
		}
		for _, parent := range parents {
			if !catalogGitObject.MatchString(parent) {
				return errors.New("remote ancestry contains invalid objects")
			}
			queue = append(queue, parent)
		}
	}
	return errors.New("remote catalog history is not a proven fast-forward")
}

func (registry *Registry) ExpandSource(source string) (string, error) {
	if strings.Contains(source, "://") {
		return source, nil
	}
	providerID, repository, ok := strings.Cut(source, ":")
	if !ok {
		return "", errors.New("remote SOURCE needs github:OWNER/REPOSITORY, gitlab:NAMESPACE/REPO, bitbucket:WORKSPACE/REPO or a configured HTTPS URL")
	}
	parts, err := cleanRemoteRepositoryParts(repository)
	if err != nil || len(parts) < 2 || strings.ContainsAny(repository, "?#%\\\r\n") {
		return "", errors.New("remote SOURCE needs safe provider repository parts")
	}
	provider := registry.ByID(providerID)
	if provider == nil {
		return "", errors.New("configure the SOURCE provider with al config provider")
	}
	host := provider.Host()
	if providerID != "gitlab" && len(parts) != 2 {
		return "", errors.New("SOURCE needs provider:OWNER/REPOSITORY")
	}
	host = strings.TrimPrefix(strings.TrimSuffix(host, "/"), "https://")
	return "https://" + host + "/" + strings.Join(parts, "/"), nil
}

func validateLocator(locator Locator, provider, host, path string) error {
	authority := strings.TrimPrefix(strings.TrimSuffix(host, "/"), "https://")
	if locator.Provider != provider || !strings.EqualFold(locator.Host, authority) {
		return errors.New("provider locator differs from configured HTTPS authority")
	}
	parts, err := cleanRemoteRepositoryParts(locator.Repository)
	if err != nil || len(parts) < 2 || strings.ContainsAny(locator.Repository, "?#%\\\r\n") || provider != "gitlab" && len(parts) != 2 {
		return errors.New("unsafe provider repository identity")
	}
	if _, err := cleanRepositoryRelativePath(path, "catalog path"); err != nil || strings.ContainsAny(path, "\x00\r\n\\?#%:") {
		return errors.New("unsafe provider catalog path")
	}
	return nil
}
