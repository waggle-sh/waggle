// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
)

// Tree entry names of a round chain commit (RFC-0006 6.1, 6.4).
const (
	RoundFile    = "round.json"
	RoundSigFile = "round.sig"
	VetoFile     = "veto.json"
	VetoSigFile  = "veto.sig"
	TallyFile    = "tally.json"
	TallySigFile = "tally.sig"
	// SignerFile names the keeper_id whose registered key signed the record.
	SignerFile = "signer"
)

// Defaults for a base config without a quorum block (RFC-0006 4.2, 5.1).
const (
	DefaultCredits   = 100
	DefaultRoundDays = 14
)

// Errors of the round lifecycle. Phase D maps them to exit codes.
var (
	ErrNoRound          = errors.New("quorum: no such round")
	ErrClosed           = errors.New("quorum: round is already closed")
	ErrNotClosed        = errors.New("quorum: round is not closed")
	ErrNotApprover      = errors.New("quorum: signer is not a gate.approvers entry at the base commit")
	ErrNotHeadKeeper    = errors.New("quorum: only the Head Keeper may sign a veto")
	ErrKeyNotRegistered = errors.New("quorum: signing key is not registered for that keeper_id at the base commit")
	ErrBadRound         = errors.New("quorum: round record is invalid")
	ErrBadRequest       = errors.New("quorum: invalid request")
	ErrTallyMismatch    = errors.New("quorum: tally does not match a recomputation")
	ErrUnknownVersion   = errors.New("quorum: unknown record version")
)

const (
	maxChain       = 4096
	maxRecordBytes = 1 << 20
	maxOpenTries   = 32
	maxAppendTries = 5
	maxRoadmap     = 1 << 20
	maxTaskBytes   = 256 << 10
)

var (
	decisionPattern = regexp.MustCompile(`^D-[0-9]{1,6}$`)
	roadmapItem     = regexp.MustCompile(`^(T-[0-9]{3,6})( |$)`)
	dependsLine     = regexp.MustCompile(`^\s*[-*]?\s*\*{0,2}Depends on:?\*{0,2}:?\s`)
	taskInText      = regexp.MustCompile(`\bT-[0-9]{3,6}\b`)
)

// Lifecycle runs rounds over one repository: open, veto, close, verify and
// status. It never reads a clock; every time comes from the caller or from
// the round record.
type Lifecycle struct {
	// Comb is the backend over the repository (local, or with a remote).
	Comb *comb.GitBackend
	// RepoDir, GitBinary and Env run the one plain git read Quorum needs (the
	// receipts head). GitBinary defaults to git.
	RepoDir   string
	GitBinary string
	Env       []string
	// SSHKeygen overrides the ssh-keygen binary for verification.
	SSHKeygen string
	// Nectar supplies the pinned snapshot at close and status.
	Nectar NectarSource

	// hookBeforeWrite, when set by tests, runs before an Open write with the
	// 0-based attempt number.
	hookBeforeWrite func(attempt int)
}

// Actor is a keeper_id and the key it signs with.
type Actor struct {
	KeeperID string
	Signer   Signer
}

// OpenRequest is the input of Open.
type OpenRequest struct {
	// Items is the explicit item list. With FromRoadmap set it is empty and
	// the T- lines under the "## Backlog" heading at Base are used.
	Items       []string
	FromRoadmap bool
	// Now is the open time; ClosesAt must be after it.
	Now      time.Time
	ClosesAt time.Time
	// Base is the full commit id whose config, registry, roadmap and task
	// briefs the round reads. MainSHA is the Nectar pin; empty means Base.
	Base    string
	MainSHA string
	// RulesVersion is the Nectar rules version to pin (RFC-0006 5.4); zero
	// means 1.
	RulesVersion int
}

func bad(format string, a ...any) error {
	return fmt.Errorf("%w: %s", ErrBadRequest, fmt.Sprintf(format, a...))
}

