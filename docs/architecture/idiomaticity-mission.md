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

_none_

### Next task

_

**Objective:** Create seams and invariants before moving code.

* Introduce `internal/sim` façade that wraps the existing engine:

  ```go
  type Engine interface {
      Apply([]Command) error
      Step()
      Snapshot() Snapshot
      DrainPatches() []Patch
  }
  ```

  All external callers (websocket, matchmaker, etc.) must use this façade instead of touching internals.

* Add a **golden determinism test**:

  * Fixed seed, fixed command script, fixed number of ticks.
  * Compute and assert checksum of patch/journal stream.
  * Runs in CI to detect any behavioral drift.

* Freeze **core data contracts**:

  * Command schema
  * Patch/journal record format
  * Tick, RNG, and sequence numbering rules

* Add `internal/sim/patches` with round-trip test: `apply(patches(snapshot)) == state`.

* Begin passing injected dependencies (`Logger`, `Metrics`, `Clock`, `RNG`) via a `Deps` struct.

*Outcome:* Simulation has a narrow interface and deterministic baseline; tests ensure safety.

---

## [NOT STARTED] Phase 1 — Structural Extraction

**Objective:** Separate concerns without changing runtime behavior.

* Move process wiring to `/cmd/server` and `internal/app`.
* Move networking into `internal/net`:

  * `ws/` for websocket sessions and fan-out.
  * `proto/` for message encode/decode and versioning.
* All networking code converts messages → `sim.Command` and reads `sim.Patch`/`Snapshot` — no direct state access.
* Introduce `telemetry` package for `Logger` and `Metrics` interfaces.
* Replace global loggers or random seeds with injected dependencies.

**Definition of done:**

* All non-simulation code talks only to `internal/sim`.
* No new `context.Background()` created inside the loop.
* Golden determinism test passes unchanged.

---

## [NOT STARTED] Phase 2 — Simulation Decomposition

**Objective:** Split the monolithic simulation into smaller packages with explicit ownership.

* Keep the tick loop in `sim/engine`:

  * Owns fixed timestep, command queue, tick progression.
  * Uses a **ring buffer** (`CommandBuffer`) for deterministic input, not unbounded channels.
* Extract subpackages:

  * `world/` – tiles, spatial index, RNG/time, map helpers
  * `journal/` – write-barriers and diff recording
  * `effects/` – authoritative visual events
  * `combat/` – hit/damage rules
  * `ecology/` – terrain evolution and CA rules
  * `ai/` – NPC logic and behaviors
* Mutations go only through `journal` APIs to record diffs.
* Each subsystem has ≤300 LOC per file and its own unit tests.

**Definition of done:**

* Engine depends downward (`engine → world → journal`).
* No cycles between subpackages.
* Golden determinism test passes.

---

## [NOT STARTED] Phase 3 — IO and Concurrency Cleanup

**Objective:** Push all concurrency to the perimeter.

* Each client connection has its own writer goroutine and bounded send queue.
* Hub and match systems coordinate but never block the simulation tick.
* Replace ad-hoc broadcast loops with fan-out queues using metrics (queue depth, drops).
* Add latency/tick metrics: p50/p95 tick duration, send queue stats.
* Compare histograms before/after refactor to confirm no performance regression.

**Definition of done:**

* Simulation tick remains single-threaded.
* WS and HTTP run in separate goroutines with clear boundaries.
* Golden test still passes; tick latency at or below baseline.

---

## [NOT STARTED] Phase 4 — Typed Contracts & Versioning

**Objective:** Solidify data interchange formats and backward compatibility.

* Replace untyped patch maps with typed structs under `sim/patches`.
* Add versioned encoders in `net/proto`; keep a compatibility layer for one release cycle.
* Freeze serialization format and validate via property tests (decode→encode→decode).
* Introduce `Version` field in client protocol messages.
* Update CI to fail on incompatible schema changes unless a migration flag is set.

**Definition of done:**

* Patches, snapshots, and messages are typed and versioned.
* Older clients can still connect with compatibility mode.
* Golden test passes identical checksums.

---

## [NOT STARTED] Phase 5 — Observability, Style, and Maintenance

**Objective:** Prevent regression and ensure future maintainability.

* Integrate `pprof` and optional tracing endpoints under `/debug/pprof/`.
* Add `make deps-check` to enforce import boundaries (`net/*` must not import `sim/*` internals).
* Configure `golangci-lint` with cyclomatic limits and forbid package cycles.
* Add CI race detection (`go test -race ./...`).
* Commit a concise `ARCHITECTURE.md` and `STYLE.md` next to the code, not only in docs.
* Document dependency rules and testing expectations.

**Definition of done:**

* All phases complete with no determinism drift.
* CI enforces architecture, tests, lint, and race checks.
* Codebase conforms to Go idioms: small packages, clear ownership, explicit deps.

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
