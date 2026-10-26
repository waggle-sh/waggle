# Brief review: T-041 round 1 (opus), target e6ee2d0

Verdict: must-fix

Risk-Tier: 3 (by content, as the brief says; Tier 0 by path, verified)

Base: origin/main 32adb18. Read: `.waggle/hive/tasks/T-041.md` at e6ee2d0; DECISIONS.md D-129, D-185, D-210, D-236, D-261, D-287 to D-290; T-037 at 58e8c32; the studio at commit b184b19 (`git -C ~/code/l19-design show b184b19:...`). The studio working tree had uncommitted edits to the hero, hive-flow and brand.md while I reviewed, so every studio finding below is against b184b19, the commit the brief names.

**Intent summary.** T-041 adds a static Astro docs site in a new top-level `site/`, rendered at build time from the five markdown files T-037 writes plus the CONTRIBUTING glossary, styled from studio assets copied byte for byte, deployed by Vercel on the free subdomain. The builder writes about 641 lines it cannot build (no npm in its Bash forms); the queen writes the asset commit, the lockfile and the host build. Blast radius: the repo's first npm tree (code that runs on the Head Keeper's host and on Vercel), the public statement of what the gate enforces, and a stub `site/go.mod` that keeps `site/node_modules/` out of `./...`. Nothing under `cmd/`, `internal/`, `.github/` or `.waggle/` changes.

## Must-fix

### 1. `check-dist.mjs` as specified fails every page, so the build can never pass

Section 7 gives the script three rules that the files it must ship break:

- "every inlined SVG has `role="img"` and a `<title>`". The nav inlines `waggle-lockup-outlined-mono.svg` on every page, and at b184b19 it is `<svg ... role="img" aria-label="Waggle">` with no `<title>` (`grep -c '<title'` is 0). 2.5 forbids editing a copied SVG.
- "no URL to a host other than `github.com` and the site itself". Every inlined SVG carries `xmlns="http://www.w3.org/2000/svg"`. T-037 AC5 (58e8c32) also allows outbound links to section 4 sources, and its brief cites `code.claude.com`, `docs.github.com` and `github.blog`, plus `developercertificate.org`. The recipe pages will carry those links.
- "no T-037 AC3 bee word outside `<code>`" on the home page. At b184b19 the hero carries `internal/comb/` three times and `T-041` twice in SVG `<text>` elements, which are not `<code>`. AC3's regex includes `\bcomb\b` and `\bT-0[0-9]{2}\b`, and finalize item 2 copies it verbatim. If the script scans raw HTML rather than text nodes, every rewritten GitHub `href` under `internal/comb/` hits too.

**Fix.** The SVG rule becomes "`role="img"` and an accessible name (`aria-label`, or `aria-labelledby` resolving to a `<title>`)". The host rule covers resource loads only: `src`, `srcset`, `<link href>`, CSS `url()` and `@import`, plus form `action`. `<a href>` to any `https:` host passes, and `xmlns` is ignored. The bee-word rule scans text nodes only and treats SVG `<text class="mono">` as code (or exempts inlined diagram text, which the studio reviews). Q-T041-8's studio fix also renames the hero's task to `T-123`, T-037's example form (the studio's uncommitted `gate-hero.mjs` already does this).

### 2. The builder cannot build, the brief supplies no framework facts, and the loop after a failed host build is undefined

- Header, Bash forms: "every package version, Node version and framework fact comes from this brief, never from memory". The brief gives no Astro API facts: no config keys for smartypants, syntax highlighting or remark/rehype plugins, nothing on the `?raw` import, nothing on the `getStaticPaths` shape. The builder must write about 641 lines from memory with no type check or build, so the sentence is false as written.
- 2.3 rule 1 gives the repo root as `new URL('../../..', import.meta.url)` from `sources.ts`. In Astro's static build, `sources.ts` is bundled into a server chunk under the build output, so `import.meta.url` names the chunk, not `src/lib/`. That path resolves to the wrong directory and every manifest read fails. It fails loud, but it costs a round the builder cannot see coming.
- Section 7 runs the queen's host build after step 3, but nothing says what happens when it fails. Is it a builder fix round, does it count toward the target (D-210 assigns only post-review rounds to the stop, so a pre-review host fix could trigger the planned cut for reasons unrelated to scope), and how many rounds run before a card?
- ACs 2, 4, 5, 6, 8 and 10 rest on the queen's host run. Its logs and screenshots go "in the records batch", which lands after merge, so the guards never see the evidence they are asked to judge.
- No step shows that any `check-dist.mjs` rule can fail. AC4 and AC5 test one missing-source case and one missing-link case, not the script.

