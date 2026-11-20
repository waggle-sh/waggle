// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// `waggle hives plan` (T-064, phase 2 of docs/hives.md). Plans live in the
// Keeper's user config area, outside every hive, and are never read from a
// repository. The only files this command writes are plan files in the plans
// directory and, on request, a draft brief at a path outside every hive. It
// writes nothing into a hive: no claim, ref, file, decision, dispatch or
// fetch. Exit codes follow `waggle hives status`: 0 done, 1 refused, 2 usage.

const (
	plansEnv     = "WAGGLE_HIVES_PLANS"
	maxPlanSize  = 1 << 20
	planUsageMsg = `Usage:
  waggle hives plan new   <plan>
  waggle hives plan add   <plan> --hive <name> [--task T-nnn] [--title <text>] [--depends-on s1,s2]
  waggle hives plan show  <plan> [--json]
  waggle hives plan list
  waggle hives plan check <plan> [--json]
  waggle hives plan brief <plan> <step> [--out <file outside every hive>]
  waggle hives plan rm    <plan> [--step <id>]
Every command takes --registry <file> and --plans-dir <dir>.
Plans are advisory and kept outside the hives. Nothing is written to any hive.`
)

type planOpts struct {
	registry string
	plansDir string
	asJSON   bool
	out      string
	hive     string
	task     string
	title    string
	deps     string
	step     string
}

func runHivesPlan(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, planUsageMsg)
		if len(args) > 0 {
			return 0
		}
		return 2
	}
	sub := args[0]
	var want int // positional arguments after the subcommand
	switch sub {
	case "new", "add", "show", "check", "rm":
		want = 1
	case "brief":
		want = 2
	case "list":
		want = 0
	default:
		fmt.Fprintf(stderr, "waggle hives plan: unknown subcommand %q\n", sub)
		return 2
	}
	name := "hives plan " + sub
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, planUsageMsg); fs.PrintDefaults() }
	var o planOpts
	fs.StringVar(&o.registry, "registry", "", "Registry file (default: $"+registryEnv+", else <user config dir>/waggle/hives.json)")
	fs.StringVar(&o.plansDir, "plans-dir", "", "Plans directory (default: $"+plansEnv+", else plans/ next to the registry)")
	switch sub {
	case "show", "check":
		fs.BoolVar(&o.asJSON, "json", false, "Print as JSON")
	case "brief":
		fs.StringVar(&o.out, "out", "", "Write the draft to this file, which must be outside every hive (default: stdout)")
	case "add":
		fs.StringVar(&o.hive, "hive", "", "Hive name from the registry (level plan or work)")
		fs.StringVar(&o.task, "task", "", "The hive's ticket id, when the task exists")
		fs.StringVar(&o.title, "title", "", "The Keeper's title for the step")
		fs.StringVar(&o.deps, "depends-on", "", "Comma-separated earlier step ids")
	case "rm":
		fs.StringVar(&o.step, "step", "", "Remove this step instead of the whole plan")
	}
	pos, err := parseInterspersed(fs, args[1:])
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if len(pos) != want {
		fmt.Fprintf(stderr, "waggle %s: expected %d argument(s), got %d\n", name, want, len(pos))
		return 2
	}
	refuse := func(err error) int {
		fmt.Fprintf(stderr, "waggle %s: refused: %v\n", name, err)
		return 1
	}

	regPath, err := registryPath(o.registry, getenv)
	if err != nil {
		return refuse(err)
	}
	dir := o.plansDir
	if dir == "" {
		dir = getenv(plansEnv)
	}
	if dir == "" {
		dir = filepath.Join(filepath.Dir(regPath), "plans")
	}
	if !filepath.IsAbs(dir) {
		return refuse(errors.New("the plans directory must be an absolute path"))
	}

	if sub == "list" {
		names, err := listPlans(dir)
		if err != nil {
			return refuse(err)
		}
		for _, n := range names {
			fmt.Fprintln(stdout, n)
		}
		return 0
	}
	plan := pos[0]
	if !hiveNameRe.MatchString(plan) {
		return refuse(fmt.Errorf("plan name %q must match %s", plan, hiveNameRe))
	}
	file := filepath.Join(dir, plan+".json")

	switch sub {
	case "new":
		p, err := NewPlan(plan)
		if err != nil {
			return refuse(err)
		}
		if err := writePlan(dir, file, p, true); err != nil {
			return refuse(err)
		}
		fmt.Fprintf(stdout, "Created plan %s at %s\n", plan, file)
		return 0
	case "rm":
		p, err := readPlan(file, plan)
		if err != nil {
			return refuse(err)
		}
		if o.step == "" {
			if err := os.Remove(file); err != nil {
				return refuse(err)
			}
			fmt.Fprintf(stdout, "Removed plan %s\n", plan)
			return 0
		}
		p, err = RemoveStep(p, o.step)
		if err != nil {
			return refuse(err)
		}
		if err := writePlan(dir, file, p, false); err != nil {
			return refuse(err)
		}
		fmt.Fprintf(stdout, "Removed step %s from plan %s\n", o.step, plan)
		return 0
	}

	p, err := readPlan(file, plan)
	if err != nil {
		return refuse(err)
	}
	if sub == "show" {
		if o.asJSON {
			stdout.Write(RenderPlanJSON(p))
		} else {
			stdout.Write(RenderPlanText(p))
		}
		return 0
	}
	reg, err := readRegistry(regPath)
	if err != nil {
		return refuse(err)
	}

	switch sub {
	case "add":
		var deps []string
		if o.deps != "" {
			deps = strings.Split(o.deps, ",")
		}
		np, step, err := AddStep(p, reg, o.hive, o.task, o.title, deps)
		if err != nil {
			return refuse(err)
		}
		if err := writePlan(dir, file, np, false); err != nil {
			return refuse(err)
		}
		fmt.Fprintf(stdout, "Added step %s (%s %s) to plan %s\n", step.ID, step.Hive, strings.TrimSpace(stepLabel(step.Task, step.Title)), plan)
		return 0
	case "check":
		c := checkPlanLive(p, reg)
		if o.asJSON {
			stdout.Write(RenderCheckJSON(c))
		} else {
			stdout.Write(RenderCheckText(c))
		}
		return 0
	case "brief":
		i := findStep(p, pos[1])
		if i < 0 {
			return refuse(fmt.Errorf("no step %q in plan %q", pos[1], plan))
		}
		if _, err := planEntry(reg, p.Steps[i].Hive); err != nil {
			return refuse(err)
		}
		if o.out != "" {
			if err := outsideEveryHive(o.out, reg); err != nil {
				return refuse(err)
			}
		}
		text, err := RenderBrief(p, checkPlanLive(p, reg), pos[1])
		if err != nil {
			return refuse(err)
		}
		if o.out == "" {
			stdout.Write(text)
			return 0
		}
		if err := writeNewFile(o.out, text); err != nil {
			return refuse(err)
		}
		fmt.Fprintf(stdout, "Wrote draft brief for step %s to %s. Land it in hive %s through that hive's normal path.\n", pos[1], o.out, p.Steps[i].Hive)
		return 0
	}
	return 2
}

