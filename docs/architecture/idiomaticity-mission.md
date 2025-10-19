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

- Initialized `internal/sim` package with façade types that mirror the current
  command, snapshot, and patch payloads. This scaffolding will let us convert
  external callers over without touching the legacy world structs yet.
- Routed the hub tick loop through a legacy adapter so it now stages commands
  and snapshots via the `internal/sim.Engine` façade while still delegating to
  the legacy world internals.
- Updated hub disconnect and reset flows to read snapshots through the
  `sim.Engine` adapter so state fan-out no longer touches `World` directly.

### Next task

- [x] Document the next logical follow-up step.
- [x] Wire the hub through a legacy adapter so external callers interact with
      `internal/sim.Engine` rather than touching `World` directly.
- [x] Move hub join/resubscribe/resync flows to fetch snapshots and patches via
      `sim.Engine` so read-only callers stop reaching into `World` internals.
- [x] Update hub disconnect and reset flows to pull snapshots via `sim.Engine`
      so state fan-out no longer reads the legacy world directly.
- [ ] Update hub console command flows that broadcast ground-item changes to
      source snapshots via `sim.Engine` instead of accessing `World`.

- [ ] Objective: Create seams and invariants before moving code.

- [ ] Introduce `internal/sim` façade that wraps the existing engine:

  ```go
  type Engine interface {
      Apply([]Command) error
      Step()
      Snapshot() Snapshot
      DrainPatches() []Patch
  }
  ```

  All external callers (websocket, matchmaker, etc.) must use this façade instead of touching internals.

- [ ] Add a **golden determinism test**:

  - [ ] Set a fixed seed, command script, and tick count.
  - [ ] Compute and assert the patch and journal checksum.
  - [ ] Run the check in CI to detect behavioral drift.

- [ ] Freeze **core data contracts**:

  - [ ] Lock the command schema.
  - [ ] Lock the patch and journal record format.
  - [ ] Lock tick, RNG, and sequence numbering rules.

- [ ] Add `internal/sim/patches` with round-trip test: `apply(patches(snapshot)) == state`.

- [ ] Pass injected dependencies (`Logger`, `Metrics`, `Clock`, `RNG`) via a `Deps` struct.

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
