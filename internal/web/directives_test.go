package web

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"testing"
)

// These tests verify the (?s) regex used to parse [TASK_CREATE] and [PROJECT_INIT]
// directives from Claude's chat responses. The root-cause bug was that Go's .*?
// does NOT match newlines without (?s), so multiline JSON between tags was silently
// ignored — the LLM said "20 tasks created" but 0 appeared in the queue.

var taskDirectiveRe = regexp.MustCompile(`(?s)\[TASK_CREATE\](.*?)\[/TASK_CREATE\]`)
var initDirectiveRe = regexp.MustCompile(`(?s)\[PROJECT_INIT\](.*?)\[/PROJECT_INIT\]`)

type taskDirective struct {
	Description string `json:"description"`
	Priority    string `json:"priority"`
	Assignee    string `json:"assignee"`
}

func TestDirectiveRegex_MultilineTaskCreate(t *testing.T) {
	// ROOT CAUSE regression test: 20 multiline directives must all match
	var sb strings.Builder
	sb.WriteString("Here are 20 tasks:\n")
	for i := 1; i <= 20; i++ {
		sb.WriteString("[TASK_CREATE]\n")
		sb.WriteString(fmt.Sprintf(`{"description":"Task %d","priority":"med","assignee":"agent"}`, i))
		sb.WriteString("\n[/TASK_CREATE]\n")
	}

	matches := taskDirectiveRe.FindAllStringSubmatch(sb.String(), -1)
	if len(matches) != 20 {
		t.Fatalf("expected 20 matches, got %d — multiline regex is broken", len(matches))
	}

	for i, m := range matches {
		raw := strings.TrimSpace(m[1])
		var td taskDirective
		if err := json.Unmarshal([]byte(raw), &td); err != nil {
			t.Errorf("match[%d] JSON parse error: %v — raw: %q", i, err, raw)
			continue
		}
		expected := fmt.Sprintf("Task %d", i+1)
		if td.Description != expected {
			t.Errorf("match[%d] description=%q, want %q", i, td.Description, expected)
		}
	}
}

func TestDirectiveRegex_SingleLine(t *testing.T) {
	response := `[TASK_CREATE]{"description":"inline task","priority":"low","assignee":"human"}[/TASK_CREATE]`
	matches := taskDirectiveRe.FindAllStringSubmatch(response, -1)
	if len(matches) != 1 {
		t.Fatalf("expected 1 match, got %d", len(matches))
	}
	var td taskDirective
	if err := json.Unmarshal([]byte(strings.TrimSpace(matches[0][1])), &td); err != nil {
		t.Fatal(err)
	}
	if td.Description != "inline task" {
		t.Errorf("description=%q, want 'inline task'", td.Description)
	}
}

func TestDirectiveRegex_NoDirectives(t *testing.T) {
	response := "Here is my analysis of your project. No tasks needed."
	matches := taskDirectiveRe.FindAllStringSubmatch(response, -1)
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
}

func TestDirectiveRegex_InvalidJSON(t *testing.T) {
	response := `[TASK_CREATE]not valid json[/TASK_CREATE]`
	matches := taskDirectiveRe.FindAllStringSubmatch(response, -1)
	if len(matches) != 1 {
		t.Fatalf("regex should still match the tags, got %d", len(matches))
	}
	var td taskDirective
	err := json.Unmarshal([]byte(strings.TrimSpace(matches[0][1])), &td)
	if err == nil {
		t.Error("expected JSON parse error for invalid content")
	}
}

func TestDirectiveRegex_MixedValid(t *testing.T) {
	response := `[TASK_CREATE]bad-json[/TASK_CREATE]
[TASK_CREATE]
{"description":"valid task","priority":"high","assignee":"agent"}
[/TASK_CREATE]
[TASK_CREATE]{"description":"also valid"}[/TASK_CREATE]`

	matches := taskDirectiveRe.FindAllStringSubmatch(response, -1)
	if len(matches) != 3 {
		t.Fatalf("expected 3 regex matches, got %d", len(matches))
	}

	var validCount int
	for _, m := range matches {
		var td taskDirective
		if err := json.Unmarshal([]byte(strings.TrimSpace(m[1])), &td); err == nil && td.Description != "" {
			validCount++
		}
	}
	if validCount != 2 {
		t.Errorf("expected 2 valid tasks, got %d", validCount)
	}
}

func TestDirectiveRegex_ProjectInit_Multiline(t *testing.T) {
	response := `Creating the project now:
[PROJECT_INIT]
{
  "name": "tetris",
  "type": "node",
  "private": false
}
[/PROJECT_INIT]
Done!`

	matches := initDirectiveRe.FindAllStringSubmatch(response, -1)
	if len(matches) != 1 {
		t.Fatalf("expected 1 PROJECT_INIT match, got %d", len(matches))
	}

	type initReq struct {
		Name    string `json:"name"`
		Type    string `json:"type"`
		Private bool   `json:"private"`
	}
	var req initReq
	if err := json.Unmarshal([]byte(strings.TrimSpace(matches[0][1])), &req); err != nil {
		t.Fatalf("JSON parse error: %v", err)
	}
	if req.Name != "tetris" {
		t.Errorf("name=%q, want 'tetris'", req.Name)
	}
	if req.Type != "node" {
		t.Errorf("type=%q, want 'node'", req.Type)
	}
}
