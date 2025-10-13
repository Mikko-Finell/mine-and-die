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
| Phase 0 – Inventory, Observability, Guardrails | Complete | Tooling and telemetry guardrails landed to support future rollout. | Keep the producer map and telemetry docs current as new definitions ship. |
| Phase 1 – Contract Types & Authoritative Manager | Complete | Contract payloads, enums, math helpers, and the server manager skeleton are feature-flagged and validated. | Monitor parity while client ingestion work consumes the new contracts. |
| Phase 2 – Transport & Journal (Dual-Write) | Complete | Dual-write journal, transport toggles, and resync policy are active behind rollout flags. | Track resync telemetry during Phase 3 rollout and capture anomalies. |
| Phase 3 – Client Ingestion & Visual Manager | In Progress | Client-side scaffolding mirrors authoritative IDs; ingestion pipeline still pending. | Implement spawn/update/end batch processor and move rendering onto replicated metadata. |
| Phase 4 – Producer Migration | Complete | Gameplay producers now execute through contract-backed definitions with parity gates, including melee, projectiles, burning, and blood decals (legacy compat shim removed in Phase 6). | Monitor rollout telemetry while planning Phase 5 determinism and performance hardening. |
| Phase 5 – Determinism & Performance Hardening | Complete | Stress testing and budgets for the new system. | Keep budget alarms wired while monitoring rollout metrics. |
| Phase 6 – Cutover, Verification & Docs | In Progress | Remove legacy paths and lock the unified contract. | Prepare the deprecation switch and legacy removal gates after validating contract telemetry. |

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
* `server/main_test.go:TestLavaAppliesBurningStatusEffect` — validates hazard tick damage, burning status effect persistence, and follower visuals.
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
| Server EffectManager skeleton | Complete | :white_check_mark: Drained queued intents into `EffectInstance`s, emitted spawn/update events, and extended tests to assert queue flush + per-ID ordering. | `enableContractEffectManager` now drives contract instances with per-effect sequence counters, definition-sourced replication, and explicit end policies (instant/duration/condition) while legacy triggers still power gameplay. |
| Deterministic math helpers | Complete | :white_check_mark: Added fixed-point coordinate/geometry helpers in `server/effects_math.go` with table-driven tests covering AoE, segment, and capsule intersections. | Uses integer quantization consistent with client expectations. |

### Phase 2 — Transport & Journal (Dual-Write Rollout)

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Journal events & storage | Complete | :white_check_mark: Journal records dual-write envelopes and hub drains `effect_spawned`/`effect_update`/`effect_ended` batches during state broadcasts. | `stateMessage` now mirrors contract batches (including `effect_seq_cursors`) behind `enableContractEffectManager` + `enableContractEffectTransport`; follow-up resync hints move to dedicated deliverable. |
| Hub/messages dual-write | Complete | :white_check_mark: Added `enableContractEffectTransport` rollout flag and documented the new payload members. | Transport fields stay gated until clients ingest them; see `docs/architecture/effects.md` for field descriptions. |
| Resync policy & keyframe flow | Complete | :white_check_mark: Journal tracks lost-spawn ratios (≥0.01%) and raises hints that force the next keyframe + resync broadcast. | Added server tests covering policy math, journal hinting, and hub resync scheduling. |

### Phase 3 — Client Ingestion & Visual Manager

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Client EffectManager adapter | Complete | :white_check_mark: Mirror server IDs in JS manager keyed by `EffectID`; :white_check_mark: Wired spawn/update/end batch ingestion with sequence dedupe and unknown-ID logging; :white_check_mark: Exposed cached lifecycle metadata to the rendering path; :white_check_mark: Translated contract lifecycle payloads into definition spawn/update inputs for default effects. | Registry mirrored in client store for lookup without duplicating arrays; lifecycle view cached for render helpers, translated into definition spawn/update payloads, and passed through effect sync for contract-driven integration. |
| Two-pass processor | Complete | :white_check_mark: Added dedicated diagnostics state and wired network ingestion to reuse the two-pass lifecycle retry handler. | Unknown updates now drive the debug panel counter via `client/effect-diagnostics.js`; retry policy logs once per batch. |
| Render integration & duplication guard | Complete | :white_check_mark: Prioritized contract lifecycle replicas for render sync and suppressed legacy duplicates during dual-write. | Rendering now sources lifecycle metadata; schedule broader visual parity sweeps once additional definitions port. |

