# KEEPTRACK — Agent Workflow

## Purpose
This file defines the workflow any agent must follow when working on this project.
It does NOT contain context itself — that goes in `CURRENT_CONTEXT.md`.

## Workflow

1. **Read PLAN.md** — understand the current phase and big-picture scope.
2. **Read KEEPTRACK.md** (this file) — know the workflow rules.
3. **Read CURRENT_CONTEXT.md** — pick up where the last session left off. Learn from past findings.
4. **DOCUMENTATION.md** — when you discover something worthy of permanent documentation (architecture decisions, setup instructions, API design), add a section to DOCUMENTATION.md and **note it in CURRENT_CONTEXT.md**.
5. **Write before you act** — in CURRENT_CONTEXT.md, state *what you are about to do* and *why* before touching any files or running commands.
6. **Update after you act** — record results, learnings, blockers in CURRENT_CONTEXT.md immediately after.
7. **Update PLAN.md** — keep the phase/step status current.

## Guidelines

- Never duplicate information between CURRENT_CONTEXT.md and DOCUMENTATION.md
  - CURRENT_CONTEXT.md = ephemeral, session-to-session working memory
  - DOCUMENTATION.md = permanent, stable reference
- Move items from CURRENT_CONTEXT.md to DOCUMENTATION.md when they stabilise
- One step at a time. Complete before starting the next.
