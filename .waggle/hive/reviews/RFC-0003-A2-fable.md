<!-- SPDX-License-Identifier: MIT-0 -->
# Guard review: RFC-0003 Amendment 2 (PR #31, `rfc-0003-a2` @ `2b7b745` on main `b1e4a3a`)

Verdict: pass
Risk-Tier: 3
Guard: fable (second guard, third model, D-100). Independent of the sonnet review.

## Intent Summary

A2 rewrites four passages of `docs/RFC-0003.md` (2.1 Cell, 3.2 item 1, 3.5 zero credentials and egress, 4 item 3) and adds the A2 appendix entry, replacing "a dev container mounts the tree" with the approved `CellRuntime` design (D-101): OS sandbox by default (Seatbelt on macOS, bubblewrap on Linux), a Docker-compatible container backend for CI, a per-Cell host egress proxy, `.git` read-only with `waggled` committing on the host (D-121), an allowlisted environment and empty home, per-target enforcement probes with refusal instead of degradation, and the `cell.inference_credential: passthrough` opt-in (D-120). Blast radius: a spec document only; no code. But it is the text `waggle doctor`, the T-024 builder and both guards will measure "enforced" against, so it is reviewed at Tier 3 by content. Every security claim in the diff is either matched by the T-024 brief or verified against the technology on this host; two Seatbelt claims were tested empirically (evidence under `.waggle/hive/media/RFC-0003-A2/fable-*.txt`).

## Blocking Issues (Must-Fix Only)

None. No claim in the amended text is false, and the one internal tension (2.1 refusal rule vs the 3.5 passthrough exception) is declared in the document as an exception, so it is not a contradiction.

## Claims checked, by line

- Line 19 (2.1): ro repository, rw lease, zero credentials, egress only through the proxy, per-target probe, refusal when unmet. Matches brief sections 1, 3, 8.
- Line 54 (3.2): bubblewrap and container use `ro` bind of the worktree and `rw` binds on leases: correct (later binds override; hard links across vfsmounts fail with EXDEV; symlinks resolve into the ro mount). Seatbelt path-based split and `(deny file-link)`: verified on macOS 27.0.1 (`fable-seatbelt-file-rules.txt`): write in lease ok, write outside denied, hard link denied, write through a symlink pointing outside denied, rename into lease denied. `.git` read-only in every backend and host-side commit: D-121.
- Line 78 (3.5): allowlist-built env, empty home, proxy-injected credential, passthrough opt-in reported not enforced: brief 3.1, 3.2, 6.3, Q-T024-1 / D-120. `.waggle/control/run.sock` matches `internal/layout/layout.go` and `config.yaml`.
- Line 79 (3.5): `(deny network*)` verified to close outbound TCP, loopback TCP, UDP, raw sockets, DNS and non-allowlisted Unix sockets while a single `path-literal` Unix-socket allow works (`fable-seatbelt-network-rules.txt`). So Docker socket, ssh-agent and gpg-agent sockets are closed on Seatbelt; bwrap `--unshare-all` and `--network none` close them by namespace. Proxy rules (exact names, IP literal and loopback/link-local/private refusal, pinned resolution, logging) match brief 6.1. "Limits where data goes, not what the agent sends" matches 6.4.
- Line 89 (4 item 3): three backends, probe per target, doctor prints it, no unsandboxed fallback: brief 1, 8.
- Line 115 (A2 summary): consistent with the sections above and with D-030, D-101, D-120, D-121.

## Passthrough exception (D-120)

Narrow: one named credential (the agent's inference login token), one config key, one target relaxed (zero credentials), still reported by `waggle doctor`, and no other target changes. The RFC states the consequences that matter at spec level: the target is reported as not enforced and it is the single exception to the refusal rule. The sentence "egress lockdown limits where data goes, not what the agent sends to its inference provider" already covers the residual: the token, like anything the worker can read, can reach the provider through the inference channel, and the proxy still confines it to allowlisted hosts.

## Non-blocking (recommend before merge, none is a security falsehood)

1. Line 19 states the refusal rule without the exception that line 78 carves out of it. Add "(except the passthrough opt-in of 3.5)" so 2.1, the section people cite, is not read as absolute. In this hive every Cell will start with the Keeper's login token inside.
2. Line 89 "Every backend provides `git worktree` provisioning": the brief puts worktree creation out of scope (T-023) and backends receive `Spec.Worktree`. Say `waggled` provisions the worktree and every backend runs it.
3. Line 79 "allowlisted package registry hosts": the source is `cell.egress_allow`, which admits any exact host name (Tier 3 config), and the private-address refusal applies even to allowlisted names, so a registry on an RFC 1918 address cannot be reached in v0.1. Worth one clause, since that is the usual enterprise case.
4. Stale neighbors outside A2's sections: line 21 says `.waggle/run.sock` (line 78 now says `.waggle/control/run.sock`); lines 17 and 87 still list `.devcontainer/`, which D-030 removed. A1 precedent allows leaving them, but line 21 sits in the same 2.1 list A2 edited.
5. The RFC says nothing about Seatbelt's residual surfaces that the brief names (shared host loopback and Mach namespace, same uid, deprecated `sandbox-exec`). "Stricter isolation" for the container backend implies it; one clause in 3.2 or 3.5 would make it explicit.

## Hazards for the T-024 builder (found while testing; not RFC defects)

- macOS `sun_path` is 104 bytes. A 105-byte socket path failed to bind during this review. The per-Cell proxy socket and `run.sock` under deep checkout paths will hit this; phase D and F should use a short runtime dir and probe the length.
- The Seatbelt profile needs `(allow network-outbound (remote ip "localhost:<port>"))` for the agent to reach the forwarder; the brief lists only the `local` bind rule, and loopback TCP is denied by default (test [c]).
- The proxy must check the allowlist before resolving a name, or lookups of attacker-chosen names through the host resolver become a DNS side channel.
- bwrap: a read-only mount does not block `connect()` on a socket inode (the brief relies on this for its two ro-bound sockets), so `/run` or `/var/run` must not be in the ro system set or the Docker socket is reachable. Consider `--disable-userns` since no seccomp ships in v0.1.
- Default-mode Claude Code needs to believe it holds a credential while `*_KEY` and `*TOKEN*` env names are refused; `apiKeyHelper` in the Cell home's settings is the likely seam. Passthrough on macOS must materialize the keychain token into the Cell home, and refreshed tokens written there never reach the host keychain.
