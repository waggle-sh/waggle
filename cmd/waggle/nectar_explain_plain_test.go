// SPDX-License-Identifier: Apache-2.0

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// These tests need no git, so they run on every platform.

// Every reason code the scorer writes has a plain sentence.
func TestNectarReasonTable(t *testing.T) {
	for _, code := range []string{"no_binding", "unconfirmed_revert", "min_changed_lines", "receipt_unmerged", "no_valid_receipt", "third_party",
		"fork_mismatch", "receipt_incomplete", "self_merge_multi_keeper", "direct_push", "restored", "revert_before_maturity",
		"revert_after_maturity", "partial_revert", "superseded", "alive_below_ratio", "reciprocal_pair", "daily_cap_carry",
		"orphaned_rewritten", "orphaned_missing"} {
		s, ok := reasonText[code]
		if !ok || s == "" {
			t.Errorf("no sentence for %s", code)
		}
		checkPlain(t, code, s)
	}
}

// B7: docs/nectar.md names every flag and exit code of both commands.
func TestNectarDocNamesFlags(t *testing.T) {
	b, err := os.ReadFile(filepath.Join("..", "..", "docs", "nectar.md"))
	if err != nil {
		t.Fatal(err)
	}
	doc := string(b)
	for _, usage := range []string{summaryUsage, explainUsage} {
		for _, f := range regexp.MustCompile(`--[a-z-]+`).FindAllString(usage, -1) {
			if !strings.Contains(doc, "`"+f) {
				t.Errorf("doc lacks flag %s", f)
			}
		}
	}
	for _, code := range []string{"`0`", "`1`", "`2`", "`3`"} {
		if strings.Count(doc, code) < 2 {
			t.Errorf("doc names exit code %s fewer than twice", code)
		}
	}
	if strings.ContainsAny(doc, dashes) {
		t.Error("doc holds an en or em dash")
	}
}
