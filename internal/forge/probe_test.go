// SPDX-License-Identifier: Apache-2.0

package forge

import (
	"bytes"
	"context"
	"errors"
	"regexp"
	"strings"
	"testing"
)

const hostile = "remote: \r::error::boom ghp_secrettoken0123456789"

// gitScript replaces probeGit with canned answers keyed by the subcommand
// (for push: "push" for the creating call, "delete" for the second). It logs
// every argv, and notice lines, in one list.
type gitScript struct {
	log     []string
	answers map[string]struct {
		out string
		err error
	}
}

func (s *gitScript) install(t *testing.T) {
	t.Helper()
	old := probeGit
	t.Cleanup(func() { probeGit = old })
	pushes := 0
	probeGit = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		s.log = append(s.log, strings.Join(args, " "))
		key := args[0]
		if key == "push" {
			pushes++
			if pushes == 2 {
				key = "delete"
			}
		}
		a := s.answers[key]
		return []byte(a.out), a.err
	}
}

type ans = struct {
	out string
	err error
}

func okScript() *gitScript {
	return &gitScript{answers: map[string]ans{
		"ls-remote": {out: "ref: refs/heads/main\tHEAD\n" + oid + "\tHEAD\n"},
		"cat-file":  {},
		"push":      {out: "*\trefs/x:refs/y\t[new reference]\nDone\n"},
		"delete":    {out: "-\t:refs/y\t[deleted]\nDone\n"},
	}}
}

var probeRef = regexp.MustCompile(`^refs/waggle/probe/[0-9a-f]{16}-[ab]$`)

func runClaimProbe(t *testing.T, s *gitScript, write bool) (Finding, string) {
	t.Helper()
	s.install(t)
	var notice bytes.Buffer
	f := ProbeClaimRefs(context.Background(), "/repo", "origin", write, &notice)
	return f, notice.String()
}

func TestProbeClaimRefsOK(t *testing.T) {
	s := okScript()
	f, notice := runClaimProbe(t, s, true)
	if f.Status != StatusOK || !f.Required || f.ID != "claim-refs" {
		t.Fatalf("got %+v", f)
	}
	if !strings.HasPrefix(notice, "Testing pushes to origin with two temporary refs under refs/waggle/probe/") {
		t.Errorf("notice %q", notice)
	}
	if len(s.log) != 4 {
		t.Fatalf("calls: %q", s.log)
	}
	create, del := strings.Fields(s.log[2]), strings.Fields(s.log[3])
	if create[0] != "push" || create[1] != "--atomic" || create[2] != "--porcelain" || create[3] != "origin" {
		t.Errorf("create argv %q", create)
	}
	var names []string
	for _, spec := range create[4:] {
		src, dst, _ := strings.Cut(spec, ":")
		if src != oid || !probeRef.MatchString(dst) {
			t.Errorf("refspec %q", spec)
		}
		names = append(names, dst)
	}
	if len(names) != 2 || del[1] != "--atomic" || del[4] != ":"+names[0] || del[5] != ":"+names[1] {
		t.Errorf("delete argv %q for %q", del, names)
	}
	for _, call := range s.log {
		if strings.Contains(call, "--force") || strings.Contains(call, "+") || strings.HasPrefix(call, "fetch") {
			t.Errorf("forbidden argv %q", call)
		}
	}
	s2 := okScript()
	runClaimProbe(t, s2, true)
	if s.log[2] == s2.log[2] {
		t.Errorf("two runs picked the same names: %q", s.log[2])
	}
}

func TestProbeClaimRefsNoticeBeforePush(t *testing.T) {
	s := okScript()
	var notice bytes.Buffer
	old := probeGit
	t.Cleanup(func() { probeGit = old })
	probeGit = func(ctx context.Context, dir string, args ...string) ([]byte, error) {
		if args[0] == "push" && notice.Len() == 0 {
			t.Error("push ran before the notice line")
		}
		return []byte(s.answers[args[0]].out), nil
	}
	ProbeClaimRefs(context.Background(), "/repo", "origin", true, &notice)
}

func TestProbeClaimRefsNoWrite(t *testing.T) {
	s := okScript()
	f, notice := runClaimProbe(t, s, false)
	if f.Status != StatusSkipped || notice != "" || len(s.log) != 1 || !strings.HasPrefix(s.log[0], "ls-remote") {
		t.Fatalf("got %+v %q %q", f, notice, s.log)
	}
}

func TestProbeClaimRefsTipMissingLocally(t *testing.T) {
	s := okScript()
	s.answers["cat-file"] = ans{err: errors.New("git cat-file: exit status 1: " + hostile)}
	f, notice := runClaimProbe(t, s, true)
	if f.Status != StatusUnknown || !strings.Contains(f.Fix, "git fetch origin") || notice != "" || len(s.log) != 2 {
		t.Fatalf("got %+v %q %q", f, notice, s.log)
	}
}

func TestProbeClaimRefsFailures(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*gitScript)
		want   Status
		in     string
	}{
		{"ls-remote fails", func(s *gitScript) { s.answers["ls-remote"] = ans{err: errors.New(hostile)} }, StatusUnknown, "Cannot read origin"},
		{"bad oid", func(s *gitScript) { s.answers["ls-remote"] = ans{out: "nothex\tHEAD\n"} }, StatusUnknown, "Cannot read origin"},
		{"no atomic", func(s *gitScript) {
			s.answers["push"] = ans{err: errors.New("git push: exit 1: the receiving end does not support --atomic push " + hostile)}
		}, StatusMissing, "atomic"},
		{"push refused", func(s *gitScript) {
			s.answers["push"] = ans{out: "!\trefs/a:refs/b\t[remote rejected]\n", err: errors.New(hostile)}
		}, StatusMissing, "does not accept pushes"},
		{"bang line without error", func(s *gitScript) { s.answers["push"] = ans{out: "!\trefs/a:refs/b\t[rejected]\n"} }, StatusMissing, "does not accept pushes"},
		{"delete fails", func(s *gitScript) { s.answers["delete"] = ans{err: errors.New(hostile)} }, StatusMissing, "could not be deleted"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := okScript()
			tc.mutate(s)
			f, _ := runClaimProbe(t, s, true)
			if f.Status != tc.want || !strings.Contains(f.Summary+f.Fix, tc.in) {
				t.Fatalf("got %+v", f)
			}
			var b bytes.Buffer
			if err := Render(&b, Report{Provider: "github", Findings: []Finding{f}}); err != nil {
				t.Fatal(err)
			}
			for _, bad := range []string{"\r", "::error::", "ghp_secret"} {
				if strings.Contains(b.String(), bad) {
					t.Errorf("report holds hostile text %q:\n%s", bad, b.String())
				}
			}
			if tc.name == "delete fails" {
				var names []string
				for _, w := range strings.Fields(s.log[2]) {
					if _, dst, ok := strings.Cut(w, ":"); ok {
						names = append(names, dst)
					}
				}
				want := "git push origin :" + names[0] + " :" + names[1]
				if !strings.Contains(f.Fix, want) || !strings.Contains(f.Summary, names[0]) || !strings.Contains(f.Summary, names[1]) {
					t.Errorf("fix %q summary %q want %q", f.Fix, f.Summary, want)
				}
			}
		})
	}
}
