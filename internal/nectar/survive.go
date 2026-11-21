// SPDX-License-Identifier: Apache-2.0

package nectar

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strings"
	"time"
)

var (
	supersedesLine = regexp.MustCompile(`(?mi)^Waggle-Supersedes:[^\n]*`)
	taskID         = regexp.MustCompile(`T-[0-9]+`)
	blameHeader    = regexp.MustCompile(`^([0-9a-f]{40}(?:[0-9a-f]{24})?) [0-9]+ [0-9]+(?: [0-9]+)?$`)
)

// noteSupersedes records the tasks a bound commit, or a commit of its merge,
// names in Waggle-Supersedes (RFC-0005 4.2). Only a later commit exempts a unit.
func (s *scorer) noteSupersedes(i int, c *commit, sides []commit) {
	for _, body := range append([]string{c.body}, sideBodies(sides)...) {
		for _, line := range supersedesLine.FindAllString(body, -1) {
			for _, id := range taskID.FindAllString(strings.ToUpper(line), -1) {
				s.supers[id] = i
			}
		}
	}
}

// survive applies survival rule 2 to a unit that reached maturity (RFC-0005
// 4.2): the share of its added lines still on main at the newest first-parent
// commit at or before matures_at, by git blame -w -M -C. Below survive_ratio the
// unit is dropped. A pure deletion and a unit a later commit supersedes use rule
// 1 only.
func (s *scorer) survive(ctx context.Context, u *bu, matures time.Time) error {
	last := u.merges[len(u.merges)-1]
	lastIdx := s.onLine[last.oid]
	if u.lc.added == 0 {
		return nil
	}
	if at, ok := s.supers[u.Task]; ok && at > lastIdx {
		u.Reasons = append(u.Reasons, "superseded")
		return nil
	}
	e := &s.hist[lastIdx]
	for i := len(s.hist) - 1; i > lastIdx; i-- {
		if !s.hist[i].at.After(matures) {
			e = &s.hist[i]
			break
		}
	}
	alive, hit := s.cache.survival(u.key, e.oid)
	if !hit {
		var err error
		if alive, err = s.aliveLines(ctx, u, last, e); err != nil {
			return err
		}
		s.cache.putSurvival(u.key, e.oid, alive)
	}
	alive = min(alive, u.lc.added)
	ratio := Milli((alive*1000 + u.lc.added/2) / u.lc.added)
	u.AliveRatio = &ratio
	if alive*1000 < u.lc.added*int64(rulesV1.surviveRatio) {
		u.State = StateDropped
		u.clawShare = 0
		u.Reasons = append(u.Reasons, "alive_below_ratio")
	}
	return nil
}

