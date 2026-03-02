package projectinit

import (
	"fmt"
	"os"
	"path/filepath"
)

const hooksSettingsJSON = `{
  "enabledPlugins": {
    "claude-mem@thedotmack": false
  },
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Bash",
        "hooks": [
          { "type": "command", "command": ".claude/hooks/combined-guard.sh", "timeout": 5000 }
        ]
      },
      {
        "matcher": "Write|Edit",
        "hooks": [
          { "type": "command", "command": ".claude/hooks/secrets-guard.sh", "timeout": 5000 }
        ]
      }
    ]
  }
}
`

const securityCheckSH = `#!/bin/bash
set -eo pipefail
INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')
[ -z "$CMD" ] && echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}' && exit 0

deny() {
  echo "{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$1\"}}"
  exit 0
}

# =============================================================================
# GIT — Force push (any variant)
# =============================================================================
echo "$CMD" | grep -qiE 'git\s+push\s+.*--force' && deny "BLOCKED: git push --force is prohibited"
echo "$CMD" | grep -qiE 'git\s+push\s+.*--force-with-lease' && deny "BLOCKED: git push --force-with-lease is prohibited"
echo "$CMD" | grep -qiE 'git\s+push\s+.*\s-f(\s|$)' && deny "BLOCKED: git push -f is prohibited"

# GIT — Reset hard
echo "$CMD" | grep -qiE 'git\s+reset\s+--hard' && deny "BLOCKED: git reset --hard is prohibited"

# GIT — Skip verification hooks
echo "$CMD" | grep -qiE 'git\s+.*--no-verify' && deny "BLOCKED: --no-verify is prohibited — never skip hooks"
echo "$CMD" | grep -qiE 'git\s+.*--no-gpg-sign' && deny "BLOCKED: --no-gpg-sign is prohibited"

# GIT — Delete remote branch
echo "$CMD" | grep -qiE 'git\s+push\s+.*--delete' && deny "BLOCKED: git push --delete is prohibited"
echo "$CMD" | grep -qiE 'git\s+push\s+\S+\s+:' && deny "BLOCKED: deleting remote branch via push is prohibited"

# GIT — Force delete local branch
echo "$CMD" | grep -qiE 'git\s+branch\s+-D\s' && deny "BLOCKED: git branch -D is prohibited — use -d for safe delete"

# GIT — Clean force (deletes untracked files)
echo "$CMD" | grep -qiE 'git\s+clean\s+.*-f' && deny "BLOCKED: git clean -f is prohibited"

# GIT — Discard all changes
echo "$CMD" | grep -qiE 'git\s+checkout\s+\.\s*$' && deny "BLOCKED: git checkout . discards all changes"
echo "$CMD" | grep -qiE 'git\s+restore\s+\.\s*$' && deny "BLOCKED: git restore . discards all changes"

# GIT — Blind staging (must review files explicitly)
echo "$CMD" | grep -qiE 'git\s+add\s+-A' && deny "BLOCKED: git add -A is prohibited — stage files explicitly"
echo "$CMD" | grep -qiE 'git\s+add\s+--all' && deny "BLOCKED: git add --all is prohibited — stage files explicitly"
echo "$CMD" | grep -qiE 'git\s+add\s+\.\s*$' && deny "BLOCKED: git add . is prohibited — stage files explicitly"

# =============================================================================
# FILESYSTEM — Destructive rm
# =============================================================================
echo "$CMD" | grep -qiE 'rm\s+-rf\s+/' && deny "BLOCKED: rm -rf / is prohibited"
echo "$CMD" | grep -qiE 'rm\s+-rf\s+~' && deny "BLOCKED: rm -rf ~ is prohibited"
echo "$CMD" | grep -qiE 'rm\s+-rf\s+\.\s*$' && deny "BLOCKED: rm -rf . is prohibited"
echo "$CMD" | grep -qiE 'rm\s+-r\s+-f\s' && deny "BLOCKED: rm -r -f is prohibited"

# FILESYSTEM — Dangerous permissions
echo "$CMD" | grep -qiE 'chmod\s+777\s' && deny "BLOCKED: chmod 777 is prohibited — use specific permissions"
echo "$CMD" | grep -qiE 'chmod\s+-R\s+777' && deny "BLOCKED: chmod -R 777 is prohibited"

# =============================================================================
# REMOTE CODE EXECUTION — Piping remote scripts to shell
# =============================================================================
echo "$CMD" | grep -qiE 'curl\s.*\|\s*(sh|bash)' && deny "BLOCKED: piping curl to shell is prohibited — download and review first"
echo "$CMD" | grep -qiE 'wget\s.*\|\s*(sh|bash)' && deny "BLOCKED: piping wget to shell is prohibited — download and review first"
echo "$CMD" | grep -qiE 'curl\s.*\|\s*sudo' && deny "BLOCKED: piping curl to sudo is prohibited"

# =============================================================================
# SQL — Destructive operations
# =============================================================================
echo "$CMD" | grep -qiE 'DROP\s+(TABLE|DATABASE|SCHEMA|INDEX)' && deny "BLOCKED: DROP operations are prohibited"
echo "$CMD" | grep -qiE 'TRUNCATE\s+TABLE' && deny "BLOCKED: TRUNCATE TABLE is prohibited"
echo "$CMD" | grep -qiE 'DELETE\s+FROM\s+\S+\s*;?\s*$' && deny "BLOCKED: DELETE without WHERE clause is prohibited"

# =============================================================================
# DOCKER — Destructive operations
# =============================================================================
echo "$CMD" | grep -qiE 'docker\s+system\s+prune' && deny "BLOCKED: docker system prune is prohibited"
echo "$CMD" | grep -qiE 'docker\s+rm\s+-f' && deny "BLOCKED: docker rm -f is prohibited"
echo "$CMD" | grep -qiE 'docker\s+rmi\s+-f' && deny "BLOCKED: docker rmi -f is prohibited"

# =============================================================================
# PROCESS — Kill signals
# =============================================================================
echo "$CMD" | grep -qiE 'kill\s+-9\s' && deny "BLOCKED: kill -9 is prohibited — use graceful signals first"
echo "$CMD" | grep -qiE 'pkill\s+-9\s' && deny "BLOCKED: pkill -9 is prohibited"
echo "$CMD" | grep -qiE 'killall\s' && deny "BLOCKED: killall is prohibited"

# =============================================================================
# SUDO — Privilege escalation
# =============================================================================
echo "$CMD" | grep -qiE '^sudo\s+rm\s' && deny "BLOCKED: sudo rm is prohibited"
echo "$CMD" | grep -qiE '^sudo\s+chmod\s' && deny "BLOCKED: sudo chmod is prohibited"
echo "$CMD" | grep -qiE '^sudo\s+chown\s' && deny "BLOCKED: sudo chown is prohibited"

# =============================================================================
# CREDENTIALS — Reading sensitive files
# =============================================================================
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*\.env' && deny "BLOCKED: reading .env files via CLI is prohibited"
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*id_rsa' && deny "BLOCKED: reading SSH keys is prohibited"
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*credentials' && deny "BLOCKED: reading credential files is prohibited"
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*\.pem' && deny "BLOCKED: reading PEM files is prohibited"
echo "$CMD" | grep -qiE 'grep\s+.*\.env' && deny "BLOCKED: searching .env files is prohibited"
echo "$CMD" | grep -qiE 'rg\s+.*\.env' && deny "BLOCKED: searching .env files is prohibited"

# =============================================================================
# AWS/CLOUD — Destructive operations
# =============================================================================
echo "$CMD" | grep -qiE 'aws\s+.*lambda\s+update-function-configuration' && deny "BLOCKED: direct lambda config update is prohibited — use CloudFormation"
echo "$CMD" | grep -qiE 'aws\s+.*s3\s+.*--delete' && deny "BLOCKED: aws s3 --delete is prohibited"
echo "$CMD" | grep -qiE 'aws\s+.*cloudformation\s+delete-stack' && deny "BLOCKED: deleting CloudFormation stacks is prohibited"
echo "$CMD" | grep -qiE 'aws\s+.*iam\s+create-access-key' && deny "BLOCKED: creating IAM access keys requires explicit approval"
echo "$CMD" | grep -qiE 'terraform\s+destroy' && deny "BLOCKED: terraform destroy is prohibited"

echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
`

