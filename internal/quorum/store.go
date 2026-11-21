// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/bits"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/layout"
)

// Reason codes added by the ballot store. Like the phase A codes they are
// fixed strings and never echo ref or ballot text.
const (
	ReasonRefName      = "ref_name"
	ReasonRefHash      = "ref_hash"
	ReasonSignature    = "signature"
	ReasonObject       = "object"
	ReasonPrevMismatch = "prev_mismatch"
	ReasonNoncanonical = "noncanonical"
)

// RefPrefix is the Quorum ref namespace (RFC-0006 6.1).
const RefPrefix = "refs/waggle/quorum/"

// Tree entry names of a ballot commit (RFC-0006 6.3).
const (
	BallotFile    = "ballot.json"
	BallotSigFile = "ballot.sig"
)

// maxBallotBytes caps ballot.json read back.
const maxBallotBytes = 64 << 10

// maxCastAttempts bounds how often Cast re-reads and retries a lost race.
const maxCastAttempts = 5

// baseOIDPattern is a full SHA-1 or SHA-256 object id, lowercase hex.
var baseOIDPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

var refPattern = regexp.MustCompile(`^refs/waggle/quorum/round-([1-9][0-9]{0,8})/(record|k-([0-9a-f]{64}))$`)

// RecordRef is the round commit chain ref of round r.
func RecordRef(r int) string { return fmt.Sprintf("%sround-%d/record", RefPrefix, r) }

// KeeperHash is the lowercase SHA-256 hex of the keeper_id bytes.
func KeeperHash(keeperID string) string {
	h := sha256.Sum256([]byte(keeperID))
	return hex.EncodeToString(h[:])
}

// BallotRef is the ballot chain ref of keeperID in round r.
func BallotRef(r int, keeperID string) string {
	return fmt.Sprintf("%sround-%d/k-%s", RefPrefix, r, KeeperHash(keeperID))
}

// RefKind says which of the two canonical forms a ref has.
type RefKind int

const (
	// RefOther is any ref under the namespace that is not canonical.
	RefOther RefKind = iota
	// RefRecord is refs/waggle/quorum/round-<r>/record.
	RefRecord
	// RefBallot is refs/waggle/quorum/round-<r>/k-<hash>.
	RefBallot
)

// ParseRef classifies a ref under the Quorum namespace. For RefBallot it also
// returns the hash text; a ref that is neither form is RefOther and is never
// parsed further.
func ParseRef(ref string) (kind RefKind, round int, hash string) {
	m := refPattern.FindStringSubmatch(ref)
	if m == nil {
		return RefOther, 0, ""
	}
	n, err := strconv.Atoi(m[1])
	if err != nil {
		return RefOther, 0, ""
	}
	if m[2] == "record" {
		return RefRecord, n, ""
	}
	return RefBallot, n, m[3]
}

// safeRefName returns ref for a report. A name with anything outside a plain
// ref alphabet is replaced, so a hostile ref name is never echoed.
var safeRefPattern = regexp.MustCompile(`^[A-Za-z0-9._/@+=-]{1,200}$`)

func safeRefName(ref string) string {
	if safeRefPattern.MatchString(ref) {
		return ref
	}
	return RefPrefix + "(invalid name)"
}

// Store is the ballot store: the git repository and the signing key.
type Store struct {
	// Comb is the backend over the repository; with a remote set, writes go
	// to the remote and reads fetch first.
	Comb *comb.GitBackend
	// Signer signs ballots.
	Signer Signer
	// Now is unused by ballot commits, whose dates come from the round's
	// opens_at. It is kept so existing callers still compile.
	Now func() time.Time

	// hookBeforeWrite, when set by tests, runs after Cast reads the tip and
	// before it writes, with the 0-based attempt number.
	hookBeforeWrite func(attempt int)
}

// ballotCredits returns sum of v^2 for the votes, or false when it cannot
// be held in a uint64.
func ballotCredits(votes []Vote) (uint64, bool) {
	var sum uint64
	for _, v := range votes {
		a := uint64(v.Votes)
		if v.Votes < 0 {
			a = uint64(-v.Votes)
		}
		hi, lo := bits.Mul64(a, a)
		var carry uint64
		sum, carry = bits.Add64(sum, lo, 0)
		if hi != 0 || carry != 0 {
			return 0, false
		}
	}
	return sum, true
}

// BallotMessage is the fixed commit message of a ballot commit.
func BallotMessage(round int) string { return fmt.Sprintf("quorum: round %d ballot", round) }

// ballotTime parses the round's opens_at, the date of every ballot commit.
func ballotTime(round Round) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, round.OpensAt)
	if err != nil || t.Unix() < 0 {
		return time.Time{}, errors.New("quorum: round opens_at is not a usable timestamp")
	}
	return t, nil
}