func isApprover(approvers []string, id string) bool {
	for _, a := range approvers {
		if a != "" && strings.EqualFold(a, id) {
			return true
		}
	}
	return false
}

// isHeadKeeper matches the first gate.approvers entry as the gate does.
func isHeadKeeper(approvers []string, id string) bool {
	return id != "" && len(approvers) > 0 && strings.EqualFold(id, approvers[0])
}

// baseInfo is what a round reads from its base commit.
type baseInfo struct {
	approvers []string
	credits   uint64
	days      int
	verifier  *Verifier
}

func (l Lifecycle) readBase(ctx context.Context, base string) (baseInfo, error) {
	var bi baseInfo
	data, err := l.Comb.ReadBlobAt(ctx, base, layout.Config, config.MaxFileSize)
	if err != nil {
		return bi, fmt.Errorf("quorum: read config at base: %w", err)
	}
	cfg, err := config.Parse(data)
	if err != nil {
		return bi, fmt.Errorf("quorum: config at base: %w", err)
	}
	bi.approvers = cfg.Gate.Approvers
	if bi.credits, bi.days, err = scanQuorumBlock(data); err != nil {
		return bi, err
	}
	bi.verifier, err = LoadVerifier(ctx, l.Comb, base, l.SSHKeygen)
	return bi, err
}

// scanQuorumBlock reads credits and round_days from an optional top-level
// quorum block. Phase D moves this into internal/config; until then the
// config parser ignores the block as an unknown top-level section.
func scanQuorumBlock(data []byte) (uint64, int, error) {
	credits, days := uint64(DefaultCredits), DefaultRoundDays
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r \t")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if line[0] != ' ' && line[0] != '\t' {
			in = t == "quorum:"
			continue
		}
		if !in {
			continue
		}
		k, v, ok := strings.Cut(t, ":")
		if !ok {
			continue
		}
		v, _, _ = strings.Cut(v, " #")
		n, err := strconv.Atoi(strings.TrimSpace(v))
		switch k {
		case "credits", "round_days":
			if err != nil || n < 1 {
				return 0, 0, fmt.Errorf("quorum: config quorum.%s must be a positive integer", k)
			}
			if k == "credits" {
				credits = uint64(n)
			} else {
				days = n
			}
		}
	}
	return credits, days, nil
}

func canonicalItem(item string) error {
	if err := CheckItem(item); err != nil {
		return err
	}
	if strings.HasPrefix(item, "T-") {
		c, err := comb.ParseTicketID(item)
		if err != nil || c != item {
			return ErrItem
		}
	}
	return nil
}

func checkItems(items []string) error {
	if len(items) == 0 {
		return bad("round has no items")
	}
	seen := map[string]bool{}
	for _, it := range items {
		if err := canonicalItem(it); err != nil {
			return err
		}
		if seen[it] {
			return bad("duplicate item")
		}
		seen[it] = true
	}
	return nil
}

// roadmapItems returns the T- lines under the first heading that starts with
// "## Backlog", in file order, until the next heading.
func roadmapItems(data []byte) []string {
	var out []string
	in := false
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimRight(line, "\r")
		if strings.HasPrefix(line, "#") {
			if in {
				break
			}
			in = strings.HasPrefix(line, "## Backlog")
			continue
		}
		if !in {
			continue
		}
		if m := roadmapItem.FindStringSubmatch(line); m != nil {
			out = append(out, m[1])
		}
	}
	return out
}

// taskDepends returns the T- ids on the Depends on line of a task brief.
func taskDepends(data []byte, self string) []string {
	for _, line := range strings.Split(string(data), "\n") {
		if !dependsLine.MatchString(line) {
			continue
		}
		var out []string
		seen := map[string]bool{self: true}
		for _, m := range taskInText.FindAllString(line, -1) {
			if c, err := comb.ParseTicketID(m); err == nil && !seen[c] {
				seen[c] = true
				out = append(out, c)
			}
		}
		return out
	}
	return nil
}

