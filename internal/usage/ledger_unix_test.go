//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package usage

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// N2: Read and Append refuse symlinks and non-regular files without hanging.
func TestLedgerRefusesSymlinkAndFIFO(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real.jsonl")
	if err := os.WriteFile(real, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.jsonl")
	dangling := filepath.Join(dir, "dangling.jsonl")
	target := filepath.Join(dir, "target")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, dangling); err != nil {
		t.Fatal(err)
	}
	fifo := filepath.Join(dir, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{link, dangling, fifo} {
		done := make(chan struct{})
		go func() {
			defer close(done)
			if _, _, err := Read(p); err == nil {
				t.Errorf("Read %s: want an error", filepath.Base(p))
			}
			if err := Append(p, Record{Ticket: "T-001"}); err == nil {
				t.Errorf("Append %s: want an error", filepath.Base(p))
			}
		}()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: hung", filepath.Base(p))
		}
	}
	if _, err := os.Lstat(target); err == nil {
		t.Error("Append created the symlink target")
	}
	if fi, _ := os.Stat(real); fi.Size() != 0 {
		t.Error("Append wrote through a symlink")
	}
}
