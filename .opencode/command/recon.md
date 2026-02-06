# Linear Workflow - Reconnaissance & Planning (READ-ONLY)

> **Configuration Required:** This template uses values from `LINEAR_WORKFLOW_CONFIG.yaml`.
> Variables are replaced from `commands.yaml` at generation time.

This document defines a **planning-only** workflow where AI agents research and create detailed implementation plans WITHOUT making any code changes. Implementation requires a separate command (`/swarm`).

## Philosophy

**Research -> Plan -> Post -> STOP**

- AI reads issue, launches scouts, and creates comprehensive plan
- **READ-ONLY** - No file modifications, no commits, no implementation
- Plan posted to Linear as comment -> **STOP**
- Spec file creation is OPTIONAL (only if requested)
- `/swarm` can use the plan later for implementation
- Human can revise plan by commenting in Linear or running `/recon` again

**Key Principle:** POST TO LINEAR -> STOP -> Wait for human

---

## Status Flow

### Normal Recon Flow
```
Needs Research
    ↓ (/recon starts research)
Research In Progress
    ↓ (research complete)
Ready For Plan
    ↓ (/recon starts planning)
Plan In Progress
    ↓ (plan complete)
Ready For Dev
    ↓ (ready for /swarm)
```

### Re-Recon Flow (when user wants to update research or plan)
```
Ready For Dev or later
    ↓ (user runs /recon again)
Research In Review OR Plan In Review
    ↓ (AI decides based on what needs updating)
    ↓ (if research is stale -> research_in_review -> research_in_progress)
    ↓ (if only plan needs update -> plan_in_review -> plan_in_progress)
Ready For Dev
```

### Re-Recon Decision Logic

When `/recon` is run on an issue that's already past `Ready For Dev`:

| Current Status | Action |
|----------------|--------|
| Dev In Progress or later | Skip research, go to Plan In Review (plan revision only) |
| Ready For Dev | AI analyzes comments - if research is stale -> Research In Review, else -> Plan In Review |

**AI determines "research is stale" when:**
- User comments mention new requirements or constraints
- User explicitly asks to "re-research" or "investigate again"
- Significant time has passed and codebase may have changed
- User mentions the original research missed something

**Default (when unclear):** Go to Plan In Review (faster, keeps existing research)

---

## CRITICAL RULES

### No Implementation

**AI using `/recon` NEVER writes code or modifies files**
- Only reads files, researches patterns, analyzes architecture
- PRIMARY OUTPUT: Posts plan to Linear as comment
- OPTIONAL OUTPUT: Creates spec file in `/specs/` (only if requested)
- Implementation requires separate command: `/swarm`
- This ensures human review of approach before any code changes

### No Git Operations

**AI using `/recon` NEVER performs git operations:**
- [X] NO `git add`
- [X] NO `git commit`
- [X] NO `git push`
- [X] NO `git branch`
- [X] NO `git switch` or `git checkout`

/recon is READ-ONLY for code. Only operations allowed:
- [OK] Read files (investigation)
- [OK] Launch scouts in parallel
- [OK] Write to Linear (comments with plan) - **PRIMARY OUTPUT**
- [OK] Create spec files in `/specs/` directory - **ONLY if explicitly requested OR after Linear comment posted**

**PRIMARY WORKFLOW:**
1. Scouts investigate (READ-ONLY)
2. Oracle synthesizes findings
3. **POST PLAN TO LINEAR -> STOP**
4. (Optional) Create spec file if requested

**NEVER create spec files BEFORE posting to Linear.**
**NEVER create documentation files (*.md) outside `/specs/` directory.**

---

## Workflow Stages

### Stage 1: Issue Analysis & Status Update

**Human invokes recon:**
```
/recon ENG-42
```

