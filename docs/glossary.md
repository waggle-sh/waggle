<!-- SPDX-License-Identifier: Apache-2.0 -->
# Glossary

Waggle's own project records and some of its docs use bee words. This page gives each one a short plain definition and a link to the doc that uses it. The README and the claims spec use plain terms: task, issue, claim, paths, gate and approver.

- **Hive.** A repository with Waggle's full set of files installed, and the rules its agents work under. See [CONTRIBUTING.md](../CONTRIBUTING.md#4-words-in-this-repos-records).
- **Comb.** The shared coordination state, kept as git refs under `refs/waggle/`. Claims live there. See [docs/claims.md](claims.md).
- **Keeper.** A person who runs agents on their own machine and plan. See [docs/multi-keeper.md](multi-keeper.md).
- **Head Keeper.** The keeper with final say over a repo: the first approver in the gate config, who can approve changes to the steering files. See [docs/gate.md](gate.md#6-approvers).
- **Queen.** The coordinating agent a keeper talks to. It agrees a task with the keeper, writes the claim and starts the workers. See [CONTRIBUTING.md](../CONTRIBUTING.md#roles).
- **Scout.** The agent that plans ahead and writes task briefs. See [CONTRIBUTING.md](../CONTRIBUTING.md#roles).
- **Forager.** The agent that reproduces bugs, writes the failing test and maps which paths a task needs. See [CONTRIBUTING.md](../CONTRIBUTING.md#roles).
- **Builder.** The agent that implements a task inside its claim. See [CONTRIBUTING.md](../CONTRIBUTING.md#roles).
- **Guard.** The agent that reviews a change and ends with a pass or a must-fix. It is not the gate, which is the merge check. See [CONTRIBUTING.md](../CONTRIBUTING.md#roles).
- **Nectar.** A contributor's reputation score, earned from merged work that survives 30 days on `main` without being reverted. See [docs/nectar.md](nectar.md).
- **Quorum.** How a project picks what to build next, by quadratic voting where each person's weight is the square root of their Nectar. See [docs/quorum.md](quorum.md).
- **Waggle dance.** The dance a honey bee does to tell its hive where food is. The project name and its bee words come from it.
