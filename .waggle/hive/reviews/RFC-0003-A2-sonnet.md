<!-- SPDX-License-Identifier: MIT-0 -->
# Review: RFC-0003 Amendment A2 (PR #31, `2b7b745`, guard 1 of 2, sonnet)

Verdict: pass (re-review of 1571737; original 2b7b745 was must-fix)
Risk-Tier: 3

## Intent Summary
Docs-only change to `docs/RFC-0003.md` (12 insertions, 6 deletions). It replaces the "dev container with ro root" Cell description with a `CellRuntime` interface and three backends (Seatbelt, bubblewrap, container), moves egress to a per-Cell host proxy, states `.git` is read-only with host-side commits, adds the D-120 `passthrough` opt-in, and appends an A2 entry shaped like A1. Amended: status line, 2.1 Cell, 3.2 item 1, 3.5 (two bullets), 4 item 3, Appendix A. No code. Blast radius: the RFC is the frozen spec the T-024 phases are built and reviewed against, so false text here becomes false guard criteria. Tier 3 because it states the isolation guarantees `waggle doctor` reports.

Checked and fine: the claims match the T-024 brief and D-101, D-120 to D-127. Covered: zero credentials via env allowlist and empty home, `ro` worktree with `rw` leases, Seatbelt as path rules with hard-link denial, `.git` read-only with host commits, leased dirs created on host, egress proxy by host name refusing IP literals and loopback/link-local/private, request logging, `.waggle/control/devcontainer.json` (D-030), probe per target, no unsandboxed fallback, `waggle doctor` output, and the closing sentence on provider egress (brief 6.4). A1 shape is matched: heading, Approved by, Ticket, Sections changed, Summary. The date and approver are filled in, whereas A1 keeps placeholders, which is fine. No em or en dashes in the file. No `architect`, `drone` or `Controller` introduced; the existing `Master Keeper`/`Controller` text (lines 22, 29, 62) is outside A2 and A1 already says older names persist outside amended sections.

## Blocking Issues

1. **Passthrough exception is stated only in 3.5 and the A2 summary; the Cell definitions that A2 itself rewrote are now false under it.**
   - Line 19 (2.1): "`waggled` refuses to start a Cell whose backend cannot enforce all of them", with "zero host credentials" as one of the targets. Under `cell.inference_credential: passthrough` the credentials target is reported not enforced and the Cell starts (D-120). 
   - Line 89 (4 item 3): "Every backend provides ... a zero-credential environment" and "no backend falls back" read as unconditional.
   - Line 78 (3.5) calls 2.1 "the refusal rule" and says the exception lives there, but 2.1 carries no exception.
   - Fix: add to line 19 a clause such as "except when the hive opts into `cell.inference_credential: passthrough` (3.5, D-120)", and a short "unless passthrough (3.5)" on line 89. Keep it narrow: opt-in only, the default is zero credentials, and `waggle doctor` reports it. The A2 summary and 3.5 wording are otherwise honest and narrow.

2. **Stale socket path in unchanged text contradicts the new 3.5.**
   - Lines 21 and 30 say `.waggle/run.sock`. A2's 3.5 (line 78) says `.waggle/control/run.sock`, which is correct (`internal/layout/layout.go:34`, `config.yaml` `socket_path`, `devcontainer.json`).
   - A2 introduces the correct path next to the wrong one in the same section list. Fix: change `.waggle/run.sock` to `.waggle/control/run.sock` on lines 21 and 30 and add 2.1 to "Sections changed" in A2 (it is already listed for Cell, so name the socket path in the Summary), or drop the path from line 78 and leave a bare `run.sock`.

## Non-blocking notes
- Lines 17 and 87 still say `waggle init` scaffolds `.devcontainer/`, and line 63 lists `.devcontainer/**` as Tier 3, while A2 says the container backend is configured by `.waggle/control/devcontainer.json`. Pre-existing, not introduced by A2 (D-030). Worth a one-line pointer if the Keeper wants it tidy.
- Line 19 does not mention macOS container backend limits (D-124). Fine for architecture text, since the probe covers it.

## Re-review (1571737)

Verdict: pass
Risk-Tier: 3

