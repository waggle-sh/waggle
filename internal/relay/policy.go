// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
)

// Kind is the outcome of a policy decision.
type Kind int

// Decision kinds.
const (
	// KindRefuse: reply with the reason, write nothing.
	KindRefuse Kind = iota
	// KindPending: reply with the digest, write nothing until an approval.
	KindPending
	// KindWrite: the caller may write (a claim) or delete (a release).
	KindWrite
)

// Decision is what policy decided. Only the fields of its Kind are set.
type Decision struct {
	Kind Kind
	// Reason is fixed text for KindRefuse.
	Reason string
	// Digest is the approval digest for KindPending.
	Digest string
	// Claim is what to write for a claim at KindWrite.
	Claim Claim
}

// Claim is a checked claim request, ready for dispatch.ClaimTask. The checks
// `waggle claim` makes (Tier 3, .waggle/, protected paths, overlap, held by
// another owner) are not repeated here; ClaimTask and AcquireLease make them.
type Claim struct {
	Task       string
	Owner      string // always the commenter, as GitHub reports it
	Dirs       []string
	Gate       string
	Branches   []string
	TTLMinutes int
	// ApprovedBy is the approver's login, auto:<association> or auto:renewal.
	ApprovedBy string
}

// Trailers returns the commit trailers for a claim written from the request
// comment at requestURL by requester.
func (c Claim) Trailers(requestURL, requester string) []comb.Trailer {
	return []comb.Trailer{
		{Key: comb.TrailerRelayRequest, Value: requestURL},
		{Key: comb.TrailerRelayRequester, Value: requester},
		{Key: comb.TrailerRelayApprovedBy, Value: c.ApprovedBy},
	}
}

// ActiveClaim is an active claim as policy sees it. RelayRequester is the
// login in the newest Waggle-Relay-Requester trailer of the claim's chain
// (comb.GitBackend.RelayRequester), "" for a maintainer claim.
type ActiveClaim struct {
	Lease          comb.Lease
	RelayRequester string
}

// Input is everything policy reads. The caller fills it from the event, the
// config on the default branch and the remote claim refs.
type Input struct {
	Req   Request
	Event Event
	// Relay is the relay: config. DefaultTTLMinutes is leases.default_ttl_minutes.
	Relay             config.RelayConfig
	DefaultTTLMinutes int
	// Active are the active claims on the remote.
	Active []ActiveClaim
	Now    time.Time
	// MaxDirs is leases.max_directories_per_claim, 0 for the default. Approval
	// re-parses the request with it.
	MaxDirs int
}

func refusal(format string, args ...any) Decision {
	return Decision{Kind: KindRefuse, Reason: refuse(format, args...).Reason}
}

// checked is a claim that passed every relay check except approval.
type checked struct {
	claim   Claim
	renewal bool
}

// check runs the relay checks on a claim request: fork branches, TTL, the
// per-login cap and the directory allowlist. It never decides approval.
func check(in Input) (checked, *Decision) {
	req, login := in.Req, in.Event.Login
	fail := func(format string, args ...any) (checked, *Decision) {
		d := refusal(format, args...)
		return checked{}, &d
	}
	if req.Command != CommandClaim {
		return fail("not a claim request")
	}
	if len(req.Branches) == 0 {
		return fail("name your fork branch with --branch <your login>/<repo>:<branch>; the default branch name is in the base repository, which you cannot push to")
	}
	for _, b := range req.Branches {
		repo, _, ok := strings.Cut(b, ":")
		owner, _, _ := strings.Cut(repo, "/")
		if !ok || !strings.EqualFold(owner, login) {
			return fail("every --branch must be a branch on your own fork, written <your login>/<repo>:<branch>")
		}
	}

	maxTTL := in.Relay.MaxTTLMinutes
	ttl := req.TTLMinutes
	if ttl == 0 {
		ttl = min(in.DefaultTTLMinutes, maxTTL)
	}
	if ttl > maxTTL {
		return fail("--ttl is over the limit of %d minutes", maxTTL)
	}

	var existing *ActiveClaim
	others := 0
	for i, a := range in.Active {
		if !a.Lease.ExpiresAt.After(in.Now) {
			continue
		}
		if a.Lease.TicketID == req.Task {
			existing = &in.Active[i]
			continue
		}
		if a.RelayRequester != "" && strings.EqualFold(a.RelayRequester, a.Lease.KeeperID) &&
			strings.EqualFold(a.Lease.KeeperID, login) {
			others++
		}
	}
	if others+1 > in.Relay.MaxActiveClaimsPerLogin {
		return fail("you already hold %d relay claims, the limit is %d", others, in.Relay.MaxActiveClaimsPerLogin)
	}

	if allowed := in.Relay.AllowedDirectories; len(allowed) > 0 {
		for _, d := range req.Dirs {
			if !underAny(d, allowed) {
				return fail("a directory is outside the directories relay claims may lease")
			}
		}
	}

	c := Claim{Task: req.Task, Owner: login, Dirs: req.Dirs, Gate: req.Gate, Branches: req.Branches, TTLMinutes: ttl}
	renewal := existing != nil && strings.EqualFold(existing.Lease.KeeperID, login) &&
		slices.Equal(existing.Lease.Directories, req.Dirs) && existing.Lease.Gate == req.Gate &&
		slices.Equal(existing.Lease.Branches, req.Branches)
	return checked{claim: c, renewal: renewal}, nil
}

