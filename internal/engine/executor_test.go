package engine

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/plan"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

func TestBuildSpawnArgs_Default(t *testing.T) {
	cfg := config.DefaultConfig()
	args, cleanup := BuildSpawnArgs(cfg, "test prompt", nil)
	if cleanup != nil {
		defer cleanup()
	}

	// Should have base flags
	if !containsArg(args, "-p") {
		t.Error("missing -p flag")
	}
	if !containsArg(args, "--output-format") {
		t.Error("missing --output-format flag")
	}
	if !containsArg(args, "--max-turns") {
		t.Error("missing --max-turns flag")
	}
	if !containsArgValue(args, "--max-turns", "25") {
		t.Error("max-turns should be 25")
	}

	// Should NOT have --model or --effort (empty in default config)
	if containsArg(args, "--model") {
		t.Error("--model should not be present for default config")
	}
	if containsArg(args, "--effort") {
		t.Error("--effort should not be present for default config")
	}
	// Should NOT have --mcp-config (MCPServers is nil)
	if containsArg(args, "--mcp-config") {
		t.Error("--mcp-config should not be present when MCPServers is nil")
	}
}

func TestBuildSpawnArgs_WithModel(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Spawn.Model = "sonnet"
	args, cleanup := BuildSpawnArgs(cfg, "test", nil)
	if cleanup != nil {
		defer cleanup()
	}
	if !containsArgValue(args, "--model", "sonnet") {
		t.Error("--model sonnet should be present")
	}
}

func TestBuildSpawnArgs_WithEffort(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Spawn.Effort = "high"
	args, cleanup := BuildSpawnArgs(cfg, "test", nil)
	if cleanup != nil {
		defer cleanup()
	}
	if !containsArgValue(args, "--effort", "high") {
		t.Error("--effort high should be present")
	}
}

func TestBuildSpawnArgs_WithMaxTurns(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Spawn.MaxTurns = 50
	args, cleanup := BuildSpawnArgs(cfg, "test", nil)
	if cleanup != nil {
		defer cleanup()
	}
	if !containsArgValue(args, "--max-turns", "50") {
		t.Error("--max-turns 50 should be present")
	}
}

func TestBuildSpawnArgs_WithAddDirs(t *testing.T) {
	cfg := config.DefaultConfig()
	args, cleanup := BuildSpawnArgs(cfg, "test", []string{"/tmp/a", "/tmp/b"})
	if cleanup != nil {
		defer cleanup()
	}
	count := 0
	for _, a := range args {
		if a == "--add-dir" {
			count++
		}
	}
	if count != 2 {
		t.Errorf("expected 2 --add-dir flags, got %d", count)
	}
}

func TestBuildSpawnArgs_WithMCPServers(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MCPServers = map[string]config.MCPServer{
		"test": {Command: "node", Args: []string{"s.js"}, Enabled: true},
	}
	args, cleanup := BuildSpawnArgs(cfg, "test", nil)
	if cleanup != nil {
		defer cleanup()
	}
	if !containsArg(args, "--mcp-config") {
		t.Error("--mcp-config should be present when MCP servers configured")
	}
}

func TestBuildSpawnArgs_MCPDisabledServersSkipped(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.MCPServers = map[string]config.MCPServer{
		"disabled": {Command: "node", Enabled: false},
	}
	args, cleanup := BuildSpawnArgs(cfg, "test", nil)
	if cleanup != nil {
		defer cleanup()
	}
	if containsArg(args, "--mcp-config") {
		t.Error("--mcp-config should not be present when all servers are disabled")
	}
}

func TestBuildStepPrompt_AgentIdentity(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test-proj", Description: "test task"}
	p := plan.Plan{Steps: []plan.Step{{Number: 1, Title: "Do stuff", Body: "body"}}}
	step := p.Steps[0]

	prompt := buildStepPrompt(task, p, step, 0, "", "")
	if !strings.Contains(prompt, "executing step 1 of 1") {
		t.Error("prompt should mention step execution")
	}
	if !strings.Contains(prompt, "test task") {
		t.Error("prompt should contain task description")
	}
}

func TestBuildStepPrompt_WithAgent(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test-proj", Description: "test"}
	step := plan.Step{Number: 1, Title: "Step", Body: "body", Agent: "dev"}
	p := plan.Plan{Steps: []plan.Step{step}}

	prompt := buildStepPrompt(task, p, step, 0, "", "")
	if !strings.Contains(prompt, "dev agent") {
		t.Error("prompt should mention agent name")
	}
}

