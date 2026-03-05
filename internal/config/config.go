package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type SkeletonStep struct {
	Label       string `json:"label"`
	Description string `json:"description"`
	Prompt      string `json:"prompt"`
	ReadOnly    bool   `json:"read_only"`
}

type MCPServer struct {
	Command      string        `json:"command"`
	Args         []string      `json:"args"`
	Enabled      bool          `json:"enabled"`
	SkeletonStep *SkeletonStep `json:"skeleton_step,omitempty"`
}

// KnownSkeletonSteps maps MCP names to their default skeleton steps.
// When an MCP is installed and matches a known name, the step is auto-attached.
var KnownSkeletonSteps = map[string]SkeletonStep{
	"context7": {
		Label:       "Context7 Lookup",
		Description: "Look up library documentation",
		Prompt:      "Use resolve-library-id then query-docs for each relevant library. Note relevant APIs and patterns.",
		ReadOnly:    true,
	},
	"chrome-devtools": {
		Label:       "Browser Verification",
		Description: "Verify UI in browser using Chrome DevTools",
		Prompt:      "Open the app in the browser, take a snapshot, verify the UI renders correctly and matches expectations. Check for console errors.",
		ReadOnly:    false,
	},
}

// SkeletonPhase represents a single phase in the task execution skeleton.
// When Phases is set in SkeletonConfig, each phase is fully defined here.
type SkeletonPhase struct {
	ID       string `json:"id"`
	Label    string `json:"label"`
	Prompt   string `json:"prompt"`
	ReadOnly bool   `json:"read_only,omitempty"`
	Generate bool   `json:"generate,omitempty"`
	Enabled  bool   `json:"enabled"`
}

type SpawnConfig struct {
	Model           string `json:"model"`
	Effort          string `json:"effort"`
	MaxTurns        int    `json:"max_turns"`
	StepTimeoutMin  int    `json:"step_timeout_min"`
	PlanTimeoutMin  int    `json:"plan_timeout_min"`   // 0 = use default (15 min)
	PlanMaxTurns    int    `json:"plan_max_turns"`     // 0 = unlimited (no --max-turns flag)
	MaxPlanAttempts int    `json:"max_plan_attempts"`  // 0 falls back to default of 3
	Backend         string `json:"backend,omitempty"`  // "claude" (default), "opencode", etc.
}

type SkeletonConfig struct {
	WebSearch      bool `json:"web_search"`
	DocSetup       bool `json:"doc_setup"`
	BuildVerify    bool `json:"build_verify"`
	Test           bool `json:"test"`
	SecurityReview bool `json:"security_review"`
	PreCommit      bool `json:"pre_commit"`
	Commit         bool `json:"commit"`
	Push           bool `json:"push"`
}

func DefaultSkeleton() SkeletonConfig {
	return SkeletonConfig{
		WebSearch:      true,
		DocSetup:       true,
		BuildVerify:    true,
		Test:           true,
		SecurityReview: true,
		PreCommit:      true,
		Commit:         true,
		Push:           false,
	}
}

// SkeletonFor returns the skeleton config for a project, falling back to global.
func SkeletonFor(cfg Config, project string) SkeletonConfig {
	if cfg.ProjectSkeletons != nil {
		if sk, ok := cfg.ProjectSkeletons[project]; ok {
			return sk
		}
	}
	return cfg.Skeleton
}

// BackendFor returns the backend name for a project, falling back to global, then "claude".
func BackendFor(cfg Config, project string) string {
	if cfg.ProjectBackends != nil {
		if b, ok := cfg.ProjectBackends[project]; ok {
			return b
		}
	}
	if cfg.Spawn.Backend != "" {
		return cfg.Spawn.Backend
	}
	return "claude"
}

type Config struct {
	ProjectsDir        string                `json:"projects_dir"`
	ClaudeDir          string                `json:"claude_dir"`
	RefreshIntervalSec int                   `json:"refresh_interval_sec"`
	ContextLimit       int                   `json:"context_limit"`
	WebEnabled         bool                  `json:"web_enabled"`
	WebPort            int                   `json:"web_port"`
	WebHost            string                `json:"web_host"`
	WebPassword        string                `json:"web_password"`
	WebhookURL         string                `json:"webhook_url,omitempty"`
	Spawn              SpawnConfig                    `json:"spawn"`
	Skeleton           SkeletonConfig                 `json:"skeleton"`
	ProjectSkeletons   map[string]SkeletonConfig      `json:"project_skeletons,omitempty"`
	MaxConcurrent      int                            `json:"max_concurrent"`
	AutopilotAutostart bool                           `json:"autopilot_autostart"`
	MCPServers         map[string]MCPServer           `json:"mcp_servers,omitempty"`
	SourceDir          string                         `json:"source_dir,omitempty"`
	Debug              bool                           `json:"debug,omitempty"`
	LogRetentionDays   int                            `json:"log_retention_days"`
	SudoEnabled        bool                           `json:"sudo_enabled,omitempty"`
	PhaseHints         map[string]string              `json:"phase_hints,omitempty"`
	ProjectBackends    map[string]string              `json:"project_backends,omitempty"`
}

