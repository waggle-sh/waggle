package comb

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Comb modes accepted by Options.CombMode.
const (
	// CombModeDirect pushes claim refs directly (the default).
	CombModeDirect = "direct"
	// CombModeRelay routes claims through the Forge Action Relay (not implemented in v0.1).
	CombModeRelay = "relay"
)

// DefaultLeaseTTL is the default for leases.default_ttl_minutes.
const DefaultLeaseTTL = 120 * time.Minute

// maxCASRetries bounds retries after lock contention (local) or a push
// rejection that left the remote ref unchanged (remote).
const maxCASRetries = 5

var (
	oidPattern    = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)
	remotePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)
)

// Options configures a GitBackend. Callers populate it explicitly; this package
// does not read .waggle/config.yaml.
type Options struct {
	RepoDir                string           // required; absolute or relative path to a git work tree or bare repo
	Remote                 string           // "" = local-only; otherwise remote name, e.g. "origin"
	CombMode               string           // "direct" (default when "") | "relay"
	MaxDirectoriesPerClaim int              // <= 0 means default 5 (leases.max_directories_per_claim)
	DefaultTTL             time.Duration    // <= 0 means default 120m (leases.default_ttl_minutes)
	Now                    func() time.Time // nil means time.Now
	GitBinary              string           // "" means "git"
	ExtraEnv               []string         // appended to the git child env (tests use this)
}

// GitBackend is the single Comb codepath of RFC-0003 section 3.1. Each ticket
// claim lives at refs/waggle/claims/ticket-<n> and points to a commit whose tree
// holds exactly one file, lease.json. Renewals and takeovers chain to the
// previous claim commit; release deletes the ref.
//
// With Options.Remote empty the backend writes the local repository with
// git update-ref CAS (Scale = 1). With a remote set, the remote is
// authoritative: every operation fetches refs/waggle/claims/* first and writes
// with git push --atomic --force-with-lease (Scale = 500+). Local claim refs are
// then only a mirror.
//
// CONCURRENCY INVARIANT: ref CAS is atomic per ticket, not per directory. Two
// Keepers claiming different tickets with overlapping directories can both pass
// the claim-time overlap check. Directory leases are therefore optimistic at
// claim time and authoritative at merge time (waggle-gate.yml). This backend
// does not try to close that TOCTOU window.
//
// A GitBackend is safe for concurrent use by multiple goroutines.
type GitBackend struct {
	repoDir  string
	remote   string
	relay    bool
	maxDirs  int
	ttl      time.Duration
	now      func() time.Time
	gitBin   string
	extraEnv []string

	mu     sync.Mutex
	oidLen int

	// hookBeforeWrite, when set by tests, runs after the claim ref is read and
	// before the CAS write, to force deterministic races.
	hookBeforeWrite func()
}

var _ Backend = (*GitBackend)(nil)

// NewGitBackend validates opts and returns a backend. It does not run git.
func NewGitBackend(opts Options) (*GitBackend, error) {
	if strings.TrimSpace(opts.RepoDir) == "" {
		return nil, errors.New("comb: Options.RepoDir is required")
	}
	b := &GitBackend{
		repoDir:  opts.RepoDir,
		remote:   opts.Remote,
		maxDirs:  opts.MaxDirectoriesPerClaim,
		ttl:      opts.DefaultTTL,
		now:      opts.Now,
		gitBin:   opts.GitBinary,
		extraEnv: append([]string(nil), opts.ExtraEnv...),
	}
	switch opts.CombMode {
	case "", CombModeDirect:
	case CombModeRelay:
		b.relay = true
	default:
		return nil, fmt.Errorf("comb: unknown comb mode %q", opts.CombMode)
	}
	if opts.Remote != "" && !remotePattern.MatchString(opts.Remote) {
		return nil, fmt.Errorf("comb: invalid remote name %q", opts.Remote)
	}
	if b.maxDirs <= 0 {
		b.maxDirs = DefaultMaxDirectoriesPerClaim
	}
	if b.ttl <= 0 {
		b.ttl = DefaultLeaseTTL
	}
	if b.now == nil {
		b.now = time.Now
	}
	if b.gitBin == "" {
		b.gitBin = "git"
	}
	return b, nil
}

