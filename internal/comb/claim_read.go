// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"fmt"
)

// ReadClaim returns the lease stored at refs/waggle/claims/ticket-<n> in the
// local repo, including expired claims. It re-validates the claim exactly as
// ListActiveLeases does (T-009 section 3.2) and returns normalized directories.
//
// It reads local refs only and never syncs, even in remote mode: the merge
// gate fetches claim refs before it runs. A missing ref returns ErrNotFound;
// a claim that fails decoding or validation returns ErrMalformedClaim with the
// reason. An invalid ticket ID returns ErrInvalidTicketID.
func (b *GitBackend) ReadClaim(ctx context.Context, ticketID string) (Lease, error) {
	ref, err := RefName(ticketID)
	if err != nil {
		return Lease{}, err
	}
	refs, err := b.listRefs(ctx, ref)
	if err != nil {
		return Lease{}, err
	}
	for _, r := range refs {
		if r.ref != ref {
			continue
		}
		claims, err := b.loadClaims(ctx, []refEntry{r})
		if err != nil {
			return Lease{}, err
		}
		if c := claims[0]; c.reason != "" {
			return Lease{}, fmt.Errorf("%s: %w: %s", ref, ErrMalformedClaim, c.reason)
		}
		return claims[0].lease, nil
	}
	return Lease{}, fmt.Errorf("%s: %w", ref, ErrNotFound)
}