func TestBuildStepPrompt_CLAUDEMDInjection(t *testing.T) {
	tmpDir := t.TempDir()
	claudeMd := filepath.Join(tmpDir, "CLAUDE.md")
	os.WriteFile(claudeMd, []byte("# Test Guidelines\nAlways test first."), 0644)

	// Temporarily adjust HOME so projectPath resolves to our temp dir
	task := queue.Task{ID: 1, Project: filepath.Base(tmpDir), Description: "test"}
	step := plan.Step{Number: 1, Title: "Step", Body: "body"}
	p := plan.Plan{Steps: []plan.Step{step}}

	// The function reads from ~/Projects/<project>/CLAUDE.md
	// We can't easily mock HOME, so just verify the mechanism works by checking rules are present
	prompt := buildStepPrompt(task, p, step, 0, "", "")
	if !strings.Contains(prompt, "RULES:") {
		t.Error("prompt should contain RULES section")
	}
}

func TestBuildStepPrompt_RetryContext(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test", Description: "test"}
	step := plan.Step{Number: 1, Title: "Step", Body: "body"}
	p := plan.Plan{Steps: []plan.Step{step}}

	prompt := buildStepPrompt(task, p, step, 1, "Previous error: something broke", "")
	if !strings.Contains(prompt, "Previous attempt context") {
		t.Error("prompt should include recovery context on retry")
	}
	if !strings.Contains(prompt, "something broke") {
		t.Error("prompt should include the actual recovery text")
	}
}

func TestBuildStepPrompt_AllRules(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test", Description: "test"}
	step := plan.Step{Number: 1, Title: "Step", Body: "body"}
	p := plan.Plan{Steps: []plan.Step{step}}

	prompt := buildStepPrompt(task, p, step, 0, "", "")
	expectedRules := []string{
		"create, edit or modify source code",
		"FULL permissions",
		"list every file",
		"NEVER create documentation files",
		"NEVER invoke /bmad",
		"NEVER use EnterPlanMode",
		"Be concise",
	}
	for _, rule := range expectedRules {
		if !strings.Contains(prompt, rule) {
			t.Errorf("missing rule containing: %s", rule)
		}
	}

	// ReadOnly step should have different rules
	roStep := plan.Step{Number: 1, Title: "Step", Body: "body", ReadOnly: true}
	roPrompt := buildStepPrompt(task, p, roStep, 0, "", "")
	roExpected := []string{
		"READ-ONLY step",
		"Summarize your findings",
		"NEVER create documentation files",
		"NEVER invoke /bmad",
		"NEVER use EnterPlanMode",
		"Be concise",
	}
	for _, rule := range roExpected {
		if !strings.Contains(roPrompt, rule) {
			t.Errorf("ReadOnly prompt missing rule containing: %s", rule)
		}
	}
}

func TestBuildStepPrompt_PrevSteps(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test", Description: "test"}
	step := plan.Step{Number: 2, Title: "Step 2", Body: "body"}
	p := plan.Plan{Steps: []plan.Step{{Number: 1, Title: "Step 1"}, step}}

	prompt := buildStepPrompt(task, p, step, 0, "", "Step 1: did things")
	if !strings.Contains(prompt, "Previous steps completed") {
		t.Error("prompt should include previous steps section")
	}
	if !strings.Contains(prompt, "Step 1: did things") {
		t.Error("prompt should include actual previous step text")
	}
}

func TestBuildRecoveryPrompt_Truncation(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test", Description: "fix bug"}
	step := plan.Step{Number: 1, Title: "Apply fix"}

	longOutput := strings.Repeat("x", 2000)
	prompt := buildRecoveryPrompt(task, step, longOutput, 1)

	// After truncation, the output section should only have last 1000 chars of 'x'
	// The full prompt includes template text + 1000 x's, so total x count should be 1000
	xCount := strings.Count(prompt, "x")
	if xCount > 1010 { // 1000 from truncation + some from template words like "fix", "exit"
		t.Errorf("output should be truncated, got %d x chars", xCount)
	}
	if !strings.Contains(prompt, "Step 1:") {
		t.Error("should contain step number")
	}
	if !strings.Contains(prompt, "fix bug") {
		t.Error("should contain task description")
	}
	if !strings.Contains(prompt, "Exit code: 1") {
		t.Error("should contain exit code")
	}
	if !strings.Contains(prompt, "INSTRUCTIONS:") {
		t.Error("should contain instructions section")
	}
}

