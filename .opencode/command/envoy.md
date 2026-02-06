# Linear Workflow - Envoy Triage (Intelligent Issue Classification)

> **Configuration Required:** This template uses values from `commands.yaml`.
> Variables use `{MISSING:path}` syntax. Replace with your config values or use a template processor.

> **Project Restriction:** This command is ONLY available for projects with `features.envoy_template: "triage"` in their config.
> Currently enabled for: PraiseCharts

This document defines the `/envoy` workflow (triage mode) for intelligent issue classification, scope evaluation, and team assignment using AI agents and Git Knowledge.

## Philosophy

**Analyze -> Contextualize -> Classify -> Route**

- AI agents analyze Linear issues to understand scope and requirements
- Git Knowledge provides codebase context (related projects, file ownership, patterns)
- Agent evaluates whether the issue is truly an "issue" or a "project"
- Intelligent routing based on complexity, domain, and team ownership
- Reduces manual triage burden while maintaining accuracy through confidence scoring

---

## Authorization

### Who Can Run /envoy

| Role | Can Run /envoy | Notes |
|------|-----------------|-------|
| Senior Developer | Yes | Full triage capabilities |
| Developer | Yes | Full triage capabilities |
| Non-Developer | No | Issues auto-enter TRIAGE status for agent processing |

### Non-Developer Flow

Non-developers create issues that automatically receive **"Triage"** status in Linear. These flow into the system for analysis and get routed appropriately without manual `/envoy` invocation.

---

## Teams

| Team | Key | URL | Purpose |
|------|-----|-----|---------|
| **Engineering** | ENG | https://linear.app/praisecharts/team/ENG/all | Developer team issues |

---

## Status Flow

```
Triage (or Backlog)
    | (/envoy starts)
Triage In Progress
    | (analysis complete)
    |
    +---> [Issue] --> Needs Research --> Normal /recon flow
    |
    +---> [Project] --> Convert to Linear Project --> Create sub-issues
    |
    +---> [Needs Research] --> Needs Research --> Assign researcher
    |
    +---> [Suggest] --> Post suggestion --> Wait for human decision
```

---

## Workflow Stages

### Stage 1: Issue Retrieval & Initial Analysis

**Human invokes triage:**
```
/envoy ENG-123
```

**AI immediately:**

1. **Verify Project Has Envoy Triage Mode**
   - Check `features.envoy_template` equals `"triage"` in config
   - If not triage mode, error and halt:
   ```
   [X] /envoy triage mode is not enabled for this project.
   This command requires envoy_template: "triage" in the project config.
   ```

2. **Fetch Complete Issue Details from Linear**
   - Issue title and description
   - Current status
   - Existing labels, assignees, metadata
   - Comments and discussion history
   - Related issues and projects

3. **Perform Semantic Analysis**
   - Extract key concepts, technologies, and feature areas
   - Identify the type of work (bug, feature, technical debt, etc.)
   - Assess scope and complexity signals from description
   - Note any ambiguities or missing information

4. **Post Initial Comment to Linear:**

```markdown
## Triage Started

**Command:** `/envoy`
**Status:** Triage In Progress

Analyzing issue for classification and routing...

### Initial Assessment
- **Type signals detected:** [bug/feature/improvement/unclear]
- **Scope signals:** [small/medium/large/unclear]
- **Domain hints:** [frontend/backend/fullstack/unclear]

Gathering codebase context now...
```

---

### Stage 2: Context Gathering from Git Knowledge

**AI launches parallel exploration agents:**

```
@scout-full: "Analyze codebase for issue context:
  - Keywords: [extracted keywords from issue]
  - Technology areas: [identified tech]
  - Feature domains: [identified domains]
  
  Return:
  - Related projects/modules
  - File ownership patterns
  - Similar past implementations
  - Architectural constraints"

@scout-lib: "Search for:
  - Documentation about [feature area]
  - Historical context from related PRs
  - External references if applicable"
```

