// SPDX-License-Identifier: Apache-2.0

//go:build unix

package main

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/waggle-sh/waggle/internal/forge"
	"github.com/waggle-sh/waggle/internal/nectar"
)

type fakeCommitForge struct {
	pr  forge.MergedPull
	ok  bool
	err error
}

func (f fakeCommitForge) MergedPRByCommit(context.Context, string) (forge.MergedPull, bool, error) {
	return f.pr, f.ok, f.err
}

// The forge fields come from the pull request merged as the commit, once; a
// failed read leaves them null, marks the receipt incomplete and warns.
func TestNectarGateReceiptForgeFields(t *testing.T) {
	for _, c := range []struct {
		name       string
		lookup     fakeCommitForge
		pr         any
		incomplete bool
		warnings   int
	}{
		{"found", fakeCommitForge{pr: forge.MergedPull{Number: 12, Author: "carol", HeadBranch: "t-023", Approvers: []string{"dave"}}, ok: true}, float64(12), false, 0},
		{"branch not in claim", fakeCommitForge{pr: forge.MergedPull{Number: 12, Author: "carol", HeadBranch: "other"}, ok: true}, nil, false, 0},
		{"no pull request", fakeCommitForge{}, nil, false, 0},
		{"no forge", fakeCommitForge{err: nectar.ErrNoForge}, nil, true, 0},
		{"failed read", fakeCommitForge{err: errors.New("boom")}, nil, true, 1},
	} {
		t.Run(c.name, func(t *testing.T) {
			repo, origin, before := gateReceiptRepo(t)
			after := commitGateFile(t, repo, "internal/x/s.go", "T-023: squash")
			git(t, repo, "push", "-q", "origin", "main")
			res, err := nectar.GateReceipt(context.Background(), nectar.GateReceiptRequest{RepoDir: repo, Before: before, After: after, Actor: "push-actor", Forge: c.lookup})
			if err != nil || res.Written != 1 || len(res.Warnings) != c.warnings {
				t.Fatalf("res %+v err %v", res, err)
			}
			var raw map[string]any
			if err := json.Unmarshal([]byte(gitStdout(t, origin, "show", "refs/waggle/receipts/main:receipt.json")), &raw); err != nil {
				t.Fatal(err)
			}
			if raw["pr_number"] != c.pr || raw["incomplete"] != c.incomplete {
				t.Errorf("pr_number %v incomplete %v, want %v %v", raw["pr_number"], raw["incomplete"], c.pr, c.incomplete)
			}
			if c.pr != nil && (raw["pr_author"] != "carol" || raw["head_branch"] != "t-023" || len(raw["approvers"].([]any)) != 1) {
				t.Errorf("receipt %v", raw)
			}
		})
	}
}
