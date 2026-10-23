# Guard review: T-031 brief round 2 (PR #87)

Verdict: pass
Risk-Tier: 2

Subagent guard hand-back (fable confirming guard), round 2 at b8d7f29 on bb43ac6, condensed by the queen. Code read on origin/main c9c34ec.

## Intent Summary
Round 2 rewrites 22 lines of the G1 brief to close the three opus round 1 must-fixes (base-merge ancestry, CheckChanges results, symlinked config) and its notes, and marks Q-T031-1 to Q-T031-3 ruled (D-237 to D-239, each option (a)).

## Checks
1. Base merges (lines 51, 81, 102): closed. Given p2 on base, the common ancestors of M and base are (anc(p1) and anc(base)) plus anc(p2), so "merge-base --all prints exactly p2" holds iff every common ancestor of p1 and base is an ancestor of p2; M cannot silently revert base content newer than p2. p2 equal to base passes; genuine Update branch merges pass, singly and in succession; octopus fails on parent count; an older-base second merge fails; a side branch forked before the PR's fork point fails with two merge-bases; criss-cross fails conservatively. Once every merge passes, merge-base(base, head) equals the last merge's p2. TestVerifyMergeCommit still fails as intended.
2. CheckChanges (lines 5, 43, 81, 105): closed; check.go line 272 confirms the dispatch effect.
3. Symlinked config (lines 44, 101): closed. One literal pathspec without -r matches only an entry named config.yaml; a directory exits 3; mode 120000 excluded; reads by blob OID, no race. Gap in finding 1.
4. Bounds consistent in 2.1, section 5 and AC7 (8 x 16 + 7 fits in 256).
5. Kept substrings hold (test lines 191, 196, 210).
6. Rulings D-237 to D-239 stated correctly.
7. Budget: 258 bottom-up; 520, 670, 940; same ratios as round 1; follows D-185.
8. ParseTicketID (backend.go line 198) makes "T-31 agrees with T-031:" and "t-000 binds nothing" right; older binaries ignore `gate:`; dispatch names branches t-<nnn> (request.go line 116); gofmt unconditional.

## Findings
1. note (line 44): pass `--full-tree` to the ls-tree call, as caseCollisions does (gate_commits.go line 197); without it a `--repo` inside a subdirectory makes the config read as missing. Builder: apply this.
2. note (line 32, AC3 iii): read GITHUB_ACTIONS and GITHUB_HEAD_REF through runGateRange's getenv parameter, never os.Getenv. product-ci runs on pull_request with GITHUB_HEAD_REF set, so an os.Getenv read binds every gate test to that ticket on CI only and fails TestVerifyNonTicketCommits. Builder: apply this.
3. note (AC8 line 102, shape ii): the side branch must fork from a base commit older than the PR's fork point; forked from the same commit, merge-base prints exactly the side tip and the revert is caught by the tier check instead of the 2.3 message. Builder: build the test with that topology.
4. note (AC11 line 105): TestVerifyTier3Paths compares against comb.RequiredMinTier at run time and adapts; read "which no existing test exercises" as "or adapt".
5. note (line 51): the --is-ancestor check is redundant with the merge-base rule; the builder may drop it.