// underAny reports whether dir equals or lies under one of prefixes.
func underAny(dir string, prefixes []string) bool {
	for _, p := range prefixes {
		p = strings.TrimSuffix(p, "/")
		if dir == p || strings.HasPrefix(dir, p+"/") {
			return true
		}
	}
	return false
}

// Decide decides a claim request. It is KindWrite when the requester's
// association is in auto_approve_associations or the request renews the same
// owner's active claim with identical directories, gate and branches, and
// KindPending (with the digest) otherwise.
func Decide(in Input) Decision {
	c, d := check(in)
	if d != nil {
		return *d
	}
	switch {
	case slices.Contains(in.Relay.AutoApproveAssociations, in.Event.Association):
		c.claim.ApprovedBy = "auto:" + in.Event.Association
	case c.renewal:
		c.claim.ApprovedBy = "auto:renewal"
	default:
		return Decision{Kind: KindPending, Digest: Digest(c.claim, in.Event.Login, in.Event.CommentID)}
	}
	return Decision{Kind: KindWrite, Claim: c.claim}
}

// ApprovalInput is the input for approve-claim. Req and Event are the
// approver's command and comment. Original and OriginalEvent are the request
// comment, re-read and decoded fresh by the caller, never trusted from the
// reply. Approvers are gate.approvers from the default branch (HK-5).
type ApprovalInput struct {
	Input
	Original      string // the request comment body as it is now
	OriginalEvent Event
	Approvers     []string
}

// DecideApproval decides an approve-claim. It re-parses the original request,
// recomputes its digest and writes only on an exact match, so an edit after
// the reply never gets a widened request written. The approver must be in
// Approvers (ASCII case folded) and must not be the requester. The relay
// checks run again on the re-parsed request.
func DecideApproval(in ApprovalInput) Decision {
	if in.Req.Command != CommandApprove {
		return refusal("not an approval")
	}
	approver, requester := in.Event.Login, in.OriginalEvent.Login
	if strings.EqualFold(approver, requester) {
		return refusal("the requester cannot approve their own request")
	}
	if len(in.Approvers) == 0 {
		return refusal("no approvers are configured in gate.approvers, so a maintainer must claim for you")
	}
	if !slices.ContainsFunc(in.Approvers, func(a string) bool { return strings.EqualFold(a, approver) }) {
		return refusal("only the approvers listed in gate.approvers can approve a claim")
	}
	req, err := Parse(in.Original, in.MaxDirs)
	if err != nil || req.Command != CommandClaim {
		return refusal("the request comment is gone or no longer a claim request")
	}
	chk := in.Input
	chk.Req, chk.Event = req, in.OriginalEvent
	c, d := check(chk)
	if d != nil {
		return *d
	}
	want := Digest(c.claim, requester, in.OriginalEvent.CommentID)
	if subtle.ConstantTimeCompare([]byte(want), []byte(in.Req.Digest)) != 1 {
		return refusal("the digest does not match the request as it stands now; it may have been edited, so ask for a new approval")
	}
	c.claim.ApprovedBy = approver
	return Decision{Kind: KindWrite, Claim: c.claim}
}

// DecideRelease decides a release. existing is the claim on the remote, nil
// when there is none. Only the claim's keeper_id (ASCII case folded) may
// release it; ReleaseLease checks the same, this refuses early with a reply.
func DecideRelease(req Request, ev Event, existing *comb.Lease) Decision {
	switch {
	case req.Command != CommandRelease:
		return refusal("not a release request")
	case existing == nil:
		return refusal("there is no claim for that task")
	case !strings.EqualFold(existing.KeeperID, ev.Login):
		return refusal("only the owner of a claim can release it")
	}
	return Decision{Kind: KindWrite, Claim: Claim{Task: req.Task, Owner: existing.KeeperID}}
}

// Digest is the first 12 hex digits of SHA-256 over the canonical request:
// task, sorted normalized directories, gate, branches in order, the effective
// TTL, the requester login (case folded) and the request comment id. A change
// to any of them changes the digest. Every field is validated printable ASCII
// without a newline, so one field per line cannot be confused with another.
func Digest(c Claim, requester string, commentID int64) string {
	var sb strings.Builder
	sb.WriteString("waggle-relay-v1\n")
	sb.WriteString("task=" + c.Task + "\n")
	for _, d := range c.Dirs {
		sb.WriteString("dir=" + d + "\n")
	}
	sb.WriteString("gate=" + c.Gate + "\n")
	for _, b := range c.Branches {
		sb.WriteString("branch=" + b + "\n")
	}
	sb.WriteString("ttl=" + strconv.Itoa(c.TTLMinutes) + "\n")
	sb.WriteString("requester=" + strings.ToLower(requester) + "\n")
	sb.WriteString("comment=" + strconv.FormatInt(commentID, 10) + "\n")
	sum := sha256.Sum256([]byte(sb.String()))
	return hex.EncodeToString(sum[:])[:12]
}
