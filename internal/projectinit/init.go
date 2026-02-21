package projectinit

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const templateRepo = "JuanVilla424/github-cicd-template"

type InitRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Version  string `json:"version"`
	Private  bool   `json:"private"`
	Separate bool   `json:"separate"`
}

type StepResult struct {
	Step    int    `json:"step"`
	Name    string `json:"name"`
	Status  string `json:"status"`
	Message string `json:"message,omitempty"`
}

type ProgressFunc func(StepResult)

func RunInit(req InitRequest, projectsDir string, progress ProgressFunc) error {
	if req.Separate {
		return runSeparateInit(req, projectsDir, progress)
	}
	return runSingleInit(req, projectsDir, progress)
}

func runSingleInit(req InitRequest, projectsDir string, progress ProgressFunc) error {
	steps := []struct {
		name string
		fn   func() error
	}{
		{"Creating repository", func() error { return stepCreateRepo(req, projectsDir) }},
		{"Fetching template files", func() error { return stepFetchTemplate(req, projectsDir) }},
		{"Setting up workflows", func() error { return stepSetupWorkflows(req, projectsDir) }},
		{"Creating dev branch", func() error { return stepCreateDevBranch(req, projectsDir) }},
		{"Cleaning template files", func() error { return stepCleanTemplate(req, projectsDir) }},
		{"Trimming workflows", func() error { return stepTrimWorkflows(req, projectsDir) }},
		{"Updating manifest", func() error { return stepUpdateManifest(req, projectsDir) }},
		{"Updating README", func() error { return stepUpdateReadme(req, projectsDir) }},
		{"Setting up environment", func() error { return stepSetupEnv(req, projectsDir) }},
		{"Committing and pushing", func() error { return stepCommitPush(req, projectsDir) }},
	}

	for i, step := range steps {
		progress(StepResult{Step: i + 1, Name: step.name, Status: "running"})
		if err := step.fn(); err != nil {
			progress(StepResult{Step: i + 1, Name: step.name, Status: "error", Message: err.Error()})
			return fmt.Errorf("step %d (%s): %w", i+1, step.name, err)
		}
		progress(StepResult{Step: i + 1, Name: step.name, Status: "done"})
	}
	return nil
}

func runSeparateInit(req InitRequest, projectsDir string, progress ProgressFunc) error {
	// Backend repo uses the original type
	backReq := req
	backReq.Separate = false

	// Frontend repo is always node
	frontReq := InitRequest{
		Name:    req.Name + "-frontend",
		Type:    "node",
		Version: req.Version,
		Private: req.Private,
	}

	stepNum := 0
	emit := func(name string, fn func() error) error {
		stepNum++
		progress(StepResult{Step: stepNum, Name: name, Status: "running"})
		if err := fn(); err != nil {
			progress(StepResult{Step: stepNum, Name: name, Status: "error", Message: err.Error()})
			return err
		}
		progress(StepResult{Step: stepNum, Name: name, Status: "done"})
		return nil
	}

	// Backend steps (1-5)
	if err := emit("Creating backend repo ("+backReq.Name+")", func() error { return stepCreateRepo(backReq, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Fetching backend template", func() error { return stepFetchTemplate(backReq, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Setting up backend workflows", func() error {
		if e := stepSetupWorkflows(backReq, projectsDir); e != nil {
			return e
		}
		if e := stepCreateDevBranch(backReq, projectsDir); e != nil {
			return e
		}
		return stepCleanTemplate(backReq, projectsDir)
	}); err != nil {
		return err
	}
	if err := emit("Configuring backend ("+backReq.Type+")", func() error {
		if e := stepTrimWorkflows(backReq, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateManifest(backReq, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateReadme(backReq, projectsDir); e != nil {
			return e
		}
		return stepSetupEnv(backReq, projectsDir)
	}); err != nil {
		return err
	}
	if err := emit("Pushing backend repo", func() error { return stepCommitPush(backReq, projectsDir) }); err != nil {
		return err
	}

	// Frontend steps (6-10)
	if err := emit("Creating frontend repo ("+frontReq.Name+")", func() error { return stepCreateRepo(frontReq, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Fetching frontend template", func() error { return stepFetchTemplate(frontReq, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Setting up frontend workflows", func() error {
		if e := stepSetupWorkflows(frontReq, projectsDir); e != nil {
			return e
		}
		if e := stepCreateDevBranch(frontReq, projectsDir); e != nil {
			return e
		}
		return stepCleanTemplate(frontReq, projectsDir)
	}); err != nil {
		return err
	}
	if err := emit("Configuring frontend (node)", func() error {
		if e := stepTrimWorkflows(frontReq, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateManifest(frontReq, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateReadme(frontReq, projectsDir); e != nil {
			return e
		}
		return stepSetupEnv(frontReq, projectsDir)
	}); err != nil {
		return err
	}
	if err := emit("Pushing frontend repo", func() error { return stepCommitPush(frontReq, projectsDir) }); err != nil {
		return err
	}

	return nil
}

func projectDir(req InitRequest, projectsDir string) string {
	return filepath.Join(projectsDir, req.Name)
}

func runCmd(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func stepCreateRepo(req InitRequest, projectsDir string) error {
	vis := "--public"
	if req.Private {
		vis = "--private"
	}
	_, err := runCmd(projectsDir, "gh", "repo", "create", req.Name, vis, "--clone")
	return err
}

func stepFetchTemplate(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	// Download template archive and extract
	_, err := runCmd(dir, "gh", "repo", "clone", templateRepo, ".tmpl-src", "--", "--depth=1")
	if err != nil {
		return fmt.Errorf("clone template: %w", err)
	}
	return nil
}

func stepSetupWorkflows(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	tmplDir := filepath.Join(dir, ".tmpl-src")

	// Copy .github/workflows from template
	srcWf := filepath.Join(tmplDir, ".github", "workflows")
	dstWf := filepath.Join(dir, ".github", "workflows")
	if _, err := os.Stat(srcWf); err == nil {
		os.MkdirAll(dstWf, 0755)
		entries, _ := os.ReadDir(srcWf)
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			data, err := os.ReadFile(filepath.Join(srcWf, e.Name()))
			if err != nil {
				continue
			}
			os.WriteFile(filepath.Join(dstWf, e.Name()), data, 0644)
		}
	}
	return nil
}

func stepCreateDevBranch(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	_, err := runCmd(dir, "git", "checkout", "-b", "dev")
	return err
}

func stepCleanTemplate(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	// Remove CNAME and template source
	os.Remove(filepath.Join(dir, "CNAME"))
	os.Remove(filepath.Join(dir, ".tmpl-src", ".git", "config"))
	os.RemoveAll(filepath.Join(dir, ".tmpl-src"))
	return nil
}

func stepTrimWorkflows(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	wfDir := filepath.Join(dir, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		return nil // no workflows dir, skip
	}

	keep := map[string]bool{"ci.yml": true}
	switch req.Type {
	case "python":
		keep["python.yml"] = true
	case "node":
		keep["node.yml"] = true
	case "go":
		keep["go.yml"] = true
	}

	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !keep[e.Name()] {
			os.Remove(filepath.Join(wfDir, e.Name()))
		}
	}
	return nil
}

func stepUpdateManifest(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	version := req.Version
	if version == "" {
		version = "1.0.0"
	}

	switch req.Type {
	case "python":
		content := fmt.Sprintf(`[project]
name = "%s"
version = "%s"
description = ""
requires-python = ">=3.11"
`, req.Name, version)
		return os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(content), 0644)

	case "node":
		content := fmt.Sprintf(`{
  "name": "%s",
  "version": "%s",
  "private": true,
  "scripts": {
    "dev": "echo 'dev'",
    "build": "echo 'build'",
    "test": "echo 'test'"
  }
}
`, req.Name, version)
		return os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644)

	case "go":
		content := fmt.Sprintf("module github.com/JuanVilla424/%s\n\ngo 1.23\n", req.Name)
		return os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644)
	}
	return nil
}

func stepUpdateReadme(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	typeBadge := ""
	switch req.Type {
	case "python":
		typeBadge = "![Python](https://img.shields.io/badge/Python-3.11%2B-blue.svg)"
	case "node":
		typeBadge = "![Node.js](https://img.shields.io/badge/Node.js-20%2B-green.svg)"
	case "go":
		typeBadge = "![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8.svg)"
	}
	content := fmt.Sprintf("# %s\n\n%s\n![Status](https://img.shields.io/badge/Status-Active-green.svg)\n\n## Getting Started\n\nTBD\n", req.Name, typeBadge)
	return os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0644)
}

func stepSetupEnv(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	switch req.Type {
	case "python":
		_, err := runCmd(dir, "python3", "-m", "venv", "venv")
		return err
	case "node":
		// package.json already created in manifest step
		return nil
	case "go":
		// go.mod already created in manifest step
		return nil
	}
	return nil
}

func stepCommitPush(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	if _, err := runCmd(dir, "git", "add", "."); err != nil {
		return err
	}
	if _, err := runCmd(dir, "git", "commit", "-m", "feat(core): initial project scaffold"); err != nil {
		return err
	}
	_, err := runCmd(dir, "git", "push", "-u", "origin", "dev")
	return err
}
