<!-- waggle-review v1 verdict=pass head=d30d31adb0c2e98b9c7898e1c895802ef1aa867a -->

Risk-Tier: 3

**Intent Summary:** This is fix round 1 on the T-047 build (`e0cea3e` to `d30d31a`). It changes `README.md` and `cmd/waggle/setup_doc_test.go` only, and `docs/setup.md` is untouched. `tagProblems` now takes the README text. It reads the captured URL group of the prompt regex and requires it to equal the pinned `refs/tags/<tag>` URL. It reports a problem when the README holds the prompt text more than once. The planted rows are now full prompt lines, with new rows for a URL prefix, a URL suffix, a second prompt line (plain, indented and trailing-word) and no prompt. The README warning now opens "docs/setup.md is instructions…". The bare link line became `See [L11](docs/gate.md#10-limits-and-known-bypasses).`. Blast radius is the README text and one drift test. No product code, fenced command, stop rule or MUST NOT line changed.

**Verification:**
- `gofmt -l .` printed nothing.
- I could not run `go test -race` or `go vet`: the sandbox denied both in this session. The test logic below is from reading the code, so it is unconfirmed by a run.
- `README.md` holds the prompt text once (line 48), so the real README should pass the new count check and the equality check.
- I traced each planted row by hand against the code:
  - URL prefix and URL suffix fail the `url != want` comparison. Only the captured group is compared, so a wrapped or extended URL no longer passes.
  - Second prompt, indented and trailing-word rows trip the `strings.Count > 1` check, which counts the text and not regex matches.
  - Indented-only, trailing-word-only and no-prompt rows give "no setup prompt line".
  - The second-tag row returns early with "exactly one tag".
  - The asset-version and branch-URL rows each fail for their own reason, and the test now asserts that reason with `strings.Contains`.
- The lease holds: the diff touches only `README.md` and `cmd/waggle/setup_doc_test.go`.
- Changes 2 and 3 match section 18 exactly:
  - The link line is on the line directly after the warning.
  - The README-only warning opening is applied, and the rest of the warning is verbatim.
- No threat-bar finding: nothing in the diff lets a faithful reader or agent harm a repo, leak a credential or switch on the gate without a person. The README prompt URL is now checked for equality, which narrows the earlier gap.

**Blocking Issues (Must-Fix Only):** None found. Please have the queen run `go test -race ./...` to confirm, since I could not.

Verdict: pass