const testGuardSH = `#!/bin/bash
set -eo pipefail
INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')
[ -z "$CMD" ] && echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}' && exit 0

FLAG="/tmp/teamoon-tests-passed"

deny() {
  echo "{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$1\"}}"
  exit 0
}

# Detect test runners — set flag
if echo "$CMD" | grep -qiE '(go\s+test|pytest|npm\s+(run\s+)?test|vitest|jest|cargo\s+test|make\s+test|bun\s+test|deno\s+test|php\s+artisan\s+test|bundle\s+exec\s+rspec)'; then
  touch "$FLAG"
  echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
  exit 0
fi

# Detect git commit — check flag
if echo "$CMD" | grep -qiE 'git\s+commit'; then
  [ ! -f "$FLAG" ] && deny "BLOCKED: Run tests before committing. No test execution detected in this session."
fi

echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
`

const secretsGuardSH = `#!/bin/bash
set -eo pipefail
INPUT=$(cat)
FILE=$(echo "$INPUT" | jq -r '.tool_input.file_path // ""')
[ -z "$FILE" ] && echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}' && exit 0

BASENAME=$(basename "$FILE")

deny() {
  echo "{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$1\"}}"
  exit 0
}

# =============================================================================
# ENVIRONMENT FILES
# =============================================================================
echo "$BASENAME" | grep -qiE '^\.env(\..+)?$' && deny "BLOCKED: Writing to $BASENAME is prohibited (environment credentials)"

# =============================================================================
# CREDENTIAL FILES
# =============================================================================
echo "$BASENAME" | grep -qiE '^credentials' && deny "BLOCKED: Writing to credential files is prohibited"
echo "$BASENAME" | grep -qiE '^\.git-credentials$' && deny "BLOCKED: Writing to .git-credentials is prohibited"
echo "$BASENAME" | grep -qiE '^\.netrc$' && deny "BLOCKED: Writing to .netrc is prohibited"

# =============================================================================
# KEY AND CERTIFICATE FILES
# =============================================================================
echo "$BASENAME" | grep -qiE '\.(pem|key|ppk|p12|pfx|jks|keystore)$' && deny "BLOCKED: Writing to key/certificate files is prohibited"

# =============================================================================
# SSH KEYS
# =============================================================================
echo "$BASENAME" | grep -qiE '^id_(rsa|ed25519|ecdsa|dsa)(\.pub)?$' && deny "BLOCKED: Writing to SSH key files is prohibited"
echo "$BASENAME" | grep -qiE '^known_hosts$' && deny "BLOCKED: Writing to known_hosts is prohibited"
echo "$BASENAME" | grep -qiE '^authorized_keys$' && deny "BLOCKED: Writing to authorized_keys is prohibited"

# =============================================================================
# CLOUD CREDENTIALS
# =============================================================================
echo "$FILE" | grep -qiE '\.aws/credentials' && deny "BLOCKED: Writing to AWS credentials is prohibited"
echo "$FILE" | grep -qiE '\.aws/config' && deny "BLOCKED: Writing to AWS config is prohibited"
echo "$FILE" | grep -qiE '\.oci/config' && deny "BLOCKED: Writing to OCI config is prohibited"
echo "$FILE" | grep -qiE '\.config/gcloud' && deny "BLOCKED: Writing to GCloud config is prohibited"
echo "$FILE" | grep -qiE '\.kube/config' && deny "BLOCKED: Writing to kubectl config is prohibited"
echo "$FILE" | grep -qiE '\.docker/config\.json' && deny "BLOCKED: Writing to Docker config is prohibited"

# =============================================================================
# DATABASE CREDENTIAL FILES
# =============================================================================
echo "$BASENAME" | grep -qiE '^\.pgpass$' && deny "BLOCKED: Writing to .pgpass is prohibited"
echo "$BASENAME" | grep -qiE '^\.my\.cnf$' && deny "BLOCKED: Writing to .my.cnf is prohibited"
echo "$BASENAME" | grep -qiE '^\.mongorc\.js$' && deny "BLOCKED: Writing to .mongorc.js is prohibited"

# =============================================================================
# SHELL HISTORY
# =============================================================================
echo "$BASENAME" | grep -qiE '^\.(bash|zsh|python|node_repl|mysql|psql)_history$' && deny "BLOCKED: Writing to shell history is prohibited"

# =============================================================================
# SENSITIVE DIRECTORIES
# =============================================================================
echo "$FILE" | grep -qiE '(secrets|keys|certs|private)/' && deny "BLOCKED: Writing to sensitive directory is prohibited"

# =============================================================================
# LOCK FILES (should not be manually edited)
# =============================================================================
echo "$BASENAME" | grep -qiE '^(package-lock\.json|pnpm-lock\.yaml|yarn\.lock|Gemfile\.lock|poetry\.lock|Cargo\.lock|go\.sum)$' && deny "BLOCKED: Lock files should not be edited manually — use package manager commands"

echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
`

