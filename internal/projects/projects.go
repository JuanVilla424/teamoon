package projects

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const templateRepo = "JuanVilla424/github-cicd-template"
const initVersion = "1.0.2"

type Project struct {
	Name       string
	Path       string
	Branch     string
	LastCommit string
	CommitTime time.Time
	Modified   int
	Active     bool
	Stale      bool
	HasGit     bool
	GitHubRepo string
}

type PR struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Author struct {
		Login string `json:"login"`
	} `json:"author"`
	BaseRefName string `json:"baseRefName"`
}

func Scan(projectsDir string) []Project {
	entries, err := os.ReadDir(projectsDir)
	if err != nil {
		return nil
	}

	var projects []Project
	for _, e := range entries {
		if !e.IsDir() || strings.HasPrefix(e.Name(), ".") {
			continue
		}

		path := filepath.Join(projectsDir, e.Name())
		p := Project{
			Name: e.Name(),
			Path: path,
		}

		gitDir := filepath.Join(path, ".git")
		if _, err := os.Stat(gitDir); err == nil {
			p.HasGit = true
			p.Branch = gitCmd(path, "branch", "--show-current")
			p.LastCommit = gitCmd(path, "log", "-1", "--format=%cr")

			remote := gitCmd(path, "remote", "get-url", "origin")
			p.GitHubRepo = parseGitHubRepo(remote)

			commitISO := gitCmd(path, "log", "-1", "--format=%cI")
			if t, err := time.Parse(time.RFC3339, commitISO); err == nil {
				p.CommitTime = t
				p.Active = time.Since(t) < 7*24*time.Hour
				p.Stale = time.Since(t) >= 60*24*time.Hour
			}

			status := gitCmd(path, "status", "--porcelain")
			if status != "" {
				p.Modified = len(strings.Split(strings.TrimSpace(status), "\n"))
			}
		}

		projects = append(projects, p)
	}

	return projects
}

