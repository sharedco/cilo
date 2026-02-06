---
description: Work one Linear ticket end-to-end (canon-first) and write Know artifacts
agent: plan
subtask: true
---

Load the `know` skill and follow it strictly. **All policy (lanes, placement, promotion, artifact semantics) is defined in the skill—do not duplicate it here.**

---

## Workspace Context

### Teams
| Prefix | Domain | Team ID |
|--------|--------|---------|
| API | Backend / Laravel | `<API_TEAM_ID>` |
| WWW | Frontend web | `<WWW_TEAM_ID>` |
| DES | Design | `<DES_TEAM_ID>` |
| SER | Infra / servers | `<SER_TEAM_ID>` |
| DAT | Data | `<DAT_TEAM_ID>` |

### Workflow States (team-specific; status progression)
**Linear workflow states are team-specific.** The same state name (e.g. "Research Needed") will have a different ID per team.

**Default rule:** resolve workflow states by **name** for the ticket's team. Only use IDs as an optional fast-path/fallback.

#### Conceptual progression (SOP)
Tickets move through these conceptual states:

| State (name) | Meaning |
|--------------|---------|
| Triage | New ticket, needs review |
| Spec Needed | Problem/solution unclear |
| Research Needed | Requires investigation before plan |
| Research In Progress | Active research underway |
| Research In Review | Research findings under review |
| Ready for Plan | Research done, needs implementation plan |
| Plan In Progress | Writing implementation plan |
| Plan In Review | Plan under discussion |
| Ready for Dev | Plan approved, ready to implement |
| In Dev | Active development |
| Code Review | PR submitted |
| Done | Completed |

#### Optional: per-team state ID map (placeholders)
Use these only if you want to hardcode IDs for speed. Keep them per-team.

**API**
- Triage: `<API_TRIAGE_STATE_ID>`
- Spec Needed: `<API_SPEC_NEEDED_STATE_ID>`
- Research Needed: `<API_RESEARCH_NEEDED_STATE_ID>`
- Research In Progress: `<API_RESEARCH_IN_PROGRESS_STATE_ID>`
- Research In Review: `<API_RESEARCH_IN_REVIEW_STATE_ID>`
- Ready for Plan: `<API_READY_FOR_PLAN_STATE_ID>`
- Plan In Progress: `<API_PLAN_IN_PROGRESS_STATE_ID>`
- Plan In Review: `<API_PLAN_IN_REVIEW_STATE_ID>`
- Ready for Dev: `<API_READY_FOR_DEV_STATE_ID>`
- In Dev: `<API_IN_DEV_STATE_ID>`
- Code Review: `<API_CODE_REVIEW_STATE_ID>`
- Done: `<API_DONE_STATE_ID>`

**WWW**
- Triage: `<WWW_TRIAGE_STATE_ID>`
- Spec Needed: `<WWW_SPEC_NEEDED_STATE_ID>`
- Research Needed: `<WWW_RESEARCH_NEEDED_STATE_ID>`
- Research In Progress: `<WWW_RESEARCH_IN_PROGRESS_STATE_ID>`
- Research In Review: `<WWW_RESEARCH_IN_REVIEW_STATE_ID>`
- Ready for Plan: `<WWW_READY_FOR_PLAN_STATE_ID>`
- Plan In Progress: `<WWW_PLAN_IN_PROGRESS_STATE_ID>`
- Plan In Review: `<WWW_PLAN_IN_REVIEW_STATE_ID>`
- Ready for Dev: `<WWW_READY_FOR_DEV_STATE_ID>`
- In Dev: `<WWW_IN_DEV_STATE_ID>`
- Code Review: `<WWW_CODE_REVIEW_STATE_ID>`
- Done: `<WWW_DONE_STATE_ID>`

