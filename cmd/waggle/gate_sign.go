// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// T-071: the Head Keeper approves a protected-path PR by signing its head
// commit with an SSH key (git commit -S under gpg.format=ssh). The gate checks
// that signature with ssh-keygen -Y verify (D-372, Q-T054-1) against the
// allowed_signers file of the PR base commit, under the Head Keeper's
// principal. It reads no Quorum ref or registry.
const (
	// allowedSignersPath is read from the base commit only, never from the
	// head tree or the worktree, so a PR cannot add its own key.
	allowedSignersPath = ".waggle/control/allowed_signers"
	maxAllowedSigners  = 64 << 10
	// maxCommitObject caps the head commit object the gate parses.
	maxCommitObject = 1 << 20
	// sigNamespace is the namespace git uses for commit signatures.
	sigNamespace = "git"
)

// keeperSig is the outcome of one Head Keeper signature check.
type keeperSig struct {
	ok  bool
	why string // plain-terms reason a present signature does not count; "" when ok or unsigned
}

// headKeeperSigned reports whether the Head Keeper, the first entry of
// gate.approvers at base, signed exactly the commit headOID. Only a git
// failure is an error; every other problem is a keeperSig with ok false.
func headKeeperSigned(g gitRunner, baseOID, headOID string, approvers []string) (keeperSig, error) {
	payload, sig, err := commitSignature(g, headOID)
	switch {
	case err != nil:
		return keeperSig{}, err
	case sig == nil:
		return keeperSig{}, nil
	case len(approvers) == 0:
		return keeperSig{why: "no Head Keeper: gate.approvers is empty at base"}, nil
	}
	signers, why, err := baseSigners(g, baseOID)
	if err != nil || why != "" {
		return keeperSig{why: why}, err
	}
	dir, err := os.MkdirTemp("", "waggle-sig-")
	if err != nil {
		return keeperSig{why: "temp dir unavailable"}, nil
	}
	defer os.RemoveAll(dir)
	sigFile, signersFile := filepath.Join(dir, "head.sig"), filepath.Join(dir, "allowed_signers")
	if os.WriteFile(sigFile, sig, 0o600) != nil || os.WriteFile(signersFile, signers, 0o600) != nil {
		return keeperSig{why: "temp file unwritable"}, nil
	}
	bin, err := exec.LookPath("ssh-keygen")
	if err != nil {
		return keeperSig{why: "ssh-keygen not found"}, nil
	}
	cmd := exec.CommandContext(g.ctx, bin, "-Y", "verify", "-f", signersFile,
		"-I", approvers[0], "-n", sigNamespace, "-s", sigFile)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	cmd.Stdin = bytes.NewReader(payload)
	// ssh-keygen output names keys and principals; it is never echoed.
	if cmd.Run() != nil {
		return keeperSig{why: "the signature does not verify for the Head Keeper's key in " + allowedSignersPath + " at base"}, nil
	}
	return keeperSig{ok: true}, nil
}

// commitSignature returns the signed payload of commit oid and its signature,
// or a nil signature when the commit carries none for the repository's hash.
// The payload is the commit object with every gpgsig and gpgsig-sha256 header
// removed, as git verify-commit builds it.
func commitSignature(g gitRunner, oid string) (payload, sig []byte, err error) {
	out, err := g.run("cat-file", "-s", oid)
	if err != nil {
		return nil, nil, err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, nil, fmt.Errorf("%w cat-file -s: unexpected output %q", errGit, out)
	}
	if size > maxCommitObject {
		return nil, nil, fmt.Errorf("%w: head commit object is %d bytes, over the %d byte cap", errGit, size, maxCommitObject)
	}
	raw, err := g.run("cat-file", "commit", oid)
	if err != nil {
		return nil, nil, err
	}
	payload, sig, _ = splitSignature(raw, len(oid) == 64)
	return payload, sig, nil
}

// splitSignature parses a raw commit object. sha256 picks gpgsig-sha256 as the
// signature header, else gpgsig. A second signature header of that name makes
// the commit count as unsigned (bad true).
func splitSignature(raw []byte, sha256 bool) (payload, sig []byte, bad bool) {
	want := "gpgsig"
	if sha256 {
		want = "gpgsig-sha256"
	}
	header, body, found := bytes.Cut(raw, []byte("\n\n"))
	var p, s bytes.Buffer
	var inSig, inOther, seen bool
	for _, line := range strings.SplitAfter(string(header), "\n") {
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, " ") && (inSig || inOther) {
			if inSig {
				s.WriteString(strings.TrimSuffix(line[1:], "\n") + "\n")
			}
			continue
		}
		inSig, inOther = false, false
		name, value, _ := strings.Cut(line, " ")
		switch name {
		case want:
			if seen {
				bad = true
			}
			seen, inSig = true, true
			s.WriteString(strings.TrimSuffix(value, "\n") + "\n")
			continue
		case "gpgsig", "gpgsig-sha256":
			inOther = true
			continue
		}
		p.WriteString(line)
	}
	if found {
		if !strings.HasSuffix(p.String(), "\n") {
			p.WriteString("\n")
		}
		p.WriteString("\n")
		p.Write(body)
	}
	if !seen || bad {
		return p.Bytes(), nil, bad
	}
	return p.Bytes(), s.Bytes(), false
}

// baseSigners reads allowedSignersPath from baseOID. why is a plain-terms
// reason when the file is absent or not a regular file under the cap.
func baseSigners(g gitRunner, baseOID string) (data []byte, why string, err error) {
	out, err := g.run("ls-tree", "-z", "--full-tree", baseOID, "--", allowedSignersPath)
	if err != nil {
		return nil, "", err
	}
	if len(out) == 0 {
		return nil, "no " + allowedSignersPath + " at base", nil
	}
	recs := strings.Split(strings.TrimSuffix(string(out), "\x00"), "\x00")
	meta, _, _ := strings.Cut(recs[0], "\t")
	f := strings.Fields(meta)
	if len(f) != 3 || !oidPattern.MatchString(f[2]) {
		return nil, "", fmt.Errorf("%w ls-tree: unexpected output %q", errGit, recs[0])
	}
	if len(recs) != 1 || f[1] != "blob" || (f[0] != "100644" && f[0] != "100755") {
		return nil, allowedSignersPath + " at base is not a regular file", nil
	}
	out, err = g.run("cat-file", "-s", f[2])
	if err != nil {
		return nil, "", err
	}
	size, err := strconv.Atoi(strings.TrimSpace(string(out)))
	if err != nil {
		return nil, "", fmt.Errorf("%w cat-file -s: unexpected output %q", errGit, out)
	}
	if size > maxAllowedSigners {
		return nil, fmt.Sprintf("%s at base is over the %d byte cap", allowedSignersPath, maxAllowedSigners), nil
	}
	data, err = g.run("cat-file", "blob", f[2])
	return data, "", err
}
