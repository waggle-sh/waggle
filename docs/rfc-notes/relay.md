<!-- SPDX-License-Identifier: Apache-2.0 -->
# RFC note: claims for contributors without push access

> **DESIGN SPIKE. NOT IMPLEMENTED.** Nothing here exists in the code beyond the stub noted in section 1. It is a proposal for the Head Keeper to rule on. No decision is recorded.

- **Status:** Proposed (ticket T-055, relay design spike). Relay stays out of the v0.1 gate slice (repositioning.md section 4).
- **Date:** 2026-11-19
- **Builds on:** `docs/gate.md` (L1, L3, L5, L11), `docs/multi-keeper.md`, `docs/RFC-0003.md` section 3.1 ("Public OSS Permission Relay"), D-232 (the lease record is the claim ref, read from the base side), D-366 (multi-Keeper restored in full).
- **Vision item served:** core item 3, "how untrusted contributors claim without push access stay on the path". Nectar (core item 5) depends on the answer for attribution.

## 1. Problem

A claim is a ref, `refs/waggle/claims/ticket-<n>`, pushed to the base repository. Only someone who can push there can write one. An outside contributor working from a fork cannot. Today the only path is that a maintainer writes the claim for them, with a branch entry `<owner>/<repo>:<branch>` that the gate matches against the fork PR's head repository (gate spec section 3 and L3).

The code carries a stub: `forge.comb_mode: relay` parses, and then every claim, release and list call refuses with `relay comb mode not implemented in v0.1` (`internal/comb/git.go`). RFC-0003 named the idea (contributors comment `/waggle claim`, a workflow writes the ref) and never specified it.

What must hold for any option:

1. The gate stays authoritative at merge and keeps reading the lease from the base side (D-232).
2. No code from a fork PR runs with a write token.
3. Contention still resolves at claim time where possible, so an outsider does not do a day of work on a task someone else holds.
4. The claim records who the work is for, as a GitHub login, because Nectar attributes through `keeper_id` (T-050).

## 2. GitHub facts that shape the options

- **`pull_request` from a fork** runs the PR's copy of the workflow with a read-only `GITHUB_TOKEN` and no repository secrets. Repos can require approval before a first-time contributor's workflow runs. This is how the gate runs today.
- **`pull_request_target`** runs the default branch's workflow file in the base repository's context, with a write-capable token and secrets. It is safe only if it never checks out or executes the PR's code; checking out the head under this event is the well-known "pwn request" pattern. The gate does not run here (gate spec L5).
- **`issue_comment`** also runs the default branch's workflow with a write-capable token. The comment body is attacker text: interpolating `${{ github.event.comment.body }}` into a shell step is a code injection. The commenter's login and `author_association` (OWNER, MEMBER, COLLABORATOR, CONTRIBUTOR, FIRST_TIME_CONTRIBUTOR, NONE) come from GitHub and can be trusted as identity.
- **Labels** can be added only by people with Triage access or more, so a label is a cheap maintainer signal.
- **Ref scope.** `contents: write` lets a token push any ref, branches included. Rulesets target branches and tags; whether one can confine a token to `refs/waggle/` is not verified. A relay token is therefore as strong as a repo writer for refs.

## 3. Options

### Option A: relay, a trusted workflow writes the claim from a request

A contributor asks for a claim. A workflow on the default branch reads the request, checks it, and runs the base tree's own `waggle claim` to write the ref, with `--owner` set to the requester's login. The request can arrive three ways:

- **A1, issue or PR comment:** `/waggle claim T-123 --dirs internal/pricing --branch alice/shop:t-123`. Works before any PR exists, on the task's issue.
- **A2, PR label:** the contributor opens the PR with a claim request in its description; a maintainer adds `waggle:claim`, and a `pull_request_target` job (no checkout of the head) writes the claim.
- **A3, a claim-request file in the PR:** the request is a file the PR adds. The relay must read it from the head without executing anything, which means fetching one blob through the API. It also cannot live under `.waggle/` (Tier 3).

**Security.**
- *Spoofing:* the requester is the comment author or PR author as GitHub reports it, never a field in the request text. The relay refuses a fork entry whose owner is not the requester, so nobody can claim on behalf of someone else's fork.
- *Lease squatting:* an outsider could claim wide directories or many tasks to block others. Answers: a per-login cap on active relay claims, a shorter maximum TTL for relay claims, an optional allowlist of directories relay may lease, and an approval mode where a maintainer label or `/waggle approve-claim` is needed before the write. `waggle claim` already refuses Tier 3 paths, `.waggle/`, Tier 2 paths under `light`, protected paths and overlaps.
- *Sybil and spam:* many throwaway accounts defeat a per-login cap. Approval mode is the default for `FIRST_TIME_CONTRIBUTOR` and `NONE`; auto-approval is opt-in by association, and later by a Nectar threshold (open question 3). A concurrency group and a per-login rate limit bound Actions minutes.
- *Injection:* the comment body reaches `waggle` only as an environment variable and is parsed by Go, never by the shell.
- *Token blast radius:* the job has `contents: write` and nothing else, never `actions: write` (which would let it delete gate runs, L11). It runs only the binary built from the default branch.

**Attribution.** `keeper_id` is the requester's login, so Nectar credits the contributor. The claim commit's author is the relay identity, and its message records the request (comment or PR URL), so `git log` on the claim ref shows both who asked and who wrote.

**What the gate checks.** Nothing new. It reads the claim ref and matches the fork entry as today. A claim written after the PR's gate run needs a fresh run: the contributor pushes again, or a maintainer re-runs the check.

**Cost.** One new workflow (a Head Keeper commit under `.github/workflows/`), a `relay` config block, the request parser and checks, and removing the stub refusal. A1 alone is the smallest useful piece.

