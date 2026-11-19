<!-- SPDX-License-Identifier: Apache-2.0 -->
# Ops log

This directory is how an outside orchestrator's work in this repo stays visible to a Claude Code session. It is not product documentation. The docs site does not publish it. `site/scripts/sync-docs.mjs` copies `docs/gate.md` and `docs/recipes/*.md` only.

A session reads [docs/HANDOFF.md](../HANDOFF.md) and the open `orchestrator` issues before acting. The rule is in `CLAUDE.md`, above the Waggle import block.

## One issue per job

Each orchestrator job gets a GitHub issue on `waggle-sh/waggle` before the work, or at the start of it if it has already begun.

- Title: the job, in plain language. When the job has a Waggle ticket, start the title with `T-<id>:`.
- Body: what the job is, the ticket id when there is one, the leased paths if a claim exists, and the links a later session needs. The issue is not a ticket and it is not a claim. The gate still binds commits to a `T-<id>`, and a person or the orchestrator writes the claim. The agent doing the work does not.
- Label: `orchestrator`. Suggested description: "Work driven by an outside orchestrator." The label was not on the repo on 2026-11-18. Create it once. Until it exists, the session-start query fails and the session falls back to the handoff.
- Comments at each milestone, on the issue, not only on the pull request: pull request opened (number and head sha), approval (who, head sha, review or `waggle-approve` comment), merge (merge commit), deploy or apply (commit, result), and follow-ups still open. Close the issue when the follow-ups are done or explicitly dropped, and say which.

A Waggle ticket stays the unit the gate checks. The issue is the record a session can list without reading the hive.

## Entries

One file per deploy or other ops action, named `YYYY-MM-DD-<slug>.md`, using the PT date. Link it here.

| Date (PT) | Entry | What |
| --- | --- | --- |
| 2026-11-18 | [waggle-site hosting](2026-11-18-waggle-site-hosting.md) | Reported apply of the site stack in `example-infra`. No deploy from this repo. |

Use this template. Times are America/Los_Angeles, written with the UTC offset (`PT (UTC-8)` or `PT (UTC-7)`).

```markdown
<!-- SPDX-License-Identifier: Apache-2.0 -->
# <what>

- **When:** YYYY-MM-DD HH:MM PT (UTC-8)
- **What:**
- **Commit:** `<full sha>` in `<owner>/<repo>` (or "no commit" and where the record lives)
- **Approvals:** who, how (review or `waggle-approve <sha>`), the head sha
- **Results:** what was confirmed, and what was not re-checked
- **Manual steps:** commands or console steps a person ran, or "none"
- **Follow-ups:** open items, each with an owner when one is known
- **Issue:** `waggle-sh/waggle#<n>` when the job has one
```

## Infra applies

Applies of `example/infra` are logged in that repo at `docs/ops/applies/`, not here. Link the apply file from the entry in this directory.

Checked 2026-11-19 03:53 UTC: `https://github.com/example/infra/tree/main/docs/ops/applies` was not on `main`. `docs/ops/` held `T-023-phone-signin.md`, `aws-verified.md`, `phase-0.md`, `phase-1.md`, `resume.md`, and `security-baseline.md`. The waggle site stack and its deploy notes are on `main`:

- https://github.com/example/infra/blob/main/stacks/waggle-site/README.md
- https://github.com/example/infra/blob/main/docs/waggle-site-deploy.md
- https://github.com/example/infra/commit/8d8c315eb597d80425064d6d0de65d2e87026243

When an apply file appears under `docs/ops/applies/`, link that file from the entry here. Do not paste a second copy of the apply log.

## This handoff's own gate

The change that added `CLAUDE.md`'s session rule, `docs/HANDOFF.md`, and this directory has no Waggle ticket. No claim was written for it.

The gate needs a new ticket id (not T-041, not T-060) and a full claim whose branch list is exactly the pull request's head branch. `CLAUDE.md` is Tier 2, so a `light` claim fails. `waggle claim` accepts a file path. The directories are `CLAUDE.md` and `docs`.

```
waggle claim T-<id> --dirs CLAUDE.md,docs --gate full --owner <login> --branch <the PR head branch>
```

A person or the orchestrator runs that. The agent that edits the files does not. On 2026-11-19 the unexpired claims did not overlap those paths. Expired claims that name `docs` (T-037, branch `t-037`; T-047, branch `t-047`) do not block a new claim: the overlap check skips a claim that is not active (`internal/comb/git.go`). T-041 leases `site` on `t-041-site` and is expired.

Until that claim exists, `verify-leases-and-tiers` fails. A commit with no `T-<id>:` subject, no `Waggle-Ticket:` trailer, and no `t-<id>` branch fails with `commit has no T-<id>: prefix, Waggle-Ticket: trailer or ticket branch`. Do not invent a ticket id in the commit to get past that line.

After the ticket exists, the pull request's last paragraph should be:

```
Waggle-Ticket: T-<id>
Waggle-Risk-Tier: 2
Waggle-Verdict: pass
```

`CLAUDE.md` makes the required tier 2. The verdict trailer is the guard's. `pass` belongs there only after a guard says so. Until then, leave the verdict off rather than invent it. These trailers warn only. They never make the gate pass.

## Issue to open: deploy the docs site

This session could not create GitHub issues or labels. Open this issue, label it `orchestrator`, and link it from [docs/HANDOFF.md](../HANDOFF.md).

Title: `Deploy the docs site from main`

Body:

```
Waggle ticket: none yet. Follow-up to T-041. Do not reuse the T-041 claim.
It leases `site` on branch `t-041-site` and is expired.

Build is not part of the handoff docs change. Add a GitHub Actions workflow
in waggle-sh/waggle that builds site/ and syncs dist/ to the site bucket,
then invalidates the distribution. The role, the bucket, and the constraints
are in example/infra docs/waggle-site-deploy.md. Do not pass --acl.
The role is arn:aws:iam::000000000000:role/example-role
It trusts main and the Actions environment `site` via GitHub OIDC. Limit
that environment's deployment branches to main. Before the first deploy,
compare a real token's sub with github_deploy_subjects in
stacks/waggle-site/locals.tf.

Reported apply, evening of 2026-11-18 PT (not re-read from AWS here):
bucket example-site-bucket-519019395035, distribution EEXAMPLE0000000,
example.cloudfront.net. Alias CNAMEs in Cloudflare are still pending:
getwaggle.dev, www.getwaggle.dev, getwaggle.sh, www.getwaggle.sh, DNS only,
not proxied.

The Astro config sets no /docs base, so the build emits pages at the site
root. The stack expects docs/index.html for https://getwaggle.dev/docs.
Resolve that before the first deploy.

Also still open, not this issue's build: Walt brand kit colors are not
applied (the kit is not in the repo). The hero diagram may overflow with
Inter and JetBrains Mono.
```
