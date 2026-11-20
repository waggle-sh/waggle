<!-- SPDX-License-Identifier: Apache-2.0 -->
# Hives: a read-only view and advisory plans across your hives

> **Roadmap feature, phases 1 and 2 of 4.** These are the first steps of the multi-repo queen in [RFC-0004](RFC-0004-multi-repo-queen.md): the read-only view (`waggle hives status`) and cross-repo planning (`waggle hives plan`). Coordinated work (phase 3) does not exist. **Nothing here writes to a hive.**

If you keep several hives, one per repository, `waggle hives status` answers "what is open, claimed, blocked or waiting on me?" across all of them, with one merged list of decision cards. `waggle hives plan` keeps an ordered plan for a change that spans several of them.

## What it never does

This holds for both `hives status` and `hives plan`.

- It writes nothing to any hive: no claim, no release, no dispatch, no decision, no file, no ref.
- It does not fetch. Each hive is read from its own checkout and its local claim refs. Run `git fetch` in a hive yourself if you want newer claims.
- It never enforces a plan. Plans are advice; the gate is unchanged and each hive's claims and leases stay authoritative.
- It never claims across hives. Claims, leases and the gate stay per hive and keep their meaning.
- It never answers a decision card for you. Answer each card in its own hive.

## The registry

The registry is a file you own, outside every hive. A repository cannot add itself to it, and a pull request cannot widen what the view reads. Its default place is `waggle/hives.json` in your user config directory (on Linux, `~/.config/waggle/hives.json`). Override it with `--registry <file>` or the `WAGGLE_HIVES_REGISTRY` environment variable.

```json
{
  "hives": [
    {"name": "cli",  "path": "/home/me/src/cli"},
    {"name": "site", "path": "/home/me/src/site", "level": "view"},
    {"name": "lib",  "repo": "me/lib"}
  ]
}
```

- `name`: lowercase letters, digits, `.`, `_` and `-`. It must be unique.
- `path`: the absolute path of a local checkout. Phases 1 and 2 read checkouts only.
- `repo`: an `owner/repo` label. An entry with only a `repo` is listed as unreachable, because a remote cannot be read yet.
- `level`: `view` (the default), `plan` or `work`. It is the ceiling you set for that hive. `hives status` reads at every level. `hives plan` needs `plan` or `work` and refuses `view`. Nothing writes to a hive at any level.

A file that is not valid JSON, has an unknown field, repeats a name or holds a relative path is refused.

## Running the view

```
waggle hives status [--registry <file>] [--json]
```

For each hive, in registry order:

- **claimed:** the active claims, as `waggle status` lists them. The view reads them with the same code, so the two agree.
- **open:** tasks in the hive's task briefs that are not done, not claimed and not waiting on another task.
- **blocked:** tasks that depend on another task of the same hive that is not done.
- **waiting on the Keeper:** decisions whose Keeper choice is still "Not decided" or "Pending".

Then one list of all those cards, each tagged with its hive. A hive that cannot be read (path missing, not a hive, no checkout) is named as unreachable; the view does not guess its state.

`--json` prints the same report as JSON. Exit codes: 0 report printed (unreachable hives are named in it), 1 registry missing or invalid, 2 usage.

Text from a hive is data. Titles are flattened to one line and cut to 100 characters, and nothing in a hive's records is treated as an instruction.

## Cross-repo plans (phase 2)

A plan is an ordered list of per-hive tasks with the dependencies between them, inside one hive or across hives. It is a document, not a lock: `hives plan` reports the ordering and flags when it is violated, and does not enforce it (RFC-0004 section 6.4).

**It writes nothing into any hive.** The only files `hives plan` writes are plan files in your plans directory and, when you ask with `--out`, a draft brief at a path outside every hive. A repository cannot create or widen a plan: plans are never read from repo contents. The tests snapshot every hive's files and refs before and after each plan command.

**Levels.** A plan step may name only a hive that is in the registry at level `plan` or `work`. A hive at `view` (the default) or one not in the registry is refused. If you later lower a hive to `view`, `check` reports its steps as `refused` and `brief` refuses them.

### Where plans live