// aliveLines counts the unit's lines that blame at e still credits to one of the
// unit's commits. A file renamed since the unit's last merge is followed commit
// by commit along main, as blame follows it, and so is every file changed since,
// because blame -C can credit a line moved there; a file gone from e counts none.
func (s *scorer) aliveLines(ctx context.Context, u *bu, last, e *commit) (int64, error) {
	set := map[string]bool{}
	for _, p := range u.lc.paths {
		for j := s.onLine[last.oid] + 1; j <= e.idx; j++ {
			m, err := s.stepRenames(ctx, j)
			if err != nil {
				return 0, err
			}
			if to, ok := m[p]; ok {
				p = to
			}
		}
		set[p] = true
	}
	if last.oid != e.oid {
		// Binary, lockfile and generated files at e are left out, as countedRows
		// leaves them out of the unit's added lines.
		rows, err := s.g.numstatRange(ctx, "--diff-filter=AM", last.oid, e.oid)
		if err != nil {
			return 0, err
		}
		gen, err := s.g.generated(ctx, e.oid, rows, rulesV1.generatedAttr)
		if err != nil {
			return 0, err
		}
		for _, row := range rows {
			if !row.binary && !rulesV1.isLockfile(row.path) && !gen[row.path] {
				set[row.path] = true
			}
		}
	}
	paths := make([]string, 0, len(set))
	for p := range set {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	specs := make([]string, len(paths))
	for i, p := range paths {
		specs[i] = e.oid + ":" + p
	}
	blobs, err := s.g.batch(ctx, specs)
	if err != nil {
		return 0, err
	}
	ignore, err := s.ignoreFile(ctx, e)
	if err != nil {
		return 0, err
	}
	var alive int64
	for i, p := range paths {
		if blobs[i] == nil {
			continue
		}
		by, err := s.blame(ctx, e.oid, p, ignore)
		if err != nil {
			return 0, err
		}
		for o := range u.all {
			alive += by[o]
		}
	}
	return alive, nil
}

// blame counts, per commit, the lines of path at oid that blame credits to it.
// A result is reused by every unit that survives at the same commit.
func (s *scorer) blame(ctx context.Context, oid, path, ignore string) (map[string]int64, error) {
	key := oid + ":" + path
	if by, ok := s.blames[key]; ok {
		return by, nil
	}
	by, err := s.g.blameCount(ctx, oid, path, ignore)
	if err != nil {
		return nil, err
	}
	s.blames[key] = by
	return by, nil
}

// stepRenames maps the paths commit hist[i] renamed, against its first parent.
func (s *scorer) stepRenames(ctx context.Context, i int) (map[string]string, error) {
	if m, ok := s.steps[i]; ok {
		return m, nil
	}
	c := &s.hist[i]
	m := map[string]string{}
	if len(c.parents) > 0 {
		var err error
		if m, err = s.g.renames(ctx, c.parents[0], c.oid); err != nil {
			return nil, err
		}
	}
	s.steps[i] = m
	return m, nil
}

// renames maps each path renamed between from and to to its new path.
func (g repo) renames(ctx context.Context, from, to string) (map[string]string, error) {
	out := map[string]string{}
	b, err := g.run(ctx, nil, "diff-tree", "-r", "-z", "-M", "--name-status", "--no-commit-id", from, to)
	if err != nil {
		return nil, err
	}
	f := strings.Split(strings.TrimSuffix(string(b), "\x00"), "\x00")
	for i := 0; i < len(f); i++ {
		switch {
		case strings.HasPrefix(f[i], "R") && i+2 < len(f):
			out[f[i+1]] = f[i+2]
			i += 2
		case f[i] != "":
			i++
		}
	}
	return out, nil
}

// blameCount counts the lines of path at oid per credited commit. ignore is a
// blame ignore-revs file or "". The empty --ignore-revs-file= first clears any
// list the clone's own blame.ignoreRevsFile set.
func (g repo) blameCount(ctx context.Context, oid, path, ignore string) (map[string]int64, error) {
	args := []string{"blame", "-w", "-M", "-C", "--porcelain", "--ignore-revs-file="}
	if ignore != "" {
		args = append(args, "--ignore-revs-file", ignore)
	}
	b, err := g.run(ctx, nil, append(args, oid, "--", path)...)
	if err != nil {
		return nil, err
	}
	by := map[string]int64{}
	cur := ""
	for _, line := range strings.Split(string(b), "\n") {
		if m := blameHeader.FindStringSubmatch(line); m != nil {
			cur = m[1]
		} else if strings.HasPrefix(line, "\t") {
			by[cur]++
		}
	}
	return by, nil
}

// ignoreFile writes the .git-blame-ignore-revs committed at e, keeping only
// commits the repository has, to a temporary file. "" means none. Scorer-owned
// files are removed when Score returns.
func (s *scorer) ignoreFile(ctx context.Context, e *commit) (string, error) {
	if p, ok := s.ignore[e.oid]; ok {
		return p, nil
	}
	s.ignore[e.oid] = ""
	blobs, err := s.g.batch(ctx, []string{e.oid + ":.git-blame-ignore-revs"})
	if err != nil || blobs[0] == nil {
		return "", err
	}
	var revs []string
	for _, line := range strings.Split(string(blobs[0]), "\n") {
		line, _, _ = strings.Cut(line, "#")
		if line = strings.TrimSpace(line); len(line) == s.oidLen && oidPattern.MatchString(line) && !slices.Contains(revs, line) {
			revs = append(revs, line)
		}
	}
	if len(revs) == 0 {
		return "", nil
	}
	have, err := s.g.batch(ctx, revs)
	if err != nil {
		return "", err
	}
	var keep []string
	for i, r := range revs {
		if have[i] != nil {
			keep = append(keep, r)
		}
	}
	if len(keep) == 0 {
		return "", nil
	}
	f, err := os.CreateTemp("", "waggle-nectar-ignore-*")
	if err != nil {
		return "", fmt.Errorf("nectar: %w", err)
	}
	defer f.Close()
	if _, err := f.WriteString(strings.Join(keep, "\n") + "\n"); err != nil {
		return "", fmt.Errorf("nectar: %w", err)
	}
	s.ignore[e.oid] = f.Name()
	return f.Name(), nil
}

func (s *scorer) cleanup() {
	for _, p := range s.ignore {
		if p != "" {
			_ = os.Remove(p)
		}
	}
}

// unitKey names a unit by its task and its credited merges, which fixes the
// facts derived from history alone.
func unitKey(u *bu) string {
	oids := make([]string, len(u.merges))
	for i, m := range u.merges {
		oids[i] = m.oid
	}
	return u.Task + "|" + strings.Join(oids, ",")
}
