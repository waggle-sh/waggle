// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// gateCountsSchema is the version of the file `waggle gate verify --counts`
// writes (T-107).
const gateCountsSchema = 1

// checkCounts is the number of non-warning findings per check. Binding
// failures are folded into the leases check, so binding is always 0.
type checkCounts struct {
	Binding   int `json:"binding"`
	Decisions int `json:"decisions"`
	Leases    int `json:"leases"`
	Tiers     int `json:"tiers"`
}

// gateCounts is the whole file. Field order is key order; encoding/json keeps
// it. It holds integers only: no path, message, SHA, login or name.
type gateCounts struct {
	Failed    checkCounts `json:"failed"`
	Schema    int         `json:"schema"`
	WouldFail checkCounts `json:"would_fail"`
}

// countGate counts non-warning findings. Under open every finding is a
// would-fail and failed stays zero; under guard and hold it is the reverse.
// decisions is true when the decisions check failed (or would fail).
func countGate(findings []gateFinding, open, decisions bool) gateCounts {
	var c checkCounts
	for _, f := range findings {
		switch {
		case f.warning:
		case f.check == checkLeases:
			c.Leases++
		case f.check == checkTiers:
			c.Tiers++
		}
	}
	if decisions {
		c.Decisions = 1
	}
	out := gateCounts{Schema: gateCountsSchema}
	if open {
		out.WouldFail = c
	} else {
		out.Failed = c
	}
	return out
}

// decisionsFailed reports whether the decisions check failed, from its exit
// code and stdout. Under open a failure exits 0 and prints no stdout line, a
// pass always prints one. Exit 3 means the check could not run: not counted.
func decisionsFailed(code int, open bool, stdout string) bool {
	return code == gateFail || (open && code == gatePass && !strings.HasPrefix(stdout, "decisions: "))
}

// writeGateCounts writes c to path atomically: a temp file in the same
// directory, then a rename. The file is mode 0644.
func writeGateCounts(path string, c gateCounts) (err error) {
	data, err := json.Marshal(c)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	f, err := os.CreateTemp(filepath.Dir(path), ".waggle-gate-counts-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			f.Close()
			os.Remove(f.Name())
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Chmod(0o644); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), path)
}
