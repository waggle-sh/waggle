Risk-Tier: 3

Reviewed head: 8aef239738a1bc1f2e53c70e61ff926e071dd127 (PR #171, branch `hive-pin-9b5de54`, base origin/main 9b5de54eee5b08fc0df15d024be09e450679ee3d). Guard model: opus. Head Keeper commit, no builder run.

**Intent Summary:** One Head Keeper commit moves `waggle.pin` in `.waggle/control/config.yaml` from 0d102ca to 9b5de54 (current main head), rewrites the gate Action ref in `.github/workflows/waggle-gate.yml` to `waggle-sh/waggle/gate@9b5de54...` with the matching `# waggle.pin` comment, and appends D-340. It has the same shape as D-320 (PR #139), but smaller: the workflow change is one line. Blast radius: the pin decides which build `bin/waggle` is rebuilt from after the merge (bringing in T-044 claim branch renewal and the T-045 `--keeper-commit` guard path), and the gate Action ref is used only when a PR base lacks `cmd/waggle/gate.go`, which never happens in this hive. The required `verify-leases-and-tiers` check keeps building the gate from the base revision, so gate behavior on hive PRs does not change.

**Evidence**
- **Render reproduced byte for byte.** I built `cmd/waggle` from a fresh detached worktree at 9b5de54 (clean tree) in my own scratch dir. In a second detached worktree at 9b5de54 I ran `waggle upgrade --diff`. The dry run printed `[pin] 0d102ca... -> 9b5de54...`, `[rewrite]` for `.github/workflows/waggle-gate.yml` with a diff whose blob line (`index f132fc0..c4b6262`) and single hunk match the PR, `[same]` for roster.json, devcontainer.json, the five role templates and the `.gitignore` and `CLAUDE.md` blocks, and `[skip]` for the seeded files. After `waggle upgrade --apply`, only `.github/workflows/waggle-gate.yml` and `.waggle/control/config.yaml` changed. The hash of `git diff 9b5de54 -- .github .waggle/control` was identical (74018e8f...) for my apply and for the PR commit. The scratch worktrees are removed.
- **Gate Action resolvable.** `gate/action.yml` exists at 9b5de54 (blob b4e4f9e). 9b5de54 is `refs/heads/main` on origin, so `waggle-sh/waggle/gate@9b5de54...` resolves. Its inputs are exactly `base-sha`, `head-sha` and `token`, which matches the workflow's `with:` block. It builds with Go 1.23, GOTOOLCHAIN local and GOPROXY off, and go.mod at 9b5de54 says `go 1.23`. Between 0d102ca and 9b5de54 the only change to the file is the SPDX header line.
- **Nothing in 0d102ca..9b5de54 breaks the hive.** Of 207 changed files, most changes add SPDX headers (T-025). The substantive changes:
  - `internal/config`: headers only, so no config key is newly refused. The new build parsed the hive's config.yaml cleanly during upgrade.
  - CLI verbs: none removed. `dispatch` gains an opt-in `--keeper-commit` flag for guards (T-045). Claim renewal through `dispatch` now adds the run's branch to the claim, capped at 4 branches (T-044). Both changes are additive.
  - `cmd/waggle/gate_commits.go` drops the warn-and-accept fallback for a claim with no branch on a ticket-named branch, so that case now fails leases. The gate builds from base, so this rule has been live on hive PRs since #163 merged; the pin move does not change it.
  - `.waggle/control/scripts/scan-hive.sh` changes by one line (a header), and the role templates render `[same]`.
- **Workflow behavior for hive PRs.** Only the Action fallback line changes. Triggers, permissions, concurrency, the `present=true` path and the job id are unchanged. The PR's own run (40617986702) took the base-build path and reported `decisions: ok, appended D-340`. It failed only on the expected Tier 3 and "Head Keeper:" maintainer-commit rules that require an admin bypass, the same as the D-320 precedent. `dco` passes. `product-ci` was still pending when I checked.
- **D-340 append-only and accurate.** The DECISIONS.md diff has 0 removed lines, and D-340 follows D-339. A local `waggle gate decisions --base 9b5de54 --head 8aef239` from my build printed `decisions: ok, appended D-340`. The entry's pins, options, tickets and PR list match `git log --first-parent 0d102ca..9b5de54` (#141 T-042, #144 T-037, #150, #153, #155 and #157 T-025, #163 T-044, #165 T-045, #169 T-041). The text extends the `waggle upgrade` decision template the same way D-320 did, including the `[same]` plan label. The 2026-10-28 date is the UTC date at commit time, and the Keeper decided on 2026-10-27 local time, so the two dates agree.
- **Sign-off.** Author and committer are both `paulternate <27632860+paulternate@users.noreply.github.com>`, and the `Signed-off-by` trailer names the same identity. The PR range holds only this one commit.

**Non-blocking notes**
- After the merge, rebuild `bin/waggle` from 9b5de54, as D-340 says. Until then, the local binary lacks `--keeper-commit` and the T-044 branch renewal.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass
