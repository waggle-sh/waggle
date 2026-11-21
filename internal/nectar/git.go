// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"
)

var oidPattern = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// repo runs the git binary against one directory. The environment drops every
// source of state that could differ between clones: user and system config,
// replace refs and grafts, optional locks.
type repo struct{ dir string }

func (g repo) run(ctx context.Context, stdin []byte, args ...string) ([]byte, error) {
	full := append([]string{"-C", g.dir, "-c", "log.showSignature=false", "-c", "core.quotepath=false"}, args...)
	cmd := exec.CommandContext(ctx, "git", full...)
	cmd.Env = append(os.Environ(), "LC_ALL=C", "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null",
		"GIT_NO_REPLACE_OBJECTS=1", "GIT_OPTIONAL_LOCKS=0")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("nectar: git %s: %w: %s", args[0], err, strings.TrimSpace(errb.String()))
	}
	return out.Bytes(), nil
}

// batch reads objects by spec with one cat-file process. A missing object is a
// nil entry.
func (g repo) batch(ctx context.Context, specs []string) ([][]byte, error) {
	if len(specs) == 0 {
		return nil, nil
	}
	out, err := g.run(ctx, []byte(strings.Join(specs, "\n")+"\n"), "cat-file", "--batch")
	if err != nil {
		return nil, err
	}
	res := make([][]byte, len(specs))
	for i := range specs {
		nl := bytes.IndexByte(out, '\n')
		if nl < 0 {
			return nil, fmt.Errorf("nectar: truncated cat-file output")
		}
		hdr := strings.Fields(string(out[:nl]))
		out = out[nl+1:]
		if len(hdr) > 0 && hdr[len(hdr)-1] == "missing" {
			continue
		}
		if len(hdr) != 3 {
			return nil, fmt.Errorf("nectar: bad cat-file header")
		}
		n, err := strconv.Atoi(hdr[2])
		if err != nil || n < 0 || n+1 > len(out) {
			return nil, fmt.Errorf("nectar: bad cat-file size")
		}
		res[i] = out[:n]
		out = out[n+1:]
	}
	return res, nil
}

// commit is one commit as the scorer reads it.
type commit struct {
	idx     int // position in the walk, oldest first
	oid     string
	parents []string
	at      time.Time // committer time
	subject string
	trailer string // Waggle-Ticket trailer values, comma separated, as the gate reads them
	body    string
	ident   string // author and committer name and email, for receipts
}

const (
	fieldSep = "\x1f"
	// logFormat carries the same trailer expression the gate uses (cmd/waggle/gate_commits.go).
	logFormat = "--format=%H" + "%x1f" + "%P" + "%x1f" + "%ct" + "%x1f" + "%s" + "%x1f" +
		"%(trailers:key=Waggle-Ticket,valueonly,unfold,separator=%x2C)" + "%x1f" + "%an <%ae> / %cn <%ce>" + "%x1f" + "%b"
)

// log reads commits with git log -z. The body is the last field, so a field
// separator inside a message cannot shift the others.
func (g repo) log(ctx context.Context, args ...string) ([]commit, error) {
	out, err := g.run(ctx, nil, append([]string{"log", "-z", logFormat}, args...)...)
	if err != nil {
		return nil, err
	}
	var cs []commit
	for _, rec := range strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00") {
		if rec == "" {
			continue
		}
		f := strings.SplitN(rec, fieldSep, 7)
		if len(f) != 7 || !oidPattern.MatchString(f[0]) {
			return nil, fmt.Errorf("nectar: unexpected git log record")
		}
		ct, err := strconv.ParseInt(f[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("nectar: bad committer time")
		}
		cs = append(cs, commit{idx: len(cs), oid: f[0], parents: strings.Fields(f[1]), at: time.Unix(ct, 0).UTC(),
			subject: f[3], trailer: f[4], ident: f[5], body: f[6]})
	}
	return cs, nil
}

// resolveCommit checks that oid names a commit in the repo and returns the
// hash length. The error names the OID, never an empty score.
func (g repo) resolveCommit(ctx context.Context, what, oid string) error {
	if !oidPattern.MatchString(oid) {
		return fmt.Errorf("nectar: %s %q is not a full commit OID", what, oid)
	}
	out, err := g.run(ctx, nil, "rev-parse", "--verify", "--quiet", "--end-of-options", oid+"^{commit}")
	if err != nil || strings.TrimSpace(string(out)) != oid {
		return fmt.Errorf("nectar: %s %s is not a commit in this repository", what, oid)
	}
	return nil
}
