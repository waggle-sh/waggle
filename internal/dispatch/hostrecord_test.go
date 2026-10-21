//go:build unix

package dispatch

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func hostRec(f *fixture, accept string, accepting bool) (string, error) {
	var out bytes.Buffer
	err := HostRecord(context.Background(), HostRecordRequest{RepoDir: f.repo, Accept: accept, Accepting: accepting}, &out)
	return out.String(), err
}

// recordNow writes host-files.json as the current state, as a clean run does.
func recordNow(t *testing.T, f *fixture) (path string) {
	t.Helper()
	dir, err := worktreesDir(f.real)
	if err != nil {
		t.Fatal(err)
	}
	pg := (&Git{Dir: f.real}).pinned()
	b, _ := json.Marshal(hostRecord{Run: "r1", Files: recordFiles(hostFiles(f.common(), ""), configKeys(context.Background(), pg, f.common()))})
	path = filepath.Join(dir, hostRecordName)
	if err := os.WriteFile(path, b, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

var digestLine = regexp.MustCompile(`(?m)^digest ([0-9a-f]{16})$`)

func listedDigest(t *testing.T, out string) string {
	t.Helper()
	m := digestLine.FindStringSubmatch(out)
	if m == nil || !strings.Contains(out, "waggle dispatch clean --host-record --accept "+m[1]) {
		t.Fatalf("no digest in %q", out)
	}
	return m[1]
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// D2b-T1: the digest is stable, 16 lowercase hex, and moves with any key, any
// value and the record's file state; printable strips control and format runes.
func TestHostDigestAndPrintable(t *testing.T) {
	a := map[string]string{"x": "1", "y": "2", "z": "3"}
	b := map[string]string{"z": "3", "y": "2", "x": "1"}
	d := hostDigest("s", a)
	if !digestRe.MatchString(d) || d != hostDigest("s", b) {
		t.Fatalf("digest %q unstable or malformed", d)
	}
	for name, got := range map[string]string{
		"state": hostDigest("t", a), "value": hostDigest("s", map[string]string{"x": "1", "y": "2", "z": "4"}),
		"key": hostDigest("s", map[string]string{"x": "1", "y": "2", "w": "3"}), "extra": hostDigest("s", map[string]string{"x": "1", "y": "2", "z": "3", "w": ""}),
	} {
		if got == d {
			t.Errorf("digest ignores a change of the %s", name)
		}
	}
	if got := printable("a\x00b\x1bc\u0085d‮e​f\n"); got != "abcdef" {
		t.Errorf("printable = %q", got)
	}
	if got := printable(strings.Repeat("é", 80)); len(got) > 100 || !strings.HasPrefix(got, "éé") {
		t.Errorf("printable cap: %d bytes", len(got))
	}
}

// D2b-T2: the listing names both values, no host path, writes nothing, and
// strips a planted file name.
func TestHostRecordListing(t *testing.T) {
	f := newFixture(t, false, false)
	if out, err := hostRec(f, "", false); err != nil || !strings.Contains(out, "no host record; the next run writes one") {
		t.Fatalf("absent: %q %v", out, err)
	}
	record := recordNow(t, f)
	if out, err := hostRec(f, "", false); err != nil || !strings.Contains(out, "host record matches") {
		t.Fatalf("matching: %q %v", out, err)
	}
	f.git("config", "remote.x.url", "y")
	write(t, f.common(), "hooks/a\x1bb‮c", "x")
	before, listing := read(t, record), dirNames(t, filepath.Dir(record))
	out, err := hostRec(f, "", false)
	if err != nil {
		t.Fatal(err)
	}
	listedDigest(t, out)
	if !strings.Contains(out, "config.keys: recorded ") || !strings.Contains(out, "hooks/abc: recorded missing current ") || strings.ContainsAny(out, "\x1b‮") {
		t.Fatalf("listing: %q", out)
	}
	for _, s := range []string{os.TempDir(), f.real, os.Getenv("HOME") + "/"} {
		if strings.Contains(out, s) {
			t.Fatalf("listing names %q: %s", s, out)
		}
	}
	if read(t, record) != before || dirNames(t, filepath.Dir(record)) != listing {
		t.Fatal("the listing wrote a file")
	}
}

func dirNames(t *testing.T, dir string) string {
	t.Helper()
	ents, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range ents {
		names = append(names, e.Name())
	}
	return strings.Join(names, ",")
}

// D2b-T2: unreadable records, a held lock and a non-repo.
func TestHostRecordUnreadableLockedAndNonRepo(t *testing.T) {
	f := newFixture(t, false, false)
	record := recordNow(t, f)
	for name, mk := range map[string]func(){
		"bad JSON": func() { os.WriteFile(record, []byte("{"), 0o600) },
		"oversize": func() { os.WriteFile(record, bytes.Repeat([]byte(" "), maxHash+1), 0o600) },
		"symlink": func() {
			os.Remove(record)
			os.WriteFile(record+".target", []byte("{}"), 0o600)
			os.Symlink(record+".target", record)
		},
	} {
		mk()
		out, err := hostRec(f, "", false)
		if err != nil || !strings.Contains(out, "host record unreadable: ") || !strings.Contains(out, "config.keys: recorded missing current ") || strings.Contains(out, f.real) {
			t.Errorf("%s: %q %v", name, out, err)
		}
		listedDigest(t, out)
	}
	writeLock(t, filepath.Join(filepath.Dir(record), "dispatch.lock"), os.Getpid())
	if _, err := hostRec(f, "", false); code(err) != ExitEnv {
		t.Errorf("held lock: %v", err)
	}
	dir := t.TempDir()
	if err := HostRecord(context.Background(), HostRecordRequest{RepoDir: dir}, &bytes.Buffer{}); code(err) != ExitEnv || exists(filepath.Join(dir, ".waggle")) {
		t.Errorf("non-repo: %v", err)
	}
}

// D2b-T3: --accept writes only for the digest of the state at write time.
func TestHostRecordAccept(t *testing.T) {
	f := newFixture(t, false, false)
	record := recordNow(t, f)
	f.git("config", "remote.x.url", "y")
	out, _ := hostRec(f, "", false)
	digest := listedDigest(t, out)
	before := read(t, record)

	f.git("config", "remote.x.url", "z") // a further edit after the listing
	if _, err := hostRec(f, digest, true); code(err) != ExitEnv || read(t, record) != before {
		t.Fatalf("stale config digest: %v", err)
	}
	f.git("config", "remote.x.url", "y")
	if err := os.WriteFile(record, []byte(`{"run":"r2","files":{}}`), 0o600); err != nil { // the record was rewritten
		t.Fatal(err)
	}
	if _, err := hostRec(f, digest, true); code(err) != ExitEnv || read(t, record) != `{"run":"r2","files":{}}` {
		t.Fatalf("rewritten record: %v", err)
	}
	if _, err := hostRec(f, "ABC", true); code(err) != ExitUsage {
		t.Fatalf("malformed digest: %v", err)
	}

	digest = listedDigest(t, mustOut(t, f))
	if out, err := hostRec(f, digest, true); err != nil || !strings.Contains(out, "host record accepted") {
		t.Fatalf("accept: %q %v", out, err)
	}
	var rec hostRecord
	if err := json.Unmarshal([]byte(read(t, record)), &rec); err != nil || rec.Run != "accepted" {
		t.Fatalf("record %q: %v", read(t, record), err)
	}
	pg := (&Git{Dir: f.real}).pinned()
	if err := checkHostRecord(context.Background(), pg, filepath.Dir(record), f.common()); err != nil {
		t.Fatalf("check after accept: %v", err)
	}

	// Over a symlinked record the link target stays as it was.
	target := record + ".target"
	os.WriteFile(target, []byte("keep"), 0o600)
	os.Remove(record)
	os.Symlink(target, record)
	digest = listedDigest(t, mustOut(t, f))
	if _, err := hostRec(f, digest, true); err != nil || read(t, target) != "keep" {
		t.Fatalf("symlink: %v target %q", err, read(t, target))
	}
	if fi, err := os.Lstat(record); err != nil || fi.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("record is still a link: %v", err)
	}
}

func mustOut(t *testing.T, f *fixture) string {
	t.Helper()
	out, err := hostRec(f, "", false)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// D2b-T3: the check names the command that records the state, not a delete.
func TestCheckHostRecordNamesTheCommand(t *testing.T) {
	f := newFixture(t, false, false)
	record := recordNow(t, f)
	pg := (&Git{Dir: f.real}).pinned()
	f.git("config", "remote.x.url", "y")
	err := checkHostRecord(context.Background(), pg, filepath.Dir(record), f.common())
	os.WriteFile(record, []byte("{"), 0o600)
	err2 := checkHostRecord(context.Background(), pg, filepath.Dir(record), f.common())
	for _, e := range []error{err, err2} {
		if e == nil || !strings.Contains(e.Error(), "waggle dispatch clean --host-record") || strings.Contains(e.Error(), "delete") {
			t.Errorf("message %v", e)
		}
	}
}
