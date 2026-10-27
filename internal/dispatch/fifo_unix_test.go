// SPDX-License-Identifier: Apache-2.0

//go:build unix && !solaris

package dispatch

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestTakeLockRefusesFifoAndSymlink(t *testing.T) {
	d := t.TempDir()
	fifo := filepath.Join(d, "fifo")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(d, "link")
	if err := os.Symlink(fifo, link); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{fifo, link} {
		done := make(chan error, 1)
		go func() { _, err := takeLock(p, t0); done <- err }()
		select {
		case err := <-done:
			if code(err) != ExitEnv || !strings.Contains(err.Error(), "unreadable") {
				t.Fatalf("%s: %v", p, err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("takeLock blocked on %s", p)
		}
	}
}
