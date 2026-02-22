package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validPlan = `# Plan: Add feature X

## Analysis
Found existing code at src/main.go.

## Steps

### Step 1: Research codebase
Agent: analyst
Read the source files and understand the architecture.
Verify: Report created

### Step 2: Implement feature
Agent: dev
Edit src/main.go to add feature X.
Add new function HandleX().
Verify: go build succeeds

### Step 3: Write tests
Agent: tea
Create tests for HandleX.
Verify: go test passes

## Constraints
- Do not break existing tests
- Follow project conventions

## Dependencies
- /home/user/Projects/shared-lib some shared code
- /tmp/data test fixtures
`

func TestParsePlan_Valid(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(path, []byte(validPlan), 0644)

	p, err := ParsePlan(path)
	if err != nil {
		t.Fatal(err)
	}

	if p.Title != "Plan: Add feature X" {
		t.Errorf("Title = %q", p.Title)
	}
	if len(p.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(p.Steps))
	}

	// Step 1
	if p.Steps[0].Number != 1 {
		t.Errorf("Step 1 number = %d", p.Steps[0].Number)
	}
	if p.Steps[0].Title != "Research codebase" {
		t.Errorf("Step 1 title = %q", p.Steps[0].Title)
	}
	if p.Steps[0].Agent != "analyst" {
		t.Errorf("Step 1 agent = %q", p.Steps[0].Agent)
	}
	if p.Steps[0].Verify != "Report created" {
		t.Errorf("Step 1 verify = %q", p.Steps[0].Verify)
	}

	// Step 2
	if p.Steps[1].Agent != "dev" {
		t.Errorf("Step 2 agent = %q", p.Steps[1].Agent)
	}
	if !strings.Contains(p.Steps[1].Body, "HandleX") {
		t.Error("Step 2 body should mention HandleX")
	}

	// Step 3
	if p.Steps[2].Agent != "tea" {
		t.Errorf("Step 3 agent = %q", p.Steps[2].Agent)
	}
}

func TestParsePlan_Constraints(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(path, []byte(validPlan), 0644)

	p, err := ParsePlan(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Constraints) != 2 {
		t.Fatalf("expected 2 constraints, got %d", len(p.Constraints))
	}
	if !strings.Contains(p.Constraints[0], "break existing") {
		t.Errorf("constraint 0 = %q", p.Constraints[0])
	}
}

func TestParsePlan_Dependencies(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(path, []byte(validPlan), 0644)

	p, err := ParsePlan(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(p.Dependencies) != 2 {
		t.Fatalf("expected 2 dependencies, got %d", len(p.Dependencies))
	}
	if p.Dependencies[0] != "/home/user/Projects/shared-lib" {
		t.Errorf("dep 0 = %q", p.Dependencies[0])
	}
	if p.Dependencies[1] != "/tmp/data" {
		t.Errorf("dep 1 = %q", p.Dependencies[1])
	}
}

func TestParsePlan_NoSteps(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.md")
	os.WriteFile(path, []byte("# Plan: Empty\n\nNo steps here.\n"), 0644)

	_, err := ParsePlan(path)
	if err == nil {
		t.Error("expected error for plan with no steps")
	}
	if !strings.Contains(err.Error(), "no steps") {
		t.Errorf("error should mention 'no steps', got: %v", err)
	}
}

func TestParsePlan_MissingAgent(t *testing.T) {
	content := `# Plan: No Agent

## Steps

### Step 1: Do stuff
Just do the thing.
Verify: It works
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(path, []byte(content), 0644)

	p, err := ParsePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if p.Steps[0].Agent != "" {
		t.Errorf("Agent should be empty when not specified, got %q", p.Steps[0].Agent)
	}
}

func TestParsePlan_FileNotFound(t *testing.T) {
	_, err := ParsePlan("/nonexistent/plan.md")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestPlanPath(t *testing.T) {
	path := PlanPath(42)
	if !strings.HasSuffix(path, "task-42.md") {
		t.Errorf("PlanPath(42) = %q, should end with task-42.md", path)
	}
}

func TestPlanPath_DifferentIDs(t *testing.T) {
	p1 := PlanPath(1)
	p2 := PlanPath(99)
	if p1 == p2 {
		t.Error("different IDs should produce different paths")
	}
	if !strings.HasSuffix(p1, "task-1.md") {
		t.Errorf("PlanPath(1) = %q", p1)
	}
	if !strings.HasSuffix(p2, "task-99.md") {
		t.Errorf("PlanPath(99) = %q", p2)
	}
}

func TestParsePlan_ReadOnlyTrue(t *testing.T) {
	content := `# Plan: ReadOnly test

## Steps

### Step 1: Investigate
Agent: analyst
ReadOnly: true
Read all files.
Verify: findings summarized

### Step 2: Implement
Agent: dev
Edit the code.
Verify: code compiles
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(path, []byte(content), 0644)

	p, err := ParsePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Steps[0].ReadOnly {
		t.Error("Step 1 should be ReadOnly")
	}
	if p.Steps[1].ReadOnly {
		t.Error("Step 2 should NOT be ReadOnly")
	}
}

func TestParsePlan_ReadOnlyYes(t *testing.T) {
	content := `# Plan: ReadOnly yes variant

## Steps

### Step 1: Research
Agent: analyst
ReadOnly: yes
Gather info.
Verify: done
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(path, []byte(content), 0644)

	p, err := ParsePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	if !p.Steps[0].ReadOnly {
		t.Error("Step 1 with ReadOnly: yes should be ReadOnly")
	}
}

func TestParsePlan_ReadOnlyFalseOmitted(t *testing.T) {
	content := `# Plan: No ReadOnly

## Steps

### Step 1: Code
Agent: dev
ReadOnly: false
Write some code.
Verify: works
`
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.md")
	os.WriteFile(path, []byte(content), 0644)

	p, err := ParsePlan(path)
	if err != nil {
		t.Fatal(err)
	}
	// "ReadOnly: false" doesn't match the regex (only true/yes match), so it goes to body
	if p.Steps[0].ReadOnly {
		t.Error("Step with ReadOnly: false should NOT be ReadOnly")
	}
}