func (l Lifecycle) git(ctx context.Context, args ...string) (string, error) {
	bin := l.GitBinary
	if bin == "" {
		bin = "git"
	}
	cmd := exec.CommandContext(ctx, bin, append([]string{"-C", l.RepoDir}, args...)...)
	cmd.Env = append(cmd.Environ(), l.Env...)
	var out bytes.Buffer
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		return "", err
	}
	return strings.TrimSpace(out.String()), nil
}

// receiptsHead returns the head of the receipts chain, or the zero OID when
// there is none (before T-051 phase A has written one).
func (l Lifecycle) receiptsHead(ctx context.Context, zero string) (string, error) {
	if _, err := l.Comb.ReadReceipts(ctx); err != nil {
		return "", err
	}
	out, err := l.git(ctx, "for-each-ref", "--format=%(objectname)", comb.ReceiptRef)
	if err != nil {
		return "", err
	}
	if !baseOIDPattern.MatchString(out) {
		return zero, nil
	}
	return out, nil
}

// Open allocates the next round number by create-only compare-and-swap on
// round-<r>/record and writes a signed round.json (RFC-0006 4.2, 5.4, 6.1). A
// lost race moves to the next number. Only a registered key of a
// gate.approvers entry at the base commit may sign. It returns the round
// record as written and the commit OID.
func (l Lifecycle) Open(ctx context.Context, req OpenRequest, actor Actor) (Round, string, error) {
	if l.Comb == nil {
		return Round{}, "", bad("lifecycle needs a backend")
	}
	if !keeperIDPattern.MatchString(actor.KeeperID) {
		return Round{}, "", bad("signer is not a keeper_id")
	}
	if !baseOIDPattern.MatchString(req.Base) {
		return Round{}, "", bad("base is not a full commit id")
	}
	main := req.MainSHA
	if main == "" {
		main = req.Base
	}
	if !baseOIDPattern.MatchString(main) {
		return Round{}, "", bad("main_sha is not a full commit id")
	}
	if req.Now.Unix() < 0 || !req.ClosesAt.After(req.Now) {
		return Round{}, "", bad("closes_at must be after the open time")
	}
	items := append([]string{}, req.Items...)
	if req.FromRoadmap {
		if len(items) != 0 {
			return Round{}, "", bad("give an item list or the roadmap, not both")
		}
	} else if err := checkItems(items); err != nil {
		return Round{}, "", err
	}
	rules := req.RulesVersion
	if rules == 0 {
		rules = 1
	}
	if rules < 0 {
		return Round{}, "", bad("rules_version")
	}

	bi, err := l.readBase(ctx, req.Base)
	if err != nil {
		return Round{}, "", err
	}
	if !isApprover(bi.approvers, actor.KeeperID) {
		return Round{}, "", ErrNotApprover
	}
	if req.FromRoadmap {
		data, err := l.Comb.ReadBlobAt(ctx, req.Base, layout.Roadmap, maxRoadmap)
		if err != nil {
			return Round{}, "", fmt.Errorf("quorum: read roadmap at base: %w", err)
		}
		items = roadmapItems(data)
		if err := checkItems(items); err != nil {
			return Round{}, "", err
		}
	}
	var deps []Dependency
	for _, it := range items {
		if !strings.HasPrefix(it, "T-") {
			continue
		}
		data, err := l.Comb.ReadBlobAt(ctx, req.Base, layout.TasksDir+"/"+it+".md", maxTaskBytes)
		if err != nil {
			continue // no brief at base, no dependencies
		}
		if on := taskDepends(data, it); len(on) > 0 {
			deps = append(deps, Dependency{Item: it, On: on})
		}
	}
	zero := strings.Repeat("0", len(req.Base))
	receipts, err := l.receiptsHead(ctx, zero)
	if err != nil {
		return Round{}, "", err
	}
	if err := l.Comb.FetchObjectNamespace(ctx, RefPrefix); err != nil {
		return Round{}, "", err
	}
	refs, err := l.Comb.ListObjectRefs(ctx, RefPrefix)
	if err != nil {
		return Round{}, "", err
	}
	next := 1
	for _, r := range refs {
		if _, n, _ := ParseRef(r.Ref); n >= next {
			next = n + 1
		}
	}
	opens := req.Now.UTC().Format(time.RFC3339)
	for attempt := 0; attempt < maxOpenTries; attempt, next = attempt+1, next+1 {
		rd := Round{
			Version: Version, Round: next, Credits: bi.credits, RoundDays: bi.days,
			Base: req.Base, OpensAt: opens, ClosesAt: req.ClosesAt.UTC().Format(time.RFC3339),
			Items:    items,
			NectarAt: NectarAt{MainSHA: main, Now: opens, ReceiptsSHA: receipts, RulesVersion: rules},
			Depends:  deps,
		}
		data := Encode(rd)
		sig, err := actor.Signer.Sign(ctx, NamespaceRound, data)
		if err != nil {
			return Round{}, "", err
		}
		if err := bi.verifier.Verify(ctx, NamespaceRound, actor.KeeperID, data, sig); err != nil {
			if errors.Is(err, ErrBadSignature) {
				return Round{}, "", ErrKeyNotRegistered
			}
			return Round{}, "", err
		}
		ref := RecordRef(next)
		oid, err := l.Comb.BuildObjectCommit(ctx, ref, recordFiles(RoundFile, data, RoundSigFile, sig, actor.KeeperID),
			"", roundMessage(next, "open"), req.Now)
		if err != nil {
			return Round{}, "", err
		}
		if l.hookBeforeWrite != nil {
			l.hookBeforeWrite(attempt)
		}
		err = l.Comb.UpdateObjectRef(ctx, ref, oid, "")
		if err == nil {
			return rd, oid, nil
		}
		if !errors.Is(err, comb.ErrRefLost) {
			return Round{}, "", err
		}
	}
	return Round{}, "", errors.New("quorum: could not allocate a round number")
}