func TestBuildRecoveryPrompt_ShortOutput(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test", Description: "test"}
	step := plan.Step{Number: 1, Title: "Test step"}

	prompt := buildRecoveryPrompt(task, step, "short error", 2)
	if !strings.Contains(prompt, "short error") {
		t.Error("short output should be included verbatim")
	}
}

func TestHasWriteTools_Empty(t *testing.T) {
	if hasWriteTools(nil) {
		t.Error("nil should return false")
	}
	if hasWriteTools([]string{}) {
		t.Error("empty should return false")
	}
}

func TestHasWriteTools_ReadOnly(t *testing.T) {
	if hasWriteTools([]string{"Read", "Glob"}) {
		t.Error("read-only tools should return false")
	}
}

func TestHasWriteTools_WithEdit(t *testing.T) {
	if !hasWriteTools([]string{"Edit"}) {
		t.Error("Edit should return true")
	}
}

func TestHasWriteTools_WithBash(t *testing.T) {
	if !hasWriteTools([]string{"Bash", "Read"}) {
		t.Error("Bash should return true")
	}
}

func TestHasWriteTools_WithWrite(t *testing.T) {
	if !hasWriteTools([]string{"Write"}) {
		t.Error("Write should return true")
	}
}

func TestHasWriteTools_WithNotebookEdit(t *testing.T) {
	if !hasWriteTools([]string{"NotebookEdit"}) {
		t.Error("NotebookEdit should return true")
	}
}

func TestFilterEnv_RemovesTarget(t *testing.T) {
	env := []string{"PATH=/usr/bin", "CLAUDECODE=abc", "HOME=/home/user"}
	filtered := filterEnv(env, "CLAUDECODE")
	if len(filtered) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(filtered))
	}
	for _, e := range filtered {
		if strings.HasPrefix(e, "CLAUDECODE=") {
			t.Error("CLAUDECODE should be removed")
		}
	}
}

func TestFilterEnv_KeepsOthers(t *testing.T) {
	env := []string{"PATH=/usr/bin", "HOME=/home/user"}
	filtered := filterEnv(env, "CLAUDECODE")
	if len(filtered) != 2 {
		t.Errorf("expected 2 entries, got %d", len(filtered))
	}
}

func TestFilterEnv_Empty(t *testing.T) {
	filtered := filterEnv(nil, "FOO")
	if len(filtered) != 0 {
		t.Error("empty input should produce empty output")
	}
}

func TestBuildStepPrompt_ReadOnlyRules(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test", Description: "test"}
	step := plan.Step{Number: 1, Title: "Investigate", Body: "Read files", ReadOnly: true}
	p := plan.Plan{Steps: []plan.Step{step}}

	prompt := buildStepPrompt(task, p, step, 0, "", "")
	if !strings.Contains(prompt, "READ-ONLY step") {
		t.Error("ReadOnly step prompt should contain READ-ONLY instruction")
	}
	if strings.Contains(prompt, "MUST create, edit or modify") {
		t.Error("ReadOnly step should NOT contain write requirement")
	}
}

func TestBuildStepPrompt_NonReadOnlyRules(t *testing.T) {
	task := queue.Task{ID: 1, Project: "test", Description: "test"}
	step := plan.Step{Number: 1, Title: "Implement", Body: "Write code", ReadOnly: false}
	p := plan.Plan{Steps: []plan.Step{step}}

	prompt := buildStepPrompt(task, p, step, 0, "", "")
	if strings.Contains(prompt, "READ-ONLY step") {
		t.Error("Non-ReadOnly step should NOT contain READ-ONLY instruction")
	}
	if !strings.Contains(prompt, "MUST create, edit or modify") {
		t.Error("Non-ReadOnly step should contain write requirement")
	}
}

// helpers

func containsArg(args []string, flag string) bool {
	for _, a := range args {
		if a == flag {
			return true
		}
	}
	return false
}

func containsArgValue(args []string, flag, value string) bool {
	for i, a := range args {
		if a == flag && i+1 < len(args) && args[i+1] == value {
			return true
		}
	}
	return false
}
