// SPDX-License-Identifier: Apache-2.0

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os/signal"
	"strings"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/dispatch"
)

type forceOKJSON struct {
	Version         int      `json:"version"`
	Command         string   `json:"command"`
	OK              bool     `json:"ok"`
	Task            string   `json:"task"`
	Outcome         string   `json:"outcome"`
	Ref             string   `json:"ref"`
	OID             string   `json:"oid"`
	Owner           string   `json:"owner"`
	Gate            string   `json:"gate"`
	Directories     []string `json:"directories"`
	ExpiresAt       string   `json:"expires_at"`
	Expired         bool     `json:"expired"`
	MalformedReason string   `json:"malformed_reason"`
	Caller          string   `json:"caller"`
	FirstApprover   string   `json:"first_approver"`
	Authorized      bool     `json:"authorized"`
	Base            string   `json:"base"`
	Next            string   `json:"next"`
	AuditOID        string   `json:"audit_oid"`
	Warnings        []string `json:"warnings"`
}

const forceHelp = `Usage: waggle force-release <task> | --ref <ref> [--oid <id> --reason <text>] [--owner <id>] [--repo <dir>] [--json]

Clears a stuck or malformed claim, whoever owns it. Only the first login in
gate.approvers, read from main on the remote (never from local files), may
clear one. It takes two runs and never asks for confirmation:

  1. Without --oid it only looks: it prints the claim, who may clear it, and
     the exact second command with --oid filled in. It writes nothing.
  2. With --oid and --reason it deletes the claim ref, but only if it is still
     at that object id, and records who cleared it and why in
     refs/waggle/audit/force-releases. Review that record with
     git log --first-parent refs/waggle/audit/force-releases

The check is advisory: anyone who can push to the remote can delete a claim
ref with raw git and leave no record. It does not run inside a dispatch
worktree. The reason is stored in the record and never printed.

Flags:`

