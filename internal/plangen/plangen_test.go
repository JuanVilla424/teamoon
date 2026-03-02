package plangen

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/queue"
	"github.com/JuanVilla424/teamoon/internal/uploads"
)

// setupTestUploads creates a temp HOME with uploads store and files, returning cleanup func.
func setupTestUploads(t *testing.T, attachments []uploads.Attachment, files map[string]string) func() {
	t.Helper()
	origHome := os.Getenv("HOME")

	tmpHome := t.TempDir()
	os.Setenv("HOME", tmpHome)

	cfgDir := filepath.Join(tmpHome, ".config", "teamoon")
	uploadsDir := filepath.Join(cfgDir, "uploads")
	os.MkdirAll(uploadsDir, 0755)

	// Write store
	store := struct {
		Attachments []uploads.Attachment `json:"attachments"`
	}{Attachments: attachments}
	data, _ := json.Marshal(store)
	os.WriteFile(filepath.Join(cfgDir, "uploads.json"), data, 0644)

	// Write files
	for name, content := range files {
		os.WriteFile(filepath.Join(uploadsDir, name), []byte(content), 0644)
	}

	return func() {
		os.Setenv("HOME", origHome)
	}
}

func TestBuildAttachmentBlock_Empty(t *testing.T) {
	result := buildAttachmentBlock(nil)
	if result != "" {
		t.Errorf("expected empty string for nil ids, got %q", result)
	}

	result = buildAttachmentBlock([]string{})
	if result != "" {
		t.Errorf("expected empty string for empty ids, got %q", result)
	}
}

func TestBuildAttachmentBlock_TextFile(t *testing.T) {
	atts := []uploads.Attachment{
		{ID: "abc123", OrigName: "notes.txt", MIMEType: "text/plain", Size: 11, StoredName: "abc123.txt"},
	}
	cleanup := setupTestUploads(t, atts, map[string]string{
		"abc123.txt": "hello world",
	})
	defer cleanup()

	result := buildAttachmentBlock([]string{"abc123"})
	if !strings.Contains(result, "### notes.txt") {
		t.Errorf("expected filename header, got %q", result)
	}
	if !strings.Contains(result, "hello world") {
		t.Errorf("expected file content, got %q", result)
	}
}

func TestBuildAttachmentBlock_TruncationCap(t *testing.T) {
	bigContent := strings.Repeat("x", 6000)
	atts := []uploads.Attachment{
		{ID: "big1", OrigName: "big.txt", MIMEType: "text/plain", Size: 6000, StoredName: "big1.txt"},
	}
	cleanup := setupTestUploads(t, atts, map[string]string{
		"big1.txt": bigContent,
	})
	defer cleanup()

	result := buildAttachmentBlock([]string{"big1"})
	if !strings.Contains(result, "[...truncated]") {
		t.Error("expected truncation marker for large file")
	}
	// Content should be capped at maxAttachmentChars (5000) + truncation marker
	lines := strings.Split(result, "\n")
	contentLen := 0
	for _, l := range lines {
		if l != "" && !strings.HasPrefix(l, "###") && l != "[...truncated]" {
			contentLen += len(l)
		}
	}
	if contentLen > maxAttachmentChars+100 {
		t.Errorf("content too long after truncation: %d chars", contentLen)
	}
}

func TestBuildAttachmentBlock_BinarySkipped(t *testing.T) {
	atts := []uploads.Attachment{
		{ID: "img1", OrigName: "photo.png", MIMEType: "image/png", Size: 50000, StoredName: "img1.png"},
	}
	cleanup := setupTestUploads(t, atts, map[string]string{})
	defer cleanup()

	result := buildAttachmentBlock([]string{"img1"})
	if !strings.Contains(result, "photo.png (binary, 50000 bytes)") {
		t.Errorf("expected binary marker, got %q", result)
	}
}