const buildGuardSH = `#!/bin/bash
set -eo pipefail
INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')
[ -z "$CMD" ] && echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}' && exit 0

FLAG="/tmp/teamoon-build-passed"

deny() {
  echo "{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$1\"}}"
  exit 0
}

# Detect build commands — set flag
if echo "$CMD" | grep -qiE '(make\s+(build|install)|npm\s+run\s+build|npx\s+vite\s+build|cargo\s+build|go\s+build|docker\s+build|gradle\s+build|mvn\s+(compile|package)|dotnet\s+build)'; then
  touch "$FLAG"
  echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
  exit 0
fi

# Detect git push — check flag
if echo "$CMD" | grep -qiE 'git\s+push'; then
  [ ! -f "$FLAG" ] && deny "BLOCKED: Build before pushing. No build execution detected in this session."
fi

echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
`

const commitFormatSH = `#!/bin/bash
set -eo pipefail
INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')
[ -z "$CMD" ] && echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}' && exit 0

deny() {
  echo "{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$1\"}}"
  exit 0
}

# Only check git commit -m
if echo "$CMD" | grep -qiE 'git\s+commit\s+.*-m\s'; then
  # Extract message between quotes (single or double) after -m
  MSG=$(echo "$CMD" | sed -nE "s/.*-m\s+[\"']([^\"']+)[\"'].*/\1/p")
  # Fallback: heredoc style (cat <<'EOF' ... EOF)
  [ -z "$MSG" ] && MSG=$(echo "$CMD" | sed -nE "s/.*-m\s+\"?\\\$\(cat <<.*//p")
  # If still empty, try unquoted
  [ -z "$MSG" ] && MSG=$(echo "$CMD" | sed -nE 's/.*-m\s+([^ ]+).*/\1/p')
  [ -z "$MSG" ] && echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}' && exit 0

  # Validate format: type(core): lowercase description
  if ! echo "$MSG" | grep -qE '^(feat|fix|refactor|docs|style|test|chore)\(core\): [a-z]'; then
    deny "BLOCKED: Commit message must match type(core): lowercase description. Got: $MSG"
  fi

  # Block multiple lines in message (should be single line)
  LINECOUNT=$(echo "$MSG" | head -1 | wc -l)
  FIRSTLINE=$(echo "$MSG" | head -1)

  # Check for emojis (common unicode ranges)
  if echo "$FIRSTLINE" | grep -qP '[\x{1F300}-\x{1F9FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]' 2>/dev/null; then
    deny "BLOCKED: Emojis are not allowed in commit messages"
  fi

  # Check for uppercase after colon-space
  AFTER_COLON=$(echo "$FIRSTLINE" | sed -nE 's/^[^:]+:\s+(.*)/\1/p')
  if [ -n "$AFTER_COLON" ]; then
    FIRST_CHAR=$(echo "$AFTER_COLON" | cut -c1)
    if echo "$FIRST_CHAR" | grep -qE '[A-Z]'; then
      deny "BLOCKED: Description must start lowercase after colon. Got: $FIRSTLINE"
    fi
  fi

  # Check scope is always core
  if echo "$FIRSTLINE" | grep -qE '^\w+\(' && ! echo "$FIRSTLINE" | grep -qE '^\w+\(core\)'; then
    deny "BLOCKED: Scope must always be (core). Got: $FIRSTLINE"
  fi
fi

echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
`