// shellArg quotes s for a POSIX shell when it holds anything but safe characters.
func shellArg(s string) string {
	if s != "" && strings.Trim(s, "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_./:@+=,") == "" {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

// runForceRelease implements `waggle force-release`: inspect, then execute
// with the inspected object id. Exit codes: 0 inspected, released or no
// claim, 1 interrupted, 2 usage, 3 environment, 4 refused.
// forceFlags is the flag set of waggle force-release. It has no flag that
// skips the inspect step or confirms on the caller's behalf.
type forceFlags struct {
	ref, oid, reason, owner, repo *string
	asJSON                        *bool
}

func newForceFlags(stderr io.Writer) (*flag.FlagSet, forceFlags) {
	fs := flag.NewFlagSet("force-release", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, forceHelp)
		fs.PrintDefaults()
	}
	return fs, forceFlags{
		ref:    fs.String("ref", "", "full claim ref name, as status or an error printed it; instead of <task>"),
		oid:    fs.String("oid", "", "the full object id the first run printed; with it the claim is deleted"),
		reason: fs.String("reason", "", "why the claim is cleared, 1 to 512 bytes; required with --oid, stored in the record, never printed"),
		owner:  fs.String("owner", "", "your id, usually your GitHub login; required without .waggle/local.yaml, else defaults to its keeper_id"),
		repo:   fs.String("repo", ".", "repository"),
		asJSON: fs.Bool("json", false, "print one JSON object"),
	}
}

// runForceRelease implements `waggle force-release`: inspect, then execute
// with the inspected object id. Exit codes: 0 inspected, released or no
// claim, 1 interrupted, 2 usage, 3 environment, 4 refused.
func runForceRelease(args []string, stdout, stderr io.Writer) int {
	fs, fl := newForceFlags(stderr)
	ref, oid, reason, owner, repo, asJSON := fl.ref, fl.oid, fl.reason, fl.owner, fl.repo, fl.asJSON
	task, extra, code, ok := parseTaskArgs(fs, args)
	if !ok {
		return code
	}
	o := taskOut{"force-release", *asJSON, stdout, stderr}
	id, _ := comb.ParseTicketID(task)
	if extra > 0 {
		return o.fail(id, usageError("usage: waggle force-release <task> | --ref <ref> [--oid <id> --reason <text>]"))
	}
	ctx, stop := signal.NotifyContext(context.Background(), stopSignals...)
	defer stop()
	res, err := dispatch.ForceReleaseTask(ctx, dispatch.ForceReleaseRequest{RepoDir: *repo, Ticket: task, Ref: *ref, OID: *oid, Reason: *reason, Owner: *owner})
	o.warn(res.Warnings)
	if err != nil {
		return o.fail(res.Task, err)
	}
	next := ""
	if res.Outcome == "inspected" {
		target := res.Task
		if *ref != "" || target == "" {
			target = "--ref " + res.Ref
		}
		next = "waggle force-release " + target + " --oid " + res.OID + ` --reason "<why>"`
		if *repo != "." {
			next += " --repo " + shellArg(*repo)
		}
	}
	if *asJSON {
		j := forceOKJSON{Version: jsonVersion, Command: "force-release", OK: true, Task: res.Task, Outcome: res.Outcome, Ref: res.Ref, OID: res.OID,
			Directories: []string{}, MalformedReason: res.Malformed, Caller: res.Caller, FirstApprover: res.FirstApprover, Authorized: res.Authorized,
			Base: res.Base, Next: next, AuditOID: res.AuditOID, Warnings: nonNil(res.Warnings)}
		if l := res.Lease; l != nil {
			j.Owner, j.Gate, j.Directories, j.ExpiresAt, j.Expired = l.KeeperID, l.Gate, nonNil(l.Directories), rfc3339(l.ExpiresAt), res.Expired
		}
		o.json(j)
		return 0
	}
	short := func(s string) string { return clean(s[:min(len(s), 12)]) }
	switch res.Outcome {
	case "no_claim":
		fmt.Fprintf(stdout, "no claim at %s\n", clean(res.Ref))
		return 0
	case "released":
		was := "malformed: " + clean(res.Malformed)
		if res.Lease != nil {
			was = "owner " + clean(res.Lease.KeeperID)
		}
		fmt.Fprintf(stdout, "force-released %s (was %s, %s); audit %s\n", clean(res.Ref), short(res.OID), was, short(res.AuditOID))
		return 0
	}
	fmt.Fprintf(stdout, "ref: %s\noid: %s\n", clean(res.Ref), clean(res.OID))
	if l := res.Lease; l != nil {
		ds := make([]string, len(l.Directories))
		for i, d := range l.Directories {
			ds[i] = clean(d)
		}
		fmt.Fprintf(stdout, "owner: %s\ngate: %s\ndirectories: %s\n", clean(l.KeeperID), clean(l.Gate), strings.Join(ds, ", "))
		if res.Expired {
			fmt.Fprintf(stdout, "expired %s; a plain claim takes it over without force-release\n", rfc3339(l.ExpiresAt))
		} else {
			fmt.Fprintf(stdout, "expires: %s\n", rfc3339(l.ExpiresAt))
		}
	} else {
		fmt.Fprintf(stdout, "malformed: %s\n", clean(res.Malformed))
	}
	switch {
	case res.FirstApprover == "":
		fmt.Fprintf(stdout, "allowed: nobody; there is no first login in gate.approvers at %s\n", short(res.Base))
	case res.Authorized:
		fmt.Fprintf(stdout, "allowed: you (%s), the first login in gate.approvers at %s\n", clean(res.Caller), short(res.Base))
	default:
		fmt.Fprintf(stdout, "allowed: only %s (first login in gate.approvers at %s), not %s\n", clean(res.FirstApprover), short(res.Base), clean(res.Caller))
	}
	fmt.Fprintf(stdout, "next: %s\n", next)
	return 0
}
