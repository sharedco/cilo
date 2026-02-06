# /setup - OpenCode Setup Wizard

Setup and validate your OpenCode environment.

## Usage

```
/setup              → Show available setups
/setup standard     → Validate standard fleet (everyone)
/setup advanced     → Install advanced fleet (fleet builders)
/setup linear       → Setup Linear workflow commands (/envoy, /recon, /swarm)
```

## Available Setups

| Setup | Purpose | Installs To |
|-------|---------|-------------|
| `standard` | Validates oh-my-opencode + standard fleet | `.opencode/` (project) |
| `advanced` | Adds OpenAI fleet + advanced commands | `~/.opencode/` (home) |
| `linear` | Configures Linear workflow commands from templates | `CLAUDE_LINEAR_*.md` + `.opencode/commands/` |

## Setup Hierarchy

```
┌─────────────────────────────────────────┐
│  /setup advanced (fleet builders)       │  ← Optional
│  - OpenAI fleet (/openai)               │
│  - /research-mode (premium override), /parallel │
│  - Home dir overrides                   │
├─────────────────────────────────────────┤
│  /setup linear (Linear workflows)       │  ← Optional (requires standard)
│  - /envoy, /recon, /swarm commands      │
│  - Config-driven templates              │
│  - CLAUDE_LINEAR_*.md docs              │
├─────────────────────────────────────────┤
│  /setup standard (everyone)             │  ← Required
│  - Big Guys, Lil Guys, Floaters         │
│  - Scouts, Hadouken, Research           │
│  - MCP validation                       │
├─────────────────────────────────────────┤
│  oh-my-opencode (foundation)            │  ← Prerequisite
│  - Provider auth                        │
│  - Base agent system                    │
└─────────────────────────────────────────┘
```

## Instructions

When user invokes `/setup` with no argument:

```
Available setups:

  standard   Validates oh-my-opencode + installs standard fleet + commands
             - 4 providers (Anthropic, Google, xAI, Z.AI)
             - 11 models
             - 6 MCPs (Linear, Figma, Context7, Exa, GitHub Grep, Playwright)
             - 6 commands (/bigguys, /lilguys, /floaters, /scouts, /hadouken, /research-mode)

  advanced   Installs OpenAI fleet + enhanced commands to ~/.opencode/
             - Requires /setup standard to pass first
             - Adds OpenAI provider (ChatGPT Plus or API)
             - Adds /openai, /research-mode (premium override), /parallel commands
             - Creates ~/.opencode.advanced marker

  linear     Sets up Linear workflow commands from portable templates
             - Requires /setup standard to pass first (Linear MCP)
             - Configures /envoy, /recon, /swarm from templates
             - Uses LINEAR_WORKFLOW_CONFIG.yaml for project-specific values
             - Generates CLAUDE_LINEAR_*.md source of truth docs

Which setup would you like to run? (standard/advanced/linear)
```

When user invokes `/setup standard`:
1. Sync standard commands from `.opencode/setup/standard-commands/*.md` to `.opencode/commands/`
2. Read and execute `.opencode/setup/standard.md` validation steps

When user invokes `/setup advanced`:
1. Sync advanced commands from `.opencode/setup/advanced-commands/*.md` to `~/.opencode/commands/`
2. Read and execute `.opencode/setup/advanced.md` validation steps

When user invokes `/setup linear`:
1. Read and execute `.opencode/setup/standard-commands/setup-linear.md`
2. Process templates from `templates/linear-workflows/`
3. Generate CLAUDE_LINEAR_*.md files
4. Update .opencode/commands/ for /envoy, /recon, /swarm

### Command Sync Logic

For each `*.md` file in source directory:
- Check if file exists in destination
- If missing OR source is newer: copy file
- Report: `✅ Updated [filename]` or `→ [filename] already current`

## Flags

| Flag | Works With | Behavior |
|------|------------|----------|
| `--dry-run` | Both | Show what would be checked/installed |
| `--force` | Advanced | Overwrite existing home dir files |
| `--verbose` | Both | Show detailed output |
| `--skip-mcp` | Standard | Skip MCP validation |