**DES**
- Triage: `<DES_TRIAGE_STATE_ID>`
- Spec Needed: `<DES_SPEC_NEEDED_STATE_ID>`
- Research Needed: `<DES_RESEARCH_NEEDED_STATE_ID>`
- Research In Progress: `<DES_RESEARCH_IN_PROGRESS_STATE_ID>`
- Research In Review: `<DES_RESEARCH_IN_REVIEW_STATE_ID>`
- Ready for Plan: `<DES_READY_FOR_PLAN_STATE_ID>`
- Plan In Progress: `<DES_PLAN_IN_PROGRESS_STATE_ID>`
- Plan In Review: `<DES_PLAN_IN_REVIEW_STATE_ID>`
- Ready for Dev: `<DES_READY_FOR_DEV_STATE_ID>`
- In Dev: `<DES_IN_DEV_STATE_ID>`
- Code Review: `<DES_CODE_REVIEW_STATE_ID>`
- Done: `<DES_DONE_STATE_ID>`

**SER**
- Triage: `<SER_TRIAGE_STATE_ID>`
- Spec Needed: `<SER_SPEC_NEEDED_STATE_ID>`
- Research Needed: `<SER_RESEARCH_NEEDED_STATE_ID>`
- Research In Progress: `<SER_RESEARCH_IN_PROGRESS_STATE_ID>`
- Research In Review: `<SER_RESEARCH_IN_REVIEW_STATE_ID>`
- Ready for Plan: `<SER_READY_FOR_PLAN_STATE_ID>`
- Plan In Progress: `<SER_PLAN_IN_PROGRESS_STATE_ID>`
- Plan In Review: `<SER_PLAN_IN_REVIEW_STATE_ID>`
- Ready for Dev: `<SER_READY_FOR_DEV_STATE_ID>`
- In Dev: `<SER_IN_DEV_STATE_ID>`
- Code Review: `<SER_CODE_REVIEW_STATE_ID>`
- Done: `<SER_DONE_STATE_ID>`

**DAT**
- Triage: `<DAT_TRIAGE_STATE_ID>`
- Spec Needed: `<DAT_SPEC_NEEDED_STATE_ID>`
- Research Needed: `<DAT_RESEARCH_NEEDED_STATE_ID>`
- Research In Progress: `<DAT_RESEARCH_IN_PROGRESS_STATE_ID>`
- Research In Review: `<DAT_RESEARCH_IN_REVIEW_STATE_ID>`
- Ready for Plan: `<DAT_READY_FOR_PLAN_STATE_ID>`
- Plan In Progress: `<DAT_PLAN_IN_PROGRESS_STATE_ID>`
- Plan In Review: `<DAT_PLAN_IN_REVIEW_STATE_ID>`
- Ready for Dev: `<DAT_READY_FOR_DEV_STATE_ID>`
- In Dev: `<DAT_IN_DEV_STATE_ID>`
- Code Review: `<DAT_CODE_REVIEW_STATE_ID>`
- Done: `<DAT_DONE_STATE_ID>`

### Labels (coordination; Option A)
**Labels are not a substitute for team ownership.** Use team (API/WWW/DES/SER/DAT) for the primary queue/owner.

Because "this touches another team's surface area" usually implies multi-team coordination anyway, we prefer explicit coordination labels:

#### Recommended baseline labels (placeholders)
| Label | Meaning |
|-------|---------|
| `cross-team` | More than one team needs to participate (general) |
| `needs-api` | Needs API/backend input/review/work |
| `needs-www` | Needs frontend/web input/review/work |
| `needs-des` | Needs design input/review/work |
| `needs-ser` | Needs infra/server input/review/work |
| `needs-dat` | Needs data/DB/analytics input/review/work |

**Rule:** When you apply `needs-*`, also leave a comment explicitly stating *what you need* from that team.

#### Optional: label ID map (placeholders)
Labels can be workspace-scoped or team-scoped in Linear. If team-scoped, keep per-team IDs.

