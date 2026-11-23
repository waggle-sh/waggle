// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"
)

func TestRunUpgradeAuthorNeedsPR(t *testing.T) {
	if code, _, errb := runUp("--author", "A <a@b.c>"); code != 2 || !strings.Contains(errb, "--author needs --pr") {
		t.Errorf("code %d, stderr %q", code, errb)
	}
}

func TestRunUpgradePRNeedsApply(t *testing.T) {
	if code, _, errb := runUp("--pr"); code != 2 || !strings.Contains(errb, "--pr needs --apply") {
		t.Errorf("code %d, stderr %q", code, errb)
	}
	if code, _, errb := runUp("--bogus"); code != 2 || !strings.Contains(errb, "--to <tag-or-sha>") {
		t.Errorf("usage should document --to: code %d, stderr %q", code, errb)
	}
}
