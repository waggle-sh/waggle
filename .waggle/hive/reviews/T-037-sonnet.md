<!-- waggle-review v1 verdict=pass head=8f9745c506c60ad822ea24f3d9ca8d09d56ccf6d -->

Risk-Tier: 2

**Intent Summary:** Fix round 1 for T-037, a docs-only change. It touches `README.md`, `docs/gate.md` and `CONTRIBUTING.md` and changes no Go. It does the following:
- Corrects the `init --gate-only` pin-refusal sentence. The code requires a clean commit stamp or a release tag, and never checks that the commit was pushed.
- Replaces the README claim shorthand with the literal usage lines from `claim.go`.
- Adds sources to limits L6, L7 and L11.
- Makes the README check 1 wording match the gate spec.
- Fixes the Lane entry and the naming-rule list in CONTRIBUTING.
- Adds a missing blank line before `## Documentation` in the README.

Blast radius is limited to reader-facing prose. No gate behavior changes.

**Evidence:**
- `git diff --name-only 52450ef 8f9745c` lists only the three leased files.
- The diff against `52450ef` contains only the brief's edits, and each replacement matches the brief's text.
- The two usage lines in the README match `cmd/waggle/claim.go:178` and `:243` byte for byte.
- The `noRefReason` string quoted in `docs/gate.md` matches `internal/scaffold/init.go:162`.
- A Grep for U+2013 and U+2014 over the three files finds no matches.
- The new text adds no bee names, decision IDs or ticket IDs outside code spans. The only added hits are `T-<id>` and `waggle/T-<id>` forms, which are already allowed.

I did not run the build, vet, format or test commands. No Go changed, and the diff is prose only.

**Blocking Issues (Must-Fix Only):** None.

Verdict: pass