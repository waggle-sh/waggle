// SPDX-License-Identifier: Apache-2.0

package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

const installTestVersion = "0.0.1"

// installScriptPath returns the repo's install.sh, skipping the test when a
// tool the script needs is missing.
func installScriptPath(t *testing.T) string {
	t.Helper()
	for _, tool := range []string{"sh", "curl", "tar"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not on PATH", tool)
		}
	}
	if _, err := exec.LookPath("sha256sum"); err != nil {
		if _, err := exec.LookPath("shasum"); err != nil {
			t.Skip("neither sha256sum nor shasum on PATH")
		}
	}
	p, err := filepath.Abs(filepath.Join("..", "..", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(p); err != nil {
		t.Fatal(err)
	}
	return p
}

// installPlatform maps the host to the names install.sh uses, or skips.
func installPlatform(t *testing.T) (string, string) {
	t.Helper()
	var os_, arch string
	switch runtime.GOOS {
	case "linux", "darwin":
		os_ = runtime.GOOS
	default:
		t.Skipf("install.sh does not handle %s", runtime.GOOS)
	}
	switch runtime.GOARCH {
	case "amd64", "arm64":
		arch = runtime.GOARCH
	default:
		t.Skipf("install.sh does not handle %s", runtime.GOARCH)
	}
	return os_, arch
}

// fakeRelease writes a release directory holding one archive and a SHA256SUMS
// for it, and returns the file:// base URL and the archive path.
func fakeRelease(t *testing.T) (string, string) {
	t.Helper()
	osName, arch := installPlatform(t)
	dir := t.TempDir()
	name := fmt.Sprintf("waggle_%s_%s_%s", installTestVersion, osName, arch)

	var tarBuf bytes.Buffer
	tw := tar.NewWriter(&tarBuf)
	body := []byte("#!/bin/sh\necho waggle " + installTestVersion + "\n")
	if err := tw.WriteHeader(&tar.Header{Name: name + "/waggle", Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	if _, err := zw.Write(tarBuf.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	archive := filepath.Join(dir, name+".tar.gz")
	if err := os.WriteFile(archive, gz.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(gz.Bytes())
	sums := hex.EncodeToString(sum[:]) + "  " + name + ".tar.gz\n"
	if err := os.WriteFile(filepath.Join(dir, "SHA256SUMS"), []byte(sums), 0o644); err != nil {
		t.Fatal(err)
	}
	return "file://" + dir, archive
}

// runInstall runs install.sh offline and returns its combined output.
func runInstall(t *testing.T, base, installDir string) (string, error) {
	t.Helper()
	cmd := exec.Command("sh", installScriptPath(t))
	cmd.Env = append(os.Environ(),
		"HOME="+t.TempDir(),
		"WAGGLE_VERSION="+installTestVersion,
		"WAGGLE_BASE_URL="+base,
		"WAGGLE_INSTALL_DIR="+installDir,
	)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInstallScriptHappyPath(t *testing.T) {
	base, _ := fakeRelease(t)
	dest := filepath.Join(t.TempDir(), "bin")
	out, err := runInstall(t, base, dest)
	if err != nil {
		t.Fatalf("install failed: %v\n%s", err, out)
	}
	if strings.Contains(out, "error") {
		t.Errorf("output mentions an error:\n%s", out)
	}
	fi, err := os.Stat(filepath.Join(dest, "waggle"))
	if err != nil {
		t.Fatalf("binary not installed: %v\n%s", err, out)
	}
	if fi.Mode()&0o111 == 0 {
		t.Errorf("binary is not executable: %v", fi.Mode())
	}
}

func TestInstallScriptChecksumMismatch(t *testing.T) {
	base, archive := fakeRelease(t)
	data, err := os.ReadFile(archive)
	if err != nil {
		t.Fatal(err)
	}
	data = append(data, 'x')
	if err := os.WriteFile(archive, data, 0o644); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "bin")
	out, err := runInstall(t, base, dest)
	if err == nil {
		t.Fatalf("install succeeded on a tampered archive:\n%s", out)
	}
	if !strings.Contains(out, "checksum") {
		t.Errorf("message does not mention checksum:\n%s", out)
	}
	if _, err := os.Stat(filepath.Join(dest, "waggle")); err == nil {
		t.Error("binary was installed despite checksum mismatch")
	}
}

func TestInstallScriptMissingArchive(t *testing.T) {
	base, archive := fakeRelease(t)
	if err := os.Remove(archive); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(t.TempDir(), "bin")
	out, err := runInstall(t, base, dest)
	if err == nil {
		t.Fatalf("install succeeded without an archive:\n%s", out)
	}
	if !strings.Contains(out, filepath.Base(archive)) {
		t.Errorf("message does not name %s:\n%s", filepath.Base(archive), out)
	}
	if _, err := os.Stat(filepath.Join(dest, "waggle")); err == nil {
		t.Error("binary was installed without an archive")
	}
}

func TestInstallScriptUnwritableDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can write to any directory")
	}
	if _, err := exec.LookPath("sudo"); err == nil {
		t.Skip("install.sh would fall back to sudo")
	}
	base, _ := fakeRelease(t)
	dest := filepath.Join(t.TempDir(), "ro")
	if err := os.Mkdir(dest, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dest, 0o700) })
	out, err := runInstall(t, base, dest)
	if err == nil {
		t.Fatalf("install succeeded into an unwritable dir:\n%s", out)
	}
	if !strings.Contains(out, dest) {
		t.Errorf("message does not name %s:\n%s", dest, out)
	}
}

func TestInstallScriptShellcheck(t *testing.T) {
	if _, err := exec.LookPath("shellcheck"); err != nil {
		t.Skip("shellcheck not on PATH")
	}
	out, err := exec.Command("shellcheck", "-s", "sh", installScriptPath(t)).CombinedOutput()
	if err != nil {
		t.Errorf("shellcheck findings:\n%s", out)
	}
}
