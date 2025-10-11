# Unified Effects Migration Tracker

This tracker keeps the unified effects effort actionable. Use it with the
[Unified Effect System — Synthesis Roadmap](../architecture/unified-effects-roadmap.md)
and the [Unified Effect System — Contract-Driven Architecture](../architecture/effect-system-unification.md)
spec to decide the next piece of work, record progress, and surface open
questions.

## How to Use This Document

* **Pick a phase** whose status is `Ready to Start` or `In Progress` and choose a
  deliverable with an open action item.
* **Create or link PRs** under the “Action Items” column so anyone can see what
  is actively being worked.
* **Update status** when a deliverable reaches its exit criteria. Keep notes so
  the next engineer understands remaining follow-up or testing debt.
* **Cross-reference docs/tests** whenever you add new artefacts; this tracker is
  the index.

Statuses use the following scale:

| Status | Meaning |
| --- | --- |
| `Not Started` | No work recorded yet. |
| `Ready to Start` | Requirements are clear; next action can begin immediately. |
| `In Progress` | Someone is actively working this deliverable. |
| `Blocked` | Waiting on another task or decision. |
| `Complete` | Exit criteria met and validated. |

---

## Phase Overview

| Phase | Status | Summary | Primary Next Step |
| --- | --- | --- | --- |
| Phase 0 – Inventory, Observability, Guardrails | Ready to Start | Tooling & telemetry foundation before touching runtime. | Build and check in the automated effect producer map script. |
| Phase 1 – Contract Types & Authoritative Manager | Not Started | Introduce contract types and manager while keeping behaviour identical. | Draft contract type definitions and enums. |
| Phase 2 – Transport & Journal (Dual-Write) | Not Started | Journal and broadcast new events alongside legacy payloads. | Design journal envelopes & toggles once Phase 1 scaffolding exists. |
| Phase 3 – Client Ingestion & Visual Manager | Not Started | Client consumes new stream deterministically with two-pass ingestion. | Prototype JS EffectManager adapter after Phase 2 dual-write exists. |
| Phase 4 – Producer Migration | Not Started | Port gameplay producers onto definitions with parity gates. | Pick one archetype (melee/projectile) for first contract-backed port. |
| Phase 5 – Determinism & Performance Hardening | Not Started | Stress testing & budgets for the new system. | Define benchmark harness & thresholds post-contract rollout. |
| Phase 6 – Cutover, Verification & Docs | Not Started | Remove legacy paths and lock contract. | Schedule adoption gate monitoring once prior phases stabilize. |

---

## Detailed Deliverable Tracking

### Phase 0 — Inventory, Observability, and Guardrails

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Auto producer map | Complete | Scripted via `npm run build:effects-producer-map` (`tools/effects/build_producer_map`) which generates `effects_producer_map.json`. | Roadmap requires CI script; coordinate with `package.json` scripts and document usage in `docs/architecture/effects.md`. |
| Current wire audit | Not Started | Document current `Hub.marshalState` snapshot contents and sequencing. | Capture ordering of `Effects[]` vs. `EffectTriggers[]`; include examples from current server. |
| Baseline tests to preserve | Not Started | Enumerate regression tests (file + test name) tied to effect behaviour. | Start with `server/main_test.go` coverage; mark tests red-listed for migration. |
| Telemetry (current system) | Not Started | Define metrics, decide aggregation surface, and note implementation plan. | Prior dashboard requirement removed—focus on counters & logging first. |

### Phase 1 — Contract Types & Authoritative Manager (Server)

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Contract types & enums | Not Started | Draft Go structs (`EffectIntent`, `EffectInstance`, `EffectDefinition`) and enums (`DeliveryKind`, etc.). | Align fields with the contract doc to avoid drift. |
| Server EffectManager skeleton | Not Started | Introduce manager struct, enqueue API, and tick scaffolding behind feature flag. | Ensure legacy path remains active until dual-write passes tests. |
| Deterministic math helpers | Not Started | Implement fixed-point geometry utilities with table-driven tests. | Use integer quantization consistent with client expectations. |

### Phase 2 — Transport & Journal (Dual-Write Rollout)

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Journal events & storage | Not Started | Extend journal schema to capture `effect_spawned`, `effect_update`, `effect_ended`. | Plan for per-effect sequence counters; tie into replay tooling. |
| Hub/messages dual-write | Not Started | Emit both legacy arrays and new event stream with configurable toggle. | Update client contracts in docs when toggling defaults. |
| Resync policy & keyframe flow | Not Started | Document thresholds and implement resync hinting once journal events exist. | Add tests for lost-spawn recovery. |

### Phase 3 — Client Ingestion & Visual Manager

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Client EffectManager adapter | Not Started | Mirror server IDs in JS manager keyed by `EffectID`. | Use shared store; avoid duplicate arrays. |
| Two-pass processor | Not Started | Implement batch processing order (spawns → updates → ends) with retry semantics. | Surface diagnostics event when unknown after retry. |
| Render integration & duplication guard | Not Started | Swap rendering onto replicated metadata and prevent double rendering during dual-write. | Validate via patch/keyframe tests. |

### Phase 4 — Producer Migration (Incremental, Shimmable)

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Intent helpers | Not Started | Provide helpers (`NewMeleeIntent`, etc.) bridging legacy calls to new contract. | Keep compatibility shim until migration complete. |
| Definition ports | Not Started | Port melee/projectile/burning/blood decal behaviours into `EffectDefinition` hooks. | Gate each archetype behind feature flags and parity dashboards. |
| Compat shim | Not Started | Translate legacy triggers into contract events during transition. | Remove once adoption thresholds satisfied. |
| Parity metrics | Not Started | Instrument hit counts, damage/tick, miss rates, AoE victim distribution. | Decide logging vs. telemetry sinks during implementation. |

### Phase 5 — Determinism & Performance Hardening

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Spatial index tuning | Not Started | Optimize integer grid and cap active effects per cell. | Measure against projectile swarm benchmarks. |
| Tick budget guards | Not Started | Add instrumentation and guardrails when tick budget exceeded. | Evaluate logging + metrics; tie into alarms later. |
| Benchmarks & SLO gates | Not Started | Create CI runnable swarm/AoE benchmarks with pass/fail thresholds. | Publish results table in docs when ready. |

### Phase 6 — Cutover, Verification & Docs

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Table-driven contract tests | Not Started | Add tests asserting spawn/update/end sequences per delivery kind. | Keep fixtures versioned with contract spec. |
| Client join/resync tests | Not Started | Script patch/keyframe scenarios and verify two-pass client behaviour. | Integrate with existing test harness if possible. |
| Docs refresh | Not Started | Update architecture docs, authoring guides, and troubleshooting notes post-cutover. | Remove legacy references once clients fully migrated. |
| Deprecation switch | Not Started | Disable legacy arrays and remove compat shim once adoption gate satisfied. | Verify telemetry thresholds (95% adoption, resync rate) before removal. |

---

## Change Log

| Date | Update | Author |
| --- | --- | --- |
| 2025-02-14 | Initial tracker created. Phase 0 map tooling marked Ready to Start with recommended first PR. | gpt-5-codex |
| 2025-10-11 | Auto producer map script added (`npm run build:effects-producer-map`) and tracker updated. | gpt-5-codex |