// DefaultPhaseHints returns descriptions for each skeleton phase.
// These hints are included in the skeleton JSON so the LLM knows what each phase means.
func DefaultPhaseHints() map[string]string {
	return map[string]string{
		"doc_setup":    "MUST be the FIRST implementation step. ReadOnly: false. Run EVERY command — this is NOT guidance. PROJECT SETUP: (1) Ensure dev branch: git checkout dev || git checkout -b dev. (2) SSH: run ssh -T git@github.com. If authenticated, check git remote -v. If remote is HTTPS, switch to SSH: git remote set-url origin git@github.com:<owner>/<repo>.git. (2b) BRANCH INFRASTRUCTURE: verify test and prod branches exist on the remote. Run git ls-remote --heads origin test prod. If test is missing: git branch test main && git push origin test. If prod is missing: git branch prod main && git push origin prod. If main has no commits yet, create them from dev instead. These branches are REQUIRED for the version-controller promotion pipeline (dev→test→prod→main). (2c) ACTIONS PERMISSIONS: enable GitHub Actions to create and approve PRs (required by version-controller). Run: gh api repos/{owner}/{repo}/actions/permissions/workflow -X PUT -f default_workflow_permissions=write -F can_approve_pull_request_reviews=true. Get owner/repo from git remote -v. Without this, version-controller will fail with 403 when creating promotion PRs. (3) TEMPLATE CHECK: verify the project has CI/CD infrastructure from github-cicd-template. Check for EACH of these specific files: .pre-commit-config.yaml, .github/workflows/version-controller.yml, .github/workflows/ci.yml, scripts/ submodule, requirements.dev.txt, .bumpversion.cfg, CONTRIBUTING.md, INSTALL.md, VERSIONING.md, SECURITY.md, CODE_OF_CONDUCT.md. If ANY of these are missing, fetch the template: gh repo clone JuanVilla424/github-cicd-template .tmpl-src -- --depth=1 && copy ONLY the missing files from .tmpl-src into the project (preserve existing files), then rm -rf .tmpl-src. The version-controller.yml is CRITICAL — without it, versioning keywords like [patch candidate] and [minor candidate] in commits do nothing. (4) Submodules: if .gitmodules exists run git submodule init && git submodule update --remote. If scripts/ directory does NOT exist, run git submodule add https://github.com/JuanVilla424/scripts.git scripts. (5) Python venv: run python3 -m venv venv && source venv/bin/activate && pip install -r requirements.dev.txt (fallback to requirements.txt if .dev doesn't exist). (6) Pre-commit MUST be installed: pip install pre-commit (use venv/bin/pip if venv exists). Then run pre-commit install && pre-commit install --hook-type pre-push. NEVER skip this. (7) Env setup: Node=npm install, Go=go mod download. (8) pre-commit run --all-files — fix any issues it reports. (9) Clean template artifacts: remove CNAME, .tmpl-src, any leftover files from the template that do not belong to this project. (10) Verify project identity: CHANGELOG, manifest, LICENSE, CONTRIBUTING.md, INSTALL.md, VERSIONING.md, SECURITY.md must have this project's name, not the template's. Fix if wrong. (11) README QUALITY: Read the README.md. If it is a skeleton (less than 50 lines, has 'TBD', missing sections, or badges are not linked), REGENERATE it completely. Read CONTRIBUTING.md, INSTALL.md, VERSIONING.md to match their style. The README MUST have ALL of these — NO EXCEPTIONS: (a) H1 with emoji icon prefix (e.g. # 📄 Project Name). (b) LINKED badges row — [![badge](img-url)](link-url) format, NOT plain ![badge](url). Include: language, version, build status, status, license. (c) Description paragraph (2+ sentences, not 'A brief description'). (d) 📚 Table of Contents with anchor links to every section. (e) 🌟 Features section with bullet points. (f) 🚀 Getting Started with sub-sections: 📋 Prerequisites, 🔨 Installation (with git clone + cd), 🔧 Environment Setup (venv/npm install), 🛸 Pre-Commit Hooks (pre-commit install commands). (g) 📋 Scripts table. (h) 🤝 Contributing section referencing CONTRIBUTING.md and CODE_OF_CONDUCT.md. (i) 📫 Contact section with email. (j) 📜 License section with full paragraph and link to LICENSE file. Every ## and ### header MUST have an emoji icon prefix. ZERO exceptions. (12) DOCUMENT THE APP BEFORE CODING: Read the ENTIRE codebase structure (all directories, key source files, entry points, configs). Then create or update these files with REAL content — not placeholders: (a) ARCHITECT.md — package structure tree, responsibility of each module, data flow diagram in text, design decisions table, technology stack. (b) CONTEXT.md — what the app does in 2-3 sentences, current state (working/WIP/broken), key entry points, environment requirements, how to run it. (c) README.md — already handled in step 11. These documents must describe what the app ACTUALLY does based on reading the code. An app without documentation is garbage — document FIRST, develop AFTER.",
		"web_search":   "Search the web for current best practices and documentation.",
		"build_verify": "Compile/build the project. Create build tooling if missing. Verify clean build.",
		"test":         "Run existing tests. Create new tests for changes. Set up test infra if missing.",
		"security_review": "Claude Code Security: Run /security-review to scan the codebase for vulnerabilities. This is Anthropic's built-in security scanner that uses deep semantic analysis. Review findings and fix any issues with severity critical or high. Categories scanned: injection attacks (SQL, command, XSS, XXE), authentication & authorization flaws, hardcoded secrets/API keys/passwords, cryptographic issues, input validation, insecure configuration, and dependency risks. If /security-review is not available, perform a manual security review analyzing the code changes for the same categories. For each finding: report severity, file, line, description, and apply the fix.",
		"pre_commit":   "Run pre-commit run --all-files. If pre-commit is not installed, install it first: pip install pre-commit && pre-commit install && pre-commit install --hook-type pre-push. Fix ALL issues reported by pre-commit before proceeding.",
		"commit":       "BEFORE committing, run MARKDOWN QUALITY GATE: list all .md files (find . -name '*.md' -not -path './venv/*' -not -path './node_modules/*' -not -path './.git/*' -not -path './.bmad/*'). For EACH .md file, verify: (a) every ## and ### header has an emoji icon prefix, (b) README.md has LINKED badges [![text](img)](link) — NOT plain ![text](img), (c) README.md has Table of Contents, Getting Started with sub-sections (Prerequisites, Installation, Environment Setup, Pre-Commit Hooks), Contributing, Contact, and License with full paragraph, (d) no placeholder text ('TBD', 'TODO', 'Lorem ipsum', 'A brief description', 'Insert here'). If ANY .md file fails these checks, FIX IT before committing. Then: single git commit. Format: type(core): description [versioning keyword]. VERSIONING KEYWORD IS MANDATORY — choose based on commit type: feat → [minor candidate], fix → [patch candidate], refactor/docs/style/test/chore → [patch candidate]. Only use [major candidate] for explicit breaking changes. This keyword triggers the version-controller CI/CD workflow that promotes branches (dev→test→prod→main) and creates releases. Without it, the commit will NOT be promoted. No emojis in commit message, no Co-Authored-By.",
		"push":         "ReadOnly: false. Steps: (1) Check remote URL with git remote -v. (2) Test SSH with ssh -T git@github.com. (3) If SSH works and remote is HTTPS, switch: git remote set-url origin git@github.com:<owner>/<repo>.git. (4) Execute git push origin <current-branch>. (5) PIPELINE VERIFICATION: run gh run list --branch <current-branch> --limit 1 --json databaseId,status,conclusion to get the latest workflow run. If a run exists, run gh run watch <run-id> to wait for completion. If the run fails, run gh run view <run-id> --log-failed to read error logs. Diagnose the failure, fix the code, rebuild, test, commit, and push again. Maximum 2 fix attempts — if still failing after 2 retries, report the error and stop. (6) GITHUB PAGES (ONLY if the task description explicitly mentions GitHub Pages, deploy to pages, or web deployment): After successful push and pipeline verification, check if GitHub Pages is already enabled: run gh api repos/{owner}/{repo}/pages 2>/dev/null. If the command fails (404), enable Pages: for projects with a deploy workflow (.github/workflows/ containing 'pages' or 'deploy') use gh api repos/{owner}/{repo}/pages -X POST -f build_type=workflow; for projects without a deploy workflow use gh api repos/{owner}/{repo}/pages -X POST -f source.branch=main -f source.path=/. Then set the homepage URL: gh repo edit {owner}/{repo} --homepage https://{owner}.github.io/{repo}/. Also update README.md: add a **Live demo:** https://{owner}.github.io/{repo}/ line right after the project description paragraph (before the badges row), matching the stakka README pattern. If README already has a Live demo line, update the URL. Get owner and repo from git remote -v (parse the github.com:<owner>/<repo>.git pattern). Skip this step entirely if the task does not mention GitHub Pages. (7) BRANCH PROMOTION (ONLY if the task description explicitly mentions deploying to main, promoting to main, reaching main, releasing, or full release): After successful push to dev and CI pipeline pass, drive changes through the full version-controller promotion pipeline. For each hop (dev→test, test→prod, prod→main): (a) Wait for version-controller workflow: gh run list --workflow=version-controller.yml --branch <current> --limit 1 --json databaseId,status,conclusion, then gh run watch <id>. (b) If version-controller fails: gh run view <id> --log-failed, report error and stop. (c) Find the PR created: gh pr list --base <next> --head <current> --json number,url --jq '.[0]'. (d) Merge it: gh pr merge <number> --merge. (e) If merge fails due to branch protection or approval requirements, report: PR #<number> requires manual approval at <url> — approve and merge it, then rerun this step. The full chain is: dev→test→prod→main. After main receives the merge, version-controller creates a tag and GitHub release. Verify with gh release list --limit 1. Skip this step entirely if the task does not mention deploying/promoting to main. This is NOT guidance — run every command.",
	}
}

