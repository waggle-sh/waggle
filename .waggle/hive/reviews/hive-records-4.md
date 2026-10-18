<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: hive-records-4 (PR #35, head 77b1fb3)

Verdict: pass
Risk-Tier: 1

## Intent Summary

Hive-layer records batch: appends D-134 to D-137 to DECISIONS.md via `waggle decide`, adds five guard reviews (RFC-0003-A2 sonnet and fable, T-006-json, T-023a sonnet and fable) with T-023a evidence under `media/T-023a/`, folds status and carry items into the T-003, T-006, T-023 and T-024 briefs, and refreshes `handoff/queen.md`. Only `.waggle/hive/**` changes (50 files, 1757 insertions, 20 deletions); blast radius is steering text only.

## Checks

1. Paths: every changed path is under `.waggle/hive/`. Nothing in product paths or `.waggle/control/**`.
2. Gate and tests: `waggle gate decisions --base origin/main --head HEAD` returned `ok, appended D-134, D-135, D-136, D-137`. `go test -race ./...` passes on all packages. DECISIONS.md diff is a pure append after the D-133 block; no earlier line altered.
3. Decision content: D-134 to D-137 match the stated rulings (kind, tickets, flag lists, allowlist as phase C precondition, added non-test lines, compaction off until phase E). Kinds are policy, technical, policy, policy as ruled.
4. Date: header says 2026-10-18 (decide stamps UTC), body says "on 2026-10-17" (Keeper's local day). Acceptable. The header is machine-stamped by the append-only writer and cannot be hand-edited without breaking the gate, and the body preserves the ruling date. Not a defect; do not alter either.
5. Secret scan: no `sk-ant`, `ghp_`, `github_pat_`, `gho_`/`ghs_`, JWTs, AKIA, `op://` or OAuth tokens in any added file. Env-like strings (`ANTHROPIC_API_KEY`, `GITHUB_TOKEN`, `OP_SERVICE_ACCOUNT_TOKEN`) are test-fixture dummy values ("k", "x"). `fable-worker-bash-env.txt` has PATH and TMPDIR redacted and no credentials. Thinking `signature` blobs in the jsonl are opaque model output, not secrets. Home paths are `/Users/example` only (scratchpad and plugin paths).
6. Canary: `probe-repo/CLAUDE.local.md` is tracked (force-added) and contains only `Local rule: the local canary is CANARY-LOCAL-4410.` The sibling files are a canary CLAUDE.md, a hooks-only settings.json and two `date >>` hook scripts writing to the scratchpad. Pure fixture, no sensitive content. Nested under `media/`, so it is not loaded by sessions at repo root.
7. Briefs: T-003 adds one `Open` line (A1 Approved-by placeholder), T-006 flips status to Done (#33, #34) and fixes the loop to `printf` and the lease to `testdata/**`, T-023 records phase A Done at `12a4372` (#32, 469 lines, D-136), phase B waiting on T-012 B and T-021 B (matches its Depends on line), the D-137 precondition, D-135, and the reviews carry list, T-024 records A2 merged at `44a5e51` (#31) and the fable builder hazards. All status or fold only, accurate to the facts given.
8. Names and dashes: no `architect` in new text outside review quotes (see N2). No em or en dashes in D-134 to D-137, briefs, handoff or review prose.

## Findings (non-blocking)

- N1 (privacy, advisory): `media/T-023a/fable-run2-tools.jsonl` contains a captured read of `~/.gitconfig`, exposing `27632860+paulternate@users.noreply.github.com` (the git author, already in history, so within the stated allowance) and the path `/Users/example/Repos/znap`. Not a secret. Worth redacting before the repo goes public (merge queue note says revisit when public).
- N2 (style, advisory): `RFC-0003-A2-sonnet.md` lines quote the word `architect` ("No `architect`... introduced", "architecture text") as a negative check, not as a role reference. `probe-control.jsonl` contains `vercel:ai-architect`, a captured plugin agent name. Neither introduces the retired role.
- N3 (style, advisory): en and em dashes appear in 5 raw evidence files (`claude-help.txt` CLI help text, and model output in four jsonl captures). These are verbatim captures, not authored text; acceptable as evidence.
- N4 (staleness): `handoff/queen.md` still says D-134 to D-137 are "NOT yet recorded". It becomes stale on merge; the queen refreshes it after the merge batch per the existing habit.

## Intent Packet

- What: records D-134 to D-137, five guard reviews, T-023a evidence, and four brief status folds; hive only.
- Why: close the T-023 A and RFC-0003 A2 loops in the ledger and carry review findings into the briefs that next touch the code.
- Risk: Tier 1, text-only; gate decisions ok, tests green, no secrets, N1 redaction advisable before going public.
