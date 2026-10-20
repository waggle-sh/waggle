package dispatch

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	"github.com/waggle-sh/waggle/internal/comb"
)

// maxViolations is how many violations result.json lists before it only counts.
const maxViolations = 200

// Survivor is a process that outlived the worker and was killed (D-187).
type Survivor struct {
	PID     int    `json:"pid"`
	Command string `json:"command"`
}

type Usage struct {
	Input      int `json:"input"`
	CacheWrite int `json:"cache_write"`
	CacheRead  int `json:"cache_read"`
	Output     int `json:"output"`
}

// Result is result.json (T-023 section 9.1) and what Run returns. The queen
// reads it when the process exits. ExitCode is the shared table of section 4.
type Result struct {
	Schema         int              `json:"schema"`
	Ticket         string           `json:"ticket"`
	Role           string           `json:"role"`
	Branch         string           `json:"branch"`
	BaseOID        string           `json:"base_oid"`
	CheckBaseOID   string           `json:"check_base_oid"`
	HeadOID        string           `json:"head_oid"`
	Worktree       string           `json:"worktree"`
	RunDir         string           `json:"run_dir"`
	KeeperID       string           `json:"keeper_id"`
	ClaimOID       string           `json:"claim_oid"`
	ClaimVerb      string           `json:"claim_verb"`
	ClaimNotes     []string         `json:"claim_notes"`
	Cell           string           `json:"cell"`
	RuntimeVersion string           `json:"runtime_version"`
	Model          string           `json:"model"`
	Profile        string           `json:"profile"`
	Floored        bool             `json:"floored"`
	StartedAt      time.Time        `json:"started_at"`
	EndedAt        time.Time        `json:"ended_at"`
	End            string           `json:"end"`
	ResultSubtype  string           `json:"result_subtype"` // the stream's own result subtype, so a budget stop is not a crash
	ExitCode       int              `json:"exit_code"`
	Error          string           `json:"error,omitempty"`
	Usage          Usage            `json:"usage"`
	CostUSD        *float64         `json:"cost_usd"`
	Compacted      bool             `json:"compacted"`
	Violations     []comb.Violation `json:"violations"`
	MoreViolations int              `json:"more_violations,omitempty"`
	Warnings       []string         `json:"warnings"`
	NotEnforced    []string         `json:"not_enforced"`
	Verdict        string           `json:"verdict"`
	Second         bool             `json:"second"`
	MustFix        int              `json:"must_fix"`         // the count the model step-up used (D-188)
	Survivors      []Survivor       `json:"survivors_killed"` // processes that outlived the worker and were killed (D-187)
	Pushed         bool             `json:"pushed"`
	Final          string           `json:"final"`
	Transcript     string           `json:"transcript"`
	StderrLog      string           `json:"stderr_log"`

	JSON []byte `json:"-"` // the bytes written to result.json
}

// Write stores r as result.json in r.RunDir, mode 0600, and keeps the bytes in
// r.JSON so the CLI's --json output equals the file.
func (r *Result) Write() error {
	if len(r.Violations) > maxViolations {
		r.MoreViolations, r.Violations = len(r.Violations)-maxViolations, r.Violations[:maxViolations]
	}
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	r.JSON = append(b, '\n')
	return writeFile(r.RunDir, "result.json", r.JSON)
}

// writeFile writes name into dir through a fresh O_EXCL temp file (mode 0600)
// and a rename, so a symlink or FIFO the agent planted at name is replaced,
// never followed or blocked on.
func writeFile(dir, name string, data []byte) error {
	f, err := os.CreateTemp(dir, "."+name+".tmp")
	if err != nil {
		return err
	}
	_, werr := f.Write(data)
	if err := errors.Join(werr, f.Close()); err != nil {
		_ = os.Remove(f.Name())
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, name))
}
