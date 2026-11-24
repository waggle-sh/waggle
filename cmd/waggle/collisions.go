// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"

	"github.com/waggle-sh/waggle/internal/comb"
)

// Wording for a case collision, shared by waggle status and waggle doctor.
const (
	collisionImpact = "they are one directory on macOS and Windows"
	collisionFix    = "release one, or ask the first login in gate.approvers to clear it with waggle force-release"
)

// collisionPair names the two claims of one collision.
func collisionPair(c comb.CaseCollision) string {
	return fmt.Sprintf("%s %s and %s %s differ only in letter case", c.TicketA, c.DirA, c.TicketB, c.DirB)
}
