# Review: T-023 phase A, guard 1 of 2 (sonnet) (branch t-023a, head 4916655)

Verdict: pass (re-review of 6cbaf04: pass, see "Re-review (6cbaf04)" at the end)

Risk-Tier: 3

Reviewed: branch t-023a, head 4916655, one commit on main 9015bc9, draft PR #32. Gate: full. Evidence in `.waggle/hive/media/T-023a/` (files prefixed `sonnet-`; the builder's files are untouched). Did not read the other guard's review.

Checks (sonnet-checks.txt): `go build ./...`, `go vet ./...`, `gofmt -l .` (empty), `go test -race -count=2 ./...` all pass. Diff touches `internal/runtime/` only (9 files), `go.mod` and `go.sum` unchanged, stdlib only, 0 em or en dashes in the diff. `os.Environ` appears once, in a comment.

## Intent Summary

Replaces `AgentRuntime.Run` with `Verify`, `Prepare` and `Collect` in `internal/runtime/`. `Prepare` builds the argv, env and stdin for one headless Claude Code session: print mode with stream-json, no setting sources, an inline D-110 permissions file (read, edit, write and the hive's Bash commands allowed; git push, gh, curl, wget, ssh and web tools denied; `dontAsk`), `--tools` limited to six built-ins, strict MCP, `--max-turns`, the stable prompt appended to the system prompt, the variable prompt on stdin, and an allowlisted env with a push tripwire. `Verify` checks `claude --help` for each security flag and `Prepare` refuses until it passes. `Collect` parses the stream-json output, enforces `max_tokens`, and returns usage, cost, final text and the end reason. Nothing calls any of it yet (the Cell seam and `dispatch.Run` are phases B and C), so the blast radius is the runtime package and its tests, but phase C will run agents with write access on this code, which is why it stays Tier 3.

## Blocking Issues

None.

## Findings (non-blocking; N1 is the one I would change before merge)

**N1. Bare `Write` and `Edit` allow rules let the worker write anywhere the Keeper can.** Verified live on Claude Code 2.1.302 (sonnet-writescope-bare.jsonl): with the builder's settings, a Write to an absolute path outside the worker's cwd succeeded, as did `../x`. With `Edit(./**)` and `Write(./**)` in place of the bare rules (sonnet-writescope-scoped.jsonl), the in-worktree write succeeded and both outside writes were denied ("don't ask mode"). D-110 says "allows the read, edit and write tools" and section 8 lists "writes outside the repo (not detected)" as not enforced, so this meets the letter of the spec. But the scoped form closes the easiest route (a prompt-injected Write to `~/.zshrc`, `~/.claude/`, `~/.ssh/config`, or the main checkout's `.git/hooks`) at the cost of two strings, and the host-state check in phase C only covers the main checkout's refs, status, config and hooks. Recommend the scoped form in this phase, or a Keeper ruling that the bare form stays. Bash can still write anywhere (see N2), so this narrows the exposure, it does not seal it. `Read` has the same shape (bare `Read` can open `~/.ssh`); `Read(./**)` would narrow it, with the usability cost that workers could not read files outside the worktree. Not tested.

**N2. The deny list is advisory; the allowlist and the post-run check are the controls.** `Bash(go test *)` is a prefix rule, so `go test -exec <prog>` and any test file the worker writes with Write can run arbitrary code, including curl and git push. The push tripwire (below) and phase C's checks are what catch that. The brief already says dispatch "reduces exposure and makes violations visible; it does not stop a determined agent". The `workerSettings` doc comment should not be read as a boundary.

**N3. `Final` truncation can flip a guard verdict (carry to phase D).** `capFinal` keeps the head and cuts the tail. A guard message over 256 KiB that quotes `Verdict: pass` early and ends with `Verdict: must-fix` returns a `Final` holding only the early pass line plus the marker (probe `spoof`, sonnet-probe-results.txt). Phase D's "last line matching" rule would then read `pass`. Nothing consumes `Final` yet, so this is not a phase A defect, but D-100 and AC9 need it closed. Cheapest fix now: add a `Truncated bool` to `RunResult` (the field is cheap and phase C and D both need it), and have phase D treat truncated as `no_verdict`. The cut can also land inside a UTF-8 rune (probe `utf8 valid after cut: false`).

**N4. One type mismatch drops a whole event.** `json.Unmarshal` into `streamEvent` fails the entire line on any mismatched field in the decoded set, so a `result` event with `"input_tokens": 12.0` or a numeric overflow is counted malformed and the session ends `error` even though it succeeded (probes `float usage`, `overflow-literal usage`). The real 2.1.302 stream uses integers (builder and my runs), so this is only a version-drift risk. Using `json.Number` or float64 fields and converting would degrade gracefully.

**N5. Smaller stream points.**
- A `result` event always replaces the per-message usage sum, even when the result carries no `usage` (probe `max_tokens then result` shows zero usage after a max_tokens stop with a bare result). Falling back to the sum when `ev.Usage == nil` is a one-liner.
- A negative `total_cost_usd` is passed through (`neg cost`); clamp or null it before the ledger.
- Assistant events with an empty `message.id` all merge into one message (`empty ids`). Claude Code always sets ids, so cosmetic.
- A result line over 4 MiB is dropped, so a very large final message ends `error` (probe `4MiB line then result`).
- `stop` is called synchronously from the read loop on `max_tokens`, and from an `AfterFunc` goroutine on cancel. Document that `stop` must be non-blocking and goroutine-safe. If `ctx` is already cancelled on entry, the `AfterFunc` goroutine can still be running `stop` after `Collect` returns; phase C's "Run returns only after every goroutine exited" must not rely on `Collect` alone.
- Claude Code exits 1 on `error_max_turns` (observed, sonnet-maxturns1.jsonl, `result` is null). Phase C must read the stream's end, not the exit code.

**N6. `Commands` validation is shallow but fails the right way.** Probe table in sonnet-probe-results.txt. Refused: `;`, `$(`, backtick, newline, tab, CR, NUL, `*`, parentheses, quotes, backslash, `|`, `&`, `<`, `>`, `${`, `2>&1`, NBSP, em space, U+2028, leading, trailing or double spaces (`git  push` is refused by the Fields-and-Join identity check), an empty entry, and a bare `curl`, `gh`, `ssh`, `wget`. Accepted: `GIT_DIR=x git push`, `git -C . push`, `git -c a=b push`, `env git push`, `sudo curl x`, `/usr/bin/curl x`, `./curl x`, `Curl x`, `bash -c curl`, `go test -exec sh ./...`, a full-width semicolon, zero-width characters inside `git push`, and `go test ~/x`, `{a,b}`, `!!`, `#x`. Only the first word is checked against the program list, and only `f[0:2]` against `git push`. This matters only if `WAGGLE.md` section 3 (Head Keeper, Tier 3, read from base) is hostile, and none of the accepted forms is shell injection (no operator reaches the rule). Even then those rules would not match `curl` under Claude Code's matcher, so the deny rules do not cover them. I would rather the doc say the check is a guard against typos than imply a policy.

**N7. Usability of `cmd *`.** `Bash(go test -race ./... *)` does not match `go test -race ./internal/runtime/...`, so workers can only run the exact `WAGGLE.md` strings plus trailing arguments. D-110 accepts "some runs end on a denied tool"; flag it for the first trial ticket.

**N8. Denylist gaps in `default_args`.** `default_args` is Keeper config (Tier 3 file), so this is hardening, not an exploit. Not refused, from the 2.1.302 `--help`: `--effort` (levels belong to T-012), `--fallback-model` (silently changes the model), `--betas`, `--permission-prompt-tool`, `--permission-prompts`, `--chrome`, `--ide`, `--file`, `--debug-file`, `--bare`, `--safe-mode`, `--restricted`, `--autocompact`, `--brief`, `--remote-control`, `--tmux`, `--json-schema`, `--include-hook-events`. Commander is case-sensitive, so `--MODEL` is just unknown. Refused and correct: `-rabc`, `-p`, `--model=x`, `--worktree=x`, and the full list of resume flags. The short-flag rule over-refuses anything single-dash containing c, p, r or w (for example a future `-w`), which is fine for fail-closed. Recommend adding at least `--effort` and `--fallback-model`.

**N9. Env and tripwire.** Variables that reach the worker, if set in the parent: `PATH HOME USER LOGNAME SHELL TMPDIR TERM TZ LANG LC_ALL LC_CTYPE ANTHROPIC_API_KEY ANTHROPIC_AUTH_TOKEN ANTHROPIC_BASE_URL`. Fixed: `CLAUDE_CODE_DISABLE_CLAUDE_MDS=1 CLAUDE_CODE_DISABLE_AUTO_MEMORY=1 ENABLE_CLAUDEAI_MCP_SERVERS=false CLAUDE_CODE_DISABLE_NONESSENTIAL_TRAFFIC=1 DISABLE_AUTO_COMPACT=1 GIT_TERMINAL_PROMPT=0 GIT_CONFIG_COUNT=3` with `GIT_CONFIG_KEY_n` and `GIT_CONFIG_VALUE_n` for n 0 to 2. Settings-driven: `BASH_MAX_OUTPUT_LENGTH`, `MAX_THINKING_TOKENS` (when the step is set). A parent holding `GITHUB_TOKEN GH_TOKEN SSH_AUTH_SOCK GIT_ASKPASS OP_SERVICE_ACCOUNT_TOKEN AWS_SECRET_ACCESS_KEY FOO BASH_ENV GIT_DIR GIT_CONFIG_GLOBAL GIT_CONFIG_COUNT=9 NODE_OPTIONS LD_PRELOAD HTTPS_PROXY CLAUDE_CONFIG_DIR CLAUDE_CODE_USE_BEDROCK` leaks none of them (TestProbeEnv). The tripwire works for https, ssh and scp-style remotes (tested with git, sonnet-checks.txt notes below) but not for `http://`, `git://` or a local path remote: `git push` to `../bare` succeeded under the exact env. Also, the rewritten host `waggle-push-blocked` is resolved as an ssh host name, not a dead scheme; it fails on DNS today. Proxy and CA variables (`HTTPS_PROXY`, `SSL_CERT_FILE`, `NODE_EXTRA_CA_CERTS`) are not passed, so a Keeper behind a proxy gets a failing worker (fails closed, noisy).

**N10. Verify covers flags, not the env names or the version.** `CLAUDE_CODE_DISABLE_CLAUDE_MDS`, `ENABLE_CLAUDEAI_MCP_SERVERS`, `DISABLE_AUTO_MEMORY` and `DISABLE_AUTO_COMPACT` cannot be checked from `--help`. A future Claude Code that renames one would silently drop that isolation. The builder's canary run proves they work on 2.1.302. Suggest phase C record `claude --version` in `result.json` and the checks add a tested-version note. `--max-turns` is hidden from help but I confirmed it is accepted (20 and 1 both ran) and that `--max-turns=1` ends with subtype `error_max_turns`, `is_error: true`, `result: null`, `terminal_reason: max_turns` (sonnet-maxturns1.jsonl).

**N11. Stable prompt on argv.** 100 KiB passes Linux (128 KiB per argument) and macOS, but Windows `CreateProcess` caps the full command line at 32,767 characters. Fine for v0.1 platforms; note it in the platform list. The settings JSON argument has no cap of its own (it grows with `Commands`).

**N12. Size.** 458 is the count of added non-test lines (numstat: launch 227, stream 198, runtime 22, claude 5, profile 5, policy 1). A literal D-089 reading ("`wc -l` of the non-test Go files the phase touches") gives 934 for the six touched files, which is over the 520 stop; the metric as used for phases that added files has been added lines. 458 is over the 400 target and under the 520 stop. I accept 458; the Keeper should confirm the metric, since the PR description must carry the count.

## Live evidence on Claude Code 2.1.302 (real model, haiku, tokens under $0.01)

Argv and env came from `Prepare` (sonnet-live_test.go.txt). With `Bash(go version)` and `Bash(go version *)` allowed and `dontAsk`:
- Allowed: `go version`, `go version -m /nonexistent`, `go version && go version` (sonnet-writescope-bare.jsonl).
- Denied, none of the m-files exist: `go version && touch`, `;`, `$(touch ...)`, backticks, `| tee`, `> file`, `||`, `&`, `--$(...)` and `go test -exec` (sonnet-live-compound.jsonl). So this version splits compound commands and requires every part to match, and treats command substitution and redirects as unmatched. A rule `Bash(go test *)` therefore does not allow `go test ./... && curl evil` (the `curl` part fails the allowlist and the deny rule), though it does allow `go test ./... -exec curl`, which is N2.
- Could not verify: newline-separated commands as a single call (the model collapsed it), behavior on other Claude Code versions, and the deny rules matching `git push` variants, since nothing allows them anyway.
- The init event lists tools `Bash Edit Glob Grep Read Write`, `mcp_servers: []`, and auth worked from the Keeper's login (`apiKeySource: none`, no key in env). Builder's canary run (probe-output.txt): repo Stop and SessionStart hooks did not fire, project and user CLAUDE.md did not reach the worker, and the control run proved the probe is valid. Built-in plugins (`cc-plugin-agents-md`, `cc-plugin-plugin-authoring`) still load; they ship inside Claude Code.

## Stream and boundary probes (sonnet-probe-results.txt)

- 4 MiB line then a result: counted malformed, drained, result parsed (done). Exactly 4,194,304 bytes accepted; one byte more malformed.
- Truncated JSON (with and without newline): one malformed, end `error`, usage from the sum.
- Negative usage clamps to 0; `int64` max clamps to 2^40 per field with no overflow; an assistant turn with 2^62 input stops once, `max_tokens`.
- Context exactly at `max_tokens` is fine; one over stops once. A second over-limit turn does not call `stop` again. A result after a max_tokens stop leaves the end `max_tokens`.
- Interleaved unknown events, arrays, bare strings, `null`: ignored or counted, never fatal. CRLF and a missing trailing newline parse.
- Final of 262,143 and 262,144 bytes is untouched; 262,145 is cut to exactly 262,144 including the marker.
- Context cancel gives `interrupted`, one `stop`.
- `nudge_tokens`: Prepare refuses 999, equal to `max_tokens`, `max_tokens` 0, negative, and `max_tokens` above 10,000,000; accepts 1000 with max 1001; `Settings` accepts 1000 (handoff 750) and 10,000,000 (7,500,000) and refuses 999, 10,000,001 and 2^62. No overflow path.
- Usage per assistant message: three events with one id count once (the last), as the real stream repeats the id per content block. Output tokens in assistant events are placeholders (builder's own capture: 2 on the message, 141 in the result), so the no-result fallback undercounts output; input and cache counts are accurate, which is what the `max_tokens` check uses.

## Rulings on the judgment calls

1. **`Verify` on the interface, `Prepare` refuses with `ErrUnverified`.** Accepted. It is the right way to make "fails closed" real, and the `--max-turns` gap is closed by observation (N10). The verified flag is per adapter instance, not per binary path or version.
2. **Settings inline as `--settings=<json>`.** Accepted. Works on 2.1.302 with `--setting-sources=` empty (both verified live). Visible in `ps`, but it holds only rules. The brief said "settings file in the run record"; phase C should still copy the JSON to `settings.json` there for audit.
3. **`Commands` into `Bash(cmd)` and `Bash(cmd *)` with refusals.** Accepted as written; see N6 and N7. The trailing-wildcard rule is needed for flags but widens D-110 slightly (any trailing argument).
4. **`DISABLE_AUTO_COMPACT=1` and `RunResult.Compacted`.** Within the builder's authority: T-021 S5 says compaction is disabled "where the adapter can do so", and T-023 section 10.1 says "Decide `DISABLE_AUTO_COMPACT` here" and "record in the result whether a session compacted". My view: still card the Keeper, because D-117 lets cutover happen without phase E. Until the handoff loop exists nothing acts on `HandoffTokens`, so a long builder session now runs to `max_tokens` (300000 in the shipped config) and ends exit 5, where before it would have compacted and continued. If the model window is below 300000, the API will fail first and the end is `error`, not `max_tokens`. I could not verify that the env var takes effect (needs a 150k-token session), only that the stream parser flags `compact_boundary`. The card should state the consequence and the order: either phase E first, or accept that long tickets need a rerun.
5. **Extended `default_args` refusals, denylist.** Accepted for Keeper-authored config in a Tier 3 file; see N8 for the gaps. An allowlist (only `--verbose`, `--effort` under a policy) would be stricter but breaks the "add your own flags" intent of `default_args`. Add `--effort` and `--fallback-model` now.
6. **`--flag=value` everywhere, 100 KiB stable cap.** Accepted. Verified: `--setting-sources=` (empty) and the variadic `--tools=Bash,Edit,...` both work. See N11.
7. **Usage summed per assistant message when no result event.** Accepted as a fallback. Known undercount of output tokens (placeholders), and see N5 on the result without usage.
8. **`CLAUDE_CONFIG_DIR` not passed.** Accepted for D-111 (the Keeper's existing login, default location): with the default dir the login worked on this machine. A Keeper with a non-default config dir gets an unauthenticated worker, which fails loudly. Add it to the allowlist only if a Keeper needs it, as a Keeper ruling because the dir also holds user skills and agents.

## Acceptance check

AC1 pass (build, vet, gofmt, race x2). AC2 pass (lease, stdlib, `go.mod`). AC3 pass at 458 added lines (N12). AC6 pass: no dropped variable reaches the worker (TestProbeEnv and builder test 3), and `Prepare` fails closed without `Verify` (N10 caveat on env names). AC7 pass on the builder's canary evidence, not independently re-run. AC4, AC5, AC8, AC9, AC10 belong to later phases. Tests 1 to 5 of section 11 are covered by `launch_test.go` and `stream_test.go`, none skipped.

## Merge notes

Tier 3, Head Keeper approval required (D-100). Pass from this guard does not replace the second guard. I would ask for N1 (two strings) and the `--effort` and `--fallback-model` refusals before merge; everything else can ride to phase B or C.

## Re-review (6cbaf04)

Verdict: pass. Risk-Tier: 3. Head 6cbaf04, one commit on 4916655. Evidence: `.waggle/hive/media/T-023a/sonnet-rereview-spec.json` and `sonnet-rereview-gobuild.jsonl`.

**Claims, verified by diff and tests**
1. `--restricted` is in argv (after `--strict-mcp-config`) and in `helpTokens`, so `Verify` requires it; `TestVerify` and `TestPrepareArgv` assert both. Pass.
2. Allow rules are `Edit(./**)` and `Write(./**)`; the test checks both scoped rules and fails on bare `"Edit"` or `"Write"`. Pass.
3. `Collect` keeps `Message` as `json.RawMessage` and decodes it only when `Type == "assistant"`. A non-object assistant message still counts malformed (tested). `TestCollectPermissionDenied` uses a verbatim line from `fable-run3-restricted.jsonl`, counts it as valid, and takes its session id. Pass.
4. `default_args` now refuses `--effort`, `--fallback-model`, `--remote-control`, `--environment`, `--chrome`, `--ide`, and any entry whose flag part does not start with `-` (so `high`, `fix the bug` and the empty string). Tests cover the `=value` forms and bare words. Pass. A bare value after a legal flag (`--debug high`) is refused, which is the intended trade.

**Gates.** `go build`, `go vet`, `gofmt -l` clean; `go test -race -count=2 ./...` all ok. Lease: `git diff 9015bc9 6cbaf04` touches only `internal/runtime/`. No en or em dashes in `internal/runtime`. Added non-test lines: 469 (stop 520, target 400), counted as added lines per the Keeper's D-089 ruling.

**Merge-clean.** Merged 6cbaf04 into main ce66c63 in a scratch worktree: automatic merge, no conflicts; build, vet, gofmt and `go test -race ./...` green on the merged tree. Scratch worktree removed.

**Live, Claude Code 2.1.302, haiku, about $0.002 total.** Argv from `Prepare` with the new code.
- N1 re-probe: Write to a path outside the working directory was denied with `system/permission_denied`, reason "--restricted: path outside the working directory", and the file was not created. Write to `./inside.txt` and Edit of `a.go` succeeded. (That run's jsonl was overwritten by the next run; the result is as stated.)
- Build cache: with `--restricted` and rules `go build`, `go vet`, `go test`, the worker ran `go build ./...`, `go vet ./...`, `go test ./...` and `go test -count=1 ./...` in a scratch module; all succeeded (sonnet-rereview-gobuild.jsonl). The Go build cache is `~/Library/Caches/go-build`, outside the working directory, and `HOME` and `TMPDIR` are in the passed env. `--restricted` confines the file tools, not Bash subprocesses, so the WAGGLE.md commands work. Not blocking.
- The model first appended `; echo` to commands, which `dontAsk` denied as a compound. That is the known compound behavior, not a `--restricted` effect.
- Not verified: a cold Go cache (the cache already existed), other Claude Code versions.

**Blocking issues.** None.

## Intent Packet (merge commit)

T-023 phase A adds the headless Claude Code adapter in `internal/runtime/` (`launch.go`, `stream.go`, small edits to `claude.go`, `policy.go`, `profile.go`, `runtime.go`). `Prepare` builds a print-mode stream-json session with no setting sources, strict MCP, `--restricted`, a six-tool allowlist, file tools scoped to `./**`, a git/gh/curl/ssh deny list, an allowlisted env and a stable prompt appended. `Collect` parses the stream into tokens, cost, end state and session id under a token cap. `Verify` checks the pinned binary's help for every required flag, and `Prepare` fails closed without it. The fix pass closed N1 (Write and Edit outside the worktree), the `permission_denied` parse bug, and the missing `--effort` and `--fallback-model` refusals. Blast radius: one package, no callers outside tests yet. Tier 3 because it sets worker permissions (D-100): Head Keeper approval is required and this pass does not replace guard 2. Carried to phase B or C: copy `settings.json` into the run record, the compaction and handoff consequence (ruling 4, phase E), and the `go test -exec` widening (N2). Size 469 added non-test lines.
