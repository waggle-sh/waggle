// SPDX-License-Identifier: Apache-2.0

package main

import (
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/forge"
)

// holdFindings adds the mode-fit finding to a GitHub report and, on a hold
// hive whose forge cannot require a check, marks the two findings no setting
// could make ok as skipped. It reads facts and advises; it never changes a
// mode. A report with no capability (the none forge) is left as it is.
func holdFindings(r *forge.Report, mode string) {
	if r.Capability == "" {
		return
	}
	if mode == config.GateModeHold && r.Capability == forge.CapabilityNone {
		r.Hold = true
		for i, f := range r.Findings {
			if f.ID == "required-gate" || f.ID == "up-to-date" {
				r.Findings[i] = forge.Finding{ID: f.ID, Status: forge.StatusSkipped,
					Summary: "Not used: gate.mode is hold and this repo cannot require a check."}
			}
		}
	}
	r.Findings = append(r.Findings, modeFit(mode, r.Capability))
}

// modeFit says which gate.mode fits what the forge can hold. It is advice
// only and never required, so it cannot change doctor's exit code.
func modeFit(mode, capability string) forge.Finding {
	f := forge.Finding{ID: "mode-fit", Status: forge.StatusOK}
	const noneFix = "ask the Head Keeper to consider gate.mode hold. waggle merge and the audit workflow are not in this build yet. waggle doctor never changes the mode."
	switch {
	case capability == forge.CapabilityRequired && mode == config.GateModeGuard:
		f.Summary = "gate.mode guard fits: a rule can require the gate check here."
	case capability == forge.CapabilityRequired && mode == config.GateModeHold:
		f.Status = forge.StatusMissing
		f.Summary = "gate.mode hold works, but a stronger mode is available: this repo can require the gate check."
		f.NotEnforced = "a manual merge click lands first and is detected after, where a required check would block it."
		f.Fix = "ask the Head Keeper to move gate.mode to guard and add a ruleset that requires verify-leases-and-tiers. waggle doctor never changes the mode."
	case capability == forge.CapabilityRequired:
		f.Summary = "gate.mode open is the staging mode. guard is available here when you are ready."
	case capability == forge.CapabilityNone && mode == config.GateModeHold:
		f.Summary = "gate.mode hold fits: no rule can require a check here, so waggle merge holds the merge."
	case capability == forge.CapabilityNone && mode == config.GateModeGuard:
		f.Status = forge.StatusMissing
		f.Summary = "gate.mode guard cannot be enforced here: no rule can require the gate check. Recommended: hold."
		f.NotEnforced = "a pull request can merge without the gate passing."
		f.Fix = noneFix
	case capability == forge.CapabilityNone:
		f.Status = forge.StatusMissing
		f.Summary = "gate.mode open records findings and holds nothing, and no rule can require a check here. Recommended: hold."
		f.NotEnforced = "a pull request can merge whatever the gate finds."
		f.Fix = noneFix
	default:
		f.Status = forge.StatusUnknown
		f.Summary = "Could not tell which mode fits (capability unknown)."
		f.NotEnforced = "this run cannot say whether gate.mode " + mode + " can be enforced."
		f.Fix = "fix the capability line above, then run waggle doctor again."
	}
	return f
}
