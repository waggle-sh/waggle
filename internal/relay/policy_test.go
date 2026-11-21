// SPDX-License-Identifier: Apache-2.0

package relay

import (
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
)

var now = time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)

func relayCfg() config.RelayConfig {
	return config.RelayConfig{MaxActiveClaimsPerLogin: 2, MaxTTLMinutes: 1440, ReleaseOnClose: true}
}

func claimReq(branches ...string) Request {
	return Request{Command: CommandClaim, Task: "T-123", Dirs: []string{"internal/pricing"}, Branches: branches, Gate: GateFull}
}

func baseInput(req Request) Input {
	return Input{
		Req: req,
		Event: Event{
			CommentID: 1001, CommentURL: "https://github.com/acme/shop/issues/40#issuecomment-1001",
			Login: "alice", UserType: "User", Association: "NONE", IssueNumber: 40, Repo: "acme/shop",
		},
		Relay: relayCfg(), DefaultTTLMinutes: 120, Now: now,
	}
}

func relayClaim(ticket, owner string, dirs ...string) ActiveClaim {
	return ActiveClaim{
		Lease: comb.Lease{
			TicketID: ticket, KeeperID: owner, Role: "builder", Gate: GateFull, Directories: dirs,
			Branches: []string{owner + "/shop:x"}, ClaimedAt: now.Add(-time.Hour), ExpiresAt: now.Add(time.Hour),
		},
		RelayRequester: owner,
	}
}

func wantRefuse(t *testing.T, d Decision) {
	t.Helper()
	if d.Kind != KindRefuse || d.Reason == "" {
		t.Fatalf("decision = %+v; want a refusal with a reason", d)
	}
}

// A3: fork entries.
func TestPolicyForkBranches(t *testing.T) {
	ok := baseInput(claimReq("alice/shop:t-123"))
	if d := Decide(ok); d.Kind != KindPending {
		t.Fatalf("own fork: %+v", d)
	}
	folded := baseInput(claimReq("ALICE/shop:t-123", "alice/other:x"))
	if d := Decide(folded); d.Kind != KindPending {
		t.Errorf("case folded owner and two forks: %+v", d)
	}
	for name, branches := range map[string][]string{
		"another owner":   {"bob/shop:t-123"},
		"mixed owners":    {"alice/shop:a", "bob/shop:b"},
		"short name":      {"t-123"},
		"short with path": {"feature/x"},
		"prefix owner":    {"alic/shop:x"},
		"longer owner":    {"alice2/shop:x"},
		"none":            nil,
	} {
		t.Run(name, func(t *testing.T) {
			d := Decide(baseInput(claimReq(branches...)))
			wantRefuse(t, d)
			if name == "none" && !strings.Contains(d.Reason, "--branch") {
				t.Errorf("no hint in %q", d.Reason)
			}
		})
	}
}

// A4: caps.
func TestPolicyCaps(t *testing.T) {
	in := baseInput(claimReq("alice/shop:t-123"))
	in.Active = []ActiveClaim{relayClaim("T-001", "alice", "a")}
	if d := Decide(in); d.Kind != KindPending {
		t.Fatalf("second claim at cap 2: %+v", d)
	}
	in.Active = append(in.Active, relayClaim("T-002", "Alice", "b"))
	wantRefuse(t, Decide(in))

	// Not counted: another login's claim, a maintainer claim, an expired
	// claim, and a claim whose newest relay requester is someone else.
	in.Active = []ActiveClaim{
		relayClaim("T-001", "bob", "a"),
		{Lease: relayClaim("T-002", "alice", "b").Lease},
		func() ActiveClaim { c := relayClaim("T-003", "alice", "c"); c.Lease.ExpiresAt = now; return c }(),
		func() ActiveClaim { c := relayClaim("T-004", "alice", "d"); c.RelayRequester = "bob"; return c }(),
	}
	if d := Decide(in); d.Kind != KindPending {
		t.Errorf("uncounted claims: %+v", d)
	}

	// A renewal of a ticket already held does not count twice.
	in.Active = []ActiveClaim{relayClaim("T-001", "alice", "a"), relayClaim("T-123", "alice", "internal/pricing")}
	if d := Decide(in); d.Kind == KindRefuse {
		t.Errorf("renewal at cap: %+v", d)
	}

	in = baseInput(claimReq("alice/shop:t-123"))
	in.Relay.MaxActiveClaimsPerLogin = 1
	in.Active = []ActiveClaim{relayClaim("T-001", "alice", "a")}
	wantRefuse(t, Decide(in))
}

