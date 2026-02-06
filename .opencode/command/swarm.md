# Linear Workflow - Implementation (Swarm)

> **Configuration Required:** This template uses values from `LINEAR_WORKFLOW_CONFIG.yaml`.
> Variables are replaced from `commands.yaml` at generation time.

This document defines an implementation workflow where AI agents implement directly, with automatic git operations and PR creation guidance.

## Philosophy

**Plan (from /recon) -> Validate Branch -> Implement -> Test -> Git Add/Commit/Push -> PR Instructions**

- AI reads issue and any existing plan from `/recon`
- Validates and creates feature branch from correct base
- Implements the solution
- Runs git add, commit, push automatically
- Provides PR creation instructions
- Human reviews code and creates PR

---

## Status Flow

```
Ready For Dev
    ↓ (/swarm starts)
Dev In Progress
    ↓ (implementation + git push complete)
Code Review
    ↓ (human approval or authorized /envoy)
Done
```

---

## CRITICAL RULES

### Branch Validation (BLOCKING)

**Before ANY implementation, AI MUST validate the current branch:**

1. **Check current branch** in EACH allowed repo ([{'path': 'sites/client-core', 'name': 'Frontend (Ionic)', 'base_branch': 'development'}, {'path': 'sites/api.praisecharts.com', 'name': 'Backend (API)', 'base_branch': 'development'}])
2. **If on `development` or any protected branch:**
   - [OK] Create feature branch automatically
   - Branch naming: `{branch_type}/{issue_id}-{issue_title_kebab}`
   - Post in Linear: "Created branch: `[branch-name]`"

3. **If on a DIFFERENT branch (not protected, not feature branch for this issue):**
   - [X] **STOP and ALERT user:**
   ```
   [!] Branch Confirmation Required
   
   You are currently on branch: `[current-branch]`
   Expected base branch: `development`
   
   Options:
   1. Type "continue" to branch from `[current-branch]`
   2. Type "switch" to checkout `development` first
   
   Which would you like to do?
   ```
   - **Wait for user confirmation before proceeding**

4. **If already on the correct feature branch for this issue:**
   - [OK] Continue on current branch

### Branch Type Determination

Branch prefix is determined by issue labels:

| Label | Branch Type | Example |
|-------|-------------|---------|
| Bug | `fix` | `fix/ENG-42-login-error` |
| Feature | `feat` | `feat/ENG-73-export-pdf` |
| Improvement | `improve` | `improve/ENG-99-optimize-query` |
| (none/other) | `fix` | `fix/ENG-100-misc-issue` |

### Git Repository Rules

**Only these paths can have git operations:**

- `sites/client-core` (Frontend (Ionic)) - base: `development`

- `sites/api.praisecharts.com` (Backend (API)) - base: `development`


**NEVER run git operations in the root folder.**

### Human-Only Completion

**AI NEVER marks issues as `Done`**
- AI moves status to `Code Review` after git push
- ONLY human approval or authorized `/envoy` users can move to `Done`
- After `/swarm` completes, run `/swarm create pr` for PR instructions

---

## Workflow Stages

### Stage 1: Issue Analysis & Branch Validation

**Human invokes swarm:**
```
/swarm ENG-42
```

**AI immediately:**
1. **Reads Linear issue** and ALL existing comments
2. **Checks for existing /recon plan** - Read Linear comments and spec files
3. **Validates branch status** (see Branch Validation section above)
4. **Assigns issue to the user** who initiated
5. **Updates status to `Dev In Progress`**
6. **Posts starting comment:**

```markdown
## Implementation Started

**Command:** `/swarm`
**Status:** Dev In Progress
**Branch:** `[branch-name]`

If a `/recon` plan exists on this issue, follow it closely (link the comment/date). If no plan exists, proceed with implementation and document assumptions in a comment.

### Implementation in Progress:
- [ ] Backend changes
- [ ] Frontend changes  
- [ ] Tests
- [ ] Git add/commit/push

ETA: [estimate based on complexity]
```

---

### Stage 2: Check for Existing Plan

Before implementation, AI checks:

1. **Read Linear comments** for `/recon` plan
2. **Check `/specs/` directory** for spec file

**If plan exists:**
- Post acknowledgment: "Found existing plan from /recon. Using it as implementation guide."
- Use plan for technical approach, files, and testing strategy

**If no plan exists:**
- Proceed with implementation
- AI determines approach from issue description

---

### Stage 3: Implementation

**AI delegates to fleets:**

```
@backend: "Implement backend for ENG-42 on branch [branch-name]: [tasks]"
@frontend: "Implement frontend for ENG-42 on branch [branch-name]: [tasks]"
@tester: "Write and run tests for ENG-42: [test cases]"
```

**Implementation rules:**
- Follow existing codebase patterns
- Run tests to verify changes work
- Fix any test failures before proceeding
- Maximum 5 retry attempts before escalating

---

### Stage 4: Git Operations (Automatic)

**After implementation is complete and tests pass:**

1. **Verify correct branch**
2. **Run git operations in each affected repo:**

```bash
cd {repo_path}
git add .
git commit -m "{branch_type}(ENG-XX): [brief description]"
git push origin [branch-name]
```

**Commit message format:**
```
{branch_type}(ENG-XX): Brief description of changes

- Detail 1
- Detail 2

Implements: ENG-XX
```

**Example:**
```
fix(ENG-42): Fix login error on mobile devices

- Add null check for user session
- Update error handling in auth service

Implements: ENG-42
```

3. **Update Linear status to `Code Review`**
4. **Assign to `Isaiah Dahl`**

---

### Stage 5: Post Completion & PR Instructions

**After git push, AI posts completion comment:**

```markdown
## [OK] Implementation Complete

**Branch:** `[branch-name]`
**Status:** Code Review
**Assigned to:** @Isaiah Dahl for review

### Changes Made
- [x] Backend: [summary]
- [x] Frontend: [summary]
- [x] Tests: All passing

### Test Results
```
just test api {test_filter}
[OK] All tests passing
```

### Files Changed
**{repo_1_name}:**
- Created: X files
- Modified: Y files

**{repo_2_name}:**
- Created: X files
- Modified: Y files

---

## Testing Guide

### Backend Tests (Automated)
```bash
just test api {test_filter}
```

### Frontend Testing (Manual)
1. [Step 1 - navigation instruction]
2. [Step 2 - action to take]
3. [Step 3 - expected result]

**[OK] Success looks like:**
- [Expected behavior 1]
- [Expected behavior 2]

---

## Next Steps

1. **Review the code** - Check the changes on branch `[branch-name]`
2. **Create PR** - Run `/swarm create pr` for instructions
3. **After PR merged** - Notify the team (authorized users will run `/envoy` if applicable)

---

## Create PR Instructions

Run `/swarm create pr` for detailed instructions, or use the quick guide below.

_Ready for review by @Isaiah Dahl_
```

---

## /swarm create pr Command

When user runs `/swarm create pr`, AI provides platform-specific instructions:

### BitBucket (Default)

```markdown
## Create Pull Request - BitBucket

**Workspace:** 

### Frontend PR (if changed)
1. Go to: https://bitbucket.org///pull-requests/new
2. Source branch: `[feature-branch]`
3. Destination branch: `development`
4. Title: `{branch_type}(ENG-XX): [description]`
5. Description: Copy from Linear issue or use completion comment

### Backend PR (if changed)
1. Go to: https://bitbucket.org///pull-requests/new
2. Source branch: `[feature-branch]`
3. Destination branch: `development`
4. Title: `{branch_type}(ENG-XX): [description]`
5. Description: Copy from Linear issue or use completion comment

### PR Description Template
```
## Summary
[Brief description of changes]

## Linear Issue
ENG-XX: [title]
https://linear.app/praisecharts/issue/ENG-XX

## Changes
- [Change 1]
- [Change 2]

## Testing
- [ ] Tests pass locally
- [ ] Manual testing completed
```

### After PR Created
- Link PR in Linear issue comment
- Assign reviewer: Isaiah Dahl
- Once approved and merged, notify the team for completion
```

