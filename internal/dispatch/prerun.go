package dispatch

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/runtime"
)

// checkIgnored requires, before anything is written, that the runtime
// directory and the usage ledger are gitignored (D-175, item 6). The Keeper's
// own ignore file is switched off, so only the repo's lines count.
func checkIgnored(ctx context.Context, pg *Git) error {
	for _, p := range []string{layout.Worktrees + "/", layout.Usage} {
		if _, err := pg.Run(ctx, "-c", "core.excludesFile=/dev/null", "check-ignore", "-q", p); err != nil {
			return failf(ExitEnv, "%s is not gitignored; add the line %q to .gitignore (waggle upgrade adds it)", p, p)
		}
	}
	return nil
}

// resolveIdentity is the worker's git identity (D-171, item 19): the repo's
// own user.name and user.email under the pinned config (D-160), else an
// explicit read of the Keeper's global config. Blank is exit 3.
func resolveIdentity(ctx context.Context, g *Git) (runtime.GitIdentity, error) {
	get := func(key string) string {
		for _, c := range []struct {
			g     *Git
			scope string
		}{{g.pinned(), "--local"}, {g, "--global"}} {
			if out, err := c.g.Run(ctx, "config", c.scope, key); err == nil && strings.TrimSpace(string(out)) != "" {
				return strings.TrimSpace(string(out))
			}
		}
		return ""
	}
	id := runtime.GitIdentity{Name: get("user.name"), Email: get("user.email")}
	if id.Name == "" || id.Email == "" {
		return id, failf(ExitEnv, "no git identity: set user.name and user.email in the repo or the global git config")
	}
	return id, nil
}

// hostRecord is host-files.json: the host file hashes after the last clean run.
type hostRecord struct {
	Run   string            `json:"run"`
	Files map[string]string `json:"files"`
}

const hostRecordName = "host-files.json"

// hostBase drops the per-worktree keys, which move between runs on purpose
// (each run's own check covers them).
func hostBase(m map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range m {
		if !strings.HasPrefix(k, "worktrees/") {
			out[k] = v
		}
	}
	return out
}

// configKeys is the hash of the common config's parsed entries without
// branch.<name>.*, which every push with -u rewrites (D-189). A symlink, a
// missing file or an unparsable one is its file state, so each still differs.
func configKeys(ctx context.Context, pg *Git, common string) string {
	p := filepath.Join(common, "config")
	if st := fileState(p); st == "absent" || st == "other" {
		return st
	}
	out, err := pg.Run(ctx, "config", "--file", p, "--list", "-z")
	if err != nil {
		return "unparsable"
	}
	var keep []string
	for _, e := range strings.Split(string(out), "\x00") {
		if k, _, _ := strings.Cut(e, "\n"); e != "" && !strings.HasPrefix(k, "branch.") {
			keep = append(keep, e)
		}
	}
	sum := sha256.Sum256([]byte(strings.Join(keep, "\x00")))
	return hex.EncodeToString(sum[:])
}

// recordFiles is the map the record stores and compares: the host file hashes
// without per-worktree keys, the raw config replaced by its parsed keys.
func recordFiles(files map[string]string, cfg string) map[string]string {
	m := hostBase(files)
	delete(m, "config")
	m["config.keys"] = cfg
	return m
}

// checkHostRecord compares the files git trusts with the snapshot the last run
// verified (item 1). The record is agent-writable, so this stops only an
// escaped process that does not also rewrite it (see not_enforced).
func checkHostRecord(ctx context.Context, pg *Git, dir, common string) error {
	data, err := readUpTo(filepath.Join(dir, hostRecordName), maxHash)
	var rec hostRecord
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err == nil:
		err = json.Unmarshal(data, &rec)
	}
	if err != nil {
		return failf(ExitEnv, "%s is unreadable (%v); inspect the host files, then delete it", hostRecordName, err)
	}
	if keys := diffKeys(rec.Files, recordFiles(hostFiles(common, ""), configKeys(ctx, pg, common))); len(keys) > 0 {
		return failf(ExitEnv, "host files changed since run %s: %s; inspect them, then delete %s", rec.Run, strings.Join(keys, ", "), hostRecordName)
	}
	return nil
}

// saveHostRecord stores the snapshot the checks verified, never a fresh hash:
// a process that wrote after the compare, or that made the run fail the host
// check, then differs from the record at the next dispatch (fable B1, opus N1).
func saveHostRecord(dir string, before snap, run string) error {
	b, err := json.Marshal(hostRecord{Run: run, Files: recordFiles(before.files, before.cfg)})
	if err == nil {
		err = writeFile(dir, hostRecordName, b)
	}
	return err
}
