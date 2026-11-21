// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"bytes"
	"errors"
	"flag"
	"math/rand"
	"os"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

func testRound(items ...string) Round {
	return Round{Version: Version, Round: 1, Credits: 100, RoundDays: 14,
		Base: "0123456789abcdef0123456789abcdef01234567", OpensAt: "2026-11-20T12:00:00Z", ClosesAt: "2026-12-04T12:00:00Z",
		Items: items,
		NectarAt: NectarAt{MainSHA: "0123456789abcdef0123456789abcdef01234567", Now: "2026-11-20T12:00:00Z",
			ReceiptsSHA: "0000000000000000000000000000000000000000", RulesVersion: 1}}
}

func ballot(keeper string, votes ...any) Ballot {
	b := Ballot{Version: Version, Round: 1, KeeperID: keeper, Votes: []Vote{}}
	for i := 0; i < len(votes); i += 2 {
		n := int64(votes[i+1].(int))
		b.Votes = append(b.Votes, Vote{Item: votes[i].(string), Votes: n})
		b.CreditsUsed += uint64(n * n)
	}
	return b
}

func fixtureNectar(t *testing.T) map[string]uint64 {
	t.Helper()
	data, err := os.ReadFile("testdata/nectar-9.2.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseNectar(data)
	if err != nil {
		t.Fatal(err)
	}
	return m
}

func rfcBallots() []Ballot {
	return []Ballot{
		ballot("A", "T-101", 8, "T-102", 6),
		ballot("B", "T-102", 10),
		ballot("C", "T-101", 3, "T-103", 9),
		ballot("D", "T-103", 10),
		ballot("E", "T-103", 5),
	}
}

func scoreOf(t *testing.T, tl TallyRecord, item string) uint64 {
	t.Helper()
	for _, s := range tl.Items {
		if s.Item == item {
			return s.ScoreMilli
		}
	}
	t.Fatalf("item %s not in tally", item)
	return 0
}

func TestA1Golden(t *testing.T) {
	r := testRound("T-101", "T-102", "T-103")
	n := fixtureNectar(t)
	tl, err := Tally(r, rfcBallots(), n, nil)
	if err != nil {
		t.Fatal(err)
	}
	if w, _ := weightMilli(n["E"]); w != 3774 {
		t.Fatalf("E weight = %d, want 3774", w)
	}
	want := map[string]uint64{"T-102": 220000, "T-101": 175000, "T-103": 45000 + 18870}
	for it, s := range want {
		if got := scoreOf(t, tl, it); got != s {
			t.Errorf("%s score = %d, want %d", it, got, s)
		}
	}
	// D adds raw votes but no score.
	if tl.Items[len(tl.Items)-1].RawVotes != 9+10+5 {
		t.Errorf("T-103 raw votes = %d", tl.Items[len(tl.Items)-1].RawVotes)
	}
	if got := strings.Join(tl.Order, ","); got != "T-102,T-101,T-103" {
		t.Errorf("order = %s", got)
	}

	golden := "testdata/tally-5.6.json"
	got := Encode(tl)
	if *update {
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	wantBytes, err := os.ReadFile(golden)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, wantBytes) {
		t.Errorf("tally bytes differ from %s", golden)
	}

	t.Run("all on T-101", func(t *testing.T) {
		bs := []Ballot{ballot("A", "T-101", 10), ballot("B", "T-102", 10), ballot("C", "T-101", 3, "T-103", 9)}
		tl, err := Tally(r, bs, n, nil)
		if err != nil {
			t.Fatal(err)
		}
		if got := scoreOf(t, tl, "T-101"); got != 215000 {
			t.Errorf("T-101 = %d, want 215000", got)
		}
		if got := scoreOf(t, tl, "T-102"); got != 100000 {
			t.Errorf("T-102 = %d, want 100000", got)
		}
	})
}

func TestA2ParseNectar(t *testing.T) {
	doc := func(nectar string) []byte {
		return []byte(`{"schema_version":1,"keepers":[{"keeper_id":"k","nectar":` + nectar + `,"pending":9.5,"clawed_back":1.5}]}`)
	}
	good := map[string]uint64{"0": 0, "5": 5000, "5.0": 5000, "5.5": 5500, "5.25": 5250, "14.250": 14250, "0.001": 1}
	for in, want := range good {
		t.Run("ok "+in, func(t *testing.T) {
			m, err := ParseNectar(doc(in))
			if err != nil {
				t.Fatal(err)
			}
			if m["k"] != want {
				t.Errorf("got %d, want %d", m["k"], want)
			}
		})
	}
	bad := []string{"-1", "-0.5", "1e3", "1E3", "14.2501", `"NaN"`, `"5"`, "null", "true", "[]"}
	for _, in := range bad {
		t.Run("bad "+in, func(t *testing.T) {
			_, err := ParseNectar(doc(in))
			if !errors.Is(err, ErrNectar) || !strings.Contains(err.Error(), "keeper k") {
				t.Errorf("err = %v, want ErrNectar naming keeper k", err)
			}
		})
	}
	t.Run("malformed fixtures", func(t *testing.T) {
		for _, name := range []string{"negative", "exponent", "four-decimals", "nan-string", "schema-2"} {
			data, err := os.ReadFile("testdata/nectar-9.2-" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := ParseNectar(data); !errors.Is(err, ErrNectar) {
				t.Errorf("%s: err = %v, want ErrNectar", name, err)
			} else if name != "schema-2" && !strings.Contains(err.Error(), "keeper E") {
				t.Errorf("%s: err = %v, want keeper E named", name, err)
			}
		}
	})
	t.Run("schema 2", func(t *testing.T) {
		_, err := ParseNectar([]byte(`{"schema_version":2,"keepers":[]}`))
		if !errors.Is(err, ErrNectar) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("absent keeper is zero", func(t *testing.T) {
		m := fixtureNectar(t)
		if _, ok := m["D"]; ok || m["D"] != 0 {
			t.Errorf("D = %v", m["D"])
		}
		w, _ := weightMilli(m["D"])
		if w != 0 {
			t.Errorf("w(D) = %d", w)
		}
	})
	t.Run("pending and clawed_back do not move weight", func(t *testing.T) {
		a := doc("5")
		b := bytes.ReplaceAll(a, []byte(`"pending":9.5,"clawed_back":1.5`), []byte(`"pending":900,"clawed_back":400,"matured":1,"units":77`))
		ma, err1 := ParseNectar(a)
		mb, err2 := ParseNectar(b)
		if err1 != nil || err2 != nil || ma["k"] != mb["k"] {
			t.Errorf("maps differ: %v %v %v %v", ma, mb, err1, err2)
		}
	})
	t.Run("isqrt", func(t *testing.T) {
		for in, want := range map[uint64]uint64{0: 0, 1: 1, 3: 1, 4: 2, 14250000: 3774, 400000000: 20000, 1<<64 - 1: 1<<32 - 1} {
			if got := isqrt(in); got != want {
				t.Errorf("isqrt(%d) = %d, want %d", in, got, want)
			}
		}
	})
}

func TestA3ValidateBallot(t *testing.T) {
	r := testRound("T-101", "T-102")
	cases := []struct {
		name string
		mut  func(b *Ballot)
		want string
	}{
		{"valid", func(b *Ballot) {}, ""},
		{"over budget", func(b *Ballot) { b.Votes[0].Votes = 11; b.CreditsUsed = 121 }, ReasonOverBudget},
		{"negative", func(b *Ballot) { b.Votes[0].Votes = -1; b.CreditsUsed = 1 }, ReasonNegativeVote},
		{"unknown item", func(b *Ballot) { b.Votes[0].Item = "T-999" }, ReasonUnknownItem},
		{"duplicate item", func(b *Ballot) { b.Votes[1].Item = "T-101" }, ReasonDuplicateItem},
		{"wrong round", func(b *Ballot) { b.Round = 2 }, ReasonRound},
		{"bad keeper id", func(b *Ballot) { b.KeeperID = "-x" }, ReasonKeeperID},
		{"credits mismatch", func(b *Ballot) { b.CreditsUsed = 1 }, ReasonCredits},
		{"bad version", func(b *Ballot) { b.Version = 2 }, ReasonVersion},
		{"bad item grammar", func(b *Ballot) { b.Votes[0].Item = "t-101" }, ReasonItem},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := ballot("alice", "T-101", 5, "T-102", 5)
			c.mut(&b)
			err := ValidateBallot(r, b)
			if got := ReasonOf(err); got != c.want {
				t.Errorf("reason = %q (%v), want %q", got, err, c.want)
			}
		})
	}
	t.Run("huge vote does not wrap", func(t *testing.T) {
		b := ballot("alice")
		b.Votes = []Vote{{Item: "T-101", Votes: 1 << 40}}
		if got := ReasonOf(ValidateBallot(r, b)); got != ReasonOverBudget {
			t.Errorf("reason = %q", got)
		}
	})
	t.Run("decode non-integer", func(t *testing.T) {
		for _, v := range []string{"1.5", "2e0", "1.0"} {
			_, err := DecodeBallot([]byte(`{"version":1,"round":1,"keeper_id":"a","votes":[{"item":"T-101","votes":` + v + `}],"credits_used":1}`))
			if ReasonOf(err) != ReasonNonInteger {
				t.Errorf("%s: err = %v", v, err)
			}
		}
	})
	t.Run("decode ok", func(t *testing.T) {
		b, err := DecodeBallot([]byte(`{"version":1,"round":1,"keeper_id":"a","votes":[{"item":"T-101","votes":3}],"credits_used":9,"prev":""}`))
		if err != nil || b.Votes[0].Votes != 3 || ValidateBallot(r, b) != nil {
			t.Errorf("b = %+v err = %v", b, err)
		}
	})
	t.Run("decode negative then refused", func(t *testing.T) {
		b, err := DecodeBallot([]byte(`{"version":1,"round":1,"keeper_id":"a","votes":[{"item":"T-101","votes":-2}],"credits_used":4}`))
		if err != nil || ReasonOf(ValidateBallot(r, b)) != ReasonNegativeVote {
			t.Errorf("b = %+v err = %v", b, err)
		}
	})
	t.Run("decode garbage", func(t *testing.T) {
		if ReasonOf(func() error { _, e := DecodeBallot([]byte("{")); return e }()) != ReasonDecode {
			t.Error("want decode")
		}
	})
}

func TestA4ItemGrammar(t *testing.T) {
	for _, ok := range []string{"T-101", "T-000", "T-999999", "issue-1", "issue-123456789"} {
		if CheckItem(ok) != nil {
			t.Errorf("%q refused", ok)
		}
	}
	for _, bad := range []string{"T-1", "T-0001234567", "T-1234567", "issue-0", "issue-1234567890", "t-101", "T-101 ", "T-101\n", "", "issue-", "ISSUE-1"} {
		if CheckItem(bad) == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestA5TieBreak(t *testing.T) {
	r := testRound("T-200", "T-150", "issue-150", "issue-7", "T-900")
	n := map[string]uint64{"A": 1000}
	bs := []Ballot{ballot("A", "T-200", 1, "T-150", 1, "issue-150", 1, "issue-7", 1, "T-900", 1)}
	tl, err := Tally(r, bs, n, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(tl.Order, ","); got != "issue-7,T-150,issue-150,T-200,T-900" {
		t.Errorf("order = %s", got)
	}
}

func TestA6Veto(t *testing.T) {
	r := testRound("T-101", "T-102", "T-103")
	n := fixtureNectar(t)
	base, _ := Tally(r, rfcBallots(), n, nil)
	t.Run("one item", func(t *testing.T) {
		tl, err := Tally(r, rfcBallots(), n, []Veto{{Version: 1, Round: 1, Item: "T-102", Decision: "D-1"}})
		if err != nil {
			t.Fatal(err)
		}
		if got := strings.Join(tl.Order, ","); got != "T-101,T-103" {
			t.Errorf("order = %s", got)
		}
		for _, it := range []string{"T-101", "T-102", "T-103"} {
			if scoreOf(t, tl, it) != scoreOf(t, base, it) {
				t.Errorf("%s score moved", it)
			}
		}
		if len(tl.Vetoes) != 1 || tl.Rejected {
			t.Errorf("tally = %+v", tl)
		}
	})
	t.Run("all", func(t *testing.T) {
		tl, _ := Tally(r, rfcBallots(), n, []Veto{{Version: 1, Round: 1, Item: VetoAll, Decision: "D-2"}})
		if !tl.Rejected || len(tl.Order) != 0 || scoreOf(t, tl, "T-102") != 220000 {
			t.Errorf("tally = %+v", tl)
		}
	})
	t.Run("bad veto ignored and reported", func(t *testing.T) {
		tl, _ := Tally(r, rfcBallots(), n, []Veto{{Round: 2, Item: "T-102"}, {Round: 1, Item: "T-555"}})
		if len(tl.Order) != 3 || len(tl.Vetoes) != 0 || len(tl.Malformed) != 2 {
			t.Errorf("tally = %+v", tl)
		}
	})
}

func TestA7Sybil(t *testing.T) {
	r := testRound("T-101")
	one, _ := Tally(r, []Ballot{ballot("whale", "T-101", 1)}, map[string]uint64{"whale": 400000}, nil)
	split := map[string]uint64{"s1": 100000, "s2": 100000, "s3": 100000, "s4": 100000}
	var bs []Ballot
	for k := range split {
		bs = append(bs, ballot(k, "T-101", 1))
	}
	four, _ := Tally(r, bs, split, nil)
	if scoreOf(t, one, "T-101") != 20000 || scoreOf(t, four, "T-101") != 40000 {
		t.Errorf("one = %d, four = %d", scoreOf(t, one, "T-101"), scoreOf(t, four, "T-101"))
	}
}

func TestA8Overflow(t *testing.T) {
	t.Run("parse", func(t *testing.T) {
		_, err := ParseNectar([]byte(`{"schema_version":1,"keepers":[{"keeper_id":"k","nectar":18446744073709552.000}]}`))
		if !errors.Is(err, ErrNectar) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("weight", func(t *testing.T) {
		r := testRound("T-101")
		_, err := Tally(r, []Ballot{ballot("k", "T-101", 1)}, map[string]uint64{"k": 1<<64 - 1}, nil)
		if !errors.Is(err, ErrOverflow) {
			t.Errorf("err = %v", err)
		}
	})
	t.Run("score", func(t *testing.T) {
		// Each ballot alone fits in uint64; their sum does not.
		r := testRound("T-101")
		r.Credits = 1 << 62
		n := map[string]uint64{}
		var bs []Ballot
		for _, k := range []string{"k1", "k2", "k3"} {
			n[k] = (1<<64 - 1) / 1000
			b := ballot(k)
			b.Votes = []Vote{{Item: "T-101", Votes: 1 << 31}}
			b.CreditsUsed = 1 << 62
			bs = append(bs, b)
		}
		if w, ok := weightMilli(n["k1"]); !ok || w != 1<<32-1 {
			t.Fatalf("w = %d ok = %v", w, ok)
		}
		_, err := Tally(r, bs, n, nil)
		if !errors.Is(err, ErrOverflow) {
			t.Errorf("err = %v", err)
		}
	})
}

func TestA9Encoding(t *testing.T) {
	r := testRound("T-101", "T-102", "T-103", "issue-9")
	n := fixtureNectar(t)
	bs := rfcBallots()
	vs := []Veto{{Round: 1, Item: "T-103", Decision: "D-3"}, {Round: 1, Item: "T-101", Decision: "D-4"}, {Round: 9, Item: "T-101"}}
	var first []byte
	rng := rand.New(rand.NewSource(1))
	for i := 0; i < 20; i++ {
		rr := r
		rr.Items = append([]string{}, r.Items...)
		rng.Shuffle(len(rr.Items), func(a, b int) { rr.Items[a], rr.Items[b] = rr.Items[b], rr.Items[a] })
		bb := append([]Ballot{}, bs...)
		rng.Shuffle(len(bb), func(a, b int) { bb[a], bb[b] = bb[b], bb[a] })
		for j := range bb {
			bb[j].Votes = append([]Vote{}, bb[j].Votes...)
			rng.Shuffle(len(bb[j].Votes), func(a, b int) { bb[j].Votes[a], bb[j].Votes[b] = bb[j].Votes[b], bb[j].Votes[a] })
		}
		vv := append([]Veto{}, vs...)
		rng.Shuffle(len(vv), func(a, b int) { vv[a], vv[b] = vv[b], vv[a] })
		tl, err := Tally(rr, bb, n, vv)
		if err != nil {
			t.Fatal(err)
		}
		got := append(Encode(tl), Encode(rr)...)
		for _, b := range bb {
			if b.KeeperID == "A" {
				got = append(got, Encode(b)...)
			}
		}
		if first == nil {
			first = got
		} else if !bytes.Equal(first, got) {
			t.Fatal("bytes differ between input orders")
		}
	}
	if !bytes.HasSuffix(first, []byte("}\n")) || bytes.Contains(first, []byte("\t")) || !bytes.Contains(first, []byte("\n  \"")) {
		t.Error("not two-space indented with a trailing newline")
	}
}

func TestMalformedBallotsListed(t *testing.T) {
	r := testRound("T-101")
	over := ballot("bad", "T-101", 11)
	badID := ballot("evil\nid", "T-101", 1)
	dupA, dupB := ballot("dup", "T-101", 1), ballot("dup", "T-101", 2)
	good := ballot("good", "T-101", 3)
	tl, err := Tally(r, []Ballot{over, badID, dupA, dupB, good}, map[string]uint64{"good": 1000}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(tl.Counted) != 1 || tl.Counted[0] != "good" || len(tl.Malformed) != 4 {
		t.Fatalf("tally = %+v", tl)
	}
	for _, m := range tl.Malformed {
		if strings.Contains(m.Ref, "\n") {
			t.Errorf("ref echoes ballot text: %q", m.Ref)
		}
	}
}