One JSON file per plan, `<plans dir>/<plan-name>.json`, mode 0600, written atomically. The plans directory is, in order: `--plans-dir <dir>`, the `WAGGLE_HIVES_PLANS` environment variable, else `plans/` next to the registry (on Linux, `~/.config/waggle/plans/`). It must be an absolute path. The registry is resolved as in the view (`--registry`, `WAGGLE_HIVES_REGISTRY`).

```json
{
  "version": 1,
  "name": "feat",
  "steps": [
    {"id": "s1", "hive": "lib", "task": "T-020", "depends_on": []},
    {"id": "s2", "hive": "cli", "title": "use the new API", "depends_on": ["s1"]}
  ]
}
```

- `id`: `s1`, `s2`, ... assigned by `add`. A step may depend only on earlier steps, so a plan cannot loop.
- `hive`: a registry name. `task`: the hive's ticket id, when the task exists. `title`: your own words. A step needs a task, a title or both.
- Parsing is strict: an unknown field, a wrong version, a repeated id, a bad task or a dependency that is not an earlier step makes the file refused.

### Commands

```
waggle hives plan new   <plan>
waggle hives plan add   <plan> --hive <name> [--task T-nnn] [--title <text>] [--depends-on s1,s2]
waggle hives plan show  <plan> [--json]
waggle hives plan list
waggle hives plan check <plan> [--json]
waggle hives plan brief <plan> <step> [--out <file>]
waggle hives plan rm    <plan> [--step <id>]
```

Every command also takes `--registry` and `--plans-dir`.

- `check` rebuilds each step from its hive's current state, read as `hives status` reads it: `done`, `claimed`, `open`, `blocked` (by the hive's own tasks or by an unfinished plan step), `draft` (no task yet), `missing` (the hive has no such task) or `refused`. It flags an **ordering violation** when a step is claimed or done while a step it depends on is known not to be done. A hive that cannot be read makes its steps `stale`; nothing is guessed, and a dependency on a stale step is listed as unverified, not as a violation. Plans keep no history, so a stale step is reported as stale as of this check, not since a recorded time.
- `brief` drafts a per-hive brief on stdout, or in `--out`. `--out` must be an absolute path, outside every registered hive and outside any git work tree, in a directory that exists, and must not already exist. You edit the draft and land it in that hive through its normal path (a branch, a pull request, its gate). The draft is built from the plan and validated fields only; hive text is never copied into it.
- `rm` removes a whole plan, or one step with `--step` (refused while another step depends on it).

Exit codes match the view: 0 done (a check with violations still exits 0, and the violations are in the report), 1 refused (registry or plan missing or invalid, a `view` or unknown hive, a bad `--out`), 2 usage.

Hive text is data here too: titles shown by `check` are flattened to one line and cut to 100 characters, and nothing in a hive is treated as an instruction.

### Example with two repos

You are changing a library (`lib`, at `plan`) and its caller (`cli`, at `work`):

```
$ waggle hives plan new feat
$ waggle hives plan add feat --hive lib --task T-020
$ waggle hives plan add feat --hive cli --title "use the new API" --depends-on s1
$ waggle hives plan check feat
Plan feat, checked at 2026-11-20T09:00:00Z. Read only: nothing is written to any hive; ordering is reported, not enforced.
  s1  lib  T-020  library change  [open]
  s2  cli  (no task yet)  use the new API  [draft]
Ordering violations (0):
$ waggle hives plan brief feat s2 --out ~/briefs/cli-new-api.md
```

You then land that brief in `cli` as an ordinary task, and the library change through its own pull request. Each goes through its own gate. If a `cli` task is claimed before `lib`'s T-020 is done, `check` says so.

### Registry questions this answers

These are RFC-0004 open questions 1 to 3, answered only as far as phases 1 and 2 need. The RFC is unchanged.

1. **Registry location and format:** a JSON file in the user config directory, as above. A daemon registry is not built. Plans sit next to it.
2. **Hub placement:** the hub lives in none of the hives. The registry and the plans are in your user config area.
3. **How the hub reads a hive:** from the local checkout and its local claim refs, without fetching. Run `git fetch` in a hive for newer claims. A remote-only entry is unreachable (stale in a plan).

## What comes next

Phase 3 would run a hive's own claim and dispatch commands for you, hive by hive, in the order a plan gives. It does not exist and is not scheduled. See the RFC for the trust model and the open questions.