Diff reviewed: `git diff 2b7b745 origin/rfc-0003-a2 -- docs/RFC-0003.md` (commits `33d87c7`, `1571737`). `git diff origin/main...origin/rfc-0003-a2 --stat` touches only `docs/RFC-0003.md` (17 insertions, 10 deletions). The two-dot diff against `origin/main` lists hive files only because main has advanced to `9015bc9`; the branch base is `b1e4a3a`, so the merge does not touch them.

### My must-fix items
1. Passthrough exception: closed. 2.1 Cell now ends "except the `passthrough` opt-in of 3.5 (D-120)"; 4 item 3 says "a zero-credential environment (except the `passthrough` opt-in of 3.5, D-120)". 3.5 still calls 2.1's rule "the refusal rule", which now carries the exception.
2. Socket path: closed. Lines 21 and 30 now say `.waggle/control/run.sock`, matching 3.5, `internal/layout/layout.go` and config. A2 "Sections changed" lists both.

### Fable recommendations 1 to 5
1. Applied (above).
2. Applied: "`waggled` provisions the task's `git worktree` and every backend runs the Cell in it (`Spec.Worktree`)".
3. Applied: egress admits "the exact host names listed in `cell.egress_allow`", private-address refusal applies to allowlisted names, "a registry on a private address is unreachable in v0.1". Accurate, no contradiction.
4. Applied: socket path fixed; `.devcontainer/` replaced in 2.1 Hive and 4 item 1 by `.waggle/control/devcontainer.json` (D-030) and `.waggle/control/WAGGLE.md`. Both templates exist in `internal/scaffold/templates/`.
5. Applied: new 3.5 "Seatbelt Residual Surfaces" bullet (shared loopback and Mach namespace, same uid, deprecated `sandbox-exec`, container backend when it matters). No overclaim.

### Ruling: Tier 3 list `.devcontainer/**` (line 63) stays
Correct to leave. That sentence describes the hardcoded path floor, which is `internal/comb/backend.go:33` (`.devcontainer/`), covered in `tier_test.go`. It is path protection, not a statement of where Waggle scaffolds. `.waggle/control/devcontainer.json` is already covered by `.waggle/**`, and keeping `.devcontainer/**` still protects a hand-made root dir. Editing it would make the RFC disagree with the binary. It is also A1 text.

### Other checks
- No em or en dashes in the file.
- No `architect`, `drone` or new stale role names in amended text. `Controller` at lines 29 and 62 is pre-existing A1-era text outside A2.
- A2 summary and "Sections changed" match the edits. Status line says Amendments 1 and 2.
- `*(Amended: A2)*` markers are on lines 19, 54, 78, 79, 80, 90, all the right bullets.

### Non-blocking
- Four bullets were substantively edited but carry no `*(Amended: A2)*` marker: line 17 (Hive), line 21 (`waggled` socket path), line 30 (`queen` table row), line 88 (`waggle init` scaffold). They are named in the appendix "Sections changed", so provenance is recoverable. Recommend adding markers if the file is touched again; I would not block on it.

### Intent Packet (merge commit)
RFC-0003 Amendment 2 (T-024, PR #31, docs only, `docs/RFC-0003.md`). Replaces the "git worktree plus dev container" Cell with a `CellRuntime` interface: OS sandbox by default (Seatbelt on macOS, bubblewrap on Linux) and a Docker-compatible container backend for CI, configured by `.waggle/control/devcontainer.json`. The three targets stay (zero credentials, `ro` repo with `rw` leases, egress lockdown). `.git` is read-only so `waggled` commits on the host. Egress goes through a per-Cell host proxy limited to the inference endpoint and exact `cell.egress_allow` names, refusing IP literals and private addresses. Each backend probes per target and `waggled` refuses to start a Cell with an unmet target, except the opt-in `cell.inference_credential: passthrough` (D-120), which `waggle doctor` reports as not enforcing zero credentials. Also corrects `run.sock` to `.waggle/control/run.sock` and the init scaffold paths. Blast radius: spec text only, but it is the criterion T-024 builders and guards measure against. Two guards (sonnet, fable) reviewed; all blocking items closed.
