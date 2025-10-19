# I.D.I.O.M. — Idiomatic Design & Implementation Optimization Mission

## Purpose

This plan guides the refactoring of the Mine & Die server codebase toward a more idiomatic Go architecture. The goal is not to rewrite or redesign gameplay logic, but to make the codebase simpler, clearer, and easier to evolve — while keeping deterministic simulation behavior intact.

---

## Guiding Principles

* **Preserve behavior** — determinism and simulation order must not change.
* **Small packages with clear ownership** — each package owns its domain, exposes a minimal interface, and hides internals.
* **Single source of truth** — one clock, one RNG, one simulation state.
* **Explicit concurrency** — the simulation loop stays single-threaded; IO and fan-out happen at the edges.
* **No globals** — logging, metrics, and randomness are injected.
* **Test-driven migration** — golden determinism tests ensure behavior parity after every refactor.

---

## [IN PROGRESS] Phase 0 — Baseline & Safety Net

### Work log

- Promoted the determinism harness into a golden test that fails when the patch
  or journal checksum drifts, giving us a CI guardrail for simulation parity.
- Initialized `internal/sim` package with façade types that mirror the current
  command, snapshot, and patch payloads. This scaffolding will let us convert
  external callers over without touching the legacy world structs yet.
- Routed the hub tick loop through a legacy adapter so it now stages commands
  and snapshots via the `internal/sim.Engine` façade while still delegating to
  the legacy world internals.
- Updated hub disconnect and reset flows to read snapshots through the
  `sim.Engine` adapter so state fan-out no longer touches `World` directly.
- Updated console `drop_gold`/`pickup_gold` broadcasts to pull ground-item
  snapshots via `sim.Engine`, keeping manual debug flows off the legacy `World`
  reads when fanning out state.
- Added a regression test to ensure the `drop_gold` console command broadcasts
  ground items from the `sim.Engine` snapshot, protecting the new read path from
  future regressions.
- Added a regression test to ensure the `pickup_gold` console command consults
  the `sim.Engine` snapshot when broadcasting, covering the complementary read
  path.
- Added a regression test that drives `RunSimulation` through a tick broadcast
  and asserts the fan-out consumes ground items from the `sim.Engine` snapshot,
  keeping the adapter authoritative for tick loops.
- Sketched a determinism harness that seeds the engine, plays a fixed command
  script, and produces patch/journal checksums for the upcoming golden test.
- Captured the determinism harness' patch and journal checksums as committed
  constants and taught the harness test to fail on drift, preserving the
  baseline record.
- Added adapter round-trip tests that verify `internal/sim` commands,
  snapshots, and patches stay in lockstep with the legacy hub structures so the
  façade data contract cannot drift silently.
- Updated hub command ingestion to stage `internal/sim.Command` values end-to-end
  so non-simulation surfaces stop depending on the legacy hub command structs.
- Added adapter round-trip coverage for effect journal batches so the façade's
  record layout matches the legacy journal before we carve out packages.
- Routed effect event draining and resync hint consumption through `sim.Engine`
  so hub fan-out and recovery flows rely on the façade rather than the legacy
  journal directly.
- Added façade-backed resync hint consumption with deep-copy conversions and a
  hub regression test that matches the legacy journal scheduling to lock the
  behavior before moving the code.
- Surfaced keyframe lookup/window access through `internal/sim.Engine`, added
  conversion round-trips for keyframes, and switched hub resync handling to rely
  on the façade instead of reading the journal directly.
- Routed hub keyframe recording through `internal/sim.Engine`, adding façade
  record-result conversions and tests so journal writes stay behind the adapter.
- Added a hub adapter regression to ensure façade-based keyframe recording
  matches the legacy journal window and eviction metadata exactly.
- Exercised the determinism harness with per-tick keyframe capture to prove the
  golden patch and journal checksums remain stable.
- Routed hub patch restoration and adapter regression to rely on
  `sim.Engine`, keeping rollback flows behind the façade.
- Added façade regression coverage for keyframe sequencing and journal effect
  batches so tick, RNG seed, and sequence semantics remain locked before
  moving simulation code.
- Added state broadcast metadata regression tests so tick/sequence/resync
  packaging stays pinned ahead of hub marshaling changes.

### Next task

- [x] Document the next logical follow-up step.
- [x] Wire the hub through a legacy adapter so external callers interact with
      `internal/sim.Engine` rather than touching `World` directly.
- [x] Move hub join/resubscribe/resync flows to fetch snapshots and patches via
      `sim.Engine` so read-only callers stop reaching into `World` internals.
- [x] Update hub disconnect and reset flows to pull snapshots via `sim.Engine`
      so state fan-out no longer reads the legacy world directly.
- [x] Update hub console command flows that broadcast ground-item changes to
      source snapshots via `sim.Engine` instead of accessing `World`.
