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
	"sync"
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
		// Only the fixed probe decides ErrNoSSHKeygen; the sign stderr is
		// never scanned. The probe runs on failure only, so a working Sign
		// costs one ssh-keygen run.
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() != -1 && ctx.Err() == nil {
			if perr := probeKeygen(ctx, bin); errors.Is(perr, ErrNoSSHKeygen) {
				return nil, fmt.Errorf("%w (sign: %v)", ErrNoSSHKeygen, err)
			}
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

	// probeMu guards the cached result of the ssh-keygen probe (keygen).
	probeMu  sync.Mutex
	probed   bool
	probeBin string
	probeErr error
	// Rejected lists the 1-based registry line numbers that were dropped.
	Rejected []int
}

// NewVerifier parses registry (the file read from the round's base commit).
// Only a line that names one keeper_id and carries namespaces= limited to the
// three Quorum namespaces is kept; every other line is dropped and listed in
// Rejected, so a line without namespaces= never signs a Quorum record. One
// keeper_id may have several lines (key rotation); a signature by any of that
// Keeper's listed keys verifies. The binary is looked up and probed on the
// first Verify; sshKeygen "" means ssh-keygen on PATH.
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

// probeRegistry and probeSig are the fixed probe inputs: a well-formed
// registry line and an armored signature whose body is not an SSHSIG. A
// working ssh-keygen -Y verify rejects the signature with exit 255.
const (
	probeRegistry = `probe namespaces="waggle-quorum-ballot" ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIKTIPVNqqpW6QWB2ACIYw5UgiK1bIngoGnoZUOfscAwj` + "\n"
	probeSig      = "-----BEGIN SSH SIGNATURE-----\nAAAA\n-----END SSH SIGNATURE-----\n"
)

// probeUnsupported lists the stderr phrases that mean ssh-keygen cannot run
// -Y verify at all. Checked against OpenSSH 10.3p1 (`Unsupported operation
// for -Y: "bogus"` then `usage: ssh-keygen ...`; `option requires an
// argument` then usage). The getopt forms an ssh-keygen without -Y would
// print (`illegal option -- Y` from BSD libc, `invalid option -- 'Y'` from
// glibc) come from the libc docs, not a run of an old OpenSSH. A working ssh-keygen answers the probe with `Couldn't
// verify signature: invalid format`, which matches none of them. Only the
// probe's own output, from fixed input, is ever matched.
var probeUnsupported = []string{"usage: ssh-keygen", "unknown option", "illegal option", "invalid option", "unsupported operation"}

// probeKeygen runs ssh-keygen -Y verify once with fixed arguments and fixed
// input. It returns nil when the tool handles -Y verify, ErrNoSSHKeygen when
// it is missing or lacks -Y, and another error when it could not run. The
// result never depends on a stored signature.
func probeKeygen(ctx context.Context, bin string) error {
	dir, err := os.MkdirTemp("", "waggle-quorum-probe-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	regPath := filepath.Join(dir, "allowed_signers")
	sigPath := filepath.Join(dir, "probe.sig")
	if err := os.WriteFile(regPath, []byte(probeRegistry), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(sigPath, []byte(probeSig), 0o600); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, bin, "-Y", "verify", "-f", regPath, "-I", "probe", "-n", NamespaceBallot, "-s", sigPath)
	cmd.Stdin = strings.NewReader("waggle quorum probe\n")
	var errb bytes.Buffer
	cmd.Stderr = &errb
	err = cmd.Run()
	if err == nil {
		return nil
	}
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() == -1 || ctx.Err() != nil {
		return fmt.Errorf("quorum: ssh-keygen probe: %v", err)
	}
	e := strings.ToLower(errb.String())
	for _, m := range probeUnsupported {
		if strings.Contains(e, m) {
			return fmt.Errorf("%w (probe: %v)", ErrNoSSHKeygen, err)
		}
	}
	return nil
}

// keygen resolves and probes the binary once per Verifier. A definite result
// (usable, or ErrNoSSHKeygen) is cached; a probe that could not run (exec
// failure, a signal, a cancelled context) is returned and retried next time.
func (v *Verifier) keygen(ctx context.Context) (string, error) {
	v.probeMu.Lock()
	defer v.probeMu.Unlock()
	if v.probed {
		return v.probeBin, v.probeErr
	}
	bin, err := sshKeygen(v.sshKeygen)
	if err == nil {
		err = probeKeygen(ctx, bin)
	}
	if err != nil && !errors.Is(err, ErrNoSSHKeygen) {
		return "", err
	}
	v.probed, v.probeBin, v.probeErr = true, bin, err
	return bin, err
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
// registered for keeperID. It returns ErrNoSSHKeygen when the one probe of
// ssh-keygen (probeKeygen) shows it missing or without -Y verify, and another
// error when ssh-keygen could not run, was killed by a signal or the context
// ended. After a good probe every other verify failure is ErrBadSignature,
// whatever ssh-keygen printed, so a stored signature cannot steer its own
// classification. A tool failure is never reported as a bad signature, so two
// Keepers with different OpenSSH builds cannot silently count different
// ballots.
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
	bin, err := v.keygen(ctx)
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
	// Stderr is discarded: what ssh-keygen prints about a stored signature
	// never affects the result.
	cmd := exec.CommandContext(ctx, bin, "-Y", "verify", "-f", regPath, "-I", keeperID, "-n", namespace, "-s", sigPath)
	cmd.Stdin = bytes.NewReader(data)
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() != -1 && ctx.Err() == nil {
			return ErrBadSignature
		}
		return fmt.Errorf("quorum: ssh-keygen verify: %v", err)
	}
	return nil
}
