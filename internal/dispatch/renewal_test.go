//go:build unix

package dispatch

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// startBackend records when each renewal was decided and stalls it until its
// context ends. A renewal's context times out renewBound after the decision,
// so the deadline gives the decision itself, not a start delayed by the
// host-file check that runs first.
type startBackend struct {
	ClaimBackend
	mu     sync.Mutex
	starts []time.Time
}

func (s *startBackend) AcquireLease(ctx context.Context, l comb.Lease) error {
	dl, ok := ctx.Deadline()
	if !ok { // the claim itself
		return s.ClaimBackend.AcquireLease(ctx, l)
	}
	s.mu.Lock()
	s.starts = append(s.starts, dl.Add(-renewBound))
	s.mu.Unlock()
	<-ctx.Done()
	return ctx.Err()
}

// D2-T10 (fable N21, opus confirmation 2 ruling 4): with a keepalive shorter
// than its stall, no renewal is decided once the session is over. The worker
// marks the moment it gets SIGTERM at the timeout (a shell builtin, so no exec
// delay); a decision after that, beyond the clock's granularity, is a renewal
// after the end. Without the check half of the ticks that follow the timeout
// start one, so a dozen runs catch its removal.
func TestNoRenewalStartsAfterSessionEnds(t *testing.T) {
	old := renewBound
	renewBound = 150 * time.Millisecond
	t.Cleanup(func() { renewBound = old })
	for range 12 {
		f := newFixture(t, false, false)
		b, _ := claimEnv(t, f.real)
		sb := &startBackend{ClaimBackend: b}
		ended := filepath.Join(t.TempDir(), "ended")
		_, err := f.run("trap 'printf x > "+ended+"; exit 0' TERM\nsleep 30 &\nwait\n", func(_ *Request, d *Deps) {
			d.Backend, d.KeepaliveEvery, d.Timeout = sb, 20*time.Millisecond, 400*time.Millisecond
		})
		fi, serr := os.Stat(ended)
		if err != nil || serr != nil {
			t.Fatalf("err %v, worker never got SIGTERM: %v", err, serr)
		}
		sb.mu.Lock()
		if len(sb.starts) < 2 {
			t.Fatalf("only %d renewals: the keepalive did not run", len(sb.starts))
		}
		for _, s := range sb.starts {
			if s.After(fi.ModTime().Add(20 * time.Millisecond)) {
				t.Fatalf("a renewal was decided %v after the worker ended", s.Sub(fi.ModTime()))
			}
		}
		sb.mu.Unlock()
	}
}