// checkPlanLive reads each hive a plan names, through the phase 1 reader, and
// checks the plan against what it found. Hives the registry no longer allows
// for planning are not read at all.
func checkPlanLive(p Plan, reg []RegistryEntry) PlanCheck {
	views := map[string]HiveView{}
	for _, s := range p.Steps {
		if _, done := views[s.Hive]; done {
			continue
		}
		if e, err := planEntry(reg, s.Hive); err == nil {
			views[s.Hive] = viewHive(e, statusClock())
		}
	}
	return CheckPlan(p, reg, views, statusClock())
}

// parseInterspersed parses flags that may come before, between or after the
// positional arguments, and returns the positional ones.
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return pos, nil
		}
		pos = append(pos, fs.Arg(0))
		args = fs.Args()[1:]
	}
}

func listPlans(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range ents {
		if n, ok := strings.CutSuffix(e.Name(), ".json"); ok && e.Type().IsRegular() && hiveNameRe.MatchString(n) {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	return names, nil
}

// readPlan reads and strictly parses a plan file. A symlink is refused.
func readPlan(file, name string) (Plan, error) {
	fi, err := os.Lstat(file)
	if errors.Is(err, os.ErrNotExist) {
		return Plan{}, fmt.Errorf("no plan %q at %s", name, file)
	}
	if err != nil {
		return Plan{}, err
	}
	if fi.Mode()&os.ModeSymlink != 0 {
		return Plan{}, fmt.Errorf("%s is a symlink", file)
	}
	data, err := readCapped(file, maxPlanSize)
	if err != nil {
		return Plan{}, err
	}
	return ParsePlan(data, name)
}

// writePlan writes a plan file atomically with mode 0600: a temp file in the
// plans directory, synced, then renamed over the plan (or, for a new plan,
// linked into place so an existing plan is never replaced).
func writePlan(dir, file string, p Plan, create bool) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".plan-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(RenderPlanFile(p)); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if create {
		if err := os.Link(tmp.Name(), file); err != nil {
			if errors.Is(err, os.ErrExist) {
				return fmt.Errorf("plan %q already exists at %s", p.Name, file)
			}
			return err
		}
		return nil
	}
	return os.Rename(tmp.Name(), file)
}

// outsideEveryHive refuses an output path that is inside any registered hive
// or inside any git work tree. The directory must already exist.
func outsideEveryHive(out string, reg []RegistryEntry) error {
	if !filepath.IsAbs(out) {
		return errors.New("--out must be an absolute path")
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(out))
	if err != nil {
		return fmt.Errorf("--out directory: %v", err)
	}
	target := filepath.Join(dir, filepath.Base(out))
	for _, e := range reg {
		if e.Path == "" {
			continue
		}
		root, err := filepath.EvalSymlinks(e.Path)
		if err != nil {
			root = e.Path
		}
		if rel, err := filepath.Rel(root, target); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return fmt.Errorf("--out is inside hive %q; a draft brief must be written outside every hive", e.Name)
		}
	}
	if _, err := repoRoot(dir); err == nil {
		return errors.New("--out is inside a git work tree; a draft brief must be written outside every hive")
	}
	return nil
}

// writeNewFile creates a new file with mode 0600 and refuses to overwrite.
func writeNewFile(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}
