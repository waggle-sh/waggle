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
	// ErrNoSSHKeygen reports that the ssh-keygen binary is missing.
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
// Rejected, so a line without namespaces= never signs a Quorum record. The
// binary is looked up at verify time; sshKeygen "" means ssh-keygen on PATH.
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
	// A key type and a key must follow the options.
	kt := rest[0]
	if !(strings.HasPrefix(kt, "ssh-") || strings.HasPrefix(kt, "ecdsa-") || strings.HasPrefix(kt, "sk-")) || len(rest) < 2 {
		return false
	}
	return true
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
// and a different error when it could not run.
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
			return ErrBadSignature
		}
		return fmt.Errorf("quorum: ssh-keygen verify: %v", err)
	}
	return nil
}
