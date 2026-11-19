<!-- SPDX-License-Identifier: Apache-2.0 -->
# Go-public readiness report

Task T-062. Written 2026-11-19 against `main` at `e0852e4` (merge of PR 206). The goal is to make the repo public and get outside users installing the gate. Nothing outside this file was changed.

## Verdict

**Not ready to flip.** There are 5 blockers, 7 gaps, and 4 items that were ready. The gate itself is in good shape: a published v0.1.0 release with six archives and checksums, an honest limits section, DCO and SPDX discipline. What is missing is the public wrapper: privacy cleanup of the history, a one-command install, a security policy, and the attribution decision.

Size key, in agent tasks: **S** is one small builder task (about 100 lines or less, one lease). **M** is one full task with a guard review. **L** is several tasks, or needs Paul's hands (GitHub settings, history rewrite, legal).

## What I could not check

- **`go build ./...`, `go vet ./...`, `go test -race ./...`, `gofmt -l .`, and the scratch-repo quickstart were not run.** The session denied Go and `gh release download` commands pending approval, so I have no local build or test result. The same four commands run in `.github/workflows/ci.yml`; see item 5.
- **CI history on main is unverified.** `gh run list` returned HTTP 403 (the token has no Actions read). Same token blocks `gh issue list` (403), so open issues are unknown. Open PRs were readable: #205 (T-060 staged gate modes RFC), #207 (orchestrator handoff), #209 (T-061 site deploy).
- **Branch protection and rulesets** are not readable with this token, so "the gate is a required check on main" is unverified.
- The repo is currently **private** (`gh api repos/waggle-sh/waggle`: `visibility: private`). Several findings below only bite once it is public, and several only matter because it is private today.

## Ordered checklist

### Blockers