// gitError describes a failed git invocation.
type gitError struct {
	subcommand string
	stderr     string
	err        error
	ctxErr     error
}

func (e *gitError) Error() string {
	msg := fmt.Sprintf("comb: git %s: %v", e.subcommand, e.err)
	if e.stderr != "" {
		msg += ": " + e.stderr
	}
	return msg
}

func (e *gitError) Unwrap() []error {
	if e.ctxErr != nil {
		return []error{e.err, e.ctxErr}
	}
	return []error{e.err}
}

// git runs one git command. It is the only place this package starts a process:
// exec.CommandContext with "-C repoDir" and separate argv entries, no shell.
// Stdout is returned even on failure (push --porcelain reports there).
func (b *GitBackend) git(ctx context.Context, stdin []byte, env []string, args ...string) ([]byte, error) {
	argv := make([]string, 0, len(args)+2)
	argv = append(argv, "-C", b.repoDir)
	argv = append(argv, args...)
	cmd := exec.CommandContext(ctx, b.gitBin, argv...)
	childEnv := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_ALL=C")
	childEnv = append(childEnv, b.extraEnv...)
	cmd.Env = append(childEnv, env...)
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), &gitError{
			subcommand: args[0],
			stderr:     strings.TrimSpace(stderr.String()),
			err:        err,
			ctxErr:     ctx.Err(),
		}
	}
	return stdout.Bytes(), nil
}

// gitOID runs git and returns its trimmed stdout, validated as an object ID.
func (b *GitBackend) gitOID(ctx context.Context, stdin []byte, env []string, args ...string) (string, error) {
	out, err := b.git(ctx, stdin, env, args...)
	if err != nil {
		return "", err
	}
	oid := strings.TrimSpace(string(out))
	if !oidPattern.MatchString(oid) {
		return "", fmt.Errorf("comb: git %s returned invalid object id %q", args[0], oid)
	}
	return oid, nil
}

// zeroOID returns the all-zero object ID at the repo's hash length.
func (b *GitBackend) zeroOID(ctx context.Context) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.oidLen == 0 {
		out, err := b.git(ctx, nil, nil, "rev-parse", "--show-object-format")
		if err != nil {
			return "", err
		}
		switch f := strings.TrimSpace(string(out)); f {
		case "sha1":
			b.oidLen = 40
		case "sha256":
			b.oidLen = 64
		default:
			return "", fmt.Errorf("comb: unsupported object format %q", f)
		}
	}
	return strings.Repeat("0", b.oidLen), nil
}

// sync mirrors the remote claim namespace into the local repo.
func (b *GitBackend) sync(ctx context.Context) error {
	_, err := b.git(ctx, nil, nil, "fetch", "--prune", "--no-tags", "--quiet", b.remote,
		"+"+ClaimRefPrefix+"*:"+ClaimRefPrefix+"*")
	return err
}

// refEntry is one ref under refs/waggle/claims/.
type refEntry struct {
	ref, oid, objType string
}

// listRefs returns claim refs matching pattern (a namespace prefix or exact ref).
func (b *GitBackend) listRefs(ctx context.Context, pattern string) ([]refEntry, error) {
	out, err := b.git(ctx, nil, nil, "for-each-ref",
		"--format=%(refname)%00%(objectname)%00%(objecttype)", pattern)
	if err != nil {
		return nil, err
	}
	var refs []refEntry
	for _, line := range strings.Split(string(out), "\n") {
		if line == "" {
			continue
		}
		f := strings.Split(line, "\x00")
		if len(f) != 3 {
			return nil, fmt.Errorf("comb: unexpected for-each-ref output %q", line)
		}
		refs = append(refs, refEntry{ref: f[0], oid: f[1], objType: f[2]})
	}
	return refs, nil
}

