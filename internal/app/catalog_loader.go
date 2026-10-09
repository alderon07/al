//go:build !windows

package app

import (
	"fmt"
	"os"
	"path/filepath"

	neutralcatalog "github.com/alderon07/al/internal/catalog"
	"github.com/alderon07/al/internal/catalogstore"
	"github.com/alderon07/al/internal/shell"
)

func (svc *Services) catalogStateRoot() string {
	return filepath.Join(svc.homeDirectory(), ".local", "state", "alias-lens")
}
func (svc *Services) catalogGeneratedRoot(shell string) string {
	return filepath.Join(filepath.Dir(svc.localCatalogPath()), "generated", shell)
}
func (svc *Services) catalogInstalledPath() string {
	return filepath.Join(svc.catalogStateRoot(), "catalog-installed.json")
}
func (svc *Services) catalogApprovalsPath() string {
	return filepath.Join(svc.catalogStateRoot(), "native-approvals.json")
}
func (svc *Services) catalogAdoptionsPath() string {
	return filepath.Join(svc.catalogStateRoot(), "adoptions.json")
}

func (svc *Services) readCatalogGeneration(root, nativePath, shell string) (catalogstore.GenerationManifest, error) {
	native, resolved, err := readCatalogNative(nativePath)
	if err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	return svc.readCatalogGenerationForNative(root, resolved, shell, native)
}

func (svc *Services) readCatalogGenerationForNative(root, nativePath, shell string, native []byte) (catalogstore.GenerationManifest, error) {
	pointer, err := catalogstore.ReadPrivateBytes(filepath.Join(root, "active"), 65)
	if err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	id, err := catalogstore.DecodePointer(pointer)
	if err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	result, err := svc.readCatalogImmutableGeneration(root, nativePath, shell, id, native)
	if err != nil {
		return result, err
	}
	if result.NativeSourcePath != "" {
		leaf := observePlanIdentity(result.NativeSourcePath)
		if leaf.FileType != "symlink" || leaf.LinkTarget != result.NativeSourceLinkTarget || fmt.Sprintf("%d:%d", leaf.Device, leaf.Inode) != result.NativeSourceIdentity {
			return catalogstore.GenerationManifest{}, fmt.Errorf("native source link changed; run al catalog enable")
		}
		observed, resolved, e := readCatalogNative(result.NativeSourcePath)
		if e != nil || resolved != result.NativePath || hashBytes(observed) != result.NativeInputSHA256 {
			return catalogstore.GenerationManifest{}, fmt.Errorf("native source referent changed; run al catalog enable")
		}
	}
	return result, nil
}

func (svc *Services) readCatalogImmutableGeneration(root, nativePath, shellName, id string, native []byte) (catalogstore.GenerationManifest, error) {
	if shellName != "bash" && shellName != "zsh" {
		return catalogstore.GenerationManifest{}, fmt.Errorf("unsupported catalog shell")
	}
	for path := root; ; path = filepath.Dir(path) {
		info, e := os.Lstat(path)
		if e != nil || !info.IsDir() || info.Mode().Perm() != 0700 || info.Mode()&os.ModeSymlink != 0 {
			return catalogstore.GenerationManifest{}, fmt.Errorf("catalog runtime directory must be private")
		}
		if filepath.Base(path) == "alias-lens" {
			break
		}
		if path == filepath.Dir(path) {
			return catalogstore.GenerationManifest{}, fmt.Errorf("catalog runtime root is invalid")
		}
	}
	pointer, err := catalogstore.EncodePointer(id)
	if err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	manifest, err := catalogstore.ReadPrivateBytes(filepath.Join(root, id+".json"), catalogstore.MaxDocumentBytes)
	if err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	body, err := catalogstore.ReadPrivateBytes(filepath.Join(root, id+".sh"), ShadowSourceLimit)
	if err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	if err := validateCatalogNativeControls(shellName, native); err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	result, err := catalogstore.VerifyGeneration(manifest, body, pointer, native, svc.validateCatalogNativeDeclaration)
	if err != nil || result.Shell != shellName || result.NativePath != nativePath {
		return catalogstore.GenerationManifest{}, fmt.Errorf("catalog package does not match this native input")
	}
	for _, entry := range result.Entries {
		if err := svc.validateCatalogDeclaration(shellName, entry.Entry, []byte(entry.Declaration)); err != nil {
			return catalogstore.GenerationManifest{}, err
		}
	}
	entries := make([]neutralcatalog.Entry, 0, len(result.Entries))
	declarations := make([][]byte, 0, len(result.Entries))
	for _, entry := range result.Entries {
		entries = append(entries, entry.Entry)
		declarations = append(declarations, []byte(entry.Declaration))
	}
	if err := shell.ValidateCatalogDependencyPolicy(shellName, entries, declarations, native); err != nil {
		return catalogstore.GenerationManifest{}, err
	}
	return result, nil
}

func catalogRuntimeHandoff(name string, entries []catalogstore.GenerationEntry) string {
	text, _ := shell.RuntimeHandoff(name, entries)
	return text
}
