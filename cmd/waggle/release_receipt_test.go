// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"testing"
)

// A10: the release help names --co in plain terms.
func TestReleaseHelpCoPlainTerms(t *testing.T) {
	var out, errb bytes.Buffer
	runTaskRelease([]string{"-h"}, &out, &errb)
	checkPlain(t, "release -h", errb.String(), "-co", "shares the credit with you", "[--co <id>]...")
}