// readRefOID returns the OID of ref, or zero when the ref is absent.
func (b *GitBackend) readRefOID(ctx context.Context, ref, zero string) (string, error) {
	refs, err := b.listRefs(ctx, ref)
	if err != nil {
		return "", err
	}
	for _, r := range refs {
		if r.ref == ref {
			if !oidPattern.MatchString(r.oid) {
				return "", fmt.Errorf("comb: ref %s has invalid object id %q", ref, r.oid)
			}
			return r.oid, nil
		}
	}
	return zero, nil
}

// claim is a decoded claim ref. A non-empty reason marks it malformed.
type claim struct {
	ref    string
	oid    string
	num    int
	lease  Lease
	reason string
}

// loadClaims decodes the lease.json of each ref with one cat-file --batch call.
func (b *GitBackend) loadClaims(ctx context.Context, refs []refEntry) ([]claim, error) {
	claims := make([]claim, len(refs))
	var batch bytes.Buffer
	var pending []int
	for i, r := range refs {
		c := claim{ref: r.ref, oid: r.oid}
		n, ok := parseClaimRef(r.ref)
		switch {
		case !ok:
			c.reason = "ref name is not canonical ticket-<n>"
		case !oidPattern.MatchString(r.oid):
			c.reason = "invalid object id"
		case r.objType != "commit":
			c.reason = "points to a " + r.objType + ", not a commit"
		default:
			c.num = n
			pending = append(pending, i)
			batch.WriteString(r.oid + ":lease.json\n")
		}
		claims[i] = c
	}
	if len(pending) == 0 {
		return claims, nil
	}
	out, err := b.git(ctx, batch.Bytes(), nil, "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	rd := bufio.NewReader(bytes.NewReader(out))
	for _, i := range pending {
		c := &claims[i]
		header, err := rd.ReadString('\n')
		if err != nil {
			return nil, fmt.Errorf("comb: short cat-file --batch output: %w", err)
		}
		f := strings.Fields(header)
		if len(f) != 3 {
			// "<name> missing" or "<name> ambiguous".
			c.reason = "tree has no lease.json"
			continue
		}
		size, err := strconv.Atoi(f[2])
		if err != nil || size < 0 {
			return nil, fmt.Errorf("comb: unexpected cat-file --batch header %q", strings.TrimSpace(header))
		}
		body := make([]byte, size+1)
		if _, err := io.ReadFull(rd, body); err != nil {
			return nil, fmt.Errorf("comb: short cat-file --batch output: %w", err)
		}
		if f[1] != "blob" {
			c.reason = "lease.json is a " + f[1] + ", not a blob"
			continue
		}
		c.lease, c.reason = decodeLease(body[:size], c.num)
	}
	return claims, nil
}

// parseLease decodes lease.json and resolves its role name by version. A
// version 1 claim was written before the role rename, so its role is mapped
// through legacyRoles and any other version 1 role is an error. A LeaseVersion
// claim must already carry a current claim role. Other versions are errors.
func parseLease(data []byte) (Lease, error) {
	var l Lease
	if err := json.Unmarshal(data, &l); err != nil {
		return Lease{}, fmt.Errorf("lease.json is not valid JSON: %w", err)
	}
	switch l.Version {
	case 1:
		mapped, ok := legacyRoles[l.Role]
		if !ok {
			return Lease{}, fmt.Errorf("unknown version 1 role %q", l.Role)
		}
		l.Role = mapped
	case LeaseVersion:
		if !isClaimRole(l.Role) {
			return Lease{}, fmt.Errorf("unknown role %q", l.Role)
		}
	default:
		return Lease{}, fmt.Errorf("unsupported lease version %d", l.Version)
	}
	return l, nil
}

// decodeLease parses lease.json, checks it against the ref's ticket number and
// re-validates every field with the rules AcquireLease applies (F4). The
// directory limit is not enforced on stored claims, so a config change never
// turns an existing claim malformed. The returned lease carries normalized
// directories, so a claim stored as "foo/" blocks "foo/bar" like one stored
// as "foo".
func decodeLease(data []byte, num int) (Lease, string) {
	l, err := parseLease(data)
	if err != nil {
		return Lease{}, err.Error()
	}
	if l.TicketID != ticketDisplay(num) {
		return Lease{}, fmt.Sprintf("lease ticket_id %q does not match ref", l.TicketID)
	}
	v, err := validateLeaseFields(l, math.MaxInt)
	if err != nil {
		return Lease{}, err.Error()
	}
	if v.ClaimedAt.After(v.ExpiresAt) {
		return Lease{}, "claimed_at is after expires_at"
	}
	return v, ""
}

// scan reads and decodes every claim ref. In remote mode callers sync first.
func (b *GitBackend) scan(ctx context.Context) ([]claim, error) {
	refs, err := b.listRefs(ctx, ClaimRefPrefix)
	if err != nil {
		return nil, err
	}
	return b.loadClaims(ctx, refs)
}

func isActive(l Lease, now time.Time) bool { return l.ExpiresAt.After(now) }

// AcquireLease validates lease, checks it against existing claims and writes the
// claim ref with compare-and-swap. A first claim creates a root commit; a renewal
// (same KeeperID, active claim) or takeover (expired claim, any Keeper) creates
// a child of the previous claim commit.
//
// Zero Version becomes LeaseVersion, zero ClaimedAt becomes now, and zero
// ExpiresAt becomes ClaimedAt plus the default TTL. Times are stored in UTC at
// second precision. Version 1 is rejected: its role names predate the rename,
// so a caller renewing from a listed version 1 lease sets Version to zero.
// Renewing or taking over a stored version 1 claim writes a LeaseVersion claim
// on top of it; the claim commit message always carries the current role name.
//
// The directory overlap check is optimistic: CAS is atomic per ticket only, so
// two different tickets with overlapping directories can both be claimed in a
// race. The merge gate (waggle-gate.yml) is authoritative. Malformed claim refs
// for other tickets are ignored by the overlap check; a malformed ref for the
// target ticket returns ErrMalformedClaim. Losing a race returns ErrAlreadyClaimed.
//
// Stored claims are re-validated with the current field and directory rules
// when they are read, and the overlap check sees their normalized directories.
// A rule change can therefore surface an existing claim as malformed.
func (b *GitBackend) AcquireLease(ctx context.Context, lease Lease) error {
	if b.relay {
		return ErrRelayNotImplemented
	}
	now := b.now()
	l, err := b.prepareLease(lease, now)
	if err != nil {
		return err
	}
	ref, err := RefName(l.TicketID)
	if err != nil {
		return err
	}
	if b.remote != "" {
		if err := b.sync(ctx); err != nil {
			return err
		}
	}
	zero, err := b.zeroOID(ctx)
	if err != nil {
		return err
	}
	claims, err := b.scan(ctx)
	if err != nil {
		return err
	}

	expected, parent, verb := zero, "", "claim"
	for _, c := range claims {
		if c.ref != ref {
			continue
		}
		if c.reason != "" {
			return fmt.Errorf("%s: %s: %w", ref, c.reason, ErrMalformedClaim)
		}
		expected, parent = c.oid, c.oid
		switch {
		case !isActive(c.lease, now):
			verb = "takeover"
		case c.lease.KeeperID != l.KeeperID:
			return fmt.Errorf("%s held by %s until %s: %w", l.TicketID, c.lease.KeeperID,
				c.lease.ExpiresAt.UTC().Format(time.RFC3339), ErrAlreadyClaimed)
		default:
			verb = "renew"
		}
	}
	for _, c := range claims {
		if c.ref == ref || c.reason != "" || !isActive(c.lease, now) {
			continue
		}
		for _, d := range l.Directories {
			for _, od := range c.lease.Directories {
				if DirsOverlap(d, od) {
					return fmt.Errorf("%s directory %q overlaps %s directory %q held by %s: %w",
						l.TicketID, d, c.lease.TicketID, od, c.lease.KeeperID, ErrLeaseOverlap)
				}
			}
		}
	}

	msg := fmt.Sprintf("waggle: %s %s by %s (%s, %s)", verb, l.TicketID, l.KeeperID, l.Role, l.Gate)
	newOID, err := b.buildCommit(ctx, l, parent, msg, now)
	if err != nil {
		return err
	}
	if b.hookBeforeWrite != nil {
		b.hookBeforeWrite()
	}
	if b.remote != "" {
		return b.pushClaim(ctx, ref, newOID, expected, zero, l.TicketID)
	}
	return b.casLocal(ctx, ref, newOID, expected, zero, msg, l.TicketID)
}

// prepareLease validates a lease and fills defaults.
func (b *GitBackend) prepareLease(lease Lease, now time.Time) (Lease, error) {
	l, err := validateLeaseFields(lease, b.maxDirs)
	if err != nil {
		return Lease{}, err
	}
	switch l.Version {
	case 0:
		l.Version = LeaseVersion
	case LeaseVersion:
	default:
		return Lease{}, fmt.Errorf("version %d: %w", l.Version, ErrInvalidLease)
	}
	if l.ClaimedAt.IsZero() {
		l.ClaimedAt = now
	}
	l.ClaimedAt = l.ClaimedAt.UTC().Truncate(time.Second)
	if l.ExpiresAt.IsZero() {
		l.ExpiresAt = l.ClaimedAt.Add(b.ttl)
	}
	l.ExpiresAt = l.ExpiresAt.UTC().Truncate(time.Second)
	if l.ClaimedAt.After(l.ExpiresAt) {
		return Lease{}, fmt.Errorf("claimed_at after expires_at: %w", ErrInvalidLease)
	}
	if !l.ExpiresAt.After(now) {
		return Lease{}, fmt.Errorf("expires_at %s is not in the future: %w",
			l.ExpiresAt.Format(time.RFC3339), ErrInvalidLease)
	}
	return l, nil
}

// buildCommit writes lease.json, a one-entry tree and a claim commit.
func (b *GitBackend) buildCommit(ctx context.Context, l Lease, parent, msg string, now time.Time) (string, error) {
	data, err := json.MarshalIndent(l, "", "  ")
	if err != nil {
		return "", fmt.Errorf("comb: encode lease: %w", err)
	}
	data = append(data, '\n')
	blob, err := b.gitOID(ctx, data, nil, "hash-object", "-w", "--stdin")
	if err != nil {
		return "", err
	}
	tree, err := b.gitOID(ctx, []byte("100644 blob "+blob+"\tlease.json\n"), nil, "mktree")
	if err != nil {
		return "", err
	}
	args := []string{"commit-tree", "--no-gpg-sign", tree}
	if parent != "" {
		args = append(args, "-p", parent)
	}
	args = append(args, "-m", msg)
	date := fmt.Sprintf("@%d +0000", now.Unix())
	env := []string{
		"GIT_AUTHOR_NAME=waggle", "GIT_AUTHOR_EMAIL=waggle@localhost", "GIT_AUTHOR_DATE=" + date,
		"GIT_COMMITTER_NAME=waggle", "GIT_COMMITTER_EMAIL=waggle@localhost", "GIT_COMMITTER_DATE=" + date,
	}
	return b.gitOID(ctx, nil, env, args...)
}

// casLocal writes ref from expected to newOID with update-ref, classifying a
// lost race as ErrAlreadyClaimed and retrying transient lock contention.
func (b *GitBackend) casLocal(ctx context.Context, ref, newOID, expected, zero, msg, ticket string) error {
	for attempt := 0; ; attempt++ {
		_, err := b.git(ctx, nil, nil, "update-ref", "-m", msg, ref, newOID, expected)
		if err == nil {
			return nil
		}
		if ctx.Err() != nil {
			return err
		}
		cur, rerr := b.readRefOID(ctx, ref, zero)
		if rerr != nil {
			return err
		}
		if cur != expected {
			return fmt.Errorf("%s: lost claim race: %w", ticket, ErrAlreadyClaimed)
		}
		if isLockContention(err) && attempt < maxCASRetries {
			if serr := sleepBackoff(ctx); serr != nil {
				return serr
			}
			continue
		}
		return err
	}
}

func isLockContention(err error) bool {
	var ge *gitError
	if !errors.As(err, &ge) {
		return false
	}
	return strings.Contains(ge.stderr, "cannot lock ref") || strings.Contains(ge.stderr, "Unable to create")
}

// sleepBackoff waits 10 to 50 ms or until ctx is done.
func sleepBackoff(ctx context.Context) error {
	d := 10*time.Millisecond + rand.N(40*time.Millisecond)
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// pushRejected reports whether push --porcelain output has a "!" status line for ref.
func pushRejected(out []byte, ref string) bool {
	for _, line := range strings.Split(string(out), "\n") {
		f := strings.Split(line, "\t")
		if len(f) < 2 || f[0] != "!" {
			continue
		}
		if i := strings.LastIndex(f[1], ":"); i >= 0 && f[1][i+1:] == ref {
			return true
		}
	}
	return false
}

// pushClaim writes ref on the remote with push --atomic --force-with-lease.
func (b *GitBackend) pushClaim(ctx context.Context, ref, newOID, expected, zero, ticket string) error {
	leaseArg := "--force-with-lease=" + ref + ":"
	if expected != zero {
		leaseArg += expected
	}
	for attempt := 0; ; attempt++ {
		out, err := b.git(ctx, nil, nil, "push", "--atomic", "--porcelain", leaseArg, b.remote, newOID+":"+ref)
		if err == nil {
			// The remote is authoritative; a failed mirror write is repaired by the next sync.
			_, _ = b.git(ctx, nil, nil, "update-ref", ref, newOID)
			return nil
		}
		if ctx.Err() != nil || !pushRejected(out, ref) {
			return err
		}
		if serr := b.sync(ctx); serr != nil {
			return fmt.Errorf("%s: push rejected (%v), resync failed (%v): %w", ticket, err, serr, ErrAlreadyClaimed)
		}
		cur, rerr := b.readRefOID(ctx, ref, zero)
		if rerr != nil {
			return fmt.Errorf("%s: push rejected (%v): %w", ticket, err, ErrAlreadyClaimed)
		}
		if cur != expected {
			return fmt.Errorf("%s: lost claim race: %w", ticket, ErrAlreadyClaimed)
		}
		if attempt >= maxCASRetries {
			return fmt.Errorf("comb: %s: push rejected %d times with remote ref unchanged: %w",
				ticket, attempt+1, err)
		}
		if serr := sleepBackoff(ctx); serr != nil {
			return serr
		}
	}
}

// ReleaseLease deletes the claim ref for ticketID if keeperID owns it, using a
// compare-and-swap delete against the OID that was read. It returns ErrNotFound
// when no claim exists, ErrNotOwner when another Keeper owns it (expired or not;
// a non-owner takes over with AcquireLease instead), and ErrConcurrentUpdate when
// the ref changed between read and delete.
func (b *GitBackend) ReleaseLease(ctx context.Context, ticketID string, keeperID string) error {
	if b.relay {
		return ErrRelayNotImplemented
	}
	ref, err := RefName(ticketID)
	if err != nil {
		return err
	}
	if b.remote != "" {
		if err := b.sync(ctx); err != nil {
			return err
		}
	}
	refs, err := b.listRefs(ctx, ref)
	if err != nil {
		return err
	}
	var entry *refEntry
	for i := range refs {
		if refs[i].ref == ref {
			entry = &refs[i]
		}
	}
	if entry == nil {
		return fmt.Errorf("%s: %w", ref, ErrNotFound)
	}
	claims, err := b.loadClaims(ctx, []refEntry{*entry})
	if err != nil {
		return err
	}
	c := claims[0]
	if c.reason != "" {
		return fmt.Errorf("%s: %s: %w", ref, c.reason, ErrMalformedClaim)
	}
	if c.lease.KeeperID != keeperID {
		return fmt.Errorf("%s owned by %s: %w", c.lease.TicketID, c.lease.KeeperID, ErrNotOwner)
	}
	if b.hookBeforeWrite != nil {
		b.hookBeforeWrite()
	}
	if b.remote != "" {
		out, err := b.git(ctx, nil, nil, "push", "--atomic", "--porcelain",
			"--force-with-lease="+ref+":"+c.oid, b.remote, ":"+ref)
		if err != nil {
			if ctx.Err() == nil && pushRejected(out, ref) {
				return fmt.Errorf("%s: %w", c.lease.TicketID, ErrConcurrentUpdate)
			}
			return err
		}
		// Ignore a mirror mismatch; the next sync fixes it.
		_, _ = b.git(ctx, nil, nil, "update-ref", "-d", ref, c.oid)
		return nil
	}
	if _, err := b.git(ctx, nil, nil, "update-ref", "-d", ref, c.oid); err != nil {
		if ctx.Err() != nil {
			return err
		}
		zero, zerr := b.zeroOID(ctx)
		if zerr != nil {
			return err
		}
		cur, rerr := b.readRefOID(ctx, ref, zero)
		if rerr == nil && cur != c.oid {
			return fmt.Errorf("%s: %w", c.lease.TicketID, ErrConcurrentUpdate)
		}
		return err
	}
	return nil
}

// ListActiveLeases returns every unexpired, well-formed claim sorted by numeric
// ticket ID. If some claim refs are malformed it returns the valid leases and a
// *MalformedClaimsError; the slice is valid in that case. A git or transport
// failure returns nil and the error.
//
// Every stored claim is re-validated with the current field and directory
// rules, so a rule change can report an existing claim as malformed. Returned
// leases carry canonical ticket IDs and normalized directories only.
func (b *GitBackend) ListActiveLeases(ctx context.Context) ([]Lease, error) {
	if b.relay {
		return nil, ErrRelayNotImplemented
	}
	if b.remote != "" {
		if err := b.sync(ctx); err != nil {
			return nil, err
		}
	}
	claims, err := b.scan(ctx)
	if err != nil {
		return nil, err
	}
	now := b.now()
	var active []claim
	var bad []MalformedClaim
	for _, c := range claims {
		switch {
		case c.reason != "":
			bad = append(bad, MalformedClaim{Ref: c.ref, Reason: c.reason})
		case isActive(c.lease, now):
			active = append(active, c)
		}
	}
	sort.SliceStable(active, func(i, j int) bool { return active[i].num < active[j].num })
	leases := make([]Lease, len(active))
	for i, c := range active {
		leases[i] = c.lease
	}
	if len(bad) > 0 {
		return leases, &MalformedClaimsError{Claims: bad}
	}
	return leases, nil
}

// ClaimOID returns the commit OID of the ticket's claim ref, or ErrNotFound.
// In remote mode it syncs first. It exists mainly for tests and diagnostics.
func (b *GitBackend) ClaimOID(ctx context.Context, ticketID string) (string, error) {
	if b.relay {
		return "", ErrRelayNotImplemented
	}
	ref, err := RefName(ticketID)
	if err != nil {
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
