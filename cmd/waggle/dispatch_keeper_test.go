// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

// T-045 AC8: --help lists --keeper-commit in the guard usage line.
func TestDispatchHelpListsKeeperCommit(t *testing.T) {
	var out, errb bytes.Buffer
	if runDispatch([]string{"--help"}, &out, &errb); !strings.Contains(errb.String(), "--no-cell [--second] [--keeper-commit] [--base <rev>]") {
		t.Fatalf("usage does not list --keeper-commit: %s", errb.String())
	}
}
