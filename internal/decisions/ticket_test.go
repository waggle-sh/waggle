// SPDX-License-Identifier: Apache-2.0

package decisions

import (
	"fmt"
	"testing"

	"github.com/waggle-sh/waggle/internal/comb"
)

func TestParseTicketAgreesWithComb(t *testing.T) {
	for _, s := range []string{"1", "001", "T-1", "T-001", "T-999999", "0", "T-0", "t-1", "T-1234567", "", "T-", " 1"} {
		n, err := parseTicket(s)
		c, cerr := comb.ParseTicketID(s)
		if (err == nil) != (cerr == nil) {
			t.Fatalf("%q: decisions err=%v, comb err=%v", s, err, cerr)
		}
		if err == nil && c != fmt.Sprintf("T-%03d", n) {
			t.Errorf("%q: decisions %d, comb %s", s, n, c)
		}
	}
}
