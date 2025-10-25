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

## Guiding Principles

* Preserve behaviour: determinism, protocol framing, and telemetry semantics must remain identical.
* Prefer promotion over invention: upgrade existing internal APIs to public seams instead of creating new abstractions.
* Collapse shims when call sites are ready: delete adapters in the same change that moves their last consumer.
* Enforce dependency walls: `internal/` owns runtime logic; entry points and tooling depend on it, never the inverse.

---

## [BLOCKED] Phase 0 — Finalize Concrete Constructors & State Ownership

**Goal:** Make `internal/world` and `internal/sim` the authoritative constructors so legacy registries become thin pass-throughs ready for removal.

### Focus Areas

1. Harden `internal/world` constructors and configuration so they emit fully-initialized state, lifecycle managers, and registries without reaching back into `server/*` for defaults.
   - _Status:_ façade-neutral state package in place; `server` now aliases shared inventory/equipment/actor/status-effect types from `internal/state`.
   - _Next task:_ collapse `server/internal/world/constructor.go`'s registration shim by moving `legacyConstructWorld` from `server/simulation.go` into a concrete state type returned by `internal/world.New`.
   - [ ] Collapse `server/internal/world/constructor.go`'s registration shim by moving `legacyConstructWorld` from `server/simulation.go` into a concrete state type returned by `internal/world.New`.
   - [ ] Lift RNG seeding, obstacle/NPC generation, and effect registry wiring helpers from `server/simulation.go`, `server/world_mutators.go`, and related files into `server/internal/world` so configuration defaults resolve locally.
   - [ ] Publish the adapters (`AbilityOwnerLookup`, projectile stop callbacks, journal accessors) needed by the engine directly from the new `internal/world` state so callers stop reaching through `server.World` internals.
2. Promote `sim.Engine` configuration (deps, journaling, command buffer sizing, keyframe hooks) so runtime callers can build it directly without `server.Hub`.
   - [ ] Introduce an `internal/sim` constructor (e.g. `NewEngine`) that wires the command buffer, loop, and engine core using `loop.go` and `command_buffer.go`, accepting a concrete `internal/world` state plus `sim.Deps`.
   - [ ] Hoist queue sizing, warning thresholds, journaling/keyframe hooks, and telemetry wiring from `server/hub.go` into options consumed by the new constructor so non-legacy callers can configure the engine without hub glue.
   - [ ] Redirect `server/hub.go` and `server/sim_engine_adapter.go` to invoke the promoted constructor, leaving only thin translation layers until the hub façade is deleted.
3. Backfill tests proving the promoted constructors produce determinism-equivalent snapshots, journal baselines, and patch streams to the legacy paths.
   - [ ] Add `server/internal/world/constructor_test.go` that instantiates worlds via both constructors and asserts snapshots/journal state match for representative configs.
   - [ ] Extend `server/internal/sim` tests to build the engine through the new constructor and compare patch/journal outputs against the current hub-driven harness script.
   - [ ] Update `server/determinism_harness_test.go` (or a shared helper) to run against both constructor paths, locking determinism-equivalent checksums before swapping entry points.

### Exit Criteria

* All runtime wiring (CLI, tests, tooling) obtains world + engine instances via `internal/world` + `internal/sim` constructors.
* No new code references `server.NewWorld` or `server.NewHub`; remaining legacy constructors are wrappers that simply delegate then panic if unused.
* Determinism harness validates the promoted constructors yield unchanged checksums.

---

## [TODO] Phase 1 — Runtime Cutover to Internal Engine & World

**Goal:** Rewire process startup, match orchestration, and IO handlers to operate directly on the promoted internals, keeping feature parity while removing legacy indirection.

### Focus Areas

1. Update `cmd/server`, integration tests, and dev harnesses to compose `internal/app`, `internal/sim`, and `internal/world` without touching legacy hubs.
2. Migrate HTTP + websocket handler construction to accept native `sim.Engine` seams, `sim.Command` intake, and `sim/patches` outputs, deleting hub-specific glue.
3. Ensure matchmaking, scripting hooks, and tooling (replay, determinism, load tests) call directly into the internal engine/world seams.

### Exit Criteria

* Process wiring compiles without importing any `server/*` packages outside transitional adapters slated for deletion.
* All inbound command paths enqueue through `sim.Engine` interfaces; outbound snapshots/patches come straight from `internal/sim` structures.
* Golden determinism, replay, and integration suites pass using the rewired runtime.

---

## [TODO] Phase 2 — Protocol & Tooling Contract Alignment

**Goal:** Ensure every external interface (network protocol, admin tools, data exporters) reads and writes the internal contract types so no conversion helpers depend on legacy structs.

### Focus Areas

1. Update HTTP/WS payload assembly, replay serializers, and admin/reporting endpoints to encode/decode internal `sim`/`world` types directly.
2. Remove shim-only DTOs and adapters (legacy snapshots, command wrappers, patch reformatters) once consumers adopt the internal contracts.
3. Verify compatibility layers (version negotiation, schema migrations) operate purely on the internal types with regression coverage for all protocol versions still supported.

### Exit Criteria

* `internal/net` packages import only internal contract packages; no `server/*` structs appear in request/response assembly.
* Tooling binaries and scripts (replayer, golden harness, CLI inspectors) rely exclusively on internal types.
* Protocol compatibility tests and golden capture/replay fixtures pass using the aligned contracts.

---

## [TODO] Phase 3 — Legacy Deletion & Guard Rails

**Goal:** Remove the remaining legacy packages, reinforce dependency guards, and document the final architecture boundaries.

### Focus Areas

1. Delete `server/*` source, tests, and shims once all references are gone; rewrite any lingering tests against the internal packages.
2. Extend `make depscheck` / lint rules to block future imports of removed paths and ensure CI enforces the new boundaries.
3. Update architecture docs, diagrams, and onboarding guides to reference the internal engine/world entry points and removal of the façade.

### Exit Criteria

* Repository builds, determinism harness, and CI pipelines succeed with `server/*` removed.
* Dependency checks fail fast if any `server/` import reappears.
* Documentation and READMEs accurately describe the new entry points without referencing legacy facades.

---

**Final exit condition:** `git grep "server/"` → empty. Determinism & build still green. Mission ends.