**Git Knowledge Returns:**
- List of related projects/repositories
- Projects with similar functionality
- Codebases that would be affected by this issue
- Historical context about related work
- File paths, modules, or components relevant to the issue
- Code ownership information (CODEOWNERS, commit patterns)

---

### Stage 3: Scope Evaluation

**AI evaluates whether this is truly an "issue" or a "project":**

| Indicator Type | Issue Signals | Project Signals |
|----------------|---------------|-----------------|
| **Scope** | Single bug fix, small feature, isolated change | Multi-component work, architectural changes |
| **Components** | Affects one area/module | Affects multiple repos/modules |
| **Dependencies** | Minimal new dependencies | Requires research phase |
| **Effort** | Hours to 1-2 days | Days to weeks |
| **Risk** | Low to medium | Medium to high |
| **Unknowns** | Few or none | Many unknowns to resolve |

**Complexity Score Calculation:**

```
Score = (
  component_count * 2 +
  dependency_risk * 3 +
  unknown_count * 2 +
  estimated_files * 0.5 +
  cross_team_impact * 3
)

Issue: Score < 10
Project: Score >= 10
Research Needed: Unknown count >= 3 OR ambiguity is high
```

---

### Stage 4: Decision Logic

Based on analysis, AI makes one of four decisions:

#### Outcome A: Suggest Classification (Default for Medium Confidence)

When confidence is **M (Medium)**, return suggestion to developer for approval.

**AI posts to Linear:**

```markdown
## Triage Suggestion - Awaiting Decision

**Classification:** `ISSUE` | Confidence: `M` (Medium)

### Analysis Summary
- **Complexity Score:** 7/20
- **Primary Domain:** Backend
- **Related Projects:** [project1, project2]
- **Estimated Effort:** 4-8 hours

### Related Projects Identified
| Project | Relevance | Impact |
|---------|-----------|--------|
| `auth-service` | High | Direct changes needed |
| `user-api` | Medium | May need updates |

### Recommendation
This appears to be a **standard issue** that can proceed through normal development workflow.

### Suggested Actions
1. **Accept as Issue** - Reply "accept" to move to Needs Research
2. **Convert to Project** - Reply "project" to create a Linear project
3. **Override** - Reply "override: [your classification]" to manually set

**Waiting for your decision...**
```

#### Outcome B: Auto-Classify as Issue (High Confidence)

When confidence is **H (High)** and classification is **Issue**:

**AI MUST execute these MCP calls:**

```python
# 1. Update status, priority, assignee, and labels
mcp_linear_update_issue(
    id="ENG-123",
    state="Needs Research",
    priority=0,  # Based on complexity score (0 for trivial, no urgency)
    assigneeId="b7fe4e5f-feb0-4eee-95da-bf5b9887ae4e",
    labels=["frontend", "Bug"]  # Domain label + type label
)

# 2. Check for project association (if relevant project found)
# mcp_linear_add_to_project(
#     issueId="ENG-123",
#     projectId="project-uuid-if-found"
# )

# 3. Post triage notes comment
mcp_linear_create_comment(
    issueId="ENG-123",
    body="## [OK] Triage Complete - Classified as Issue\n\n..."
)
```

**Comment to post:**

```markdown
## [OK] Triage Complete - Classified as Issue

**Classification:** `ISSUE` | Confidence: `H` (High)

### Analysis Summary
- **Complexity Score:** 4/20
- **Primary Domain:** Frontend
- **Related Projects:** [project1]
- **Estimated Effort:** 2-4 hours

### Actions Taken
- [x] Status → Needs Research
- [x] Priority → No Priority (complexity: 2/20, trivial)
- [x] Assigned to: Isaiah Dahl (Frontend specialist)
- [x] Labels: `frontend`, `Bug`
- [x] Project: [Added to Project Name] OR [No matching project]

### Related Files (from Git Knowledge)
- `src/components/UserCard.tsx` - Primary component to modify
- `src/services/user.service.ts` - May need API updates

### Next Steps
Developer can now run `/recon ENG-123` to research and plan.
```