**Fix.** The finalize round adds a short facts list that the queen verifies on the host against the pinned Astro: the config keys, the plugin hook, the raw import, `getStaticPaths`, and the root resolution. For the root, use `process.cwd()` (Vercel and `npm run build` both run in `site/`) asserted by `../README.md` existing, or a path computed in `astro.config.mjs`. Replace the "never from memory" sentence with "from this brief's facts list". State the loop: a failed host build goes back to the builder as a pre-review fix round with the log in the prompt, and say how those lines count. The queen hands the host logs and screenshots to both guards at dispatch, and the records batch archives them afterward. The queen plants one violation per `check-dist.mjs` rule in a scratch copy and records each exit 1.

### 3. Threat model item 1 is false for the host run the brief prescribes

5.1 says "a hostile package can deface the site but reaches no credential". That holds only for Vercel. Section 7 has the queen run `npm ci --ignore-scripts` and `npm run build` on the Head Keeper's host. `--ignore-scripts` blocks install hooks only. `astro build` then executes Astro, Vite, Rollup, esbuild and every transitive module with the host user's rights, on the machine that holds the deploy credentials (the 1Password service account token at `~/.config/claude/op-token`, and the authenticated `gh`, `vercel` and `supabase` CLIs). On Vercel, the build checkout holds the whole private repo with network egress, including the hive records the D-235 redaction has not touched yet, so a hostile build can exfiltrate unredacted source before the flip.

**Fix.** Run the host npm steps in a throwaway container or VM that mounts only a copy of the repo, with no home directory or credentials. Network is on for `npm ci` and off for `npm run build`. State the Vercel residual (source readable by build code before the flip) in 5.1 and on the Q-T041-4 card. The brief already treats this as a Tier 3 input, and its own mitigation list has to cover the host it names.

### 4. "check-dist runs on every deploy" rests on dashboard settings no review sees

2.7 says `check-dist.mjs` runs on every deploy because Vercel runs `npm ci --ignore-scripts` then `npm run build`. Those commands are typed into the dashboard by hand (section 10 item 2). A later dashboard edit, or a framework-preset default, drops the check or `--ignore-scripts` silently, and nothing in the repo records it. Separately, Q-T041-5 (a) lands only after merge, which leaves a window where a later `site` ticket can change dependencies at Tier 0 by path.

**Fix.** The builder's `site/vercel.json` sets `framework`, `installCommand: "npm ci --ignore-scripts"`, `buildCommand: "npm run build"`, `outputDirectory: "dist"` and `ignoreCommand`; `vercel.json` overrides the dashboard. Section 10 item 2 then only confirms the dashboard leaves them alone. Add `site/vercel.json` to Q-T041-5 (a)'s list, since it controls what runs at install. Move the `gate.protected_paths` edit into precondition 1's Head Keeper control commit, so the gate enforces the Tier 3 call on T-041's own PR and no unprotected window opens. With no `gate.approvers` set, T-041 then merges by admin bypass, which a Tier 3 merge needs anyway.

### 5. Q-T041-8 is right but incomplete: the hero makes two more false claims

Confirmed against D-261 and the code. At b184b19, `e19f60` changes `.github/workflows/waggle-gate.yml` and the hero shows it passing once dana, a listed approver, approves. `approvablePath` (`cmd/waggle/gate_approve.go:87-104`) returns false for every `HardcodedTier3Prefixes` path, so that commit stays must-fix whatever approval it gets. Option (a) is sound: for a repo-listed path the approver filter also drops the outside-lease finding (`gate_commits.go`, the `filter` field), so a `deploy/` path outside the lease passing on approval is true.

The question misses two claims:

- The label reads "dana approves at e19f60", but the PR's head is `c3d81f` (the fourth commit, y=384). The gate passes only an approval at the exact head (`cmd/waggle/gate.go` usage, "approving the exact head"), and the hero's own `<desc>` says "at its head commit".
- The example task is `T-041`, which breaks T-037's example form (`T-<id>` or `T-123`) and must-fix 1's bee-word scan.

**Fix.** Q-T041-8 (a) lists all three changes (path, head label, task ID) in both cuts and their `<desc>`. The queen checks each one at the recorded studio commit before the asset PR. The studio's uncommitted `gate-hero.mjs` already holds `deploy/production.tf`, `T-123` and "approves at c3d81f", so this is a check on a fix already in progress.

## Notes

