# Phase 1B: `cilo run` Command

**Duration:** 0.5-1 day  
**Dependencies:** Phase 1 (Foundations) - requires init check, env create/up  
**Goal:** Enable agent-first workflow with a single command

---

## Motivation

Docker Sandboxes introduced `docker sandbox run claude ~/my-project` for agent workflows.
Cilo's equivalent should be even simpler for compose-based environments:

```bash
cilo run opencode feat/testing
```

This creates the environment (if needed), starts it (if down), and launches the agent in the env workspace.

---

## Command Specification

### Syntax

```bash
cilo run <command> <env-name> [flags]
```

### Arguments

| Argument | Required | Description |
|----------|----------|-------------|
| `command` | Yes | Any executable (e.g., `opencode`, `vim`, `bash`, `code`) |
| `env-name` | Yes | Environment name (will be normalized: `feat/testing` → `feat-testing`) |

### Flags

| Flag | Default | Description |
|------|---------|-------------|
| `--from <path>` | Current directory | Project source path (docker-compose.yml location) |
| `--project <name>` | Auto-detected from path | Project namespace for DNS |
| `--no-up` | false | Skip `cilo up` even if env is down |
| `--no-create` | false | Error if env doesn't exist (don't auto-create) |

---

## Behavior Contract

### Execution Flow

```
cilo run <cmd> <env>
       │
       ▼
┌──────────────────┐
│ Check init done? │
└────────┬─────────┘
         │ No
         ├────────────► ERROR: "cilo init required. Run: sudo cilo init"
         │ Yes
         ▼
┌──────────────────┐
│ Env exists?      │
└────────┬─────────┘
         │ No
         ├────────────► Create env from --from (default: cwd)
         │ Yes
         ▼
┌──────────────────┐
│ Env running?     │
└────────┬─────────┘
         │ No
         ├────────────► Run `cilo up <env>`
         │ Yes
         ▼
┌──────────────────┐
│ Set env vars     │
│ cd to workspace  │
│ exec <command>   │
└──────────────────┘
```

### Init Check (BLOCKING)

If `~/.cilo/` doesn't exist or DNS is not configured, `cilo run` **errors immediately**:

```
Error: cilo is not initialized. DNS resolution for *.test domains won't work.

Run:
  sudo cilo init

Then retry:
  cilo run opencode feat-testing
```

This prevents users from launching agents in broken environments.

### Auto-Create Behavior

If the environment doesn't exist:

1. Use `--from` path (or current directory) as project source
2. Auto-detect project name from directory basename (or use `--project`)
3. Create environment: equivalent to `cilo create <env> --from <path> --project <project>`
4. Print: `Created environment: <project>/<env>`

### Auto-Up Behavior

If the environment exists but is not running:

1. Run equivalent of `cilo up <env>`
2. Wait for services to be healthy (or timeout)
3. Print: `Started environment: <project>/<env>`

### Environment Variables

Before exec'ing the command, set:

| Variable | Value | Example |
|----------|-------|---------|
| `CILO_ENV` | Environment name | `feat-testing` |
| `CILO_PROJECT` | Project name | `myapp` |
| `CILO_WORKSPACE` | Absolute workspace path | `/home/user/.cilo/envs/myapp/feat-testing` |
| `CILO_BASE_URL` | Base URL for ingress | `http://myapp.feat-testing.test` |

### Terminal Takeover

The command **replaces the cilo process** (exec, not fork):

```go
// After setup is complete
syscall.Exec(cmdPath, []string{cmd, args...}, env)
```

This means:
- Agent has full terminal control (stdin/stdout/stderr)
- When agent exits, shell returns to prompt
- No cilo wrapper process running

---

## Implementation

### Task 1: Init Check Helper (30 min)

**File:** `pkg/config/init.go`

```go
package config

import (
    "os"
    "path/filepath"
)

// IsInitialized checks if cilo init has been run
func IsInitialized() bool {
    ciloHome := GetCiloHome()
    
    // Check for state file
    statePath := filepath.Join(ciloHome, "state.json")
    if _, err := os.Stat(statePath); os.IsNotExist(err) {
        return false
    }
    
    // Check for DNS config
    dnsPath := filepath.Join(ciloHome, "dns", "dnsmasq.conf")
    if _, err := os.Stat(dnsPath); os.IsNotExist(err) {
        return false
    }
    
    return true
}
```

### Task 2: Run Command (2-3 hours)

**File:** `cmd/run.go`

```go
package cmd

import (
    "fmt"
    "os"
    "os/exec"
    "path/filepath"
    "syscall"

    "github.com/spf13/cobra"
    "github.com/cilo/cilo/pkg/config"
    "github.com/cilo/cilo/pkg/state"
)

var runCmd = &cobra.Command{
    Use:   "run <command> <env>",
    Short: "Run a command in an environment",
    Long: `Create/start an environment and run a command in its workspace.

Examples:
  cilo run opencode dev           # Launch opencode in 'dev' env
  cilo run bash feat-testing      # Open shell in env workspace
  cilo run code staging           # Open VS Code in env workspace
  cilo run vim prod --no-up       # Open vim without starting services`,
    Args: cobra.ExactArgs(2),
    RunE: runRun,
}

func init() {
    runCmd.Flags().String("from", "", "Project source path (default: current directory)")
    runCmd.Flags().String("project", "", "Project name (default: directory basename)")
    runCmd.Flags().Bool("no-up", false, "Don't start the environment")
    runCmd.Flags().Bool("no-create", false, "Don't create if missing")
    rootCmd.AddCommand(runCmd)
}

