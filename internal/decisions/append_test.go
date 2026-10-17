package decisions

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite testdata/backfill.golden")

// assertClean fails if a lock or temp file is left in dir.
func assertClean(t *testing.T, dir string) {
	t.Helper()
	names, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range names {
		if filepath.Ext(n.Name()) == ".lock" || filepath.Ext(n.Name()) == ".tmp" {
			t.Errorf("leftover file %s", n.Name())
		}
	}
}

func TestFirstAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DECISIONS.md")
	e, err := Append(path, valid(), clock)
	if err != nil {
		t.Fatal(err)
	}
	if e.ID != 1 || e.Date != "2026-10-17" || e.Kind != KindTechnical || e.Tickets[0] != 4 {
		t.Errorf("entry %+v", e)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(data, Render([]Entry{e})) {
		t.Errorf("file != Render(entry):\n%s", data)
	}
	if !bytes.HasPrefix(data, []byte(header+"\n## D-001: ")) {
		t.Errorf("file does not start with header and D-001:\n%s", data)
	}
	fi, _ := os.Stat(path)
	if fi.Mode().Perm() != 0o644 {
		t.Errorf("mode %o", fi.Mode().Perm())
	}
	assertClean(t, dir)
}

func TestSequentialAppends(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DECISIONS.md")
	var prev []byte
	for i := 1; i <= 3; i++ {
		e, err := Append(path, valid(), clock)
		if err != nil {
			t.Fatal(err)
		}
		if e.ID != i {
			t.Errorf("append %d got ID %d", i, e.ID)
		}
		data, _ := os.ReadFile(path)
		entries, err := Parse(data)
		if err != nil || len(entries) != i || !bytes.Equal(Render(entries), data) {
			t.Fatalf("after append %d: %d entries, err %v", i, len(entries), err)
		}
		if !bytes.HasPrefix(data, prev) || len(data) <= len(prev) {
			t.Errorf("append %d: previous bytes are not a strict prefix", i)
		}
		prev = data
	}
}

func TestGoldenBackfill(t *testing.T) {
	files, err := filepath.Glob(filepath.Join("testdata", "backfill", "D-*.json"))
	if err != nil || len(files) < 85 {
		t.Fatalf("backfill inputs: %d files, err %v", len(files), err)
	}
	sort.Strings(files)
	path := filepath.Join(t.TempDir(), "DECISIONS.md")
	for i, f := range files {
		if want := fmt.Sprintf("D-%03d.json", i+1); filepath.Base(f) != want {
			t.Fatalf("file %d is %s, want %s", i, filepath.Base(f), want)
		}
		src, err := os.Open(f)
		if err != nil {
			t.Fatal(err)
		}
		in, err := DecodeInput(src)
		_ = src.Close()
		if err != nil {
			t.Fatalf("%s: %v", f, err)
		}
		if e, err := Append(path, in, clock); err != nil || e.ID != i+1 {
			t.Fatalf("%s: ID %d, err %v", f, e.ID, err)
		}
	}
	got, _ := os.ReadFile(path)
	golden := filepath.Join("testdata", "backfill.golden")
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("rendered backfill differs from %s (run with -update to regenerate)", golden)
	}
	entries, err := Parse(got)
	if err != nil || len(entries) != len(files) {
		t.Errorf("golden parse: %d entries, err %v", len(entries), err)
	}
}

func TestLock(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DECISIONS.md")
	if err := os.WriteFile(path+".lock", []byte("1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Append(path, valid(), clock)
	if !errors.Is(err, ErrLocked) {
		t.Fatalf("got %v, want ErrLocked", err)
	}
	for _, want := range []string{path + ".lock", "safe", "waggle decide"} {
		if !bytes.Contains([]byte(err.Error()), []byte(want)) {
			t.Errorf("lock error %q lacks %q", err, want)
		}
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("file was written while locked")
	}
	if err := os.Remove(path + ".lock"); err != nil {
		t.Fatal(err)
	}
	if _, err := Append(path, valid(), clock); err != nil {
		t.Fatal(err)
	}
	assertClean(t, dir)
}

func TestConcurrentAppends(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DECISIONS.md")
	const n = 8
	start := make(chan struct{})
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for {
				_, err := Append(path, valid(), clock)
				if errors.Is(err, ErrLocked) {
					time.Sleep(time.Millisecond)
					continue
				}
				if err != nil {
					errs <- err
				}
				return
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	entries, err := Parse(data)
	if err != nil || len(entries) != n {
		t.Fatalf("%d entries, err %v", len(entries), err)
	}
	for i, e := range entries {
		if e.ID != i+1 {
			t.Errorf("entry %d has ID %d", i, e.ID)
		}
	}
	assertClean(t, dir)
}

func TestClockBackwards(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DECISIONS.md")
	if _, err := Append(path, valid(), clock.AddDate(0, 0, 1)); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	_, err := Append(path, valid(), clock)
	if !errors.Is(err, ErrInvalidInput) {
		t.Errorf("got %v, want ErrInvalidInput", err)
	}
	if after, _ := os.ReadFile(path); !bytes.Equal(before, after) {
		t.Error("file changed")
	}
	// Same day and a later day are fine; the date is UTC.
	if e, err := Append(path, valid(), time.Date(2026, 10, 18, 23, 30, 0, 0, time.FixedZone("west", -5*3600))); err != nil || e.Date != "2026-10-19" {
		t.Errorf("UTC date: %s, %v", e.Date, err)
	}
}

func TestNextDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "DECISIONS.md")
	e, err := Next(path, valid(), clock)
	if err != nil || e.ID != 1 {
		t.Fatalf("Next: %+v, %v", e, err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Error("Next created the file")
	}
	assertClean(t, dir)
}