func TestPolicyTTL(t *testing.T) {
	in := baseInput(claimReq("alice/shop:t-123"))
	in.Req.TTLMinutes = 1441
	wantRefuse(t, Decide(in))
	in.Req.TTLMinutes = 1440
	if d := Decide(in); d.Kind != KindPending {
		t.Errorf("at cap: %+v", d)
	}
	// Default is the lesser of leases.default_ttl_minutes and the relay cap.
	in = baseInput(claimReq("alice/shop:t-123"))
	in.Relay.AutoApproveAssociations = []string{"OWNER"}
	in.Event.Association = "OWNER"
	if d := Decide(in); d.Claim.TTLMinutes != 120 {
		t.Errorf("default ttl = %d, want 120", d.Claim.TTLMinutes)
	}
	in.Relay.MaxTTLMinutes = 30
	if d := Decide(in); d.Claim.TTLMinutes != 30 {
		t.Errorf("capped default ttl = %d, want 30", d.Claim.TTLMinutes)
	}
	in.Req.TTLMinutes = 31
	wantRefuse(t, Decide(in))
}

func TestPolicyAllowedDirectories(t *testing.T) {
	in := baseInput(claimReq("alice/shop:t-123"))
	in.Relay.AllowedDirectories = []string{"internal/pricing", "docs/"}
	if d := Decide(in); d.Kind != KindPending {
		t.Errorf("exact hit: %+v", d)
	}
	in.Req.Dirs = []string{"docs/guide", "internal/pricing/sub"}
	if d := Decide(in); d.Kind != KindPending {
		t.Errorf("hit under prefix: %+v", d)
	}
	for _, dirs := range [][]string{
		{"internal"}, {"internal/pricing2"}, {"docs2"}, {"docs/guide", "cmd"}, {"internal/pricingx/a"},
	} {
		in.Req.Dirs = dirs
		wantRefuse(t, Decide(in))
	}
	in.Relay.AllowedDirectories = nil
	in.Req.Dirs = []string{"cmd"}
	if d := Decide(in); d.Kind != KindPending {
		t.Errorf("empty list limits nothing: %+v", d)
	}
}

// A5: approval.
func TestPolicyApproval(t *testing.T) {
	for _, assoc := range []string{"OWNER", "MEMBER", "COLLABORATOR", "CONTRIBUTOR", "FIRST_TIME_CONTRIBUTOR", "NONE"} {
		in := baseInput(claimReq("alice/shop:t-123"))
		in.Event.Association = assoc
		d := Decide(in)
		if d.Kind != KindPending || len(d.Digest) != 12 {
			t.Errorf("empty list, %s: %+v", assoc, d)
		}
	}
	in := baseInput(claimReq("alice/shop:t-123"))
	in.Relay.AutoApproveAssociations = []string{"MEMBER"}
	for assoc, want := range map[string]Kind{"MEMBER": KindWrite, "OWNER": KindPending, "COLLABORATOR": KindPending, "NONE": KindPending} {
		in.Event.Association = assoc
		d := Decide(in)
		if d.Kind != want {
			t.Errorf("MEMBER listed, %s: %+v", assoc, d)
		}
		if want == KindWrite && (d.Claim.ApprovedBy != "auto:MEMBER" || d.Claim.Owner != "alice") {
			t.Errorf("write claim: %+v", d.Claim)
		}
	}

	// Renewal: same owner, same dirs, gate and branches, before expiry.
	renew := baseInput(claimReq("alice/shop:x"))
	renew.Req.Task = "T-123"
	existing := relayClaim("T-123", "alice", "internal/pricing")
	renew.Active = []ActiveClaim{existing}
	if d := Decide(renew); d.Kind != KindWrite || d.Claim.ApprovedBy != "auto:renewal" {
		t.Errorf("identical renewal: %+v", d)
	}
	renew.Active = []ActiveClaim{func() ActiveClaim { c := existing; c.Lease.KeeperID = "ALICE"; return c }()}
	if d := Decide(renew); d.Kind != KindWrite {
		t.Errorf("case folded owner renewal: %+v", d)
	}
	for name, mutate := range map[string]func(*Input, *ActiveClaim){
		"new dir":     func(in *Input, _ *ActiveClaim) { in.Req.Dirs = []string{"internal/pricing", "docs"} },
		"new gate":    func(in *Input, _ *ActiveClaim) { in.Req.Gate = GateLight },
		"new branch":  func(in *Input, _ *ActiveClaim) { in.Req.Branches = []string{"alice/shop:y"} },
		"more branch": func(in *Input, _ *ActiveClaim) { in.Req.Branches = []string{"alice/shop:x", "alice/shop:y"} },
		"expired":     func(_ *Input, a *ActiveClaim) { a.Lease.ExpiresAt = now },
		"other owner": func(_ *Input, a *ActiveClaim) { a.Lease.KeeperID = "bob" },
	} {
		in := baseInput(claimReq("alice/shop:x"))
		a := existing
		mutate(&in, &a)
		in.Active = []ActiveClaim{a}
		if d := Decide(in); d.Kind != KindPending {
			t.Errorf("%s: %+v; want pending", name, d)
		}
	}
}

