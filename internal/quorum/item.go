// SPDX-License-Identifier: Apache-2.0

package quorum

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
)

// itemPattern is the RFC-0006 4.1 grammar. Go's $ matches only at the end of
// the text, so a trailing newline or space is refused.
var itemPattern = regexp.MustCompile(`^(T-[0-9]{3,6}|issue-[0-9]{1,9})$`)

// keeperIDPattern is the D-016 pattern, as in internal/config.
var keeperIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@+-]{0,127}$`)

// ErrItem reports an item that does not match the grammar.
var ErrItem = errors.New("quorum: item does not match the item grammar")

// CheckItem validates an item id. It runs before any other check on an item.
func CheckItem(item string) error {
	if _, _, ok := parseItem(item); !ok {
		return ErrItem
	}
	return nil
}

// parseItem splits a valid item into its kind rank and number. T- ranks
// before issue- at an equal number (Q-T054-7). A zero issue number is
// refused (test A4).
func parseItem(item string) (rank int, num uint64, ok bool) {
	if !itemPattern.MatchString(item) {
		return 0, 0, false
	}
	var digits string
	if strings.HasPrefix(item, "T-") {
		rank, digits = 0, item[2:]
	} else {
		rank, digits = 1, item[6:]
	}
	n, err := strconv.ParseUint(digits, 10, 64)
	if err != nil || (rank == 1 && n == 0) {
		return 0, 0, false
	}
	return rank, n, true
}

// itemLess orders by number, then T- before issue-. Text that is not a valid
// item sorts after valid items, by text, so the order stays total.
func itemLess(a, b string) bool {
	ra, na, oka := parseItem(a)
	rb, nb, okb := parseItem(b)
	switch {
	case oka && okb:
		if na != nb {
			return na < nb
		}
		return ra < rb
	case oka != okb:
		return oka
	}
	return a < b
}

func scoreLess(a, b ItemScore) bool {
	if a.ScoreMilli != b.ScoreMilli {
		return a.ScoreMilli > b.ScoreMilli
	}
	return itemLess(a.Item, b.Item)
}
