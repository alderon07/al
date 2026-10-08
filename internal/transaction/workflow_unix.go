//go:build !windows

package transaction

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/sys/unix"
)

type workflowParent struct {
	file *os.File
	name string
}

func openWorkflowParent(path string) (*workflowParent, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, ErrUnsafePath
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(filepath.Dir(path), "/"), "/") {
		if part == "" {
			continue
		}
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if e != nil {
			return nil, fmt.Errorf("%w: target parent must be a real directory", ErrUnsafePath)
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), filepath.Dir(path))
	info, err := f.Stat()
	if err == nil {
		err = validateDirectoryOwner(info)
	}
	if err == nil && info.Mode().Perm()&0o022 != 0 {
		err = ErrUnsafePath
	}
	if err != nil {
		f.Close()
		return nil, err
	}
	return &workflowParent{f, filepath.Base(path)}, nil
}
func (p *workflowParent) close() { p.file.Close() }
func (p *workflowParent) open(flags int, mode uint32) (*os.File, error) {
	fd, err := unix.Openat(int(p.file.Fd()), p.name, flags|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, mode)
	if err != nil {
		return nil, err
	}
	return os.NewFile(uintptr(fd), p.name), nil
}
func (p *workflowParent) readLink() (string, error) {
	b := make([]byte, 4096)
	n, err := unix.Readlinkat(int(p.file.Fd()), p.name, b)
	if err != nil {
		return "", err
	}
	if n == len(b) {
		return "", ErrUnsafePath
	}
	return string(b[:n]), nil
}
func (p *workflowParent) rename(source string) error {
	return unix.Renameat(int(p.file.Fd()), source, int(p.file.Fd()), p.name)
}
func (p *workflowParent) remove() error { return unix.Unlinkat(int(p.file.Fd()), p.name, 0) }
func inspectWorkflowFile(path string, limit int64) (FileIdentity, []byte, error) {
	p, err := openWorkflowParent(path)
	if err != nil {
		return FileIdentity{}, nil, err
	}
	defer p.close()
	f, err := p.open(unix.O_RDONLY, 0)
	if errors.Is(err, os.ErrNotExist) {
		return FileIdentity{}, nil, nil
	}
	if err != nil {
		return FileIdentity{}, nil, ErrUnsafePath
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return FileIdentity{}, nil, err
	}
	id, err := identityFromInfo(info)
	if err != nil {
		return id, nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || id.Links != 1 || id.Owner != uint64(os.Geteuid()) || info.Size() > limit {
		return id, nil, ErrUnsafePath
	}
	n, err := unix.Flistxattr(int(f.Fd()), nil)
	if err != nil && !errors.Is(err, unix.ENOTSUP) {
		return id, nil, err
	}
	if n != 0 {
		return id, nil, fmt.Errorf("%w: extended file metadata cannot be preserved", ErrUnsafePath)
	}
	b, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return id, nil, err
	}
	if int64(len(b)) > limit {
		return id, nil, ErrUnsafePath
	}
	after, err := f.Stat()
	if err != nil {
		return id, nil, err
	}
	next, err := identityFromInfo(after)
	if err != nil || !sameOpenIdentity(id, next) || id.Size != next.Size || info.Mode() != after.Mode() || !info.ModTime().Equal(after.ModTime()) {
		return id, nil, ErrIdentityChanged
	}
	n, err = unix.Flistxattr(int(f.Fd()), nil)
	if err != nil && !errors.Is(err, unix.ENOTSUP) || n != 0 {
		return id, nil, ErrUnsafePath
	}
	id.SHA256 = hashBytes(b)
	return id, b, nil
}
func writeWorkflowTemp(p *workflowParent, name string, b []byte, mode uint32, owner, group uint64) error {
	fd, err := unix.Openat(int(p.file.Fd()), name, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_CLOEXEC|unix.O_NOFOLLOW, 0o600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(fd), name)
	defer f.Close()
	if err = writeAll(f, b); err != nil {
		return err
	}
	if owner != uint64(os.Geteuid()) {
		return ErrUnsafePath
	}
	if err = f.Chown(int(owner), int(group)); err != nil {
		return err
	}
	if err = f.Chmod(os.FileMode(mode)); err != nil {
		return err
	}
	return f.Sync()
}
func workflowOwnerGroup() (uint64, uint64) { return uint64(os.Geteuid()), uint64(os.Getegid()) }
func removeWorkflowTemp(p *workflowParent, name string) error {
	err := unix.Unlinkat(int(p.file.Fd()), name, 0)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
func promoteWorkflowDirectory(source, target string) error {
	a, err := openWorkflowParent(source)
	if err != nil {
		return err
	}
	defer a.close()
	b, err := openWorkflowParent(target)
	if err != nil {
		return err
	}
	defer b.close()
	if err = unix.Renameat(int(a.file.Fd()), a.name, int(b.file.Fd()), b.name); err != nil {
		return err
	}
	if err = a.file.Sync(); err != nil {
		return err
	}
	return b.file.Sync()
}

func EnsureWorkflowDirectory(path string, private bool) error {
	p, err := openWorkflowParent(path)
	if err != nil {
		return err
	}
	defer p.close()
	err = unix.Mkdirat(int(p.file.Fd()), p.name, 0o700)
	created := err == nil
	if err != nil && !errors.Is(err, unix.EEXIST) {
		return err
	}
	f, err := p.open(unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if err != nil {
		return ErrUnsafePath
	}
	defer f.Close()
	if created {
		if err = f.Chmod(0o700); err != nil {
			return err
		}
	}
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if err = validateDirectoryOwner(info); err != nil {
		return err
	}
	if info.Mode().Perm()&0o022 != 0 || private && info.Mode().Perm() != 0o700 {
		return ErrUnsafePath
	}
	return p.file.Sync()
}

func createWorkflowDirectory(p *workflowParent) error {
	if err := unix.Mkdirat(int(p.file.Fd()), p.name, 0o700); err != nil {
		return err
	}
	return p.file.Sync()
}
func removeWorkflowDirectory(p *workflowParent) error {
	if err := unix.Unlinkat(int(p.file.Fd()), p.name, unix.AT_REMOVEDIR); err != nil {
		return err
	}
	return p.file.Sync()
}
func workflowDirectoryMetadata(path string) error {
	p, e := openWorkflowParent(path)
	if e != nil {
		return e
	}
	defer p.close()
	f, e := p.open(unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	if e = validateDirectoryOwner(info); e != nil {
		return e
	}
	n, e := unix.Flistxattr(int(f.Fd()), nil)
	if e != nil && !errors.Is(e, unix.ENOTSUP) {
		return e
	}
	if n != 0 {
		return ErrUnsafePath
	}
	return nil
}

func inspectWorkflowDirectory(path string) (FileIdentity, error) {
	p, e := openWorkflowParent(path)
	if e != nil {
		return FileIdentity{}, e
	}
	defer p.close()
	root, e := p.open(unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if e != nil {
		return FileIdentity{}, e
	}
	defer root.Close()
	before, e := root.Stat()
	if e != nil {
		return FileIdentity{}, e
	}
	identity, e := identityFromInfo(before)
	if e != nil {
		return identity, e
	}
	hash := sha256.New()
	var count int
	var total int64
	var walk func(*os.File, string, int) error
	walk = func(directory *os.File, relative string, depth int) error {
		if depth > 64 {
			return ErrUnsafePath
		}
		info, e := directory.Stat()
		if e != nil {
			return e
		}
		if e = validateDirectoryOwner(info); e != nil {
			return e
		}
		if info.Mode().Perm()&0o022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
			return ErrUnsafePath
		}
		n, e := unix.Flistxattr(int(directory.Fd()), nil)
		if e != nil && !errors.Is(e, unix.ENOTSUP) {
			return e
		}
		if n != 0 {
			return ErrUnsafePath
		}
		frame := func(value any) error {
			b, e := json.Marshal(value)
			if e != nil {
				return e
			}
			var length [8]byte
			binary.BigEndian.PutUint64(length[:], uint64(len(b)))
			hash.Write(length[:])
			hash.Write(b)
			return nil
		}
		if e = frame(struct {
			Kind, Path   string
			Mode         uint32
			Owner, Group uint64
		}{"directory", relative, uint32(info.Mode().Perm()), uint64(os.Geteuid()), func() uint64 { id, _ := identityFromInfo(info); return id.Group }()}); e != nil {
			return e
		}
		entries, e := directory.ReadDir(-1)
		if e != nil {
			return e
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
		for _, entry := range entries {
			if !utf8.ValidString(entry.Name()) {
				return ErrUnsafePath
			}
			count++
			if count > 100000 {
				return ErrUnsafePath
			}
			fd, e := unix.Openat(int(directory.Fd()), entry.Name(), unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
			if e != nil {
				return ErrUnsafePath
			}
			child := os.NewFile(uintptr(fd), entry.Name())
			childInfo, e := child.Stat()
			if e != nil {
				child.Close()
				return e
			}
			name := filepath.Join(relative, entry.Name())
			if childInfo.IsDir() {
				e = walk(child, name, depth+1)
				child.Close()
				if e != nil {
					return e
				}
				continue
			}
			limit := int64(MaxPrivateFileSize)
			if strings.HasPrefix(name, ".git/objects/pack/") {
				switch filepath.Ext(name) {
				case ".pack", ".idx", ".rev", ".bitmap":
					limit = 256 << 20
				}
			}
			id, e := identityFromInfo(childInfo)
			if e != nil || !childInfo.Mode().IsRegular() || id.Owner != uint64(os.Geteuid()) || id.Links != 1 || childInfo.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 || childInfo.Size() > limit {
				child.Close()
				return ErrUnsafePath
			}
			n, e = unix.Flistxattr(fd, nil)
			if e != nil && !errors.Is(e, unix.ENOTSUP) || n != 0 {
				child.Close()
				return ErrUnsafePath
			}
			fileHash := sha256.New()
			read, e := io.Copy(fileHash, io.LimitReader(child, limit+1))
			after, se := child.Stat()
			child.Close()
			if e != nil || se != nil || read > limit {
				return ErrUnsafePath
			}
			afterID, se := identityFromInfo(after)
			if se != nil || !sameOpenIdentity(id, afterID) || id.Size != afterID.Size || !childInfo.ModTime().Equal(after.ModTime()) {
				return ErrIdentityChanged
			}
			total += read
			if total > 256<<20 {
				return ErrUnsafePath
			}
			if e = frame(struct {
				Kind, Path   string
				Mode         uint32
				Owner, Group uint64
				SHA256       string
			}{"file", name, id.Mode, id.Owner, id.Group, hex.EncodeToString(fileHash.Sum(nil))}); e != nil {
				return e
			}
		}
		after, e := directory.Stat()
		if e != nil {
			return e
		}
		beforeID, e := identityFromInfo(info)
		if e != nil {
			return e
		}
		afterID, e := identityFromInfo(after)
		if e != nil {
			return e
		}
		if !sameOpenIdentity(beforeID, afterID) || !info.ModTime().Equal(after.ModTime()) || info.Size() != after.Size() {
			return ErrIdentityChanged
		}
		return nil
	}
	if e = walk(root, ".", 0); e != nil {
		return identity, e
	}
	after, e := root.Stat()
	if e != nil {
		return identity, e
	}
	afterID, e := identityFromInfo(after)
	if e != nil || !sameOpenIdentity(identity, afterID) || !before.ModTime().Equal(after.ModTime()) {
		return identity, ErrIdentityChanged
	}
	identity.SHA256 = hex.EncodeToString(hash.Sum(nil))
	return identity, nil
}

func ReadWorkflowTarget(path string, limit int64) (FileIdentity, []byte, error) {
	if limit <= 0 {
		limit = MaxPrivateFileSize
	}
	return inspectWorkflowFile(path, limit)
}
func AppendWorkflowPrivateFile(path string, contents []byte) error {
	p, e := openWorkflowParent(path)
	if e != nil {
		return e
	}
	defer p.close()
	f, e := p.open(unix.O_WRONLY|unix.O_CREAT|unix.O_APPEND, 0o600)
	if e != nil {
		return e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return e
	}
	id, e := identityFromInfo(info)
	if e != nil {
		return e
	}
	if e = validatePrivateFileInfo(info, id, MaxPrivateFileSize-int64(len(contents))); e != nil {
		return e
	}
	n, e := unix.Flistxattr(int(f.Fd()), nil)
	if e != nil && !errors.Is(e, unix.ENOTSUP) || n != 0 {
		return ErrUnsafePath
	}
	if e = writeAll(f, contents); e != nil {
		return e
	}
	if e = f.Sync(); e != nil {
		return e
	}
	return p.file.Sync()
}

func RemoveWorkflowPrivateFile(path string) error {
	id, _, e := inspectWorkflowFile(path, MaxPrivateFileSize)
	if e != nil {
		return e
	}
	if !id.Exists {
		return nil
	}
	if id.Mode != 0o600 {
		return ErrUnsafePath
	}
	p, e := openWorkflowParent(path)
	if e != nil {
		return e
	}
	defer p.close()
	current, _, e := inspectWorkflowFile(path, MaxPrivateFileSize)
	if e != nil || !sameIdentity(id, current) {
		return ErrIdentityChanged
	}
	if e = p.remove(); e != nil {
		return e
	}
	return p.file.Sync()
}

func InspectWorkflowDirectoryMetadata(path string) (FileIdentity, error) {
	p, e := openWorkflowParent(path)
	if e != nil {
		return FileIdentity{}, e
	}
	defer p.close()
	f, e := p.open(unix.O_RDONLY|unix.O_DIRECTORY, 0)
	if e != nil {
		return FileIdentity{}, e
	}
	defer f.Close()
	info, e := f.Stat()
	if e != nil {
		return FileIdentity{}, e
	}
	if e = validateDirectoryOwner(info); e != nil {
		return FileIdentity{}, e
	}
	if info.Mode().Perm()&0o022 != 0 || info.Mode()&(os.ModeSetuid|os.ModeSetgid|os.ModeSticky) != 0 {
		return FileIdentity{}, ErrUnsafePath
	}
	n, e := unix.Flistxattr(int(f.Fd()), nil)
	if e != nil && !errors.Is(e, unix.ENOTSUP) {
		return FileIdentity{}, e
	}
	if n != 0 {
		return FileIdentity{}, ErrUnsafePath
	}
	return identityFromInfo(info)
}
