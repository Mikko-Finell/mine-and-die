# Go Idiomaticity Migration Plan

## Purpose

This plan guides the refactoring of the Mine & Die server codebase toward more idiomatic Go architecture and function-level structure. The goal is not to rewrite the simulation model, but to express it in Go’s native style—small packages, clear ownership, short functions, and explicit concurrency—while keeping the deterministic single-goroutine simulation intact.

---

## Phase 1 — Structural Extraction

**Objective:** Separate concerns without changing runtime behavior.

* Move `main`’s HTTP, WebSocket, and initialization logic into dedicated packages (e.g. `net/httpapi`, `net/ws`, `telemetry`, `match`, `sim`).
* Keep the authoritative simulation loop and its `time.Ticker` model unchanged, but restrict its public surface to minimal methods (`Apply(commands)`, `Step()`, `Snapshot()`).
* Extract logging and telemetry from deep calls; stop creating new `context.Background()` values mid-loop.
* Introduce lightweight interfaces for logger and metrics so simulation code depends only on them, not globals.
* Wrap the command queue in a small `CommandBuffer` type with metrics, keeping its lock-and-drain semantics.

*Outcome:* same runtime behaviour, but clearer package seams and smaller surface in `main`.

---

## Phase 2 — Tick Decomposition

**Objective:** Make the simulation loop readable and testable.

* Split `World.Step` into narrow helpers (`resolveCommands`, `doMovement`, `doCollisions`, `doCombat`, `doHazards`, `pruneEntities`, etc.).
* Keep tick order deterministic, but move each subsystem into its own function or file within `sim/`.
* Ensure each helper mutates state through the existing write-barrier helpers to preserve journal consistency.
* Add simple unit tests for each subsystem using fixed world seeds and replayed command sequences.

*Outcome:* identical behavior, clearer control flow, improved testability.

---

## Phase 3 — Concurrency and IO Hygiene

**Objective:** Formalize and isolate side effects.

* Replace the ad-hoc locked slice for command staging with either:

  * A preallocated ring buffer drained at tick start, **or**
  * A buffered channel drained non-blocking per tick, if determinism allows.
* Move WebSocket fan-out into per-connection goroutines with bounded queues; the Hub coordinates only.
* Simplify broadcast logic—copy subscriber map once per tick, send on each connection’s writer loop, and avoid recursive broadcast retries.
* Add metrics on dropped connections, queue depth, and tick duration.

*Outcome:* predictable IO, bounded latency, and clean separation between simulation and transport.

---

## Phase 4 — Type and Style Refinement

**Objective:** Align code with idiomatic Go syntax and ergonomics.

* Replace `any` in patch payloads with small typed structs that capture known fields.
* Remove C-style idioms (`*version++`, redundant nil guards); adopt Go’s natural `(*version)++` form and rely on panics for impossible nils.
* Shorten long methods and reduce branch depth through early returns and small helpers.
* Ensure each package has minimal exported symbols; use package-level doc comments for clarity.
* Add `go test -race`, `pprof`, and benchmark suites for tick and broadcast performance.

*Outcome:* the code reads and feels like Go—simple, direct, and maintainable.

---

## Phase 5 — Observability and Maintenance

**Objective:** Prevent regression and institutionalize idiomatic practice.

* Integrate lint checks (`golangci-lint`) for naming, complexity, and cyclomatic depth.
* Add `pprof` endpoints for tick timing and allocation tracking.
* Write a short “Style and Architecture” guide describing the intended package layout, concurrency model, and error-handling conventions.
* Require new subsystems or major PRs to demonstrate adherence to this guide.

*Outcome:* idiomatic structure becomes the baseline standard for future development.

---

### Summary

This plan preserves the existing deterministic simulation loop but expresses it in Go’s natural idioms: small packages, explicit concurrency boundaries, clear ownership, and minimal cross-coupling. Each phase can be completed independently, giving the team a controlled path from functional but C-style Go toward clean, idiomatic Go architecture.
