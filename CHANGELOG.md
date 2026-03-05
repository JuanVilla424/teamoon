## [1.1.10] - 2026-03-03

### Features

- **core**: add harvester default job for security scanning and dependabot auto-merge (`patch candidate`)
- **core**: sync local bmad agents into embedded assets
- **core**: add claude code security review skeleton phase
- **core**: add post-autopilot branch stabilization and keep version-controller workflow

### Bug Fixes

- **core**: default new repos to private unless explicitly requested public

### Documentation

- **core**: add chrome devtools and security review to readme

## [1.1.9] - 2026-03-03

### Bug Fixes

- **core**: eliminate queue state and step counter flickering with monotonic high-water marks (`patch candidate`)

### Documentation

- **core**: update screenshots and readme with new views and light mode

## [1.1.8] - 2026-03-02

### Features

- **core**: add refresh buttons, wave badges, nebula token system overhaul and daily usage fix (`patch candidate`)
- **core**: add concurrent wave execution with global task semaphore and plan retry backoff
- **core**: add job_create directive to chat handler with inline parsing and plugin system
- **core**: add i18n multi-language support with 8 locales

## [1.1.7] - 2026-02-26

### Features

- **core**: add file attachments for chat and tasks with upload infrastructure (`patch candidate`)
- **core**: add macos installer support with launchd, homebrew, and password prompt
- **core**: add session auth with bcrypt, login page, jobs scheduler, and pr detail view

## [1.1.6] - 2026-02-25

### Features

- **core**: add system executor with sudo guard and chat system mode (`patch candidate`)
- **core**: add service uptime to topbar and chat tetris screenshot (`patch candidate`)

## [1.1.4] - 2026-02-25

### Features

- **core**: add daily usage bar, session bar, log retention, and readme overhaul with screenshots (`patch candidate`)

## [1.1.3] - 2026-02-24

### Features

- **core**: enrich marketplace detail with repo links dates author and uninstall (`patch candidate`)

## [1.1.2] - 2026-02-24

### Features

- **core**: fix go.mod lipgloss and mousetrap versions to match go.sum
- **core**: exclude test_usage dir from tracking and update binary
- **core**: normalize BMAD asset quote style and update binary
- **core**: sync BMAD 6.0.0-alpha.15 assets and update binary
- **core**: redesign dashboard grid layout with explicit bento areas for uniform screen coverage (`patch candidate`)
- **core**: uniform bento grid layout covering full screen
- **core**: apply bento grid named-area classes and add min row height for uniform dashboard layout

### Bug Fixes

- **core**: correct go.mod version mismatches for lipgloss and mousetrap

## [1.1.1] - 2026-02-24

### Features

- **core**: add branch-based updates, env autoconfig, step timeout, autostart toggle and ui improvements (`patch candidate`)

## [1.1.0] - 2026-02-24

### Other Changes

- ️ refactor(core): remove failed state, add guardrail wait-loop and transparent favicon [minor candidate]

## [1.0.39] - 2026-02-24

### Features

- **core**: add dark/light theme toggle and remove budget enforcement (`patch candidate`)

## [1.0.38] - 2026-02-24

### Features

- **core**: add usage guardrails, budget enforcement and responsive dashboard (`patch candidate`)

### Other Changes

- ️ refactor(core): replace blocked state with failed, add sequential chain stop and optional tasks

## [1.0.37] - 2026-02-23

### Features

- **core**: add debug mode, autopilot response logging and fix task logs scrolling (`patch candidate`)

### Bug Fixes

- **core**: remove hardcoded paths, fix autopilot ordering and plan state, add ui-ux-pro-max skill

## [1.0.36] - 2026-02-23

### Bug Fixes

- **core**: unify logging to /var/log/teamoon.log and fix root permissions (`patch candidate`)

## [1.0.35] - 2026-02-23

### Bug Fixes

- **core**: set main as default branch in installer prompt (`patch candidate`)

## [1.0.34] - 2026-02-23

### Bug Fixes

- **core**: clean branch default extraction from git ls-remote (`patch candidate`)

## [1.0.33] - 2026-02-23

### Bug Fixes

- **core**: improve branch prompt options and default bind to localhost (`patch candidate`)
- **core**: handle branch switching in existing installer clone
- **core**: default web_host to empty in installer for all-interfaces bind

### Refactors

- **core**: dynamic branch detection in installer from download url

### Other Changes

- ️ refactor(core): interactive branch selection in installer via git ls-remote

## [1.0.32] - 2026-02-23