func TestBuildAttachmentBlock_MultipleMixed(t *testing.T) {
	atts := []uploads.Attachment{
		{ID: "t1", OrigName: "readme.md", MIMEType: "text/markdown", Size: 100, StoredName: "t1.md"},
		{ID: "b1", OrigName: "app.exe", MIMEType: "application/octet-stream", Size: 99999, StoredName: "b1.exe"},
		{ID: "t2", OrigName: "config.json", MIMEType: "application/json", Size: 50, StoredName: "t2.json"},
	}
	cleanup := setupTestUploads(t, atts, map[string]string{
		"t1.md":    "# Readme content",
		"t2.json":  `{"key": "value"}`,
	})
	defer cleanup()

	result := buildAttachmentBlock([]string{"t1", "b1", "t2"})
	if !strings.Contains(result, "### readme.md") {
		t.Error("missing readme.md header")
	}
	if !strings.Contains(result, "# Readme content") {
		t.Error("missing readme content")
	}
	if !strings.Contains(result, "app.exe (binary, 99999 bytes)") {
		t.Error("missing binary marker for app.exe")
	}
	if !strings.Contains(result, "### config.json") {
		t.Error("missing config.json header")
	}
	if !strings.Contains(result, `"key": "value"`) {
		t.Error("missing config.json content")
	}
}

func TestBuildAttachmentBlock_TotalBudgetExceeded(t *testing.T) {
	// 5 files x 5000 chars each = 25000 > maxTotalChars (20000)
	content := strings.Repeat("a", 4500)
	atts := make([]uploads.Attachment, 5)
	files := make(map[string]string)
	ids := make([]string, 5)
	for i := 0; i < 5; i++ {
		id := strings.Repeat(string(rune('a'+i)), 4)
		stored := id + ".txt"
		atts[i] = uploads.Attachment{
			ID: id, OrigName: id + ".txt", MIMEType: "text/plain", Size: 4500, StoredName: stored,
		}
		files[stored] = content
		ids[i] = id
	}
	cleanup := setupTestUploads(t, atts, files)
	defer cleanup()

	result := buildAttachmentBlock(ids)
	// Should not exceed total budget
	if len(result) > maxTotalChars+5000 { // some overhead for headers
		t.Errorf("total result too large: %d chars (budget %d)", len(result), maxTotalChars)
	}
	// At least one file should be truncated or the last file should be missing
	if !strings.Contains(result, "[...truncated]") && strings.Contains(result, "eeee.txt\n"+content) {
		t.Error("expected budget enforcement — either truncation or omission of last files")
	}
}

func TestBuildPlanPrompt_WithAttachments(t *testing.T) {
	atts := []uploads.Attachment{
		{ID: "att1", OrigName: "spec.txt", MIMEType: "text/plain", Size: 12, StoredName: "att1.txt"},
	}
	cleanup := setupTestUploads(t, atts, map[string]string{
		"att1.txt": "feature spec",
	})
	defer cleanup()

	task := queue.Task{
		ID:          1,
		Project:     "testproj",
		Description: "implement feature",
		Attachments: []string{"att1"},
	}
	result := BuildPlanPrompt(task, "SKELETON_BLOCK", "/projects")
	if !strings.Contains(result, "CONTEXT FROM ATTACHMENTS:") {
		t.Error("expected CONTEXT FROM ATTACHMENTS section")
	}
	if !strings.Contains(result, "feature spec") {
		t.Error("expected attachment content in prompt")
	}
	if !strings.Contains(result, "spec.txt") {
		t.Error("expected attachment filename in prompt")
	}
}

func TestBuildPlanPrompt_NoAttachments(t *testing.T) {
	task := queue.Task{
		ID:          2,
		Project:     "testproj",
		Description: "simple task",
	}
	result := BuildPlanPrompt(task, "SKELETON_BLOCK", "/projects")
	if strings.Contains(result, "CONTEXT FROM ATTACHMENTS") {
		t.Error("should not contain CONTEXT FROM ATTACHMENTS when no attachments")
	}
	if !strings.Contains(result, "simple task") {
		t.Error("expected task description in prompt")
	}
	if !strings.Contains(result, "SKELETON_BLOCK") {
		t.Error("expected skeleton block in prompt")
	}
}

func TestSkeletonJSON_DocSetup(t *testing.T) {
	sk := config.SkeletonConfig{DocSetup: true}
	hints := config.DefaultPhaseHints()
	result := SkeletonJSON(sk, nil, hints)
	if !strings.Contains(result, `"id": "doc_setup"`) {
		t.Error("expected doc_setup phase in skeleton JSON")
	}
	if !strings.Contains(result, `"enabled": true`) {
		t.Error("expected enabled: true for doc_setup")
	}
}

