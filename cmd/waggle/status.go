package main

import (
	"bytes"
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
	"github.com/waggle-sh/waggle/internal/config"
	"github.com/waggle-sh/waggle/internal/layout"
	"github.com/waggle-sh/waggle/internal/scaffold"
)

// Injection points for tests, which never run in parallel in this package.
// socketExists is the whole daemon check for now: there is no socket protocol
// yet. When waggled lands, a ping over the socket replaces it.
var (
	statusClock  = time.Now
	socketExists = func(path string) bool { _, err := os.Lstat(path); return err == nil }
)

// usagePath is the per-machine token ledger. Phase B (T-021) adds
// layout.Usage; until then the path is derived here from layout.ControlDir.
const usagePath = layout.ControlDir + "/usage.jsonl"

// header is the hive line and the daemon line of the report.
type header struct {
	Name   string // repo root basename
	Pin    string // waggle.pin as written in config.yaml
	Mode   string // forge.comb_mode
	Remote string // forge.remote; "" prints as (local)
	Daemon bool   // the waggled socket exists
}

// runStatus implements `waggle status`. Exit codes: 0 report printed (even
// with malformed claims), 1 refused (not a hive, legacy layout, config
// invalid), 2 usage, 3 git or I/O failure. It writes nothing anywhere.
func runStatus(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("status", flag.ContinueOnError)
	fs.SetOutput(stderr)
	fs.Usage = func() { fmt.Fprintln(stderr, "Usage: waggle status [--repo <dir>]"); fs.PrintDefaults() }
	repo := fs.String("repo", "", "Repository directory (default: the current work tree root)")
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}
	exit := func(code int, format string, a ...any) int {
		fmt.Fprintf(stderr, "waggle status "+format+"\n", a...)
		return code
	}
	if fs.NArg() > 0 {
		return exit(2, "unexpected argument %q", fs.Arg(0))
	}

	root, err := repoRoot(*repo)
	if err != nil {
		return exit(3, "failed: %v", err)
	}
	cfgPath := filepath.Join(root, layout.Config)
	if _, err := os.Stat(cfgPath); err != nil {
		for _, legacy := range []string{layout.Root + "/config.yaml", layout.Root + "/roles"} {
			if _, err := os.Stat(filepath.Join(root, legacy)); err == nil {
				return exit(1, "refused: found %s: %v", legacy, scaffold.ErrLegacyLayout)
			}
		}
		return exit(1, "refused: not a hive: %s not found; run waggle init", layout.Config)
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		return exit(1, "refused: %v", err)
	}
	opts, err := cfg.CombOptions(root)
	if err != nil {
		return exit(1, "refused: %v", err)
	}
	opts.Now = statusClock
	backend, err := comb.NewGitBackend(opts)
	if err != nil {
		return exit(1, "refused: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	leases, err := backend.ListActiveLeases(ctx)
	var malformed *comb.MalformedClaimsError
	if err != nil && !errors.As(err, &malformed) {
		return exit(3, "failed: %v", err)
	}

	h := header{
		Name: filepath.Base(root), Pin: readPin(cfgPath),
		Mode: cfg.Forge.CombMode, Remote: cfg.Forge.Remote,
		Daemon: socketExists(filepath.Join(root, layout.Socket)),
	}
	stdout.Write(renderStatus(h, leases, malformed, statusClock()))
	return 0
}

// repoRoot resolves the work tree root of dir ("" means the current
// directory) with git rev-parse --show-toplevel.
func repoRoot(dir string) (string, error) {
	dir = cmp.Or(dir, ".")
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--show-toplevel")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse --show-toplevel in %s: %v: %s", dir, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(string(out)), nil
}

// readPin returns the waggle.pin scalar of the config file at path, or
// scaffold.Unpinned when the key is absent or empty. The config package parses
// the waggle section for well-formedness only (T-004), so this is a minimal
// scan of the block waggle init writes: a top-level "waggle:" key, then an
// indented "pin:" child with an optional trailing comment and optional quotes.
// A read error also yields Unpinned; the caller has already loaded the file.
func readPin(path string) string {
	data, _ := os.ReadFile(path)
	inWaggle := false
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "" || strings.HasPrefix(trimmed, "#"):
		case !strings.HasPrefix(line, " "):
			inWaggle = trimmed == "waggle:"
		case inWaggle && strings.HasPrefix(trimmed, "pin:"):
			v, _, _ := strings.Cut(strings.TrimPrefix(trimmed, "pin:"), "#")
			return cmp.Or(strings.Trim(strings.TrimSpace(v), `"'`), scaffold.Unpinned)
		}
	}
	return scaffold.Unpinned
}

// renderStatus renders the report; it is pure so tests use golden output.
// Leases print sorted by ticket number (the backend already filters expired
// ones) with role names as the backend returns them.
func renderStatus(h header, leases []comb.Lease, malformed *comb.MalformedClaimsError, now time.Time) []byte {
	var b bytes.Buffer
	pin := h.Pin
	if len(pin) == 40 && strings.Trim(pin, "0123456789abcdef") == "" {
		pin = pin[:7] + "..."
	}
	fmt.Fprintf(&b, "Hive: %s  pin %s  comb %s (%s)\n", h.Name, pin, h.Mode, cmp.Or(h.Remote, "local"))
	if h.Daemon {
		b.WriteString("waggled: running (socket present)\n")
	} else {
		fmt.Fprintf(&b, "waggled: stopped (no %s)\n", layout.Socket)
	}

	sorted := append([]comb.Lease(nil), leases...)
	sort.SliceStable(sorted, func(i, j int) bool { return ticketNum(sorted[i].TicketID) < ticketNum(sorted[j].TicketID) })
	fmt.Fprintf(&b, "\nClaims (%d active):\n", len(sorted))
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, l := range sorted {
		// Directories are stored without a trailing slash; print them as dirs.
		fmt.Fprintf(tw, "  %s\t%s\t%s\t%s\t%s/\texpires in %s\n", l.TicketID, l.Role, l.KeeperID, l.Gate,
			strings.Join(l.Directories, "/ "), expiresIn(l.ExpiresAt.Sub(now)))
	}
	tw.Flush()
	if malformed != nil && len(malformed.Claims) > 0 {
		first := malformed.Claims[0]
		fmt.Fprintf(&b, "  malformed: %d (%s: %s)\n", len(malformed.Claims), first.Ref, first.Reason)
	}

	// Phase A prints the absent-ledger line unconditionally; phase B reads it.
	fmt.Fprintf(&b, "\nUsage: none recorded (%s absent)\n", usagePath)
	return b.Bytes()
}

// ticketNum is the number of a canonical T-<n> ticket id, 0 when unparsable.
func ticketNum(id string) int {
	n, _ := strconv.Atoi(strings.TrimPrefix(id, "T-"))
	return n
}

// expiresIn formats a duration as whole minutes rounded down: 1h42m, 12m, 0m.
func expiresIn(d time.Duration) string {
	mins := int(max(d, 0) / time.Minute)
	if mins >= 60 {
		return fmt.Sprintf("%dh%dm", mins/60, mins%60)
	}
	return fmt.Sprintf("%dm", mins)
}
