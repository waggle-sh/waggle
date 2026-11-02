<!-- waggle-review v1 verdict=must-fix head=fc60d1806a718da03b5f17c900004ae5ad3a974d -->

Risk-Tier: 2

**Intent Summary:** One commit (`fc60d18`) on `t-048` rewrites the gate's admin-bypass claims in `README.md`, `docs/gate.md` and the three recipe pages to state that the required check can be forged, names the two routes and the three partial protections, adds the approver re-run paragraph to section 6, and replaces doctor's closing line in `internal/forge/report.go` with two lines (no setting found missing; the check can still be forged, see L11), with the golden updated. Seven files changed, all inside the lease rule; no template, Action, exit code, finding or label changes. Blast radius: user-facing docs and one doctor output line; the hive sees the new doctor line at its next pin move. Build, vet, gofmt and `go test -race ./...` all pass on the head.

**What I verified**

- **Lease:** the seven changed files are exactly the files the lease rule allows. Nothing under `gate/`, `.github/`, `.waggle/`, `internal/scaffold/`, `internal/comb/` or `cmd/waggle/gate*.go` changed.
- **Program messages byte for byte:** `Tier 3 path; only an admin bypass merges it, approval does not` and `protected path (listed in .waggle/control/config.yaml); needs an admin bypass to merge` match `internal/comb/changecheck.go:96, 98`; the maintainer-commit message matches `cmd/waggle/gate_commits.go:268`. Each quote has `(limit: L11)` or an L11 link beside it (D-356).
- **Section 3 coverage (AC1, AC3, AC4):** every listed change is present. L11 names both routes, the three places and which test workflow shape each ran on, the three protections with "none is complete alone", the S12 copy exception, the untested Pull requests token, and "test workflows, not with the real gate". Section 6 states a review alone does not clear a failed run, names Write, says the real gate was not run, and uses "should" for dismissal and request for changes. `recipes/README.md:31`, `copilot-cloud-agent.md:25` and README line 14 link to section 6. No README heading changed.
- **Fact-check against the records:** the S12 paragraph, the required-review row, the token and bound rows, the green-to-red row and "merged with no admin flag" in the docs match `media/T-046/probe-log.md:50-68`; the 8 December 2025 date matches [CL] in `findings.md:31`. "The review runs the gate" matches the template's `pull_request_review` trigger (`waggle-gate.yml.tmpl:8-9`).
- **AC2 (`admin` search):** every hit in `README.md`, `docs/gate.md` and `docs/recipes/` is corrected and linked, or exempt as the brief lists (README line 27, `gate.md` L6, the three install-PR sentences). None presents a bypass as the only way to merge without a limit link.
- **AC6:** no en or em dash, ticket id, decision id, hive record path or bee word in any added or changed user-doc line. The only hits in the searched files are `T-123` examples, `Head Keeper: ` as the matched subject, the product paths under `.waggle/hive/`, and `docs/RFC-0003.md` and `docs/rfc-notes/`, which this ticket does not touch.
- **Threat word list:** no "prevents", "stops" or "guarantees" in changed lines. The "cannot" uses ("a workflow token cannot edit one", "a PR cannot change the copy") are the brief's own sentences and are supported by the log and [CL].
- **AC5 in part:** `report.go:107-108` prints the section 2 text; the two lines are 44 and 63 columns; `github_enforced.golden:14-15` follows; the `Enforcement: enforced` label, exit codes and findings are unchanged; `report_test.go:90` still holds because neither new line contains `Not enforced`.

**Blocking Issues (Must-Fix Only)**

1. **`cmd/waggle/doctor_test.go:210` was not updated (AC5, lease justification, section 6).** It still asserts that `Everything Waggle relies on is enforced` is absent, a string the program can no longer print, so the assertion is dead. The brief requires it to assert `found missing` is absent from `queueGolden`. The `cmd/waggle` lease exists only for this line. Coverage is not lost today because `out != queueGolden` is an exact match, but the brief's required change was skipped and the dead string misleads a reader into thinking the old line still exists. One-token fix.
2. **Cite ranges the brief told the builder to re-match were left stale (section 3.2, "Cites").** On `docs/gate.md:20`: `gate_verify.go:258-296` is now `264-298` (`baseTierPolicy`). On the edited line `docs/gate.md:114`: `probe_github.go:153-170` is `155-172`, `:172-189` is `174-191`, `doctor.go:25-28` is `27-30`, `:56-59` is `64-67`, `:76-81` is `84-89`, and `probe.go:36-41` is `38-43`. `doctor.go:32-34` is close enough. Fix these in the same round.

**Notes (not blocking)**

