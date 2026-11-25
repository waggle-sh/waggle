<!-- SPDX-License-Identifier: Apache-2.0 -->
# Gate walkthrough

This example shows the gate passing and then failing, on your machine, with no GitHub repo and no network. [`run.sh`](run.sh) works in a `mktemp -d` directory and removes it on exit. It writes nowhere else.

## Run it

Build `waggle` from a clean clone to a path outside the clone, then run the script:

```sh
go build -o /tmp/waggle ./cmd/waggle
sh examples/gate-walkthrough/run.sh /tmp/waggle
```

`waggle init --gate-only` pins the gate Action to a clean commit stamp or a release tag. A binary built from a modified tree has neither, and `init` refuses it (see [gate spec](../../docs/gate.md) section 12). The installed release binary works too.

## What the script does

1. Makes a bare repo as `origin` and a clone of it.
2. Runs `waggle init --gate-only` and commits the result on `main`.
3. Writes a claim: `waggle claim T-001 --dirs src --gate light --branch t-001 --owner demo`. The claim leases `src/` for task `T-001`.
4. Commits `T-001: add src/a.txt` on branch `t-001` and runs `waggle gate verify --base main --head t-001`.
5. Commits `T-001: touch README.md`, a file outside `src/`, and runs the same check again.

## Real output

This is the output of a run. The claim expiry time and claim id change on every run.

```

== bare repo as origin, and a clone

== waggle init --gate-only, committed on main
Initializing Waggle gate (gate-only)...
  [create] .github/workflows/waggle-gate.yml
  [create] .waggle/control/config.yaml
  [create] .waggle/hive/DECISIONS.md
Gate initialized. Next: commit .github/workflows/waggle-gate.yml, .waggle/control/config.yaml and .waggle/hive/DECISIONS.md, then require the check verify-leases-and-tiers on your default branch.

== waggle claim T-001 --dirs src --gate light --branch t-001 --owner demo
claimed T-001 for demo until 2026-11-25T19:42:33Z: src (gate light, claim ab02b5fc183d)

== commit T-001: add src/a.txt on branch t-001

== waggle gate verify --base main --head t-001
leases: warning: no pull request branch; claim branches not checked (pass --branch)
leases: ok
tiers: ok
decisions: unchanged
exit 0

== commit T-001: touch README.md, outside the leased src/

== waggle gate verify --base main --head t-001
leases: warning: no pull request branch; claim branches not checked (pass --branch)
leases: FAIL: README.md: T-001: path is outside the claim directories
tiers: ok
decisions: unchanged
exit 1
```

## What each line means

- `claimed T-001 for demo until <time>: src (gate light, claim ...)`: the claim now exists as the ref `refs/waggle/claims/ticket-1`. It leases `src/` and accepts the PR branch `t-001`.
- `leases: warning: no pull request branch; claim branches not checked (pass --branch)`: the walkthrough runs the gate by hand, not as a PR, so it names no head branch and the check that the claim names the branch is skipped. In CI the gate learns the branch from the PR event.
- `leases: ok`: every commit from the merge base to `t-001` binds to `T-001` and changes only paths under `src/`.
- `tiers: ok`: no changed path is protected or needs a higher risk tier.
- `decisions: unchanged`: `.waggle/hive/DECISIONS.md` was not touched.
- `exit 0`: the gate passed.
- `leases: FAIL: README.md: T-001: path is outside the claim directories`: the second commit changed `README.md`, which is not under `src/`. The gate checks every commit, so the first clean commit does not save the branch.
- `exit 1`: the gate failed. Exit 2 is a usage error and exit 3 is a git or I/O error.

The full rules are in the [gate spec](../../docs/gate.md).