**Workspace-scoped label IDs (if applicable)**
- cross-team: `<LBL_CROSS_TEAM_ID>`
- needs-api: `<LBL_NEEDS_API_ID>`
- needs-www: `<LBL_NEEDS_WWW_ID>`
- needs-des: `<LBL_NEEDS_DES_ID>`
- needs-ser: `<LBL_NEEDS_SER_ID>`
- needs-dat: `<LBL_NEEDS_DAT_ID>`

**Per-team label IDs (if applicable)**
- API needs-www: `<API_LBL_NEEDS_WWW_ID>`
- WWW needs-api: `<WWW_LBL_NEEDS_API_ID>`
- (add more as needed)

### Priority vs Estimate (two different axes)
**Priority** expresses urgency/impact (what to do first).

**Estimate** expresses size/effort/uncertainty (how big the work is). Estimates are configured per team in Linear (some teams may use t-shirt, some Fibonacci, etc.).

#### Priority Scale
| Value | Meaning | Use for |
|-------|---------|---------|
| 1 | Urgent | Critical blockers, security, data loss |
| 2 | High | Important features with deadlines, major bugs |
| 3 | Medium | Standard implementation tasks (default) |
| 4 | Low | Nice-to-haves, minor improvements |

#### Estimate scale (placeholders)
Document what your teams use. If teams differ, keep a per-team mapping.

| Estimate | Meaning |
|----------|---------|
| XS | `<XS_MEANING>` |
| S | `<S_MEANING>` |
| M | `<M_MEANING>` |
| L | `<L_MEANING>` |
| XL | `<XL_MEANING>` |

### User IDs
| Name | ID |
|------|----|
| `<USER_1_NAME>` | `<USER_1_ID>` |
| `<USER_2_NAME>` | `<USER_2_ID>` |

---

## Safety / Prerequisites

Before any action:

1. **Verify Linear MCP tools exist** (any tool containing `linear` + `issue` primitives, e.g., `mcp__linear__*` OR `linear_*`). If not available:
   - Stop and tell the user: "Linear MCP not available. Run `/mcp` to enable, then retry."

2. **Verify required workflow states exist (team-specific)** by resolving names for the relevant team:
   - "Research Needed", "Research In Progress", "Research In Review", "Triage"
   - If any missing: list available states for that team and ask user for mapping.

3. **If available, use QMD for knowledge discovery** (recommended):
   - QMD MCP is the discovery layer for canon + research.
   - Do not fall back to the QMD CLI inside agent workflows.

---

## Inputs

Command arguments: `$ARGUMENTS`

Origin remote (for GitHub link construction):
!`git remote get-url origin`

---

## ONE command, two modes

### Mode A — Research one ticket (default)
Trigger when:
- `$ARGUMENTS` contains a ticket identifier (e.g., `API-123`, `WWW-456`), OR
- `$ARGUMENTS` is empty, OR
- `$ARGUMENTS` is a Linear URL (extract ticket key)

### Mode B — Create a Linear ticket from a Know doc
Trigger when `$ARGUMENTS` starts with (case-insensitive):
- `create <path>`

Example:
- `/linear create knowledge/drafts/subscriptions/subscription-pause-resume.md`

If neither mode matches, ask ONE clarifying question.

---

# Mode A — Research one ticket (canon-first + research)

## A1) Select ticket

**If ticket ID/URL provided:**
1. Fetch that issue via Linear MCP.
2. Continue to A2.

**If no ticket ID provided (auto-select):**
1. Resolve team IDs for: API, WWW, DES, SER, DAT.
2. Search/list issues in "Research Needed" state across all teams (use `search_issues` if available, else `list_issues`), ordered by priority, limit ≤10 total.
3. From combined set, select the single highest-priority issue that is also **small** (estimate indicates XS or S, or equivalent for that team).
4. If none are XS/S: stop and tell user "No XS/S tickets in Research Needed."

**Hard rule:** Work exactly ONE ticket.

## A2) Validate ticket is researchable

