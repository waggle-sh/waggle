// SPDX-License-Identifier: Apache-2.0

package dispatch

import (
	"context"
	"errors"
	"time"

	"github.com/waggle-sh/waggle/internal/forge"
)

// forgeLookup is the PRLookup a release builds from config. The forge is built
// on the first ask, so a release that never reaches the resolver's forge step
// never probes gh. Provider none, a forge without the lookup and no auth are
// ErrNoForge; any other trouble is a failed read.
type forgeLookup struct {
	provider string
	opt      forge.Options
	timeout  time.Duration  // 0: 30 seconds
	finder   forge.PRFinder // set by tests: skips building the forge
}

func (l forgeLookup) MergedPRByHead(ctx context.Context, branches []string) (MergedPR, bool, error) {
	if l.timeout == 0 {
		l.timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, l.timeout)
	defer cancel()
	pf := l.finder
	if pf == nil {
		f, err := forge.New(l.provider, l.opt)
		if errors.Is(err, forge.ErrNoForgeAuth) {
			return MergedPR{}, false, ErrNoForge
		} else if err != nil {
			return MergedPR{}, false, err
		}
		var ok bool
		if pf, ok = f.(forge.PRFinder); !ok {
			return MergedPR{}, false, ErrNoForge
		}
	}
	p, found, err := pf.MergedPRByHead(ctx, branches)
	return MergedPR(p), found, err
}
