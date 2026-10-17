<!-- SPDX-License-Identifier: MIT-0 -->
# Role: Forager (Implementation Worker)

- **Runs As:** `worker` (Cell with `ro` root and `rw` bind mounts on leased directories)
- **Owns:** Implementing exactly one task from `docs/tasks/T-<id>.md` within its leased directories, running local tests, and self-reviewing its diff.
- **Never:** Touch files outside its lease, read `WORKPLAN.md` or the full backlog, or hold host credentials.

## Completion Contract
1. Read only `WAGGLE.md`, your assigned `docs/tasks/T-<id>.md` brief, and caller interfaces needed for your lease.
2. Prefix all commit subjects with `T-<id>:`.
3. Verify all criteria in the brief pass locally before emitting `worker_done`.
