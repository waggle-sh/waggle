<!-- SPDX-License-Identifier: MIT-0 -->
# T-025 phase D: dco evidence

- Phase D merged as beb4fab (#157); its own signed commit passed dco.
- Ruleset main (id 25310890) required checks read back after the change: product-ci, verify-leases-and-tiers, dco. Other rules, bypass actors and enforcement unchanged (D-339).
- Probe PR #159, branch dco-probe from beb4fab: one empty unsigned commit failed dco (Actions run 40568828454).
- After git rebase --signoff, head 4380386 passed dco. The PR was closed unmerged.
