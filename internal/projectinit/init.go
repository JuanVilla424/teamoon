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
		{"Creating changelog", func() error { return stepCreateChangelog(req, projectsDir) }},
		{"Creating version config", func() error { return stepCreateBumpversion(req, projectsDir) }},
		{"Updating README", func() error { return stepUpdateReadme(req, projectsDir) }},
		{"Setting up environment", func() error { return stepSetupEnv(req, projectsDir) }},
		{"Installing Claude Code hooks", func() error { return InstallHooks(projectDir(req, projectsDir), req.Name, req.Type) }},
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
		if e := stepCreateChangelog(backReq, projectsDir); e != nil {
			return e
		}
		if e := stepCreateBumpversion(backReq, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateReadme(backReq, projectsDir); e != nil {
			return e
		}
		if e := stepSetupEnv(backReq, projectsDir); e != nil {
			return e
		}
		return InstallHooks(projectDir(backReq, projectsDir), backReq.Name, backReq.Type)
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
		if e := stepCreateChangelog(frontReq, projectsDir); e != nil {
			return e
		}
		if e := stepCreateBumpversion(frontReq, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateReadme(frontReq, projectsDir); e != nil {
			return e
		}
		if e := stepSetupEnv(frontReq, projectsDir); e != nil {
			return e
		}
		return InstallHooks(projectDir(frontReq, projectsDir), frontReq.Name, frontReq.Type)
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
	dir := filepath.Join(projectsDir, req.Name)

	// If directory already exists, skip creation
	if _, err := os.Stat(dir); err == nil {
		return nil
	}

	vis := "--public"
	if req.Private {
		vis = "--private"
	}
	_, err := runCmd(projectsDir, "gh", "repo", "create", req.Name, vis, "--clone")
	if err != nil {
		// Repo might already exist on GitHub — try cloning it
		_, cloneErr := runCmd(projectsDir, "gh", "repo", "clone", req.Name)
		if cloneErr != nil {
			// Final fallback: create local directory + git init
			if mkErr := os.MkdirAll(dir, 0755); mkErr != nil {
				return fmt.Errorf("all repo creation methods failed: %w", mkErr)
			}
			if _, gitErr := runCmd(dir, "git", "init"); gitErr != nil {
				return fmt.Errorf("git init failed: %w", gitErr)
			}
		}
	}
	return nil
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

	// Type-specific manifest
	switch req.Type {
	case "python":
		// Python gets full poetry pyproject.toml as its primary manifest
		content := fmt.Sprintf(`[tool.poetry]
name = "%s"
version = "%s"
description = ""
package-mode = false

[tool.poetry.dependencies]
python = "^3.12"
setuptools = "^78.1.0"
bump2version = "^1.0.0"

[tool.poetry.group.dev.dependencies]
pre-commit = "^4.0.1"
pylint = "^3.3.0"
yamllint = "^1.35.0"
isort = "^6.0.1"
toml = "^0.10.0"
black = "^25.1.0"
pytest = "^8.3.1"
pytest-cov = "^6.1.1"
coverage = "^7.2.5"

[tool.black]
line-length = 100
target-version = ['py312']

[tool.isort]
profile = "black"
line_length = 100

[tool.pylint]
rcfile = ".pylintrc"

[build-system]
requires = ["poetry-core>=1.0.0"]
build-backend = "poetry.core.masonry.api"
`, req.Name, version)
		if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(content), 0644); err != nil {
			return err
		}

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
		if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(content), 0644); err != nil {
			return err
		}

	case "go":
		content := fmt.Sprintf("module github.com/JuanVilla424/%s\n\ngo 1.23\n", req.Name)
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0644); err != nil {
			return err
		}
	}

	// CI/CD pyproject.toml for go and node (python already has it above)
	if req.Type == "go" || req.Type == "node" {
		pyproject := fmt.Sprintf(`[tool.poetry]
name = "%s"
version = "%s"
description = ""
package-mode = false

[tool.poetry.dependencies]
python = "^3.12"
bump2version = "^1.0.0"

[tool.poetry.group.dev.dependencies]
pre-commit = "^4.0.1"
pylint = "^3.3.0"
yamllint = "^1.35.0"
black = "^25.1.0"

[build-system]
requires = ["poetry-core>=1.0.0"]
build-backend = "poetry.core.masonry.api"
`, req.Name, version)
		os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0644)
	}

	// requirements.dev.txt for all types
	reqDev := `pre-commit>=4.0.0
pylint>=3.3.1
poetry>=1.8.4
bump2version>=1.0.0
toml>=0.10.1
black>=24.3.0
pytest>=7.3.1
pytest-cov>=4.0.0
coverage>=7.2.5
`
	os.WriteFile(filepath.Join(dir, "requirements.dev.txt"), []byte(reqDev), 0644)

	return nil
}

func stepCreateChangelog(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	content := `# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

### Changed

### Fixed

### Removed
`
	return os.WriteFile(filepath.Join(dir, "CHANGELOG.md"), []byte(content), 0644)
}

func stepCreateBumpversion(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	version := req.Version
	if version == "" {
		version = "1.0.0"
	}

	var files string
	switch req.Type {
	case "python":
		files = "\n[bumpversion:file:pyproject.toml]\n"
	case "node":
		files = "\n[bumpversion:file:package.json]\n\n[bumpversion:file:pyproject.toml]\n"
	case "go":
		files = "\n[bumpversion:file:go.mod]\n\n[bumpversion:file:pyproject.toml]\n"
	}

	content := fmt.Sprintf("[bumpversion]\ncurrent_version = %s\ncommit = True\ntag = False\n%s", version, files)
	return os.WriteFile(filepath.Join(dir, ".bumpversion.cfg"), []byte(content), 0644)
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

	// Git submodules (if .gitmodules exists from template)
	if _, err := os.Stat(filepath.Join(dir, ".gitmodules")); err == nil {
		runCmd(dir, "git", "submodule", "init")
		runCmd(dir, "git", "submodule", "update")
	}

	// Type-specific setup
	switch req.Type {
	case "python":
		if _, err := runCmd(dir, "python3", "-m", "venv", "venv"); err != nil {
			return err
		}
		pip := filepath.Join(dir, "venv", "bin", "pip")
		if _, err := os.Stat(filepath.Join(dir, "requirements.dev.txt")); err == nil {
			runCmd(dir, pip, "install", "-r", "requirements.dev.txt")
		}
	case "node":
		if _, err := os.Stat(filepath.Join(dir, "package.json")); err == nil {
			runCmd(dir, "npm", "install")
		}
	}

	// Pre-commit install (all types)
	precommit := "pre-commit"
	if req.Type == "python" {
		venvPC := filepath.Join(dir, "venv", "bin", "pre-commit")
		if _, err := os.Stat(venvPC); err == nil {
			precommit = venvPC
		}
	}
	if _, err := exec.LookPath(precommit); err == nil || precommit != "pre-commit" {
		runCmd(dir, precommit, "install")
		runCmd(dir, precommit, "install", "--hook-type", "pre-push")
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