### Phase 4 — Producer Migration (Incremental, Shimmable)

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Intent helpers | Complete | :white_check_mark: Added helpers for melee/projectile/status visuals/blood decals so contract manager sees all legacy spawns. | Helpers live in `server/effect_intents.go`; ensure future definitions reuse shared quantizers. |
| Definition ports | Complete | :white_check_mark: Ported melee swings into contract hooks gated by `enableContractMeleeDefinitions`; :white_check_mark: Ported fireball projectiles behind `enableContractProjectileDefinitions`; :white_check_mark: Ported burning status visuals and ticks via `enableContractBurningDefinitions`; :white_check_mark: Ported blood decal visuals via `enableContractBloodDecalDefinitions`. | All archetypes now live behind contract definitions with dedicated rollout flags; continue monitoring parity dashboards before broad enablement. |
| Compat shim | Complete | :white_check_mark: Mirror legacy projectile explosion area effects into contract lifecycle events while definitions migrate; :white_check_mark: Extended coverage to remaining legacy-only producers. | Contract transports now include explosion spawns/ends even before dedicated definitions land; audit remaining triggers (e.g., status-specific visuals). |
| Parity metrics | Complete | :white_check_mark: Instrumented hit counts, damage accumulation, miss totals, and AoE victim buckets with per-1k tick normalization exposed via `/diagnostics.telemetry.effectParity`. | Dashboards now surface rate comparisons plus first-hit latency gauges for legacy vs. contract pipelines; monitor them while expanding rollout flags. |

### Phase 5 — Determinism & Performance Hardening

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Spatial index tuning | Complete | :white_check_mark: Tuned tile-aligned spatial index, enforced per-cell cap, and surfaced overflow telemetry. | `effects.spatialOverflow` captures capped spawns; continue validating with projectile swarm benchmarks. |
| Tick budget guards | Complete | :white_check_mark: Detect and log tick budget overruns with telemetry buckets and streak counters; :white_check_mark: escalate sustained overruns into alarms + forced resyncs. | Run loop clamps dt catch-up to two ticks, escalates ratio ≥2.0 or streak ≥3 into a forced keyframe/resync, and records alarm diagnostics for alerting. |

### Phase 6 — Cutover, Verification & Docs

| Deliverable | Status | Action Items | Notes |
| --- | --- | --- | --- |
| Table-driven contract tests | Complete | :white_check_mark: Added table-driven lifecycle tests covering area/target/visual deliveries in `server/effects_manager_contract_test.go`. | Keep fixtures versioned with contract spec. |
| Client join/resync tests | Complete | :white_check_mark: Added join/resync integration tests that exercise the two-pass patch pipeline in `client/__tests__/network.test.js`. | Integrate with existing test harness if possible. |
| Docs refresh | Complete | :white_check_mark: Refreshed the architecture overview, client module guide, playground authoring instructions, and testing/troubleshooting notes to document the contract transport. | Legacy doc references now point to contract lifecycle batches, client two-pass replay, and diagnostics surfaces. |
| Deprecation switch | Complete | :white_check_mark: Disabled legacy effect snapshots, removed the compat shim, and defaulted contract definitions to on. | Verify telemetry thresholds (95% adoption, resync rate) before removal. |

---

## Change Log