#### Outcome C: Convert to Project (High Confidence)

When confidence is **H (High)** and classification is **Project**:

```markdown
## [!] Triage Complete - Requires Project

**Classification:** `PROJECT` | Confidence: `H` (High)

### Analysis Summary
- **Complexity Score:** 15/20
- **Primary Domain:** Fullstack
- **Related Projects:** [project1, project2, project3]
- **Estimated Effort:** 2-3 weeks

### Why This Is a Project
- Affects 5+ components across frontend and backend
- Requires architectural decisions before implementation
- Has 4 identified unknowns that need research
- Cross-team coordination required

### Suggested Project Structure
```
Project: [Issue Title]
|-- Epic: Backend API Changes
|   |-- Task: Design API schema
|   |-- Task: Implement endpoints
|   |-- Task: Write tests
|-- Epic: Frontend Implementation
|   |-- Task: UI design review
|   |-- Task: Component development
|   |-- Task: Integration testing
|-- Milestone: MVP Release
```

### Actions Needed
1. **Create Project** - AI will create Linear Project with structure above
2. **Override** - Reply "keep as issue" to proceed as single issue

**Awaiting your decision...**
```

#### Outcome D: Ready for Research (Insufficient Information)

When unknowns are high or scope cannot be determined:

**AI MUST execute these MCP calls:**

```python
# 1. Update status, priority, assignee, and labels
mcp_linear_update_issue(
    id="ENG-123",
    state="Needs Research",
    priority=3,  # Medium - needs attention
    assigneeId="b7fe4e5f-feb0-4eee-95da-bf5b9887ae4e",
    labels=["frontend", "Needs-Research"]  # Or "backend" depending on domain
)

# 2. Post triage notes comment
mcp_linear_create_comment(
    issueId="ENG-123",
    body="## [!] Triage Paused - Research Required\n\n..."
)
```

**Comment to post:**

```markdown
## [!] Triage Paused - Research Required

**Classification:** `UNKNOWN` | Confidence: `L` (Low)

### Analysis Summary
- **Complexity Score:** Cannot determine
- **Primary Domain:** Unclear
- **Related Projects:** Potentially many
- **Estimated Effort:** Unknown

### Why Research Is Needed
- [ ] Issue description lacks technical specifics
- [ ] Multiple possible interpretations exist
- [ ] Affected systems cannot be determined from description
- [ ] No similar past work found in codebase

### Questions That Need Answers
1. **[Question about scope]** - Which users/scenarios are affected?
2. **[Question about implementation]** - Should this modify existing behavior or add new?
3. **[Question about dependencies]** - Are there external service dependencies?

### Actions Taken
- [x] Status -> Needs Research
- [x] Priority -> Medium (needs investigation)
- [x] Assigned to: Isaiah Dahl (for investigation)
- [x] Labels: `Fullstack`, `Needs-Research`

### Next Steps
Assigned developer should:
1. Investigate the questions above
2. Update the issue description with findings
3. Run `/envoy ENG-123` again with more context

OR reply to this comment with clarifications.
```

---

### Stage 5: Linear Updates (MANDATORY)

Based on the decision, AI **MUST** update Linear using the MCP tools. This is not optional documentation - these are required actions.

#### Required MCP Tool Calls

**AI MUST execute ALL applicable updates using these MCP tools:**

```
1. mcp_linear_update_issue - Update status, priority, assignee
2. mcp_linear_add_labels - Add domain and complexity labels
3. mcp_linear_add_to_project - Add to related project (if applicable)
4. mcp_linear_create_comment - Post triage notes
```

#### Status Updates (REQUIRED)

