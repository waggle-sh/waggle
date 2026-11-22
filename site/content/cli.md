<!-- SPDX-License-Identifier: Apache-2.0 -->
# CLI reference

`waggle <command> [flags]`. Usage lines and flag help below are the ones the source on `main` defines (`cmd/waggle/`); `waggle <command> -h` prints the same for your build. Commands and flags marked **on main** are merged but not in a release yet (the latest is v0.1.1), so the install script does not get them; build from source to use them. The commands you need for the gate are `init --gate-only`, `claim`, `release`, `gate` and `doctor`. The rest serve the full **hive** workflow (the hive is the repo's shared Waggle setup; its **keepers** are the humans, its **queen** the coordinating agent they talk to).

## Exit codes

`gate` and `doctor` share a convention: `0` pass, `1` a check failed or a required setting is missing, `2` usage, `3` git or I/O error (an unreadable config, forge or default branch).

## Gate commands

### `waggle init`

Scaffold Waggle into the current directory.

| Flag | Default | Meaning |
| :--- | :--- | :--- |
| `--gate-only` | off | Write only `.github/workflows/waggle-gate.yml`, a small `.waggle/control/config.yaml` and `.waggle/hive/DECISIONS.md`. |
| `--profile` | `cli` | Project profile (v0.1: `cli`). |
| `--delivery` | `installed` | `installed` or `hosted`. |
| `--forge` | `github` | Forge adapter (v0.1: `github`). |
| `--comb-mode` | `direct` | Claim ref mode: `direct` (`relay` is accepted but not implemented). |
| `--runtime` | `claude-code` | Local agent runtime: `claude-code` or `none`. |
| `--force` | off | Rewrite managed Waggle files. Seeded files are never overwritten. |

`--gate-only` refuses a build that has neither a clean commit stamp nor a release tag, because it pins the gate Action to that build.

### `waggle claim`

```
waggle claim <task> --dirs <a,b> --gate light|full [--branch <name>]... [--ttl <duration>] [--owner <id>] [--base <rev>] [--repo <dir>] [--json]
```

Lease directories for a task. The queen (the coordinating agent a keeper talks to) runs this, or a trusted person without a queen, never the forager or builder doing the task. See [Claims and leases](/docs/claims-and-leases/).

| Flag | Meaning |
| :--- | :--- |
| `--dirs` | Comma-separated directories to lease. |
| `--gate` | `light` or `full`. |
| `--branch` | PR branch this claim accepts. Repeatable, up to 4. |
| `--ttl` | Claim lifetime in whole minutes. Default `leases.default_ttl_minutes`. |
| `--owner` | Claim owner, usually your GitHub login. Required without `.waggle/control/local.yaml`; otherwise defaults to its `keeper_id`. |
| `--base` | Revision whose config lists protected paths. Default: `main` on the remote. |
| `--repo` | Repository. Default `.`. |
| `--json` | Print one JSON object. |

### `waggle release`

```
waggle release <task> [--branch <name>] [--unmerged] [--co <id>]... [--owner <id>] [--repo <dir>] [--json]
```

Give back your claim once its branch has merged. The release also records who held the task, in `refs/waggle/receipts/main`; [Nectar](/docs/nectar/) scores from those records.

| Flag | Meaning |
| :--- | :--- |
| `--branch` | The branch: `t-<nnn>` or `waggle/T-<nnn>` with an optional suffix. |
| `--unmerged` | Release without the merged check, for a task that makes no PR or a PR merged by squash or rebase. |
| `--co` | Another keeper who shares the credit with you in the release record. Repeatable, up to 4. |
| `--owner`, `--repo`, `--json` | As for `claim`. |

### `waggle force-release`

**On main.**

```
waggle force-release <task> | --ref <ref> [--oid <id> --reason <text>] [--owner <id>] [--repo <dir>] [--json]
```

Clear a stuck or malformed claim, whoever owns it. Only the first login in `gate.approvers`, read from main on the remote and never from local files, may clear one. It takes two runs and never asks for confirmation.

1. Without `--oid` it only looks. It prints the claim, who may clear it and the exact second command, and writes nothing.
2. With `--oid` and `--reason` it deletes the claim ref, but only if the ref is still at that object id. It records who cleared it and why in `refs/waggle/audit/force-releases`.

| Flag | Meaning |
| :--- | :--- |
| `--ref` | The full claim ref, as `status` or an error printed it, instead of `<task>`. |
| `--oid` | The full object id the first run printed. |
| `--reason` | Why the claim is cleared, 1 to 512 bytes. Stored in the record, never printed. |
| `--owner`, `--repo`, `--json` | As for `claim`. |

The check is advisory: anyone who can push to the remote can delete a claim ref with raw git and leave no record. The command does not run inside a dispatch worktree.

### `waggle gate`

```
waggle gate <check> --base <rev> --head <rev> [--repo <dir>]
```

The merge-gate checks CI runs against a pull request. Each compares `merge-base(base, head)` to head. There is no override flag: an override is an admin merge that bypasses the required check.

| Check | What it does |
| :--- | :--- |
| `verify` | Runs `leases`, `tiers` and `decisions`, reports every failure and exits with the worst code. This is what the workflow runs. |
| `leases` | Every commit is bound to a task and its paths stay inside the claim's directories. |
| `tiers` | No commit touches a Tier 3 path (including `gate.protected_paths` of the base config) or, under a `light` claim, a Tier 2 path. |
| `decisions` | Rejects any change to `DECISIONS.md` that is not a pure append of well-formed entries. |

`--base` and `--head` are required. `--repo` defaults to `.`. `verify`, `leases` and `tiers` also take `--branch <name>` (default `GITHUB_HEAD_REF` under GitHub Actions). Details and the code behind each check are in the [gate spec](/docs/gate/#9-output-and-exit-codes).

### `waggle doctor`

```
waggle doctor [--repo <dir>] [--no-write]
```

Checks that claim refs can be pushed and the gate is a required check. It reports `claim-refs`, `required-gate` and `up-to-date`. On main it also prints an informational `claims` finding, which names the first malformed claim and the `waggle force-release --ref` line that clears it, and says who may run `force-release`; it never changes the exit code. `--no-write` skips the temporary refs it would otherwise push under `refs/waggle/probe/`. It does not check that PRs are required, required reviews, code-owner review, or which app may post the check. See [the gate spec](/docs/gate/#12-install-and-waggle-doctor).

## Hive commands

These serve the full Waggle hive workflow: roles (a **forager** triages and maps leases, a **builder** implements, a **guard** reviews, a **scout** writes briefs), worktrees and a decisions log. You do not need them to run the gate. The Nectar and Quorum commands are in [their own section](#nectar-and-quorum).

| Command | Usage | Purpose |
| :--- | :--- | :--- |
| `waggle status` | `waggle status [--repo <dir>]` | Active claims, daemon state and per-ticket token use. On main it also names malformed claims with the `force-release` line that clears them, and warns about two claims whose directories differ only in letter case. |
| `waggle hives` | `waggle hives status [--registry <file>] [--json]`, `waggle hives plan new\|add\|show\|list\|check\|brief\|rm` | A read-only view across several hives listed in a registry file, and cross-repo plans. |
| `waggle decide` | `waggle decide --file <path \| -> [--repo <dir>] [--dry-run]` | Append one decision entry to `.waggle/hive/DECISIONS.md` from a JSON file. `--dry-run` validates and prints the entry and writes nothing. |
| `waggle dispatch` | `waggle dispatch <ticket> --role builder\|forager --dir <path>... --gate light\|full --no-cell` | Claim a ticket, run a worker in a worktree and check its changes against the claim. Run `waggle dispatch <ticket> --help` for every flag. |
| `waggle pace` | `waggle pace [--repo <dir>] [--tool claude-code\|copilot\|gas-town]` | Show the agent pace set for this repo and how to apply it in each agent tool. |
| `waggle upgrade` | `waggle upgrade [--apply] [--diff] [--allow-downgrade] [--runtime=claude-code\|none]` | Print the plan to re-render managed files from this build and move `waggle.pin`. `--apply` executes it. |
| `waggle uninstall` | `waggle uninstall [--apply]` | Print the plan to remove everything `init` installed. `--apply` executes it and needs a clean tree. |
| `waggle version` | `waggle version` | Print the version. |

### Role status

`waggle dispatch` runs these roles today.

| Role | Status | What it does |
| :--- | :--- | :--- |
| Builder | Ready | `waggle dispatch <ticket> --role builder` claims the ticket, runs the agent in a worktree and checks its changes against the claim. |
| Forager | Ready | `--role forager` does triage, repro and lease mapping the same way. |
| Guard | Ready | `--role guard` runs the review agent on a branch. The guard is the review bee; the required merge check `verify-leases-and-tiers` is the gate, not the guard. |

## Nectar and Quorum

[Nectar](/docs/nectar/) is the reputation score and [Quorum](/docs/quorum/) the priority rounds weighted by it. `waggle nectar backfill` is in v0.1.1. Every other command here is **on main**.

| Command | Usage | Purpose |
| :--- | :--- | :--- |
| `waggle nectar` | `waggle nectar [--repo <dir>] [--keeper <id>] [--now <rfc3339>] [--main <oid>] [--receipts <oid>] [--no-fetch] [--recompute] [--json]` | Print every keeper's score, every count of a rule that gave less than full credit, and the inputs it was scored at. |
| `waggle nectar explain` | `waggle nectar explain <task> [--repo <dir>] [--now <rfc3339>] [--main <oid>] [--receipts <oid>] [--no-fetch] [--json]` | Show why one task earned what it did. |
| `waggle nectar backfill` | `waggle nectar backfill [--apply] [--owner <id>] [--repo <dir>] [--json]` | Add release records for tasks that merged before records existed. |
| `waggle nectar gate-receipt` | `waggle nectar gate-receipt --before <sha> --after <sha> [--actor <id>] [--repo <dir>] [--json]` | Write a release record after a merge for a task that still has a live claim. The gate workflow's optional `gate-receipt` job runs it; see [the recipe](/docs/recipes/gate-run-receipts/). |
| `waggle vote` | `waggle vote --round <n> <item>=<votes>... [--dry-run] [--key <path>] [--now <rfc3339>] [--repo <dir>] [--json]` | Cast a signed ballot in an open round. `--dry-run` checks the votes and prints the cost. |
| `waggle quorum open` | `waggle quorum open (--items T-101,issue-7 \| --from-roadmap) [--closes <rfc3339>] [--key <path>] [--now <rfc3339>] [--repo <dir>] [--json]` | Start a round. Only a registered key of a `gate.approvers` login can open one. |
| `waggle quorum status` | `waggle quorum status [--round <n>] [--next] [--now <rfc3339>] [--repo <dir>] [--json]` | Scores so far, raw votes, credits left, vetoes and ballots not counted. `--next` lists what can be picked up from a closed round. |
| `waggle quorum veto` | `waggle quorum veto --round <n> --item <id\|all> --decision D-<n> [--key <path>] [--repo <dir>]` | The Head Keeper removes one item, or rejects the round, citing a decision in `DECISIONS.md`. |
| `waggle quorum close` | `waggle quorum close --round <n> [--key <path>] [--repo <dir>] [--json]` | End a round after its close time and record the signed tally. |
| `waggle quorum verify` | `waggle quorum verify --round <n> [--receipt <oid>] [--repo <dir>] [--json]` | Recompute the tally from the refs and fail unless it matches byte for byte. |

Quorum needs OpenSSH `ssh-keygen` with `-Y` support. Exit codes for `vote` and `quorum`: `0` done, `1` a check or validation failed, `2` usage, `3` git, signing tool or file error.
