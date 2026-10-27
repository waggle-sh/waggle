// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// DefaultGateCheck is the status check name the gate workflow posts.
const DefaultGateCheck = "verify-leases-and-tiers"

// probeGit runs git for the claim-refs probe; tests replace it. It returns
// stdout even on failure, like gitRun.
var probeGit = probeGitRun

// probeGitRun is gitRun with GIT_NO_LAZY_FETCH=1, so a local object check in
// a partial clone cannot fetch from the promisor remote (git 2.44 or later;
// older versions ignore it).
func probeGitRun(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(childEnv(os.Environ()), "GIT_NO_LAZY_FETCH=1")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return stdout.Bytes(), fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), nil
}

// ProbeClaimRefs checks that this clone can push claim refs to remote. With
// write true it pushes two temporary refs under refs/waggle/probe/ in one
// atomic push and deletes them again; notice (if not nil) gets one line
// before the first push. It never fetches, never forces and never retries,
// and no git or remote text reaches the finding.
func ProbeClaimRefs(ctx context.Context, dir, remote string, write bool, notice io.Writer) Finding {
	f := Finding{ID: "claim-refs", Required: true}
	unknown := func(summary, fix string) Finding {
		f.Status, f.Summary, f.Fix = StatusUnknown, summary, fix
		f.NotEnforced = "push access to refs/waggle/* was not tested, so claims may not reach the remote."
		return f
	}
	if !remotePattern.MatchString(remote) {
		return unknown("The remote name is not valid.", "set forge.remote to a remote name that git remote lists.")
	}
	out, err := probeGit(ctx, dir, "ls-remote", "--symref", remote, "HEAD")
	tip := ""
	for _, line := range strings.Split(string(out), "\n") {
		if oid, ref, ok := strings.Cut(line, "\t"); ok && ref == "HEAD" && oidPattern.MatchString(oid) {
			tip = oid
		}
	}
	if err != nil || tip == "" {
		return unknown("Cannot read "+remote+".", "check the remote URL and your git credentials, then run waggle doctor again.")
	}
	if !write {
		f.Status, f.Summary = StatusSkipped, "Not tested; run waggle doctor without --no-write."
		return f
	}
	if _, err := probeGit(ctx, dir, "cat-file", "-e", tip+"^{commit}"); err != nil {
		return unknown("The tip of "+remote+" is not in this clone, so a test push cannot run.",
			"run git fetch "+remote+", then waggle doctor.")
	}
	a, b, err := probeRefs()
	if err != nil {
		return unknown("Could not pick names for the test refs.", "run waggle doctor again.")
	}
	if notice != nil {
		fmt.Fprintf(notice, "Testing pushes to %s with two temporary refs under refs/waggle/probe/ (deleted afterwards).\n", remote)
	}
	missing := func(summary, fix string) Finding {
		f.Status, f.Summary, f.Fix = StatusMissing, summary, fix
		f.NotEnforced = "claims cannot be pushed to the remote, so two workers can take the same task."
		return f
	}
	pout, err := probeGit(ctx, dir, "push", "--atomic", "--porcelain", remote, tip+":"+a, tip+":"+b)
	if err != nil || rejected(pout) {
		if err != nil && strings.Contains(err.Error(), "does not support --atomic") {
			return missing("This remote does not support atomic pushes, which claims need.",
				"use a git server that supports push --atomic, such as GitHub.")
		}
		return missing("This remote does not accept pushes to refs/waggle/*.",
			"allow your user to create refs under refs/waggle/ on "+remote+".")
	}
	dout, err := probeGit(ctx, dir, "push", "--atomic", "--porcelain", remote, ":"+a, ":"+b)
	if err != nil || rejected(dout) {
		cmd := "git push " + remote + " :" + a + " :" + b
		return missing("The test refs "+a+" and "+b+" were created on "+remote+" but could not be deleted.",
			"delete them with: "+cmd+" (claims are released by deleting refs, so delete access is needed).")
	}
	f.Status, f.Summary = StatusOK, "Pushes to refs/waggle/* on "+remote+" work."
	return f
}

// rejected reports a "!" porcelain status line.
func rejected(out []byte) bool { return strings.Contains("\n"+string(out), "\n!\t") }

// probeRefs picks two ref names that share 16 random hex digits.
func probeRefs() (a, b string, err error) {
	var r [8]byte
	if _, err := rand.Read(r[:]); err != nil {
		return "", "", err
	}
	base := "refs/waggle/probe/" + hex.EncodeToString(r[:])
	return base + "-a", base + "-b", nil
}
