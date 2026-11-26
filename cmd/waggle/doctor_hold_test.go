// SPDX-License-Identifier: Apache-2.0

package main

import (
	"strings"
	"testing"

	"github.com/waggle-sh/waggle/internal/forge"
)

const (
	guardNoneFix = "ask the Head Keeper to consider gate.mode hold. waggle merge and the audit workflow are not in this build yet. waggle doctor never changes the mode."
	holdSkipped  = "Not used: gate.mode is hold and this repo cannot require a check."
)

func TestModeFitTable(t *testing.T) {
	tests := []struct {
		capab, mode string
		status      forge.Status
		summary     string
		notEnforced string
		fix         string
	}{
		{"required_check", "guard", forge.StatusOK, "gate.mode guard fits: a rule can require the gate check here.", "", ""},
		{"required_check", "hold", forge.StatusMissing, "gate.mode hold works, but a stronger mode is available: this repo can require the gate check.",
			"a manual merge click lands first and is detected after, where a required check would block it.",
			"ask the Head Keeper to move gate.mode to guard and add a ruleset that requires verify-leases-and-tiers. waggle doctor never changes the mode."},
		{"required_check", "open", forge.StatusOK, "gate.mode open is the staging mode. guard is available here when you are ready.", "", ""},
		{"none", "hold", forge.StatusOK, "gate.mode hold fits: no rule can require a check here, so waggle merge holds the merge.", "", ""},
		{"none", "guard", forge.StatusMissing, "gate.mode guard cannot be enforced here: no rule can require the gate check. Recommended: hold.",
			"a pull request can merge without the gate passing.", guardNoneFix},
		{"none", "open", forge.StatusMissing, "gate.mode open records findings and holds nothing, and no rule can require a check here. Recommended: hold.",
			"a pull request can merge whatever the gate finds.", guardNoneFix},
		{"unknown", "guard", forge.StatusUnknown, "Could not tell which mode fits (capability unknown).",
			"this run cannot say whether gate.mode guard can be enforced.", "fix the capability line above, then run waggle doctor again."},
		{"unknown", "hold", forge.StatusUnknown, "Could not tell which mode fits (capability unknown).",
			"this run cannot say whether gate.mode hold can be enforced.", "fix the capability line above, then run waggle doctor again."},
		{"unknown", "open", forge.StatusUnknown, "Could not tell which mode fits (capability unknown).",
			"this run cannot say whether gate.mode open can be enforced.", "fix the capability line above, then run waggle doctor again."},
	}
	for _, tt := range tests {
		t.Run(tt.capab+"/"+tt.mode, func(t *testing.T) {
			f := modeFit(tt.mode, tt.capab)
			if f.ID != "mode-fit" || f.Required || f.Status != tt.status || f.Summary != tt.summary ||
				f.NotEnforced != tt.notEnforced || f.Fix != tt.fix {
				t.Errorf("got %+v", f)
			}
			// M2: hold is never recommended where a check can be required,
			// and guard never where none can.
			if tt.capab == "required_check" && strings.Contains(f.Summary+f.Fix, "Recommended: hold") {
				t.Error("recommends hold where guard is possible")
			}
			if tt.capab == "none" && strings.Contains(f.Summary+f.Fix, "move gate.mode to guard") {
				t.Error("recommends guard where no check can be required")
			}
		})
	}
}

func capReport(capab string, fs ...forge.Finding) forge.Report {
	r := report("up_to_date", fs...)
	r.Capability = capab
	return r
}

func modeCfg(mode string) string { return githubCfg + "gate:\n  mode: " + mode + "\n" }

