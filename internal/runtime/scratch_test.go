// SPDX-License-Identifier: Apache-2.0

package runtime

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// scratchEnv is the env Prepare builds for a builder whose scratch dir is dir.
func scratchEnv(t *testing.T, dir string) ([]string, error) {
	t.Helper()
	inv, s := testLaunch(t, "T-5")
	inv.ScratchDir = dir
	spec, err := verifiedAdapter(t, AdapterConfig{}).Prepare(inv, s, nil, nil)
	return spec.Env, err
}

func gitIn(dir string, env []string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir, cmd.Env = dir, env
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// AC3: pushes into the scratch dir go through; pushes anywhere else, a prefix
// sibling and the top-level .. spellings are still rewritten to file:///dev/null/.
func TestTripwireScratchExemption(t *testing.T) {
	scratch := t.TempDir()
	env, err := scratchEnv(t, scratch)
	if err != nil {
		t.Fatal(err)
	}
	env = append(env, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1")
	setup := append(slices.Clone(env), "GIT_CONFIG_COUNT=0")
	work := filepath.Join(t.TempDir(), "work")
	sibling := scratch + "-x"
	if err := os.MkdirAll(sibling, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(sibling) })
	outside := t.TempDir()
	bare := func(dir string) string {
		path := filepath.Join(dir, "o.git")
		if out, err := gitIn(dir, setup, "init", "-q", "--bare", path); err != nil {
			t.Fatalf("git init: %v\n%s", err, out)
		}
		return path
	}
	if out, err := gitIn(t.TempDir(), setup, "init", "-q", work); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	writeFile(t, filepath.Join(work, "f.txt"), "x\n")
	for _, c := range [][]string{{"add", "f.txt"}, {"-c", "user.name=t", "-c", "user.email=t@example.com", "commit", "-q", "-m", "m"}} {
		if out, err := gitIn(work, setup, c...); err != nil {
			t.Fatalf("git %v: %v\n%s", c, err, out)
		}
	}
	refs := func(repo string) string {
		out, _ := gitIn(repo, setup, "for-each-ref")
		return out
	}
	push := func(url string) (string, error) { return gitIn(work, env, "push", url, "HEAD:refs/heads/pushed") }

	in := bare(scratch)
	urls := []string{in, "file://" + in}
	if r, err := filepath.EvalSymlinks(in); err == nil && r != in {
		urls = append(urls, r, "file://"+r)
	}
	for _, url := range urls {
		if out, err := push(url); err != nil {
			t.Errorf("push to %s: %v\n%s", url, err, out)
		}
		if refs(in) == "" {
			t.Errorf("push to %s moved no ref", url)
		}
		if out, err := gitIn(in, setup, "update-ref", "-d", "refs/heads/pushed"); err != nil {
			t.Fatalf("update-ref: %v\n%s", err, out)
		}
	}

	out, sib := bare(outside), bare(sibling)
	blocked := map[string]string{"outside": out, "outside file": "file://" + out, "prefix sibling": sib, "prefix sibling file": "file://" + sib,
		"dotdot": scratch + "/../" + filepath.Base(sibling) + "/o.git", "dotdot file": "file://" + scratch + "/../" + filepath.Base(sibling) + "/o.git",
		"http": "http://127.0.0.1:1/x.git", "https": "https://127.0.0.1:1/x.git", "ssh": "ssh://127.0.0.1:1/x.git",
		"scp": "git@127.0.0.1:x.git", "git": "git://127.0.0.1:1/x.git", "relative": "../o.git"}
	for name, url := range blocked {
		o, err := push(url)
		if err == nil || !strings.Contains(o, "/dev/null/") {
			t.Errorf("%s push to %s: err = %v, output %q; want a failure at the file:///dev/null/ target", name, url, err, o)
		}
	}
	if r := refs(out) + refs(sib); r != "" {
		t.Errorf("a blocked push moved a ref: %s", r)
	}
}

// AC4: without a scratch dir the tripwire entries are today's, byte for byte.
func TestTripwireEnvNoScratch(t *testing.T) {
	want := []string{"GIT_CONFIG_COUNT=14"}
	keys := []string{"https://", "ssh://", "git@", "http://", "git://", "git+ssh://", "ssh+git://", "file://", "/", "."}
	for i, k := range keys {
		n := strconv.Itoa(i)
		want = append(want, "GIT_CONFIG_KEY_"+n+"=url.file:///dev/null/.pushInsteadOf", "GIT_CONFIG_VALUE_"+n+"="+k)
	}
	want = append(want, "GIT_CONFIG_KEY_10=core.excludesFile", "GIT_CONFIG_VALUE_10=/dev/null",
		"GIT_CONFIG_KEY_11=core.attributesFile", "GIT_CONFIG_VALUE_11=/dev/null",
		"GIT_CONFIG_KEY_12=maintenance.auto", "GIT_CONFIG_VALUE_12=false", "GIT_CONFIG_KEY_13=gc.auto", "GIT_CONFIG_VALUE_13=0")
	if got := tripwireEnv(nil); !slices.Equal(got, want) {
		t.Errorf("tripwireEnv(nil) =\n%q\nwant\n%q", got, want)
	}
}

// AC5: one TMPDIR equal to the scratch dir, one GIT_CONFIG_COUNT matching the pairs.
func TestScratchEnvShape(t *testing.T) {
	scratch := t.TempDir()
	t.Setenv("TMPDIR", "/parent/tmp")
	env, err := scratchEnv(t, scratch)
	if err != nil {
		t.Fatal(err)
	}
	var tmp, counts, keys, values []string
	for _, kv := range env {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case k == "TMPDIR":
			tmp = append(tmp, v)
		case k == "GIT_CONFIG_COUNT":
			counts = append(counts, v)
		case strings.HasPrefix(k, "GIT_CONFIG_KEY_"):
			keys = append(keys, k)
		case strings.HasPrefix(k, "GIT_CONFIG_VALUE_"):
			values = append(values, k)
		}
	}
	if !slices.Equal(tmp, []string{scratch}) {
		t.Errorf("TMPDIR entries = %q; want only %q", tmp, scratch)
	}
	if len(counts) != 1 || counts[0] != strconv.Itoa(len(keys)) || len(keys) != len(values) || len(keys) <= 14 {
		t.Errorf("GIT_CONFIG_COUNT = %q for %d keys and %d values", counts, len(keys), len(values))
	}
	for i := range keys {
		if keys[i] != "GIT_CONFIG_KEY_"+strconv.Itoa(i) {
			t.Errorf("key %d is %s; indexes must be contiguous", i, keys[i])
		}
	}
}

// AC6: a relative, unclean or control-character scratch path is refused.
func TestScratchPathRefused(t *testing.T) {
	for _, bad := range []string{"rel/dir", ".", "/tmp/a/../b", "/tmp//a", "/tmp/a/", "/tmp/a\nb", "/tmp/a\x00b", "/tmp/a\tb", "/"} {
		_, err := scratchEnv(t, bad)
		wantErr(t, err, ErrAdapterConfig, "scratch directory")
	}
}
