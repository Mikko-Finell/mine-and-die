# Mission: Legacy Façade Cutover & Deletion

## Objective

Retire the remaining `server/*` façade layer by promoting the `internal/sim` engine and `internal/world` state as the *real* public entry point — eliminating all legacy constructor seams, hot-path adapters, and type translations.
The server should boot purely from the new architecture, with golden determinism, dependency wall rules, and protocol behavior fully preserved.

We will **not** introduce new gameplay. This is a **deletion-driven integration mission** — execution must remain *100% behaviour-preserving* at every step.

---

## Success Criteria ("Deletion-Ready State")

✅ `internal/world.New` constructs concrete world state (no LegacyWorld registry/indirection)

✅ `sim.Engine` drives only internal packages — no conversion hops into legacy `server.World/Hub`

✅ HTTP/WS handlers speak `sim.Command` + typed `sim/patches` snapshots — no legacy structs imported

✅ `make depscheck` forbids imports from `server/*` across the tree

✅ Golden determinism checksums unchanged

✅ Only after all above — delete `server/*` entirely and CI stays green

---

## Deletion Checklist

Progress is tracked exclusively through the checklist below. When every unchecked item is complete, we can delete `server/*` and the mission ends.

### 1. Constructors & State Ownership [BLOCKED]
- [x] Extract shared inventory/equipment/actor state into `internal/state` so both legacy and internal constructors share the same types.
- [x] Relocate world state files (`inventory.go`, `equipment.go`, `player.go`, `npc.go`, `status_effects.go`, helpers) into a new internal package so `internal/world` owns the canonical structs.
- [x] Move `legacyConstructWorld` logic into a concrete type returned by `internal/world.New`, leaving the legacy constructor as a pass-through wrapper.
- [ ] Hoist RNG seeding, NPC/obstacle generation, and effect registry wiring helpers from legacy paths into `internal/world`.
- [ ] Publish adapters (`AbilityOwnerLookup`, projectile stop callbacks, journal accessors) straight from the new world state so the engine never reaches through `server.World` internals.

> **Blocked:** `internal/world` still lacks equivalents for the server façade dependencies (`EffectManager`, ability-gate wiring, status effect handlers). The constructor cannot be moved until those building blocks are promoted or new internal replacements exist.

#### Blocker remediation plan [DONE]

The remediation work below deliberately mirrors the idiomaticity mandates: each item moves legacy helpers into focussed internal
packages with explicit constructors, keeps state ownership inside `internal/world`, and deletes façade reach-ins once the
internal replacements are wired. That keeps the cutover in lockstep with the "small packages, clear ownership, explicit
dependencies" goals captured in the [idiomaticity mission](./idiomaticity-mission.md).

| Legacy dependency | Why the constructor still reaches it | Replacement we need inside `internal/world` | Idiomaticity alignment |
| --- | --- | --- | --- |
| `server/effects_manager.go` (`EffectManager`) | The world constructor still calls into the façade to register core projectile + aura behaviours, and the combat pipeline relies on façade-owned cooldown tracking. | Promote the effect manager implementation that already lives under `server/internal/effects` into `internal/world/effects`, expose a constructor that wires the projectile/aura registries, then inject that concrete type from `internal/world.New`. | Promoting the manager collapses a façade singleton into a package-scoped constructor, giving the world a concrete dependency and removing hidden global wiring. **done** |
| Ability gating hooks (`server/effects_gate_test.go`, `server/effects_manager.go`) | Ability unlock checks are delegated to façade helpers that query legacy hub state. | Extract the gate calculation (`CanCastAbility`, unlock timers, faction checks) into a pure helper under `internal/world/abilities` fed by world state, then swap the façade calls for direct helpers when building the constructor. | Pulling the logic into `internal/world/abilities` keeps gating colocated with the data it reads, replacing cross-package reach-ins with explicit, testable helpers. **done** |
| Status effect handlers (`server/status_effects.go`) | Status lifecycle wiring (apply, tick, expire) is still owned by façade helpers so `internal/world` cannot instantiate the registries alone. | Lift the registry definitions into `internal/world/status`, export helpers for registration + lifecycle wiring, and update `internal/world.New` to create the full handler set without touching legacy wiring. | Relocating the registries makes lifecycle wiring a normal dependency of the constructor, eliminating the façade hook and keeping status behaviour inside the world package boundary. |

#### Status effect remediation plan

- [x] Promote the core registries and handler interfaces from `server/status_effects.go` into a new `internal/world/status` package with explicit constructors.
- [x] Update `internal/world.New` (and dependent wiring) to build the status runtime using the new package while removing façade reach-ins.
- [x] Backfill unit coverage by migrating `server/internal/world/status_effects_test.go` to the new package and adjusting world integration tests.

Once those three replacements exist, the world constructor can build its dependencies without calling back into the façade; the
checklist items above unblock automatically because the legacy `NewWorld` wrapper becomes a pass-through and we stay within the
idiomatic ownership rules.

### 2. Engine Promotion [TODO]
- [ ] Add an `internal/sim` constructor (e.g. `NewEngine`) that wires the command buffer, loop, and engine core from existing internal pieces.
- [ ] Surface queue sizing, warnings, journaling/keyframe hooks, and telemetry wiring as options consumed by the new constructor.
- [ ] Redirect `server/hub.go` and `server/sim_engine_adapter.go` to call the promoted constructor, retaining only thin translation layers until deletion day.