// A6: digest.
func TestDigest(t *testing.T) {
	c := Claim{Task: "T-123", Dirs: []string{"a", "b"}, Gate: GateFull, Branches: []string{"alice/shop:x", "alice/shop:y"}, TTLMinutes: 120}
	d := Digest(c, "alice", 1001)
	if len(d) != 12 || strings.Trim(d, "0123456789abcdef") != "" {
		t.Fatalf("digest %q is not 12 hex digits", d)
	}
	if Digest(c, "alice", 1001) != d || Digest(c, "ALICE", 1001) != d {
		t.Error("digest is not stable or not case folded on the login")
	}
	changes := map[string]func(Claim) (Claim, string, int64){
		"task":      func(c Claim) (Claim, string, int64) { c.Task = "T-124"; return c, "alice", 1001 },
		"dir":       func(c Claim) (Claim, string, int64) { c.Dirs = []string{"a", "c"}; return c, "alice", 1001 },
		"more dirs": func(c Claim) (Claim, string, int64) { c.Dirs = []string{"a", "b", "c"}; return c, "alice", 1001 },
		"gate":      func(c Claim) (Claim, string, int64) { c.Gate = GateLight; return c, "alice", 1001 },
		"branch": func(c Claim) (Claim, string, int64) {
			c.Branches = []string{"alice/shop:x", "alice/shop:z"}
			return c, "alice", 1001
		},
		"order": func(c Claim) (Claim, string, int64) {
			c.Branches = []string{"alice/shop:y", "alice/shop:x"}
			return c, "alice", 1001
		},
		"ttl":       func(c Claim) (Claim, string, int64) { c.TTLMinutes = 121; return c, "alice", 1001 },
		"requester": func(c Claim) (Claim, string, int64) { return c, "bob", 1001 },
		"comment":   func(c Claim) (Claim, string, int64) { return c, "alice", 1002 },
		"split dir": func(c Claim) (Claim, string, int64) { c.Dirs = []string{"a=b"}; return c, "alice", 1001 },
	}
	for name, f := range changes {
		cc, who, id := f(c)
		if Digest(cc, who, id) == d {
			t.Errorf("%s did not change the digest", name)
		}
	}
}

func approvalInput(t *testing.T) ApprovalInput {
	t.Helper()
	orig := baseInput(claimReq("alice/shop:t-123"))
	pending := Decide(orig)
	if pending.Kind != KindPending {
		t.Fatalf("setup: %+v", pending)
	}
	body := "/waggle claim T-123 --dirs internal/pricing --branch alice/shop:t-123"
	oe := orig.Event
	oe.Body = body
	in := baseInput(Request{Command: CommandApprove, Digest: pending.Digest})
	in.Event = Event{CommentID: 1005, CommentURL: "https://github.com/acme/shop/issues/40#issuecomment-1005", Login: "maint", UserType: "User", Association: "OWNER", IssueNumber: 40, Repo: "acme/shop"}
	return ApprovalInput{Input: in, Original: body, OriginalEvent: oe, Approvers: []string{"Maint", "other"}}
}