func TestSkeletonJSON_NoDocSetup(t *testing.T) {
	sk := config.SkeletonConfig{DocSetup: false}
	result := SkeletonJSON(sk, nil, nil)
	if !strings.Contains(result, `"id": "doc_setup"`) {
		t.Error("doc_setup phase should always be present")
	}
	// Parse to verify enabled is false
	var parsed struct {
		Phases []struct {
			ID      string `json:"id"`
			Enabled bool   `json:"enabled"`
		} `json:"phases"`
	}
	json.Unmarshal([]byte(result), &parsed)
	for _, p := range parsed.Phases {
		if p.ID == "doc_setup" && p.Enabled {
			t.Error("doc_setup should be disabled when DocSetup=false")
		}
	}
}

func TestSkeletonJSON_DocSetupFirst(t *testing.T) {
	sk := config.SkeletonConfig{DocSetup: true, WebSearch: true, Test: true}
	hints := config.DefaultPhaseHints()
	result := SkeletonJSON(sk, nil, hints)
	var parsed struct {
		Phases []struct {
			ID string `json:"id"`
		} `json:"phases"`
	}
	json.Unmarshal([]byte(result), &parsed)
	if len(parsed.Phases) == 0 {
		t.Fatal("no phases in skeleton JSON")
	}
	if parsed.Phases[0].ID != "doc_setup" {
		t.Errorf("expected doc_setup as first phase, got %s", parsed.Phases[0].ID)
	}
}

func TestSkeletonJSON_HintsIncluded(t *testing.T) {
	sk := config.SkeletonConfig{DocSetup: true}
	hints := map[string]string{"doc_setup": "create ARCHITECT.md"}
	result := SkeletonJSON(sk, nil, hints)
	if !strings.Contains(result, "create ARCHITECT.md") {
		t.Error("expected hint text in skeleton JSON")
	}
}

func TestSkeletonJSON_MCPSteps(t *testing.T) {
	sk := config.SkeletonConfig{Test: true}
	mcpServers := map[string]config.MCPServer{
		"context7": {
			Enabled:      true,
			SkeletonStep: &config.SkeletonStep{Label: "Context7 Lookup", Prompt: "query docs", ReadOnly: true},
		},
	}
	result := SkeletonJSON(sk, mcpServers, nil)
	if !strings.Contains(result, "Context7 Lookup") {
		t.Error("expected MCP step label in skeleton JSON")
	}
	if !strings.Contains(result, "query docs") {
		t.Error("expected MCP step prompt in skeleton JSON")
	}
}

func TestBuildPlanPrompt_PartyMode(t *testing.T) {
	task := queue.Task{
		ID:          3,
		Project:     "webproj",
		Description: "fix ui bug",
	}
	result := BuildPlanPrompt(task, "{}", "/projects")
	if !strings.Contains(result, "party-mode") {
		t.Error("expected party-mode skill invocation in plan prompt")
	}
}

func TestBuildPlanPrompt_OrderingHint(t *testing.T) {
	task := queue.Task{ID: 4, Project: "p", Description: "d"}
	result := BuildPlanPrompt(task, "{}", "/projects")
	if !strings.Contains(result, "Respect ordering hints") {
		t.Error("expected ordering hint instruction in plan prompt")
	}
}

func TestBuildPlanPrompt_FrontendFirstMethodology(t *testing.T) {
	task := queue.Task{ID: 5, Project: "p", Description: "d"}
	result := BuildPlanPrompt(task, "{}", "/projects")
	if !strings.Contains(result, "FRONTEND FIRST") {
		t.Error("expected FRONTEND FIRST methodology in plan prompt")
	}
	if !strings.Contains(result, "MOCK CLEANUP") {
		t.Error("expected MOCK CLEANUP methodology in plan prompt")
	}
	if !strings.Contains(result, "BACKEND IMPLEMENTATION") {
		t.Error("expected BACKEND IMPLEMENTATION methodology in plan prompt")
	}
	if !strings.Contains(result, "MOCK_ prefix") {
		t.Error("expected MOCK_ prefix convention in plan prompt")
	}
}
