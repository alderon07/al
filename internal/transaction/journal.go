package transaction

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

var journalMagic = [8]byte{'A', 'L', 'T', 'X', 'N', '1', 0, '\n'}

type frameEnvelope struct {
	Kind     string    `json:"kind"`
	Manifest *Manifest `json:"manifest,omitempty"`
	Record   *Record   `json:"record,omitempty"`
}

// Journal owns an open append descriptor after validating all existing
// checksummed frames. Journal is not safe for concurrent use.
type Journal struct {
	root          string
	path          string
	file          *os.File
	manifest      Manifest
	records       []Record
	truncatedTail bool
	boundaryHook  func(Boundary) error
}

func validateManifest(manifest Manifest) error {
	if manifest.SchemaVersion != SchemaVersion {
		return fmt.Errorf("journal schema version must be %d", SchemaVersion)
	}
	if !operationIDPattern.MatchString(manifest.OperationID) {
		return fmt.Errorf("operation ID is invalid")
	}
	root, err := validatePrivateRoot(manifest.PrivateRoot)
	if err != nil {
		return err
	}
	if root != manifest.PrivateRoot {
		return fmt.Errorf("private root must be absolute and clean")
	}
	if len(manifest.Actions) == 0 || len(manifest.Actions) > MaxActions {
		return fmt.Errorf("journal needs between 1 and %d actions", MaxActions)
	}
	seenSequence := make(map[int]bool, len(manifest.Actions))
	seenPath := make(map[string]bool, len(manifest.Actions)*3)
	for index, action := range manifest.Actions {
		if action.Sequence != index+1 || seenSequence[action.Sequence] {
			return fmt.Errorf("action sequences must be contiguous from 1")
		}
		seenSequence[action.Sequence] = true
		if action.Mode != 0o600 {
			return fmt.Errorf("action %d mode must be 0600", action.Sequence)
		}
		if action.Remove && !action.Expected.Exists {
			return fmt.Errorf("action %d cannot remove an absent target", action.Sequence)
		}
		if err := validateHash(action.PlannedSHA256, "planned_sha256"); err != nil {
			return fmt.Errorf("action %d: %w", action.Sequence, err)
		}
		if action.Expected.Exists {
			if err := validateHash(action.Expected.SHA256, "expected.sha256"); err != nil {
				return fmt.Errorf("action %d: %w", action.Sequence, err)
			}
			if action.Expected.Mode != 0o600 || action.Expected.Links > 1 {
				return fmt.Errorf("action %d expected identity is not private", action.Sequence)
			}
			if action.BackupPath == "" {
				return fmt.Errorf("action %d needs a backup path", action.Sequence)
			}
		} else if action.BackupPath != "" {
			return fmt.Errorf("action %d cannot back up an absent target", action.Sequence)
		}
		for role, path := range map[string]string{
			"target":    action.TargetPath,
			"temporary": action.TemporaryPath,
			"backup":    action.BackupPath,
		} {
			if path == "" && role == "backup" {
				continue
			}
			absolute, err := validatePath(root, path, true)
			if err != nil {
				return fmt.Errorf("action %d %s path: %w", action.Sequence, role, err)
			}
			if absolute != path {
				return fmt.Errorf("action %d %s path must be absolute and clean", action.Sequence, role)
			}
			if seenPath[path] {
				return fmt.Errorf("action %d reuses a transaction path", action.Sequence)
			}
			seenPath[path] = true
		}
	}
	return nil
}

// CreateJournal creates and fsyncs a new journal containing only its immutable
// manifest. The parent directory is fsynced before the function returns.
func CreateJournal(root, path string, manifest Manifest) (*Journal, error) {
	root, err := validatePrivateRoot(root)
	if err != nil {
		return nil, err
	}
	if manifest.PrivateRoot != root {
		return nil, fmt.Errorf("manifest private root does not match journal root")
	}
	if err := validateManifest(manifest); err != nil {
		return nil, err
	}
	path, err = validatePath(root, path, true)
	if err != nil {
		return nil, err
	}
	file, err := createPrivateNoFollow(path)
	if err != nil {
		return nil, fmt.Errorf("create transaction journal: %w", err)
	}
	journal := &Journal{root: root, path: path, file: file, manifest: manifest}
	fail := func(cause error) (*Journal, error) {
		_ = file.Close()
		return nil, cause
	}
	if err := writeAll(file, journalMagic[:]); err != nil {
		return fail(fmt.Errorf("write transaction journal magic: %w", err))
	}
	if err := journal.appendEnvelope(frameEnvelope{Kind: "manifest", Manifest: &manifest}); err != nil {
		return fail(err)
	}
	if err := syncDirectory(filepath.Dir(path)); err != nil {
		return fail(fmt.Errorf("sync transaction journal directory: %w", err))
	}
	if err := journal.callBoundary(BoundaryManifestSynced); err != nil {
		return fail(err)
	}
	return journal, nil
}