func roundMessage(n int, what string) string { return fmt.Sprintf("quorum: round %d %s", n, what) }

func recordFiles(name string, data []byte, sigName string, sig []byte, signer string) []comb.ObjectFile {
	return []comb.ObjectFile{{Name: name, Data: data}, {Name: sigName, Data: sig}, {Name: SignerFile, Data: []byte(signer + "\n")}}
}

// signedRecord is one commit of a round chain.
type signedRecord struct {
	OID    string
	Data   []byte
	Sig    []byte
	Signer string
}

// chain is a round's record ref read and checked up to the first tally.
type chain struct {
	Tip       string
	Round     Round
	Info      baseInfo
	RoundRec  signedRecord
	Vetoes    []signedRecord
	TallyRec  *signedRecord
	roundBase string
}

func decodeStrict(data []byte, v any) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return err
	}
	if _, err := dec.Token(); err != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}

// checkVersion reads only the version field of a record and refuses any
// version this build does not know, before the strict decode can fail on
// fields a newer version added. what names the record in the message.
func checkVersion(data []byte, what string) error {
	var v struct {
		Version *int `json:"version"`
	}
	if json.Unmarshal(data, &v) != nil || v.Version == nil || *v.Version == Version {
		return nil // a malformed record is the strict decoder's to report
	}
	return fmt.Errorf("%w: %s version %d, this build reads version %d", ErrUnknownVersion, what, *v.Version, Version)
}

// DecodeRound decodes round.json, refusing unknown fields and trailing data.
// A version other than Version is ErrUnknownVersion.
func DecodeRound(data []byte) (Round, error) {
	if err := checkVersion(data, "round.json"); err != nil {
		return Round{}, err
	}
	var r Round
	if err := decodeStrict(data, &r); err != nil {
		return Round{}, fmt.Errorf("%w: %v", ErrBadRound, err)
	}
	return r, nil
}

