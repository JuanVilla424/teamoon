package jobs

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/JuanVilla424/teamoon/internal/backend"
	"github.com/JuanVilla424/teamoon/internal/config"
)

// RunJob spawns a backend session for the given job and captures the result.
func RunJob(ctx context.Context, job Job, cfg config.Config) string {
	SetStatus(job.ID, StatusRunning)

	// Native harvester — no CLI spawn needed
	if job.Instruction == harvesterMarker {
		result := RunHarvester(cfg)
		SetStatus(job.ID, StatusDone)
		SetLastRun(job.ID, result)
		log.Printf("[jobs] job #%d %q finished: %s", job.ID, job.Name, result)
		return result
	}

	projectPath := filepath.Join(cfg.ProjectsDir, job.Project)
	if job.Project == "_system" {
		home, _ := os.UserHomeDir()
		projectPath = home
	} else if _, err := os.Stat(projectPath); err != nil {
		home, _ := os.UserHomeDir()
		projectPath = home
	}

	spawnCtx := ctx
	var cancel context.CancelFunc
	if cfg.Spawn.StepTimeoutMin > 0 {
		spawnCtx, cancel = context.WithTimeout(ctx, time.Duration(cfg.Spawn.StepTimeoutMin)*time.Minute)
	} else {
		spawnCtx, cancel = context.WithTimeout(ctx, 30*time.Minute)
	}
	defer cancel()

	b := backend.Resolve(config.BackendFor(cfg, job.Project))
	env := backend.FilterEnv(os.Environ(), "CLAUDECODE")

	req := backend.SpawnRequest{
		Prompt:     job.Instruction,
		ProjectDir: projectPath,
		WorkDir:    projectPath,
		Model:      b.ResolveModel(cfg.Spawn.Model, "job"),
		Effort:     cfg.Spawn.Effort,
		MaxTurns:   cfg.Spawn.MaxTurns,
		Env:        env,
		Phase:      "job",
		DisallowedTools: []string{
			"AskUserQuestion", "EnterPlanMode", "ExitPlanMode",
		},
	}

	log.Printf("[jobs] job #%d %q running in %s", job.ID, job.Name, projectPath)

	events := make(chan backend.Event, 64)
	var lastText string
	var execErr error

	go func() {
		_, execErr = b.Execute(spawnCtx, req, events)
	}()

	for ev := range events {
		switch ev.Type {
		case "assistant":
			if ev.Text != "" {
				lastText = ev.Text
			}
		case "result":
			if ev.Result != "" {
				lastText = ev.Result
			}
		}
	}

	// Truncate result for storage
	result := lastText
	if len(result) > 500 {
		result = result[:500] + "..."
	}

	if execErr != nil {
		if strings.Contains(execErr.Error(), "signal: killed") || spawnCtx.Err() != nil {
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
