package plan

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/JuanVilla424/teamoon/internal/config"
)

type Step struct {
	Number   int
	Title    string
	Body     string
	Verify   string
	Agent    string
	ReadOnly bool
}

type Plan struct {
	Title        string
	Steps        []Step
	Constraints  []string
	Dependencies []string
	FilePath     string
}

func PlansDir() string {
	return filepath.Join(config.ConfigDir(), "plans")
}

func PlanPath(taskID int) string {
	return filepath.Join(PlansDir(), fmt.Sprintf("task-%d.md", taskID))
}

func PlanExists(taskID int) bool {
	_, err := os.Stat(PlanPath(taskID))
	return err == nil
}

func SavePlan(taskID int, content string) error {
	if err := os.MkdirAll(PlansDir(), 0755); err != nil {
		return err
	}
	return os.WriteFile(PlanPath(taskID), []byte(content), 0644)
}

var (
	stepRe     = regexp.MustCompile(`^###\s+Step\s+(\d+):\s+(.+)$`)
	verifyRe   = regexp.MustCompile(`(?i)^Verify:\s+(.+)$`)
	agentRe    = regexp.MustCompile(`(?i)^Agent:\s+(.+)$`)
	readOnlyRe = regexp.MustCompile(`(?i)^ReadOnly:\s*(true|yes)$`)
)

func ParsePlan(path string) (Plan, error) {
	f, err := os.Open(path)
	if err != nil {
		return Plan{}, err
	}
	defer f.Close()

	p := Plan{FilePath: path}
	scanner := bufio.NewScanner(f)

	type state int
	const (
		stateHeader state = iota
		stateStep
		stateConstraints
		stateDependencies
	)

	current := stateHeader
	var curStep *Step
	var bodyLines []string

	finishStep := func() {
		if curStep != nil {
			curStep.Body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
			p.Steps = append(p.Steps, *curStep)
			curStep = nil
			bodyLines = nil
		}
	}

	for scanner.Scan() {
		line := scanner.Text()

		if strings.HasPrefix(line, "# ") && p.Title == "" {
			p.Title = strings.TrimPrefix(line, "# ")
			continue
		}

		if strings.HasPrefix(line, "## Constraints") {
			finishStep()
			current = stateConstraints
			continue
		}

		if strings.HasPrefix(line, "## Dependencies") {
			finishStep()
			current = stateDependencies
			continue
		}

		if matches := stepRe.FindStringSubmatch(line); matches != nil {
			finishStep()
			current = stateStep
			num := 0
			fmt.Sscanf(matches[1], "%d", &num)
			curStep = &Step{
				Number: num,
				Title:  matches[2],
			}
			bodyLines = nil
			continue
		}

		switch current {
		case stateStep:
			if matches := verifyRe.FindStringSubmatch(line); matches != nil {
				if curStep != nil {
					curStep.Verify = matches[1]
				}
			} else if matches := agentRe.FindStringSubmatch(line); matches != nil {
				if curStep != nil {
					curStep.Agent = strings.TrimSpace(matches[1])
				}
			} else if readOnlyRe.MatchString(line) {
				if curStep != nil {
					curStep.ReadOnly = true
				}
			} else {
				bodyLines = append(bodyLines, line)
			}
		case stateConstraints:
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				p.Constraints = append(p.Constraints, strings.TrimPrefix(trimmed, "- "))
			}
		case stateDependencies:
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				dep := strings.TrimPrefix(trimmed, "- ")
				if idx := strings.Index(dep, " "); idx > 0 {
					dep = dep[:idx]
				}
				p.Dependencies = append(p.Dependencies, dep)
			}
		}
	}

	finishStep()

	if len(p.Steps) == 0 {
		return p, fmt.Errorf("no steps found in plan: %s", path)
	}

	return p, scanner.Err()
}