// combinedGuardSH merges security-check, test-guard, build-guard, and commit-format
// into a single script. This reduces PreToolUse hook events from 8 per Bash call to 2,
// preventing spawned agents from wasting turns responding to hook event noise.
const combinedGuardSH = `#!/bin/bash
set -eo pipefail
INPUT=$(cat)
CMD=$(echo "$INPUT" | jq -r '.tool_input.command // ""')
[ -z "$CMD" ] && echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}' && exit 0

deny() {
  echo "{\"hookSpecificOutput\":{\"permissionDecision\":\"deny\",\"permissionDecisionReason\":\"$1\"}}"
  exit 0
}

# =============================================================================
# SECURITY — Git destructive operations
# =============================================================================
echo "$CMD" | grep -qiE 'git\s+push\s+.*--force' && deny "BLOCKED: git push --force is prohibited"
echo "$CMD" | grep -qiE 'git\s+push\s+.*--force-with-lease' && deny "BLOCKED: git push --force-with-lease is prohibited"
echo "$CMD" | grep -qiE 'git\s+push\s+.*\s-f(\s|$)' && deny "BLOCKED: git push -f is prohibited"
echo "$CMD" | grep -qiE 'git\s+reset\s+--hard' && deny "BLOCKED: git reset --hard is prohibited"
echo "$CMD" | grep -qiE 'git\s+.*--no-verify' && deny "BLOCKED: --no-verify is prohibited — never skip hooks"
echo "$CMD" | grep -qiE 'git\s+.*--no-gpg-sign' && deny "BLOCKED: --no-gpg-sign is prohibited"
echo "$CMD" | grep -qiE 'git\s+push\s+.*--delete' && deny "BLOCKED: git push --delete is prohibited"
echo "$CMD" | grep -qiE 'git\s+push\s+\S+\s+:' && deny "BLOCKED: deleting remote branch via push is prohibited"
echo "$CMD" | grep -qiE 'git\s+branch\s+-D\s' && deny "BLOCKED: git branch -D is prohibited — use -d for safe delete"
echo "$CMD" | grep -qiE 'git\s+clean\s+.*-f' && deny "BLOCKED: git clean -f is prohibited"
echo "$CMD" | grep -qiE 'git\s+checkout\s+\.\s*$' && deny "BLOCKED: git checkout . discards all changes"
echo "$CMD" | grep -qiE 'git\s+restore\s+\.\s*$' && deny "BLOCKED: git restore . discards all changes"
echo "$CMD" | grep -qiE 'git\s+add\s+-A' && deny "BLOCKED: git add -A is prohibited — stage files explicitly"
echo "$CMD" | grep -qiE 'git\s+add\s+--all' && deny "BLOCKED: git add --all is prohibited — stage files explicitly"
echo "$CMD" | grep -qiE 'git\s+add\s+\.\s*$' && deny "BLOCKED: git add . is prohibited — stage files explicitly"

# SECURITY — Filesystem
echo "$CMD" | grep -qiE 'rm\s+-rf\s+/' && deny "BLOCKED: rm -rf / is prohibited"
echo "$CMD" | grep -qiE 'rm\s+-rf\s+~' && deny "BLOCKED: rm -rf ~ is prohibited"
echo "$CMD" | grep -qiE 'rm\s+-rf\s+\.\s*$' && deny "BLOCKED: rm -rf . is prohibited"
echo "$CMD" | grep -qiE 'rm\s+-r\s+-f\s' && deny "BLOCKED: rm -r -f is prohibited"
echo "$CMD" | grep -qiE 'chmod\s+777\s' && deny "BLOCKED: chmod 777 is prohibited — use specific permissions"
echo "$CMD" | grep -qiE 'chmod\s+-R\s+777' && deny "BLOCKED: chmod -R 777 is prohibited"

# SECURITY — Remote code execution
echo "$CMD" | grep -qiE 'curl\s.*\|\s*(sh|bash)' && deny "BLOCKED: piping curl to shell is prohibited — download and review first"
echo "$CMD" | grep -qiE 'wget\s.*\|\s*(sh|bash)' && deny "BLOCKED: piping wget to shell is prohibited — download and review first"
echo "$CMD" | grep -qiE 'curl\s.*\|\s*sudo' && deny "BLOCKED: piping curl to sudo is prohibited"

# SECURITY — SQL
echo "$CMD" | grep -qiE 'DROP\s+(TABLE|DATABASE|SCHEMA|INDEX)' && deny "BLOCKED: DROP operations are prohibited"
echo "$CMD" | grep -qiE 'TRUNCATE\s+TABLE' && deny "BLOCKED: TRUNCATE TABLE is prohibited"
echo "$CMD" | grep -qiE 'DELETE\s+FROM\s+\S+\s*;?\s*$' && deny "BLOCKED: DELETE without WHERE clause is prohibited"

# SECURITY — Docker
echo "$CMD" | grep -qiE 'docker\s+system\s+prune' && deny "BLOCKED: docker system prune is prohibited"
echo "$CMD" | grep -qiE 'docker\s+rm\s+-f' && deny "BLOCKED: docker rm -f is prohibited"
echo "$CMD" | grep -qiE 'docker\s+rmi\s+-f' && deny "BLOCKED: docker rmi -f is prohibited"

# SECURITY — Process signals
echo "$CMD" | grep -qiE 'kill\s+-9\s' && deny "BLOCKED: kill -9 is prohibited — use graceful signals first"
echo "$CMD" | grep -qiE 'pkill\s+-9\s' && deny "BLOCKED: pkill -9 is prohibited"
echo "$CMD" | grep -qiE 'killall\s' && deny "BLOCKED: killall is prohibited"

# SECURITY — Sudo
echo "$CMD" | grep -qiE '^sudo\s+rm\s' && deny "BLOCKED: sudo rm is prohibited"
echo "$CMD" | grep -qiE '^sudo\s+chmod\s' && deny "BLOCKED: sudo chmod is prohibited"
echo "$CMD" | grep -qiE '^sudo\s+chown\s' && deny "BLOCKED: sudo chown is prohibited"

# SECURITY — Credentials
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*\.env' && deny "BLOCKED: reading .env files via CLI is prohibited"
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*id_rsa' && deny "BLOCKED: reading SSH keys is prohibited"
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*credentials' && deny "BLOCKED: reading credential files is prohibited"
echo "$CMD" | grep -qiE '(cat|less|more|head|tail|bat)\s+.*\.pem' && deny "BLOCKED: reading PEM files is prohibited"
echo "$CMD" | grep -qiE 'grep\s+.*\.env' && deny "BLOCKED: searching .env files is prohibited"
echo "$CMD" | grep -qiE 'rg\s+.*\.env' && deny "BLOCKED: searching .env files is prohibited"

# SECURITY — AWS/Cloud
echo "$CMD" | grep -qiE 'aws\s+.*lambda\s+update-function-configuration' && deny "BLOCKED: direct lambda config update is prohibited — use CloudFormation"
echo "$CMD" | grep -qiE 'aws\s+.*s3\s+.*--delete' && deny "BLOCKED: aws s3 --delete is prohibited"
echo "$CMD" | grep -qiE 'aws\s+.*cloudformation\s+delete-stack' && deny "BLOCKED: deleting CloudFormation stacks is prohibited"
echo "$CMD" | grep -qiE 'aws\s+.*iam\s+create-access-key' && deny "BLOCKED: creating IAM access keys requires explicit approval"
echo "$CMD" | grep -qiE 'terraform\s+destroy' && deny "BLOCKED: terraform destroy is prohibited"

# =============================================================================
# TEST GUARD — Track test execution, require before commit
# =============================================================================
TEST_FLAG="/tmp/teamoon-tests-passed"
if echo "$CMD" | grep -qiE '(go\s+test|pytest|npm\s+(run\s+)?test|vitest|jest|cargo\s+test|make\s+test|bun\s+test|deno\s+test|php\s+artisan\s+test|bundle\s+exec\s+rspec)'; then
  touch "$TEST_FLAG"
fi

# =============================================================================
# BUILD GUARD — Track build execution, require before push
# =============================================================================
BUILD_FLAG="/tmp/teamoon-build-passed"
if echo "$CMD" | grep -qiE '(make\s+(build|install)|npm\s+run\s+build|npx\s+vite\s+build|cargo\s+build|go\s+build|docker\s+build|gradle\s+build|mvn\s+(compile|package)|dotnet\s+build)'; then
  touch "$BUILD_FLAG"
fi

# =============================================================================
# COMMIT — Require tests + validate format
# =============================================================================
if echo "$CMD" | grep -qiE 'git\s+commit'; then
  [ ! -f "$TEST_FLAG" ] && deny "BLOCKED: Run tests before committing. No test execution detected in this session."
  if echo "$CMD" | grep -qiE 'git\s+commit\s+.*-m\s'; then
    MSG=$(echo "$CMD" | sed -nE "s/.*-m\s+[\"']([^\"']+)[\"'].*/\1/p")
    [ -z "$MSG" ] && MSG=$(echo "$CMD" | sed -nE "s/.*-m\s+\"?\\\$\(cat <<.*//p")
    [ -z "$MSG" ] && MSG=$(echo "$CMD" | sed -nE 's/.*-m\s+([^ ]+).*/\1/p')
    if [ -n "$MSG" ]; then
      echo "$MSG" | grep -qE '^(feat|fix|refactor|docs|style|test|chore)\(core\): [a-z]' || \
        deny "BLOCKED: Commit message must match type(core): lowercase description. Got: $MSG"
      FIRSTLINE=$(echo "$MSG" | head -1)
      if echo "$FIRSTLINE" | grep -qP '[\x{1F300}-\x{1F9FF}\x{2600}-\x{26FF}\x{2700}-\x{27BF}]' 2>/dev/null; then
        deny "BLOCKED: Emojis are not allowed in commit messages"
      fi
      AFTER_COLON=$(echo "$FIRSTLINE" | sed -nE 's/^[^:]+:\s+(.*)/\1/p')
      if [ -n "$AFTER_COLON" ]; then
        FIRST_CHAR=$(echo "$AFTER_COLON" | cut -c1)
        echo "$FIRST_CHAR" | grep -qE '[A-Z]' && deny "BLOCKED: Description must start lowercase after colon. Got: $FIRSTLINE"
      fi
      echo "$FIRSTLINE" | grep -qE '^\w+\(' && ! echo "$FIRSTLINE" | grep -qE '^\w+\(core\)' && \
        deny "BLOCKED: Scope must always be (core). Got: $FIRSTLINE"
    fi
  fi
fi

# =============================================================================
# PUSH — Require build before push
# =============================================================================
if echo "$CMD" | grep -qiE 'git\s+push'; then
  [ ! -f "$BUILD_FLAG" ] && deny "BLOCKED: Build before pushing. No build execution detected in this session."
fi

echo '{"hookSpecificOutput":{"permissionDecision":"allow"}}'
`

