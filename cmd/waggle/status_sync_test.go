// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/scaffold"
)

// T-081: a plain waggle status fetches claim refs exactly once.
func TestStatusOneClaimSync(t *testing.T) {
	requireGit(t)
	fixClock(t)
	dir := filepath.Join(t.TempDir(), "onesync")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "init", "-q")
	err := scaffold.InitRepo(scaffold.InitOptions{
		TargetDir: dir, Profile: "cli", Delivery: "installed", Forge: "github", CombMode: "direct",
		Runtime: "none", GateOnly: true, Out: &bytes.Buffer{},
		BuildInfo: func() (*debug.BuildInfo, bool) {
			return &debug.BuildInfo{Settings: []debug.BuildSetting{
				{Key: "vcs.revision", Value: strings.Repeat("a", 40)}, {Key: "vcs.modified", Value: "false"},
			}}, true
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	bare := filepath.Join(t.TempDir(), "origin.git")
	git(t, filepath.Dir(bare), "init", "-q", "--bare", bare)
	git(t, dir, "remote", "add", "origin", bare)

	// Every fetch from origin runs this upload-pack wrapper, which logs a line.
	logFile := filepath.Join(t.TempDir(), "fetches.log")
	wrapper := filepath.Join(t.TempDir(), "upload-pack.sh")
	script := "#!/bin/sh\necho x >> '" + logFile + "'\nexec git-upload-pack \"$@\"\n"
	if err := os.WriteFile(wrapper, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, dir, "config", "remote.origin.uploadpack", wrapper)

	if code, _, errb := runStatusT(t, "--repo", dir); code != 0 {
		t.Fatalf("status code %d, stderr %q", code, errb)
	}
	b, _ := os.ReadFile(logFile)
	if n := strings.Count(string(b), "x\n"); n != 1 {
		t.Errorf("plain status fetched %d times; want 1", n)
	}
}
