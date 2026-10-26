package comb

import "testing"

// T-040 rows 7 and 8: the sentinel texts a gate-only user reads.
func TestSentinelPlainTerms(t *testing.T) {
	for _, c := range []struct {
		name string
		err  error
		want string
	}{
		{"ErrWaggleDir", ErrWaggleDir, "comb: leases never include .waggle/; files there change only through waggle commands or an admin merge"},
		{"ErrNotOwner", ErrNotOwner, "comb: claim held by another owner"},
	} {
		if got := c.err.Error(); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}
