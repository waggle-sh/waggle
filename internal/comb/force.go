// SPDX-License-Identifier: Apache-2.0

package comb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// ForceReleaseAuditRef is the append-only audit chain written by ForceRelease
// (D-010). The chain starts at a root commit with an empty tree and no
// parents; each force-release then appends one commit whose first parent is
// the previous audit head and whose second parent, when the released target
// is a commit, is the released claim commit, so the whole claim chain stays
// reachable and is pushed with the audit. The first-parent walk therefore
// holds only commits written here: review it with
// git log --first-parent refs/waggle/audit/force-releases, and a team-mode
// verifier (D-068) checks signatures along it without entering claim
// chains. It lives outside ClaimRefPrefix, so ListActiveLeases and the
// claim sync never see or prune it, and the merge gate does not fetch it in
// v0.1.
const ForceReleaseAuditRef = "refs/waggle/audit/force-releases"

// auditRefPrefix is the namespace ForceRelease mirrors from the remote.
const auditRefPrefix = "refs/waggle/audit/"

const (
	// maxReasonBytes bounds ForceRelease.Reason.
	maxReasonBytes = 512
	// auditRecordVersion is the force-release.json version written here.
	auditRecordVersion = 1
	// auditRecordFile and auditRawFile are the audit tree entry names.
	auditRecordFile = "force-release.json"
	auditRawFile    = "released-claim.raw"
)

// claimRefSuffixAllowed bounds the part of a force-release ref after ClaimRefPrefix.
var claimRefSuffixAllowed = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)

// HeadKeeperAuthorizer decides whether headKeeperID may force-release claims.
// Implementations must be deterministic and must not prompt. The comb package
// does not decide who the Head Keeper is: Options.HeadKeeper supplies the
// policy, and a nil authorizer denies everyone.
//
// The check is advisory against humans with push access (D-009): git refs
// have no per-ref ACL, so in direct comb mode anyone who can push to the
// remote can delete a claim ref with raw git. The check stops honest mistakes
// and agents acting through the CLI or waggled. Real enforcement for
// untrusted contributors is relay mode, forge permissions where the host
// offers them, and in team mode the gate's signature check (D-068).
type HeadKeeperAuthorizer interface {
	AuthorizeHeadKeeper(ctx context.Context, headKeeperID string) error
}

// StaticHeadKeepers authorizes an explicit allow-list of Keeper IDs. Per D-009
// the v0.1 allow-list lives in a Tier 3 control file and is matched against
// the local keeper_id (a GitHub login, D-016); loading it is the CLI's job.
// Matching is exact and case-sensitive, and an empty ID is never authorized.
// Like every HeadKeeperAuthorizer it is advisory against anyone with raw push
// access to the remote.
type StaticHeadKeepers []string

// AuthorizeHeadKeeper implements HeadKeeperAuthorizer. A rejected ID returns
// ErrNotHeadKeeper wrapped with the ID.
func (s StaticHeadKeepers) AuthorizeHeadKeeper(_ context.Context, headKeeperID string) error {
	if headKeeperID == "" {
		return fmt.Errorf("empty head keeper id: %w", ErrNotHeadKeeper)
	}
	for _, id := range s {
		if id == headKeeperID {
			return nil
		}
	}
	return fmt.Errorf("%q: %w", headKeeperID, ErrNotHeadKeeper)
}

// ForceRelease is a request to delete one claim ref regardless of its owner
// or shape. Callers that start from a ticket ID build Ref with RefName; a
// malformed ref is cleared by passing exactly the name reported by
// MalformedClaimsError, which may be non-canonical (ticket-007).
//
// ExpectedOID is mandatory: it is the OID the Head Keeper inspected (RefOID
// or ClaimOID), and the delete is a compare-and-swap against it. There is no
// "delete whatever is there" mode, so a blind delete is impossible by
// construction. Under RFC-0003 section 3.5 a force-release is a delete
// decision card: the CLI and queen wiring must present it for an explicit
// decision and must never auto-apply a default.
type ForceRelease struct {
	// Ref is the full ref under refs/waggle/claims/, as reported by
	// ListActiveLeases (through RefName) or MalformedClaimsError.
	Ref string
	// ExpectedOID is the exact OID the Head Keeper inspected. Required.
	ExpectedOID string
	// HeadKeeperID is the caller's Keeper ID, checked by Options.HeadKeeper. Required.
	HeadKeeperID string
	// Reason is recorded in the audit entry. Required: 1 to 512 bytes of
	// valid UTF-8 with at least one non-blank character, no NUL and no
	// control characters other than '\n'.
	Reason string
}

