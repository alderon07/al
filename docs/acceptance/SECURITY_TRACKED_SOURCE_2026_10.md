# Tracked source observation acceptance

- Read enrolled source bytes only from a regular file opened through descriptor anchored, trusted parent directories. Validate each symlink target before opening it and preserve safe owned dotfile symlinks.
- Reject credential shaped paths, private application files, foreign ownership, writable parent directories, nonregular files, and unsupported platforms before reading source bytes.
- Bind publication and backup bytes to the validated file identity and fail when the source changes between observation and publication.
- Preserve missing source waiting and conflict behavior and bounded reads.
- Verify hostile substitutions, unsafe parents, owned symlinks, source changes, and existing tracked workflows in temporary directories. Run required repository checks and actual PTY verification.
- Validate persisted registry shape independently of current source trust. A source that becomes unsafe must remain removable with `al untrack FILE`, and ordinary settings must still load; enrollment and every source read enforce trust separately.
- macOS tracked source observation must prove descriptor based native ACL safety for each directory and file. Confirm filesystem ACL metadata support, reject ACL write grants and malformed or unavailable metadata, and preserve deny and read only ACLs. POSIX permission bits and xattr absence do not prove Darwin ACL safety.
- Verify invalidated source removal and a compiled CLI flow through an actual PTY. Cross compile Darwin and explicitly retain the native ACL verification release gate.

Native macOS runtime verification remains required in macOS CI: `TestTrackedDarwinNativeACLBoundary` adds file write and parent add/delete ACL grants, verifies rejection, and checks ordinary and deny ACL controls. Linux decoder tests and Darwin cross compilation do not establish native execution correctness.

The native attribute format and ACL handling follow Apple's [getattrlist manual](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/man/man2/getattrlist.2), [ACL definitions](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/sys/kauth.h), and [attribute packing implementation](https://github.com/apple-oss-distributions/xnu/blob/main/bsd/vfs/vfs_attrlist.c). Volume attribute support is checked separately because an empty security attribute alone cannot distinguish absent from unsupported metadata.
