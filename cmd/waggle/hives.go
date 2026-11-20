// SPDX-License-Identifier: Apache-2.0

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/waggle-sh/waggle/internal/decisions"
	"github.com/waggle-sh/waggle/internal/layout"
)

// `waggle hives status` (T-064, phase 1 of docs/hives.md). It reads; it never
// writes. It does not fetch, claim, release, dispatch or append to any log in
// any hive, and no registry level changes that. Claims come from
// loadStatus, the code `waggle status` runs, with the remote sync switched off.

const (
	registryEnv     = "WAGGLE_HIVES_REGISTRY"
	maxRegistrySize = 1 << 20
	maxReadSize     = 8 << 20
	maxTasks        = 5000
)

// runHives implements `waggle hives`. Exit codes: 0 report printed (even with
// unreachable hives, which the report names), 1 registry missing or invalid,
// 2 usage.
func runHives(args []string, stdout, stderr io.Writer, getenv func(string) string) int {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" || args[0] == "help" {
		fmt.Fprintln(stderr, "Usage: waggle hives status [--registry <file>] [--json]")
		if len(args) > 0 {
			return 0
		}
		return 2
	}
	if args[0] != "status" {
		fmt.Fprintf(stderr, "waggle hives: unknown subcommand %q\n", args[0])
		return 2
	}
	fs := flag.NewFlagSet("hives status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() {
		fmt.Fprintln(stderr, "Usage: waggle hives status [--registry <file>] [--json]")
		fmt.Fprintln(stderr, "Read only. What is open, claimed, blocked or waiting on the Keeper across the hives in the registry.")
		fs.PrintDefaults()
	}
	registry := fs.String("registry", "", "Registry file (default: $"+registryEnv+", else <user config dir>/waggle/hives.json)")
	asJSON := fs.Bool("json", false, "Print the report as JSON")
	if err := fs.Parse(args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "waggle hives status: unexpected argument %q\n", fs.Arg(0))
		return 2
	}

	path := *registry
	if path == "" {
		path = getenv(registryEnv)
	}
	if path == "" {
		dir, err := os.UserConfigDir()
		if err != nil {
			fmt.Fprintf(stderr, "waggle hives status: refused: no registry given and no user config dir: %v\n", err)
			return 1
		}
		path = filepath.Join(dir, "waggle", "hives.json")
	}
	entries, err := readRegistry(path)
	if err != nil {
		fmt.Fprintf(stderr, "waggle hives status: refused: %v\n", err)
		return 1
	}

	views := make([]HiveView, 0, len(entries))
	for _, e := range entries {
		in, err := readHive(e)
		if err != nil {
			views = append(views, UnreachableView(e, err.Error()))
			continue
		}
		views = append(views, BuildHiveView(e, in, statusClock()))
	}
	report := BuildReport(views)
	if *asJSON {
		stdout.Write(RenderHivesJSON(report))
	} else {
		stdout.Write(RenderHivesText(report))
	}
	return 0
}

// readRegistry reads the Keeper's registry file. It is a user file outside any
// hive, so a repository cannot add itself to it (RFC-0004 section 5.3).
func readRegistry(path string) ([]RegistryEntry, error) {
	data, err := readCapped(path, maxRegistrySize)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("no registry at %s; create it as {\"hives\":[{\"name\":\"<name>\",\"path\":\"<absolute path>\"}]} (see docs/hives.md)", path)
		}
		return nil, err
	}
	return ParseRegistry(data)
}

// readCapped reads a regular file of at most max bytes and refuses symlinks
// to anything else.
func readCapped(path string, max int64) ([]byte, error) {
	fi, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		return nil, fmt.Errorf("%s is not a regular file", path)
	}
	if fi.Size() > max {
		return nil, fmt.Errorf("%s is larger than %d bytes", path, max)
	}
	return os.ReadFile(path)
}

// readHive reads one home hive's claims (through loadStatus), tasks and
// decision log. Any failure makes the hive unreachable in the view.
func readHive(e RegistryEntry) (HiveInput, error) {
	var in HiveInput
	if e.Path == "" {
		return in, errors.New("no local checkout: phase 1 reads checkouts only")
	}
	root, err := repoRoot(e.Path)
	if err != nil {
		return in, fmt.Errorf("not a git work tree: %s", e.Path)
	}
	d, fault := loadStatus(root, true)
	if fault != nil {
		return in, fmt.Errorf("%s", fault.msg)
	}
	in.Leases = d.Leases
	if d.Malformed != nil {
		in.Malformed = len(d.Malformed.Claims)
	}
	in.Tasks = readTasks(filepath.Join(root, layout.TasksDir))
	if data, err := readCapped(filepath.Join(root, layout.Decisions), maxReadSize); err == nil {
		if in.Decisions, err = decisions.Parse(data); err != nil {
			in.DecisionErr = err.Error()
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		in.DecisionErr = err.Error()
	}
	return in, nil
}

var (
	taskFileRe   = regexp.MustCompile(`^(T-\d+)\.md$`)
	ticketTextRe = regexp.MustCompile(`T-\d+`)
	doneWords    = []string{"done", "merged", "cancel", "superseded", "dropped", "closed", "rejected", "obsolete"}
)

// readTasks reads the briefs under dir. A missing directory is no tasks; a
// brief that cannot be read is skipped, since it cannot be classified.
func readTasks(dir string) []Task {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var tasks []Task
	for _, ent := range ents {
		if len(tasks) >= maxTasks || !ent.Type().IsRegular() || !taskFileRe.MatchString(ent.Name()) {
			continue
		}
		data, err := readCapped(filepath.Join(dir, ent.Name()), maxReadSize)
		if err != nil {
			continue
		}
		if t, ok := ParseTask(ent.Name(), data); ok {
			tasks = append(tasks, t)
		}
	}
	return tasks
}

// ParseTask reduces a brief to its id (from the file name), title (the first
// heading), done state (the Status line) and dependencies (the Depends on
// line). Only the first 200 lines are scanned. It is pure.
func ParseTask(name string, data []byte) (Task, bool) {
	m := taskFileRe.FindStringSubmatch(name)
	if m == nil {
		return Task{}, false
	}
	t := Task{ID: canonTicket(m[1])}
	for i, line := range strings.Split(string(data), "\n") {
		if i >= 200 {
			break
		}
		line = strings.TrimSpace(line)
		switch {
		case t.Title == "" && strings.HasPrefix(line, "# "):
			title := strings.TrimPrefix(line, "# ")
			if _, rest, ok := strings.Cut(title, ":"); ok {
				title = rest
			}
			t.Title = strings.TrimSpace(title)
		case strings.HasPrefix(line, "- **Status:**"):
			status := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(line, "- **Status:**")))
			for _, w := range doneWords {
				if strings.HasPrefix(status, w) {
					t.Done = true
				}
			}
		case strings.HasPrefix(line, "- **Depends on:**"):
			for _, id := range ticketTextRe.FindAllString(strings.TrimPrefix(line, "- **Depends on:**"), -1) {
				t.DependsOn = append(t.DependsOn, canonTicket(id))
			}
		}
	}
	return t, true
}