### Option B: claims carried in the PR, validated at merge

The PR declares its own task and directories (in the description or a file). No ref is written before merge. The gate reads the declaration from the head and passes the PR only if a listed approver approved it at the head commit.

- **Security:** no write token anywhere, which is its strength. But the lease is declared by the author, possibly an agent, which is the weak case repositioning.md section 6 warns about; it is only as good as the approver who reads it. It needs a new rule in the gate that reads lease data from the head, which D-232 ruled out.
- **Contention:** none at claim time. Two contributors can work the same task for days and find out at review. This drops "optimistic at claim time" for fork PRs (core item 1).
- **Attribution:** the PR author, as GitHub reports it. Clean for Nectar, but it lives in the forge, not in git, so it is not portable.
- **Gate:** a new head-side reader and an approval tie-in. Larger gate change than A.

### Option C: maintainer pre-claims

The maintainer writes the claim: `waggle claim T-123 --dirs internal/pricing --branch alice/shop:t-123 --owner alice`. This works today with no code (gate spec L3).

- **Security:** strongest. A person with push access decides every lease.
- **Contention:** full claim-time checks.
- **Attribution:** correct only if the maintainer passes `--owner <contributor>`; otherwise the maintainer gets the credit.
- **Scale:** the maintainer is a bottleneck, and the contributor must know their fork and branch name in advance (up to 4 entries). Fine for a handful of outsiders, not for a busy project.

### Comparison

| | A: relay | B: claim in PR | C: maintainer pre-claim |
| :--- | :--- | :--- | :--- |
| Works today | no | no | yes |
| Write token in CI | yes, `contents: write`, base code only | no | no |
| Claim-time contention | yes | no | yes |
| Lease read from base (D-232) | yes | no, needs a new decision | yes |
| `keeper_id` is the contributor | yes, from the GitHub event | PR author, forge only | only if `--owner` is set |
| Maintainer effort per claim | none, or one label in approval mode | one review | one command |
| Gate change | none | new head-side rule | none |

## 4. Recommendation

**Option C now, documented as the supported path, and build Option A1 (comment relay) as the v0.2 follow-up, with approval mode on by default.** A keeps D-232 and the gate unchanged, keeps contention at claim time, and gets attribution from GitHub's own identity, which Nectar needs. C remains the fallback when relay is off or refuses. A2 and A3 can follow if A1 is too awkward in practice. B is not recommended: it moves the lease to author-declared data and drops claim-time contention for the very contributors who most need it.

## 5. Proposed follow-up ticket (number assigned by scout)

**Relay claims: `/waggle claim` comments write claim refs for contributors without push access.**

- **Release:** v0.2. Not part of the v0.1 gate slice.
- **Lease (likely):** `cmd/waggle/`, `internal/comb/`, `internal/config/`, `internal/scaffold/`, `docs/`; the workflow lands as a Head Keeper commit under `.github/workflows/`.
- **Scope:** a `waggle relay` entry point that reads one `issue_comment` event; `relay.*` config (enabled, auto-approve associations, max active claims per login, max TTL, optional directory allowlist); the fork-owner-equals-requester check; approval mode via label or `/waggle approve-claim`; `/waggle release` by the requester; a scaffolded workflow with `contents: write` only; remove the `ErrRelayNotImplemented` refusal for the paths relay owns.
- **Acceptance:** a fork contributor's comment produces a claim with their login as `keeper_id` and their fork branch listed; the gate passes their PR unchanged; a comment that names another user's fork, a protected path, an overlap, or exceeds a cap is refused with a reply and no ref write; no step interpolates comment text into a shell; tests cover injection strings in the comment.
- **Depends on:** T-050 for the final attribution fields; can start without it.

## 6. Open questions

1. **Where the relay's write identity comes from:** the workflow's `GITHUB_TOKEN` or a GitHub App token. An App gives a distinct commit author and can be revoked alone; it costs setup for every adopting repo.
2. **Recording the request in the claim:** commit message only, or an optional field in `lease.json`. A new field risks L10: an older CLI that renews the claim drops it.
3. **Auto-approval by Nectar:** should a contributor above a Nectar threshold skip approval mode? This ties relay to T-050 and is the "earn a say" path in VISION.md.
4. **Release on PR close:** a `pull_request_target` job on `closed` (no checkout) could release relay claims, or a scheduled sweep could. Either adds a write-token trigger.
5. **Durable history for Nectar:** release deletes the claim ref today, so a relay claim's attribution disappears with it. Whether Nectar keeps its own record is a T-050 question, flagged here.
6. **Ref scope:** can a ruleset or App permission confine the relay token to `refs/waggle/claims/`? If not, relay is a repo-writer credential and must be documented as one.
7. **CUT? needs Head Keeper typed reason.** Option B would drop claim-time leases for fork PRs, which narrows core item 1 ("Optimistic at claim time, authoritative at merge"). It is listed for comparison, not recommended.

## 7. Decision requested

**Question:** How should contributors without push access, fork PRs included, get a claim?

**Options:**
1. Document maintainer pre-claims (Option C) as the supported path now, and build the comment relay (Option A1) in v0.2 under the follow-up ticket in section 5, approval mode on by default.
2. Claims carried in the PR and validated at merge (Option B). Needs a new decision that supersedes D-232 for fork PRs. **CUT? needs Head Keeper typed reason** (open question 7).
3. Relay alone (Option A1) in v0.2, with no documented interim path.

**Hive recommendation:** Option 1.

**Keeper choice:** Fork contributor claims: maintainer claims on contributor's behalf now; v0.2 builds /waggle claim comment relay with maintainer approval on by default. Head Keeper decision, Paul Cleary, 2026-11-19.
