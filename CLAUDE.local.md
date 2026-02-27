# Local — teamoon

## Workflow Rules

### Party Mode — ALWAYS ACTIVE

- ALWAYS load `/bmad:core:workflows:party-mode` at session start
- After context compaction, reload party mode FIRST before anything else
- Party mode agents MUST participate in Plan Mode discussions — all agents give input
- Party mode and Plan Mode COEXIST — entering Plan Mode does NOT exit party mode

### Plan Mode — MANDATORY for all implementation

- ALWAYS use `EnterPlanMode` before any implementation task (we have `opusplan` configured)
- NEVER code without an approved plan
- Plans MUST create `TaskCreate` tasks for every step — tasks survive context compaction, inline plans do not
- Each plan step = one task with subject, description, and activeForm

### Research & Tools — use what's available

- Investigate before coding — read relevant code, understand the problem first
- Use `context7` MCP when a library is involved (e.g., Bubbletea, Cobra, Lipgloss) to get up-to-date docs
- Use `chrome-devtools` MCP to inspect web pages when WebFetch/curl fails or returns incomplete content
- If chrome-devtools also fails, fall back to `brave-search` MCP for web content
- Escalation order: WebFetch → chrome-devtools → brave-search
- NEVER read or apply information from other projects (cloud-agent-package, cloud-adm, etc.) to teamoon — each project is independent
- NEVER invent data — if you need project metadata (author, license, org, URLs), read it from THIS repo's README.md or LICENSE

## Systemd Service

```bash
sudo systemctl status teamoon     # status
sudo systemctl restart teamoon    # restart
sudo journalctl -u teamoon -f     # live logs
```

## Web UI

- Port: 7777 (configurable in config.json)
- URL: http://localhost:7777
- Auth: Basic Auth (credentials in `~/.config/teamoon/.env`)

## Autopilot Config

- Model: `opusplan` (Opus in Plan Mode)
- Effort: `high`
- Max turns per step: 15
- Step timeout: 4 min
- Max concurrent projects: 3