func runRun(cmd *cobra.Command, args []string) error {
    command := args[0]
    envName := state.NormalizeName(args[1])
    
    fromPath, _ := cmd.Flags().GetString("from")
    projectFlag, _ := cmd.Flags().GetString("project")
    noUp, _ := cmd.Flags().GetBool("no-up")
    noCreate, _ := cmd.Flags().GetBool("no-create")
    
    // 1. Check init
    if !config.IsInitialized() {
        return fmt.Errorf(`cilo is not initialized. DNS resolution for *.test domains won't work.

Run:
  sudo cilo init

Then retry:
  cilo run %s %s`, command, envName)
    }
    
    // 2. Resolve source path
    if fromPath == "" {
        cwd, err := os.Getwd()
        if err != nil {
            return fmt.Errorf("failed to get current directory: %w", err)
        }
        fromPath = cwd
    }
    fromPath, _ = filepath.Abs(fromPath)
    
    // 3. Resolve project name
    project := projectFlag
    if project == "" {
        project = filepath.Base(fromPath)
    }
    project = state.NormalizeName(project)
    
    // 4. Check if env exists
    env, err := state.GetEnvironment(project, envName)
    if err != nil || env == nil {
        if noCreate {
            return fmt.Errorf("environment %s/%s does not exist (use 'cilo create' first, or remove --no-create)", project, envName)
        }
        
        // Create environment
        fmt.Printf("Creating environment: %s/%s\n", project, envName)
        env, err = state.CreateEnvironment(envName, fromPath, project)
        if err != nil {
            return fmt.Errorf("failed to create environment: %w", err)
        }
    }
    
    // 5. Check if running, start if needed
    if !noUp && env.Status != "running" {
        fmt.Printf("Starting environment: %s/%s\n", project, envName)
        if err := upEnvironment(env); err != nil {
            return fmt.Errorf("failed to start environment: %w", err)
        }
    }
    
    // 6. Resolve workspace path
    workspace := getWorkspacePath(env)
    
    // 7. Find command executable
    cmdPath, err := exec.LookPath(command)
    if err != nil {
        return fmt.Errorf("command not found: %s", command)
    }
    
    // 8. Build environment
    environ := os.Environ()
    environ = append(environ,
        fmt.Sprintf("CILO_ENV=%s", envName),
        fmt.Sprintf("CILO_PROJECT=%s", project),
        fmt.Sprintf("CILO_WORKSPACE=%s", workspace),
        fmt.Sprintf("CILO_BASE_URL=http://%s.%s.test", project, envName),
    )
    
    // 9. Change to workspace directory
    if err := os.Chdir(workspace); err != nil {
        return fmt.Errorf("failed to change to workspace: %w", err)
    }
    
    // 10. Exec (replace process)
    fmt.Printf("Launching %s in %s\n\n", command, workspace)
    return syscall.Exec(cmdPath, []string{command}, environ)
}

func getWorkspacePath(env *state.Environment) string {
    return filepath.Join(config.GetCiloHome(), "envs", env.Project, env.Name)
}

func upEnvironment(env *state.Environment) error {
    // Reuse existing up logic
    // This should call the same code path as `cilo up`
    return doUp(env.Project, env.Name)
}
```

---

## Examples

### Basic Usage

```bash
# From project directory
cd ~/projects/myapp
cilo run opencode dev

# Output:
# Creating environment: myapp/dev
# Starting environment: myapp/dev
# Launching opencode in /home/user/.cilo/envs/myapp/dev
# 
# [opencode starts and takes over terminal]
```

### With Existing Environment

```bash
cilo run bash staging

# Output:
# Starting environment: myapp/staging
# Launching bash in /home/user/.cilo/envs/myapp/staging
#
# user@host:~/.cilo/envs/myapp/staging$ 
```

### With Flags

```bash
# Explicit project source
cilo run opencode prod --from ~/projects/myapp-v2 --project myapp

# Skip starting services (just enter workspace)
cilo run vim dev --no-up

# Error if env doesn't exist
cilo run opencode new-feature --no-create
# Error: environment myapp/new-feature does not exist
```

### Environment Variables in Agent

Inside opencode/agent:
```bash
echo $CILO_ENV        # dev
echo $CILO_PROJECT    # myapp
echo $CILO_WORKSPACE  # /home/user/.cilo/envs/myapp/dev
echo $CILO_BASE_URL   # http://myapp.dev.test
```

---

## Success Criteria

- [ ] `cilo run opencode dev` creates + starts + launches in one command
- [ ] Init check blocks with clear error if not initialized
- [ ] Auto-create uses cwd as default source
- [ ] Auto-up starts stopped environments
- [ ] Environment variables are set correctly
- [ ] Command execs (replaces process, no wrapper)
- [ ] `--no-up` and `--no-create` flags work as documented
- [ ] Works with any command (vim, bash, code, cursor, etc.)

---

## Integration with Agent Workflows

### OpenCode Integration

Agents using cilo can:

1. Read `CILO_BASE_URL` to construct service URLs
2. Use `CILO_WORKSPACE` for file operations
3. Call `cilo` commands for environment management

```bash
# Agent can manage sibling environments
cilo create feature-2 --from $CILO_WORKSPACE
cilo up feature-2
curl http://$CILO_PROJECT.feature-2.test/api/health
```

### Suggested Agent Prompt Context

```
You are working in a cilo environment:
- Environment: $CILO_ENV
- Project: $CILO_PROJECT  
- Workspace: $CILO_WORKSPACE
- Base URL: $CILO_BASE_URL

Services are accessible at: <service>.$CILO_PROJECT.$CILO_ENV.test
You can manage environments with: cilo create/up/down/destroy
```

---

## Future Extensions

- `cilo run --attach` - attach to running agent session (tmux integration)
- `cilo run --detach` - start agent in background
- `cilo run --watch` - restart agent on file changes
- Agent registry with pre-configured setup (API keys, configs)