Fetch the ticket. A ticket is **researchable** only if it has ALL of:
- [ ] A clear **Problem to solve** OR a reproducible symptom
- [ ] A **target area** (component, container, endpoint, or user flow)
- [ ] A **success condition** (what "better" means)
- [ ] At least one **pointer** (file path, route, log string, or owner)

**If any missing:**
1. Add a comment listing exactly which items are missing.
2. Ensure ticket is in "Research Needed" (or move it there).
3. Stop.

**If all present:**
1. Move ticket to "Research In Progress".
2. Continue to A3.

## A3) Cache the ticket locally (gitignored)

Write a cache file: `knowledge/cache/linear/<TEAM>-<NNNN>.md`

**Required header format:**
```
# <TEAM>-<NNNN> — <title>

- **URL**: <linear url>
- **Team**: <team>
- **State**: <state>
- **Priority**: <priority>
- **Estimate**: <XS/S/M/L/XL or null>
- **Labels**: <comma-separated>
- **Fetched At**: <ISO timestamp>

---

## Description
<body>

## Comments
### <author> — <timestamp>
<comment body>
```

**Rule:** This file is a read-only mirror of Linear. Do not add analysis here.

## A4) Canon-first context + research (be unbiased)

Goal:
- **Canon-first**: identify existing durable rules/decisions that should govern this work.
- Then document how the system works today + constraints.

Steps:
0. Load pinned business rules entrypoint first (fast, deterministic):
   - Use QMD MCP `get` on `knowledge/canon/business-rules.md`
1. Discover governing canon (QMD preferred):
    - If unsure of the canon collection name, use `qmd_status` and choose the collection that indexes `knowledge/canon/**`.
    - Use MCP tool `qmd_query` (collection: canon-collection, limit: 10, minScore ~0.45).
    - If QMD MCP is not available, instruct the user to run `/know-init`.
2. Read the top relevant canon docs to understand business rules/invariants.
3. Then inspect codebase to understand the current behavior.
4. If canon is missing/insufficient, consult research:
    - If unsure of the research collection name, use `qmd_status` and choose the collection that indexes `knowledge/research/**`.
    - Use MCP tool `qmd_query` (collection: research-collection, limit: 10, minScore ~0.25).

1. Search codebase for relevant files/patterns.
2. If external research needed, do it.
4. Produce a **committed research note**:

**Path:** `knowledge/research/YYYY-MM-DD-<TEAM>-<NNNN>-<topic>.md`

**Research note required sections:**
- **Summary** (2-3 sentences)
- **Key findings** (3-7 bullets, facts + file references)
- **Constraints / gotchas**
- **Open questions** (if any)
- **References**:
  - Cached ticket path
  - Key code paths (`file:line`)
  - External links (if any)

**Rule:** Prefer facts and file references over ideals.

## A5) Auto-promotion to canon (no user confirmation)

Apply the **canon promotion rules** from the `know` skill.

Summary:
- If ticket contains `NO_KNOW` (or legacy `NO_THOUGHT`): skip canon updates.
- If canon update is warranted:
  1. Use QMD to find existing related canon (preferred), otherwise fall back to grep/glob.
  2. Prefer updating an existing canon doc over creating a new one.
  3. If creating new: apply the skill's routing/placement rules.
  4. Include references: Linear ticket, research note path, key code paths, `Last Updated`.

## A6) Update Linear

Add a comment with this structure:

```
## Research Complete

**Research note**: `knowledge/research/YYYY-MM-DD-<TEAM>-<NNNN>-<topic>.md`
**Canon**: [Updated `path/to/canon.md` | Created `path/to/canon.md` | Skipped (reason)]

### Key Findings
- [Finding 1]
- [Finding 2]
- [Finding 3]

### Open Questions
- [Question if any, else "None"]

### Next Steps
- [Suggested next action]
```

Then move ticket to "Research In Review".

## A7) Output to user

Print:
- Ticket key + title
- Cache path
- Research note path
- Promotion decision + path (if applicable)
- Key findings (top 3)
- Ticket URL

---

# Mode B — Create a Linear ticket from a Know doc

