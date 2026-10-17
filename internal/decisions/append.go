package decisions

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// maxFileBytes caps the log size Append and Next will read.
const maxFileBytes = 4 << 20

// Append validates in against the current file, assigns the next ID and
// today's UTC date from now, and atomically replaces path with the old
// content plus one entry. Writers are serialized by <path>.lock; a crash
// leaves either the old file or the new file, never a partial one.
func Append(path string, in Input, now time.Time) (Entry, error) {
	unlock, err := lock(path)
	if err != nil {
		return Entry{}, err
	}
	defer unlock()
	old, entries, err := load(path)
	if err != nil {
		return Entry{}, err
	}
	e, err := next(entries, in, now)
	if err != nil {
		return e, err
	}
	var buf bytes.Buffer
	if len(old) == 0 {
		buf.Write(Render([]Entry{e}))
	} else {
		buf.Write(old)
		buf.WriteByte('\n')
		buf.Write(RenderEntry(e))
	}
	if err := VerifyAppendOnly(old, buf.Bytes()); err != nil {
		return e, err
	}
	if err := writeAtomic(path, buf.Bytes()); err != nil {
		return e, err
	}
	return e, nil
}

// Next returns the entry Append would write, without writing or locking
// (for --dry-run).
func Next(path string, in Input, now time.Time) (Entry, error) {
	_, entries, err := load(path)
	if err != nil {
		return Entry{}, err
	}
	return next(entries, in, now)
}

// VerifyAppendOnly reports ErrNotAppendOnly unless next == old + exactly the
// rendered bytes of one or more new, valid, contiguous entries. A corrupt old
// is reported as ErrCorrupt.
func VerifyAppendOnly(old, next []byte) error {
	before, err := Parse(old)
	if err != nil {
		return err
	}
	if !bytes.HasPrefix(next, old) {
		return fmt.Errorf("%w: existing bytes were changed, reordered, or removed", ErrNotAppendOnly)
	}
	after, err := Parse(next)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrNotAppendOnly, err)
	}
	if len(after) <= len(before) {
		return fmt.Errorf("%w: no complete entry was added", ErrNotAppendOnly)
	}
	return nil
}

func next(entries []Entry, in Input, now time.Time) (Entry, error) {
	e, err := in.entry(len(entries))
	if err != nil {
		return e, err
	}
	e.ID = len(entries) + 1
	e.Date = now.UTC().Format("2006-01-02")
	if n := len(entries); n > 0 && e.Date < entries[n-1].Date {
		return e, fmt.Errorf("%w: date %s is earlier than the last entry's %s (clock went backwards)", ErrInvalidInput, e.Date, entries[n-1].Date)
	}
	return e, nil
}

// load reads and parses path. A missing file is zero entries.
func load(path string) ([]byte, []Entry, error) {
	fi, err := os.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%w: %s is not a regular file", ErrCorrupt, path)
	}
	if fi.Size() > maxFileBytes {
		return nil, nil, fmt.Errorf("%w: %s is over %d bytes", ErrCorrupt, path, maxFileBytes)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	entries, err := Parse(data)
	if err != nil {
		return nil, nil, err
	}
	return data, entries, nil
}

// lock creates <path>.lock exclusively. It is never broken automatically.
func lock(path string) (func(), error) {
	lp := path + ".lock"
	f, err := os.OpenFile(lp, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, fs.ErrExist) {
		return nil, fmt.Errorf("%w: %s exists; it is safe to remove only if no waggle decide is running", ErrLocked, lp)
	}
	if err != nil {
		return nil, err
	}
	fmt.Fprintf(f, "%d\n", os.Getpid())
	_ = f.Close()
	return func() { _ = os.Remove(lp) }, nil
}

// writeAtomic writes data to a temp file in path's directory, syncs it, and
// renames it onto path. The temp file is removed on any error.
func writeAtomic(path string, data []byte) (err error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".DECISIONS-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	defer func() {
		if err != nil {
			_ = tmp.Close()
			_ = os.Remove(name)
		}
	}()
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Chmod(name, 0o644); err != nil {
		return err
	}
	if err = os.Rename(name, path); err != nil {
		return err
	}
	// Best effort: some platforms and filesystems reject directory sync.
	if d, derr := os.Open(dir); derr == nil {
		_ = d.Sync()
		_ = d.Close()
	}
	return nil
}