var hookFiles = map[string]string{
	"combined-guard.sh": combinedGuardSH,
	"secrets-guard.sh":  secretsGuardSH,
}

// GlobalHookFiles returns the hook script contents keyed by filename.
// Allows other packages to access the hook scripts without duplicating them.
func GlobalHookFiles() map[string]string {
	return hookFiles
}

// InstallHooks creates .claude/hooks/ in the target project directory
// with all security hook scripts, settings.json, CLAUDE.md, and MEMORY.md.
func InstallHooks(projectDir, projectName, projectType string) error {
	hooksDir := filepath.Join(projectDir, ".claude", "hooks")
	if err := os.MkdirAll(hooksDir, 0755); err != nil {
		return err
	}

	settingsPath := filepath.Join(projectDir, ".claude", "settings.json")
	if err := os.WriteFile(settingsPath, []byte(hooksSettingsJSON), 0644); err != nil {
		return err
	}

	for name, content := range hookFiles {
		path := filepath.Join(hooksDir, name)
		if err := os.WriteFile(path, []byte(content), 0755); err != nil {
			return err
		}
	}

	// Write CLAUDE.md and MEMORY.md at project root
	claudeMD := buildClaudeMD(projectName, projectType)
	if err := os.WriteFile(filepath.Join(projectDir, "CLAUDE.md"), []byte(claudeMD), 0644); err != nil {
		return err
	}

	memoryMD := buildMemoryMD(projectName, projectType)
	if err := os.WriteFile(filepath.Join(projectDir, "MEMORY.md"), []byte(memoryMD), 0644); err != nil {
		return err
	}

	// Symlink .bmad → teamoon's installed BMAD commands so @.bmad/ paths resolve
	if err := EnsureBMADLink(projectDir); err != nil {
		return err
	}

	return nil
}

