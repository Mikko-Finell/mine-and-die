# Unblock plan (3 PRs, no behaviour change)

### [DONE] PR-0: Introduce a façade-neutral state package

Goal: move the *data* out of `server/*` without moving *behaviour* yet.

* Create `internal/state` (or `internal/worldstate`) with **no imports of `sim`, `net`, or `server`**.
* Move the minimal, shared types:

  * inventory/equipment structs
  * actor/player/NPC structs
  * status-effect instances & registries (split registry traits to `internal/effects/registry` if needed to break cycles)
  * journal/event *interfaces* (append, read, snapshot) but not the legacy concrete impl
  * lightweight `Deps` interfaces needed by world (RNG, telemetry counters)
* In `server/*`, add **type aliases** (not wrappers) that point to the moved types to preserve current callers:

  ```go
  // server/player.go
  type Player = state.Player
  ```
* Build runs unchanged; determinism checksums must match.

**Acceptance:** `server` compiles with aliases; `internal/world` can import `internal/state` without cycles; determinism green.

---

### [DONE] PR-1: Make `internal/world.New` real

Goal: build the world *internally* using the new state package.

* Implement `internal/world.New(cfg, deps) (*World, error)` that:

  * [x] seeds RNG, sets defaults (copied from legacy)
  * instantiates state graphs (players/NPCs/inventory/registries)
    - [x] players
    - [x] NPCs
    - [x] inventory
    - [x] registers
    - [x] journal
    - [x] expose journal append/drain adapters on the internal world so engine callers can rely on it directly
    - [x] migrate legacy journal call sites to the new adapters so the field stops leaking through tests
    - [x] thread journal telemetry through `world.Deps` and attach it during construction
    - [x] ensure legacy boot paths supply journal telemetry via `world.Deps` when invoking the internal constructor
  * [x] wires effect registries from `internal/effects/registry`
  * [x] exposes **adapters** needed by `sim` (`AbilityOwnerLookup`, projectile stop, journal accessors) directly
* [x] Keep legacy boot alive by having `server` call into `internal/world.New` and then **decorate** with any legacy-only façade needs (no logic divergence).
* [x] Add tests that boot world via **both** constructors and assert:

  * [x] patch/journal **semantics** equal (ordering/content/timing), and
  * [x] determinism checksum unchanged.
    (Don’t require byte-for-byte buffer identity.)
  * [x] Extract a shared constructor harness that instantiates both worlds so parity tests and the determinism suite can reuse the same setup.

  * [x] extend the determinism harness to exercise both constructors using the shared harness and confirm the recorded checksums remain unchanged before promoting the new path.

  * [x] extend `server/internal/sim` tests to build the engine through the new constructor and compare patch/journal outputs against the current hub-driven harness script.

**Acceptance:** All world creation in tests/tools can use `internal/world.New`; legacy path compiles but is now just a thin forwarder.

---

### [TODO] PR-2: Promote `sim.NewEngine` and rewire the runtime

Goal: cut the hot path off the legacy hub.

*Next task:* sketch the `internal/sim.NewEngine` constructor signature and option surface so follow-up work can begin wiring the production hub through the internal entry point.

* [ ] Add `internal/sim.NewEngine(world *world.World, opts …Option) (*Engine, error)` that accepts `sim.Deps`, queue sizes, keyframe/journal hooks.
* [ ] Point `cmd/server`, `internal/app`, and handlers to build via `world.New` + `sim.NewEngine`.
  Keep `server.Hub` as a **translation façade only** (no engine creation).
* [ ] Kill tick-path converters (`legacyEngineAdapter`) by emitting snapshots/patches natively from `sim`/`world`.
* [ ] Extend determinism/replay suites to run both paths until cutover, then freeze on the new path.

**Acceptance:** Runtime boots with **zero** legacy types on the hot path; determinism & integration tests green.

---

## Guardrails while you do this

* **Import wall:** `internal/*` never imports `server/*`. Enforce with depscheck now; set it to *warn* for the first PR, *fail* by PR-2.
* **No feature drift:** only moves, promotions, and constructor plumbing; no gameplay touches.
* **Adapters last:** delete a shim in the same PR that moves its final consumer.