**AI immediately:**
1. **Reads Linear issue** and ALL existing comments
2. **Checks current status** to determine if this is initial recon or re-recon
3. **Updates status:**
   - If `Needs Research` or `Backlog` or `Triage` -> `Research In Progress`
   - If `Ready For Dev` or later -> `Plan In Review` (default) or `Research In Review` (if research is stale)
4. **Assigns issue to the user** who initiated reconnaissance
5. **Identifies ambiguities** and prepares clarifying questions
6. **Posts initial comment** to Linear:

```markdown
## Reconnaissance Started

**Command:** `/recon`  
**Mode:** READ-ONLY (Planning only, no implementation)
**Status:** Research In Progress

I'm analyzing this issue and will create a detailed implementation plan.

### Clarifying Questions

Before I dive into research, I have a few quick questions:

1. **[Question about ambiguity from issue description]**
2. **[Question about scope or requirements]**
3. **[Question about integration or dependencies]**

Please clarify these points so my plan is accurate.

### Research Areas (In Parallel):
- [ ] Codebase patterns and existing implementations
- [ ] Architecture and file structure
- [ ] Dependencies and libraries
- [ ] Testing requirements
- [ ] Edge cases and considerations

I'll post the full plan after clarification and research complete. ETA: 10-15 minutes
```

---

### Stage 2: Parallel Scout Dispatch

AI launches appropriate scouts based on issue domain:

| Issue Type | Primary | Supporting |
|------------|---------|------------|
| Frontend bug/feature | @scout-fe | @scout-be |
| Backend bug/feature | @scout-be | @scout-fe |
| Full-stack/unclear | @scout-full | both |
| Library/external issue | @scout-lib | relevant layer |

**Scout missions (READ-ONLY):**
```
Investigate ENG-XX: [Title]
[Issue description]

Your role: PRIMARY/SUPPORTING scout for [domain]
Goal: Find root cause, relevant files, patterns, existing implementations

READ-ONLY MODE: No file modifications allowed

Return:
- Files involved (max 10 most relevant)
- Complexity assessment (trivial/medium/complex)
- Existing patterns we can follow
- Dependencies and libraries used
- Edge cases to consider
- Security/performance considerations
```
**Critical:** All scouts operate in READ-ONLY mode. They can:
- [OK] Read files (mcp_read)
- [OK] Search code (mcp_grep, mcp_ast_grep)
- [OK] Analyze structure (mcp_glob, mcp_lsp_*)
- [X] Write files
- [X] Modify code
- [X] Create commits
- [X] Run implementation

**After research completes:** Update status to `Ready For Plan`

---

### Stage 3: Oracle Synthesizes Plan

**Update status to:** `Plan In Progress`

After scouts report, @scout-oracle:

1. **Synthesizes all findings** into comprehensive understanding
2. **Creates detailed implementation plan** (see template below)
3. **Prepares plan for Linear** (does NOT create files yet)

---

### Stage 4: Post Plan and STOP (CRITICAL CHECKPOINT)

**Oracle MUST:**
1. [OK] Post complete plan to Linear as comment
2. [OK] Update status to `Ready For Dev`
3. [OK] Wait for confirmation plan was posted
4. [!] **STOP HERE** - Do NOT proceed to spec file creation
5. [!] **STOP HERE** - Do NOT create any other files

**What happens next:**
- Human reviews plan in Linear
- If human wants spec file, they can:
  - Ask explicitly: "Create spec file for this plan"
  - Run `/swarm ENG-XX` which will read plan from Linear
  
**Spec file is OPTIONAL, not automatic.**

---

## Implementation Plan Template

AI uses this structure for all recon plans:

```markdown
## Implementation Plan - Ready for Development

**Command Used:** `/recon`  
**Planning Agent:** @scout-oracle  
**Status:** Ready For Dev (awaiting implementation via /swarm)

### Executive Summary
[One paragraph: what we're building, how, effort estimate, risk level]

**Complexity:** Trivial / Medium / High  
**Estimated Effort:** X-Y hours  
**Risk Level:** Low / Medium / High  

---

## Scout Findings Summary

### Frontend Investigation (@scout-fe)
**Files Identified:**
- `path/to/component.ts` - [Description]
- `path/to/service.ts` - [Description]

**Existing Patterns:**
- [Pattern 1 we can follow]
- [Pattern 2 to avoid]

**Complexity Assessment:** [Trivial/Medium/High]

### Backend Investigation (@scout-be)
**Files Identified:**
- `path/to/file` - [Description]
- `path/to/file` - [Description]

**Existing Patterns:**
- [Pattern 1 we can follow]
- [Pattern 2 to avoid]

**Complexity Assessment:** [Trivial/Medium/High]

### External Research (@scout-lib)
[Library docs, examples, best practices - if applicable]

**Dependencies:**
- [Library 1] - [Why/how we'll use it]
- [Library 2] - [Why/how we'll use it]

---

## Technical Approach

### Backend Changes

#### 1. [Component Name]
**File:** `path/to/file`

**What it does:** [Clear description]

**Implementation steps:**
1. [Step 1]
2. [Step 2]
3. [Step 3]

**Risk:** [None/Low/Medium/High] - [Explanation]

### Frontend Changes

#### 1. [Component/Service Name]
**File:** `path/to/file`

**What it does:** [Clear description]

**Implementation steps:**
1. [Step 1]
2. [Step 2]
3. [Step 3]

**Risk:** [None/Low/Medium/High] - [Explanation]

### Files to Create (New)
- `NewFile` (~XX lines) - [Purpose]
- `NewComponent` (~XX lines) - [Purpose]

### Files to Modify (Existing)
- `ExistingFile` (+XX/-YY lines) - [Changes needed]
- `ExistingComponent` (+XX/-YY lines) - [Changes needed]

---

## Testing Strategy

### Backend Tests
**Test File:** `path/to/TestFile`

#### Test Cases Required:
- [ ] **test_happy_path_scenario** - [Description]
- [ ] **test_permissions** - [Description]
- [ ] **test_edge_case_1** - [Description]

### Frontend Tests
**Test File:** `path/to/test.spec`

#### Test Cases Required:
- [ ] **should render component** - [Description]
- [ ] **should handle user interaction** - [Description]
- [ ] **should display error states** - [Description]

---

## Edge Cases & Considerations

### Security
- [OK] [Security check 1 - status]
- [!] [Security concern 2 - needs attention]

### Performance
- [OK] [Performance consideration 1]
- [!] [Potential bottleneck - mitigation strategy]

---

## Questions for Review

### Critical Decisions Needed:
1. **[Question 1]** - [Options A vs B, recommendation]
2. **[Question 2]** - [Trade-offs to consider]

### Assumptions Made:
- [Assumption 1] - [Confirm or correct]
- [Assumption 2] - [Confirm or correct]

---

## Implementation Sequence

**Recommended order:**
1. **Backend first** (API must be ready for frontend)
   - [ ] Core logic
   - [ ] API endpoints
   - [ ] Tests
2. **Frontend second** (consumes backend API)
   - [ ] Services
   - [ ] Components
   - [ ] Tests
3. **Integration testing** (end-to-end)

**Branch strategy:**
- Create branch: `{branch_type}/{issue_id}-{issue_title_kebab}`
- All commits go to this branch
- PR when complete

---

## [OK] Plan Complete - Ready for Implementation

**Next Steps:**
1. **Review this plan** - Add comments in Linear with questions/feedback
2. **Revise if needed** - Run `/recon ENG-XX` again to update plan
3. **Implement** - Run `/swarm ENG-XX` to implement this plan automatically

**How to implement this plan:**
```bash
/swarm ENG-XX
```

**How to revise this plan:**
- Comment feedback directly in Linear
- Run `/recon ENG-XX` again to generate updated plan

---

_Posted by @scout-oracle | Recon Mode | [timestamp]_
```

---

## Stage 5: Spec File Creation (OPTIONAL - Only if Requested)