- [x] Add a hub console command test proving ground-item broadcasts consult the
      `sim.Engine` snapshot instead of reading `World` directly.
- [x] Add a hub console command test proving gold pickup broadcasts consult the
      `sim.Engine` snapshot instead of reading `World` directly.
- [x] Add a hub tick broadcast test proving `RunSimulation` fan-out pulls ground
      items from the `sim.Engine` snapshot so the adapter stays authoritative.
- [x] Sketch a determinism harness that seeds the engine RNG, feeds a fixed
      command script for a handful of ticks, and records baseline patch/journal
      checksums for the upcoming golden test.
- [x] Capture the harness' recorded patch and journal checksums as constants so
      the forthcoming golden test can assert against a committed baseline.
- [x] Promote the determinism harness into a golden test that asserts the
      recorded checksums against the committed baseline constants.

- [x] Introduce the `internal/sim.Engine` interface in its own package along
      with façade command/snapshot/patch types so callers can stop reaching into
      the legacy hub and world directly.
- [x] Add journal round-trip coverage that proves `sim.Engine` exposes the same
      effect batch layout as the legacy journal so the façade's record format is
      locked down before carving packages.
- [x] Expose effect batch draining through `sim.Engine` and route hub broadcast
      and resync flows through the façade so non-simulation callers stop
      touching the legacy journal directly.
- [x] Route effect resync hints through `sim.Engine` with deep-copy conversions
      and lock behavior parity with a hub regression test before moving code.

- [x] Surface keyframe lookups and restores through `sim.Engine` so hub
      resynchronisation handlers stop reading the legacy journal directly.
- [x] Add adapter round-trip coverage for keyframe payloads to freeze the data
      contract before switching hub lookups to the façade.
- [x] Route keyframe recording through `sim.Engine` so hub state fan-out stops
      writing to the legacy journal directly when capturing frames.
- [x] Route patch restoration through `sim.Engine` so hub error handling stops
      writing directly to the legacy journal when replaying patches.
- [x] Add adapter regression coverage for patch restoration once the façade
      handles the replay path.

- [x] Lock tick/RNG/sequence numbering semantics behind the façade with
      regression coverage so deterministic sequencing stays frozen before
      moving code.
- [x] Add regression coverage for state broadcast metadata so tick, sequence,
      and resync packaging stays stable before adjusting hub marshaling.
- [x] Route hub state marshaling through `sim.Engine` snapshots so outbound
      payload assembly stops depending on legacy world structures.

- [x] Stand up `internal/sim/patches` with apply/snapshot round-trip coverage so
      patch application semantics are frozen before pulling more hub code
      behind the façade.
- [x] Route hub patch replay and resubscribe flows through
      `internal/sim/patches.ApplyPlayers` so diff rehydration stays behind the
      façade.
- [ ] Switch hub resubscribe baselines to cache `patches.PlayerView` values so
      façade types flow through without legacy conversions.

- [ ] Objective: Create seams and invariants before moving code.

- [x] Introduce `internal/sim` façade that wraps the existing engine:

  ```go
  type Engine interface {
      Apply([]Command) error
      Step()
      Snapshot() Snapshot
      DrainPatches() []Patch
  }
  ```

  All external callers (websocket, matchmaker, etc.) must use this façade instead of touching internals.

- [x] Add a **golden determinism test**:

  - [x] Set a fixed seed, command script, and tick count.
  - [x] Compute and assert the patch and journal checksum.
  - [x] Run the check in CI to detect behavioral drift.

- [ ] Freeze **core data contracts**:

  - [x] Lock the command schema.
  - [x] Lock the patch format via adapter round-trip tests.
  - [x] Lock the journal record format.
    - Journal record format — journal round-trip tests in place.
  - [x] Lock tick, RNG, and sequence numbering rules.

- [x] Add `internal/sim/patches` with round-trip test: `apply(patches(snapshot)) == state`.

- [ ] Pass injected dependencies (`Logger`, `Metrics`, `Clock`, `RNG`) via a `Deps` struct.

- [x] Add adapter coverage for journal effect batches so the façade captures the
      exact record layout before we split packages.

*Outcome:* Simulation has a narrow interface and deterministic baseline; tests ensure safety.

---

## [NOT STARTED] Phase 1 — Structural Extraction

- [ ] Objective: Separate concerns without changing runtime behavior.

### Next task

- [ ] Document the next logical follow-up step.

- [ ] Move process wiring into `/cmd/server` and `internal/app`.
- [ ] Move networking into `internal/net`:

  - [ ] Add `ws/` for websocket sessions and fan-out.
  - [ ] Add `proto/` for message encode/decode and versioning.
- [ ] Convert all networking code to map messages → `sim.Command` and read `sim.Patch`/`Snapshot` without direct state access.
- [ ] Introduce `telemetry` package for `Logger` and `Metrics` interfaces.
- [ ] Replace global loggers or random seeds with injected dependencies.