## B1) De-duplicate check

Before creating, search for existing tickets:
- Search Linear for: doc basename, key terms from title/problem
- Search repo for existing `knowledge/cache/linear/*.md` referencing this doc

**If matches found:**
- Show matches and ask: "Update existing ticket, or create new?"
- If user says update: switch to comment/update flow.
- If user says create: continue.

## B2) Read source doc

1. Read the provided file path.
2. Extract: Problem to solve, context, constraints, desired outcomes.

**If "Problem to solve" is missing/unclear:**
- Ask ONE question: "What problem does this solve from a user perspective?"
- Stop and wait for answer.

## B3) Choose team

Infer from doc path:
| Path pattern | Team |
|--------------|------|
| `app/Containers/` | API |
| `resources/`, `*wui*` | WWW |
| `*design*`, `*des*` | DES |
| `*infra*`, `*server*`, `*deploy*` | SER |
| `*data*`, `*migration*`, `*analytics*` | DAT |

If ambiguous: ask user to choose one team.

## B4) Auto-assign labels (coordination)

Apply labels only when coordination is actually needed:
- If the ticket is owned by API but clearly requires frontend work/review → add `needs-www` and `cross-team`
- If the ticket is owned by WWW but clearly requires backend work/review → add `needs-api` and `cross-team`
- If the ticket requires design review → add `needs-des` (and `cross-team` if applicable)
- If the ticket requires infra/server work/review → add `needs-ser` (and `cross-team` if applicable)
- If the ticket requires data/DB/analytics work/review → add `needs-dat` (and `cross-team` if applicable)

**Rule:** When adding `needs-*`, also add a brief comment describing the dependency/request.

## B5) Create issue

Create via MCP with:
- **Title**: action-oriented (verb + object)
- **Description** (required sections):
  ```
  ## Problem to solve
  [User-impact framing]

  ## Context
  [Background, constraints]

  ## Proposed approach
  [If known, else "TBD after research"]

  ## Acceptance criteria
  - [ ] [Testable criterion 1]
  - [ ] [Testable criterion 2]

  ## Risks / unknowns
  - [Risk or unknown]

  ## References
  - Source: `<know doc path>`
  ```
- **teamId**: resolved team
- **stateId**: "Triage" (default)
- **priority**: 3 (Medium) unless user specifies
- **labelIds**: auto-assigned coordination labels (use your label ID map; if labels are team-scoped, pick IDs for the issue's team)

Then print: created ticket URL.

## B6) Optional: update source doc

Ask: "Add ticket reference to the source Know doc?"

If yes, prepend to doc:
```
---
linear_ticket: <URL>
created: <YYYY-MM-DD>
---
```

---

## Comment Quality Guidelines

When adding comments to tickets, focus on **value for future readers**:

**Good comments include:**
- Key insights (the "aha" moment)
- Decisions made and tradeoffs
- Blockers resolved and how
- State changes and what they mean
- Surprises or discoveries

**Avoid:**
- Mechanical lists of changes without context
- Restating what's obvious from diffs
- Generic summaries that don't add value

**Format:** Keep comments concise (~10 lines) unless detail is genuinely needed.

---

## Error Recovery

| Failure | Recovery |
|---------|----------|
| Linear MCP unavailable | Tell user to run `/mcp`, stop |
| State name not found | List available states, ask user for equivalent |
| Ticket not found | Verify key format, check team, ask user |
| Insufficient permissions | Report error, suggest user check Linear access |
| Research note write fails | Print content to user, ask them to save manually |

---

## Non-goals

- Do NOT work multiple tickets at once.
- Do NOT hardcode workflow **state** IDs as a single global map (states are team-specific). Prefer resolve-by-name; if hardcoding, keep **per-team** maps.
- Do NOT assume label IDs are global; labels may be workspace-scoped or team-scoped.
- Do NOT add extra commands for Linear. This is the single entrypoint.
- Do NOT duplicate Know skill policy here.
