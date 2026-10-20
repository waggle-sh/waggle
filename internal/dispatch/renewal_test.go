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

// startBackend records the start of every claim write and stalls each renewal
// until its context ends.
type startBackend struct {
	ClaimBackend
	mu     sync.Mutex
	starts []time.Time
}

func (s *startBackend) AcquireLease(ctx context.Context, l comb.Lease) error {
	s.mu.Lock()
	n := len(s.starts)
	s.starts = append(s.starts, time.Now())
	s.mu.Unlock()
	if n == 0 {
		return s.ClaimBackend.AcquireLease(ctx, l)
	}
	<-ctx.Done()
	return ctx.Err()
}

// D2-T10 (fable N21, opus confirmation 2 ruling 4): with a keepalive shorter
// than its stall, no renewal starts once the session is over. The worker marks
// the moment it gets SIGTERM at the timeout; a start later than that, beyond a
// slack for a tick already inside the check, is a renewal after the end.
func TestNoRenewalStartsAfterSessionEnds(t *testing.T) {
	old := renewBound
	renewBound = 150 * time.Millisecond
	t.Cleanup(func() { renewBound = old })
	for range 6 {
		f := newFixture(t, false, false)
		b, _ := claimEnv(t, f.real)
		sb := &startBackend{ClaimBackend: b}
		ended := filepath.Join(t.TempDir(), "ended")
		_, err := f.run("trap 'touch "+ended+"; exit 0' TERM\nsleep 30 &\nwait\n", func(_ *Request, d *Deps) {
			d.Backend, d.KeepaliveEvery, d.Timeout = sb, 20*time.Millisecond, 600*time.Millisecond
		})
		fi, serr := os.Stat(ended)
		if err != nil || serr != nil {
			t.Fatalf("err %v, worker never got SIGTERM: %v", err, serr)
		}
		sb.mu.Lock()
		if len(sb.starts) < 3 {
			t.Fatalf("only %d claim writes: no renewal ran", len(sb.starts))
		}
		for _, s := range sb.starts {
			if s.After(fi.ModTime().Add(100 * time.Millisecond)) {
				t.Fatalf("a renewal started %v after the worker ended", s.Sub(fi.ModTime()))
			}
		}
		sb.mu.Unlock()
	}
}