### 3. Determinism & Parity Guarantees [DONE]
- [x] Add `server/internal/world/constructor_test.go` proving snapshots/journals match between constructors.
- [x] Extend `server/internal/sim` tests to compare engine output between constructors.
- [x] Update `server/determinism_harness_test.go` (and helpers) so both constructors lock the same checksums.
- [x] Refactor determinism helpers to a single `RunDeterminismHarness` entry point.
- [x] Rename the internal helper `runDeterminismHarnessLockstep` to `runDeterminismHarness` to finish the consolidation.

### 4. Runtime Cutover (Phase 1) [TODO]
- [ ] Update `cmd/server` startup and dev harnesses to compose `internal/app`, `internal/world`, and `internal/sim` directly (no hubs).
- [ ] Migrate HTTP + websocket handlers to accept `sim.Engine`, `sim.Command`, and `sim/patches` without legacy DTOs.
- [ ] Point matchmaking, scripting hooks, and tooling (replay, determinism, load tests) at the promoted seams.

### 5. Protocol & Tooling Contracts (Phase 2) [TODO]
- [ ] Encode/decode internal `sim`/`world` types in HTTP/WS payloads, replay serializers, and admin/reporting endpoints.
- [ ] Remove legacy DTO shims once consumers switch to internal contracts.
- [ ] Keep compatibility layers operating solely on internal types with regression coverage for all supported protocol versions.

### 6. Deletion & Guard Rails (Phase 3) [TODO]
- [ ] Delete `server/*` code, tests, and shims once no callers remain; rewrite lingering tests against internal packages.
- [ ] Extend `make depscheck`/lint rules to block future imports of removed paths.
- [ ] Update architecture docs, diagrams, and onboarding guides to reference the internal entry points.

**Final exit condition:** `git grep "server/"` → empty while determinism and CI stay green.

---

## Guiding Principles

* Preserve behaviour: determinism, protocol framing, and telemetry semantics must remain identical.
* Prefer promotion over invention: upgrade existing internal APIs to public seams instead of creating new abstractions.
* Collapse shims when call sites are ready: delete adapters in the same change that moves their last consumer.
* Enforce dependency walls: `internal/` owns runtime logic; entry points and tooling depend on it, never the inverse.

---

## [IN PROGRESS] Phase 0 — Finalize Concrete Constructors & State Ownership

**Goal:** Make `internal/world` and `internal/sim` the authoritative constructors so legacy registries become thin pass-throughs ready for removal.

### Focus Areas

1. Harden `internal/world` constructors until they no longer reach into `server/*` for defaults. Remaining work is tracked in the [Constructors & State Ownership checklist](#1-constructors--state-ownership).
2. Promote `sim.Engine` configuration so runtime callers can build it directly without `server.Hub`. Outstanding steps live in the [Engine Promotion checklist](#2-engine-promotion).
3. Keep determinism parity locked. The lone remaining task is recorded in the [Determinism & Parity Guarantees checklist](#3-determinism--parity-guarantees).

### Exit Criteria

* All runtime wiring (CLI, tests, tooling) obtains world + engine instances via `internal/world` + `internal/sim` constructors.
* No new code references `server.NewWorld` or `server.NewHub`; remaining legacy constructors are wrappers that simply delegate then panic if unused.
* Determinism harness validates the promoted constructors yield unchanged checksums.

---

## [IN PROGRESS] Phase 1 — Runtime Cutover to Internal Engine & World

**Goal:** Rewire process startup, match orchestration, and IO handlers to operate directly on the promoted internals, keeping feature parity while removing legacy indirection.

### Focus Areas

Complete every item under [Runtime Cutover (Phase 1)](#4-runtime-cutover-phase-1).

### Exit Criteria

* Process wiring compiles without importing any `server/*` packages outside transitional adapters slated for deletion.
* All inbound command paths enqueue through `sim.Engine` interfaces; outbound snapshots/patches come straight from `internal/sim` structures.
* Golden determinism, replay, and integration suites pass using the rewired runtime.

---

## [IN PROGRESS] Phase 2 — Protocol & Tooling Contract Alignment

**Goal:** Ensure every external interface (network protocol, admin tools, data exporters) reads and writes the internal contract types so no conversion helpers depend on legacy structs.

### Focus Areas

Complete every item under [Protocol & Tooling Contracts (Phase 2)](#5-protocol--tooling-contracts-phase-2).

### Exit Criteria

* `internal/net` packages import only internal contract packages; no `server/*` structs appear in request/response assembly.
* Tooling binaries and scripts (replayer, golden harness, CLI inspectors) rely exclusively on internal types.
* Protocol compatibility tests and golden capture/replay fixtures pass using the aligned contracts.

---

## [IN PROGRESS] Phase 3 — Legacy Deletion & Guard Rails

**Goal:** Remove the remaining legacy packages, reinforce dependency guards, and document the final architecture boundaries.

### Focus Areas

Complete every item under [Deletion & Guard Rails (Phase 3)](#6-deletion--guard-rails-phase-3).

### Exit Criteria

* Repository builds, determinism harness, and CI pipelines succeed with `server/*` removed.
* Dependency checks fail fast if any `server/` import reappears.
* Documentation and READMEs accurately describe the new entry points without referencing legacy facades.
