<!-- SPDX-License-Identifier: Apache-2.0 -->
# Handoff

As of 2026-11-18 19:53 PT (UTC-8). `origin/main` was `e0852e48e313d7b729bfbae98e1e325a72a01b0f`.

This file is the orchestrator handoff for any Claude Code session in this repo. Update it when an orchestrator job changes what the next session must know, and point at the GitHub issue for that job. The queen's own handoff stays `.waggle/control/handoffs/queen-<n>.md` (gitignored) and the short summary `.waggle/hive/handoff/queen.md`. Those do not replace this file.

The docs site build does not publish this file or [docs/ops/](ops/README.md). `site/scripts/sync-docs.mjs` copies `docs/gate.md` and `docs/recipes/*.md` only. A relative link from one of those sources to a file the site does not publish fails the build. This file names the account, the bucket, the distribution, and the deploy role. The repo is private. A later decision to make the repo public has to say what happens to this file and to `docs/ops/` first.

## Current state

- The repo is private. It stays private until Paul explicitly approves making it public.
- `origin/main` is the merge of pull request 206, commit `e0852e48e313d7b729bfbae98e1e325a72a01b0f`, merged 2026-11-18 19:26 PT by `paulternate`. The pull request was opened by `paulternate19`.
- The docs site source is in `site/`. It is Astro Starlight (decision D-367, 2026-11-18, superseding D-310 and D-311). D-367 is in `.waggle/hive/DECISIONS.md` on main. The Head Keeper's approval of pull request 206 covered that hive edit: review APPROVED at head `872db1b910d125949181657f7c86398337e7fc3d`, body `waggle-approve 872db1b910d125949181657f7c86398337e7fc3d`, submitted 2026-11-18 19:22 PT. Review: https://github.com/waggle-sh/waggle/pull/206#pullrequestreview-5968859288
- Nothing in this repo deploys the site. There is no site workflow under `.github/workflows/`. `site/astro.config.mjs` writes no public URL unless `SITE_URL` is set, and it sets no `/docs` base. Built pages are at the site root (`/`, `/gate/`, `/quick-start/`, and the other sidebar slugs).
- `.waggle/control/WAGGLE.md` still says `site/` is a static docs site deployed to Vercel (T-041, D-289). That line is stale. Changing it is a Head Keeper control commit. This handoff does not change `.waggle/`.
- Hosting for the site was reported applied on the evening of 2026-11-18 PT, in `l19-ltd/l19-infra` `stacks/waggle-site`, account `519019395035` (l19-prod, us-west-2). Reported resources: S3 bucket `l19-waggle-site-519019395035`, CloudFront distribution `E2AAE775EEXD3M` (`dxbcf6ndsiref.cloudfront.net`), deploy role `arn:aws:iam::519019395035:role/l19/waggle-site/l19-waggle-site-deploy`. The role trusts this repo's `main` branch and the GitHub Actions environment `site`, through the existing GitHub OIDC provider. The bucket name, the role ARN, and the trust subjects match `stacks/waggle-site/locals.tf`. The distribution id and the `cloudfront.net` name are not in git. This session could not re-read AWS. The stack README on `l19-infra` main still says "Not applied" (commit `8d8c315eb597d80425064d6d0de65d2e87026243`, 2026-11-18 06:06 PT). Details and the gap are in [docs/ops/2026-11-18-waggle-site-hosting.md](ops/2026-11-18-waggle-site-hosting.md).
- `getwaggle.dev` is the main domain. `www.getwaggle.dev`, `getwaggle.sh`, and `www.getwaggle.sh` redirect to it. Docs are planned at `getwaggle.dev/docs`. Do not refer to the bare domain `waggle.sh`. It is someone else's domain. The GitHub org `waggle-sh` is this repo's org and is not that domain.
- Fonts on the site are Inter and JetBrains Mono. The Walt brand kit is not in this repo, so its colors are not applied. The hero diagram was drawn for Public Sans and Martian Mono. `site.css` aliases those family names. Nobody has checked the rendered hero, so the diagram may overflow.
- The live claim `refs/waggle/claims/ticket-41` is a full claim, keeper `paulternate`, directory `site`, branch `t-041-site`, claimed 2026-11-18 06:28 PT, expired 2026-11-18 08:28 PT. An expired claim still passes the gate with a warning, and only for a pull request whose head branch is `t-041-site`. It does not cover docs or `CLAUDE.md`.

## In flight

- Pull request 205, ticket T-060, branch `t-060-staged-gate-modes`, head `18657121b69033507bad7f2a2af207bd84fa49c8`. Design only: `docs/rfc-notes/staged-gate-modes.md` and `.waggle/hive/tasks/T-060.md`. It is open, with no review and no comment. Its base is `220f1a7fae4f9dcaa93a3761cd146c00a6b5a269`, which is main from before pull request 206. https://github.com/waggle-sh/waggle/pull/205
- Alias DNS CNAMEs in Cloudflare are not created yet. After the apply they are DNS-only CNAMEs for `getwaggle.dev`, `www.getwaggle.dev`, `getwaggle.sh`, and `www.getwaggle.sh`, aimed at `dxbcf6ndsiref.cloudfront.net`. Do not proxy them.
- A site deploy workflow in this repo is not built. Do not add it while doing a docs or handoff change. The tracking issue was not opened: this session could not create GitHub issues. The text to open is in [docs/ops/README.md](ops/README.md#issue-to-open-deploy-the-docs-site). When the issue exists, replace that pointer with the issue link.
- The label `orchestrator` does not exist on `waggle-sh/waggle`. The convention is in [docs/ops/README.md](ops/README.md). Create the label before relying on the session-start query.

## Waiting on Paul

- Head Keeper approval of pull request 205 at its exact head, then a re-run of the failed check. The brief is under `.waggle/hive/`, which no claim can lease. Approval is a review, or a comment whose own line is `waggle-approve` plus the full 40-character head sha.
- Rulings on RFC questions 1, 2, 3, and 9 before any code ticket is cut. In short: (1) how a new install chooses the mode, (2) whether raising `open` to `guard` can avoid a Tier 3 admin merge, (3) whether a broken base config still fails in `open`, (9) whether staged modes land before or after the public gate. Questions 4 to 8 are also open. They are not the ones blocking the ruling.
- The alias CNAMEs above. DNS stays on Cloudflare. The stack creates no Route 53 records.
- An explicit yes before this repo becomes public.
- A new Waggle ticket and a full claim for the handoff docs, if that pull request is still open. This session did not write a claim. T-041 and T-060 do not cover it. The claim the gate needs is stated in [docs/ops/README.md](ops/README.md) under "This handoff's own gate".

## Recent orchestrator changes

- 2026-11-18 19:26 PT: merged pull request 206 (T-041) after the Head Keeper approval at `872db1b9`. Merge commit `e0852e48e313d7b729bfbae98e1e325a72a01b0f`.
- 2026-11-18: recorded D-367 on that pull request (commit `05b22a103f9737410b45fc44dce44ecea5d4f15d`). Astro Starlight supersedes D-310 and D-311.
- 2026-11-18 06:28 PT: claim ref `refs/waggle/claims/ticket-41` written for branch `t-041-site` and directory `site`. The ref's keeper id is `paulternate`. It is expired. See Current state.
- 2026-11-18, evening PT: reported apply of `l19-infra` `stacks/waggle-site`. See the ops entry. `docs/ops/applies/` on `l19-infra` main did not exist when this handoff was written.
