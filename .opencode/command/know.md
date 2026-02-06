---
description: Canon-first knowledge workflow powered by QMD (discover, write, and maintain rules + research)
agent: plan
subtask: true
---

Load the `know` skill and follow it strictly.

Command arguments: `$ARGUMENTS`

## Pre-check: Should you even run this?

Before executing `/know`, check:

1. **Did canon-gate already inject relevant hits?** If the injected snippet fully answers the question, you're done—no need to run `/know`.
2. **Is the topic generic (not PraiseCharts-specific)?** If canon-gate said "no relevant canon hits" and the question is about external libraries, general programming, or code location—**skip `/know` entirely**. Canon has nothing to say.

Only proceed with `/know` when:
- You need **more detail** from a doc that was partially matched, OR
- The topic is **org-specific** (ops, workflows, commands, business rules) even if no auto-hits

## Purpose

`/know` is the **workflow entrypoint** (a verb). It does not define policy—policy lives in the skill.

This workflow is designed so agents can:
- **discover** relevant knowledge reliably via QMD
- **treat canon as truth** (high-trust business rules / invariants)
- **capture research** as ticket outcomes
- **avoid duplication** by updating existing canon

## Prerequisite: QMD MCP

This workflow assumes the **QMD MCP server** is connected (configured in `.opencode/opencode.json` as `qmd`).

This workflow also assumes QMD has collections set up:

- `canon` (covers `knowledge/canon/**` + `apps/**/Rationale/**`)
- `research` (covers `knowledge/research/**`)

If collections are missing, ask the user to do one-time QMD setup outside this workflow.

Preferred: run `/know-init`.

## Modes

### Mode A — Find governing rules (default)
Trigger when `$ARGUMENTS` is a natural language query.

Steps:
0) **Skip check**: If canon-gate already injected a relevant snippet that answers the query, return that answer immediately without further QMD calls.
1) Load pinned business rules entrypoint (fast, deterministic):
   - Use QMD MCP `get` on `knowledge/canon/business-rules.md`
2) Identify the canon collection name (if unsure):
   - Use `qmd_status` and choose the collection that indexes `knowledge/canon/**`.
   - Prefer a friendly name like `canon`. Some environments may show an absolute-path collection (e.g. `/var/.../knowledge/canon`).
3) Query canon using `qmd_query` (collection: canon-collection, limit: 10, minScore ~0.45).
4) Retrieve the top 1–3 docs for full context using `qmd_get`.
5) **Stop condition**: If steps 3-4 return no meaningful hits, do NOT retry with different keywords. Canon has nothing to say—proceed with normal reasoning or pivot to code/runtime enumeration.
6) If canon results are weak but topic is clearly org-specific, you may expand into research using `qmd_query` (collection: research-collection, minScore ~0.25). But only one attempt.

If QMD MCP is not available, instruct the user to run `/know-init` (do not fall back to CLI).

Output:
- 1–3 governing canon docs (paths)
- 1–3 supporting research notes (if any)
- a short statement of the rule(s) discovered with citations

### Mode B — Start a draft
Trigger when `$ARGUMENTS` starts with:
- `draft ...`

Delegate to the `know-draft` command behavior:
- create `knowledge/drafts/<topic>.md` (gitignored)

### Mode C — Audit canon for a topic
Trigger when `$ARGUMENTS` starts with:
- `audit ...`

Use QMD to find:
- duplicates (semantic overlap)
- stale docs (missing/old Last Updated)
- conflicting statements

Output:
- suggested merges/updates
- list of docs to touch