// auditRecord is the force-release.json entry in an audit commit's tree.
type auditRecord struct {
	Version         int       `json:"version"`
	Ref             string    `json:"ref"`
	ReleasedOID     string    `json:"released_oid"`
	ReleasedType    string    `json:"released_type"`
	Lease           *Lease    `json:"lease"`
	MalformedReason string    `json:"malformed_reason"`
	HeadKeeperID    string    `json:"head_keeper_id"`
	Reason          string    `json:"reason"`
	ReleasedAt      time.Time `json:"released_at"`
}

// validateClaimRef applies the pure ref rules of ForceRelease: ClaimRefPrefix,
// a non-empty suffix of at most 64 bytes from [A-Za-z0-9._-], no "..", no
// trailing ".lock" or ".", and never starting with "-". Callers also run
// git check-ref-format before the ref reaches a write.
func validateClaimRef(ref string) error {
	suffix, ok := strings.CutPrefix(ref, ClaimRefPrefix)
	switch {
	case !ok:
		return fmt.Errorf("ref %q is not under %s: %w", ref, ClaimRefPrefix, ErrInvalidForceRelease)
	case !claimRefSuffixAllowed.MatchString(suffix):
		return fmt.Errorf("ref %q has an invalid suffix: %w", ref, ErrInvalidForceRelease)
	case strings.Contains(suffix, ".."):
		return fmt.Errorf("ref %q contains \"..\": %w", ref, ErrInvalidForceRelease)
	case strings.HasSuffix(suffix, ".lock"), strings.HasSuffix(suffix, "."):
		return fmt.Errorf("ref %q has a reserved ending: %w", ref, ErrInvalidForceRelease)
	case strings.HasPrefix(suffix, "-"):
		return fmt.Errorf("ref %q starts with a dash: %w", ref, ErrInvalidForceRelease)
	}
	return nil
}

// validateReason applies the Reason rules of ForceRelease.
func validateReason(reason string) error {
	switch {
	case reason == "":
		return fmt.Errorf("empty reason: %w", ErrInvalidForceRelease)
	case len(reason) > maxReasonBytes:
		return fmt.Errorf("reason of %d bytes exceeds %d: %w", len(reason), maxReasonBytes, ErrInvalidForceRelease)
	case !utf8.ValidString(reason):
		return fmt.Errorf("reason is not valid UTF-8: %w", ErrInvalidForceRelease)
	case strings.TrimSpace(reason) == "":
		return fmt.Errorf("reason is blank: %w", ErrInvalidForceRelease)
	}
	for i := 0; i < len(reason); i++ {
		if c := reason[i]; (c < 0x20 && c != '\n') || c == 0x7f {
			return fmt.Errorf("reason contains control character %#x: %w", c, ErrInvalidForceRelease)
		}
	}
	return nil
}

// validateForceRelease checks every field of req without running git. oidLen
// is the repo's hash length (40 or 64).
func validateForceRelease(req ForceRelease, oidLen int) error {
	if err := validateClaimRef(req.Ref); err != nil {
		return err
	}
	if !oidPattern.MatchString(req.ExpectedOID) || len(req.ExpectedOID) != oidLen {
		return fmt.Errorf("expected oid %q is not a %d-digit object id: %w", req.ExpectedOID, oidLen, ErrInvalidForceRelease)
	}
	if !keeperIDPattern.MatchString(req.HeadKeeperID) {
		return fmt.Errorf("head keeper id %q: %w", req.HeadKeeperID, ErrInvalidForceRelease)
	}
	return validateReason(req.Reason)
}

// sanitizeReason returns reason for a commit message: every control character
// other than '\n' is dropped and surrounding whitespace is trimmed. The exact
// reason is stored in force-release.json.
func sanitizeReason(reason string) string {
	var sb strings.Builder
	sb.Grow(len(reason))
	for _, r := range reason {
		if (r < 0x20 && r != '\n') || r == 0x7f {
			continue
		}
		sb.WriteRune(r)
	}
	return strings.TrimSpace(sb.String())
}