func TestDoctorHoldExit(t *testing.T) {
	// M3 and M4: the exit code is what main gives for the same required
	// findings, except hold with capability none.
	rawGate := forge.Finding{ID: "required-gate", Status: forge.StatusUnknown, Required: true, Summary: "x", NotEnforced: "n", Fix: "f"}
	rawUp := forge.Finding{ID: "up-to-date", Status: forge.StatusUnknown, Required: true, Summary: "x", NotEnforced: "n", Fix: "f"}
	for _, mode := range []string{"guard", "open", "hold"} {
		for _, capab := range []string{"required_check", "none", "unknown"} {
			for _, bad := range []bool{false, true} {
				gate, up := okGate, okUpTo
				want := 0
				if bad {
					gate, up, want = rawGate, rawUp, 1
					if mode == "hold" && capab == "none" {
						want = 0
					}
				}
				name := mode + "/" + capab
				if bad {
					name += "/raw"
				}
				t.Run(name, func(t *testing.T) {
					dir := doctorRepo(t, modeCfg(mode))
					seamForge(t, &fakeForge{report: capReport(capab, gate, up)}, okClaims)
					code, out, _ := runDoc("--repo", dir, "--no-write")
					if code != want {
						t.Errorf("exit %d, want %d\n%s", code, want, out)
					}
					if !strings.Contains(out, "mode-fit") {
						t.Errorf("no mode-fit line:\n%s", out)
					}
				})
			}
		}
	}
}

func TestDoctorHoldSkipAndEnforcement(t *testing.T) {
	dir := doctorRepo(t, modeCfg("hold"))
	seamForge(t, &fakeForge{report: capReport("none", okGate, okUpTo)}, okClaims)
	code, out, _ := runDoc("--repo", dir, "--no-write")
	if code != 0 {
		t.Errorf("exit %d", code)
	}
	for _, want := range []string{
		"Enforcement: advisory (hold parts not checked)\n",
		"skipped   required-gate   " + holdSkipped + "\n",
		"skipped   up-to-date      " + holdSkipped + "\n",
		"ok        mode-fit        gate.mode hold fits: no rule can require a check here, so waggle merge\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// E1: other modes and capabilities keep the Enforcement line of main.
	for _, tc := range []struct{ mode, capab string }{
		{"guard", "none"}, {"guard", "required_check"}, {"open", "none"}, {"open", "required_check"}, {"hold", "required_check"},
	} {
		dir := doctorRepo(t, modeCfg(tc.mode))
		seamForge(t, &fakeForge{report: capReport(tc.capab, okGate, okUpTo)}, okClaims)
		_, out, _ := runDoc("--repo", dir, "--no-write")
		if !strings.Contains(out, "Enforcement: enforced\n") || strings.Contains(out, "hold parts") || strings.Contains(out, "skipped") {
			t.Errorf("%s/%s changed:\n%s", tc.mode, tc.capab, out)
		}
	}
}

func TestDoctorNoCapabilityPrintsNothingNew(t *testing.T) {
	dir := doctorRepo(t, modeCfg("hold"))
	seamForge(t, &fakeForge{report: report("up_to_date", okGate, okUpTo)}, okClaims)
	code, out, _ := runDoc("--repo", dir, "--no-write")
	if code != 0 || strings.Contains(out, "mode-fit") || !strings.Contains(out, "Enforcement: enforced\n") {
		t.Errorf("exit %d\n%s", code, out)
	}
}

func TestDoctorFitLinesSameWithAndWithoutWrite(t *testing.T) {
	dir := doctorRepo(t, modeCfg("guard"))
	lines := func(args ...string) string {
		seamForge(t, &fakeForge{report: capReport("none", okGate, okUpTo)}, okClaims)
		_, out, _ := runDoc(append([]string{"--repo", dir}, args...)...)
		var keep []string
		for _, l := range strings.Split(out, "\n") {
			if strings.Contains(l, "mode-fit") || strings.Contains(l, "capability") {
				keep = append(keep, l)
			}
		}
		return strings.Join(keep, "\n")
	}
	a, b := lines(), lines("--no-write")
	if a == "" || a != b {
		t.Errorf("write %q, no-write %q", a, b)
	}
}
