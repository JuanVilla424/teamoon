# 🏗️ teamoon — Architecture

[![Go](https://img.shields.io/badge/Go-1.24%2B-00ADD8.svg)](https://go.dev/)
[![Architecture](https://img.shields.io/badge/Pattern-Elm%20Architecture-blueviolet.svg)]()
[![Storage](https://img.shields.io/badge/Storage-JSON%20Files-orange.svg)]()
[![UI](https://img.shields.io/badge/UI-Dual%20Mode-green.svg)]()

Development autopilot that manages task execution by spawning Claude CLI sessions. Dual-mode: **terminal UI** (Bubbletea) + **web SPA** (embedded via `go:embed`, served as systemd/launchd service). All state persisted as JSON files — no database.

---

## 📑 Table of Contents

- [Core Patterns](#-core-patterns)
- [Package Map](#-package-map)
- [Package Reference](#-package-reference)
- [Data Flow](#-data-flow)
- [Design Decisions](#%EF%B8%8F-design-decisions)

---

## 🧬 Core Patterns

| Pattern             | Implementation                                                                 |
| ------------------- | ------------------------------------------------------------------------------ |
| 🏛️ Elm Architecture | Bubbletea `Model → Update → View` cycle for TUI state                          |
| 🔌 CLI Subprocess   | Spawns `claude` binary with `--output-format stream-json`, parses JSONL stream |
| 📡 SSE Push         | Server-Sent Events broadcast `DataSnapshot` JSON to all connected browsers     |
| 🚌 Message Bus      | `send func(tea.Msg)` bridges engine goroutines → TUI channel or web SSE        |
| 💾 JSON Persistence | All state in `~/.config/teamoon/*.json` — no database                          |
| 📦 Embedded SPA     | `//go:embed static` compiles HTML/CSS/JS into binary at build time             |
| 🤖 BMAD Party-Mode  | Plan generation invokes BMAD agents that assign themselves to execution steps  |

---

## 🗺️ Package Map

```
teamoon/
├── 🚀 cmd/teamoon/
│   └── main.go                  # Cobra CLI commands (serve, init, task, set-password)
│
├── 📦 internal/
│   ├── 💬 chat/                 # Chat history (JSON, 50-msg ring, per-project)
│   ├── ⚙️  config/              # Config, SpawnConfig, SkeletonConfig, MCP, PhaseHints
│   ├── 🖥️  dashboard/           # Bubbletea TUI (Model, Update, View, Styles)
│   │
│   ├── 🤖 engine/               # ── AUTOPILOT CORE ──
│   │   ├── engine.go            #   Manager: semaphore, runners, project loops
│   │   ├── executor.go          #   runTask: step execution, 3-layer failure recovery
│   │   ├── guardrails.go        #   Circuit breaker (pause at 90% utilization)
│   │   ├── project_loop.go      #   Wave-based parallel execution, stabilization
│   │   └── wordgen.go           #   Satirical git identities for autopilot commits
│   │
│   ├── ⏰ jobs/                  # ── CRON SYSTEM ──
│   │   ├── scheduler.go         #   1-minute tick scheduler
│   │   ├── cron.go              #   5-field cron parser + HumanReadable()
│   │   ├── runner.go            #   Job execution (Claude CLI or native harvester)
│   │   ├── harvester.go         #   Dependabot auto-merge + security task creation
│   │   └── store.go             #   Job persistence (JSON)
│   │
│   ├── 🌐 web/                  # ── HTTP SERVER ──
│   │   ├── server.go            #   SSE Hub, recovery, scheduler start
│   │   ├── handlers.go          #   50+ API endpoints, chat streaming, plan gen
│   │   ├── data.go              #   DataSnapshot builder, Store.Refresh()
│   │   ├── session.go           #   In-memory sessions (24h sliding window)
│   │   └── static.go            #   go:embed static assets
│   │
│   ├── 📋 queue/                # Task store, state machine, wave sorting, webhooks
│   ├── 📝 plan/                 # Plan/Step markdown parser, Agent validation
│   ├── 🧠 plangen/              # Plan generation via Claude CLI + BMAD party-mode
│   ├── 📊 metrics/              # Token scanning, usage fetching, cost calculation
│   ├── 📁 projects/             # Directory scanning, git ops, GitHub PR management
│   ├── 📜 logs/                 # Ring buffer (memory + file + per-task logs)
│   ├── 🧙 onboarding/           # Setup wizard (CLI + Web streaming, 8 steps)
│   ├── 🏭 projectinit/          # Project scaffolding from github-cicd-template
│   ├── 🔌 plugins/              # Claude Code plugin manager (11 defaults)
│   ├── 🎯 skills/               # Claude Code skills manager (28 defaults)
│   ├── 📎 uploads/              # File upload storage + text MIME detection
│   ├── 📄 templates/            # Reusable task description templates
│   └── 🛤️  pathutil/            # PATH augmentation for subprocess tool discovery
│
├── 🎨 web/static/               # Frontend SPA (embedded at compile time)
│   ├── index.html, app.js, style.css, i18n.js, locales/
│
└── 🔧 Makefile                  # build, test, install, service, release, clean
```

---

## 📖 Package Reference

### 🚀 `cmd/teamoon` — CLI Entry Point

| Command                | Action                                 |
| ---------------------- | -------------------------------------- |
| `teamoon`              | 🖥️ Launches Bubbletea TUI dashboard    |
| `teamoon serve`        | 🌐 Web server only (production mode)   |
| `teamoon init`         | 🧙 Interactive onboarding wizard       |
| `teamoon task add`     | ➕ Add task from CLI                   |
| `teamoon task done`    | ✅ Mark task done                      |
| `teamoon task list`    | 📋 List pending tasks                  |
| `teamoon set-password` | 🔒 Set bcrypt-hashed web auth password |

**🔄 Startup sequence (`serve`):**

1. `config.Load()` → `~/.config/teamoon/config.json`
2. `engine.NewManager()` → concurrency semaphore
3. `logs.NewRingBuffer()` → `/var/log/teamoon.log`
4. `config.InitMCPFromGlobal()` → `~/.claude/settings.json`
5. `web.NewServer().Start(ctx)` → HTTP + SSE
6. Signal wait `SIGINT`/`SIGTERM`

---

### ⚙️ `internal/config` — Configuration

Manages `~/.config/teamoon/config.json`.

| Type             | Fields                                                                                                                                        |
| ---------------- | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `Config`         | projects dir, claude dir, web port/password, webhook URL, max concurrent, autopilot autostart, MCP servers, phase hints, log retention, debug |
| `SpawnConfig`    | model, effort, max turns, step/plan timeout, max plan attempts                                                                                |
| `SkeletonConfig` | per-phase toggles: web search, doc setup, build verify, test, security review, pre-commit, commit, push                                       |
| `MCPServer`      | command, args, enabled, auto-attached skeleton step                                                                                           |
| `PhaseHints`     | LLM-visible instructions injected per skeleton phase                                                                                          |

**🔑 Key functions:**

- `SkeletonFor(cfg, project)` — returns per-project skeleton or global fallback
- `BuildMCPConfigJSON()` — writes temp JSON for `--mcp-config` flag
- `FilterPlanMCP()` — returns only `context7` for plan generation (excludes heavy servers)
- `InstallMCPToGlobal()` / `RemoveMCPFromGlobal()` — mutates `~/.claude/settings.json`

**🎛️ Model Resolution** (`engine.ResolveModel`):

| Alias      | Plan Phase                  | Exec Phase                  |
| ---------- | --------------------------- | --------------------------- |
| `opusplan` | `claude-opus-4-6`           | `claude-sonnet-4-6`         |
| `opus`     | `claude-opus-4-6`           | `claude-opus-4-6`           |
| `sonnet`   | `claude-sonnet-4-6`         | `claude-sonnet-4-6`         |
| `haiku`    | `claude-haiku-4-5-20251001` | `claude-haiku-4-5-20251001` |

---

### 📋 `internal/queue` — Task Store

JSON persistence at `~/.config/teamoon/tasks.json`.

**📊 State Machine:**

```
  pending ──► planned ──► running ──► done
    ▲            │           │
    └────────────┘           ├──► pending  (failure + fail_reason)
                             └──► planned  (stopped with SessionID)

  any state ──► archived
```

**🏷️ Key Task Fields:**

| Field                        | Description                                          |
| ---------------------------- | ---------------------------------------------------- |
| `Priority`                   | `high` / `med` / `low`                               |
| `AutoPilot`                  | Eligible for project autopilot loop                  |
| `Assignee`                   | `""` = normal task, `"system"` = system loop         |
| `Wave`                       | `0` = sequential (runs last), `N>0` = parallel group |
| `SessionID`                  | Claude CLI session for `--resume` restart recovery   |
| `CurrentStep` / `TotalSteps` | Progress tracking for resume                         |

**🌊 Wave Sorting:** Same positive wave = parallel execution. Different waves = strict sequential. Wave 0 treated as MaxInt (runs last).

🔔 Webhook: fires async HTTP POST to `cfg.WebhookURL` on task events.

---

### 📝 `internal/plan` — Plan Parser

Parses plan markdown into structured `Plan` with `Steps`:

```markdown
# Plan: [title]

## Steps

### Step N: [title]

Agent: [agent-id] ← REQUIRED (validated)
ReadOnly: true|false

- instruction bullets
  Verify: success criteria

## Constraints

- constraint

## Dependencies

- /path/to/project ← added via --add-dir
```

> ⚠️ Every step **must** have an `Agent:` field — plan parsing rejects plans without it.

---

### 🧠 `internal/plangen` — Plan Generation

Generates plans by spawning Claude CLI as a subprocess:

1. 🔗 Ensures `.bmad` symlink exists in project directory
2. 📦 Serializes skeleton config + MCP steps to JSON
3. 🎭 Builds prompt instructing Claude to call `Skill bmad:core:workflows:party-mode`
4. 🔒 Spawns `claude` with **read-only tools only** (`--disallowedTools Edit,Write,Bash,...`)
5. 📡 Streams output, detects completion (`# Plan:` + `## Steps` + `## Constraints`)
6. 💾 Saves to `~/.config/teamoon/plans/task-{id}.md` → state = `planned`

Uses **Opus** model for planning (`opusplan` → `claude-opus-4-6`). 💓 Heartbeat goroutine logs progress every 15s.

---

### 🤖 `internal/engine` — Autopilot Core

#### 🎛️ Manager (`engine.go`)

```go
type Manager struct {
    runners      map[int]*Runner           // taskID → running Claude process
    projectLoops map[string]*ProjectLoop   // project → autopilot loop goroutine
    taskSem      chan struct{}              // concurrency semaphore
}
```

| Method                            | Action                                         |
| --------------------------------- | ---------------------------------------------- |
| `Start()`                         | 🚀 Launches `runTask()` goroutine (idempotent) |
| `Stop()`                          | 🛑 Cancels context, waits on done channel      |
| `StartProject()`                  | 🔄 Launches `RunProjectLoop` goroutine         |
| `AcquireSlot()` / `ReleaseSlot()` | 🔐 Global Claude process concurrency control   |

#### ⚡ Executor (`executor.go`)

**`runTask()`** — the autopilot state machine:

```
for each step in plan:
    ├─ skip if step.Number <= task.CurrentStep (resume)
    ├─ CheckGuardrails() → wait 2m if triggered
    │
    └─ for retry in 0..2:
         ├─ buildStepPrompt() → inject docs + previous results + rules
         ├─ spawnClaude() → {ExitCode, Output, Denials, ToolsUsed, SessionID}
         │
         ├─ ✅ success → break
         ├─ ⚠️  no write tools used → retry (soft failure)
         └─ ❌ failed → Layer 2: deliberative recovery via second Claude
                        feed analysis as context → next retry

    3 failures → SetFailReason → state=pending
all steps done → state=done ✅
```

**🔧 `spawnClaude()` configuration:**

- 🎭 Generates satirical git identity via `wordgen.GenerateName()`
- 🔄 If `SessionID`: uses `--resume` (restart recovery)
- 🔌 Passes only `context7` + `github` MCP servers
- 🚫 `--disallowedTools AskUserQuestion,EnterPlanMode,ExitPlanMode,TodoWrite`
- ⏱️ `StepTimeoutMin` via `context.WithTimeout`

**📚 `buildStepPrompt()` injects:**

CLAUDE.md, README.md, INSTALL.md, MEMORY.md, CONTEXT.md, ARCHITECT.md, AGENTS.md, CHANGELOG.md, VERSIONING.md, CONTRIBUTING.md (char-limited) + previous step summaries + 15 rules.

**🛡️ Three-Layer Failure Recovery:**

| Layer      | Strategy                                                      |
| ---------- | ------------------------------------------------------------- |
| 🔁 Layer 1 | Retry with accumulated context (max 3 attempts)               |
| 🧠 Layer 2 | Deliberative — spawn second Claude to analyze failure and fix |
| 💀 Layer 3 | Meta-cognitive — fail the task gracefully                     |

#### 🌊 Project Loop (`project_loop.go`)

```
RunProjectLoop(project):
  loop:
    tasks = ListAutopilotPending(project)   // wave-sorted
    │
    ├─ Wave 1 (parallel): [task-A, task-B, task-C]
    │    plan all → run all (WaitGroup) → wait
    │
    ├─ Wave 2 (parallel): [task-D, task-E]
    │    plan all → run all (WaitGroup) → wait
    │
    └─ Wave 0 (sequential): [task-F, task-G]
         plan → run → plan → run

  empty queue → stabilizeProject()
                  ├─ create test/prod branches
                  └─ set branch protection on main
```

**⏳ Plan backoff:** attempt 1: immediate → attempt 2: 30s → attempt 3: 2m → attempt 4+: 5m

#### 🛡️ Guardrails (`guardrails.go`)

Circuit breaker. Reads `metrics.GetUsage()` (cached 60s). **Pauses autopilot when weekly or session utilization ≥ 90%.**

---

### 🌐 `internal/web` — HTTP Server

#### 🖥️ Server (`server.go`)

| Component           | Description                                                                           |
| ------------------- | ------------------------------------------------------------------------------------- |
| 📡 SSE Hub          | `map[sseClient]struct{}` with non-blocking broadcast                                  |
| 🔄 RecoverAndResume | Phase 1: reset orphans. Phase 2: `--resume` SessionID. Phase 3: restart project loops |
| ⏱️ scheduleRefresh  | Debounced 200ms → `store.Refresh()` → SSE broadcast                                   |

**🔄 `Start(ctx)` sequence:**

usage fetcher → store refresh → recover & resume → seed jobs → start scheduler → ticker → register routes → `ListenAndServe(:7777)`

#### 🛣️ Handlers (`handlers.go`) — 50+ Endpoints

| Category      | Endpoints                                                                                                           |
| ------------- | ------------------------------------------------------------------------------------------------------------------- |
| 🔐 Auth       | `login`, `logout`                                                                                                   |
| 📋 Tasks      | `add`, `done`, `archive`, `replan`, `autopilot`, `stop`, `plan`, `detail`, `update`, `assignee`, `attach`           |
| 📁 Projects   | `prs`, `pr-detail`, `merge-dependabot`, `pull`, `git-init`, `autopilot/start`, `autopilot/stop`, `skeleton`, `init` |
| 💬 Chat       | `send`, `history`, `clear`                                                                                          |
| ⚙️ Config     | `get`, `save`                                                                                                       |
| 🔌 MCP        | `list`, `toggle`, `init`, `catalog`, `install`, `uninstall`                                                         |
| 🧩 Plugins    | `list`, `install`, `uninstall`                                                                                      |
| 🎯 Skills     | `list`, `catalog`, `install`, `uninstall`                                                                           |
| ⏰ Jobs       | `list`, `add`, `update`, `delete`, `run`                                                                            |
| 🧙 Onboarding | `status`, `prereqs`, `prereqs/install`, `config`, `skills`, `bmad`, `hooks`, `mcp`, `plugins`                       |
| 🔄 Update     | `check`, `update`                                                                                                   |
| 📎 Uploads    | `upload`, `uploads/`                                                                                                |
| 📡 Data/SSE   | `data`, `sse`                                                                                                       |

**💬 `handleChatSend`** — streams Claude CLI output as SSE. Detects JSON directives in response:

- 📋 `taskDirective` → `queue.Add()` + optional plan generation
- ⏰ `jobDirective` → `jobs.Add()`

#### 📊 Data (`data.go`)

**`DataSnapshot`** sent via `/api/data` and SSE:

| Field            | Content                                             |
| ---------------- | --------------------------------------------------- |
| 📈 Token metrics | today/week/month aggregations                       |
| 💰 Cost          | USD calculation from token counts                   |
| 📋 Tasks         | with `EffectiveState` override (generating/running) |
| 📁 Projects      | with autopilot status, task counts                  |
| 📜 Log entries   | from ring buffer                                    |
| ⏰ Jobs          | cron job list with status                           |
| ℹ️ Meta          | version, build number, uptime, auth status          |

#### 🔒 Session (`session.go`)

In-memory `map[token]expiry`. 24h sliding window. 32 random bytes → hex (64 chars). Cleanup goroutine every 1h.

#### 📦 Static (`static.go`)

`//go:embed static` — entire `web/static/` directory compiled into binary.

---

### ⏰ `internal/jobs` — Cron System

JSON persistence at `~/.config/teamoon/jobs.json`.

| Component      | Description                                                                                                           |
| -------------- | --------------------------------------------------------------------------------------------------------------------- |
| 🕐 Scheduler   | Ticks every 1 minute, fires `MatchesCron(schedule, now)` per enabled job                                              |
| 🔢 Cron Parser | 5-field standard (`min hour dom month dow`), supports `*`, `*/N`, `N`, `N-M`, commas                                  |
| ▶️ Runner      | `__harvester__` → native `RunHarvester()`. Otherwise → Claude CLI subprocess                                          |
| 🌿 Harvester   | Scans projects → auto-merges dependabot PRs → creates security tasks with autopilot. Seeded at startup as `0 3 * * *` |

---

### 💬 `internal/chat` — Chat History

Persistence at `~/.config/teamoon/chat.json`. 50-message ring buffer. Per-project filtering. `RecentContextForProject(n, project)` provides conversation context for the chat handler.

---

### 📜 `internal/logs` — Log System

**Triple persistence:**

| Target            | Path                                   |
| ----------------- | -------------------------------------- |
| 🧠 Memory         | Fixed-capacity circular ring buffer    |
| 📄 Global file    | `/var/log/teamoon.log` (append)        |
| 📋 Per-task files | `~/.config/teamoon/logs/task-{id}.log` |

Levels: `Debug` · `Info` · `Success` · `Warn` · `Error`

Format: `2006-01-02 15:04:05 [TAG ] #ID project [agent]: message`

`CleanupLogs(retentionDays)` rewrites global log keeping entries within retention window.

---

### 📊 `internal/metrics` — Token & Cost Metrics

| File        | Responsibility                                                                                               |
| ----------- | ------------------------------------------------------------------------------------------------------------ |
| `tokens.go` | Walks `~/.claude/**/*.jsonl`, aggregates by today/week/month, per-model counts (opus/sonnet/haiku)           |
| `usage.go`  | Spawns `claude` via `expect`, sends `/usage`, parses ANSI output for utilization %. Background poll every 2m |
| `cost.go`   | USD calculation from token counts × model pricing                                                            |

---

### 📁 `internal/projects` — Project Scanner

- 🔍 `Scan()` — reads subdirectories, detects git repos, extracts branch/commit/status/remote
- 🔀 `FetchPRs()` / `MergePR()` — uses `gh` CLI
- 🤖 `FilterDependabot()` — filters `app/dependabot` PRs
- 🏭 `GitInitRepo()` — creates from `JuanVilla424/github-cicd-template` or connects existing
- 🔎 `DetectProjectType()` — checks `package.json` (node), `go.mod` (go), `pyproject.toml` (python)

---

### 🧙 `internal/onboarding` — Setup Wizard

8-step wizard (CLI interactive or web streaming via SSE):

| Step | Action                                                                                               |
| ---- | ---------------------------------------------------------------------------------------------------- |
| 1️⃣   | **Prereqs** — checks claude, git, gh, node, npx, expect                                              |
| 2️⃣   | **Config** — projects dir, web port, password, concurrency                                           |
| 3️⃣   | **Environment** — Go, Node, Python, Rust installation                                                |
| 4️⃣   | **Skills** — 28 Claude Code skills via `npx skills add`                                              |
| 5️⃣   | **BMAD** — copies commands to `~/.claude/commands/bmad/`                                             |
| 6️⃣   | **Hooks** — 5 security hooks (security-check, test-guard, secrets-guard, build-guard, commit-format) |
| 7️⃣   | **MCP** — context7, memory, sequential-thinking                                                      |
| 8️⃣   | **Plugins** — 11 Claude Code plugins (5 LSPs + 6 enhancements)                                       |

---

### 🏭 `internal/projectinit` — Project Scaffolding

Creates projects from `JuanVilla424/github-cicd-template`:

- 📦 Single repo or separate repos (backend + frontend)
- ✂️ Trims CI/CD workflows per language type
- 📝 Updates manifest (pyproject.toml / package.json / go.mod)
- 📄 Generates README.md, ARCHITECT.md, CONTEXT.md, CHANGELOG.md
- 🔗 `EnsureBMADLink()` — creates `.bmad` symlink for party-mode

---

### 🔌 `internal/plugins` — Plugin Manager

Manages Claude Code plugins via `claude plugin` CLI. Parses `~/.claude/settings.json`. 11 defaults: 5 LSPs (typescript, pyright, gopls, rust-analyzer, clangd) + 6 enhancements (hookify, security-guidance, pr-review-toolkit, claude-code-setup, code-simplifier, feature-dev).

### 🎯 `internal/skills` — Skills Manager

28 default skills from 6 sources: superpowers (14), anthropics (2), vercel-labs (5), nextlevelbuilder (1), bmad-skills (6). Install via `npx skills add`.

### 📎 `internal/uploads` — Upload Storage

File storage at `~/.config/teamoon/uploads/`. Metadata JSON. `IsTextMIME()` determines if content inlines into plan prompts (max 20KB total, 5KB per file).

### 📄 `internal/templates` — Task Templates

Reusable task description templates at `~/.config/teamoon/templates.json`. CRUD operations.

### 🛤️ `internal/pathutil` — PATH Augmentation

`AugmentPath()` adds common binary paths at startup (`~/.local/bin`, `/usr/local/bin`, nvm paths, Go paths) so tools are findable from systemd service context.

---

## 🔄 Data Flow

### 🚀 Startup Sequence (Web Mode)

```
main.go (serve)
  │
  ├─ config.Load()              ──► ~/.config/teamoon/config.json
  ├─ engine.NewManager(max)     ──► concurrency semaphore
  ├─ logs.NewRingBuffer(cfg)    ──► /var/log/teamoon.log (preload retention)
  ├─ config.InitMCPFromGlobal() ──► ~/.claude/settings.json → cfg.MCPServers
  │
  └─ web.NewServer().Start(ctx)
        ├─ metrics.StartUsageFetcher()  ──► background poll (2m)
        ├─ store.Refresh()              ──► initial DataSnapshot
        ├─ RecoverAndResume()
        │     ├─ queue.RecoverRunning() ──► reset orphaned tasks
        │     ├─ queue.ListResumable()  ──► --resume <SessionID>
        │     └─ StartProject() × N     ──► restart autopilot loops
        ├─ jobs.SeedDefaults()          ──► ensure harvester (0 3 * * *)
        ├─ jobs.StartScheduler()        ──► 1-min tick goroutine
        ├─ ticker(RefreshIntervalSec)   ──► periodic Refresh + SSE broadcast
        ├─ RegisterRoutes()             ──► 50+ endpoints
        └─ ListenAndServe(:7777)
```

### 📋 Task Lifecycle

```
User (browser / CLI)
  │
  ├─ POST /api/tasks/add ──► queue.Add() ──► tasks.json
  │                               │
  │                         (if autoRun)
  │                               ▼
  │                       generatePlanAsync()
  │                         │  claude CLI (read-only, opus model)
  │                         │  calls Skill bmad:core:workflows:party-mode
  │                         │  streams plan → saves task-N.md
  │                         └─► state = planned
  │
  ├─ POST /api/tasks/autopilot ──► engine.Start()
  │                                   │
  │                                   │  for each step:
  │                                   │    CheckGuardrails()
  │                                   │    spawnClaude() (sonnet model)
  │                                   │      └─ stream-json → LogMsg
  │                                   │
  │                                   │  ✅ all done → state=done
  │                                   │  ❌ 3 failures → state=pending
  │                                   │
  │                                   └─► webSend() → logBuf.Add()
  │                                         scheduleRefresh() (200ms)
  │                                           └─ hub.broadcast(DataSnapshot)
  │
  └─ GET /api/sse ──► browser receives JSON ──► DOM update (no reload)
```

### 🌊 Autopilot Project Loop

```
POST /api/projects/autopilot/start?project=myapp
  │
  └─ engine.StartProject("myapp", RunProjectLoop)
       │
       └─ loop:
            tasks = ListAutopilotPending("myapp")   // wave-sorted
            │
            ├─ 🌊 Wave 1 (parallel): [task-A, task-B, task-C]
            │    plan all → run all (WaitGroup) → wait
            │
            ├─ 🌊 Wave 2 (parallel): [task-D, task-E]
            │    plan all → run all (WaitGroup) → wait
            │
            └─ 🔗 Wave 0 (sequential): [task-F, task-G]
                 plan → run → plan → run

          empty queue → stabilizeProject()
                          ├─ create test/prod branches
                          └─ set branch protection on main
```

### ⏰ Cron Job Flow

```
Server.Start()
  ├─ jobs.SeedDefaults()      ──► harvester (0 3 * * *)
  └─ jobs.StartScheduler()
       └─ every 1 minute:
            for each enabled job:
              if MatchesCron(schedule, now):
                ├─ 🌿 __harvester__
                │    scan projects → merge dependabot → create security tasks
                └─ 🤖 else
                     claude -p <instruction> → capture result
```

---

## ⚖️ Design Decisions

| Decision                                  | Rationale                                                                                         |
| ----------------------------------------- | ------------------------------------------------------------------------------------------------- |
| 💾 JSON persistence (not SQLite)          | Go stdlib `encoding/json`, human-readable, no CGo dependency                                      |
| 📦 `internal/` packages                   | Enforces encapsulation; prevents external imports                                                 |
| 🔌 Claude CLI as subprocess (not API SDK) | Full Claude Code toolset (Bash, Read, Write, Edit, Glob, Grep, Skill, MCP) available to autopilot |
| 🔄 Session persistence for restart        | `SessionID` in tasks.json → `--resume` allows exact continuation after service restart            |
| 📡 SSE (not WebSocket/polling)            | Unidirectional push sufficient; simpler than WebSocket; no polling overhead                       |
| 📦 `go:embed` static assets               | Single binary deployment; zero file dependencies                                                  |
| 🔑 `gh` CLI (not GitHub API)              | Reuses user's existing auth; no token management                                                  |
| 🌊 Wave-based parallelism                 | Same wave = concurrent; different waves = sequential; wave 0 = fallback                           |
| 🎭 BMAD party-mode for plans              | Multiple agent personas collaborate; each step gets domain-expert assignment                      |
| 🛡️ Three-layer failure recovery           | L1: retry with context. L2: deliberative fix. L3: graceful failure                                |
| ⚡ Guardrails circuit breaker             | Pauses at 90% utilization; prevents quota exhaustion                                              |
| 🚌 `send func(tea.Msg)` bus               | Universal callback: TUI channel (TUI mode) or logBuf + SSE (web mode)                             |
| 🎯 Model split (`opusplan`)               | Opus for planning (reasoning), Sonnet for execution (speed + cost)                                |
| 🔌 MCP filtering per phase                | Plan gen = `context7` only; exec = `context7` + `github` (others waste turns)                     |
| 🎭 Satirical git identities               | `wordgen` generates random names for autopilot commits; 1600 combinations                         |

---

## 📜 License

2026 - This project is licensed under the [GNU General Public License v3.0](https://www.gnu.org/licenses/gpl-3.0.en.html). You are free to use, modify, and distribute this software under the terms of the GPL-3.0 license. For more details, please refer to the [LICENSE](LICENSE) file included in this repository.
