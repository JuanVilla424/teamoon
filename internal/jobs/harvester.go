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

	// Phase 2: create security tasks for projects without pending ones
	for _, p := range projs {
		if !p.HasGit {
			continue
		}
		if hasSecurityTask(p.Name) {
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

	return fmt.Sprintf("Merged %d dependabot PRs (%d failed), created %d security tasks", totalMerged, totalFailed, tasksCreated)
}

// hasSecurityTask checks if the project already has a non-done security harvest task.
func hasSecurityTask(project string) bool {
	pending, err := queue.ListPending()
	if err != nil {
		return false
	}
	for _, t := range pending {
		if t.Project == project && strings.Contains(t.Description, "Security Harvest") {
			return true
		}
	}
	return false
}