// DecodeTally decodes tally.json, refusing unknown fields and trailing data.
// A version other than Version is ErrUnknownVersion.
func DecodeTally(data []byte) (TallyRecord, error) {
	if err := checkVersion(data, "tally.json"); err != nil {
		return TallyRecord{}, err
	}
	var t TallyRecord
	if err := decodeStrict(data, &t); err != nil {
		return TallyRecord{}, fmt.Errorf("%w: tally: %v", ErrTallyMismatch, err)
	}
	return t, nil
}

func decodeVeto(data []byte) (Veto, error) {
	var v Veto
	if err := decodeStrict(data, &v); err != nil {
		return Veto{}, err
	}
	if !bytes.Equal(Encode(v), data) {
		return Veto{}, errors.New("not canonical")
	}
	return v, nil
}

func (l Lifecycle) readRecord(ctx context.Context, oid string, name, sigName string) (signedRecord, comb.ObjectCommit, bool, error) {
	c, err := l.Comb.ReadObjectCommit(ctx, oid, maxRecordBytes)
	if err != nil {
		if errors.Is(err, comb.ErrBadObject) {
			return signedRecord{}, c, false, nil
		}
		return signedRecord{}, c, false, err
	}
	data, sig, who := c.Files[name], c.Files[sigName], strings.TrimSpace(string(c.Files[SignerFile]))
	if len(c.Files) != 3 || data == nil || sig == nil || !keeperIDPattern.MatchString(who) || len(c.Parents) > 1 {
		return signedRecord{}, c, false, nil
	}
	return signedRecord{OID: oid, Data: data, Sig: sig, Signer: who}, c, true, nil
}

// loadChain fetches the namespace and reads the round chain. The root must be
// a canonical round.json signed by an approver's registered key at its base.
// Vetoes are returned as found, to be judged at close; the first tally ends
// the chain and anything after it is ignored.
func (l Lifecycle) loadChain(ctx context.Context, n int) (*chain, error) {
	if l.Comb == nil {
		return nil, bad("lifecycle needs a backend")
	}
	if n < 1 {
		return nil, ErrNoRound
	}
	if err := l.Comb.FetchObjectNamespace(ctx, RefPrefix); err != nil {
		return nil, err
	}
	tip, err := l.Comb.ReadObjectRef(ctx, RecordRef(n))
	if err != nil {
		return nil, err
	}
	if tip == "" {
		return nil, ErrNoRound
	}
	var oids []string
	for cur := tip; cur != ""; {
		if len(oids) >= maxChain {
			return nil, fmt.Errorf("%w: chain too long", ErrBadRound)
		}
		oids = append(oids, cur)
		c, err := l.Comb.ReadObjectCommit(ctx, cur, maxRecordBytes)
		if err != nil {
			if errors.Is(err, comb.ErrBadObject) {
				return nil, fmt.Errorf("%w: unreadable record", ErrBadRound)
			}
			return nil, err
		}
		switch len(c.Parents) {
		case 0:
			cur = ""
		case 1:
			cur = c.Parents[0]
		default:
			return nil, fmt.Errorf("%w: merge in chain", ErrBadRound)
		}
	}
	ch := &chain{Tip: tip}
	root, _, ok, err := l.readRecord(ctx, oids[len(oids)-1], RoundFile, RoundSigFile)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, fmt.Errorf("%w: root shape", ErrBadRound)
	}
	rd, err := DecodeRound(root.Data)
	if err != nil {
		return nil, err
	}
	if rd.Round != n || rd.Version != Version || !bytes.Equal(Encode(rd), root.Data) || !baseOIDPattern.MatchString(rd.Base) {
		return nil, fmt.Errorf("%w: root content", ErrBadRound)
	}
	if err := checkItems(rd.Items); err != nil {
		return nil, fmt.Errorf("%w: items", ErrBadRound)
	}
	if ch.Info, err = l.readBase(ctx, rd.Base); err != nil {
		return nil, err
	}
	if !isApprover(ch.Info.approvers, root.Signer) {
		return nil, fmt.Errorf("%w: %v", ErrBadRound, ErrNotApprover)
	}
	if err := ch.Info.verifier.Verify(ctx, NamespaceRound, root.Signer, root.Data, root.Sig); err != nil {
		if errors.Is(err, ErrBadSignature) {
			return nil, fmt.Errorf("%w: root signature", ErrBadRound)
		}
		return nil, err
	}
	ch.Round, ch.RoundRec = rd, root
	for i := len(oids) - 2; i >= 0 && ch.TallyRec == nil; i-- {
		if rec, _, ok, err := l.readRecord(ctx, oids[i], TallyFile, TallySigFile); err != nil {
			return nil, err
		} else if ok {
			if err := checkVersion(rec.Data, "tally.json"); err != nil {
				return nil, err
			}
			ch.TallyRec = &rec
			break
		}
		rec, _, ok, err := l.readRecord(ctx, oids[i], VetoFile, VetoSigFile)
		if err != nil {
			return nil, err
		}
		if !ok {
			rec = signedRecord{OID: oids[i]} // undecodable record, reported at close
		}
		ch.Vetoes = append(ch.Vetoes, rec)
	}
	return ch, nil
}

