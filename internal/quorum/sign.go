// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Signature namespaces (RFC-0006 6.3, 8).
const (
	NamespaceBallot = "waggle-quorum-ballot"
	NamespaceRound  = "waggle-quorum-round"
	NamespaceVeto   = "waggle-quorum-veto"
)

// MaxRegistryBytes caps the Keeper registry, as config.MaxFileSize caps the
// config. A larger file is refused unread.
const MaxRegistryBytes = 64 << 10

// maxSigBytes caps one stored signature.
const maxSigBytes = 8 << 10

// Errors from signing and verification.
var (
	// ErrNoSSHKeygen reports that the ssh-keygen binary is missing, or that it
	// runs but does not support the -Y form Quorum calls (an OpenSSH before
	// 8.0 answers with usage text or an unknown option).
	ErrNoSSHKeygen = errors.New("quorum: ssh-keygen not found; Quorum signs and verifies through OpenSSH ssh-keygen with -Y support")
	// ErrBadSignature reports a signature that does not verify.
	ErrBadSignature = errors.New("quorum: signature does not verify")
	// ErrRegistry reports a registry that is too large or has no usable line.
	ErrRegistry = errors.New("quorum: keeper registry is unusable")
	// ErrNamespace reports a signature namespace Quorum does not use.
	ErrNamespace = errors.New("quorum: unknown signature namespace")
)

func validNamespace(ns string) bool {
	return ns == NamespaceBallot || ns == NamespaceRound || ns == NamespaceVeto
}

// sshKeygen resolves the binary; bin "" means "ssh-keygen" on PATH.
func sshKeygen(bin string) (string, error) {
	if bin == "" {
		bin = "ssh-keygen"
	}
	p, err := exec.LookPath(bin)
	if err != nil {
		return "", ErrNoSSHKeygen
	}
	return p, nil
}

// Signer signs record bytes with an SSH key through ssh-keygen -Y sign.
type Signer struct {
	// KeyPath is the private key file (or the public key of a key held by an
	// agent), as ssh-keygen -f takes it.
	KeyPath string
	// SSHKeygen is the binary; "" means ssh-keygen on PATH.
	SSHKeygen string
}

// Sign returns the SSH signature of data in the given namespace. Data goes
// through stdin, so the signed bytes are exactly the bytes given.
func (s Signer) Sign(ctx context.Context, namespace string, data []byte) ([]byte, error) {
	if !validNamespace(namespace) {
		return nil, ErrNamespace
	}
	if s.KeyPath == "" || strings.HasPrefix(s.KeyPath, "-") {
		return nil, errors.New("quorum: no signing key path")
	}
	bin, err := sshKeygen(s.SSHKeygen)
	if err != nil {
		return nil, err
	}
	cmd := exec.CommandContext(ctx, bin, "-Y", "sign", "-n", namespace, "-f", s.KeyPath)
	cmd.Stdin = bytes.NewReader(data)
	var out, errb bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errb
	if err := cmd.Run(); err != nil {
		if toolUnsupported(errb.Bytes()) {
			return nil, fmt.Errorf("%w (sign: %v)", ErrNoSSHKeygen, err)
		}
		return nil, fmt.Errorf("quorum: ssh-keygen sign: %v: %s", err, strings.TrimSpace(errb.String()))
	}
	if out.Len() == 0 || out.Len() > maxSigBytes {
		return nil, errors.New("quorum: ssh-keygen sign produced no usable signature")
	}
	return out.Bytes(), nil
}

// Verifier checks signatures against a Keeper registry in allowed_signers
// format. Build it with NewVerifier; it holds only registry lines that passed
// the Quorum rules.
type Verifier struct {
	registry  []byte
	sshKeygen string
	// Rejected lists the 1-based registry line numbers that were dropped.
	Rejected []int
}

// NewVerifier parses registry (the file read from the round's base commit).
// Only a line that names one keeper_id and carries namespaces= limited to the
// three Quorum namespaces is kept; every other line is dropped and listed in
// Rejected, so a line without namespaces= never signs a Quorum record. One
// keeper_id may have several lines (key rotation); a signature by any of that
// Keeper's listed keys verifies. The binary is looked up at verify time;
// sshKeygen "" means ssh-keygen on PATH.
func NewVerifier(registry []byte, sshKeygen string) (*Verifier, error) {
	if len(registry) > MaxRegistryBytes {
		return nil, fmt.Errorf("%w: over %d bytes", ErrRegistry, MaxRegistryBytes)
	}
	v := &Verifier{sshKeygen: sshKeygen}
	var kept bytes.Buffer
	for i, line := range strings.Split(string(registry), "\n") {
		line = strings.TrimRight(line, "\r")
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if !registryLineOK(t) {
			v.Rejected = append(v.Rejected, i+1)
			continue
		}
		kept.WriteString(t + "\n")
	}
	if kept.Len() == 0 {
		return nil, fmt.Errorf("%w: no valid line", ErrRegistry)
	}
	v.registry = kept.Bytes()
	return v, nil
}