1. **Tier.** Verified: `HardcodedTier3Prefixes` (`internal/comb/backend.go:29-39`) holds no npm file, and `matchesPrefix` (`:56-64`) matches `site/go.mod` against neither `go.mod` nor `go.work`. "Tier 3 by content plus protected_paths later" is a review rule until must-fix 4 lands. With it, the call is enough. Still say who merges: the PR body carries `Waggle-Risk-Tier: 3` and the merge is a Head Keeper card.
2. **`site/go.mod` stub, verified on a scratch copy** (`git archive 32adb18` under the scratchpad, Go 1.27.2). I planted `site/node_modules/evil/pkg/x.go` (import `example.invalid/nope`) and an unformatted `y.go`. Without the stub, `go build ./...` and `go vet ./...` both exit 1, and `go list ./...` lists `.../site/node_modules/evil/pkg`. With the two-line stub (`//` comment plus `module waggle.invalid/site`), build and vet exit 0, and `go list ./...` is identical to the clean tree (10 packages). `gofmt -l .` still lists `site/node_modules/evil/pkg/y.go`, as the brief states. product-ci's tracked-files gofmt is unaffected. `go test -race ./...` sees the same package list. In my full run, forge, runtime, scaffold and usage passed, and internal/dispatch hit Go's 10-minute default timeout. Rerun with `-timeout 30m`, internal/dispatch passed with the stub and `node_modules` present (796 s) and on a clean copy (788 s), so the timeout came from this host's speed, not the stub. The product-ci gofmt step formats tracked files only, so the stub does not change it. Have `site/.gitignore` list `node_modules/`, `dist/` and `.astro/`.
3. **Route table.** The no-glob manifest is the right control. Gaps:
   - Rule 5 maps "a relative link to a manifest file" to its route, so a README link to `CONTRIBUTING.md` (contributor rules) would land on the glossary-only page. Map `CONTRIBUTING.md` to GitHub unless the fragment is in the glossary section.
   - Resolve relative links against the source file's directory.
   - "Tracked file" should read "exists in the build checkout": Vercel gives no `.git` guarantee, and the host build passes an untracked file.
   - Fragments are kept but never checked; optional.
4. **`check-dist.mjs` scope.** It compares the page set only, so a stray file under `site/public/` ships unchecked. Compare the whole `dist/` file set: pages, `_astro/*.css`, the two fonts, both OFL texts, the favicon and the touch icon. It is a drift and accident check, not a security boundary, since a PR can edit it. The PR body should say so.
5. **CSP.** It suits a no-JS static site. `'unsafe-inline'` for styles is needed for the SVG `<style>` blocks and Astro's inlined stylesheets. `default-src 'none'` plus the explicit directives would be tighter. The headers are never checked live, because a local static server ignores `vercel.json`. Add a `curl -sI` header check on the first protected preview (with a protection bypass) or at the flip. Vercel's preview toolbar will be blocked by `script-src 'none'`, which is expected.
6. **JavaScript off and accessibility.** The JS-off claims hold by construction. `display: none` does drop the hidden cut from the accessibility tree. Verified at b184b19:
   - The four diagram files' only ids are their title and desc pairs (`wg-gate-t`/`-d`, `wg-gate-n-t`/`-d`, `wg-hive-t`/`-d`, `wg-hive-n-t`/`-d`), all unique.
   - They hold no `url(#...)` references.
   - Their `<style>` only sets `--wg-*` and classes under `.wg`, so inlining both cuts on one page leaks nothing.
