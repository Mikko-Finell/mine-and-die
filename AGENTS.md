# Instructions

We are now implementing the **world-init initiative** — bringing the client renderer fully online against authoritative world state streamed from the server.

This project is responsible for:

* simplifying the contract system

This work replaces all temporary “effects-only” rendering. Once complete, the client visibly mirrors the active 100×100 world, with real actor positions driven only by server simulation.

Start by reading `docs/architecture/effects.md` and `docs/architecture/world-init.md`.

---

### Core rules

* The client **sends only inputs** — movement and action intents.
  All authoritative world state comes **from the server only**.
* It is allowed to **lerp for visual smoothness**, but **never extrapolate or infer** missing data.
* The client must **not normalize, patch, mask, or reinterpret** anything sent by the server.
* **No fallback logic**, feature flags, or compatibility layers allowed. We fix the code, not shape the data.
* **If the server contract states a field exists with type X, it exists and is type X. Code accordingly.**

---

### You are working inside `docs/architecture/world-init.md`

This file is the **single source of truth for progress**. It replaces all tickets/spreadsheets/Notion boards for this initiative.

It tracks the exact order of implementation, from first hydration to fully visible live movement.

---

### Workflow rules

* Look for the next proposed task and start working on it. 
* The current phase is always the one marked `[IN PROGRES]`.
  Read it, do it, update it immediately when progress is made.
* When you've finished a task propose the most logical next task in `### Next task` under that phase so the next contributor can easily continue the work.
* When a phase is truly finished end-to-end, mark it `[DONE]` — **never before it is actually visible/tested.**
* If you discover a missing step, **append it as its own phase** `[TODO]` or `[IN PROGRESS]`. Do not silently improvise.
* If something is blocked by something *external*, mark it `[BLOCKED]` and state the exact reason.