// registryLineOK applies the Quorum rules to one allowed_signers line:
// `<keeper_id> namespaces="a,b" <keytype> <key> [comment]`. The principal is
// one plain keeper_id (no wildcard, no list, no negation), the only option is
// namespaces=, and every namespace is one of the three Quorum namespaces.
func registryLineOK(line string) bool {
	fields := splitQuoted(line)
	if len(fields) < 3 {
		return false
	}
	if !keeperIDPattern.MatchString(fields[0]) {
		return false
	}
	opts, rest := fields[1], fields[2:]
	val, ok := strings.CutPrefix(opts, `namespaces="`)
	if !ok || !strings.HasSuffix(val, `"`) {
		return false
	}
	val = strings.TrimSuffix(val, `"`)
	if val == "" || strings.Contains(val, `"`) {
		return false
	}
	for _, ns := range strings.Split(val, ",") {
		if !validNamespace(ns) {
			return false
		}
	}
	// A plain (non-certificate) key type and a key must follow the options.
	return len(rest) >= 2 && keyTypeOK(rest[0])
}

// keyTypeOK allows the plain public key types ssh-keygen -Y signs with. A
// certificate type (a -cert-v01@openssh.com name) or anything else is refused.
func keyTypeOK(kt string) bool {
	switch kt {
	case "ssh-ed25519", "ssh-rsa", "sk-ssh-ed25519@openssh.com", "sk-ecdsa-sha2-nistp256@openssh.com",
		"ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521":
		return true
	}
	return false
}

// toolUnsupported reports whether ssh-keygen's stderr shows it could not run
// the requested -Y form at all: usage text, an unknown or illegal option, or
// an unsupported -Y operation. A failed verification prints none of these.
func toolUnsupported(stderr []byte) bool {
	e := strings.ToLower(string(stderr))
	for _, m := range []string{"usage: ssh-keygen", "unknown option", "illegal option", "invalid option", "unsupported operation"} {
		if strings.Contains(e, m) {
			return true
		}
	}
	return false
}

// splitQuoted splits on spaces and tabs, keeping a double-quoted run whole.
func splitQuoted(s string) []string {
	var out []string
	var cur strings.Builder
	inQ, has := false, false
	for _, r := range s {
		switch {
		case r == '"':
			inQ = !inQ
			cur.WriteRune(r)
			has = true
		case (r == ' ' || r == '\t') && !inQ:
			if has {
				out = append(out, cur.String())
				cur.Reset()
				has = false
			}
		default:
			cur.WriteRune(r)
			has = true
		}
	}
	if has {
		out = append(out, cur.String())
	}
	return out
}

// Verify checks that sig is a valid signature over data in namespace by a key
// registered for keeperID. It returns ErrBadSignature when the check fails,
// ErrNoSSHKeygen when ssh-keygen is missing or lacks -Y verify, and another
// error when it could not run. A tool failure is never reported as a bad
// signature, so two Keepers with different OpenSSH builds cannot silently
// count different ballots.
func (v *Verifier) Verify(ctx context.Context, namespace, keeperID string, data, sig []byte) error {
	if !validNamespace(namespace) {
		return ErrNamespace
	}
	if !keeperIDPattern.MatchString(keeperID) {
		return ErrBadSignature
	}
	if len(sig) == 0 || len(sig) > maxSigBytes {
		return ErrBadSignature
	}
	bin, err := sshKeygen(v.sshKeygen)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp("", "waggle-quorum-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	regPath := filepath.Join(dir, "allowed_signers")
	sigPath := filepath.Join(dir, "record.sig")
	if err := os.WriteFile(regPath, v.registry, 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(sigPath, sig, 0o600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "-Y", "verify", "-f", regPath, "-I", keeperID, "-n", namespace, "-s", sigPath)
	cmd.Stdin = bytes.NewReader(data)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ctx.Err() == nil {
			if toolUnsupported(errb.Bytes()) {
				return fmt.Errorf("%w (verify: %v)", ErrNoSSHKeygen, err)
			}
			return ErrBadSignature
		}
		return fmt.Errorf("quorum: ssh-keygen verify: %v", err)
	}
	return nil
}
