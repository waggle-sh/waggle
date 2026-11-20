<!-- SPDX-License-Identifier: Apache-2.0 -->
# Configuration

Waggle reads one file, `.waggle/control/config.yaml`. `waggle init --gate-only` seeds a small one; a full `waggle init` seeds a larger one. The file is capped at 64 KiB and uses a small YAML subset: mappings and scalars.

A key the running build does not know exits 3, so a config written for a newer build fails closed on an older one. The gate reads `gate.protected_paths` and `gate.approvers` from the **PR's base commit**, never from the head, so a PR cannot widen its own permissions.

## Gate-only install

```yaml
version: 1

waggle:
  pin: <commit-sha>

forge:
  provider: github
  remote: origin

gate:
  protected_paths: ""
  approvers: ""
```

## Keys

### `version`

Required. Must be `1`.

### `waggle`

`waggle.pin` is the Waggle build the repo runs: a release tag or a 40-hex commit. Change it only through `waggle upgrade`.

### `forge`

| Key | Values | Default | Meaning |
| :--- | :--- | :--- | :--- |
| `provider` | `github`, `none` | `github` | The forge adapter. The gate and `waggle init` support `github`; `none` is accepted for a repo with no forge. |
| `remote` | a git remote name | empty (local only) | Where claims are pushed and the gate fetches them from. Must match `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`. |
| `comb_mode` | `direct`, `relay` | `direct` | Claim ref mode. `relay` is accepted but not implemented in v0.1. |
| `merge_strategy` | `up_to_date`, `native_queue` | `up_to_date` | `native_queue` only makes `waggle doctor` skip the up-to-date check and print an informational finding. It does not make the gate run in a merge queue. |

### `leases`

| Key | Range | Default | Meaning |
| :--- | :--- | :--- | :--- |
| `default_ttl_minutes` | 1 to 10080 (7 days) | 120 | Claim lifetime when `--ttl` is not given. |
| `max_directories_per_claim` | 1 to 64 | 5 | Most directories one claim may lease. |

### `gate`

| Key | Format | Meaning |
| :--- | :--- | :--- |
| `protected_paths` | comma separated repo-relative paths, up to 64 | Paths whose change fails the check unless a listed approver's approval and a re-run pass it, or an admin bypasses. No wildcards, no absolute paths, no `..`. Raise-only: it adds to the hardcoded control paths and never removes one. |
| `approvers` | comma separated GitHub logins without `@`, up to 32 | Who may approve a change to a protected path. The first entry is the Head Keeper, the only approver who can pass a change under `.waggle/hive/`. Compared with ASCII case folding. |

The gate spec has the full rules: [protected paths](/docs/gate/#5-risk-tiers-and-protected-paths) and [approvers](/docs/gate/#6-approvers).

## Full install keys

A full `waggle init` (not `--gate-only`) also writes sections the hive workflow uses: `hive` (profile and delivery mode), `daemon`, `runtimes`, `context`, `pace` and `cell`. The gate does not consume them. The build parses them for well-formedness and otherwise ignores them, so they are not part of the gate's contract.

## Local preferences

`.waggle/control/local.yaml` is gitignored and holds one person's preferences, such as `keeper_id` (the default claim owner) and the runtime. It is never read by the gate.