func DefaultConfig() Config {
	home, _ := os.UserHomeDir()
	return Config{
		ProjectsDir:        filepath.Join(home, "Projects"),
		ClaudeDir:          filepath.Join(home, ".claude"),
		RefreshIntervalSec: 30,
		ContextLimit:       0,
		WebEnabled:         false,
		WebPort:            7777,
		WebHost:            "",
		WebPassword:        "",
		Spawn:              SpawnConfig{Model: "opusplan", Effort: "high", MaxTurns: 15, StepTimeoutMin: 4, PlanMaxTurns: 15, MaxPlanAttempts: 3},
		Skeleton:           DefaultSkeleton(),
		MaxConcurrent:      3,
		AutopilotAutostart: false,
		MCPServers:         nil,
		SourceDir:          filepath.Join(home, "Projects", "teamoon"),
		LogRetentionDays:   20,
		PhaseHints:         DefaultPhaseHints(),
	}
}

// IsPasswordHashed returns true if the password string is a bcrypt hash.
func IsPasswordHashed(pw string) bool {
	return strings.HasPrefix(pw, "$2a$") || strings.HasPrefix(pw, "$2b$")
}

func ConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "teamoon")
}

func Load() (Config, error) {
	cfg := DefaultConfig()
	path := filepath.Join(ConfigDir(), "config.json")

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if saveErr := Save(cfg); saveErr != nil {
				return cfg, saveErr
			}
			return cfg, nil
		}
		return cfg, err
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	// Backfill missing phase hints for existing configs
	defaults := DefaultPhaseHints()
	if cfg.PhaseHints == nil {
		cfg.PhaseHints = defaults
	} else {
		for k, v := range defaults {
			if _, ok := cfg.PhaseHints[k]; !ok {
				cfg.PhaseHints[k] = v
			}
		}
	}
	return cfg, nil
}