// EnsureBMADLink creates symlinks so BMAD skills are discoverable by spawned claude processes:
//   - <project>/.bmad → ~/.config/teamoon/commands/bmad (for @.bmad/ paths)
//   - <project>/.claude/commands/bmad → same (for Claude Code Skill tool)
func EnsureBMADLink(projectDir string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	target := filepath.Join(home, ".config", "teamoon", "commands", "bmad")
	if _, err := os.Stat(target); err != nil {
		return nil
	}
	ensureSingleSymlink(filepath.Join(projectDir, ".bmad"), target)
	cmdDir := filepath.Join(projectDir, ".claude", "commands")
	os.MkdirAll(cmdDir, 0755)
	ensureSingleSymlink(filepath.Join(cmdDir, "bmad"), target)
	return nil
}

func ensureSingleSymlink(link, target string) {
	if dest, err := os.Readlink(link); err == nil && dest == target {
		return
	}
	if info, err := os.Lstat(link); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(link)
		} else if !info.IsDir() {
			os.Remove(link)
		} else {
			return
		}
	}
	os.Symlink(target, link)
}

func buildClaudeMD(name, projectType string) string {
	// Build/test commands per project type
	var buildCmd, testCmd, lintCmd, fmtCmd string
	switch projectType {
	case "python":
		buildCmd = "poetry install"
		testCmd = "pytest tests/ -v --cov=src"
		lintCmd = "pylint **/*.py"
		fmtCmd = "black . && isort ."
	case "node":
		buildCmd = "npm run build"
		testCmd = "npm run test"
		lintCmd = "eslint . --ext .js,.ts,.vue"
		fmtCmd = "prettier --write ."
	case "go":
		buildCmd = "make build"
		testCmd = "go test ./... -v -count=1"
		lintCmd = "golangci-lint run"
		fmtCmd = "gofmt -w ."
	default:
		buildCmd = "make build"
		testCmd = "make test"
		lintCmd = "make lint"
		fmtCmd = "make fmt"
	}

	return fmt.Sprintf(`# Claude Instructions — %s

## Project

- **Name**: %s
- **Type**: %s
- **Branch strategy**: main (stable) + dev (development)

## Build & Test

| Action | Command |
|--------|---------|
| Build | %s |
| Test | %s |
| Lint | %s |
| Format | %s |

## Workflow — MANDATORY Steps

Every code change MUST follow these steps in order:

1. **Investigate** — Read CLAUDE.md, MEMORY.md, README.md, CONTRIBUTING.md. Understand the codebase before changing anything.
2. **Research** — If using unfamiliar libraries, use resolve-library-id + query-docs (Context7) to look up current API docs.
3. **Implement** — Make the actual code changes. Be precise, minimal, and follow existing patterns.
4. **Build** — Run: %s — Fix any compilation errors.
5. **Test** — Run: %s — Write new tests for new code. Fix failures.
6. **Pre-commit** — Run: pre-commit run --all-files — Fix any linting/formatting issues.
7. **Commit** — Single line: type(core): description in lowercase. NO emojis, NO Co-Authored-By, NO 'Made by Claude'. Stage specific files (never git add -A).
8. **Push** — Only when explicitly requested.

## Commit Format

` + "```" + `
type(core): description in lowercase
` + "```" + `

Types: feat, fix, refactor, docs, style, test, chore
Scope: ALWAYS core
Type by PURPOSE: feat includes its tests, fix includes its tests, test(core) ONLY for coverage of existing code.

## Rules

- NEVER git push --force, git reset --hard, --no-verify
- NEVER use heredoc (<<EOF, <<'EOF', cat <<) in any command. Use direct strings with quotes.
- NEVER create .md files unless explicitly requested (no SUMMARY.md, ANALYSIS.md, etc.)
- NEVER commit: .env, CLAUDE.md, MEMORY.md, CONTEXT.md, *.pem, certs/, secrets/
- NEVER read or print credential files (.env, *.pem, id_rsa, etc.)
- ALWAYS work on dev branch. If not on dev: git checkout dev
- ALWAYS run tests before committing
- ALWAYS run build before pushing
- ONE commit per task, grouping all changes
- Commits: single line only. NO Co-Authored-By, NO 'Made by Claude', NO AI attribution.
- NEVER create PRs with Claude/AI mentions in title or body.
- NEVER install packages directly (npm install pkg, pip install pkg). ALWAYS add to the manifest file first (package.json, requirements.txt, go.mod) then run the install command without package names.

`, name, name, projectType, buildCmd, testCmd, lintCmd, fmtCmd, buildCmd, testCmd)
}

func buildMemoryMD(name, projectType string) string {
	return fmt.Sprintf(`# %s — Memory

## Project Info

- Type: %s
- Created: auto-generated by teamoon init

## Patterns Discovered

(Auto-populated as Claude learns about this project)

## Known Issues

(Track recurring problems here)
`, name, projectType)
}