### Bug Fixes

- **core**: default web_host to empty for backward compatible all-interfaces bind (`patch candidate`)

## [1.0.31] - 2026-02-23

### Features

- **core**: add svg favicon matching moon logo (`patch candidate`)
- **core**: add rhel 8/9 support to installer prereqs and service

### Bug Fixes

- **core**: remove unavailable rhel packages and enable epel+crb repos

## [1.0.30] - 2026-02-22

### Bug Fixes

- **core**: run install+restart as background script to survive service stop (`patch candidate`)

## [1.0.29] - 2026-02-22

### Bug Fixes

- **core**: stop service before replacing binary during self-update (`patch candidate`)

## [1.0.28] - 2026-02-22

### Bug Fixes

- **core**: validate source_dir exists before using it for self-update (`patch candidate`)

## [1.0.27] - 2026-02-22

### Bug Fixes

- **core**: update source_dir in existing config when path is invalid (`patch candidate`)

## [1.0.26] - 2026-02-22

### Bug Fixes

- **core**: use ~/.local/src/teamoon as default source dir for self-update (`patch candidate`)

## [1.0.25] - 2026-02-22

### Bug Fixes

- **core**: enforce mandatory task creation in chat prompt (`patch candidate`)

## [1.0.24] - 2026-02-22

### Features

- **core**: add self-update with tag/channel selection and config setup tab (`patch candidate`)

### Bug Fixes

- **core**: setup sidebar status display and content key caching

## [1.0.23] - 2026-02-22

### Bug Fixes

- **core**: path augmentation, prereqs detection, chat errors and onboarding ux (`patch candidate`)

## [1.0.22] - 2026-02-22

### Bug Fixes

- **core**: redirect ask_input prompt to tty so it displays in subshell (`patch candidate`)

## [1.0.21] - 2026-02-22

### Bug Fixes

- **core**: make installer interactive and silence sync-bmad for fresh clones (`patch candidate`)

## [1.0.20] - 2026-02-22

### Bug Fixes

- **core**: handle nounset in install.sh for nvm pyenv and rustup (`patch candidate`)

## [1.0.19] - 2026-02-22

### Features

- **core**: add prereqs installer to web onboarding and restructure docs (`patch candidate`)
- **core**: add single-command installer script

## [1.0.18] - 2026-02-22

### Features

- **core**: add web onboarding wizard with ubuntu installer style ui (`patch candidate`)

## [1.0.17] - 2026-02-22

### Features

- **core**: add interactive onboarding wizard to teamoon init (`patch candidate`)

## [1.0.16] - 2026-02-22

### Bug Fixes

- **core**: use workflow_call to trigger release from version controller (`patch candidate`)

## [1.0.15] - 2026-02-22

### Bug Fixes

- **core**: trigger release via repository_dispatch from version controller (`patch candidate`)

## [1.0.14] - 2026-02-22

### Bug Fixes

- **core**: dynamic version badge and fix pflag dependency version (`patch candidate`)
- **core**: revert persist-credentials that caused startup failure (`patch candidate`)
- **core**: revert tag pattern to original that passes github validation (`patch candidate`)
- **core**: use persist-credentials false for PAT tag push (`patch candidate`)
- **core**: unset checkout extraheader so PAT triggers release workflow (`patch candidate`)

### Chores

- **core**: use dynamic version badge from github tags (`patch candidate`)

## [1.0.8] - 2026-02-22

### Bug Fixes

- **core**: fix release tag pattern to use glob instead of regex (`patch candidate`)

## [1.0.7] - 2026-02-22

### Bug Fixes

- **core**: use PAT for release tag push to trigger release controller (`patch candidate`)
- **core**: use PAT for release tag push to trigger release controller

### Chores

- **core**: update version-controller actions to latest versions (`patch candidate`)
- **core**: update version-controller actions to latest versions (`patch candidate`)

## [1.0.6] - 2026-02-22

### Bug Fixes

- **core**: fix gitignore blocking internal/logs and install-hooks vet error (`patch candidate`)
- **core**: add go workflow, make targets in ci/cd, and update docs (`patch candidate`)

## [1.0.4] - 2026-02-22

### Bug Fixes

- **core**: fix github actions ci/cd for go project and add step timeout (`patch candidate`)

## [1.0.3] - 2026-02-22

### Features

- **core**: add autopilot recovery on service restart (`patch candidate`)
- **core**: nebula web ui redesign with real metrics and per-model cost tracking
- **core**: initial project scaffold

### Other Changes

- Initial commit