| Decision | Status Change | MCP Call |
|----------|---------------|----------|
| Auto-classify as Issue | `Triage` -> `Needs Research` | `mcp_linear_update_issue(id, state="Needs Research")` |
| Suggest Classification | `Triage` -> `Triage In Progress` (awaiting) | `mcp_linear_update_issue(id, state="Triage In Progress")` |
| Convert to Project | Issue converted to Project in Linear | `mcp_linear_update_issue(id, state="Triage In Progress")` then await confirmation |
| Ready for Research | `Triage` -> `Needs Research` | `mcp_linear_update_issue(id, state="Needs Research")` |

#### Priority Assignment (REQUIRED)

AI MUST set priority based on complexity and urgency signals:

| Complexity Score | Urgency Signals | Priority | MCP Call |
|-----------------|-----------------|----------|----------|
| < 5 | None | No Priority | (leave as-is, don't set) |
| < 5 | Has urgency keywords | Medium | `mcp_linear_update_issue(id, priority=3)` |
| 5-10 | None | Low | `mcp_linear_update_issue(id, priority=4)` |
| 5-10 | Has urgency keywords | Medium | `mcp_linear_update_issue(id, priority=3)` |
| 10-20 | Any | High | `mcp_linear_update_issue(id, priority=2)` |
| > 20 | Any | Urgent | `mcp_linear_update_issue(id, priority=1)` |

**Urgency keywords:** "ASAP", "urgent", "critical", "blocking", "production", "outage", "broken", "can't", "emergency"

#### Team/Person Assignment (REQUIRED)

AI MUST assign based on domain classification:

| Domain Classification | Assignee | MCP Call |
|----------------------|----------|----------|
| Frontend | Isaiah Dahl | `mcp_linear_update_issue(id, assigneeId="b7fe4e5f-feb0-4eee-95da-bf5b9887ae4e")` |
| Backend | Isaiah Dahl | `mcp_linear_update_issue(id, assigneeId="b7fe4e5f-feb0-4eee-95da-bf5b9887ae4e")` |
| Fullstack (default) | Isaiah Dahl | `mcp_linear_update_issue(id, assigneeId="b7fe4e5f-feb0-4eee-95da-bf5b9887ae4e")` |
| Research needed | Isaiah Dahl | `mcp_linear_update_issue(id, assigneeId="b7fe4e5f-feb0-4eee-95da-bf5b9887ae4e")` |

#### Label Management (REQUIRED)

AI MUST add appropriate labels via the `labels` parameter in `mcp_linear_update_issue`:

**Domain labels (pick ONE):**
- `frontend` - Primarily frontend work
- `backend` - Primarily backend work  
- `Fullstack` - Requires both frontend and backend (if it exists, otherwise pick frontend or backend)

**Type labels (add if applicable):**
- `Bug` - When issue type is bug
- `Feature` - When issue type is feature
- `Improvement` - When issue type is improvement

**Special labels (add if applicable):**
- `Needs-Research` - When routing to research
- `Project-Candidate` - When suggesting project conversion

**Note:** Complexity labels (Trivial, Medium, Complex) do not exist in the PraiseCharts workspace. Do NOT add complexity labels. Instead, document complexity in the triage comment.

**MCP Call Example:**
```
mcp_linear_add_labels(issueId, labelNames=["frontend", "Bug"])
```

#### Project Association (CONDITIONAL)

AI MUST check for related Linear projects and add the issue if a match is found:

**Step 1: Fetch active projects**
```
mcp_linear_list_projects(teamId="{MISSING:teams.dev.id}")
```

**Step 2: Analyze for relevance**
Compare issue keywords, domain, and feature area against project names and descriptions:
- Match by feature area (e.g., "authentication", "checkout", "mobile app")
- Match by epic/initiative keywords
- Match by component names mentioned in issue

**Step 3: Add to project if confidence > 70%**
```
mcp_linear_add_to_project(issueId, projectId)
```

**Step 4: Document in triage notes**
If added to project, include in the triage comment:
```markdown
### Project Association
- [x] Added to project: **[Project Name]**
- Relevance: [brief explanation of why this issue belongs to this project]
```

If no project match found:
```markdown
### Project Association
- [ ] No matching project found
```

**Project matching signals:**
| Signal | Weight |
|--------|--------|
| Exact feature area match | High |
| Component name in project title | High |
| Related technology stack | Medium |
| Similar past issues in project | Medium |
| Keyword overlap with project description | Low |

---

## Triage Notes Comment Template

After triage completes, AI posts a structured comment documenting the analysis:

```markdown
## Triage Notes

**Triaged by:** AI Agent
**Timestamp:** [ISO timestamp]
**Confidence:** H | M | L

---

### Issue Analysis

**Extracted Concepts:**
- Technology: [list]
- Feature Area: [list]
- Issue Type: [bug/feature/improvement]

**Semantic Signals:**
- Scope indicators: [small/medium/large]
- Risk indicators: [low/medium/high]
- Ambiguity level: [low/medium/high]

---

### Git Knowledge Context

**Related Projects:**
{FOR_EACH related_projects}
- `{name}` - {relevance} relevance - {impact}
{END_FOR_EACH}

**File Ownership:**
- Primary owner: [team/person based on git blame patterns]
- Last modified: [date range]
- Change frequency: [low/medium/high]

**Historical Context:**
- Similar issues: [list of related past issues if found]
- Relevant PRs: [list of related PRs if found]

---

### Classification Rationale

**Complexity Score Breakdown:**
| Factor | Score | Reasoning |
|--------|-------|-----------|
| Component count | X/10 | [explanation] |
| Dependency risk | X/15 | [explanation] |
| Unknown count | X/10 | [explanation] |
| File estimate | X/10 | [explanation] |
| Cross-team impact | X/15 | [explanation] |
| **Total** | **X/60** | |

**Decision:** [ISSUE/PROJECT/NEEDS-RESEARCH]

**Confidence Reasoning:**
[Why H/M/L confidence was assigned]

---

### Actions Taken (Linear Updates)

| Action | Status | Details |
|--------|--------|---------|
| Status Update | ✅ | [Previous] → [New Status] |
| Priority Set | ✅ | [Priority Level] (0=None, 1=Urgent, 2=High, 3=Medium, 4=Low) |
| Assignee Set | ✅ | [Developer Name] ([Domain] specialist) |
| Labels Added | ✅ | [List of labels added] |
| Project Added | ✅/❌ | [Project Name] OR "No matching project" |

---

### Recommended Next Steps

1. [Step 1]
2. [Step 2]
3. [Step 3]

---

_This triage was performed automatically. For questions or to override, comment on this issue._
```

---

## Required MCP Tool Calls Summary

**This section provides a quick reference for ALL required Linear MCP calls during triage.**

### Minimum Required Calls (EVERY triage)

```python
# 1. ALWAYS: Update issue status, priority, assignee, AND labels (all in one call)
mcp_linear_update_issue(
    id="ENG-XXX",
    state="[target status]",        # REQUIRED
    priority=[0-4],                 # REQUIRED based on complexity (0=No Priority, 1=Urgent, 4=Low)
    assigneeId="[developer-uuid]",  # REQUIRED based on domain
    labels=["[frontend|backend]", "[Bug|Feature|Improvement]"]  # REQUIRED
)

# 2. ALWAYS: Post triage comment
mcp_linear_create_comment(
    issueId="ENG-XXX",
    body="[Full triage notes markdown]"
)
```

### Conditional Calls

```python
# 4. IF relevant project found: Add to project
projects = mcp_linear_list_projects(teamId="[team-uuid]")
# AI analyzes for relevance...
if relevant_project_found:
    mcp_linear_add_to_project(
        issueId="ENG-XXX",
        projectId="[project-uuid]"
    )
```

### Priority Values (PraiseCharts Workspace)

**Note:** PraiseCharts Linear workspace uses this scale:
- 0 = No Priority (leave unset for trivial non-urgent issues)
- 1 = Urgent (most critical)
- 2 = High
- 3 = Medium
- 4 = Low (least critical)

| Priority | Value | When to Use |
|----------|-------|-------------|
| Urgent | 1 | Production issues, outages, blocking (score > 20) |
| High | 2 | Complex issues (score 10-20) |
| Medium | 3 | Moderate complexity OR has urgency keywords (score 5-10 with urgency) |
| Low | 4 | Simple issues (score 5-10), no urgency |
| No Priority | 0 | Trivial issues (score < 5), no urgency - don't set or set to 0 |

### Label Requirements

**Every triaged issue MUST have:**
1. **One domain label:** `frontend` or `backend` (Fullstack label does not exist in workspace)
2. **Type label (if determinable):** `Bug`, `Feature`, OR `Improvement`
3. **Special labels (if applicable):** `Needs-Research`, `Project-Candidate`

**Note:** Complexity labels (Trivial, Medium, Complex) do not exist in PraiseCharts workspace. Document complexity in triage comment instead.

---

## CLI Commands

| Command | What It Does |
|---------|--------------|
| `/envoy ENG-123` | Run triage analysis on an issue |
| `/envoy ENG-123 --force` | Re-run triage even if already triaged |
| `/envoy queue` | Show issues in TRIAGE status awaiting processing |

---

## Integration with Other Workflows

### After Triage -> /recon

When triage classifies as **Issue** and moves to `Needs Research`:
- Developer can run `/recon ENG-123` to create implementation plan
- Triage notes provide context for research phase
- Related projects inform scout dispatch

### After Triage -> Project Creation

When triage recommends **Project**:
- AI suggests project structure with epics and tasks
- Human confirms project creation
- Each task becomes individual issue that can be triaged

### Integration with /envoy

For support issues escalated via `/envoy`:
- Newly created dev issues can be triaged for classification
- Triage helps determine if support issue requires project-level work
- Classification informs response timeline to support team

---

## AI Agent Behavior Rules

### When Running /envoy

**MANDATORY Linear Updates (in order):**

1. **ALWAYS update status FIRST** - Use `mcp_linear_update_issue` to change status immediately
2. **ALWAYS set priority** - Based on complexity score and urgency signals
3. **ALWAYS assign to developer** - Based on domain classification (Frontend/Backend/Fullstack)
4. **ALWAYS add labels** - Domain label + Complexity label + Type label
5. **ALWAYS check for project association** - Use `mcp_linear_list_projects` and add if relevant
6. **ALWAYS post triage notes** - Use `mcp_linear_create_comment` with full analysis

**Research & Analysis Rules:**

7. **ALWAYS check envoy triage mode is enabled** - Verify `features.envoy_template: "triage"`
8. **ALWAYS read all issue comments** - Context matters for classification
9. **ALWAYS gather codebase context** - Launch scouts for Git Knowledge
10. **ALWAYS calculate complexity score** - Don't guess classification

**Restrictions:**

11. **NEVER auto-convert to project without confirmation** - Suggest, don't force
12. **NEVER skip confidence assessment** - Be transparent about certainty
13. **NEVER implement during triage** - This is analysis only
14. **NEVER modify files** - Triage is READ-ONLY like /recon
15. **NEVER skip Linear updates** - Status, priority, assignee, and labels are MANDATORY

### Confidence Level Guidelines

| Confidence | When to Use | Action |
|------------|-------------|--------|
| **H (High)** | Clear scope, single domain, low ambiguity | Auto-classify |
| **M (Medium)** | Some ambiguity, multiple possible approaches | Suggest and wait |
| **L (Low)** | High ambiguity, many unknowns, unclear scope | Route to research |

### Error Handling

| Error Case | Message |
|------------|---------|
| Triage not enabled | `[X] /envoy is not enabled for this project.` |
| Issue not found | `[X] Issue ENG-XXX not found.` |
| Already triaged | `[!] Issue already triaged. Use --force to re-triage.` |
| Invalid issue format | `[X] Invalid issue ID format. Use ENG-XXX.` |
| Analysis failed | `[X] Triage analysis failed: [reason]. Please try again.` |

---

## Configuration Reference

### Required Config Values

```yaml
features:
  envoy_enabled: true
  envoy_template: "triage"
  
statuses:
  triage: "Triage"
  triage_in_progress: "Triage In Progress"
  research_needed: "Research Needed"

envoy_triage:
  auto_classify_threshold: 0.8
  project_complexity_threshold: 10
  max_related_projects: 5
```

### Labels Used by Triage

| Label | Purpose | Exists? |
|-------|---------|---------|
| `frontend` | Primarily frontend work | ✅ Yes |
| `backend` | Primarily backend work | ✅ Yes |
| `Bug` | Bug fix | ✅ Yes |
| `Feature` | New feature | ✅ Yes |
| `Improvement` | Enhancement to existing feature | ✅ Yes |
| `Needs-Research` | Issue requires investigation before development | ❌ No (create if needed) |
| `Project-Candidate` | Issue may need to be converted to project | ❌ No (create if needed) |
| `Trivial` | Very simple, low-effort issue | ❌ No (document in comment instead) |
| `Complex` | Multi-faceted, higher-effort issue | ❌ No (document in comment instead) |
| `Fullstack` | Requires both frontend and backend | ❌ No (use frontend or backend) |

---

## Flow Diagram

```
                         +-------------------+
                         |  /envoy ENG-123  |
                         +--------+----------+
                                  |
                    +-------------v--------------+
                    |  Stage 1: Issue Retrieval  |
                    |  - Fetch from Linear       |
                    |  - Semantic analysis       |
                    +-------------+--------------+
                                  |
                    +-------------v--------------+
                    |  Stage 2: Git Knowledge    |
                    |  - Launch scouts           |
                    |  - Gather codebase context |
                    |  - Find related projects   |
                    +-------------+--------------+
                                  |
                    +-------------v--------------+
                    |  Stage 3: Scope Evaluation |
                    |  - Calculate complexity    |
                    |  - Assess unknowns         |
                    |  - Determine classification|
                    +-------------+--------------+
                                  |
              +-------------------+-------------------+
              |                   |                   |
    +---------v--------+ +--------v--------+ +-------v--------+
    |  HIGH Confidence | | MED Confidence  | | LOW Confidence |
    |  Auto-classify   | | Suggest & Wait  | | Route Research |
    +--------+---------+ +--------+--------+ +-------+--------+
             |                    |                   |
    +--------v---------+         |          +--------v--------+
    |   ISSUE          |<--------+          |  NEEDS RESEARCH |
    |   -> Research    |                    |  -> Investigate |
    |      Needed      |                    +--------+--------+
    +--------+---------+                             |
             |                                       |
    +--------v---------+                    +--------v--------+
    |   PROJECT        |                    |  Re-triage      |
    |   -> Create      |                    |  after clarity  |
    |      Project     |                    +-----------------+
    +------------------+
```

---

## Local Cache Strategy

For performance, triage uses local caching:

**Cached Data:**
- Recent issue metadata (5 minute TTL)
- Project mappings (1 hour TTL)
- Team configurations (1 day TTL)
- Git ownership patterns (1 day TTL)

**Cache Invalidation:**
- Manual: `/envoy --clear-cache`
- Automatic: On config file changes
- Automatic: On Git Knowledge index update

---

_This workflow enables intelligent issue triage that reduces manual overhead while maintaining accuracy through confidence scoring and human oversight for ambiguous cases._
