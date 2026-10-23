The review is written to `.waggle/hive/reviews/T-032.md` and is the only new file in the worktree. Nothing is committed and no probe files were made.

**Outcome.** T-032 at f5f86c3 passes. Build, vet and gofmt are clean, and the full race-enabled suite passes in every package. Added non-test lines total 635, under the 800 target, so the planned cut does not apply.

**What the diff does.** It adds `waggle claim` and `waggle release` over new `ClaimTask` and `ReleaseTask` entry points with a version 1 JSON schema and an error-code map. `waggle dispatch release` becomes a wrapper that prints the same lines as before. Protected paths from the base config now refuse covered directories at every lease write, in dispatch before the claim, and in the post-run check. The default base is resolved fully qualified, closing the tag-shadowing hole for dispatch inputs, the policy read, the merged check and `offBranch`.

**What I verified against the must-fix list.**
- The changed files match the lease rule exactly. Every file the rule forbids has an empty diff.
- Every ref write goes through `AcquireLease` and every delete through `ReleaseLease`.
- Policy is read only at the base OID through `blobAt`, which rejects symlinks, trees, oversize and invalid files with exit 3. Working-tree and other-branch entries are proven not to apply.
- The policy is raise-only. Stored claims are not re-validated, and a pre-policy claim still reads, lists and blocks overlaps. `CheckChanges` and `CheckChangesPolicy` are untouched.
- A covered directory is refused before any claim write, also with an injected backend.
- The merged check still requires a real second-parent merge, and a merged `waggle/T-<nnn>` cannot stand in for an unmerged `t-<nnn>`.
- JSON mode prints exactly one object with `error.exit` equal to the process exit. Text mode strips control, format and separator characters from every ref, remote or config value.
- No new test calls `Run`, `sweep()` or sends a signal. New strings contain none of the banned words.

**Notes, none blocking.** The CLI's own usage errors report an empty task even when the argument parses. A cancellation during a git call surfaces as exit 3 rather than 1, as in dispatch today. `waggle dispatch release` now also accepts the `waggle/T-<nnn>` branch form. Once the hive pin moves past this merge, every dispatch run fails with exit 3 if the base config is invalid, which the queen's handoff should mention.

Verdict: pass