**Definition of done:**

- [ ] Keep all non-simulation code talking only to `internal/sim`.
- [ ] Avoid creating new `context.Background()` inside the loop.
- [ ] Keep the golden determinism test passing unchanged.

---

## [NOT STARTED] Phase 2 — Simulation Decomposition

- [ ] Objective: Split the monolithic simulation into smaller packages with explicit ownership.

### Next task

- [ ] Document the next logical follow-up step.

- [ ] Keep the tick loop in `sim/engine`:

  - [ ] Maintain the fixed timestep, command queue, and tick progression inside the engine.
  - [ ] Use a **ring buffer** (`CommandBuffer`) for deterministic input instead of unbounded channels.
- [ ] Extract subpackages:

  - [ ] Carve out `world/` for tiles, spatial index, RNG/time, and map helpers.
  - [ ] Carve out `journal/` for write-barriers and diff recording.
  - [ ] Carve out `effects/` for authoritative visual events.
  - [ ] Carve out `combat/` for hit and damage rules.
  - [ ] Carve out `stats/` for actor stats.
  - [ ] Carve out `items/` for items and equipment.
  - [ ] Carve out `ai/` for NPC logic and behaviors.
  - [ ] Others as needed. Possibly `net/` etc if it makes sense.
- [ ] Route mutations only through `journal` APIs to record diffs.
- Keep each subsystem small, try not to make any file a lot longer than 300 LOC. Not a hard requirement.

**Definition of done:**

- [ ] Ensure the engine depends downward (`engine → world → journal`).
- [ ] Keep subpackages acyclic.
- [ ] Keep the golden determinism test passing.

---

## [NOT STARTED] Phase 3 — IO and Concurrency Cleanup

- [ ] Objective: Push all concurrency to the perimeter.

### Next task

- [ ] Document the next logical follow-up step.

- [ ] Give each client connection its own writer goroutine and bounded send queue.
- [ ] Coordinate hub and match systems without blocking the simulation tick.
- [ ] Replace ad-hoc broadcast loops with metrics-backed fan-out queues (queue depth, drops).
- [ ] Add latency and tick metrics for p50/p95 duration and send queue stats.
- [ ] Compare histograms before and after the refactor to confirm no performance regression.

**Definition of done:**

- [ ] Keep the simulation tick single-threaded.
- [ ] Keep WS and HTTP in separate goroutines with clear boundaries.
- [ ] Keep the golden test passing with tick latency at or below baseline.

---

## [NOT STARTED] Phase 4 — Typed Contracts & Versioning

- [ ] Objective: Solidify data interchange formats and backward compatibility.

### Next task

- [ ] Document the next logical follow-up step.

- [ ] Replace untyped patch maps with typed structs under `sim/patches`.
- [ ] Add versioned encoders in `net/proto`; keep a compatibility layer for one release cycle.
- [ ] Freeze serialization format and validate via property tests (decode→encode→decode).
- [ ] Introduce `Version` field in client protocol messages.
- [ ] Update CI to fail on incompatible schema changes unless a migration flag is set.

**Definition of done:**

- [ ] Keep patches, snapshots, and messages typed and versioned.
- [ ] Keep compatibility mode available for older clients.
- [ ] Keep the golden test producing identical checksums.

---

## [NOT STARTED] Phase 5 — Observability, Style, and Maintenance

- [ ] Objective: Prevent regression and ensure future maintainability.

### Next task

- [ ] Document the next logical follow-up step.

- [ ] Integrate `pprof` and optional tracing endpoints under `/debug/pprof/`.
- [ ] Add `make deps-check` to enforce import boundaries (`net/*` must not import `sim/*` internals).
- [ ] Configure `golangci-lint` with cyclomatic limits and forbid package cycles.
- [ ] Add CI race detection (`go test -race ./...`).
- [ ] Commit a concise `ARCHITECTURE.md` and `STYLE.md` next to the code, not only in docs.
- [ ] Document dependency rules and testing expectations.

**Definition of done:**

- [ ] Confirm all phases complete with no determinism drift.
- [ ] Ensure CI enforces architecture, tests, lint, and race checks.
- [ ] Ensure the codebase conforms to Go idioms: small packages, clear ownership, explicit dependencies.

---

## Quick-Win Checklist

* [ ] Introduce `internal/sim` façade and route all callers through it.
* [ ] Add golden determinism test to CI.
* [ ] Extract `proto` and make WS a pure translator.
* [ ] Implement ring-buffer `CommandBuffer` with metrics.
* [ ] Carve out `journal` and `patches` packages; route writes through them.
* [ ] Add `telemetry` injection and remove globals.
* [ ] Write short `ARCHITECTURE.md` explaining new package rules.

---

**Outcome:**
Incremental migration to a clean, idiomatic Go architecture with deterministic core, clear package boundaries, safe concurrency, and test-backed confidence in every change.