func Save(cfg Config) error {
	dir := ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0644)
}

// ReadGlobalMCPServers reads MCP servers from ~/.claude/settings.json.
func ReadGlobalMCPServers() map[string]MCPServer {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude", "settings.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]MCPServer, len(raw.MCPServers))
	for name, s := range raw.MCPServers {
		result[name] = MCPServer{Command: s.Command, Args: s.Args, Enabled: true}
	}
	return result
}

// ReadGlobalMCPServersFrom reads MCP servers from a specific file path (for testing).
func ReadGlobalMCPServersFrom(path string) map[string]MCPServer {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw struct {
		MCPServers map[string]struct {
			Command string   `json:"command"`
			Args    []string `json:"args"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	result := make(map[string]MCPServer, len(raw.MCPServers))
	for name, s := range raw.MCPServers {
		result[name] = MCPServer{Command: s.Command, Args: s.Args, Enabled: true}
	}
	return result
}

// BuildMCPConfigJSON writes enabled MCP servers to a temp JSON file and returns the path.
func BuildMCPConfigJSON(servers map[string]MCPServer) (string, error) {
	type mcpEntry struct {
		Command string   `json:"command"`
		Args    []string `json:"args"`
	}
	out := struct {
		MCPServers map[string]mcpEntry `json:"mcpServers"`
	}{
		MCPServers: make(map[string]mcpEntry, len(servers)),
	}
	for name, s := range servers {
		out.MCPServers[name] = mcpEntry{Command: s.Command, Args: s.Args}
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "teamoon-mcp-*.json")
	if err != nil {
		return "", err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		os.Remove(f.Name())
		return "", err
	}
	f.Close()
	return f.Name(), nil
}

// AttachKnownSkeletonSteps ensures every MCP with a known skeleton step has it set.
// Returns true if any step was attached (config needs saving).
func AttachKnownSkeletonSteps(servers map[string]MCPServer) bool {
	changed := false
	for name, mcp := range servers {
		if step, ok := KnownSkeletonSteps[name]; ok && mcp.SkeletonStep == nil {
			mcp.SkeletonStep = &step
			servers[name] = mcp
			changed = true
		}
	}
	return changed
}

// InitMCPFromGlobal populates MCPServers from global settings if nil (one-time bootstrap).
// Persists config if skeleton steps were attached to existing servers.
func InitMCPFromGlobal(cfg *Config) {
	if cfg.MCPServers != nil {
		if AttachKnownSkeletonSteps(cfg.MCPServers) {
			Save(*cfg)
		}
		return
	}
	cfg.MCPServers = ReadGlobalMCPServers()
	AttachKnownSkeletonSteps(cfg.MCPServers)
}

// RemoveMCPFromGlobal removes an MCP server entry from ~/.claude/settings.json.
func RemoveMCPFromGlobal(name string) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude", "settings.json")

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	servers := make(map[string]mcpEntry)
	if existing, ok := raw["mcpServers"]; ok {
		json.Unmarshal(existing, &servers)
	}

	delete(servers, name)

	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	raw["mcpServers"] = serversJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// InstallMCPToGlobal adds an MCP server entry to ~/.claude/settings.json.
// It reads the file, merges the new server into mcpServers, and writes back.
// If envVars is non-empty, they are set in the "env" field of the server entry.
func InstallMCPToGlobal(name, command string, args []string, envVars map[string]string) error {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".claude", "settings.json")

	// Read existing file (or start fresh)
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = make(map[string]json.RawMessage)
	} else {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}

	// Parse existing mcpServers
	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	servers := make(map[string]mcpEntry)
	if existing, ok := raw["mcpServers"]; ok {
		json.Unmarshal(existing, &servers)
	}

	entry := mcpEntry{Command: command, Args: args}
	if len(envVars) > 0 {
		entry.Env = envVars
	}
	servers[name] = entry

	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	raw["mcpServers"] = serversJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// InstallMCPToGlobalAt is like InstallMCPToGlobal but writes to a specific path (for testing).
func InstallMCPToGlobalAt(path, name, command string, args []string, envVars map[string]string) error {
	var raw map[string]json.RawMessage
	data, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		raw = make(map[string]json.RawMessage)
	} else {
		if err := json.Unmarshal(data, &raw); err != nil {
			return err
		}
	}

	type mcpEntry struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env,omitempty"`
	}
	servers := make(map[string]mcpEntry)
	if existing, ok := raw["mcpServers"]; ok {
		json.Unmarshal(existing, &servers)
	}

	entry := mcpEntry{Command: command, Args: args}
	if len(envVars) > 0 {
		entry.Env = envVars
	}
	servers[name] = entry

	serversJSON, err := json.Marshal(servers)
	if err != nil {
		return err
	}
	raw["mcpServers"] = serversJSON

	out, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, 0644)
}

// planMCPAllowlist contains MCP servers suitable for plan generation (lightweight, read-only).
var planMCPAllowlist = map[string]bool{"context7": true}

// FilterPlanMCP returns only lightweight MCP servers suitable for plan generation.
// Excludes heavy servers like chrome-devtools (spawns Chrome browser) and others
// that add startup overhead without benefiting read-only investigation.
func FilterPlanMCP(servers map[string]MCPServer) map[string]MCPServer {
	result := make(map[string]MCPServer)
	for name, s := range servers {
		if s.Enabled && planMCPAllowlist[name] {
			result[name] = s
		}
	}
	return result
}

// FilterEnabledMCP returns only enabled servers from the map.
func FilterEnabledMCP(servers map[string]MCPServer) map[string]MCPServer {
	result := make(map[string]MCPServer)
	for name, s := range servers {
		if s.Enabled {
			result[name] = s
		}
	}
	return result
}
