# /scouts - Planning First Workflow (Thorough)

> **Configuration Required:** This template uses values from `commands.yaml`.
> Variables are replaced from `commands.yaml` at generation time.

For complex architecture or risky refactors. Plan first, get approval, then implement.

## Philosophy

**Research -> Plan -> Approve -> Implement**

- AI researches issue thoroughly using scout agents
- Creates detailed plan posted to Linear
- **WAITS for human approval** before implementing
- After approval, implements following the plan
- Combines /recon (planning) with /swarm (implementation) in one workflow

---

## Teams

| Team | Key | URL | Purpose |
|------|-----|-----|---------|
| **Engineering** | ENG | https://linear.app/praisecharts/team/ENG/all | Developer team issues |

---

## Scout Fleet (READ-ONLY Investigation)

| Agent | Role | Context | Cost |
|-------|------|---------|------|
| @scout-fe | Frontend files (READ-ONLY) | 1M | FREE |
| @scout-be | Backend files (READ-ONLY) | 200K | ¢ |
| @scout-full | Full-stack trace (READ-ONLY) | 2M | $$ |
| @scout-lib | External docs (READ-ONLY) | 1M | FREE |
| @scout-oracle | Synthesize + Plan + Implement | 200K | $$$ |

---

## Status Flow

```
Needs Research (or Backlog/Triage)
    ↓ (/scouts starts)
Research In Progress
    ↓ (research complete)
Ready For Plan
    ↓ (planning)
Plan In Progress
    ↓ (plan complete, posted to Linear)
Ready For Dev
    ↓ **WAIT FOR HUMAN APPROVAL**
    ↓ (human approves: "approved", "lgtm", "go ahead")
Dev In Progress
    ↓ (implementation + git push complete)
Code Review
    ↓ (human or authorized /envoy)
Done
```

---

## Workflow Stages

### Stage 1: Issue Analysis

**Human invokes:**
```
/scouts ENG-42
```

**AI immediately:**
1. Reads Linear issue and ALL existing comments
2. Assigns issue to the user who initiated
3. Updates status to `Research In Progress`
4. Posts initial comment to Linear

### Stage 2: Parallel Scout Dispatch

AI launches scouts based on issue domain:

| Issue Type | Primary | Supporting |
|------------|---------|------------|
| Frontend bug/feature | @scout-fe | @scout-be |
| Backend bug/feature | @scout-be | @scout-fe |
| Full-stack/unclear | @scout-full | both |
| Library/external issue | @scout-lib | relevant layer |

**All scouts operate in READ-ONLY mode:**
- [OK] Read files (mcp_read)
- [OK] Search code (mcp_grep, mcp_ast_grep)
- [OK] Analyze structure (mcp_glob, mcp_lsp_*)
- [X] Write files
- [X] Modify code
- [X] Create commits

### Stage 3: Oracle Synthesizes Plan

After scouts report, @scout-oracle:
1. Synthesizes all findings
2. Creates detailed implementation plan
3. Posts plan to Linear as comment
4. Updates status to `Ready For Dev`

### Stage 4: WAIT FOR HUMAN APPROVAL (CRITICAL)

**AI MUST STOP and wait for explicit approval:**

```markdown
## [!] PLAN READY - Awaiting Approval

The plan has been posted above. Please review and respond:

- **"approved"** / **"lgtm"** / **"go ahead"** → I will implement
- **"changes needed: [feedback]"** → I will revise the plan
- **"cancel"** → I will stop

Waiting for your response...
```

**AI does NOT proceed until human responds with approval.**

### Stage 5: Implementation (After Approval)

Once approved, AI:
1. Validates branch (creates feature branch if needed)
2. Updates status to `Dev In Progress`
3. Implements following the plan
4. Runs tests
5. Git add/commit/push
6. Updates status to `Code Review`
7. Assigns to Isaiah Dahl

---

## Branch Rules

### Branch Validation (BLOCKING)

Before implementation, AI validates current branch:

| Current Branch | Action |
|----------------|--------|
| `development` or protected | Create feature branch automatically |
| Unexpected branch | **STOP and ask user to confirm** |
| Correct feature branch | Continue |

### Branch Naming

| Issue Label | Branch Prefix | Example |
|-------------|---------------|---------|
| Bug | `fix/` | `fix/ENG-42-login-error` |
| Feature | `feat/` | `feat/ENG-73-export-pdf` |
| Improvement | `improve/` | `improve/ENG-99-optimize-query` |

### Git Repositories

**Only commit to these paths:**

- `sites/client-core` (Frontend (Ionic))

- `sites/api.praisecharts.com` (Backend (API))


**NEVER run git operations in root folder.**

---

## CLI Commands

| Command | What It Does |
|---------|--------------|
| `/scouts ENG-42` | Research, plan, wait for approval, then implement |
| `/scouts ENG-42 plan-only` | Same as /recon (plan only, no implementation) |

---

## vs /recon and /swarm

| Aspect | /scouts | /recon | /swarm |
|--------|---------|--------|--------|
| Planning | Yes (with scouts) | Yes (READ-ONLY) | Uses existing plan |
| Human Approval | Required before implementation | N/A (plan only) | Not required |
| Implementation | Yes (after approval) | Never | Yes |
| Git Operations | Yes (after approval) | Never | Yes (automatic) |
| Final Status | Code Review | Ready For Dev | Code Review |
| Use when | Complex/risky changes | Want plan review | Ready to implement |

---

## Key Rules

1. **ALWAYS use scout agents for research** - Parallel investigation
2. **ALWAYS post plan to Linear** - Before asking for approval
3. **ALWAYS wait for human approval** - Never implement without explicit approval
4. **ALWAYS validate branch before implementation**
5. **ALWAYS run tests before git push**
6. **ALWAYS update status through the progression**
7. **NEVER mark issue as Done** - Only human or authorized /envoy users can

---

## Delegation Rules

When working from a Linear Document:

| Task Type | Agent | What They Do |
|-----------|-------|--------------|
| Backend (API, database, Actions, Tasks) | @backend | Implement, update document when done |
| Frontend (components, NGXS, UI) | @frontend | Implement, update document when done |
| Tests | @tester | Write/run tests, report results |
| Investigation/debugging | @hunter | Research, report findings |

---

## Completion Rules

**A task is NOT complete until:**
- [ ] ALL implementation steps are done
- [ ] Tests pass: `just test api`
- [ ] Build passes (if frontend changes)
- [ ] Git add/commit/push completed
- [ ] Status updated to Code Review
- [ ] PR reviewer assigned: Isaiah Dahl

---

_This workflow enables thorough planning with human oversight before implementation._