// checkRef validates ref with the pure rules and git check-ref-format.
func (b *GitBackend) checkRef(ctx context.Context, ref string) error {
	if err := validateClaimRef(ref); err != nil {
		return err
	}
	if _, err := b.git(ctx, nil, nil, "check-ref-format", ref); err != nil {
		return fmt.Errorf("ref %q rejected by git: %w: %w", ref, ErrInvalidForceRelease, err)
	}
	return nil
}

// authorizeHeadKeeper consults Options.HeadKeeper. A nil authorizer and every
// failure return ErrNotHeadKeeper.
func (b *GitBackend) authorizeHeadKeeper(ctx context.Context, id string) error {
	if b.headKeeper == nil {
		return fmt.Errorf("%q: no head keeper authorizer configured: %w", id, ErrNotHeadKeeper)
	}
	err := b.headKeeper.AuthorizeHeadKeeper(ctx, id)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrNotHeadKeeper):
		return err
	default:
		return fmt.Errorf("%q: %w: %w", id, ErrNotHeadKeeper, err)
	}
}

// RefOID returns the OID of a claim ref given by its full name, which may be
// non-canonical, so a Head Keeper can inspect what MalformedClaimsError
// reported before force-releasing it. The ref is validated like
// ForceRelease.Ref (ErrInvalidForceRelease). In remote mode it syncs first.
// An absent ref returns ErrNotFound.
func (b *GitBackend) RefOID(ctx context.Context, ref string) (string, error) {
	if b.relay {
		return "", ErrRelayNotImplemented
	}
	if err := b.checkRef(ctx, ref); err != nil {
		return "", err
	}
	if b.remote != "" {
		if err := b.sync(ctx); err != nil {
			return "", err
		}
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return "", err
	}
	oid, err := b.readRefOID(ctx, ref, zero)
	if err != nil {
		return "", err
	}
	if oid == zero {
		return "", fmt.Errorf("%s: %w", ref, ErrNotFound)
	}
	return oid, nil
}

