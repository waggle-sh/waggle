<!-- SPDX-License-Identifier: MIT-0 -->
# Contributing to Waggle

This file is for people building Waggle itself. If you use Waggle in your own repo, the README's Roles and Terms sections are what you need; nothing here applies to your hive.

## 1. Two layers in this repo

- **Product:** Waggle itself. `cmd/`, `internal/` (including `internal/scaffold/templates/`, which is what every other hive receives), `go.mod`, `LICENSE`, `README.md`, `docs/`, and the product CI workflow. This is what ships.
- **Hive** (layer sense): our own installation of Waggle, used to build the product (own honey, defined in the README). `.waggle/`, `.github/workflows/waggle-gate.yml`, and the marked import block in `CLAUDE.md`. Nothing in the hive ships.

## 2. Rules

- Our hive runs a pinned, known-good Waggle build recorded as `waggle.pin` in `.waggle/control/config.yaml`, never the code under construction. Changing the pin is a deliberate upgrade logged with `waggle decide` (D-041).
- Template changes reach our hive only through `waggle upgrade`, run from the target build, which becomes the new pin when the hive PR merges. Do not hand-edit hive copies of templates (D-042, D-063).
- Worker leases never include `.waggle/`; hive files change only through waggle commands or the Head Keeper. Waggle enforces this for every hive (D-043).
- A change that touches both layers is split: a product ticket first, then a hive upgrade.
- `DECISIONS.md` records how our hive decided. Anything users rely on also lands in the RFC or product docs (D-045).

## 3. Conventions only this repo uses

These are not part of Waggle's schema.

- Every brief carries a `Layer: product | hive` line.
- CODEOWNERS protects `internal/scaffold/templates/` (D-034) and `README.md` (D-065), both via T-015.
- `CONTRIBUTING.md` is not Tier 2 (D-058).

## 4. Words

The README's Roles and Terms sections define every Waggle role and term. Keeping them current:

- Any brief that introduces a role or term updates the README Roles or Terms section in the same PR. The guard checks this: a new capitalized or code-formatted term in a brief with no README entry is a must-fix.
- Each entry is one or two plain sentences. No decision IDs in the README; sources stay in the brief that adds the entry.
- The scout owns both sections. Changes take a Full gate, and in this repo CODEOWNERS requires Head Keeper review (T-015, D-065).
- Retired names stay listed here with a pointer to the current term, so old commits, decisions, and review records stay readable.
- Product design that users rely on also lands in the RFC or product docs (D-045); `DECISIONS.md` records how our hive decided, not how Waggle works.
- Future, not now: if the Terms list passes about 60 entries, it may split into `docs/GLOSSARY.md`, linked from the README (D-065).

Naming rule (D-073): bee terms are used only for concepts Waggle invents (Hive, Comb, Cell, roles). Things users already know keep their standard industry words (forge, PR, CI, commit, ref), and so do precise mechanisms (Gate, Lease, Claim, Risk Tier, Intent Packet).

No em dashes or en dashes in code, docs, or commits.

## 5. Retired names

| Retired | Use instead | Retired by |
| :--- | :--- | :--- |
| Master Keeper | Head Keeper | D-031 |
| Controller | Head Keeper | D-031 |
| architect (before T-020) | scout | D-074 |
| scout (before T-020) | forager | D-074 |
| forager (before T-020) | builder | D-074 |
| drone | none; retired | D-073 |

In records written before T-020, read architect as Scout, scout as Forager, and forager as Builder.
