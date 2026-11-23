<!-- SPDX-License-Identifier: Apache-2.0 -->
# Agent guide

This file is for agents working in this repository.

## Install

If you do not have the `waggle` CLI, follow [INSTALL-AGENT.md](INSTALL-AGENT.md). This repository already has the gate, so skip `waggle init --gate-only` unless you are setting up a different repository.

## How to work here

Read `CONTRIBUTING.md` and `.waggle/control/WAGGLE.md`.

A queen or keeper writes the claim before work starts. Stay inside the claim's directories and branches. Every commit must name its ticket (`T-<id>:` in the subject, a `Waggle-Ticket:` trailer, or a `t-<id>` / `waggle/T-<id>` branch) and include a DCO sign-off.

Do not edit `.waggle/hive` or `.waggle/control` unless the ticket and current rules allow it.