7. **License.** Option (a) is sound: Level 19 holds the copyright in both repos (`NOTICE`), and Apache-2.0 section 6 leaves the mark alone. The studio holds no font files (no woff2 under `products/waggle/`; brand.md says the sizes were measured from Google Fonts), so the "Source (studio)" column is wrong for the fonts and the OFL texts. Name the fetch (the Google Fonts CSS2 URL with axes, subset and user agent, or the upstream release) and record the URL and sha256 in `SOURCE.md`. A Google-subset woff2 is a Modified Version under the OFL, so the queen should confirm neither `OFL.txt` declares a Reserved Font Name. The copied files hold no U+2013 or U+2014 (checked), so AC11 holds.
8. **D-289 and Q-T041-3.** "Under docs/" appears only in D-289's option text. The Keeper choice says "in this repo", so `site/` contradicts no ruling. The Q-T041-3 card is still the right record, and precondition 1 (the WAGGLE.md section 2 entry) covers the lease.
9. **Vercel and exposure.** From memory and unverified here: Hobby may not deploy private repos owned by an organization, and protecting production may need Pro, at about $20 per member per month. The queen check is right, but the brief should say a plan upgrade is a money card for the Head Keeper, never an automatic step. There is a free variant of (a): previews under Standard Protection, and production off until the flip with `git.deploymentEnabled: {"main": false}` in `vercel.json`. The ignored-build command `git diff --quiet HEAD^ HEAD` sees only the last commit of a multi-commit push; diff against `$VERCEL_GIT_PREVIOUS_SHA` with a fallback to build. Installing the Vercel GitHub App on this repo only is right (5.5).
10. **Supply chain.** Exact pins cover only the direct dependency. Generate the lockfile with `npm install --package-lock-only --ignore-scripts --before=<two weeks back>` so no transitive version is days old.
11. **Budget.** The arithmetic checks: 79+60+55+100+117+160+70 = 641; doubled 1,282; target 1,667; stop 2,308, per D-185 and WAGGLE.md section 4. One phase is reasonable. The cut (hive page, glossary include, footer link, 404, about 70 bottom-up or about 140 doubled) leaves a standalone site. See must-fix 2 on how host-build fix rounds count.
12. **Asset PR.** Name its branch (for example `t-041-assets`, which D-237's suffix rule binds) and list it on the step 1 claim, so dispatch's `t-041` starts fresh.
13. **Builder grep.** `#[0-9a-fA-F]{3,8}\b` also matches fragments such as `#add-the-gate...`. Tell the builder to report and explain such hits rather than reword links.

Counts: 5 must-fix, 13 notes.

# Brief review: T-041 round 2 (opus, confirming), target 9709cd9

Verdict: must-fix

Risk-Tier: 3 (by content; Tier 0 by path, unchanged from round 1)

Base: main 32adb18. Read: `.waggle/hive/tasks/T-041.md` at 9709cd9 and `git diff e6ee2d0 9709cd9` (one file, +103 -71); both round 1 reviews; `internal/config/config.go`, `internal/comb/backend.go`, `internal/comb/changecheck.go`, `internal/dispatch/task.go` and `run.go`, `cmd/waggle/gate_commits.go:205-235`, `gate_approve.go:87-104` and `:205-225`, `gate.go:70-78`; DECISIONS.md D-204, D-237, D-238, D-243; T-036 section 2.11; the studio working tree under `~/code/l19-design/products/waggle/` (uncommitted, on top of b184b19); Vercel docs fetched 2026-10-25 (project-configuration, vercel-json, git-configuration, plans/hobby, deployment-protection).

**Intent summary.** Round 2 closes the round 1 must-fixes in text: check-dist's rules are rescoped (text nodes, path tokens, resource loads, accessible names), the root comes from `process.cwd()`, Astro facts become a host-verified finalize list, a capped pre-review fix loop is written down, host npm steps move into a credential-free container, `site/vercel.json` pins the install, build, output, framework and ignore commands and turns production off, the hero's three false claims and the agent line are listed for the studio fix, and the T-025 phase C order and SPDX rows are written out. The budget grows to 706 bottom-up with two cut tiers. Blast radius is unchanged: a new `site/` npm tree, the public statement of the gate, and (new in round 2) a control commit that lists three `site/` paths in `gate.protected_paths` before dispatch. That last change blocks the ticket itself (must-fix 1).

## Must-fix

### 1. Precondition 1 makes T-041 unclaimable: no lease can cover a directory that contains a protected path

The round 2 text (Depends on, precondition 1; Risk Tier line; 2.7 "The gate"; Q-T041-5 (a); section 10 item 1) lists `site/package.json`, `site/package-lock.json` and `site/vercel.json` in `gate.protected_paths` before dispatch. I recommended this in round 1 (my must-fix 4); it is wrong, and the code shows why.

- `internal/dispatch/task.go:83-89` (`waggle claim` and dispatch's claim step) and `internal/dispatch/run.go:310-311` (every non-guard run) call `refuseProtected(basePolicy(...), req.Dirs)`.
- `refuseProtected` (`task.go:131-138`) fails with `comb.ErrProtectedPath` ("protected path listed in .waggle/control/config.yaml; leases never cover it", `backend.go:152-154`) when `p.CoversDir(d)`.
- `CoversDir` is `touchesPrefixes` (`changecheck.go:74-76`, `backend.go:390-400`), which is true when the directory contains an entry: `DirsOverlap("site", "site/package.json")` is true (`backend.go:383-385`, `strings.HasPrefix(b, a+"/")`).
- D-243: the rule applies at claim, renewal and takeover. T-036 2.11 item 2 is the precedent: `gate/` was listed only after phase A merged, "From then on no lease can cover `gate/`".

So once precondition 1 lands, the queen's step 1 claim on `site` (branch `t-041-assets`) and dispatch's claim on `site` both exit with `protected_path`. Claiming first does not help: a renewal during the pre-review loop is refused, and dispatch's post-run check reads the policy from base (`run.go:310-313`, `check.go:273` via `CheckChangesPolicy`), so the builder's `site/package.json` and `site/vercel.json` come back as `tier3` violations.

The syntax itself is fine: `gate/,site/package.json,site/package-lock.json,site/vercel.json` is one scalar that `config.go:199-205` splits on commas and trims, and each entry passes `validProtectedPath` (`config.go:231-249`: no wildcard, no leading `/`, clean, under 256 bytes; 4 of at most 64 entries), per D-238.

**Fix.** Return to the T-036 pattern. Precondition 1 keeps only the WAGGLE.md section 2 `site/` entry (and the CODEOWNERS lines if wanted). The `gate.protected_paths` edit becomes a Head Keeper control commit right after T-041 merges, made while no `site` claim is live (D-243 does not re-check stored claims), before any later `site` claim. T-041's own PR stays Tier 3 by review rule: it carries `Waggle-Risk-Tier: 3`, gets two guards, and merges by admin bypass on a Head Keeper card, as the brief already says. Rewrite Q-T041-5 (a) as "after merge, same session, before any other `site` claim" and drop the "no Tier 0 window" and "this PR ... needs a listed approver at the head or an admin merge" claims (Risk Tier line, 2.7, Q-T041-5). Add one sentence on what listing costs: after it, no lease can be `site`. A later site ticket leases `site/src` or `site/scripts`, and the root files of `site/` (`astro.config.mjs`, `tsconfig.json`, `.gitignore`, `go.mod`) then change only by a Head Keeper commit or an admin-merged PR. That includes section 10 item 2's follow-up that writes `site` into `astro.config.mjs`, so name its route. If the Head Keeper prefers the small dependency files leasable later, (d) is what the brief should recommend, with this cost stated.

### 2. Q-T041-9's money card prices the wrong thing and leaves out Hobby's commercial-use limit

Vercel's docs, fetched today, contradict the card's two premises:

- "protecting production (Vercel Authentication on All Deployments) may need Pro". `/docs/deployment-protection`: "All Deployments ... Protecting all deployments is available on all plans", and "Vercel Authentication for All Deployments and Deployment Protection Exceptions do not require a paid add-on". `/docs/plans/hobby` lists Vercel Authentication "for preview and production deployments" on Hobby. Only Password Protection costs money ($20 per month per protected project, Pro only).
- The card never says what `/docs/plans/hobby` says: "the Hobby plan restricts users to non-commercial, personal use only". Waggle is a Level 19 Enterprises product, so "(a) $0 until the flip" on a Hobby team is a terms problem, not a free option. On Hobby, the honest cost of any Vercel option is Pro, at "$20 per user / month" for developer seats (viewer seats free).

So option (b) as written ("Pro from merge, to protect production too") buys nothing Hobby lacks, and the recommendation hides the one real cost driver. Q-T041-4 (a) repeats the first error ("Standard Protection (the free tier ...)").

**Fix.** Rewrite Q-T041-9 from the plan check outward. If the l19 team is already on Pro, every option costs $0 more, and the card is only (a) versus (c). If it is on Hobby, the card says Hobby is non-commercial only, so Vercel for this site means Pro at $20 per developer seat per month from the day the project is created, or (c) at $0 until the flip. Drop "protecting production may need Pro". In Q-T041-4 (a), prefer Vercel Authentication at All Deployments (free on every plan) over Standard Protection, keeping `deploymentEnabled` `{"main": false}` as the second layer. Keep the round 1 line that no plan change is ever an automatic step. The private-org-repo question stays a queen check, as written.

## Round 1 must-fixes: closure

| Round 1 item | Closed? | Where |
| :--- | :--- | :--- |
| opus M1, check-dist fails every page | yes | section 7 rules 4, 5, 6; Q-T041-8 `T-123` |
| opus M2, no facts, `import.meta.url`, no loop, evidence after merge, no failing case | yes | Bash forms line; 2.3 rule 1; section 12 Astro facts (finalize, marker barred at dispatch by 2.2); section 7 loop and planted violations; AC13 |
| opus M3, threat model false for the host | yes | 5.1 container paragraph; Vercel residual in 5.1 and on Q-T041-4 |
| opus M4, Vercel settings off-repo, protected-path window | first half yes (`site/vercel.json`, verified below); second half closed by a change that breaks claims (must-fix 1) | 2.2, 2.7, AC3, precondition 1 |
| opus M5, two more hero claims | yes | Q-T041-8 (path, head label, task ID, agent line, `#1042`) |
| fable M1, external-host rule | yes | section 7 rule 5; AC8 |
| fable M2, `T-041` in the hero, ID pattern over SVG text | yes | Q-T041-8; section 7 rule 4 runs the ID patterns over all text, SVG `<text>`, `<title>` and `<desc>` included; AC7 |
| fable M3, T-025 phase C order | yes | Depends on; 2.8 rows; section 10 item 4 |
| fable M4, pre-review loop | yes | section 7 "The pre-review fix loop"; section 9 |

Eight of nine closed cleanly. The ninth (opus M4) is closed as asked, but the fix I asked for is must-fix 1.

## The confirm points

1. **check-dist rules against the studio's files and the recipe links: satisfiable.** Checked against the studio working tree (the hero fix, uncommitted):
   - Rule 6. All four diagrams carry `role="img"` and `aria-labelledby="<title id> <desc id>"`, and the lockup carries `role="img" aria-label="Waggle"`, so each has an accessible name.
   - Rule 5. No `url(`, `@import`, `src=` or `href=` appears in any of the five inlined SVGs. Only `xmlns`, which the rule exempts. `<a href>` to `code.claude.com`, `docs.github.com` and the like passes.
   - Rule 4. The ID patterns find nothing: the copies hold `T-123` and `ticket-123` only, and `T-123` does not match `\bT-0[0-9]{2}\b`. With `<style>` stripped and path tokens dropped, the AC3 word list has zero hits in either hero cut. The hive diagrams hold bee words, but they appear only on the hive page, which the rule exempts.
   - Rule 3. No U+2013 or U+2014 in `tokens.css`, the four diagrams, the lockup or the favicon.
   - The hero now reads "approves at c3d81f", "From any agent, or a person with a branch." (wide) and `#1042`, as Q-T041-8 (a) says.
   - Two wording gaps are notes 1 and 2.
2. **Pre-review loop: well defined and capped.** The trigger is a failed host build or check-dist; the builder round carries the full log; the lines count toward the target and the cut, not the D-210 stop; after two failing rounds, a D-204 design card; guards start only from a passing branch; the evidence goes to the guards at dispatch. One gap is note 3.
3. **Container plan: concrete enough to execute.** It mounts only a branch copy, with no home directory, `~/.config`, SSH agent or Docker socket. Network is on for `npm install` and `npm ci` and `--network none` for `npm run build`. `dist/` is served from the host as static files. No container runtime means a card, and the runtime is recorded at finalize (item 6). Gaps are note 4.
4. **`vercel.json` overrides the dashboard: confirmed.** `/docs/project-configuration/vercel-json` says `buildCommand` "can be used to override the Build Command in the Project Settings dashboard"; `framework`, `installCommand` and `outputDirectory` likewise; `ignoreCommand` "This value overrides the Ignored Build Step in Project Settings ... When the command exits with code 1, the build will continue. When the command exits with 0, the build is ignored." The file goes "in your project's root directory", which is `site/` under a Root Directory of `site`. `git.deploymentEnabled` takes a branch-to-boolean object (`/docs/project-configuration/git-configuration`), so `{"main": false}` is valid syntax. The docs name exit 1, not "any nonzero", so the queen check in section 12 (a missing SHA gives git's 128) stays.
5. **`protected_paths` syntax: valid** (must-fix 1 has the evidence). The problem is ordering, not syntax.
6. **Budget arithmetic: correct.**
   - Config 20+5+25+2+5+2+30 = 89, components 60+20+20 = 100, pages 35+25+45+12 = 117, and 89+65+65+100+117+160+110 = 706.
   - Doubled 1,412; target 1,835.6, rounded to 1,836; stop 2,541.6, rounded to 2,542.
   - Tier 1 at about 75 bottom-up (150 doubled) is plausible: the hive page 45 plus the 404 page 12 plus the include, footer link and mapping.
   - Tier 2 is about 40 (80 doubled). Both together are 230, 12.5 percent of the target ("13 percent" rounds it).
   - Rules 1, 2 and 5 survive tier 2, which keeps the security rules.
   - See note 5 for the references tier 2 leaves dangling.
7. **Q-T041-9 money framing: wrong.** See must-fix 2.

## Notes

1. **Rule 6 wording.** The diagrams' `aria-labelledby` lists two ids, and the second resolves to `<desc>`. "aria-labelledby resolving to a `<title>`" read strictly as "every id is a `<title>`" fails all four. Say "every listed id resolves inside the SVG, and one of them is its `<title>`".
2. **Rule 5 and non-https anchors.** "`<a href>` may point to any `https:` host" leaves `http:` and `mailto:` undefined (the DCO page or a contact address in CONTRIBUTING could carry either). Say whether they fail. Failing `http:` is fine. Do not fail `mailto:` unless T-037 bars it.
3. **The loop and a rule that never fires.** When a planted violation exits 0, check-dist is broken, not the site. Say this counts as a check-dist failure for the loop, so it goes to the builder rather than being waved through.
4. **Container details for finalize item 6.**
   - Pin the Node image by digest at the `engines.node` major. An unpinned tag is one more supply-chain input.
   - The planted-violation runs are `npm run build` too, so name the container for them. Section 7 says "in a scratch copy of the branch" after an "On the host" sentence, which reads as a bare host run. 5.1's blanket rule covers it, but the sentence should match.
   - `astro build` with `--network none` is expected to work (telemetry fails quietly), and the host check confirms it.
5. **Tier 2 cut leaves references.** Cutting rules 3, 4, 6 and 7 leaves AC7 ("section 7 rule 4"), AC11's `dist/` clause and AC13 ("seven runs") pointing at rules that no longer exist. Add one line: under tier 2, those ACs are checked by the queen's host grep, and AC13 plants three violations.
6. **AC1 after the asset PR.** The asset set merges in its own PR (2.2 step 1), so T-041's PR holds the builder set plus the lockfile, not "2.2's three sets". Say that AC1 checks the PR against the builder set and the lockfile, and checks the merged asset set against `SOURCE.md`.
7. **Cited lines.** `launch.go:74` is `workerTools`; the deny list is `:81-84`. `gate.go:74` falls inside the usage text that says "approving the exact head", which is fine. Section 11 item 7 moves them all anyway.
8. **G3 claim in Q-T041-8 (a): verified.** `gate_commits.go:220-224` drops `tier3` and `outside_lease` findings when `approvable(v.Path)` and `filter` hold. `approvablePath` (`gate_approve.go:90-104`) is false for every hardcoded prefix and true for a listed path. The rescan sets `in.filter = true` (`:216-225`) only after `approvedBy(..., out.headOID)`. So `deploy/production.tf`, approved at `c3d81f`, passes.
9. **No other regression found.** The brief is 239 lines, dash-free, refers to the Head Keeper by role, and the round 1 notes it took (CONTRIBUTING mapping, source-directory resolution, "exists in the build checkout", whole-`dist/` file set, `curl -sI` header check, `--before`, the `t-041-assets` branch, the fragment grep, the RFN check, the fetched-font sources) read correctly. The CSP could use `default-src 'none'` (round 1 note 5); optional.

Counts: 2 must-fix, 9 notes.

# Brief review: T-041 round 3 (opus, confirming), target 5025cf9

Verdict: pass

Risk-Tier: 3 (by content; Tier 0 by path until the post-merge control commit)

Base: parent 9709cd9, branch `t041-fix` in the scratch clone. Read: `.waggle/hive/tasks/T-041.md` at 5025cf9 (242 lines) and `git diff 9709cd9 5025cf9` (one file, +35 -32); both round 2 reviews; at 5025cf9, `internal/dispatch/task.go:75-138`, `internal/dispatch/run.go:300-320`, `internal/comb/backend.go:145-156` and `:375-402`, `internal/comb/changecheck.go:65-80`, `.waggle/control/config.yaml:48-56`, `.github/CODEOWNERS`; a grep of `internal/` and `cmd/` for any code that reads WAGGLE.md section 2. No file in the repo, its worktrees or the clone was written, and no git state changed. No WebFetch: the Vercel facts were fetched by both round 2 guards and the plan fact comes from the queen's API check.

**Intent summary.** Round 3 is text only and closes the two round 2 must-fixes. The `gate.protected_paths` and CODEOWNERS edits leave precondition 1 and become a Head Keeper control commit right after merge, with the cost of listing stated (no lease can be `site` afterward; `site/` root files change only by a Head Keeper commit). Q-T041-9 is rebuilt from the plan check (level19 on Pro, active; All Deployments protection free on every plan), and Q-T041-4 (a) now names Vercel Authentication at All Deployments. The round 2 notes are folded into check-dist rules 2, 5, 6 and 7, the pre-review loop, section 7's container line, AC1, the tier 2 cut and section 11. Blast radius is unchanged: new files under `site/` in two PRs, one Head Keeper control commit before dispatch (WAGGLE.md section 2 only) and one after merge (config and CODEOWNERS).

## Must-fix

None.

## Confirmations

1. **M1 closed.**
   - Code at 5025cf9: `refuseProtected` (`task.go:131-138`) fails with `ErrProtectedPath` when `p.CoversDir(d)`; it is called by claim at `task.go:87` and by every non-guard dispatch run at `run.go:310-311`, with the policy read from base by `basePolicy` (`task.go:115-128`). `CoversDir` (`changecheck.go:74-76`) is `touchesPrefixes` (`backend.go:390-400`), which uses `DirsOverlap` (`backend.go:383-385`), so `site` covers `site/package.json`.
   - Nothing before merge lists a `site/` file. Precondition 1 (Depends on; section 10 item 1) is the WAGGLE.md section 2 line only. No Go code reads WAGGLE.md section 2: the only WAGGLE.md readers are the prompt inputs (`inputs.go:92`) and the section 3 command list (`run.go:158`), so "a lease entry protects nothing" is right. `config.yaml:53` stays `protected_paths: gate/` until the post-merge commit.
   - The post-merge commit (Q-T041-5 (a), section 10 item 2) is ordered after T-041's claim is released and before any other `site` claim, with D-243's no-recheck of stored claims stated as the reason. The 2.2 step 1 asset claim on `site` and dispatch's claim both precede it, so neither is refused.
   - The window is named honestly in the Risk Tier line, Q-T041-5 and 2.7: the three files are Tier 0 by path from merge to the control commit, and T-041's own PR is Tier 3 by review rule only (trailer, two guards, admin merge). The round 2 "no Tier 0 window" claim is gone; a grep finds no stale wording.
   - Later root-file edits are routed to Head Keeper commits: 2.7 (no lease can be `site`; later work leases `site/src` or `site/scripts`), Q-T041-5 (names `astro.config.mjs`, `tsconfig.json`, `.gitignore`, `go.mod`), section 10 item 3 (the `astro.config.mjs` `site` value is a Head Keeper commit or rides in item 2's commit) and item 4 (the flip edit to `site/vercel.json`). `site/src` does not cover any of the three listed files under `touchesPrefixes`, so the narrower leases stay claimable.
2. **M2 closed.** Q-T041-9 opens with the plan fact (`pro`, `active`, Vercel API, 2026-10-25), states that All Deployments protection is on every plan and only Password Protection costs extra, that Hobby's non-commercial clause does not apply, and prices every option at $0 more. "Protecting production may need Pro" is gone. Q-T041-4 (a) names Vercel Authentication at All Deployments "available on every plan at no extra cost" with `deploymentEnabled` `{"main": false}` as the second layer, and the flip lowers production protection. Q-T041-9 (b) (Standard Protection) is now an honest alternative, not a cost driver. Section 10 item 3 and section 11 item 6 re-confirm the plan before the project is created, and "none is ever an automatic step" stays.
3. **Folded notes: correct.**
   - Rule 2 allows zero or more `_astro/*.css` (fable note 4).
   - Rule 5 allows `mailto:`, fails `http:` and every other scheme (opus note 2).
   - Rule 6 reads "every listed id resolves to an element inside the same `<svg>` and at least one of them is its `<title>`", which the studio's `aria-labelledby="<title id> <desc id>"` passes (opus note 1, fable note 2).
   - Rule 7 covers `<style>` elements, `style` attributes and the CSS files (fable note 3).
   - A planted violation that exits 0 counts as a check-dist failure and opens a builder round (opus note 3).
   - The planted runs are in the 5.1 container with network off, and the image is pinned by digest at the `engines.node` major, recorded at section 11 item 6 (opus note 4).
   - AC1 now names two PRs, the asset PR checked against `SOURCE.md` sha256s and T-041's PR against the builder set plus the lockfile; the lease rule line matches (opus note 6, fable note 1).
   - The tier 2 cut maps AC7's rule 4 check and AC11's `dist/` clause to the queen's host grep, and AC13 to three planted violations (rules 1, 2, 5) plus the AC4 and AC5 cases (opus note 5). AC8 cites rule 5, which tier 2 keeps.
   - The `--before` fallback (move the date, not the pin, name both in the PR body) and the `brand.md` and guide `T-123` check are in (fable notes 7 and 6).
4. **Budget: unchanged and still correct.** The diff touches no section 9 number: 89+65+65+100+117+160+110 = 706, doubled 1,412, target 1,836, stop 2,542, cut tiers 150 and 80 doubled (230, 12.5 percent). The round 3 rule changes add a handful of check-dist lines, inside the 110 estimate.
5. **Whole-brief read: no regression.** 242 lines, no U+2013 or U+2014, the human is the Head Keeper throughout. The Layer line now names both Head Keeper control commits, the threat model's supply-chain item says the protected paths apply "once the post-merge control commit lists them", and section 12 records the claim-refusal fact with its source.

## Notes (not blocking)

1. **One cite.** The Risk Tier line cites "`CoversDir`, `internal/comb/backend.go:383-400`"; `CoversDir` is at `changecheck.go:74-76`, and `backend.go:383-400` is `DirsOverlap` and `touchesPrefixes`. Section 12's wording ("`CoversDir` via `touchesPrefixes` and `DirsOverlap`") is right. Section 11 item 7 moves the lines anyway.
2. **Tier 2 host grep is not text-node aware.** If tier 2 fires, a plain grep of the AC3 word list over `dist/` also hits attribute values, class names and path tokens that rule 4 would skip (for example `internal/comb/` in the hero). Say the queen lists and explains such hits for the guards, as the builder does for color hits.
3. **Who releases T-041's claim before the post-merge commit.** Section 10 item 2 says "once T-041's claim is released". Naming the step (the queen runs the release after merge, then tells the Head Keeper the commit can land) would make the ordering one line to follow. Optional.
4. **CODEOWNERS could have stayed in precondition 1.** A CODEOWNERS line does not touch claims, so it was safe before dispatch. Moving it with the config edit keeps one commit and changes nothing for T-041's PR, which merges by admin bypass. Fine as written.

Counts: 0 must-fix, 4 notes.