- [ ] **B1. The tree still carries the private strategy doc and internal references.** `docs/rfc-notes/repositioning.md` is the private version (`.waggle/hive/tasks/T-037.md:163` calls it "l19 strategy, the kill criterion, competitor assessments"). A scrubbed copy exists at `docs/redaction/repositioning.public.md`. Hive files name a private design path (`~/code/l19-design` in `.waggle/hive/reviews/brief-T-041-fable.md` and `-opus.md`, per `docs/redaction/README.md:250`) and the Level 19 Vercel team and plan (`.waggle/hive/tasks/T-041.md:273, 286`). **Fix:** swap the public text over `repositioning.md`, scrub the named hive lines, and run `scan-hive.sh` plus the cookbook's dry run. **Size: M.**
- [ ] **B2. Git history contains private identities and the pre-scrub files.** `docs/redaction/README.md` is a dry-run-only cookbook. It says three private author emails exist in history (counts at its line 36 onward) and lists blob replacements. It has not been executed: `git log` shows 679 commits and the v0.1.0 tag unchanged. Flipping public publishes all of it, and deleting later does not unpublish it. A rewrite changes every SHA, which breaks: the `v0.1.0` tag and its release (the release workflow requires the tag commit to be an ancestor of `origin/main`, `.github/workflows/release.yml`), the pinned Action ref `waggle-sh/waggle/gate@3a9c054e...` in `.github/workflows/waggle-gate.yml`, `waggle.pin` in `.waggle/control/config.yaml`, and any `<commit-sha>` pins in docs. **Fix:** Paul decides between rewriting history and publishing a fresh squashed repo, then runs it on a mirror clone, re-pins, retags and re-releases (v0.1.1 or similar). I did not re-verify the cookbook's census; I only counted 5 author name lines (Paul Cleary, paulternate, paulternate19, GitHub, Cursor Agent) without printing emails. **Size: L** (needs Paul; two guards).
- [ ] **B3. Attribution is undecided and gets published with the flip.** Level 19 is named as copyright holder in `NOTICE`, `LICENSES/MIT-0.txt`, `README.md`, the site footer, and `VISION.md`. Paul wants Waggle attributed to him personally. See section 7 for the full list. If this is decided after launch, outside contributions arrive under the old notice. **Fix:** Paul decides holder, org and module path first (section 7); then one edit pass. Who legally holds the copyright today is not something I assessed. **Size: M** to edit once decided (S if the org and module path stay `waggle-sh`).
- [ ] **B4. There is no one-command install.** There is no goreleaser config, install script, Homebrew tap, or `go install` instruction in the repo. The README's Install section (lines 40 to 53) starts with "Build `waggle` from a clean clone of a pushed commit... outside the clone", which needs Go and a clean checkout, and only mentions release archives in passing. The v0.1.0 release is published and has all six archives plus `SHA256SUMS` (`gh release view v0.1.0`: not a draft, 2026-11-06), so the artifact exists; the docs do not point at it. While the repo is private, nobody outside can download it, and the `waggle-sh/waggle/gate@<sha>` Action ref that `init` writes cannot resolve for other repos. **Fix:** add a short install block to the README and `site/content/quick-start.md` (download, `sha256sum -c`, extract, move to PATH) or a small `install.sh` that does exactly that; document `waggle version`. Homebrew can wait. **Size: M** (S for docs only, M with a script and test).
- [ ] **B5. No security policy.** No `SECURITY.md` anywhere. This is a security-relevant tool whose README documents a forgeable required check ([L11](gate.md#10-limits-and-known-bypasses)); reporters need a private channel and a statement of what is in and out of scope. **Fix:** add `SECURITY.md` (point at GitHub private vulnerability reporting, reference L11 as known) and turn that setting on. **Size: S.**

### Gaps

- [ ] **G1. No code of conduct, issue templates or PR template.** None of `CODE_OF_CONDUCT.md`, `.github/ISSUE_TEMPLATE/`, `.github/pull_request_template.md` exist. The PR template matters here because the DCO check (`ci.yml`, job `dco`) fails PRs without `Signed-off-by`. **Fix:** Contributor Covenant plus a bug template, a feature template and a PR template that mentions `git commit -s`. **Size: S.**
- [ ] **G2. CONTRIBUTING.md is for the maintainer hive, not outsiders.** Line 4 says it is for "people building Waggle itself", but sections 1 to 5 are hive vocabulary (queen, scout, Tier 3, decision cards). The outsider essentials (build, test, DCO, how to open a PR, what the gate will do to your PR) are one paragraph at section 6. **Fix:** put a short "Contributing" lead on top (clone, `go build ./... && go test -race ./...`, `git commit -s`, no em dashes, SPDX header) and keep the hive vocabulary below. **Size: S.**
- [ ] **G3. The repo's own gate blocks outside contributors.** `waggle-gate.yml` runs on every PR. A contributor without push access cannot write `refs/waggle/claims/*`, so their commits cannot bind to a claim. The gate accepts a fork branch only as `<owner>/<repo>:<branch>` in a claim a maintainer wrote (`docs/gate.md` section 3), and the repo requires a `T-<id>:` subject or trailer. Fork PRs also need first-run approval for workflows. `.waggle/hive/ROADMAP.md:18` lists the relay for contributors without push access as v0.2 (T-055), and VISION item 3 names it. So at launch, every outside PR needs a maintainer to write a claim for it, or an admin merge. **Fix:** document the maintainer procedure in CONTRIBUTING (write a claim naming the fork branch, ask the contributor to use `T-<id>:` subjects) or exempt `Signed-off-by` DCO-only PRs through a documented label; decide before launch which. **Size: M** (docs S; a code path is a separate ticket).
- [ ] **G4. README does not say what is roadmap.** `grep` for Nectar, Quorum, honey, multi-Keeper, roadmap finds nothing in `README.md`, `docs/gate.md`, the recipes, the site content or the landing page; only `CONTRIBUTING.md` section 4 mentions them (5 hits). D-366 restores Nectar, Quorum and multi-Keeper as core and `VISION.md` says the gate ships first and the rest follows in v0.2, so a public reader sees a gate product with no sign of the larger plan. D-236 and D-287 keep plain vocabulary and a gate-led message, so this is a "Roadmap" section, not a rewrite. **Fix:** add a short "Roadmap" section to the README and the site, labelled planned, not shipped: Nectar and Quorum (v0.2), multi-Keeper hardening and relay for contributors without push access. **Size: S.**
- [ ] **G5. Bee language and glosses are thin.** README and site both follow D-236 plain terms (task, claim, lease, gate), so they are consistent with each other. But the few bee terms that do surface ("Head Keeper" at README lines 14 and 16, paths like `.waggle/hive/`) are only fully glossed in `CONTRIBUTING.md` section 4; README line 14 gives "Head Keeper" a parenthetical (the first entry of `gate.approvers`) but no plain definition, and nothing explains the name Waggle or what the hive path holds. **Fix:** one gloss line on first use of each bee term, linking the glossary. **Size: S.**
- [ ] **G6. README opens with a spec, not a way in.** The README is 70 lines but the first screen is the 5-item check list; the first-claim walkthrough exists only on the site (`site/content/quick-start.md`, lines 26 to 39), and the README does not link the site (which has no public URL yet: `site/astro.config.mjs` and `site/.env.example` say `SITE_URL` is undecided; PR #209 for deployment is open). The README Install and the site Quick start are near copies that will drift. **Fix:** put a 5-line "try it" block (install, `init --gate-only`, `claim`, open a PR, see the failure) above "What it checks"; link the site once it exists; make the README the only source (the sync script already builds the site page from it). **Size: S.**
- [ ] **G7. Hive tree ships with the product.** `.waggle/hive/` is 887 tracked files and 8.8 MB, including 7,100 lines of `DECISIONS.md`, review transcripts and probe scripts. `.claude/hooks/session-start.sh` and `.claude/settings.json` are tracked. This is consistent with "own honey", but outsiders searching the repo will hit it as noise in code search, and it multiplies the privacy surface (B1, B2). **Fix:** keep it (it is the dogfood evidence) but add a one-paragraph note in the README and `CONTRIBUTING.md` saying what it is; scan with `.waggle/control/scripts/scan-hive.sh` after B1. **Size: S.**

### Ready

- **R1. License.** `LICENSE` is the Apache-2.0 text, `LICENSES/MIT-0.txt` covers the scaffold templates, `NOTICE` exists, the README License section explains the split, and files carry SPDX headers (checked `ci.yml`, `release.yml`, `waggle-gate.yml`, `gate/action.yml`, `site/astro.config.mjs`, `index.astro`, `docs/gate.md`, `CONTRIBUTING.md`). GitHub detects `Apache-2.0` for the repo. The SPDX header check itself is planned, not running (`CONTRIBUTING.md` section 6).
- **R2. Release pipeline.** `release.yml` is thorough: tag must be annotated, on `main`, clean, VCS-stamped; six targets; reproducible archives; `SHA256SUMS`; opens a draft that the Head Keeper publishes by hand. v0.1.0 went out that way with all 6 archives. No signing or provenance attestation yet (nice to have, not a blocker).
- **R3. Secrets.** A pattern scan (AWS, GitHub tokens, `sk-`, Slack, private keys) found no real-looking secret outside fixtures. Matches are in `.waggle/control/scripts/scan-hive.sh` (the scanner's own patterns), probe and review files under `.waggle/hive/media/` and `.waggle/hive/reviews/`, and two forge tests (`internal/forge/github_transport_test.go`, `internal/forge/probe_test.go`). `docs/redaction/README.md:23` states the `ghp_` strings are short test fixtures; I did not open each one to confirm. No `.env`, `.pem`, `.key` or credentials files are tracked. Emails in the tree are `example.com`-style test values, `git@github.com` URLs, and the GitHub noreply forms; no private address appears outside the cookbook. Home paths in the tree are `/home/keeper`, `/Users/example`, `/home/u` style placeholders. I did not print any match values.
- **R4. Honest limits.** README and site both say what the gate does not do (forgeable check, admin bypass, no sandbox), and state pre-1.0 and GitHub-only. This is a strength for a public launch.

### Check results by item

| # | Item | Status | Evidence |
|---|------|--------|----------|
| 1 | Install a released binary in one command; gate in under 10 minutes | **blocker** (B4) | Release archives exist; README Install step 1 is a source build; no script, brew or `go install`; build/test and scratch quickstart not run (see above) |
| 2 | README clear, bee language with glosses, roadmap marked | **gap** (G4, G5, G6) | `README.md` lines 1 to 70; no Nectar or Quorum mention outside `CONTRIBUTING.md` |
| 3 | Community files | **blocker** (B5) and **gap** (G1, G2); license **ready** (R1) | No `SECURITY.md`, `CODE_OF_CONDUCT.md`, templates; `.github/` holds only `CODEOWNERS` and `workflows/` |
| 4 | Secrets, private emails, internal paths | **blocker** (B1, B2); secrets **ready** (R3) | `docs/rfc-notes/repositioning.md`; `docs/redaction/README.md`; hive briefs |
| 5 | CI health on main | **unverified** | See below |
| 6 | Gate works for contributors without push access | **gap** (G3) | `docs/gate.md` section 3; ROADMAP v0.2 |
| 7 | Attribution | **decision needed** (B3) | Section 7 below |
| 8 | Anything else | see section 8 | |

### 5. CI health on main

Unverified; the token could not read Actions. What the files say:

- `ci.yml` job `product-ci` runs `go build ./...`, `go vet ./...`, a tracked-files `gofmt -l` check, and `go test -race ./...` on every PR and push to `main`; job `dco` checks `Signed-off-by` against the commit author's email on every PR. It uses unpinned `actions/checkout@v5` and `actions/setup-go@v6` while the other workflows pin by SHA. Pin them for consistency.
- `waggle-gate.yml` pins its actions by SHA and builds the checker from the base revision (not the PR head).
- Recent history on main is a clean sequence of merged PRs (#204, #206 in the last five commits), which suggests the required checks pass, but that is inference, not evidence.

**Fix:** run `gh run list --branch main --limit 20` with a token that has Actions read, and confirm the ruleset requires `product-ci`, `dco` and `verify-leases-and-tiers`. **Size: S** (Paul, 5 minutes).

### 7. Attribution map (input for Paul's decision; nothing changed)

Copyright holder named, as text:

| Where | What it says |
|-------|--------------|
| `NOTICE:2` | `Copyright 2026 Level 19 Enterprises, LLC` |
| `LICENSES/MIT-0.txt:3` | `Copyright 2026 Level 19 Enterprises, LLC` |
| `README.md:70` | License section: `Copyright 2026 Level 19 Enterprises, LLC` |
| `site/src/pages/index.astro:107` | Landing page footer: `Copyright 2026 Level 19 Enterprises, LLC.` |
| `LICENSE` | Standard Apache-2.0 text only. The `[yyyy] [name of copyright owner]` hit is the appendix boilerplate, so no holder is named here. |
| Source files | SPDX license identifiers only (`SPDX-License-Identifier: ...`); no per-file copyright line. `Copyright` appears in only the files above plus `DECISIONS.md`, `T-006-backfill.md`, `T-025.md` (the licensing task). |

Level 19 named elsewhere (hive and records; ships with the repo unless scrubbed):

- `.waggle/hive/VISION.md:17` ("Level 19's own repos (own honey)"); also core to the "own honey" story in `CONTRIBUTING.md` section 4, which does not name Level 19.
- `.waggle/hive/DECISIONS.md` (18 hits), `tasks/T-006-backfill.md` (8), `T-025.md` (9), `T-037.md` (2), `T-041.md` (6), `T-042.md` (2), reviews `brief-T-037-opus.md`, `brief-T-041-fable.md` (5), `brief-T-041-opus.md` (6), `T-025a.md` (2), `media/T-025a/hashes.txt`.
- Test fixtures: `internal/decisions/testdata/backfill.golden` (8) and `backfill/D-046.json`, `D-059`, `D-102`, `D-128`, `D-129`. Changing these means regenerating the golden file.
- `docs/rfc-notes/repositioning.md`, `docs/redaction/repositioning.public.md`, `docs/redaction/README.md` (keeps the NOTICE line deliberately, and names an `@l19.ltd` email domain at line 107).
- `site/src/assets/brand/mark/waggle-lockup-outlined-mono.svg` and `site/package-lock.json` matched `l19` case-insensitively; most likely path data or a hash, not an attribution. Not opened.

Personal and org identity:

- GitHub org **`waggle-sh`** (repo `waggle-sh/waggle`). `gh api orgs/waggle-sh` returns no name, email or description, so I cannot tell who owns it. **Check who the org owners are.** The T-041 brief says the org's private repos deploy through the "l19 team" Vercel Pro plan (`.waggle/hive/tasks/T-041.md:273, 286`), so billing for the docs site is tied to Level 19 today.
- Go module path `github.com/waggle-sh/waggle` in `go.mod`; `waggle-sh/waggle` appears in about 123 files (every package import under `cmd/` and `internal/`, plus `.github/workflows/waggle-gate.yml`, the scaffold template, `internal/forge/remote.go`, `gate/action.yml` consumers, and hive docs). **Moving the repo to a personal account or another org means a module path change across all of those, a new pin, and a new Action ref.** It is cheap now and expensive after users pin it.
- `.github/CODEOWNERS`: `@paulternate` on templates, README, CODEOWNERS, workflows, LICENSE, NOTICE, LICENSES.
- Commit identity: `paulternate` and `Paul Cleary` on the GitHub noreply address, plus `paulternate19`, `GitHub`, and 3 `Cursor Agent` commits. DCO trailers read `Signed-off-by: Paul Cleary <...noreply...>` (the form this task uses). `scan-hive.sh:170` also contains this address.
- `.waggle/control/config.yaml`: Head Keeper and approvers are GitHub logins (not read in full here).

Decision points for Paul: (a) who is named in `NOTICE`, `MIT-0.txt`, README and footer, which also changes who may relicense; (b) whether the org stays `waggle-sh` (then S) or moves (then M, plus L for the pin and release); (c) whether the hive records that name Level 19 stay, as they describe the "own honey" story, or get scrubbed with B1 and B2.

### 8. Other things a real user would trip on

- [ ] **T1. Pinned Action and private repo.** `init` writes `uses: waggle-sh/waggle/gate@<sha>`. While the repo is private, other repos cannot use it (the Quick start already says "a pushed commit... so the Action ref exists on GitHub"). After B2's rewrite, any adopter's pin to an old SHA breaks. **Fix:** decide the rewrite before anyone pins; document pin-by-release-tag in the install block. **Size: S.**
- [ ] **T2. The install PR fails its own gate.** README Install step 3 says so, and an admin merge is needed. That is honest, but it is the first thing a new user hits on a repo they do not administer. **Fix:** make this the first caution in the "try it" block (G6); consider a `--first-run` note from `waggle init`. **Size: S.**
- [ ] **T3. `gate.protected_paths` and `go.mod` as a protected path.** The README lists `go.mod`, `go.sum`, `go.work` as always-protected control paths, so any Go user's dependency PR fails the gate. Worth one line in the quick start so Go users are not surprised. **Size: S.**
- [ ] **T4. Site is not live.** No domain, `SITE_URL` unset, deploy PR #209 open, docs site depends on Vercel through the Level 19 team (see section 7). **Fix:** land #209 after the hosting and attribution decisions; link the site from the README and the repo "About" field. **Size: S** after decisions.
- [ ] **T5. No GitHub repo metadata.** Topics, description and website field are not visible from here; set them at flip time. **Size: S** (Paul).
- [ ] **T6. Windows and macOS gate runs are untested here.** Release has `windows` and `darwin` archives, but `ci.yml` runs only `ubuntu-latest`, and the gate workflow also runs there. **Fix:** note "Linux runners" in the docs or add an OS matrix. **Size: S.**
- [ ] **T7. Pin the actions in `ci.yml` by SHA** to match the other two workflows (see item 5). **Size: S.**

## Suggested order

1. **Paul decides** (no agent work): attribution holder, org and module path (B3), rewrite vs fresh repo (B2), Vercel and domain owner (T4).
2. **Agent tasks, in parallel:** B5 + G1 (security, templates, CoC), G2 + G3 (CONTRIBUTING), B4 + G6 + G4 + G5 (README and install block), T7.
3. **Privacy pass:** B1 then B2 on a mirror, then re-pin, retag, re-release (L).
4. **Then flip,** after item 5 CI and branch rules are confirmed by someone with Actions read, and after a dry-run quickstart of the published archive on a clean machine (B4's acceptance check, which I could not run).

Rough total: about 13 S-sized items, 3 M items (B1, B4, G3) and 2 L items needing Paul (B2 and the B3 decision), plus GitHub settings changes only Paul can make.
