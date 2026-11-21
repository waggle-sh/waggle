// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const hostilePhrase = "usage: ssh-keygen unknown option unsupported operation illegal option invalid option"

func armor(body []byte) []byte {
	return []byte("-----BEGIN SSH SIGNATURE-----\n" + base64.StdEncoding.EncodeToString(body) + "\n-----END SSH SIGNATURE-----\n")
}

// sshsigFields splits a decoded SSHSIG blob into its magic, version and five
// length-prefixed strings (publickey, namespace, reserved, hash, signature).
func sshsigFields(t *testing.T, sig []byte) (magic, version []byte, fields [][]byte) {
	t.Helper()
	s := string(sig)
	s = strings.TrimPrefix(s, "-----BEGIN SSH SIGNATURE-----")
	s = strings.TrimSpace(strings.Split(s, "-----END SSH SIGNATURE-----")[0])
	blob, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(s, "\n", ""))
	if err != nil || len(blob) < 10 || string(blob[:6]) != "SSHSIG" {
		t.Fatalf("not an SSHSIG blob: %v", err)
	}
	magic, version, rest := blob[:6], blob[6:10], blob[10:]
	for len(rest) >= 4 {
		n := binary.BigEndian.Uint32(rest)
		if int(n) > len(rest)-4 {
			t.Fatal("bad SSHSIG field length")
		}
		fields = append(fields, rest[4:4+n])
		rest = rest[4+n:]
	}
	if len(fields) != 5 {
		t.Fatalf("SSHSIG has %d fields", len(fields))
	}
	return magic, version, fields
}

func sshsigBlob(magic, version []byte, fields [][]byte) []byte {
	var b bytes.Buffer
	b.Write(magic)
	b.Write(version)
	for _, f := range fields {
		var n [4]byte
		binary.BigEndian.PutUint32(n[:], uint32(len(f)))
		b.Write(n[:])
		b.Write(f)
	}
	return b.Bytes()
}

// With the real ssh-keygen, a crafted signature is a bad signature, never a
// tool error, whatever ssh-keygen prints about it.
func TestRealSSHCraftedSignaturesAreBad(t *testing.T) {
	if _, err := exec.LookPath("ssh-keygen"); err != nil {
		t.Skip("ssh-keygen not on PATH")
	}
	ctx := context.Background()
	kd := t.TempDir()
	alicePriv, alicePub := genKey(t, kd, "alice")
	signer := Signer{KeyPath: alicePriv}
	data := []byte("ballot bytes\n")
	v, err := NewVerifier([]byte(realLine("alice", alicePub, NamespaceBallot)), "")
	if err != nil {
		t.Fatal(err)
	}
	sig, err := signer.Sign(ctx, NamespaceBallot, data)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Verify(ctx, NamespaceBallot, "alice", data, sig); err != nil {
		t.Fatalf("valid signature: %v", err)
	}
	roundSig, err := signer.Sign(ctx, NamespaceRound, data)
	if err != nil {
		t.Fatal(err)
	}
	magic, version, fields := sshsigFields(t, sig)
	oddHash := func(h string) []byte {
		f := append([][]byte(nil), fields...)
		f[3] = []byte(h)
		return armor(sshsigBlob(magic, version, f))
	}
	oddNS := append([][]byte(nil), fields...)
	oddNS[1] = []byte(hostilePhrase)

	cases := map[string][]byte{
		"truncated armor":       sig[:len(sig)/2],
		"garbage body":          []byte("-----BEGIN SSH SIGNATURE-----\n!!!!not base64!!!!\n-----END SSH SIGNATURE-----\n"),
		"phrase in armor":       []byte("-----BEGIN SSH SIGNATURE-----\n" + hostilePhrase + "\n-----END SSH SIGNATURE-----\n"),
		"phrase base64 armored": armor([]byte(hostilePhrase)),
		"phrase bare":           []byte(hostilePhrase + "\n"),
		"wrong namespace":       roundSig,
		"phrase namespace":      armor(sshsigBlob(magic, version, oddNS)),
		"hash md5":              oddHash("md5"),
		"hash phrase":           oddHash(hostilePhrase),
		"bad magic":             armor(append([]byte("XXXXXX"), sshsigBlob(magic, version, fields)[6:]...)),
	}
	for name, s := range cases {
		err := v.Verify(ctx, NamespaceBallot, "alice", data, s)
		if !errors.Is(err, ErrBadSignature) || errors.Is(err, ErrNoSSHKeygen) {
			t.Errorf("%s: err = %v, want ErrBadSignature", name, err)
		}
	}
}

