package skills

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

var defaultSkills = []struct {
	ID   string // npx skills add <ID>
	Name string // ~/.agents/skills/<Name>
}{
	// Superpowers (14)
	{"obra/superpowers@brainstorming", "brainstorming"},
	{"obra/superpowers@systematic-debugging", "systematic-debugging"},
	{"obra/superpowers@writing-plans", "writing-plans"},
	{"obra/superpowers@test-driven-development", "test-driven-development"},
	{"obra/superpowers@executing-plans", "executing-plans"},
	{"obra/superpowers@requesting-code-review", "requesting-code-review"},
	{"obra/superpowers@using-superpowers", "using-superpowers"},
	{"obra/superpowers@subagent-driven-development", "subagent-driven-development"},
	{"obra/superpowers@verification-before-completion", "verification-before-completion"},
	{"obra/superpowers@receiving-code-review", "receiving-code-review"},
	{"obra/superpowers@using-git-worktrees", "using-git-worktrees"},
	{"obra/superpowers@writing-skills", "writing-skills"},
	{"obra/superpowers@dispatching-parallel-agents", "dispatching-parallel-agents"},
	{"obra/superpowers@finishing-a-development-branch", "finishing-a-development-branch"},
	// Anthropic (2)
	{"anthropics/skills@frontend-design", "frontend-design"},
	{"anthropics/skills@skill-creator", "skill-creator"},
	// Vercel (5)
	{"vercel-labs/agent-skills@vercel-react-best-practices", "vercel-react-best-practices"},
	{"vercel-labs/agent-skills@web-design-guidelines", "web-design-guidelines"},
	{"vercel-labs/agent-skills@vercel-composition-patterns", "vercel-composition-patterns"},
	{"vercel-labs/agent-skills@vercel-react-native-skills", "vercel-react-native-skills"},
	{"vercel-labs/agent-browser@agent-browser", "agent-browser"},
	// UI/UX Design (1)
	{"nextlevelbuilder/ui-ux-pro-max-skill@ui-ux-pro-max", "ui-ux-pro-max"},
}

func Install() error {
	npxPath, err := exec.LookPath("npx")
	if err != nil {
		return fmt.Errorf("npx not found — install Node.js first (nvm install --lts)")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	skillsDir := filepath.Join(home, ".agents", "skills")

	var failed []string
	for _, skill := range defaultSkills {
		if _, err := os.Stat(filepath.Join(skillsDir, skill.Name)); err == nil {
			continue
		}
		cmd := exec.Command(npxPath, "-y", "skills", "add", skill.ID, "-g", "-y")
		if out, err := cmd.CombinedOutput(); err != nil {
			failed = append(failed, fmt.Sprintf("%s: %s", skill.Name, string(out)))
			continue
		}
	}

	if len(failed) > 0 {
		return fmt.Errorf("some skills failed to install: %v", failed)
	}
	return nil
}

// InstallWithProgress installs skills with per-skill progress callback.
// progress receives (skillName, status) where status is "installing", "done", "skipped", or "error:msg".
func InstallWithProgress(progress func(name, status string)) error {
	npxPath, err := exec.LookPath("npx")
	if err != nil {
		return fmt.Errorf("npx not found — install Node.js first (nvm install --lts)")
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	skillsDir := filepath.Join(home, ".agents", "skills")

	var failed []string
	for _, skill := range defaultSkills {
		if _, err := os.Stat(filepath.Join(skillsDir, skill.Name)); err == nil {
			progress(skill.Name, "skipped")
			continue
		}
		progress(skill.Name, "installing")
		cmd := exec.Command(npxPath, "-y", "skills", "add", skill.ID, "-g", "-y")
		if out, err := cmd.CombinedOutput(); err != nil {
			progress(skill.Name, "error:"+string(out))
			failed = append(failed, skill.Name)
			continue
		}
		progress(skill.Name, "done")
	}

	if len(failed) > 0 {
		return fmt.Errorf("some skills failed: %v", failed)
	}
	return nil
}
