// Package layout names every path Waggle owns in a host repository.
//
// Values are repo-relative and slash-separated. Waggle owns .waggle/ and,
// outside it, only the gate workflow, one marked import block in CLAUDE.md or
// AGENTS.md, one marked .gitignore block, and generated runtime shims.
// Product packages that name a Waggle path take it from here. internal/comb
// keeps literal tier lists for auditability and checks them against these
// values in its tests.
package layout

const (
	// Root is the single folder Waggle owns.
	Root = ".waggle"
	// ControlDir holds the control plane. Tier 3.
	ControlDir = Root + "/control"
	// HiveDir holds the hive record. Tier 2 minimum.
	HiveDir = Root + "/hive"
	// Worktrees holds Cell worktrees. Runtime state, gitignored.
	Worktrees = Root + "/worktrees"

	// Config is the hive configuration, including waggle.pin. Seeded.
	Config = ControlDir + "/config.yaml"
	// Roster lists the roles. Managed.
	Roster = ControlDir + "/roster.json"
	// RolesDir holds one prompt per role. Managed.
	RolesDir = ControlDir + "/roles"
	// WaggleMD is the hive architecture and boundary map. Seeded.
	WaggleMD = ControlDir + "/WAGGLE.md"
	// Devcontainer is the Cell sandbox definition. Managed.
	Devcontainer = ControlDir + "/devcontainer.json"
	// Local holds per-Keeper preferences. Seeded, gitignored.
	Local = ControlDir + "/local.yaml"
	// Socket is the waggled control socket. Runtime state, gitignored.
	Socket = ControlDir + "/run.sock"

	// TasksDir holds task briefs.
	TasksDir = HiveDir + "/tasks"
	// ReviewsDir holds Guard reviews.
	ReviewsDir = HiveDir + "/reviews"
	// MediaDir holds verification evidence.
	MediaDir = HiveDir + "/media"
	// Decisions is the hive decision log.
	Decisions = HiveDir + "/DECISIONS.md"
	// Workplan is the derived work plan.
	Workplan = HiveDir + "/WORKPLAN.md"
	// Roadmap is the hive roadmap.
	Roadmap = HiveDir + "/ROADMAP.md"
	// SpecsDir holds specs.
	SpecsDir = HiveDir + "/specs"

	// GateWorkflow is the CI gate. Managed, Tier 3.
	GateWorkflow = ".github/workflows/waggle-gate.yml"
	// ShimPrefix starts every generated Claude Code agent shim path.
	ShimPrefix = ".claude/agents/waggle-"
)

// Managed lists files and directories that are pure template output.
// waggle init --force rewrites them; hand edits are not allowed.
var Managed = []string{Roster, RolesDir, Devcontainer, GateWorkflow}

// Seeded lists files rendered once and then owned by the hive.
// waggle init --force never overwrites them.
var Seeded = []string{Config, WaggleMD, Local}