// writeFake writes a shell ssh-keygen stand-in.
func writeFake(t *testing.T, dir, name, body string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte("#!/bin/sh\ncat >/dev/null\n"+body), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// probeCase answers the fixed probe as a working ssh-keygen does; the probe
// is the only call that passes -I probe.
const probeCase = `case "$*" in *"-I probe "*) echo "probe" >> "$(dirname "$0")/probes"; echo "Couldn't verify signature: invalid format" >&2; exit 255;; esac
`

// After a good probe, per-ballot stderr never turns a failure into a tool
// error, and the probe runs once per Verifier.
func TestProbeDecidesToolError(t *testing.T) {
	dir := t.TempDir()
	fake := writeFake(t, dir, "keygen-hostile", probeCase+"echo '"+hostilePhrase+"' >&2\nexit 1\n")
	v, err := NewVerifier([]byte(regLine("alice", "k", NamespaceBallot)), fake)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		err := v.Verify(context.Background(), NamespaceBallot, "alice", []byte("x"), []byte(hostilePhrase))
		if !errors.Is(err, ErrBadSignature) || errors.Is(err, ErrNoSSHKeygen) {
			t.Fatalf("verify %d: err = %v, want ErrBadSignature", i, err)
		}
	}
	b, err := os.ReadFile(filepath.Join(dir, "probes"))
	if err != nil || strings.Count(string(b), "probe") != 1 {
		t.Fatalf("probes = %q, %v; want one", b, err)
	}
	// A failing sign after a good probe is a plain error.
	key := filepath.Join(dir, "key")
	if err := os.WriteFile(key, []byte("k\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Signer{KeyPath: key, SSHKeygen: fake}.Sign(context.Background(), NamespaceBallot, []byte("x"))
	if err == nil || errors.Is(err, ErrNoSSHKeygen) {
		t.Fatalf("sign: err = %v, want a plain error", err)
	}
}

// A signal-killed ssh-keygen is a plain error, not a bad signature and not a
// tool error.
func TestKeygenSignalIsPlainError(t *testing.T) {
	dir := t.TempDir()
	fake := writeFake(t, dir, "keygen-kill", probeCase+"kill -9 $$\n")
	v, err := NewVerifier([]byte(regLine("alice", "k", NamespaceBallot)), fake)
	if err != nil {
		t.Fatal(err)
	}
	err = v.Verify(context.Background(), NamespaceBallot, "alice", []byte("x"), []byte("sig"))
	if err == nil || errors.Is(err, ErrBadSignature) || errors.Is(err, ErrNoSSHKeygen) {
		t.Fatalf("verify: err = %v, want a plain error", err)
	}
	key := filepath.Join(dir, "key")
	if err := os.WriteFile(key, []byte("k\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err = Signer{KeyPath: key, SSHKeygen: fake}.Sign(context.Background(), NamespaceBallot, []byte("x"))
	if err == nil || errors.Is(err, ErrNoSSHKeygen) {
		t.Fatalf("sign: err = %v, want a plain error", err)
	}
	// A probe that is itself killed is a plain error and is not cached.
	killAll := writeFake(t, dir, "keygen-killall", "kill -9 $$\n")
	v2, err := NewVerifier([]byte(regLine("alice", "k", NamespaceBallot)), killAll)
	if err != nil {
		t.Fatal(err)
	}
	err = v2.Verify(context.Background(), NamespaceBallot, "alice", []byte("x"), []byte("sig"))
	if err == nil || errors.Is(err, ErrBadSignature) || errors.Is(err, ErrNoSSHKeygen) {
		t.Fatalf("killed probe: err = %v, want a plain error", err)
	}
	if v2.probed {
		t.Fatal("a killed probe was cached")
	}
}

// A cancelled context during the probe is a plain error and is not cached.
func TestProbeCancelledNotCached(t *testing.T) {
	v, err := NewVerifier([]byte(regLine("alice", "k", NamespaceBallot)), fakeKeygenBin(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = v.Verify(ctx, NamespaceBallot, "alice", []byte("x"), []byte("sig"))
	if err == nil || errors.Is(err, ErrBadSignature) || errors.Is(err, ErrNoSSHKeygen) {
		t.Fatalf("cancelled: err = %v, want a plain error", err)
	}
	if v.probed {
		t.Fatal("a cancelled probe was cached")
	}
	if err := v.Verify(context.Background(), NamespaceBallot, "alice", []byte("x"), []byte("sig")); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("after cancel: err = %v, want ErrBadSignature", err)
	}
}
