<!-- SPDX-License-Identifier: Apache-2.0 -->
# AGENTS.md section: claim a task before you work on it

Copy everything between the two marker lines into your own `AGENTS.md`. It needs only `gh`. The full spec, including the Waggle layer that makes this atomic and expiring, is in [docs/claims.md](../docs/claims.md).

<!-- BEGIN COPY -->
## Claims

A claim tells other people and agent teams who holds a task. It shows on the issue as an assignee, the `claimed` label and one sticky comment.

Claims are cooperative. A claim stops honest agents and people from colliding. It is not a lock against anyone who can push.

**GitHub issues, with only `gh`:**

1. Read the issue first. Skip it if it shows a live claim (the `claimed` label, or a claim comment that has not expired) or an assignee.
2. To claim: assign yourself (`gh issue edit <n> --add-assignee @me`), add the label (`gh issue edit <n> --add-label claimed`), and post one comment that starts with `<!-- waggle:claim v1 key=gh-<n> -->` and lists holder, lead, paths, expires and last checked.
3. Keep it current while you work: edit that same comment, never add a second one. Renew the expiry before it passes.
4. When the work is merged, abandoned or handed off, remove all three: the assignee, the label and the comment.

**Backlog.md tasks:** read `backlog/tasks/` to choose work. The Backlog.md form needs the Waggle CLI: claim through it (`waggle claim <id>`), which shows the claim on the issue. Never edit another lead's task file.

**Governance:** vision and roadmap are human-owned. Owners (by default the repo's admins; T-141 makes the list configurable) merge steering files (`VISION.md`, `ROADMAP.md` and the Waggle config). Agents propose by PR or issue and never merge them. An owner approves a steering-file PR at its exact head.
<!-- END COPY -->
