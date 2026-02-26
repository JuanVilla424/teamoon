package jobs

import (
	"bufio"
	"context"
	"encoding/json"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/engine"
)

// RunJob spawns a Claude session for the given job and captures the result.
func RunJob(ctx context.Context, job Job, cfg config.Config) string {
	SetStatus(job.ID, StatusRunning)

	projectPath := filepath.Join(cfg.ProjectsDir, job.Project)
	if job.Project == "_system" {
		home, _ := os.UserHomeDir()
		projectPath = home
	} else if _, err := os.Stat(projectPath); err != nil {
		home, _ := os.UserHomeDir()
		projectPath = home
	}

	args, cleanup := engine.BuildSpawnArgs(cfg, job.Instruction, nil)
	if cleanup != nil {
		defer cleanup()
	}

	spawnCtx := ctx
	var cancel context.CancelFunc
	if cfg.Spawn.StepTimeoutMin > 0 {
		spawnCtx, cancel = context.WithTimeout(ctx, time.Duration(cfg.Spawn.StepTimeoutMin)*time.Minute)
	} else {
		spawnCtx, cancel = context.WithTimeout(ctx, 30*time.Minute)
	}
	defer cancel()

	env := os.Environ()
	cmd := exec.CommandContext(spawnCtx, "claude", args...)
	cmd.Env = env
	cmd.Dir = projectPath

	devNull, _ := os.Open(os.DevNull)
	if devNull != nil {
		cmd.Stdin = devNull
		defer devNull.Close()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		result := "failed to create pipe: " + err.Error()
		SetStatus(job.ID, StatusError)
		SetLastRun(job.ID, result)
		return result
	}
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		result := "failed to start claude: " + err.Error()
		SetStatus(job.ID, StatusError)
		SetLastRun(job.ID, result)
		return result
	}

	log.Printf("[jobs] job #%d %q running in %s", job.ID, job.Name, projectPath)

	var lastText string
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 256*1024), 256*1024)
	for scanner.Scan() {
		line := scanner.Text()
		var event struct {
			Type    string `json:"type"`
			Result  string `json:"result,omitempty"`
			Message *struct {
				Content []struct {
					Type string `json:"type"`
					Text string `json:"text,omitempty"`
				} `json:"content"`
			} `json:"message,omitempty"`
		}
		if json.Unmarshal([]byte(line), &event) != nil {
			continue
		}
		switch event.Type {
		case "assistant":
			if event.Message != nil {
				for _, c := range event.Message.Content {
					if c.Type == "text" && c.Text != "" {
						lastText = c.Text
					}
				}
			}
		case "result":
			if event.Result != "" {
				lastText = event.Result
			}
		}
	}

	err = cmd.Wait()

	// Truncate result for storage
	result := lastText
	if len(result) > 500 {
		result = result[:500] + "..."
	}

	if err != nil {
		if strings.Contains(err.Error(), "signal: killed") || spawnCtx.Err() != nil {
			result = "timeout: " + result
		}
		SetStatus(job.ID, StatusError)
	} else {
		SetStatus(job.ID, StatusDone)
	}

	SetLastRun(job.ID, result)
	log.Printf("[jobs] job #%d %q finished", job.ID, job.Name)
	return result
}
