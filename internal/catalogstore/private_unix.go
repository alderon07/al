//go:build !windows

package catalogstore

import (
	"errors"
	"golang.org/x/sys/unix"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
)

func ReadPrivate(path string, destination any) error { return readPrivate(path, destination, nil) }
func readPrivate(path string, destination any, observed func()) error {
	data, err := readPrivateBytes(path, MaxDocumentBytes, observed)
	if err != nil {
		return err
	}
	return Decode(data, destination)
}
func ReadPrivateBytes(path string, limit int64) ([]byte, error) {
	return readPrivateBytes(path, limit, nil)
}
func readPrivateBytes(path string, limit int64, observed func()) ([]byte, error) {
	if limit < 1 || limit > MaxDocumentBytes {
		return nil, errors.New("invalid private file limit")
	}
	if !absolute(path) {
		return nil, errors.New("catalog state path must be absolute")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, errors.New("cannot open catalog state root")
	}
	parts := strings.Split(strings.TrimPrefix(filepath.Clean(path), "/"), "/")
	for _, part := range parts[:len(parts)-1] {
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		unix.Close(fd)
		if e != nil {
			return nil, errors.Join(errors.New("catalog state parent must not be a symlink"), e)
		}
		fd = next
	}
	defer unix.Close(fd)
	leaf := parts[len(parts)-1]
	opened, err := unix.Openat(fd, leaf, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, errors.Join(errors.New("cannot safely open catalog state"), err)
	}
	f := os.NewFile(uintptr(opened), leaf)
	defer f.Close()
	var before unix.Stat_t
	if unix.Fstat(opened, &before) != nil || !privateStat(before) {
		return nil, errors.New("catalog state must be owned, private and singly linked")
	}
	if observed != nil {
		observed()
	}
	if before.Size > limit {
		return nil, errors.New("private file exceeds limit")
	}
	data := make([]byte, int(before.Size))
	if _, err := io.ReadFull(f, data); err != nil {
		return nil, errors.New("cannot read catalog state")
	}
	var extra [1]byte
	if n, err := f.Read(extra[:]); n != 0 || err != io.EOF {
		return nil, errors.New("catalog state changed while reading")
	}
	var after, current unix.Stat_t
	if unix.Fstat(opened, &after) != nil || unix.Fstatat(fd, leaf, &current, unix.AT_SYMLINK_NOFOLLOW) != nil || !stableStat(before, after) || !stableStat(after, current) || !privateStat(after) {
		return nil, errors.New("catalog state changed while reading")
	}
	if int64(len(data)) > limit {
		return nil, errors.New("private file exceeds limit")
	}
	return data, nil
}
func privateStat(s unix.Stat_t) bool {
	return s.Mode&unix.S_IFMT == unix.S_IFREG && s.Mode&0077 == 0 && s.Uid == uint32(os.Geteuid()) && s.Nlink == 1 && s.Size >= 0 && s.Size <= MaxDocumentBytes
}

func stableStat(a, b unix.Stat_t) bool {
	left, right := reflect.ValueOf(&a).Elem(), reflect.ValueOf(&b).Elem()
	for _, name := range []string{"Atim", "Atimespec", "Atime", "Atimensec"} {
		if f := left.FieldByName(name); f.IsValid() && f.CanSet() {
			f.Set(reflect.Zero(f.Type()))
		}
		if f := right.FieldByName(name); f.IsValid() && f.CanSet() {
			f.Set(reflect.Zero(f.Type()))
		}
	}
	return reflect.DeepEqual(a, b)
}
