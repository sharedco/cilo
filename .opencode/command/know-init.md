---
description: Initialize QMD collections for the /know workflow (idempotent; safe to re-run)
agent: build
subtask: true
---

This command **initializes and validates** the QMD-backed knowledge discovery layer used by `/know` and `/linear`.

Design goal: this should be safe to run repeatedly.
- Prefer **additive** changes.
- Do not break/replace existing QMD collections.
- When something already exists and looks correct, **leave it alone**.

## Requirements

- QMD MCP server must be configured as `qmd` in `.opencode/opencode.json`.
- This workflow uses MCP for validation.

## What this command MUST do

1) Ensure required directories exist (global plane)
2) Ensure `.gitignore` contains required ignore rules
3) Ensure QMD MCP server is configured
4) Ensure QMD collections exist (create them automatically via CLI if missing)
5) Add QMD contexts (optional but recommended)
6) Verify via MCP (`qmd_status`) that collections exist

If any step cannot be completed automatically, stop and ask for **specific human intervention**.

---

## Step 0 — Ensure directories exist

Create (if missing):

- `knowledge/`
- `knowledge/canon/`
- `knowledge/research/`
- `knowledge/drafts/`
- `knowledge/cache/linear/`

Also ensure local evergreen why directory convention exists in repo (do not create container-specific directories):

- `apps/**/Rationale/` (do not mkdir wildcards)

You MUST create these directories using shell commands:

```sh
mkdir -p knowledge/canon knowledge/research knowledge/drafts knowledge/cache/linear
```

## Step 0.2 — Ensure canon entrypoint exists

Verify `knowledge/canon/business-rules.md` exists.

If missing, you MUST create it with:
- a short description that this is the fast-start entrypoint
- links to existing high-trust local evergreen why docs under `apps/**/Rationale/` when relevant
- `Last Updated: YYYY-MM-DD`

## Step 0.5 — Ensure `.gitignore` has required lane ignores

Verify `.gitignore` includes (at minimum):

```
**/knowledge/drafts/*
**/Knowledge/drafts/*
**/knowledge/cache/*
**/Knowledge/cache/*
```

If missing, you MUST add them automatically.

Idempotency rule:
- Never add duplicate lines.
- Append missing lines exactly once.

## Step 0.6 — Ensure QMD MCP is configured

Verify `.opencode/opencode.json` contains an MCP server named `qmd`:

```json
"qmd": {
  "type": "local",
  "command": ["qmd", "mcp"],
  "enabled": true
}
```

If missing, you MUST add it automatically.

## Step 1 — Check QMD index status (via MCP)

Use the MCP tool:
- `qmd_status`

Confirm collections exist that index:

1) `knowledge/canon/**` (+ `apps/**/Rationale/**`)
2) `knowledge/research/**`

Notes:
- Depending on environment, QMD collections may be named either friendly (`canon`/`research`) or absolute paths (e.g. `/var/deployment/pc/knowledge/canon`).
- Treat either as valid.

Record the collection names you will use:

- `CANON_COLLECTION`: prefer `canon` if it exists, otherwise use the collection whose name/path clearly corresponds to `knowledge/canon`.
- `RESEARCH_COLLECTION`: prefer `research` if it exists, otherwise use the collection whose name/path clearly corresponds to `knowledge/research`.

If both exist, continue (do not stop): still ensure contexts and print a success summary at the end.

## Step 2 — If collections are missing

QMD MCP does not expose collection creation tools. However, since this is an **init** command, you MUST create missing collections automatically using the QMD CLI.

### 2.1 Preflight: verify `qmd` binary exists

Run:

```sh
command -v qmd
```

If missing, stop and ask the user to install QMD (one-time). Suggest:

```sh
bun install -g https://github.com/tobi/qmd
```

(If Bun isn't installed, ask the user how they want QMD installed in their environment.)

If `qmd` exists but `qmd_status` fails, ask the user to restart the OpenCode backend so the MCP server can start cleanly.

### 2.2 Create collections (automatic)

From repo root, run the commands below.

#### Create `canon`

Attempt a single collection that covers both global canon and local evergreen why:

```sh
qmd collection add . --name canon --mask "{knowledge/canon/**/*.md,apps/**/Rationale/**/*.md,apps/**/rationale/**/*.md}"
```

Idempotency rule:
- Only run the command above if **no existing collection** already indexes `knowledge/canon/**`.
  Use `qmd_status` first. Treat an existing collection as valid if you see either:
  - a collection named `canon`, OR
  - a collection whose name/path ends with (or contains) `/knowledge/canon`.
- If a collection named `canon` already exists, do **not** attempt to overwrite it.

If that fails due to glob/mask support, stop and ask for intervention with the exact error message.

If it fails due to an existing collection name conflict, you MUST:
 - list existing collections via MCP (`qmd_status`)
- ask the user whether to (a) delete/rename conflicting collections manually, or (b) adjust the required collection names in the workflow

Do NOT create a second canon collection under a different name unless the user explicitly requests it (duplicate indexing can cause confusion).

#### Create `research`

```sh
qmd collection add knowledge/research --name research --mask "**/*.md"
```

Idempotency rule:
- Only run this if **no existing collection** already indexes `knowledge/research/**`.
  Use `qmd_status` first. Treat an existing collection as valid if you see either:
  - a collection named `research`, OR
  - a collection whose name/path ends with (or contains) `/knowledge/research`.

### 2.3 Add context labels (recommended)

Run:

```sh
qmd context add "qmd://$CANON_COLLECTION" "HIGH TRUST: business rules/invariants/why. Prefer for truth."
qmd context add "qmd://$RESEARCH_COLLECTION" "Ticket-backed research notes (time-bound, cite code paths)."
```

(If you’re not using shell variables, substitute the literal collection names you recorded in Step 1.)

Idempotency rule:
- If context entries already exist (or the add command fails with an "already exists"-type error), treat that as success.

If context commands fail for other reasons, do not stop the init (contexts are helpful but non-blocking). Report the failure.

### 2.4 Optional: embeddings

If the user wants high-quality fuzzy search (vector + rerank), QMD will need embeddings/models.

This can involve large local model downloads.

Ask for explicit confirmation before running:

```sh
qmd embed
```

## Step 3 — Re-check

Re-run `qmd_status` and confirm collections are present.

If they are present, report success and stop.

Success output MUST include:
- Which canon collection name to use (e.g. `canon` or an absolute-path collection)
- Which research collection name to use
- Reminder: local evergreen why is `apps/**/Rationale/**` (high-trust "why" docs treated like code)