// OpenJournal verifies a journal and opens it for appending. A truncated final
// frame is ignored. Any other framing, checksum, schema, or ordering error
// blocks recovery.
func OpenJournal(root, path string) (*Journal, error) {
	manifest, records, truncated, err := ReadJournal(root, path)
	if err != nil {
		return nil, err
	}
	path, err = validatePath(root, path, false)
	if err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0)
	if err != nil {
		return nil, fmt.Errorf("open transaction journal for append: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	identity, err := identityFromInfo(info)
	if err != nil {
		file.Close()
		return nil, err
	}
	if err := validatePrivateFileInfo(info, identity, MaxJournalBytes); err != nil {
		file.Close()
		return nil, err
	}
	return &Journal{root: manifest.PrivateRoot, path: path, file: file, manifest: manifest, records: records, truncatedTail: truncated}, nil
}

func ReadJournal(root, path string) (Manifest, []Record, bool, error) {
	validatedRoot, err := validatePrivateRoot(root)
	if err != nil {
		return Manifest{}, nil, false, err
	}
	identity, err := InspectPrivateFile(root, path, MaxJournalBytes)
	if err != nil {
		return Manifest{}, nil, false, err
	}
	if !identity.Exists {
		return Manifest{}, nil, false, os.ErrNotExist
	}
	file, err := openRegularNoFollow(path)
	if err != nil {
		return Manifest{}, nil, false, err
	}
	defer file.Close()
	reader := bufio.NewReader(io.LimitReader(file, MaxJournalBytes+1))
	magic := make([]byte, len(journalMagic))
	if _, err := io.ReadFull(reader, magic); err != nil || !bytes.Equal(magic, journalMagic[:]) {
		return Manifest{}, nil, false, ErrJournalCorrupt
	}

	var manifest Manifest
	var records []Record
	truncated := false
	frameIndex := 0
	for {
		envelope, tail, done, err := readEnvelope(reader)
		if err != nil {
			return Manifest{}, nil, false, err
		}
		if tail {
			truncated = true
			break
		}
		if done {
			break
		}
		if frameIndex == 0 {
			if envelope.Kind != "manifest" || envelope.Manifest == nil || envelope.Record != nil {
				return Manifest{}, nil, false, ErrJournalCorrupt
			}
			manifest = *envelope.Manifest
			if err := validateManifest(manifest); err != nil {
				return Manifest{}, nil, false, fmt.Errorf("%w: %v", ErrJournalCorrupt, err)
			}
			manifestRoot, rootErr := filepath.Abs(manifest.PrivateRoot)
			if rootErr != nil || filepath.Clean(manifestRoot) != validatedRoot {
				return Manifest{}, nil, false, fmt.Errorf("%w: manifest private root does not match the requested root", ErrJournalCorrupt)
			}
		} else {
			if envelope.Kind != "record" || envelope.Record == nil || envelope.Manifest != nil {
				return Manifest{}, nil, false, ErrJournalCorrupt
			}
			records = append(records, *envelope.Record)
		}
		frameIndex++
	}
	if frameIndex == 0 {
		return Manifest{}, nil, false, ErrJournalCorrupt
	}
	if err := validateRecords(manifest, records); err != nil {
		return Manifest{}, nil, false, err
	}
	return manifest, records, truncated, nil
}

func readEnvelope(reader *bufio.Reader) (frameEnvelope, bool, bool, error) {
	header := make([]byte, 8)
	read, err := io.ReadFull(reader, header)
	if errors.Is(err, io.EOF) && read == 0 {
		return frameEnvelope{}, false, true, nil
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return frameEnvelope{}, true, false, nil
	}
	if err != nil {
		return frameEnvelope{}, false, false, err
	}
	length := binary.BigEndian.Uint64(header)
	if length == 0 || length > MaxFrameBytes {
		return frameEnvelope{}, false, false, ErrJournalCorrupt
	}
	payload := make([]byte, int(length))
	if _, err := io.ReadFull(reader, payload); errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return frameEnvelope{}, true, false, nil
	} else if err != nil {
		return frameEnvelope{}, false, false, err
	}
	checksum := make([]byte, sha256.Size)
	if _, err := io.ReadFull(reader, checksum); errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, io.EOF) {
		return frameEnvelope{}, true, false, nil
	} else if err != nil {
		return frameEnvelope{}, false, false, err
	}
	digest := sha256.Sum256(payload)
	if !bytes.Equal(checksum, digest[:]) {
		return frameEnvelope{}, false, false, ErrJournalCorrupt
	}
	var envelope frameEnvelope
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&envelope); err != nil {
		return frameEnvelope{}, false, false, ErrJournalCorrupt
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		return frameEnvelope{}, false, false, ErrJournalCorrupt
	}
	return envelope, false, false, nil
}

