# Unblock plan (3 PRs, no behaviour change)

### [TODO] PR-0: Introduce a façade-neutral state package

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

### [TODO] PR-1: Make `internal/world.New` real

Goal: build the world *internally* using the new state package.

* Implement `internal/world.New(cfg, deps) (*World, error)` that:

  * instantiates state graphs (players/NPCs/inventory/registries)
  * wires effect registries from `internal/effects/registry`
  * seeds RNG, sets defaults (copied from legacy)
  * exposes **adapters** needed by `sim` (`AbilityOwnerLookup`, projectile stop, journal accessors) directly
* Keep legacy boot alive by having `server` call into `internal/world.New` and then **decorate** with any legacy-only façade needs (no logic divergence).
* Add tests that boot world via **both** constructors and assert:

  * patch/journal **semantics** equal (ordering/content/timing), and
  * determinism checksum unchanged.
    (Don’t require byte-for-byte buffer identity.)

**Acceptance:** All world creation in tests/tools can use `internal/world.New`; legacy path compiles but is now just a thin forwarder.

---

### [TODO] PR-2: Promote `sim.NewEngine` and rewire the runtime

Goal: cut the hot path off the legacy hub.

* Add `internal/sim.NewEngine(world *world.World, opts …Option) (*Engine, error)` that accepts `sim.Deps`, queue sizes, keyframe/journal hooks.
* Point `cmd/server`, `internal/app`, and handlers to build via `world.New` + `sim.NewEngine`.
  Keep `server.Hub` as a **translation façade only** (no engine creation).
* Kill tick-path converters (`legacyEngineAdapter`) by emitting snapshots/patches natively from `sim`/`world`.
* Extend determinism/replay suites to run both paths until cutover, then freeze on the new path.

**Acceptance:** Runtime boots with **zero** legacy types on the hot path; determinism & integration tests green.

---

## Guardrails while you do this

* **Import wall:** `internal/*` never imports `server/*`. Enforce with depscheck now; set it to *warn* for the first PR, *fail* by PR-2.
* **No feature drift:** only moves, promotions, and constructor plumbing; no gameplay touches.
* **Adapters last:** delete a shim in the same PR that moves its final consumer.
