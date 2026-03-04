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

	if err := emit("Creating repo from template ("+req.Name+")", func() error { return stepCreateRepo(req, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Setting up dev branch", func() error { return stepCreateDevBranch(req, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Configuring project ("+req.Type+")", func() error {
		if e := stepTrimWorkflows(req, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateManifest(req, projectsDir); e != nil {
			return e
		}
		if e := stepCreateChangelog(req, projectsDir); e != nil {
			return e
		}
		if e := stepCreateBumpversion(req, projectsDir); e != nil {
			return e
		}
		if e := stepUpdateDocs(req, projectsDir); e != nil {
			return e
		}
		if e := stepSetupEnv(req, projectsDir); e != nil {
			return e
		}
		return InstallHooks(projectDir(req, projectsDir), req.Name, req.Type)
	}); err != nil {
		return err
	}
	if err := emit("Pushing repo", func() error { return stepCommitPush(req, projectsDir) }); err != nil {
		return err
	}

	return nil
}

func runSeparateInit(req InitRequest, projectsDir string, progress ProgressFunc) error {
	// Backend repo: {name}-backend with selected type
	backReq := req
	backReq.Name = req.Name + "-backend"

	// Frontend repo: {name}-frontend, always node
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

	// Backend steps
	if err := emit("Creating backend repo from template ("+backReq.Name+")", func() error { return stepCreateRepo(backReq, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Setting up backend dev branch", func() error { return stepCreateDevBranch(backReq, projectsDir) }); err != nil {
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
		if e := stepUpdateDocs(backReq, projectsDir); e != nil {
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

	// Frontend steps
	if err := emit("Creating frontend repo from template ("+frontReq.Name+")", func() error { return stepCreateRepo(frontReq, projectsDir) }); err != nil {
		return err
	}
	if err := emit("Setting up frontend dev branch", func() error { return stepCreateDevBranch(frontReq, projectsDir) }); err != nil {
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
		if e := stepUpdateDocs(frontReq, projectsDir); e != nil {
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

	// Create repo FROM template — this copies all files, workflows, configs automatically
	_, err := runCmd(projectsDir, "gh", "repo", "create", req.Name, vis, "--template", templateRepo, "--clone")
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

func stepCreateDevBranch(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	_, err := runCmd(dir, "git", "checkout", "-b", "dev")
	return err
}

func stepTrimWorkflows(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	wfDir := filepath.Join(dir, ".github", "workflows")
	entries, err := os.ReadDir(wfDir)
	if err != nil {
		return nil // no workflows dir, skip
	}

	keep := map[string]bool{"ci.yml": true, "version-controller.yml": true}
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

// resolveGitHubSlug extracts "owner/repo" from the git remote origin URL.
// Returns empty string if no remote is configured or parsing fails.
func resolveGitHubSlug(dir string) string {
	out, err := exec.Command("git", "-C", dir, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	remote := strings.TrimSpace(string(out))
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

func stepUpdateDocs(req InitRequest, projectsDir string) error {
	dir := projectDir(req, projectsDir)
	slug := resolveGitHubSlug(dir)

	var langBadge, quickStart, scripts string
	switch req.Type {
	case "python":
		langBadge = "[![Python](https://img.shields.io/badge/Python-3.11%2B-blue.svg)](https://python.org/)"
		quickStart = "python -m venv venv && source venv/bin/activate\npip install -r requirements.txt"
		scripts = "| Command | Description |\n|---------|-------------|\n| `pytest tests/ -v` | Run test suite |\n| `pylint **/*.py` | Lint source files |\n| `black .` | Format code |"
	case "node":
		langBadge = "[![Node.js](https://img.shields.io/badge/Node.js-20%2B-green.svg)](https://nodejs.org/)"
		quickStart = "npm install\nnpm run dev"
		scripts = "| Command | Description |\n|---------|-------------|\n| `npm run dev` | Start dev server |\n| `npm run build` | Production build |\n| `npm run test` | Run test suite |\n| `npx eslint src/` | Lint source files |"
	case "go":
		langBadge = "[![Go](https://img.shields.io/badge/Go-1.23%2B-00ADD8.svg)](https://go.dev/)"
		quickStart = "go mod download\nmake build"
		scripts = "| Command | Description |\n|---------|-------------|\n| `make build` | Compile binary |\n| `make test` | Run test suite |\n| `golangci-lint run` | Lint source files |"
	default:
		langBadge = "![Status](https://img.shields.io/badge/Status-Active-green.svg)"
		quickStart = "# See project documentation"
		scripts = "| Command | Description |\n|---------|-------------|\n| TBD | TBD |"
	}

	// Build GitHub-linked badges when slug is available
	versionBadge := ""
	buildBadge := ""
	licenseBadge := "[![License](https://img.shields.io/badge/License-GPLv3-purple.svg)](LICENSE)"
	repoURL := ""
	if slug != "" {
		versionBadge = fmt.Sprintf("\n[![Version](https://img.shields.io/github/v/tag/%s?label=Version&color=blue)](VERSIONING.md)", slug)
		buildBadge = fmt.Sprintf("\n[![Build](https://img.shields.io/github/actions/workflow/status/%s/ci.yml?branch=dev&label=Build)](https://github.com/%s/actions)", slug, slug)
		repoURL = fmt.Sprintf("https://github.com/%s.git", slug)
	}

	cloneBlock := ""
	if repoURL != "" {
		cloneBlock = fmt.Sprintf("git clone %s\ncd %s\n", repoURL, req.Name)
	} else {
		cloneBlock = fmt.Sprintf("cd %s\n", req.Name)
	}

	content := fmt.Sprintf(`# %s

%s%s%s
[![Status](https://img.shields.io/badge/Status-Active-green.svg)]()
%s

A brief description of what this project does.

## ✨ Features

- Feature 1
- Feature 2
- Feature 3

## 🚀 Quick Start

`+"```bash\n"+`%s%s
`+"```\n\n"+`## 📋 Scripts

%s

## 🤝 Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md)

## 📄 License

GNU GPL v3.0 — see [LICENSE](LICENSE)
`, req.Name, langBadge, versionBadge, buildBadge, licenseBadge, cloneBlock, quickStart, scripts)

	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte(content), 0644); err != nil {
		return err
	}

	// Generate ARCHITECT.md
	var lang, pkgMgr, typeFiles string
	switch req.Type {
	case "python":
		lang = "Python 3.12+"
		pkgMgr = "Poetry"
		typeFiles = "├── pyproject.toml          # Poetry config + dependencies\n├── requirements.dev.txt    # Dev dependencies\n├── .pylintrc               # Linting config"
	case "node":
		lang = "Node.js 20+"
		pkgMgr = "npm"
		typeFiles = "├── package.json            # Dependencies and scripts\n├── tsconfig.json           # TypeScript config (if applicable)"
	case "go":
		lang = "Go 1.23+"
		pkgMgr = "Go Modules"
		typeFiles = "├── go.mod                  # Module definition\n├── go.sum                  # Dependency checksums"
	default:
		lang = req.Type
		pkgMgr = "N/A"
		typeFiles = "├── (project files)"
	}

	architect := fmt.Sprintf(`# Architecture — %s

## Overview

%s is a %s project created from the github-cicd-template.

## Tech Stack

| Layer | Technology |
|-------|-----------|
| Language | %s |
| Package Manager | %s |
| CI/CD | GitHub Actions |
| Pre-commit | pre-commit framework |
| Version Control | Git + GitHub |

## Project Structure

`+"```"+`
%s/
├── .github/workflows/      # CI/CD pipelines
├── scripts/                 # Shared CI/CD scripts (submodule)
%s
├── .bumpversion.cfg         # Version bump config
├── .pre-commit-config.yaml  # Pre-commit hooks
├── CHANGELOG.md
├── ARCHITECT.md
├── CONTEXT.md
├── CLAUDE.md
├── MEMORY.md
└── README.md
`+"```"+`

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Template | github-cicd-template | Standard CI/CD, pre-commit hooks, workflows |
| Branch strategy | dev → main | Trunk-based development with dev branch |
| License | GPLv3 | Standard open-source license |
`, req.Name, req.Name, req.Type, lang, pkgMgr, req.Name, typeFiles)

	if err := os.WriteFile(filepath.Join(dir, "ARCHITECT.md"), []byte(architect), 0644); err != nil {
		return err
	}

	// Generate CONTEXT.md
	var entryPoint, envReqs, howToRun string
	switch req.Type {
	case "python":
		entryPoint = "main.py (to be created)"
		envReqs = "- Python 3.12+\n- Poetry\n- pre-commit"
		howToRun = "python -m venv venv && source venv/bin/activate\npip install -r requirements.dev.txt\npython main.py"
	case "node":
		entryPoint = "src/index.ts or src/main.ts (to be created)"
		envReqs = "- Node.js 20+\n- npm 10+\n- pre-commit"
		howToRun = "npm install\nnpm run dev"
	case "go":
		entryPoint = "cmd/main.go or main.go (to be created)"
		envReqs = "- Go 1.23+\n- pre-commit"
		howToRun = "go mod download\nmake build\n./bin/%s"
	default:
		entryPoint = "(to be defined)"
		envReqs = "- (to be defined)"
		howToRun = "# See project documentation"
	}
	if req.Type == "go" {
		howToRun = fmt.Sprintf(howToRun, req.Name)
	}

	context := fmt.Sprintf(`# Context — %s

## What This Project Does

%s is a new %s project. Purpose to be defined during development.

## Current State

- Status: Initial scaffold
- Branch: dev
- CI/CD: Configured via github-cicd-template

## Key Entry Points

- %s

## Environment Requirements

%s

## How to Run

`+"```bash\n"+`%s
`+"```\n", req.Name, req.Name, req.Type, entryPoint, envReqs, howToRun)

	return os.WriteFile(filepath.Join(dir, "CONTEXT.md"), []byte(context), 0644)
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
