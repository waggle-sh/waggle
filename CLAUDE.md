# Session start

Do this before editing, reviewing, or answering from this checkout. A checkout that skips it can be commits behind `origin/main` and will not see later handoff notes.

1. `git fetch origin`
2. Report how far this checkout is behind `origin/main`. Run `git rev-list --left-right --count HEAD...origin/main` and report both numbers, plus the `HEAD` and `origin/main` shas. Say so when the count behind is 0.
3. Read [docs/HANDOFF.md](docs/HANDOFF.md).
4. Read the open GitHub issues labeled `orchestrator` on `waggle-sh/waggle` (`gh issue list --repo waggle-sh/waggle --label orchestrator --state open`). Read each open issue's body and its latest comments, not the titles alone. If the label does not exist, say so and use the handoff.

Orchestrator jobs, the work an outside assistant does in this repo, are recorded in [docs/ops/README.md](docs/ops/README.md): one GitHub issue per job, a comment at each milestone, and an ops entry when the job deploys or applies something. A Waggle ticket id, when the job has one, goes on the issue. The issue does not replace the ticket, and it is not a claim. A person or the orchestrator writes the claim. The agent doing the work does not.

<!-- waggle:start -->
@.waggle/control/WAGGLE.md
<!-- waggle:end -->
