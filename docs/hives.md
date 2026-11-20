<!-- SPDX-License-Identifier: Apache-2.0 -->
# Hives status: one read-only view across your hives

> **Roadmap feature, phase 1 of 4.** This is the first step of the multi-repo queen in [RFC-0004](RFC-0004-multi-repo-queen.md). Only the read-only view exists. Planning across hives (phase 2) and coordinated work (phase 3) do not, and nothing here writes to a hive.

If you keep several hives, one per repository, `waggle hives status` answers "what is open, claimed, blocked or waiting on me?" across all of them, with one merged list of decision cards.

## What it never does

- It writes nothing to any hive: no claim, no release, no dispatch, no decision, no file, no ref.
- It does not fetch. Each hive is read from its own checkout and its local claim refs. Run `git fetch` in a hive yourself if you want newer claims.
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
- `path`: the absolute path of a local checkout. Phase 1 reads checkouts only.
- `repo`: an `owner/repo` label. An entry with only a `repo` is listed as unreachable, because phase 1 cannot read a remote.
- `level`: `view` (the default), `plan` or `work`. It is the ceiling you set for that hive. Phase 1 only reads, whatever the level.

A file that is not valid JSON, has an unknown field, repeats a name or holds a relative path is refused.

## Running it

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

## What comes next

Phase 2 would keep cross-repo plans in the hub, outside the hives. Phase 3 would run a hive's own claim and dispatch commands for you, hive by hive. Neither exists, and neither is scheduled. See the RFC for the trust model and the open questions.
