# Unified Effects Migration Tracker

This tracker keeps the unified effects effort actionable. Use it with the
[Unified Effect System — Synthesis Roadmap](../architecture/unified-effects-roadmap.md)
and the [Unified Effect System — Contract-Driven Architecture](../architecture/effect-system-unification.md)
spec to decide the next piece of work, record progress, and surface open
questions.

## How to Use This Document

* **Review the shared architecture references** in
  [`docs/architecture/effect-system-unification.md`](../architecture/effect-system-unification.md)
  before you pick up work so the contract assumptions stay aligned.
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
| Phase 1 – Contract Types & Authoritative Manager | In Progress | Introduce contract types and manager while keeping behaviour identical. | Prototype EffectManager skeleton behind feature flag. |
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
| Auto producer map | Complete | :white_check_mark: Implemented `tools/effects/build_producer_map`; run `npm run effects:map` to refresh `effects_producer_map.json`. | Script documents coverage in `docs/architecture/effects.md`; map checked in under repo root. |
| Current wire audit | Complete | :white_check_mark: Documented `Hub.marshalState` payload flow and sequencing in `docs/architecture/effects.md`. | Notes & payload examples live under the new “marshalState payload layout” section. |
| Baseline tests to preserve | Complete | :white_check_mark: Catalogued effect regression coverage in `server/main_test.go`. | Red list documented below for migration guardrails. |
| Telemetry (current system) | Complete | :white_check_mark: Wired spawn/update/end/trigger counters and the active gauge into `telemetryCounters`, exposed them via `/diagnostics`, and validated the debug print path. | Metrics now surface under the `/diagnostics.telemetry.effects` and `.effectTriggers` fields; capture melee/projectile/burning baselines with `DEBUG_TELEMETRY=1` before large gameplay changes. |

#### Phase 0 Red List — Effect Regression Tests

* `server/main_test.go:TestMeleeAttackCreatesEffectAndRespectsCooldown` — verifies melee swings spawn the attack effect, enforce cooldowns, and generate unique IDs.
* `server/main_test.go:TestMeleeAttackDealsDamage` — asserts melee hitboxes apply expected damage to other players.
* `server/main_test.go:TestMeleeAttackCanDefeatGoblin` — covers NPC damage resolution and death from melee effects.
* `server/main_test.go:TestMeleeAttackAgainstGoldOreAwardsCoin` — ensures melee mining triggers resource drops through effect handling.
* `server/main_test.go:TestLavaAppliesBurningCondition` — validates hazard tick damage, burning condition persistence, and follower visuals.
* `server/main_test.go:TestTriggerFireballCreatesProjectile` — confirms trigger pipeline spawns projectile effects with travel state.
* `server/main_test.go:TestFireballDealsDamageOnHit` — exercises projectile collision damage to players.
* `server/main_test.go:TestHealthDeltaHealingClampsToMax` — checks healing effect maths clamp to max health.
* `server/main_test.go:TestHealthDamageClampsToZero` — guards lethal damage floors when applying attack effects.
* `server/main_test.go:TestProjectileExplodeOnImpactSpawnsAreaEffect` — ensures impact explosions spawn and register IDs.
* `server/main_test.go:TestProjectileExplodeOnExpirySpawnsAreaEffect` — covers expiry-triggered area effects.
* `server/main_test.go:TestFireballExpiresOnObstacleCollision` — verifies projectile effects terminate on obstacle contact.
* `server/main_test.go:TestProjectileStopPolicies` — keeps piercing vs. stop-on-hit policy behaviour deterministic.
* `server/main_test.go:TestProjectileMaxTargetsLimit` — guards maximum target hit tracking and expiry.
* `server/main_test.go:TestProjectileObstacleImpactExplosion` — checks obstacle collisions spawn configured AoE effects.
* `server/main_test.go:TestProjectileExpiryExplosionPolicy` — documents whiff-only vs. always-on expiry explosion rules.
* `server/main_test.go:TestProjectileBoundsAndLifetimeExpiry` — ensures projectiles expire when leaving bounds or exceeding lifetime.
* `server/main_test.go:TestProjectileSpawnDefaults` — guards default projectile parameters when definitions omit overrides.
* `server/main_test.go:TestProjectileOwnerImmunity` — prevents projectiles from damaging their owner through effect resolution.

### Phase 1 — Contract Types & Authoritative Manager (Server)

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Contract types & enums | Complete | :white_check_mark: Added `server/effects_contract.go` with contract structs, enums, and deterministic transport payloads. | Mirrors `effect-system-unification.md` spec; includes `Seq`/`Tick`, `FollowMode`, `EndReason`, and `ReplicationSpec` scaffolding. |
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
| 2025-10-14 | Landed unified contract structs/enums and deterministic transport events in `server/effects_contract.go`; marked the Phase 1 contract deliverable complete. | gpt-5-codex |
| 2025-10-13 | Wired effect telemetry counters, exposed diagnostics metrics, and marked the Phase 0 telemetry deliverable complete. | gpt-5-codex |
| 2025-10-12 | Catalogued effect regression test red list and closed the Phase 0 baseline test deliverable. | gpt-5-codex |
| 2025-10-11 | Recorded snapshot payload audit and marked the Phase 0 wire documentation deliverable complete. | gpt-5-codex |
| 2025-02-14 | Initial tracker created. Phase 0 map tooling marked Ready to Start with recommended first PR. | gpt-5-codex |

