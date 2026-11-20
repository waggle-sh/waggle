// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

var update = flag.Bool("update", false, "rewrite the golden files")

func golden(t *testing.T, name string, got []byte) {
	t.Helper()
	p := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(p, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if string(want) != string(got) {
		t.Fatalf("%s differs from the golden file (run with -update to rewrite it):\n%s", name, got)
	}
}

func TestSizeFactorGolden(t *testing.T) {
	var b strings.Builder
	for half := int64(0); half <= 800; half++ {
		fmt.Fprintf(&b, "%d %d\n", half*500, rulesV1.sizeFactor(half*500))
	}
	golden(t, "size_factor.golden", []byte(b.String()))
	for _, tc := range []struct{ changed, want int64 }{{0, 1000}, {25_000, 2000}, {100_000, 3322}, {400_000, 5087}, {900_000, 5087}} {
		if got := rulesV1.sizeFactor(tc.changed); int64(got) != tc.want {
			t.Errorf("sizeFactor(%d) = %d, want %d", tc.changed, got, tc.want)
		}
	}
	prev := Milli(0)
	for half := int64(0); half <= 800; half++ {
		v := rulesV1.sizeFactor(half * 500)
		if v < prev || v < 1000 || v > 5087 {
			t.Fatalf("size factor not monotonic in range at %d", half)
		}
		prev = v
	}
}

func TestMilli(t *testing.T) {
	for _, tc := range []struct {
		m    Milli
		want string
	}{{0, "0.000"}, {1, "0.001"}, {999, "0.999"}, {1000, "1.000"}, {1234567891, "1234567.891"}, {5087, "5.087"}} {
		b, err := tc.m.MarshalJSON()
		if err != nil || string(b) != tc.want {
			t.Errorf("%d: %s %v", tc.m, b, err)
		}
	}
	if _, err := Milli(-1).MarshalJSON(); err == nil {
		t.Error("a negative number must not marshal")
	}
	if got := mulMilli(1485, 3000); got != 4455 {
		t.Errorf("mulMilli = %d", got)
	}
	if got := mulMilli(1, 500); got != 1 { // 0.5 rounds half up
		t.Errorf("half up: %d", got)
	}
}

var (
	decimal  = regexp.MustCompile(`^[0-9]+\.[0-9]{3}$`)
	numField = regexp.MustCompile(`"(nectar|matured|pending|clawed_back|weight|keeper_share|share|size|review|changed_lines)": ?([^,}\s]+)`)
	exponent = regexp.MustCompile(`[:,\[] ?-?[0-9.]+[eE][+-]?[0-9]`)
)

// checkNumbers checks every non-integer field is a plain 3 decimal number in the raw text.
func checkNumbers(t *testing.T, raw []byte) {
	t.Helper()
	for _, m := range numField.FindAllSubmatch(raw, -1) {
		if !decimal.Match(m[2]) {
			t.Errorf("%s = %s is not a 3 decimal number", m[1], m[2])
		}
	}
	if exponent.Match(raw) {
		t.Error("exponent form in output")
	}
}

func reportFixture(t *testing.T) Report {
	f := newFx(t)
	a := f.squash("T-001", nil)
	b := f.pr("t-002-x", "T-002: m", map[string]string{"m.txt": lines(120, "m"), ".waggle/hive/n.md": lines(4, "n")})
	f.tick(40 * day)
	c := f.squash("T-003", map[string]string{"c.txt": lines(1, "c")})
	f.addReceipt(f.receipt("T-001", "alice", a), "")
	f.addReceipt(f.receipt("T-002", "bob", b, func(r *comb.Receipt) { r.CoKeepers = []string{"alice"} }), "")
	f.addReceipt(f.receipt("T-003", "carol", c), "")
	f.commit("no task here", nil)
	return f.score(f.clock.Add(day))
}

func TestReportShape(t *testing.T) {
	rep := reportFixture(t)
	sum, err := json.Marshal(rep)
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(sum, &top); err != nil {
		t.Fatal(err)
	}
	want := strings.Fields("computed_at keepers main_sha orphaned repo rules_version schema_version unattributed")
	if len(top) != len(want) {
		t.Fatalf("fields %v, want %v", top, want)
	}
	for _, k := range want {
		if _, ok := top[k]; !ok {
			t.Fatalf("missing field %s", k)
		}
	}
	checkNumbers(t, sum)
	var s struct {
		Keepers []map[string]json.RawMessage `json:"keepers"`
	}
	if err := json.Unmarshal(sum, &s); err != nil || len(s.Keepers) != 3 {
		t.Fatalf("%v %s", err, sum)
	}
	for _, k := range s.Keepers {
		if len(k) != 6 {
			t.Fatalf("keeper fields: %v", k)
		}
	}
	for i, k := range rep.Summary.Keepers {
		if i > 0 && rep.Summary.Keepers[i-1].KeeperID >= k.KeeperID {
			t.Fatal("keepers not sorted")
		}
		if k.Nectar != max(0, k.Matured-k.ClawedBack) {
			t.Fatalf("%+v", k)
		}
	}
	golden(t, "summary.golden.json", append(sum, '\n'))
	d, err := rep.Detail()
	if err != nil {
		t.Fatal(err)
	}
	checkNumbers(t, d)
	golden(t, "detail.golden.json", append(d, '\n'))
}

func TestPendingOnlyKeeperPresent(t *testing.T) {
	f := newFx(t)
	a := f.squash("T-001", nil)
	f.addReceipt(f.receipt("T-001", "alice", a), "")
	rep := f.score(f.clock.Add(time.Hour))
	k := keeper(rep, "alice")
	if k == nil || k.Nectar != 0 || k.Pending != w10 || k.Units != 1 {
		t.Fatalf("%+v", k)
	}
	raw, _ := json.Marshal(rep)
	if !strings.Contains(string(raw), `"nectar":0.000`) {
		t.Fatal(string(raw))
	}
}

func TestNumberFormatLarge(t *testing.T) {
	s := Summary{Keepers: []KeeperScore{{KeeperID: "x", Nectar: 1234567891, Matured: 1234567891, Pending: 1, Units: 1}}}
	raw, err := json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	checkNumbers(t, raw)
	if !strings.Contains(string(raw), `"nectar":1234567.891`) || !strings.Contains(string(raw), `"pending":0.001`) {
		t.Fatal(string(raw))
	}
}
