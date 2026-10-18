package usage

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/waggle-sh/waggle/internal/comb"
)

// maxLine is the per-line buffer of Read.
const maxLine = 1 << 20

// openRegular opens path without following a symlink and refuses anything
// but a regular file (a FIFO or device would block or misbehave). A symlink,
// dangling or not, fails and creates nothing.
func openRegular(path string, flag int, perm os.FileMode) (*os.File, error) {
	if fi, err := os.Lstat(path); err == nil && !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	f, err := os.OpenFile(path, flag|openFlags, perm)
	if err != nil {
		return nil, err
	}
	if fi, err := f.Stat(); err != nil || !fi.Mode().IsRegular() {
		f.Close()
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	return f, nil
}

// Append writes r as one JSON line to the ledger at path. The file is
// created with mode 0600 if absent; the directory must already exist, and
// Append never creates one. The line goes out in a single O_APPEND write.
func Append(path string, r Record) error {
	line, err := json.Marshal(r)
	if err != nil {
		return fmt.Errorf("usage: encode record: %w", err)
	}
	f, err := openRegular(path, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o600)
	if err != nil {
		return fmt.Errorf("usage: open ledger: %w", err)
	}
	if _, err := f.Write(append(line, '\n')); err != nil {
		f.Close()
		return fmt.Errorf("usage: append to %s: %w", path, err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("usage: close %s: %w", path, err)
	}
	return nil
}

// Read loads the ledger at path. An absent file yields nil, nil, nil.
// Malformed lines and lines whose ticket fails comb.ParseTicketID are skipped
// and reported as BadLine; they are never fatal. Blank lines are ignored.
// Only an I/O error (or a line of 1 MiB or more, newline excluded) is
// returned as the error. A symlink or non-regular file is refused.
func Read(path string) ([]Record, []BadLine, error) {
	f, err := openRegular(path, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("usage: open ledger: %w", err)
	}
	defer f.Close()

	var recs []Record
	var bad []BadLine
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), maxLine)
	for n := 1; sc.Scan(); n++ {
		if len(sc.Bytes()) == 0 {
			continue
		}
		var r Record
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			bad = append(bad, BadLine{n, err})
			continue
		}
		if _, err := comb.ParseTicketID(r.Ticket); err != nil {
			bad = append(bad, BadLine{n, fmt.Errorf("ticket %q: %w", r.Ticket, err)})
			continue
		}
		recs = append(recs, r)
	}
	if err := sc.Err(); err != nil {
		return nil, nil, fmt.Errorf("usage: read %s: %w", path, err)
	}
	return recs, bad, nil
}