func validateRecords(manifest Manifest, records []Record) error {
	validSequences := make(map[int]bool, len(manifest.Actions))
	for _, action := range manifest.Actions {
		validSequences[action.Sequence] = true
	}
	committed := false
	lastRank := make(map[int]int)
	for index, record := range records {
		if record.Ordinal != index+1 || committed {
			return ErrJournalCorrupt
		}
		if record.Kind == RecordCommitted {
			if record.Sequence != 0 || record.TargetSHA256 != "" {
				return ErrJournalCorrupt
			}
			committed = true
			continue
		}
		if !validSequences[record.Sequence] {
			return ErrJournalCorrupt
		}
		rank := map[RecordKind]int{
			RecordBackupSynced:  1,
			RecordTempSynced:    2,
			RecordTargetRenamed: 3,
			RecordTargetSynced:  4,
		}[record.Kind]
		if rank == 0 || rank <= lastRank[record.Sequence] {
			return ErrJournalCorrupt
		}
		if record.Kind == RecordTargetRenamed || record.Kind == RecordTargetSynced {
			if err := validateHash(record.TargetSHA256, "record target hash"); err != nil {
				return ErrJournalCorrupt
			}
		} else if record.TargetSHA256 != "" {
			return ErrJournalCorrupt
		}
		lastRank[record.Sequence] = rank
	}
	return nil
}

func (journal *Journal) appendRecord(record Record) error {
	if journal == nil || journal.file == nil {
		return fmt.Errorf("transaction journal is closed")
	}
	record.Ordinal = len(journal.records) + 1
	candidate := append(append([]Record(nil), journal.records...), record)
	if err := validateRecords(journal.manifest, candidate); err != nil {
		return err
	}
	if err := journal.appendEnvelope(frameEnvelope{Kind: "record", Record: &record}); err != nil {
		return err
	}
	journal.records = candidate
	return nil
}

func (journal *Journal) appendEnvelope(envelope frameEnvelope) error {
	payload, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	if len(payload) == 0 || len(payload) > MaxFrameBytes {
		return fmt.Errorf("journal frame exceeds %d bytes", MaxFrameBytes)
	}
	var header [8]byte
	binary.BigEndian.PutUint64(header[:], uint64(len(payload)))
	digest := sha256.Sum256(payload)
	if err := writeAll(journal.file, header[:]); err != nil {
		return fmt.Errorf("write journal frame header: %w", err)
	}
	if err := writeAll(journal.file, payload); err != nil {
		return fmt.Errorf("write journal frame: %w", err)
	}
	if err := writeAll(journal.file, digest[:]); err != nil {
		return fmt.Errorf("write journal checksum: %w", err)
	}
	if err := journal.file.Sync(); err != nil {
		return fmt.Errorf("sync transaction journal: %w", err)
	}
	return nil
}

func (journal *Journal) action(sequence int) (Action, error) {
	index := sort.Search(len(journal.manifest.Actions), func(index int) bool {
		return journal.manifest.Actions[index].Sequence >= sequence
	})
	if index >= len(journal.manifest.Actions) || journal.manifest.Actions[index].Sequence != sequence {
		return Action{}, fmt.Errorf("transaction action %d does not exist", sequence)
	}
	return journal.manifest.Actions[index], nil
}

func (journal *Journal) callBoundary(boundary Boundary) error {
	if journal.boundaryHook == nil {
		return nil
	}
	return journal.boundaryHook(boundary)
}

func (journal *Journal) Close() error {
	if journal == nil || journal.file == nil {
		return nil
	}
	file := journal.file
	journal.file = nil
	return file.Close()
}

func (journal *Journal) Manifest() Manifest {
	return journal.manifest
}

func (journal *Journal) Records() []Record {
	return append([]Record(nil), journal.records...)
}

// Finalize removes only a committed journal and unused operation-owned
// temporary files. Backups remain available for rollback.
func (journal *Journal) Finalize() error {
	if journal == nil {
		return nil
	}
	committed := false
	for _, record := range journal.records {
		committed = committed || record.Kind == RecordCommitted
	}
	if !committed {
		return fmt.Errorf("cannot finalize an uncommitted transaction")
	}
	if err := journal.Close(); err != nil {
		return err
	}
	for _, action := range journal.manifest.Actions {
		if err := os.Remove(action.TemporaryPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	if err := os.Remove(journal.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return syncDirectory(filepath.Dir(journal.path))
}