**When to create spec file:**
- [OK] Human explicitly requests it: "Create spec file"
- [OK] During `/swarm` execution (implementation phase needs reference)
- [X] NEVER automatically during `/recon`

AI creates spec file at `/specs/ENG-XX_description.md`:

**Filename format:**
```
/specs/ENG-XX_short_description.md
```

**Examples:**
- `/specs/ENG-42_export_timesheets_pdf.md`
- `/specs/ENG-73_fix_modal_scroll_ios.md`

---

## CLI Commands

| Command | What It Does |
|---------|--------------|
| `/recon ENG-42` | Research and create plan (READ-ONLY, no implementation) |
| `/recon [description]` | Create issue + research + plan (no implementation) |
| `/recon ENG-42` (again) | Update existing plan (goes to review status first) |

---

## Status Management

### First-Time Recon

| Phase | Status |
|-------|--------|
| Start | Research In Progress |
| Research done | Ready For Plan |
| Planning | Plan In Progress |
| Complete | Ready For Dev |

### Re-Recon (updating existing plan)

| Scenario | Status Flow |
|----------|-------------|
| Only plan needs update | Plan In Review -> Plan In Progress -> Ready For Dev |
| Research is stale | Research In Review -> Research In Progress -> Ready For Plan -> Plan In Progress -> Ready For Dev |

---

## Comparison: /recon vs /swarm

| Aspect | /recon | /swarm |
|--------|--------|--------|
| **Planning** | Always (READ-ONLY) | Uses /recon plan if exists |
| **Implementation** | Never | Yes |
| **File Changes** | No | Yes |
| **Git Operations** | No | Yes (add/commit/push) |
| **Use when** | Want plan first, implement later | Ready to implement |
| **Output** | Plan in Linear | Completed code + PR |
| **Status after** | Ready For Dev | Code Review |

---

## Integration with /swarm

When `/swarm` runs on an issue that has a `/recon` plan:

1. **Check for existing plan:**
   - Read Linear comments for `/recon` plan
   - Read `/specs/ENG-XX_description.md` if exists

2. **If plan exists:**
   - Use the plan as implementation guide
   - Skip creating new plan (unless explicitly requested)
   - Reference plan in progress updates
   - Follow recommended fleet and sequence

3. **If no plan exists:**
   - Proceed with implementation
   - Create minimal plan on-the-fly

---

## Integration with /envoy

When `/recon` is used on a dev issue created via `/envoy`:

1. **Support reports issue** in SE team
2. **Authorized user runs `/envoy SE-XXX`** to create ENG issue
3. **ENG issue starts at** `Needs Research`
4. **Developer runs `/recon ENG-XXX`** to research and plan
5. **Status progresses:** research_needed -> research_in_progress -> ready_for_plan -> plan_in_progress -> ready_for_dev
6. **Plan is posted** to Linear for review
7. **Developer runs `/swarm ENG-XXX`** to implement using the plan

See `LINEAR_ENVOY_TEMPLATE.md` for complete `/envoy` workflow documentation.

---

## Critical Rules

1. [!] **NEVER write files during /recon** - READ-ONLY mode (except Linear comment)
2. [!] **NEVER create commits during /recon** - No git operations
3. [!] **NEVER implement during /recon** - Plan only
4. [!] **ALWAYS read existing Linear comments** - Check for previous plans
5. [!] **ALWAYS post plan to Linear FIRST** - Before any file creation
6. [!] **STOP after posting to Linear** - Wait for human input
7. [!] **Create spec file ONLY if requested** - Not automatic
8. [!] **ALWAYS update status through the progression** - research_in_progress -> ready_for_plan -> plan_in_progress -> ready_for_dev
9. [!] **ALWAYS assign to initiating user** - When recon starts
10. [!] **Handle re-recon appropriately** - Go to review status, then back through progression

---

_This workflow enables strategic planning without the commitment of immediate implementation._
