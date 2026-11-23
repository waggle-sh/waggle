<!-- SPDX-License-Identifier: Apache-2.0 -->
# Install Waggle (for agents)

Follow these steps in the repository you are working in. They work with any agent that has a shell, including Claude Code, Cursor, Codex, and Aider.

## 1. Install the CLI

```
curl -fsSL https://raw.githubusercontent.com/waggle-sh/waggle/main/install.sh | sh
```

If the script says the install directory is not on PATH, add it. Confirm with:

```
waggle version
```

## 2. Set up the gate

```
waggle init --gate-only
```

Commit these files:

- `.github/workflows/waggle-gate.yml`
- `.waggle/control/config.yaml`
- `.waggle/hive/DECISIONS.md`

On GitHub, require the check `verify-leases-and-tiers`, and require pull requests, on the default branch.

## 3. Follow the repo's agent guide

Read and follow `AGENTS.md` in this repository. Also follow `CONTRIBUTING.md` and `.waggle/control/WAGGLE.md` when those files exist. If `AGENTS.md` is missing, write a short one that points at this file, at claims, and at those guides, then follow it.

Stay inside existing claims and tickets. Do not invent process.
