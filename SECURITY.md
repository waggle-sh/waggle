<!-- SPDX-License-Identifier: Apache-2.0 -->
# Security policy

## Reporting a vulnerability

Please report vulnerabilities privately through GitHub private vulnerability reporting: open the repository's **Security** tab and choose **Report a vulnerability**. Do not open a public issue, pull request or discussion for a suspected vulnerability.

Include the Waggle version or Action ref, what you did, what you expected and what happened. A minimal reproduction helps most.

## Supported versions

Waggle is pre-1.0. Only the latest release (currently the v0.1 line) and `main` receive security fixes. Older releases are not patched; upgrade to the latest release.

## What to expect

- We aim to acknowledge a report within 5 working days.
- We aim to give an initial assessment within 14 days.
- Fixes are developed privately and released with an advisory. We will credit you if you want credit.
- Waggle is maintained by a small team, so these are targets, not guarantees.

## Known limitation: the required check can be forged (L11)

The merge gate's required check (`verify-leases-and-tiers`) can be forged. A workflow on any branch can declare `actions: write` and `checks: write`, delete the gate's failing run and post a passing check of the same name, so a pull request can merge without admin rights. This is documented as limit L11 in [docs/gate.md](docs/gate.md#10-limits-and-known-bypasses), together with the other known limits and bypasses (L1 to L10).

Because this is known and documented, a report that only restates L11 or another listed limit is not treated as a new vulnerability. Reports that show a new way around the gate, a way to widen a lease or claim, or a bypass the documented limits do not cover are in scope and welcome. If you are unsure, report it privately.

## Scope

In scope: the `waggle` CLI, the gate (`waggle gate` and the composite Action in `gate/`), the scaffold templates in `internal/scaffold/templates/`, and the release artifacts.

Out of scope: vulnerabilities in GitHub, in Go or in third-party agent tools, and findings that need an attacker to already hold admin rights on the repository.