func TestDecideApproval(t *testing.T) {
	in := approvalInput(t)
	d := DecideApproval(in)
	if d.Kind != KindWrite || d.Claim.Owner != "alice" || d.Claim.ApprovedBy != "maint" ||
		d.Claim.Task != "T-123" || d.Claim.TTLMinutes != 120 || d.Claim.Branches[0] != "alice/shop:t-123" {
		t.Fatalf("approval: %+v", d)
	}
	tr := d.Claim.Trailers(in.OriginalEvent.CommentURL, in.OriginalEvent.Login)
	if len(tr) != 3 || tr[0].Key != comb.TrailerRelayRequest || tr[1].Value != "alice" || tr[2].Value != "maint" {
		t.Errorf("trailers: %+v", tr)
	}

	t.Run("requester approves self", func(t *testing.T) {
		in := approvalInput(t)
		in.Event.Login = "alice"
		in.Approvers = append(in.Approvers, "alice")
		wantRefuse(t, DecideApproval(in))
		in.Event.Login = "ALICE"
		wantRefuse(t, DecideApproval(in))
	})
	t.Run("not an approver", func(t *testing.T) {
		in := approvalInput(t)
		in.Event.Login = "carol"
		in.Event.Association = "OWNER"
		wantRefuse(t, DecideApproval(in))
	})
	t.Run("no approvers configured", func(t *testing.T) {
		in := approvalInput(t)
		in.Approvers = nil
		d := DecideApproval(in)
		wantRefuse(t, d)
		if !strings.Contains(d.Reason, "gate.approvers") {
			t.Errorf("reason %q", d.Reason)
		}
	})
	t.Run("edited request", func(t *testing.T) {
		for _, body := range []string{
			"/waggle claim T-123 --dirs internal/pricing,internal/billing --branch alice/shop:t-123",
			"/waggle claim T-123 --dirs internal/pricing --branch alice/shop:t-123 --gate light",
			"/waggle claim T-123 --dirs internal/pricing --branch alice/shop:other",
			"/waggle claim T-123 --dirs internal/pricing --branch alice/shop:t-123 --ttl 600m",
			"/waggle claim T-124 --dirs internal/pricing --branch alice/shop:t-123",
		} {
			in := approvalInput(t)
			in.Original = body
			wantRefuse(t, DecideApproval(in))
		}
	})
	t.Run("request deleted or no longer a claim", func(t *testing.T) {
		for _, body := range []string{"", "thanks", "/waggle release T-123", "/waggle claim T-123 --owner bob --dirs a"} {
			in := approvalInput(t)
			in.Original = body
			wantRefuse(t, DecideApproval(in))
		}
	})
	t.Run("different comment", func(t *testing.T) {
		in := approvalInput(t)
		in.OriginalEvent.CommentID = 2002
		wantRefuse(t, DecideApproval(in))
	})
	t.Run("wrong digest", func(t *testing.T) {
		in := approvalInput(t)
		in.Req.Digest = "000000000000"
		wantRefuse(t, DecideApproval(in))
	})
	t.Run("caps still apply at approval", func(t *testing.T) {
		in := approvalInput(t)
		in.Active = []ActiveClaim{relayClaim("T-001", "alice", "a"), relayClaim("T-002", "alice", "b")}
		wantRefuse(t, DecideApproval(in))
	})
	t.Run("config tightened after the request", func(t *testing.T) {
		in := approvalInput(t)
		in.Relay.AllowedDirectories = []string{"docs"}
		wantRefuse(t, DecideApproval(in))
	})
	t.Run("default ttl changed after the request", func(t *testing.T) {
		in := approvalInput(t)
		in.DefaultTTLMinutes = 600
		wantRefuse(t, DecideApproval(in))
	})
	t.Run("not an approve command", func(t *testing.T) {
		in := approvalInput(t)
		in.Req = Request{Command: CommandRelease, Task: "T-123"}
		wantRefuse(t, DecideApproval(in))
	})
}

func TestDecideRelease(t *testing.T) {
	ev := Event{Login: "Alice"}
	req := Request{Command: CommandRelease, Task: "T-123"}
	held := comb.Lease{TicketID: "T-123", KeeperID: "alice"}
	if d := DecideRelease(req, ev, &held); d.Kind != KindWrite || d.Claim.Owner != "alice" || d.Claim.Task != "T-123" {
		t.Errorf("owner: %+v", d)
	}
	other := comb.Lease{TicketID: "T-123", KeeperID: "bob"}
	wantRefuse(t, DecideRelease(req, ev, &other))
	wantRefuse(t, DecideRelease(req, ev, nil))
	wantRefuse(t, DecideRelease(Request{Command: CommandClaim}, ev, &held))
}

// Refusal reasons never carry request text.
func TestPolicyReasonsAreFixed(t *testing.T) {
	in := baseInput(claimReq("EVIL$(id)/x:y"))
	if d := Decide(in); strings.Contains(d.Reason, "EVIL") || strings.Contains(d.Reason, "$(") {
		t.Errorf("reason %q", d.Reason)
	}
}