// ForceRelease deletes a claim ref regardless of owner, malformed or not,
// after authorizing req.HeadKeeperID, and records an audit entry on
// ForceReleaseAuditRef atomically with the delete. It is an admin method on
// *GitBackend only; it is not part of Backend, so workers holding a Backend
// never see it.
//
// Order: relay mode returns ErrRelayNotImplemented; the request is validated
// (ErrInvalidForceRelease); Options.HeadKeeper is consulted, and a nil
// authorizer or any refusal returns ErrNotHeadKeeper before any claim is
// read. Then the ref is read: absent returns ErrNotFound, and an OID other
// than req.ExpectedOID returns ErrConcurrentUpdate (the claim was renewed,
// taken over or replaced since the Head Keeper looked; re-inspect). A
// malformed target never fails the release; its shape is recorded in the
// audit entry instead.
//
// Atomicity: locally the delete and the audit append run in one
// git update-ref --stdin transaction. In remote mode the remote is
// authoritative: both namespaces are fetched first, then one
// git push --atomic with a --force-with-lease on each ref deletes the claim
// and appends the audit commit, and both are mirrored locally afterwards.
// Either both changes apply or neither. A lost race on the audit chain alone
// is retried on the new audit head up to maxCASRetries times, then returns
// ErrConcurrentUpdate.
//
// Limitation (D-009): git refs have no per-ref ACL. In direct comb mode a
// Keeper with push access can delete the same ref with raw git, bypassing
// both the authorizer and the audit trail; nothing here is signed. In team
// mode (D-068) a raw ref deletion still succeeds where the forge allows it;
// it leaves no commit to verify and the PR then fails the gate for having no
// claim, which is a denial of service, not a forged claim. T-018 reports
// whether the forge can block pushes to refs/waggle/**. Under RFC-0003
// section 3.5 every force-release is a delete decision card that the CLI and
// queen must never auto-apply.
func (b *GitBackend) ForceRelease(ctx context.Context, req ForceRelease) error {
	if b.relay {
		return ErrRelayNotImplemented
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return err
	}
	if err := validateForceRelease(req, len(zero)); err != nil {
		return err
	}
	if err := b.checkRef(ctx, req.Ref); err != nil {
		return err
	}
	if err := b.authorizeHeadKeeper(ctx, req.HeadKeeperID); err != nil {
		return err
	}
	now := b.now().UTC().Truncate(time.Second)
	for attempt := 0; ; attempt++ {
		if b.remote != "" {
			if err := b.syncWithAudit(ctx); err != nil {
				return err
			}
		}
		target, err := b.inspectTarget(ctx, req)
		if err != nil {
			return err
		}
		oldAudit, err := b.readRefOID(ctx, ForceReleaseAuditRef, zero)
		if err != nil {
			return err
		}
		newAudit, err := b.buildAudit(ctx, req, target, oldAudit, zero, now)
		if err != nil {
			return err
		}
		if b.hookBeforeWrite != nil {
			b.hookBeforeWrite()
		}
		var out []byte
		var werr error
		if b.remote != "" {
			out, werr = b.pushForceRelease(ctx, req.Ref, req.ExpectedOID, oldAudit, newAudit, zero)
			if werr == nil {
				// The remote is authoritative; a failed mirror write is repaired by the next sync.
				_ = b.refTransaction(ctx, req.Ref, req.ExpectedOID, oldAudit, newAudit)
				return nil
			}
		} else {
			werr = b.refTransaction(ctx, req.Ref, req.ExpectedOID, oldAudit, newAudit)
			if werr == nil {
				return nil
			}
		}
		if ctx.Err() != nil {
			return werr
		}
		if b.remote != "" {
			if !pushRejected(out, req.Ref) && !pushRejected(out, ForceReleaseAuditRef) {
				return werr
			}
			if serr := b.syncWithAudit(ctx); serr != nil {
				return fmt.Errorf("%s: push rejected (%v), resync failed (%v): %w", req.Ref, werr, serr, ErrConcurrentUpdate)
			}
		}
		cur, rerr := b.readRefOID(ctx, req.Ref, zero)
		if rerr != nil {
			return werr
		}
		if cur != req.ExpectedOID {
			return fmt.Errorf("%s: %w", req.Ref, ErrConcurrentUpdate)
		}
		curAudit, rerr := b.readRefOID(ctx, ForceReleaseAuditRef, zero)
		if rerr != nil {
			return werr
		}
		auditMoved := curAudit != oldAudit
		if !auditMoved && b.remote == "" && !isLockContention(werr) {
			return werr
		}
		if attempt >= maxCASRetries {
			if auditMoved {
				return fmt.Errorf("%s: audit ref %s changed on each of %d attempts: %w",
					req.Ref, ForceReleaseAuditRef, attempt+1, ErrConcurrentUpdate)
			}
			return fmt.Errorf("comb: %s: write rejected %d times with refs unchanged: %w", req.Ref, attempt+1, werr)
		}
		if b.hookBeforeRetry != nil {
			b.hookBeforeRetry(attempt)
		}
		if serr := sleepBackoff(ctx); serr != nil {
			return serr
		}
	}
}

// syncWithAudit mirrors the remote claim and audit namespaces into the local
// repo in one fetch, so the audit head read afterwards is the remote's.
func (b *GitBackend) syncWithAudit(ctx context.Context) error {
	_, err := b.git(ctx, nil, nil, "fetch", "--prune", "--no-tags", "--quiet", b.remote,
		"+"+ClaimRefPrefix+"*:"+ClaimRefPrefix+"*",
		"+"+auditRefPrefix+"*:"+auditRefPrefix+"*")
	return err
}

// inspectTarget reads req.Ref, checks it against req.ExpectedOID and
// classifies it for the audit record with the same checks as
// ListActiveLeases (object type, decoded lease or malformation reason).
func (b *GitBackend) inspectTarget(ctx context.Context, req ForceRelease) (claim, error) {
	refs, err := b.listRefs(ctx, req.Ref)
	if err != nil {
		return claim{}, err
	}
	var entry *refEntry
	for i := range refs {
		if refs[i].ref == req.Ref {
			entry = &refs[i]
		}
	}
	if entry == nil {
		return claim{}, fmt.Errorf("%s: %w", req.Ref, ErrNotFound)
	}
	if entry.oid != req.ExpectedOID {
		return claim{}, fmt.Errorf("%s is at %s, expected %s: %w", req.Ref, entry.oid, req.ExpectedOID, ErrConcurrentUpdate)
	}
	claims, err := b.loadClaims(ctx, []refEntry{*entry})
	if err != nil {
		return claim{}, err
	}
	return claims[0], nil
}

