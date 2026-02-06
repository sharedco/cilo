---
description: Draft a new gitignored Know artifact (lane-based, canon-first system)
agent: build
---

Load the `know` skill and follow it strictly.

I want to create a new **draft** Know artifact.

Arguments:
- $ARGUMENTS

Rules:
1) Drafts are **gitignored** and live in the `drafts/` lane.
2) Ask ONE clarifying question if scope is ambiguous:
   - GLOBAL (`knowledge/drafts/`)
3) If user provides no path, default to GLOBAL: `knowledge/drafts/`.
4) Propose the exact file path you will create.
6) Create a concise Markdown skeleton that is explicitly a DRAFT and includes placeholders for:
   - Summary
   - Context
   - Hypothesis / Questions
   - Next actions
   - References
   - Last Updated

If the user included a title, use it. Otherwise infer a title from $ARGUMENTS and ask for confirmation.
