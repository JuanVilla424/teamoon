package jobs

import (
	"fmt"
	"log"
	"strings"

	"github.com/JuanVilla424/teamoon/internal/config"
	"github.com/JuanVilla424/teamoon/internal/projects"
	"github.com/JuanVilla424/teamoon/internal/queue"
)

const harvesterMarker = "__harvester__"

const securityInstruction = `Security Harvest: Run /security-review to scan the entire codebase for vulnerabilities.
Read CONTRIBUTING.md for this project's contribution guidelines.
For each finding with severity Critical or High, apply the fix following the project's coding standards.
Commit fixes with the appropriate commit type (fix for vulnerabilities, chore for dependency updates).`

// RunHarvester scans all projects, merges dependabot PRs, and creates security tasks.
func RunHarvester(cfg config.Config) string {
	projs := projects.Scan(cfg.ProjectsDir)

	totalMerged := 0
	totalFailed := 0
	tasksCreated := 0

	// Phase 1: merge dependabot PRs and pull changes
	for _, p := range projs {
		if p.GitHubRepo == "" {
			continue
		}
		prs, err := projects.FetchPRs(p.GitHubRepo)
		if err != nil {
			log.Printf("[harvester] %s: failed to fetch PRs: %v", p.Name, err)
			continue
		}
		depBot := projects.FilterDependabot(prs)
		for _, pr := range depBot {
			if err := projects.MergePR(p.GitHubRepo, pr.Number); err != nil {
				log.Printf("[harvester] %s: failed to merge PR #%d: %v", p.Name, pr.Number, err)
				totalFailed++
			} else {
				log.Printf("[harvester] %s: merged dependabot PR #%d", p.Name, pr.Number)
				totalMerged++
			}
		}
		if len(depBot) > 0 {
			projects.GitPull(p.Path)
		}
	}

	// Phase 2: ensure one security task per project — recycle existing or create new
	tasksRecycled := 0
	for _, p := range projs {
		if !p.HasGit {
			continue
		}
		existing := findSecurityTask(p.Name)
		if existing != nil {
			// Task already exists — if done, recycle it (replan)
			state := queue.EffectiveState(*existing)
			if state == queue.StateDone || state == queue.StateArchived {
				queue.ResetPlan(existing.ID)
				queue.ForceUpdateState(existing.ID, queue.StatePending)
				queue.SetFailReason(existing.ID, "")
				if !existing.AutoPilot {
					queue.ToggleAutoPilot(existing.ID)
				}
				tasksRecycled++
				log.Printf("[harvester] %s: recycled security task #%d", p.Name, existing.ID)
			} else if !existing.AutoPilot {
				// Pending/planned/running but autopilot off — enable it so it executes
				queue.ToggleAutoPilot(existing.ID)
				log.Printf("[harvester] %s: enabled autopilot on security task #%d", p.Name, existing.ID)
			}
			continue
		}
		t, err := queue.Add(p.Name, securityInstruction, "med")
		if err != nil {
			log.Printf("[harvester] %s: failed to create task: %v", p.Name, err)
			continue
		}
		queue.ToggleAutoPilot(t.ID)
		tasksCreated++
		log.Printf("[harvester] %s: created security task #%d", p.Name, t.ID)
	}

	return fmt.Sprintf("Merged %d dependabot PRs (%d failed), created %d / recycled %d security tasks", totalMerged, totalFailed, tasksCreated, tasksRecycled)
}

// findSecurityTask finds the existing security harvest task for a project (any state).
func findSecurityTask(project string) *queue.Task {
	all, err := queue.ListAll()
	if err != nil {
		return nil
	}
	for i := range all {
		if all[i].Project == project && strings.Contains(all[i].Description, "Security Harvest") {
			return &all[i]
		}
	}
	return nil
}