// buildAudit writes force-release.json, the audit tree and the audit commit
// for target, chained on oldAudit (zero for the first entry, which chains on
// a fresh auditRoot instead). A commit target becomes parent 2; a blob
// target is retained as released-claim.raw.
func (b *GitBackend) buildAudit(ctx context.Context, req ForceRelease, target claim, oldAudit, zero string, now time.Time) (string, error) {
	rec := auditRecord{
		Version:         auditRecordVersion,
		Ref:             req.Ref,
		ReleasedOID:     target.oid,
		ReleasedType:    target.objType,
		MalformedReason: target.reason,
		HeadKeeperID:    req.HeadKeeperID,
		Reason:          req.Reason,
		ReleasedAt:      now,
	}
	if target.reason == "" {
		lease := target.lease
		rec.Lease = &lease
	}
	data, err := json.MarshalIndent(rec, "", "  ")
	if err != nil {
		return "", fmt.Errorf("comb: encode audit record: %w", err)
	}
	data = append(data, '\n')
	blob, err := b.gitOID(ctx, data, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	entries := "100644 blob " + blob + "\t" + auditRecordFile + "\n"
	if target.objType == "blob" {
		entries += "100644 blob " + target.oid + "\t" + auditRawFile + "\n"
	}
	tree, err := b.gitOID(ctx, []byte(entries), nil, "mktree")
	if err != nil {
		return "", err
	}
	head := oldAudit
	if head == zero {
		if head, err = b.auditRoot(ctx, now); err != nil {
			return "", err
		}
	}
	parents := []string{head}
	if target.objType == "commit" {
		parents = append(parents, target.oid)
	}
	msg := "waggle: force-release " + req.Ref + " by " + req.HeadKeeperID + "\n\n" + sanitizeReason(req.Reason)
	return b.commitTree(ctx, tree, parents, msg, now)
}

// auditRoot writes the root commit of a new audit chain: an empty tree, no
// parents. It exists so that a released claim commit is always parent 2 and
// never enters the chain's first-parent walk.
func (b *GitBackend) auditRoot(ctx context.Context, now time.Time) (string, error) {
	tree, err := b.gitOID(ctx, []byte{}, nil, "mktree")
	if err != nil {
		return "", err
	}
	return b.commitTree(ctx, tree, nil, "waggle: audit root", now)
}

// refTransaction deletes ref from expected and moves ForceReleaseAuditRef from
// oldAudit (zero for create-only) to newAudit in one git update-ref --stdin
// transaction. Either both apply or neither. Every argument has been validated
// by the caller, so the script needs no quoting.
func (b *GitBackend) refTransaction(ctx context.Context, ref, expected, oldAudit, newAudit string) error {
	script := "start\n" +
		"delete " + ref + " " + expected + "\n" +
		"update " + ForceReleaseAuditRef + " " + newAudit + " " + oldAudit + "\n" +
		"prepare\n" +
		"commit\n"
	_, err := b.git(ctx, []byte(script), nil, "update-ref", "-m", "waggle: force-release "+ref, "--stdin")
	return err
}

// pushForceRelease deletes ref and appends newAudit on the remote in one
// git push --atomic guarded by a --force-with-lease on each ref. It returns
// the porcelain output for race classification.
func (b *GitBackend) pushForceRelease(ctx context.Context, ref, expected, oldAudit, newAudit, zero string) ([]byte, error) {
	auditLease := "--force-with-lease=" + ForceReleaseAuditRef + ":"
	if oldAudit != zero {
		auditLease += oldAudit
	}
	return b.git(ctx, nil, nil, "push", "--atomic", "--porcelain",
		"--force-with-lease="+ref+":"+expected, auditLease,
		b.remote, ":"+ref, newAudit+":"+ForceReleaseAuditRef)
}