| Entry | Update | Author |
| --- | --- | --- |
| 36 | Removed the legacy effect snapshots and compat shim, defaulted contract definitions to enabled, and closed out the Phase 6 deprecation switch. | gpt-5-codex |
| 35 | Refreshed contract-era docs (architecture, client module guide, authoring, troubleshooting) and marked the Phase 6 docs deliverable complete. | gpt-5-codex |
| 34 | Landed client join/resync two-pass integration tests, marked Phase 5 complete, and advanced the Phase 6 tracker. | gpt-5-codex |
| 33 | Added table-driven contract lifecycle tests covering delivery kinds and marked the Phase 6 contract test deliverable complete. | gpt-5-codex |
| 32 | Tick budget alarms now schedule forced resyncs when ratio ≥2× or streak ≥3, expose alarm diagnostics, and mark the Phase 5 guardrail deliverable complete. | gpt-5-codex |
| 31 | Added tick budget overrun guardrails with logging, telemetry buckets, and dt catch-up clamp; marked the tracker deliverable In Progress. | gpt-5-codex |
| 30 | Tile-aligned spatial index now caps active effects per cell, updates deterministically with integer math, and emits overflow telemetry for benchmark tracking. | gpt-5-codex |
| 29 | Added a legacy-to-contract shim for projectile explosion area effects so the contract transport emits lifecycle events before full definition ports, and documented pending follow-ups. | gpt-5-codex |
| 28 | Instrumented effect parity telemetry (hit/damage/miss rates and latency buckets) and documented the diagnostics dashboard expectations. | gpt-5-codex |
| 27 | Ported blood decal visuals behind `enableContractBloodDecalDefinitions`, added contract lifecycle coverage, and marked the definition ports deliverable complete. | gpt-5-codex |
| 26 | Ported burning status damage/visuals behind `enableContractBurningDefinitions`, added contract regression tests, and synced the tracker. | gpt-5-codex |
| 25 | Ported fireball projectiles onto contract hooks gated by `enableContractProjectileDefinitions` and extended server coverage for contract-driven hits. | gpt-5-codex |
| 24 | Ported melee swings onto contract hooks behind `enableContractMeleeDefinitions`, added rollout flag documentation, and extended tests for contract execution. | gpt-5-codex |
| 23 | Completed intent helpers by translating status visuals and blood decals into contract intents with tests. | gpt-5-codex |
| 22 | Added melee/projectile intent helpers and marked the deliverable in progress for contract shims. | gpt-5-codex |
| 21 | Render loop now prefers contract lifecycle payloads and skips legacy duplicates, completing the render integration deliverable. | gpt-5-codex |
| 20 | Wired contract unknown-update diagnostics into the debug panel and marked the two-pass processor deliverable complete. | gpt-5-codex |
| 19 | Completed the client lifecycle translator so render definitions receive contract spawn/update payloads. | gpt-5-codex |
| 18 | Exposed cached lifecycle metadata to render helpers and surfaced contract entries through the client effect sync path. | gpt-5-codex |
| 17 | Added client lifecycle batch processor to ingest contract events, track sequence cursors, and surface unknown update diagnostics. | gpt-5-codex |
| 16 | Updated the Phase Overview to reflect completed groundwork and clarified the immediate client ingestion objectives. | gpt-5-codex |
| 15 | Mirrored client EffectManager instances by server `EffectID`, updated tracker to reflect in-progress adapter work, and documented follow-on wiring needs. | gpt-5-codex |
| 14 | Wired resync policy thresholds, forced keyframe scheduling, and documented the 0.01% lost-spawn trigger alongside new regression tests. | gpt-5-codex |
| 13 | Added an explicit transport rollout flag, documented the dual-write payload fields, and marked the hub/messages deliverable complete. | gpt-5-codex |
| 12 | Threaded hub dual-write onto journal batches so state payloads emit contract event envelopes; updated tracker to reflect journal deliverable completion and ongoing dual-write rollout. | gpt-5-codex |
| 11 | Added journal effect event storage, per-effect sequence cursors, and replay guidance; Phase 2 journal deliverable marked In Progress. | gpt-5-codex |
| 10 | Added contract end policies (instant/duration/condition), owner-lost handling, and selective replication checks with dedicated lifecycle tests for melee, projectile, replication-off, and sequence monotonicity. | gpt-5-codex |
| 9 | Corrected projectile delivery, added per-effect sequence counters, and sourced replication rules from definitions while tightening contract regression tests. | gpt-5-codex |
| 8 | Enabled the contract manager to emit spawn/update/end events while draining intents and added feature-flagged tests validating queue flush + ordering. | gpt-5-codex |
| 7 | Landed deterministic fixed-point math helpers (`server/effects_math.go`) plus table-driven coverage for AoE, segment, and capsule intersections to satisfy the Phase 1 math deliverable. | gpt-5-codex |
| 6 | Hardened the server `EffectManager` skeleton by clearing staged intents every tick and tracking `totalDrained` alongside `totalEnqueued` to validate parity before spawn/update/end orchestration lands. | gpt-5-codex |
| 5 | Landed unified contract structs/enums and deterministic transport events in `server/effects_contract.go`; marked the Phase 1 contract deliverable complete. | gpt-5-codex |
| 4 | Wired effect telemetry counters, exposed diagnostics metrics, and marked the Phase 0 telemetry deliverable complete. | gpt-5-codex |
| 3 | Catalogued effect regression test red list and closed the Phase 0 baseline test deliverable. | gpt-5-codex |
| 2 | Recorded snapshot payload audit and marked the Phase 0 wire documentation deliverable complete. | gpt-5-codex |
| 1 | Initial tracker created. Phase 0 map tooling marked Ready to Start with recommended first PR. | gpt-5-codex |

#### Reading the Parity Metrics

* `/diagnostics.telemetry.effectParity.totalTicks` reports the number of simulation ticks represented in the aggregate rates.
* Entries are grouped by `effectType` and `source` (legacy vs. contract) with:
  * `hitsPer1kTicks` and `damagePer1kTicks` for rate comparisons.
  * `firstHitLatencyTicks`/`Millis` capturing average tick-to-impact delay from spawn.
  * `misses` indicating total zero-hit completions.
  * `victimBuckets` distributing unique victim counts (`0`, `1`, `2`, `3`, `4+`) per effect instance.
* Use these gauges to validate parity before enabling additional contract definitions and record findings back in this tracker.

