# Unified Effect System — Synthesis Roadmap (Battle-tested & Low-Risk)

A single, executable plan that blends the strongest ideas: **contract-first types without big-bang rewrites**, **dual-write rollout**, **deterministic math**, **two-pass client ingestion**, **quantified parity gates**, and **tight observability**. Each phase has: Goal → Deliverables → Exit Criteria → PR Slicing.

> **See also:** the contract specification in [Unified Effect System — Contract-Driven Architecture](./effect-system-unification.md) and the living status dashboard in [Unified Effects Migration Tracker](../notes/unified-effects-tracker.md).

---

## Phase 0 — Inventory, Observability, and Guardrails

**Goal:** Freeze the moving parts and make problems visible before changes.

**Deliverables**

* **Auto producer map**: script that enumerates every effect producer and mutation site (melee/projectile/burning/env/trigger helpers) and outputs a JSON/CSV index (file path, fn, delivery kind, cooldown/logging/journal invariants).
* **Current wire audit**: document how `Hub.marshalState` emits `Effects[]` + `EffectTriggers[]`, including sequencing/cadence.
* **Baseline tests to preserve**: list of gameplay/regression tests that must still pass post-migration (link to exact test names).
* **Telemetry (current system)**: counters for trigger usage, effect count per tick, average projectile step time, desync symptoms.

**Exit Criteria**

* Producer map checked into repo and CI-generated.
* Dashboard shows baseline rates (effects/tick, triggers/min).
* “Red list” of tests that must remain green through the migration.

**PR Slicing**

* 1 PR for the map script, 1 PR for telemetry, 1 PR for wire audit doc.

---

## Phase 1 — Contract Types & Authoritative Manager (Server)

**Goal:** Introduce the contract without breaking the loop.

**Deliverables**

* Types: `EffectIntent`, `EffectInstance`, `EffectDefinition`, shared enums (`DeliveryKind`, geometry, motion, impact policy), **minimal viable set** (melee + projectile first).
* **EffectManager** (server): intent queue, spawn/update/end phases; embedded in `World.Step` **before** pruning.
* **Deterministic math**: fixed-point coords, quantized velocity, integer AoE/segment/capsule tests (table-driven).
* **Neutral “EffectEvent” shape** to decouple journal vs. hub payloads.

**Exit Criteria**

* Melee/projectile demo definitions compile and **no behavioural change** yet (legacy path still active).
* Deterministic math unit tests pass for movement & intersections.

**PR Slicing**

* PR1: core types + minimal enums; PR2: manager skeleton; PR3: fixed-point + geometry helpers + tests.

---

## Phase 2 — Transport & Journal (Dual-Write Rollout)

**Goal:** Ship the new event stream alongside legacy payloads.

**Deliverables**

* Journal events: `effect_spawned`, `effect_update`, `effect_ended` with `(tick, seq)` and **per-effect sequence counters**.
* Hub/messages: **dual-write toggle** to emit both legacy arrays and new spawn/update/end arrays.
* Keyframe/resync: send **active-effects snapshot first**, then diffs (spec join/resync semantics).
* **Resync policy**: thresholds (e.g., ≥1 “lost spawn” per 10k events or unknown ID rate ≥0.01%) → emit resync hint.

**Exit Criteria**

* Wire fuzzer shows idempotent replay; ordering/idempotency rules documented and tested.
* Dual-write on by default; clients still consume legacy.

**PR Slicing**

* PR1: journal envelopes + storage; PR2: hub dual-write; PR3: resync/keyframe path + tests.

---

## Phase 3 — Client Ingestion & Visual Manager

**Goal:** Client consumes the contract deterministically.

**Deliverables**

* Client **EffectManager adapter** keyed by `EffectID`.
* **Two-pass batch algorithm**: (1) apply spawns, (2) apply updates with one retry, (3) apply ends.

  * Policy: **“unknown after retry = bug”** (diagnostics event).
* Rendering uses **replicated instance metadata** only (bbox, follow IDs, lifetimes), no fabricated state.
* Transitional coexistence guard: prevent double-rendering when legacy arrays + new events are both present.

**Exit Criteria**

* Patch/keyframe tests for out-of-order, duplicates, and reconnect jitter pass.
* No visual regressions in melee/projectile demo when consuming the new stream.

**PR Slicing**

* PR1: adapter + store; PR2: two-pass processor + tests; PR3: render glue + duplication guard.

---

## Phase 4 — Producer Migration (Incremental, Shimmable)

**Goal:** Move gameplay onto definitions safely.

**Deliverables**

* Intent helpers: `NewMeleeIntent`, `NewProjectileIntent`, `NewConditionTickIntent`, etc.
* Port behaviours into `EffectDefinition` hooks (`OnSpawn/OnTick/OnHit/OnExpire`) for **melee, projectile, burning, blood decal**.
* **Compat shim** (temporary): translate legacy trigger calls → contract events if needed during partial rollout.
* Consolidate **collision, cooldown, logging** in definitions (no ad-hoc helpers).

**Exit Criteria (Quantified Parity Gates)**

* **Parity metrics** (must match legacy within tolerance before delete):

  * Hit count per 1k ticks, mean damage/tick, average time-to-hit, miss rate, AoE victim distribution.
* All green on regression suite from Phase 0.

**PR Slicing**

* One archetype per PR; keep PRs <1k LOC; enable per-archetype feature flag.

---

## Phase 5 — Determinism & Performance Hardening

**Goal:** Prove it holds under stress.

**Deliverables**

* Spatial index tuned for integer math; caps on active effects per cell; overflow queues with back-pressure.
* Tick-budget guards with logs when budget exceeded; owner-loss expiry policies implemented.
* **Projectile swarm & AoE spam benchmarks** (CI runnable) with thresholds.

**Exit Criteria**

* Benchmarks meet SLOs (e.g., 15 Hz server step, P99 tick ≤ X ms with N active effects).
* Zero nondeterministic test failures across seeded runs.

**PR Slicing**

* PR1: index + caps; PR2: budget guards; PR3: benchmarks + SLO gates.

---

## Phase 6 — Cutover, Verification & Docs

**Goal:** Remove legacy safely and lock the model.

**Deliverables**

* Table-driven tests that feed intents → assert exact spawn/update/end sequences per **delivery kind**.
* Client tests for join/resync and duplicate/out-of-order handling.
* Docs refreshed: architecture, authoring new EffectDefinitions, migration notes, troubleshooting guide.
* **Deprecation switch**: turn off legacy arrays; remove compat shim.

**Exit Criteria**

* Adoption gates: ≥95% clients on new stream for 2 weeks; resync rate under threshold; parity dashboards stable.
* Legacy code removed, CI green.

**PR Slicing**

* PR1: test expansion; PR2: docs; PR3: final deprecation & cleanup.

---

## Cross-Cutting Policies (Apply Throughout)

* **Neutral Event Schema**: `EffectEvent { id, typeId, tick, seq, phase, params{geometry, followId, lifetime, custom} }` used in both journal and hub (transport decoupled from storage).
* **Telemetry** (server & client):

  * `lost_spawns`, `unknown_updates`, `duplicate_updates`, `overflow_queue_drops`, `owner_loss_expiries`, `tick_budget_overruns`.
  * Client-side desync counters mirrored to server logs for correlation.
* **Resync Triggers**: if any threshold breached (Phase 2 policy), auto-hint resync; log offending IDs.
* **Testing Discipline**: add tests **in the same PR** as feature changes; no trailing test debt.
* **PR Hygiene**: feature-flagged slices, per-archetype migrations, ≤1k LOC, clear rollback plan.