// LoadRound returns the signed round record of round n.
func (l Lifecycle) LoadRound(ctx context.Context, n int) (Round, error) {
	ch, err := l.loadChain(ctx, n)
	if err != nil {
		return Round{}, err
	}
	return ch.Round, nil
}

// Veto appends a signed veto.json to the round chain (RFC-0006 8). item is an
// item id or VetoAll; decision is D-<n>. Only the Head Keeper's registered key
// may sign. The decision entry is checked at close, not here.
func (l Lifecycle) Veto(ctx context.Context, n int, item, decision string, actor Actor) (string, error) {
	if item != VetoAll {
		if err := CheckItem(item); err != nil {
			return "", err
		}
	}
	if !decisionPattern.MatchString(decision) {
		return "", bad("decision must look like D-<n>")
	}
	if !keeperIDPattern.MatchString(actor.KeeperID) {
		return "", bad("signer is not a keeper_id")
	}
	for attempt := 0; ; attempt++ {
		ch, err := l.loadChain(ctx, n)
		if err != nil {
			return "", err
		}
		if ch.TallyRec != nil {
			return "", ErrClosed
		}
		if item != VetoAll && !containsItem(ch.Round.Items, item) {
			return "", bad("item is not in the round")
		}
		if !isHeadKeeper(ch.Info.approvers, actor.KeeperID) {
			return "", ErrNotHeadKeeper
		}
		data := Encode(Veto{Version: Version, Round: n, Item: item, Decision: decision})
		sig, err := actor.Signer.Sign(ctx, NamespaceVeto, data)
		if err != nil {
			return "", err
		}
		if err := ch.Info.verifier.Verify(ctx, NamespaceVeto, actor.KeeperID, data, sig); err != nil {
			if errors.Is(err, ErrBadSignature) {
				return "", ErrKeyNotRegistered
			}
			return "", err
		}
		when, err := ballotTime(ch.Round)
		if err != nil {
			return "", err
		}
		oid, err := l.appendRecord(ctx, n, ch.Tip, recordFiles(VetoFile, data, VetoSigFile, sig, actor.KeeperID), "veto", when)
		if err == nil {
			return oid, nil
		}
		if !errors.Is(err, comb.ErrRefLost) || attempt+1 >= maxAppendTries {
			return "", err
		}
	}
}

func (l Lifecycle) appendRecord(ctx context.Context, n int, tip string, files []comb.ObjectFile, what string, when time.Time) (string, error) {
	ref := RecordRef(n)
	oid, err := l.Comb.BuildObjectCommit(ctx, ref, files, tip, roundMessage(n, what), when)
	if err != nil {
		return "", err
	}
	if err := l.Comb.UpdateObjectRef(ctx, ref, oid, tip); err != nil {
		return "", err
	}
	return oid, nil
}

func containsItem(items []string, it string) bool {
	for _, x := range items {
		if x == it {
			return true
		}
	}
	return false
}
