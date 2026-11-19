<!-- SPDX-License-Identifier: Apache-2.0 -->
# CLI reference

`waggle <command> [flags]`. Usage lines and flag help below are the ones the v0.1 source defines (`cmd/waggle/`); `waggle <command> -h` prints the same for your build. The commands you need for the gate are `init --gate-only`, `claim`, `release`, `gate` and `doctor`. The rest serve the full hive workflow.

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

Lease directories for a task. See [Claims and leases](/docs/claims-and-leases/).

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
waggle release <task> [--branch <name>] [--unmerged] [--owner <id>] [--repo <dir>] [--json]
```

Give back your claim once its branch has merged.

| Flag | Meaning |
| :--- | :--- |
| `--branch` | The branch: `t-<nnn>` or `waggle/T-<nnn>` with an optional suffix. |
| `--unmerged` | Release without the merged check, for a task that makes no PR or a PR merged by squash or rebase. |
| `--owner`, `--repo`, `--json` | As for `claim`. |

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

Checks that claim refs can be pushed and the gate is a required check. It reports `claim-refs`, `required-gate` and `up-to-date`. `--no-write` skips the temporary refs it would otherwise push under `refs/waggle/probe/`. It does not check that PRs are required, required reviews, code-owner review, or which app may post the check. See [the gate spec](/docs/gate/#12-install-and-waggle-doctor).

## Hive commands

These serve the full Waggle hive workflow (roles, worktrees, a decisions log). You do not need them to run the gate.

| Command | Usage | Purpose |
| :--- | :--- | :--- |
| `waggle status` | `waggle status [--repo <dir>]` | Active claims, daemon state and per-ticket token use. |
| `waggle decide` | `waggle decide --file <path \| -> [--repo <dir>] [--dry-run]` | Append one decision entry to `.waggle/hive/DECISIONS.md` from a JSON file. `--dry-run` validates and prints the entry and writes nothing. |
| `waggle dispatch` | `waggle dispatch <ticket> --role builder\|forager --dir <path>... --gate light\|full --no-cell` | Claim a ticket, run a worker in a worktree and check its changes against the claim. Run `waggle dispatch <ticket> --help` for every flag. |
| `waggle pace` | `waggle pace [--repo <dir>] [--tool claude-code\|copilot\|gas-town]` | Show the agent pace set for this repo and how to apply it in each agent tool. |
| `waggle upgrade` | `waggle upgrade [--apply] [--diff] [--allow-downgrade] [--runtime=claude-code\|none]` | Print the plan to re-render managed files from this build and move `waggle.pin`. `--apply` executes it. |
| `waggle uninstall` | `waggle uninstall [--apply]` | Print the plan to remove everything `init` installed. `--apply` executes it and needs a clean tree. |
| `waggle version` | `waggle version` | Print the version. |
