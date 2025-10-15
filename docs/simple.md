# S.I.M.P.L.E. — Systematic Improvement & Maintenance for Project Logic Efficiency

This document tracks the ongoing effort to simplify and stabilize the codebase by removing duplication, reducing unnecessary checks, and consolidating systems into clear, maintainable flows. Developers can continue the work by reviewing the roadmap and active work tables below — no other documents are needed.

## Roadmap

| Phase | Goal                                                   | Exit Criteria                                                   | Status         |
| ----- | ------------------------------------------------------ | ----------------------------------------------------------------| -------------- |
| 1     | Identify duplication and unnecessary checks            | At least 10 simplification targets validated and documented     | 🟢 Complete    |
| 2     | Consolidate shared controllers for high-impact systems | Shared path engine, effect pipeline, and inventory helper prototypes merged into mainline | 🟡 In progress |
| 3     | Standardise client/server payload contracts            | Unified snapshot structs adopted and defaults sourced directly from authoritative configs | ⚪ Planned      |
| 4     | Trim residual duplication and hot-path overhead        | Generic helpers replace bespoke copies, and noisy logging paths become optional | ⚪ Planned      |

## Active Work

| Item | Goal | Status | Notes |
| --- | --- | --- | --- |
| [TOP PRIORITY] Unify snapshot/keyframe signal | Replace `hasExplicitEntityArrays` heuristics and optional `payload.type` hints with one canonical keyframe contract shared across services. | 🛑 Needed | Entity arrays currently force keyframe handling regardless of declared type, so baselines drift and cache resets fire unpredictably. |
| [TOP PRIORITY] Retire legacy sequence/reset probes | Remove the grab-bag parsing of `keyframeSeq`, `kfSeq`, `baselineSeq`, `resync`, `patchReset`, `full`, and `reset`, enforcing one authoritative sequence/reset field. | 🛑 Needed | Compatibility shims keep contradictory payloads alive; collapsing them is required to enforce a single schema. |
| Centralise effect spawn detection | Replace scattered `is*Spawn` helpers with a single matcher in `client/network.js`. | 🟡 In progress | Keeps future transport schema changes in sync. |
| Build shared actor path engine | Merge player and NPC path-following into one configurable controller. | ⚪ Planned | Covers waypoint progression and stall detection. |
| Unify inventory and equipment mutation flows | Collapse the per-actor helpers into a clone/mutate/patch routine used by every actor. | ⚪ Planned | Ensures consistent versioning and patch emission. |
| Standardise snapshot payloads and defaults | Define shared structs for `joinResponse`, `stateMessage`, and `keyframeMessage` with server-owned defaults. | ⚪ Planned | Prevents schema drift between bootstrap and runtime. |
| Remove redundant camelCase lifecycle fallbacks | Strip legacy casing branches now that the server emits snake_case arrays. | 🟡 In progress | Simplifies client payload readers. |
| Replace bespoke map copy helpers | Swap ad-hoc map copy utilities for a shared implementation or `maps.Clone`. | ⚪ Planned | Reduces duplication across the server. |
| Gate or remove always-on spawn logging | Delete or flag the per-tick spawn substring scans in `server/hub.go`. | ⚪ Planned | Cuts unnecessary hot-path work. |

## Program Goals

* Each domain (effects, inventory, pathing, etc.) has exactly one mutation entry point.
* No duplicated logic between systems or between client and server.
* Patches and keyframes follow one consistent schema.
* Logs and debug checks exist only where they serve a real diagnostic purpose.
* Prefer deterministic data-driven behavior over flag-driven branches.
* If you discover new bugs during your work, add them to `bugs.md` unless they are blocking for the current task.
* Simplification work should reduce total code volume without altering semantics.

## Suggested Next Task