func parseGitHubRepo(remote string) string {
	remote = strings.TrimSpace(remote)
	remote = strings.TrimSuffix(remote, ".git")
	if strings.Contains(remote, "github.com/") {
		parts := strings.SplitN(remote, "github.com/", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	if strings.Contains(remote, "github.com:") {
		parts := strings.SplitN(remote, "github.com:", 2)
		if len(parts) == 2 {
			return parts[1]
		}
	}
	return ""
}

type PRDetail struct {
	Number       int    `json:"number"`
	Title        string `json:"title"`
	Body         string `json:"body"`
	State        string `json:"state"`
	IsDraft      bool   `json:"isDraft"`
	Author       struct {
		Login string `json:"login"`
	} `json:"author"`
	HeadRefName    string `json:"headRefName"`
	BaseRefName    string `json:"baseRefName"`
	ChangedFiles   int    `json:"changedFiles"`
	Additions      int    `json:"additions"`
	Deletions      int    `json:"deletions"`
	Labels         []struct {
		Name string `json:"name"`
	} `json:"labels"`
	ReviewDecision string `json:"reviewDecision"`
	CreatedAt      string `json:"createdAt"`
	UpdatedAt      string `json:"updatedAt"`
	URL            string `json:"url"`
}

func FetchPRDetail(repo string, number int) (*PRDetail, error) {
	if repo == "" {
		return nil, fmt.Errorf("no github repo")
	}
	cmd := exec.Command("gh", "pr", "view",
		fmt.Sprintf("%d", number),
		"--repo", repo,
		"--json", "number,title,body,state,isDraft,author,headRefName,baseRefName,changedFiles,additions,deletions,labels,reviewDecision,createdAt,updatedAt,url",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var detail PRDetail
	if err := json.Unmarshal(out, &detail); err != nil {
		return nil, err
	}
	return &detail, nil
}

func FetchPRs(repo string) ([]PR, error) {
	if repo == "" {
		return nil, fmt.Errorf("no github repo")
	}
	cmd := exec.Command("gh", "pr", "list",
		"--repo", repo,
		"--state", "open",
		"--limit", "50",
		"--json", "number,title,author,baseRefName",
	)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var prs []PR
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, err
	}
	return prs, nil
}

func FilterDependabot(prs []PR) []PR {
	var result []PR
	for _, pr := range prs {
		if pr.Author.Login == "app/dependabot" || pr.Author.Login == "dependabot[bot]" {
			result = append(result, pr)
		}
	}
	return result
}

func MergePR(repo string, number int) error {
	cmd := exec.Command("gh", "pr", "merge",
		fmt.Sprintf("%d", number),
		"--repo", repo,
		"--merge",
	)
	return cmd.Run()
}

func GitPull(projectPath string) (string, error) {
	cmd := exec.Command("git", "pull")
	cmd.Dir = projectPath
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func gitCmd(dir string, args ...string) string {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func gitCmdFull(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// GitInitRepo initializes a local project directory with git and connects to GitHub.
// Returns (output, backupDir, createdNew, error). createdNew=true when a new repo was created from template.
func GitInitRepo(projectPath, name string) (string, string, bool, error) {
	var out strings.Builder

	// 0. Backup
	backupDir := fmt.Sprintf("/tmp/teamoon-backup-%s-%d", name, time.Now().Unix())
	cpCmd := exec.Command("cp", "-a", projectPath, backupDir)
	if cpOut, err := cpCmd.CombinedOutput(); err != nil {
		return string(cpOut), "", false, fmt.Errorf("backup failed: %w", err)
	}
	out.WriteString("backup: " + backupDir + "\n")

	// 1. Auto-detect project type
	projectType := DetectProjectType(projectPath)
	out.WriteString("type: " + projectType + "\n")

	// 2. git init
	res, err := gitCmdFull(projectPath, "init")
	out.WriteString(res + "\n")
	if err != nil {
		return out.String(), backupDir, false, fmt.Errorf("git init: %w", err)
	}

	// 3. Check if GitHub repo exists
	ghCmd := exec.Command("gh", "repo", "view", name, "--json", "sshUrl", "-q", ".sshUrl")
	urlOut, err := ghCmd.Output()

	if err == nil {
		// Path A: repo exists — connect to it
		repoURL := strings.TrimSpace(string(urlOut))
		out.WriteString("remote: " + repoURL + " (existing)\n")

		res, err = gitCmdFull(projectPath, "remote", "add", "origin", repoURL)
		out.WriteString(res + "\n")
		if err != nil {
			return out.String(), backupDir, false, fmt.Errorf("git remote add: %w", err)
		}

		res, err = gitCmdFull(projectPath, "fetch", "--all")
		out.WriteString(res + "\n")
		if err != nil {
			return out.String(), backupDir, false, fmt.Errorf("git fetch: %w", err)
		}

		res, err = gitCmdFull(projectPath, "checkout", "dev")
		if err != nil {
			res, err = gitCmdFull(projectPath, "checkout", "main")
		}
		out.WriteString(res + "\n")
		return out.String(), backupDir, false, err
	}

	// Path B: repo does NOT exist — create from template on GitHub
	out.WriteString("repo not found, creating from template...\n")

	// B1. Create repo FROM template (GitHub copies template content)
	createCmd := exec.Command("gh", "repo", "create", name, "--private",
		"--template", templateRepo)
	if createOut, err := createCmd.CombinedOutput(); err != nil {
		return out.String() + string(createOut), backupDir, false, fmt.Errorf("gh repo create --template: %w", err)
	}
	out.WriteString("created from template: " + name + "\n")

	// Wait for GitHub to finish templating
	time.Sleep(3 * time.Second)

	// B2. Get SSH URL
	ghCmd2 := exec.Command("gh", "repo", "view", name, "--json", "sshUrl", "-q", ".sshUrl")
	urlOut2, err := ghCmd2.Output()
	if err != nil {
		return out.String(), backupDir, false, fmt.Errorf("could not get repo URL: %w", err)
	}
	repoURL := strings.TrimSpace(string(urlOut2))
	out.WriteString("remote: " + repoURL + "\n")

	// B3. Connect local dir to remote and fetch template content
	res, err = gitCmdFull(projectPath, "remote", "add", "origin", repoURL)
	out.WriteString(res + "\n")
	if err != nil {
		return out.String(), backupDir, false, fmt.Errorf("git remote add: %w", err)
	}

	res, err = gitCmdFull(projectPath, "fetch", "origin")
	out.WriteString(res + "\n")
	if err != nil {
		return out.String(), backupDir, false, fmt.Errorf("git fetch: %w", err)
	}

	// B4. Clear local files before pull (backup already exists)
	entries, _ := os.ReadDir(projectPath)
	for _, entry := range entries {
		name := entry.Name()
		if name == ".git" || name == "node_modules" {
			continue
		}
		os.RemoveAll(filepath.Join(projectPath, name))
	}
	out.WriteString("cleared local files for pull\n")

	// B5. Pull
	res, err = gitCmdFull(projectPath, "pull", "origin", "main")
	out.WriteString(res + "\n")
	if err != nil {
		return out.String(), backupDir, false, fmt.Errorf("git pull: %w", err)
	}

	// B5. Initialize submodules
	res, _ = gitCmdFull(projectPath, "submodule", "init")
	out.WriteString(res + "\n")
	res, _ = gitCmdFull(projectPath, "submodule", "update")
	out.WriteString(res + "\n")

	// B6. Create dev branch
	res, _ = gitCmdFull(projectPath, "checkout", "-b", "dev")
	out.WriteString(res + "\n")

	out.WriteString("repo created, task will handle cleanup\n")
	return out.String(), backupDir, true, nil
}

// DetectProjectType returns "node", "python", or "go" based on files present.
func DetectProjectType(projectPath string) string {
	if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err == nil {
		return "node"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
		return "go"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "pyproject.toml")); err == nil {
		return "python"
	}
	if _, err := os.Stat(filepath.Join(projectPath, "requirements.txt")); err == nil {
		return "python"
	}
	return "node"
}