// SignedBallot is a signed ballot commit that is built but not yet on its
// ref. Receipt is the commit OID; it is a pure function of ballot.json,
// ballot.sig and Parent, because the message and both dates are fixed by the
// round.
type SignedBallot struct {
	Ref     string
	Receipt string
	// Parent is the commit the ballot chains to and the expected value of Ref
	// ("" for a first ballot).
	Parent string
}

// SignBallot validates the ballot before any git or ssh-keygen call, sets prev
// to the given parent, signs it and builds the ballot commit with the fixed
// message and opens_at as both dates. It does not write the ref; WriteBallot
// does. parent is the Keeper's current tip, or "" for a first ballot.
func (s Store) SignBallot(ctx context.Context, round Round, keeperID string, votes []Vote, parent string) (SignedBallot, error) {
	b, err := s.precheck(round, keeperID, votes)
	if err != nil {
		return SignedBallot{}, err
	}
	when, _ := ballotTime(round)
	b.Prev = parent
	data := Encode(b)
	sig, err := s.Signer.Sign(ctx, NamespaceBallot, data)
	if err != nil {
		return SignedBallot{}, err
	}
	ref := BallotRef(round.Round, keeperID)
	oid, err := s.Comb.BuildObjectCommit(ctx, ref, []comb.ObjectFile{
		{Name: BallotFile, Data: data}, {Name: BallotSigFile, Data: sig},
	}, parent, BallotMessage(round.Round), when)
	if err != nil {
		return SignedBallot{}, err
	}
	return SignedBallot{Ref: ref, Receipt: oid, Parent: parent}, nil
}

// WriteBallot points the ballot ref at the built commit by compare-and-swap:
// expected is the parent, or the ref must not exist for a first ballot. A lost
// race returns an error wrapping comb.ErrRefLost.
func (s Store) WriteBallot(ctx context.Context, sb SignedBallot) error {
	if s.Comb == nil {
		return errors.New("quorum: store needs a backend")
	}
	return s.Comb.UpdateObjectRef(ctx, sb.Ref, sb.Receipt, sb.Parent)
}

// Cast records keeperID's votes in round (RFC-0006 6.2, 6.3, 6.4). The ballot
// is validated before any git or ssh-keygen call. It chains to the Keeper's
// previous ballot with that OID as expected value (create-only for a first
// ballot), and on a lost race re-reads and retries on the new parent. The
// commit message and dates are fixed by the round, so Cast never reads a
// clock. It returns the ballot commit OID, the receipt.
func (s Store) Cast(ctx context.Context, round Round, keeperID string, votes []Vote) (string, error) {
	ref := BallotRef(round.Round, keeperID)
	for attempt := 0; ; attempt++ {
		// Validation errors surface before the first git call.
		if attempt == 0 {
			if _, err := s.precheck(round, keeperID, votes); err != nil {
				return "", err
			}
		}
		if err := s.Comb.FetchObjectNamespace(ctx, RefPrefix); err != nil {
			return "", err
		}
		tip, err := s.Comb.ReadObjectRef(ctx, ref)
		if err != nil {
			return "", err
		}
		sb, err := s.SignBallot(ctx, round, keeperID, votes, tip)
		if err != nil {
			return "", err
		}
		if s.hookBeforeWrite != nil {
			s.hookBeforeWrite(attempt)
		}
		err = s.WriteBallot(ctx, sb)
		if err == nil {
			return sb.Receipt, nil
		}
		if !errors.Is(err, comb.ErrRefLost) || attempt+1 >= maxCastAttempts {
			return "", err
		}
	}
}

// precheck runs the validation SignBallot runs, plus the backend check, with
// no git or ssh-keygen call.
func (s Store) precheck(round Round, keeperID string, votes []Vote) (Ballot, error) {
	b := Ballot{Version: Version, Round: round.Round, KeeperID: keeperID,
		Votes: append([]Vote{}, votes...)}
	sum, ok := ballotCredits(b.Votes)
	if !ok {
		return b, reason(ReasonOverBudget)
	}
	b.CreditsUsed = sum
	if err := ValidateBallot(round, b); err != nil {
		return b, err
	}
	if _, err := ballotTime(round); err != nil {
		return b, err
	}
	if s.Comb == nil {
		return b, errors.New("quorum: store needs a backend")
	}
	return b, nil
}

// LoadVerifier reads the Keeper registry from the round's base commit, never
// from the worktree, and returns a Verifier over it. base must be a full 40
// or 64 hex commit id; a ref name such as HEAD or a branch would move and is
// refused.
func LoadVerifier(ctx context.Context, g *comb.GitBackend, base, sshKeygen string) (*Verifier, error) {
	if !baseOIDPattern.MatchString(base) {
		return nil, errors.New("quorum: round base is not a full commit id")
	}
	data, err := g.ReadBlobAt(ctx, base, layout.Keepers, MaxRegistryBytes)
	if err != nil {
		return nil, fmt.Errorf("quorum: read keeper registry at base: %w", err)
	}
	return NewVerifier(data, sshKeygen)
}