- The fix round touches a test assertion and cite numbers only, none of the section 7 must-fix sentences, so one confirming guard on the other model suffices (brief Risk Tier line, D-205).
- README's install step 5 adds "and the required check can still be forged (L11)" beyond the brief's "add required reviews"; it is consistent with L11 and within the brief's "light edits for flow" allowance.
- `gate.md:92` cite `probe_github.go:99-115, 138-143` is off by two at the tail but still points inside the two matching functions; fine to adjust with item 2 or leave.

**Evidence**

```
go build ./...      (clean)
go vet ./...        (clean)
gofmt -l .          (no output)
go test -race ./... ok for cmd/waggle, internal/comb, config, decisions, dispatch,
                    forge, license, pace, runtime, scaffold, usage
git status          nothing to commit, working tree clean
```

Verdict: must-fix
<!-- waggle-review v1 verdict=pass head=d9b7581896f1b987132da53b0b67b2ecd0e60fd0 -->

Risk-Tier: 2

**Intent Summary:** T-048 rewrites what the gate's user docs claim about admin bypass, the required check and approvals, in light of the T-046 probe: the required check can be forged by a workflow on any branch (delete the failing run, post a passing check) or by a token that can delete runs and write commit statuses, and an approval alone does not clear a failed run. It adds L11's full account and the three partial protections to `docs/gate.md`, the README, and the recipes; links every remaining admin-bypass claim to the limit; and replaces `waggle doctor`'s false closing line "Everything Waggle relies on is enforced on this repo." with two lines that say no required setting was found missing and the check can still be forged. One golden and one test assertion follow. Blast radius: user-facing text and one doctor output line; no exit code, finding, label, template, Action or program message changes. Eight files changed, all inside the lease rule.

**Verification**

| Check | Result |
|---|---|
| `go build ./...`, `go vet ./...`, `gofmt -l .` | clean |
| `go test -race ./...` | all packages ok |
| Files changed | exactly the 8 the lease rule allows; no `gate/**`, `.github/**`, `.waggle/**`, `internal/scaffold/**`, `internal/comb/**` or `cmd/waggle/gate*.go` |
| Commits | both `T-048: ` prefixed and signed off |

- **AC1:** every section 3 edit is present; README lines 14, 16, new limits bullet, 27, 33, 35 only; no heading renamed.
- **AC2:** every `admin` hit in `README.md`, `docs/gate.md`, `docs/recipes/` is linked (L11, `[limits]`, `(limit: L11)`), inside L11 itself, or exempt per section 3 (README 27 and 34, `gate.md` 74, 83 and 112, `recipes/README.md` 16). The three program messages at `gate.md:27`, `:49`, `:50` match `gate_commits.go:268` and `changecheck.go:96, 98` byte for byte.
- **AC3:** L11 names both routes, the own-PR, other-PR and plain-branch places with their shape, "none is complete alone", the S12 copy exception, the untested Pull requests token and "test workflows, not with the real gate". Each sentence I checked against `media/T-046/probe-log.md` is supported: "a workflow token cannot edit one" is P4b run 3's PATCH 403; "one such PR was merged" is #11; the token, bound and required-review rows match #16, #18, #19 and #15; the 8 December date is the findings' [CL] changelog.
- **AC4:** section 6 says an approval alone does not turn the check green, names Write, says the real gate was not run, words dismissal and request for changes as "should"; `recipes/README.md:31`, `copilot-cloud-agent.md:25` and README line 14 link to it.
- **AC5:** `report.go:107-108` prints the section 2 text; lines are 45 and 63 columns and the golden loop at `report_test.go:61-63` checks them; `Enforcement: enforced` and the `Not enforced` branch are untouched. `doctor_test.go:210` asserts `found missing` absent on the native-queue path, which has an unknown finding, so the assertion is meaningful.
- **AC6:** no en or em dash in any of the five docs. Word search hits are all section 7 non-hits: `.waggle/hive/` product paths, `internal/comb/` code paths, the `Head Keeper: ` subject and `T-123` examples.
- **Cites:** `gate_verify.go:260-298`, `probe_github.go:97-118, 140-144, 155-172, 174-191`, `probe.go:38-42`, `doctor.go:27-30, 64-67, 84-89` all match the code at `d9b7581`. The `enforced` sentence matches `Report.Enforcement` at `report.go:52-73`.

**Notes (not must-fix)**

- `gate.md:116` says "With no finding missing it closes with" the new line. An `unknown` finding also suppresses it (`NotEnforced` includes unknown, `report.go:42-49`). Precision nit for a polish pass.
- Cites this ticket was not asked to touch have drifted: `changecheck.go:93-94` and `95-96` are now 95-96 and 97-98; `gate_commits.go:265-268` is now 267-269; `gate_verify.go:225-229` is now 227-230. Polish ticket material.
- L11's "a classic token with `repo` already reaches Actions and commit statuses" rests on GitHub's scope documentation, not a probe row (the token used also held `workflow`). The brief required the sentence; recording the basis here.

**Blocking Issues (Must-Fix Only):** none.

Verdict: pass