### GitHub (Alternative)

```markdown
## Create Pull Request - GitHub

### Using gh CLI (Recommended)
```bash
cd {repo_path}
gh pr create --title "{branch_type}(ENG-XX): [description]" --body "..."
```

### Manual (Web UI)
1. Go to repository on GitHub
2. Click "Compare & pull request" for your branch
3. Fill in title and description
4. Request review from Isaiah Dahl
```

---

## CLI Commands

| Command | What It Does |
|---------|--------------|
| `/swarm ENG-42` | Implement issue, git add/commit/push |
| `/swarm ENG-42` (after changes requested) | Continue implementation, push updates |
| `/swarm create pr` | Show PR creation instructions |
| `AI test ENG-42` | Run tests and post results |

---

## Status Management

| Action | Status |
|--------|--------|
| /swarm starts | Dev In Progress |
| Git push complete | Code Review |
| Human approval or authorized /envoy | Done |

---

## Integration with /recon

When `/swarm` runs on an issue that has a `/recon` plan:

1. **Detect existing plan** from Linear comments or spec files
2. **Post acknowledgment:** "Found existing plan from /recon. Using it."
3. **Follow the plan** for:
   - Technical approach
   - File structure
   - Testing strategy
   - Implementation sequence

---

## Integration with /envoy

When `/swarm` completes on a dev issue from `/envoy`:

1. **Implementation complete** -> Status: Code Review
2. **PR created and reviewed**
3. **Authorized users run `/envoy ENG-XXX`** to:
   - Mark dev issue as Done
   - Update support issue to Stage Review
   - Post testing instructions for support team

**Note:** `/envoy` is restricted to authorized users only. After completing `/swarm`, run `/swarm create pr` and notify the team. An authorized user will handle the `/envoy` completion step.

See `LINEAR_ENVOY_TEMPLATE.md` for complete workflow and authorized users list.

---

## AI Agent Behavior Rules

### Branch Validation Rules (BLOCKING)

1. **ALWAYS check current branch before implementation**
2. **ALWAYS confirm with user if on unexpected branch**
3. **NEVER implement on protected branches** (['development', 'stage', 'master', 'main'])
4. **ALWAYS create feature branch if on base branch**
5. **ALWAYS use correct branch naming format**

### Git Operation Rules

1. **ALWAYS run git operations ONLY in allowed repos** ([{'path': 'sites/client-core', 'name': 'Frontend (Ionic)', 'base_branch': 'development'}, {'path': 'sites/api.praisecharts.com', 'name': 'Backend (API)', 'base_branch': 'development'}])
2. **NEVER run git operations in root folder**
3. **ALWAYS use proper commit message format**
4. **ALWAYS push to feature branch, never to protected branches**

### Implementation Rules

1. **ALWAYS check for existing /recon plan first**
2. **ALWAYS run tests before git push**
3. **ALWAYS fix test failures (max 5 attempts)**
4. **ALWAYS post completion comment with Testing Guide**
5. **ALWAYS update status to Code Review after push**
6. **ALWAYS assign to Isaiah Dahl after push**
7. **NEVER mark issue as Done** - only human or authorized /envoy users can do this

### Status Update Rules

1. **Start:** Dev In Progress
2. **After git push:** Code Review
3. **NEVER:** Done (human/envoy only)

---

## Error Handling

| Error | Action |
|-------|--------|
| On protected branch | Create feature branch automatically |
| On unexpected branch | STOP and ask user to confirm |
| Tests failing | Fix and retry (max 5 times) |
| Git push fails | Report error, suggest resolution |
| No /recon plan | Proceed without plan (acceptable) |

---

## Comparison: /swarm vs /recon

| Aspect | /swarm | /recon |
|--------|--------|--------|
| Planning | Uses existing plan | Creates plan |
| Implementation | Yes | Never |
| Git Operations | Yes (automatic) | Never |
| Status After | Code Review | Ready For Dev |
| Use when | Ready to implement | Need plan first |

---

_This workflow enables fast implementation with automatic git operations and clear PR guidance._