// CountedBallot is a ballot that passed every check at its chain tip.
type CountedBallot struct {
	Ref    string
	OID    string
	Ballot Ballot
}

// Loaded is the result of LoadBallots.
type Loaded struct {
	// Counted is sorted by keeper_id.
	Counted []CountedBallot
	// Malformed lists what was not counted, by ref and fixed reason code.
	Malformed []Malformed
}

// HasReceipt reports whether oid is the commit of a counted ballot. A voter
// whose receipt is missing re-pushes the ballot before close (RFC-0006 6.4).
func (l Loaded) HasReceipt(oid string) bool {
	for _, c := range l.Counted {
		if c.OID == oid {
			return true
		}
	}
	return false
}

// LoadBallots fetches the namespace, lists the ballot refs of round, and for
// each tip checks the tree shape, the canonical bytes, the ref hash, the
// signature against v and ValidateBallot. Only the tip of a chain counts. A
// ref under the namespace that is not canonical is reported and never parsed;
// a canonical ref of another round is ignored. The error is for failures to
// run git or ssh-keygen, not for bad ballots.
func (s Store) LoadBallots(ctx context.Context, round Round, v *Verifier) (Loaded, error) {
	if s.Comb == nil || v == nil {
		return Loaded{}, errors.New("quorum: store needs a backend and a verifier")
	}
	if err := s.Comb.FetchObjectNamespace(ctx, RefPrefix); err != nil {
		return Loaded{}, err
	}
	refs, err := s.Comb.ListObjectRefs(ctx, RefPrefix)
	if err != nil {
		return Loaded{}, err
	}
	out := Loaded{Counted: []CountedBallot{}, Malformed: []Malformed{}}
	for _, r := range refs {
		kind, n, hash := ParseRef(r.Ref)
		name := safeRefName(r.Ref)
		switch {
		case kind == RefOther:
			out.Malformed = append(out.Malformed, Malformed{Ref: name, Reason: ReasonRefName})
			continue
		case kind == RefRecord || n != round.Round:
			continue
		}
		cb, code, err := s.loadOne(ctx, round, v, r, hash)
		if err != nil {
			return Loaded{}, err
		}
		if code != "" {
			out.Malformed = append(out.Malformed, Malformed{Ref: name, Reason: code})
			continue
		}
		out.Counted = append(out.Counted, cb)
	}
	sort.Slice(out.Counted, func(i, j int) bool { return out.Counted[i].Ballot.KeeperID < out.Counted[j].Ballot.KeeperID })
	sort.Slice(out.Malformed, func(i, j int) bool {
		if out.Malformed[i].Ref != out.Malformed[j].Ref {
			return out.Malformed[i].Ref < out.Malformed[j].Ref
		}
		return out.Malformed[i].Reason < out.Malformed[j].Reason
	})
	return out, nil
}

// loadOne checks one ballot ref tip. It returns a reason code for a bad
// ballot and an error only when git or ssh-keygen could not run.
func (s Store) loadOne(ctx context.Context, round Round, v *Verifier, r comb.ObjectRef, hash string) (CountedBallot, string, error) {
	if r.Type != "commit" {
		return CountedBallot{}, ReasonObject, nil
	}
	c, err := s.Comb.ReadObjectCommit(ctx, r.OID, maxBallotBytes)
	if err != nil {
		if errors.Is(err, comb.ErrBadObject) {
			return CountedBallot{}, ReasonObject, nil
		}
		return CountedBallot{}, "", err
	}
	data, sig := c.Files[BallotFile], c.Files[BallotSigFile]
	if len(c.Files) != 2 || data == nil || sig == nil {
		return CountedBallot{}, ReasonObject, nil
	}
	if len(c.Parents) > 1 {
		return CountedBallot{}, ReasonPrevMismatch, nil
	}
	b, err := DecodeBallot(data)
	if err != nil {
		return CountedBallot{}, ReasonOf(err), nil
	}
	if !bytes.Equal(Encode(b), data) {
		return CountedBallot{}, ReasonNoncanonical, nil
	}
	if KeeperHash(b.KeeperID) != hash {
		return CountedBallot{}, ReasonRefHash, nil
	}
	if err := v.Verify(ctx, NamespaceBallot, b.KeeperID, data, sig); err != nil {
		if errors.Is(err, ErrBadSignature) {
			return CountedBallot{}, ReasonSignature, nil
		}
		return CountedBallot{}, "", err
	}
	if err := ValidateBallot(round, b); err != nil {
		return CountedBallot{}, ReasonOf(err), nil
	}
	wantPrev := ""
	if len(c.Parents) == 1 {
		wantPrev = c.Parents[0]
	}
	if b.Prev != wantPrev {
		return CountedBallot{}, ReasonPrevMismatch, nil
	}
	return CountedBallot{Ref: r.Ref, OID: r.OID, Ballot: b}, "", nil
}
