// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"fmt"
	"strings"
)

// Relay trailer keys allowed on a claim commit (T-065 section 2.1 item 2).
const (
	// TrailerRelayRequest is the URL of the request comment.
	TrailerRelayRequest = "Waggle-Relay-Request"
	// TrailerRelayRequester is the login of the contributor who asked.
	TrailerRelayRequester = "Waggle-Relay-Requester"
	// TrailerRelayApprovedBy is the approver's login, or auto:<association>,
	// or auto:nectar.
	TrailerRelayApprovedBy = "Waggle-Relay-Approved-By"
)

// maxTrailerValue is the longest allowed trailer value in bytes.
const maxTrailerValue = 256

// Trailer is one Key: value line appended to a claim commit message. The
// trailers are a record of who asked and who approved, not a signature.
type Trailer struct {
	Key   string
	Value string
}

// formatTrailers validates trailers and returns the text to append to the
// claim commit message: a blank line, then one Key: value line each. No
// trailers return the empty string. Errors wrap ErrInvalidLease and never
// carry the value.
//
// Keys must be one of the Waggle-Relay-* keys, each at most once. A value is
// printable ASCII with no newline, at most 256 bytes. Requester and approver
// values are logins (the keeper id pattern) or, for the approver, auto:<word>.
func formatTrailers(trailers []Trailer) (string, error) {
	if len(trailers) == 0 {
		return "", nil
	}
	seen := map[string]bool{}
	var sb strings.Builder
	sb.WriteString("\n\n")
	for i, t := range trailers {
		switch t.Key {
		case TrailerRelayRequest, TrailerRelayRequester, TrailerRelayApprovedBy:
		default:
			return "", fmt.Errorf("trailer %d: key not allowed: %w", i+1, ErrInvalidLease)
		}
		if seen[t.Key] {
			return "", fmt.Errorf("trailer %d: duplicate key %s: %w", i+1, t.Key, ErrInvalidLease)
		}
		seen[t.Key] = true
		if t.Value == "" || len(t.Value) > maxTrailerValue {
			return "", fmt.Errorf("trailer %s: value empty or over %d bytes: %w", t.Key, maxTrailerValue, ErrInvalidLease)
		}
		for j := 0; j < len(t.Value); j++ {
			if c := t.Value[j]; c < 0x20 || c > 0x7e {
				return "", fmt.Errorf("trailer %s: value is not printable ASCII on one line: %w", t.Key, ErrInvalidLease)
			}
		}
		switch t.Key {
		case TrailerRelayRequester:
			if !keeperIDPattern.MatchString(t.Value) {
				return "", fmt.Errorf("trailer %s: value is not a login: %w", t.Key, ErrInvalidLease)
			}
		case TrailerRelayApprovedBy:
			login := strings.TrimPrefix(t.Value, "auto:")
			if !keeperIDPattern.MatchString(login) {
				return "", fmt.Errorf("trailer %s: value is not a login: %w", t.Key, ErrInvalidLease)
			}
		}
		sb.WriteString(t.Key + ": " + t.Value + "\n")
	}
	return sb.String(), nil
}

// RelayRequester returns the login in the Waggle-Relay-Requester trailer of
// the newest commit in the ticket's claim chain that carries one, or "" when
// no commit does. A claim that is not there returns ErrNotFound. In remote
// mode it syncs first. The relay policy uses it to tell relay claims from
// maintainer claims: a later renewal by an older CLI does not hide the trailer.
func (b *GitBackend) RelayRequester(ctx context.Context, ticketID string) (string, error) {
	oid, err := b.ClaimOID(ctx, ticketID)
	if err != nil {
		return "", err
	}
	out, err := b.git(ctx, nil, nil, "log", "--first-parent", "-z", "--format=%B", oid)
	if err != nil {
		return "", err
	}
	for _, msg := range strings.Split(string(out), "\x00") {
		if v := trailerValue(msg, TrailerRelayRequester); v != "" {
			return v, nil
		}
	}
	return "", nil
}

// trailerValue returns the value of the last line "key: value" in the final
// paragraph of msg, or "".
func trailerValue(msg, key string) string {
	msg = strings.TrimRight(msg, "\n")
	if i := strings.LastIndex(msg, "\n\n"); i >= 0 {
		msg = msg[i+2:]
	} else {
		return ""
	}
	prefix := key + ": "
	val := ""
	for _, line := range strings.Split(msg, "\n") {
		if strings.HasPrefix(line, prefix) {
			val = strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return val
}
