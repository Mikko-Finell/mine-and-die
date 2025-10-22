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

## [DONE] Phase 0 — Baseline & Safety Net

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
- Removed the hub's legacy world fallback for resubscribe baseline capture so
  reconnect caching depends entirely on `sim.Engine` snapshots.
- Published telemetry journal and command drop metrics through `logging.Metrics`
  so the Phase 0 instrumentation backlog is closed out in one sweep.

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
- [x] Switch hub resubscribe baselines to cache `patches.PlayerView` values so
      façade types flow through without legacy conversions.
- [x] Persist the hub's resubscribe baseline map using `patches.PlayerView` data and
      update any callers still referencing legacy player views.
- [x] Teach the hub resubscribe flow to clone the persisted `patches.PlayerView`
      baselines when staging replay payloads so reconnects no longer depend on
      legacy player structs.
- [x] Integrate the cloned resubscribe baselines into the replay packaging so
      reconnect payloads are staged from the façade types end-to-end.
- [x] Route hub resubscribe patch snapshots through `sim.Engine` so replay
      packaging stops reading the legacy journal during reconnect staging.
- [x] Capture resubscribe baselines from `sim.Engine` snapshots so reconnect
      caching stops reading legacy player state during full snapshot packaging.
- [x] Refresh resubscribe baseline fallbacks from `sim.Engine` snapshots when
      patch application fails so hub recovery stays off `World` state.

- [x] Remove the legacy world fallback from resubscribe baseline capture so
      reconnect caching relies exclusively on `sim.Engine` snapshots.
- [x] Introduce a `sim.Deps` struct that carries `Logger`, `Metrics`, `Clock`,
      and `RNG`, and update the legacy engine adapter constructor to accept it.
- [x] Thread the hub logger, telemetry, clock, and deterministic RNG into
      `sim.Deps` when constructing the legacy adapter so the façade receives
      injected dependencies.

- [x] Expose the injected `sim.Deps` bundle through the façade so upcoming
      refactors can consume the logger, metrics, clock, and RNG without touching
      hub internals.

- [x] Switch hub consumers that need logging, metrics, clock, or RNG access to
      pull them from `sim.Engine.Deps()` so future refactors can stop reaching
      through hub fields for infrastructure.

- [x] Route hub telemetry counters through `sim.Engine.Deps().Metrics` so
      instrumentation can drop direct `logging.Publisher` dependencies next.
- [x] Introduce a metrics-backed telemetry adapter so hub tick budget and
      broadcast instrumentation no longer depend on `logging.Publisher`
      helpers directly.
- [x] Expand the telemetry metrics adapter to surface keyframe and effect
      counters through `logging.Metrics` so diagnostics can observe them
      without hub snapshots.
- [x] Publish effect parity aggregates through `logging.Metrics` so telemetry
      consumers can inspect hit/miss rates without pulling hub snapshots.
- [x] Publish journal and command drop aggregates through `logging.Metrics` so
      monitoring clients can observe drop rates without scraping telemetry
      snapshots.

- [x] Objective: Create seams and invariants before moving code.

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

- [x] Freeze **core data contracts**:

  - [x] Lock the command schema.
  - [x] Lock the patch format via adapter round-trip tests.
  - [x] Lock the journal record format.
    - Journal record format — journal round-trip tests in place.
  - [x] Lock tick, RNG, and sequence numbering rules.

- [x] Add `internal/sim/patches` with round-trip test: `apply(patches(snapshot)) == state`.

- [x] Pass injected dependencies (`Logger`, `Metrics`, `Clock`, `RNG`) via a `Deps` struct.

- [x] Add adapter coverage for journal effect batches so the façade captures the
      exact record layout before we split packages.

*Outcome:* Simulation has a narrow interface and deterministic baseline; tests ensure safety.

---

## [DONE] Phase 1 — Structural Extraction

- [ ] Objective: Separate concerns without changing runtime behavior.

### Next task

- [x] Document the next logical follow-up step.
- [x] Document the next logical follow-up step.

- [x] Move process wiring into `/cmd/server` and `internal/app`.
- [x] Move the HTTP and websocket handler construction into `internal/net` so `internal/app` depends on networking packages instead of hub internals.
- [x] Extract websocket session orchestration into `internal/net/ws` so handler code depends on a narrow subscription interface instead of hub internals.
- [x] Move networking into `internal/net`:

  - [x] Add `ws/` for websocket sessions and fan-out.
  - [x] Add `proto/` for message encode/decode and versioning.
- [x] Extract websocket message encode/decode into `internal/net/proto` and route the session handler through it.
- [x] Route keyframe and state snapshot responses through `internal/net/proto` so websocket handlers stop marshalling server messages directly.
- [x] Route HTTP join and resubscribe responses through `internal/net/proto` so REST handlers stop marshalling server messages directly.
- [x] Add an HTTP resubscribe endpoint under `internal/net` that proxies `hub.MarshalState` and returns `proto`-encoded snapshots for reconnecting clients.
- [x] Introduce `internal/net/proto` helpers that translate websocket client payloads into `sim.Command` values so the handler stops decoding request fields directly.
- [x] Convert all networking code to map messages → `sim.Command` and read `sim.Patch`/`Snapshot` without direct state access.
- [x] Route websocket command handling through a hub-level intake that accepts `sim.Command` values so networking stops calling hub-specific helper methods.
- [x] Introduce `telemetry` package for `Logger` and `Metrics` interfaces.
  - [x] Sketch interfaces for the telemetry package that wrap the existing logger and metrics dependencies before switching call sites.
- [x] Replace global loggers or random seeds with injected dependencies.
  - [x] Switch hub logging to rely on the new telemetry.Logger adapters.
- [x] Adapt hub metrics consumption to the telemetry.Metrics interface exposed by the new package.
- [x] Allow `HubConfig` to accept injected `telemetry.Metrics` instances so callers are not tied to the logging router.
  - [x] Allow `HubConfig` to accept a `telemetry.Logger` implementation to decouple logger injection from the standard library.
- [x] Allow `internal/net.HTTPHandlerConfig` to accept a `telemetry.Logger` implementation so HTTP wiring no longer depends on the standard library logger.
- [x] Allow `internal/net/ws.HandlerConfig` to accept a `telemetry.Logger` implementation so websocket wiring can drop the standard library logger dependency.
- [x] Allow `internal/app` to accept an injected `telemetry.Logger` so the process wiring can stop constructing its own standard library logger.

**Definition of done:**

- [ ] Keep all non-simulation code talking only to `internal/sim`.
- [ ] Avoid creating new `context.Background()` inside the loop.
- [ ] Keep the golden determinism test passing unchanged.

---

## [IN PROGRESS] Phase 2 — Simulation Decomposition

- [ ] Objective: Split the monolithic simulation into smaller packages with explicit ownership.

### Next task

- [x] Document the next logical follow-up step for Phase 2, outlining how to keep the tick loop inside `sim/engine` while introducing the ring-buffered command queue.

  1. **Model the bounded queue inside the engine.** Add `internal/sim/command_buffer.go` that owns a fixed-size ring of `sim.Command` values with `Push`, `Drain`, and `Len` helpers. Thread `Deps().Metrics` into the buffer so overflow and occupancy counters replace the hub's ad-hoc logging. Size the buffer from a constructor argument so tests can exercise wraparound deterministically.
  2. **Host the fixed-timestep loop in `sim`.** Introduce a `Loop` helper in `internal/sim` (for example `loop.go`) that keeps the current `RunSimulation` cadence logic — ticker setup, dt clamping, and tick accounting — but drives `Engine.Apply/Step` against the buffer. The loop should accept callbacks for fan-out (`onStep(snapshot, diff)`) so the hub can continue to broadcast without depending on world internals while the tick scheduling stays in the engine package.
  3. **Adapt hub intake to the new seam.** Replace `Hub.pendingCommands` with calls into the buffer via a thin `engine.Enqueue(Command)` façade. Migrate the per-actor throttling and drop warnings from `hub.go` into the engine/buffer layer so command ordering stays deterministic regardless of where the call originates.
  4. **Lock behavior with tests.** Port `hub_command_queue_test.go` to target the new buffer API, add focused ring-buffer coverage for wrap/drop behavior, and rerun the determinism harness to prove tick sequencing and patch/journal checksums stay unchanged once the loop lives under `internal/sim`.

- [x] Introduce the `internal/sim` ring buffer (`CommandBuffer`) and delegate the hub command queue + tick loop to the engine while keeping fan-out behavior unchanged.

- [x] Route websocket and HTTP command ingestion through `sim.Engine.Enqueue` so the hub wrapper can be retired once callers stop relying on it.

- [x] Retire `Hub.HandleCommand` by moving command validation into a shared intake helper under `internal/net` that normalizes `proto` payloads and calls `sim.Engine.Enqueue` directly, keeping rejection telemetry identical.

- [x] Begin carving out `server/internal/world` by moving the world struct, tile helpers, and RNG/time wiring into the new package while keeping the hub adapter and simulation loop behavior unchanged.
- [x] Move the world constructor (`newWorld`) and default configuration helpers into `internal/world`, exposing a constructor that the hub and tests call while leaving adapters and loop wiring untouched.
- [x] Move world obstacle generation and NPC seeding helpers into `internal/world` so the constructor's dependencies live alongside it, leaving wrappers for any legacy call sites.
- [x] Move the navigation grid helpers (`path_utils.go`) into `internal/world`, exposing wrappers in the legacy world so pathfinding continues to compile without depending on server internals.
- [x] Move the player path-following helpers (`player_path.go`) into `internal/world`, exposing adapters on the legacy world so intent updates sit alongside the navigation grid.
- [x] Move the NPC path-following helpers (`npc_path.go`) into `internal/world`, exposing adapters on the legacy world so NPC intent updates share the centralized navigation logic.
- [x] Move the path navigation helpers (`computePathFrom`, `dynamicBlockerPositions`, and `convertWorldPath` in `path_navigation.go`) into `internal/world`, exposing thin wrappers on the legacy world that delegate to the centralized pathfinding package.
- [x] Move the actor movement helpers (`moveActorWithObstacles`, `resolveAxisMoveX`, `resolveAxisMoveY`, and `resolveObstaclePenetration` in `movement.go`) into `internal/world`, exposing thin wrappers on the legacy world that delegate to the centralized movement helpers.
- [x] Move the actor collision resolver (`resolveActorCollisions` in `movement.go`) into `internal/world`, providing wrappers on the legacy world so collision separation sits alongside the movement helpers.
- [x] Move the player and NPC position mutation helpers (`applyPlayerPositionMutations` and `applyNPCPositionMutations` in `simulation.go`) into `internal/world`, exposing wrappers on the legacy world so movement commits live alongside collision handling.

- [x] Move the world stat resolution helpers (`resolveStats` and `syncMaxHealth` in `simulation.go`) into `internal/world`, exposing wrappers on the legacy world so actor stat updates live alongside the centralized movement and mutation helpers.
- [x] Move the actor health mutation helper (`setActorHealth` in `world_mutators.go`) into `internal/world`, exposing a wrapper on the legacy world so health patch emission stays alongside the centralized stat helpers.
- [x] Move the actor inventory mutation helper (`mutateActorInventory` in `world_mutators.go`) into `internal/world`, exposing wrappers on the legacy world so inventory patch emission lives alongside the centralized stat and health helpers.
- [x] Move the actor equipment mutation helper (`mutateActorEquipment` in `world_mutators.go`) into `internal/world`, exposing wrappers on the legacy world so equipment patch emission sits alongside the centralized stat, health, and inventory helpers.
- [x] Move the effect mutation helpers (`SetEffectPosition` and `SetEffectParam` in `world_mutators.go`) into `internal/world`, exposing wrappers on the legacy world so effect patch emission shares the centralized mutation utilities.
- [x] Move the ground item mutation helpers (`SetGroundItemPosition` and `SetGroundItemQuantity` in `world_mutators.go`) into `internal/world`, exposing wrappers on the legacy world so ground item patch emission shares the centralized mutation utilities.
- [x] Move the ground item lifecycle helpers (`scatterGroundItemPosition`, `upsertGroundItem`, and `removeGroundItem` in `ground_items.go`) into `internal/world`, exposing wrappers on the legacy world so item placement and cleanup share the centralized ground item utilities.
- [x] Move the ground item snapshot helpers (`groundItemsSnapshot`, `GroundItemsSnapshot`) and the shared `GroundItem` state definition into `internal/world`, adding legacy adapters so broadcast assembly keeps using the centralized structures.
- [x] Move the ground item proximity helper (`nearestGroundItem`) into `internal/world`, exposing wrappers on the legacy world so pickup targeting relies on the centralized search utilities.
- [x] Move the ground item drop helpers (`dropAllGold`, `dropAllInventory`, and `dropAllItemsOfType`) into `internal/world`, exposing wrappers on the legacy world so drop flows live alongside the centralized ground item utilities.
- [x] Move the gold pickup console flow (`pickup_gold` handling in `hub.go`) into `internal/world`, exposing thin wrappers on the hub/world so pickup validation and transfers sit with the centralized ground item utilities.
- [x] Move the gold drop console flow (`drop_gold` handling in `hub.go`) into `internal/world`, exposing thin wrappers on the hub/world so inventory removal and ground placement rely on the centralized ground item utilities.

- [x] Start carving out the journal subsystem by moving the `Journal` struct and `newJournal` constructor from `server/patches.go` into a new `internal/journal` package, exposing legacy adapters so existing callers continue to compile.
- [x] Add focused tests for `internal/journal` that cover patch/effect cloning and resync policy signals so the new package's API stays locked.

- [x] Begin carving out `internal/effects` by moving the effect manager state and lifecycle helpers from `server/effects_manager.go` into the new package, leaving legacy wrappers for existing call sites.
- [x] Move the legacy `effectState`/projectile/status helper definitions from `server/effects.go` into `internal/effects`, introducing adapters so world mutation code continues to operate on the shared types.
- [x] Move the effect spatial index (`effectSpatialIndex` and its helpers) from `server/effects_spatial_index.go` into `internal/effects`, leaving thin wrappers so hub/world code continues to compile against the shared structures.
- [x] Move the effect registration helpers (`registerEffect`/`unregisterEffect` in `effects.go`) into `internal/effects`, exposing adapters so telemetry and world slices keep using the centralized index bookkeeping.
- [x] Move the effect lookup (`findEffectByID`) and pruning (`pruneEffects`) helpers from `effects.go` into `internal/effects`, exposing legacy adapters so the centralized registry owns active-instance bookkeeping.
- [x] Persist an `internal/effects.Registry` on the legacy world so register/find/prune call sites reuse a shared instance with wired telemetry callbacks instead of rebuilding the struct each time.
- [x] Thread the world's persisted `effects.Registry` into `EffectManager` so contract-managed hooks interact with the shared bookkeeping without reaching directly into legacy slices.
- [x] Teach `internal/effects.Manager` to accept a shared registry view so contract-managed spawn and teardown hooks can register or unregister effects without calling the legacy world wrappers.
- [x] Move the contract projectile spawn helper into `internal/effects` so the manager owns contract effect instantiation without routing through the legacy world wrapper.
- [x] Move the contract blood decal spawn helper into `internal/effects` so the manager owns blood decal instantiation without relying on the legacy world wrapper.
- [x] Move the contract blood decal sync helper into `internal/effects` so the manager updates contract instances without depending on the legacy world wrapper.
- [x] Move the contract status visual sync helper into `internal/effects` so the manager updates burning visuals without depending on the legacy world wrapper.
- [x] Move the contract status visual spawn helper into `internal/effects` so the manager can instantiate burning visuals without relying on the legacy world wrapper.
- [x] Move the status visual attachment helper (`attachVisualToStatusEffect`) into `internal/world` so the effect manager can link burning visuals to actor status state without mutating legacy structs directly.
- [x] Move the status visual lifetime helpers (`extendAttachedEffect` and `expireAttachedEffect`) into `internal/world` so contract-managed visuals share centralized duration bookkeeping.
- [x] Teach the contract burning visual hook in `effects_manager.go` to call the new `internal/world` lifetime helpers when extending or ending the attached effect so contract-managed visuals reuse the centralized bookkeeping.
- [x] Extract the contract burning visual hook into `internal/effects`, introducing a configuration struct so the server wrapper only wires actor lookup and lifetime helper adapters.
- [x] Move the contract burning damage hook into `internal/effects`, reusing the shared actor lookup seam so lava damage continues to read status metadata without touching legacy world state.
- [x] Move the legacy burning damage helper (`applyBurningDamage` and its wrapper) into `internal/world`, exposing a thin adapter on `World` so the new effects hook applies lava damage through the centralized world package.
- [x] Move `NewBurningTickIntent` into `internal/effects`, returning a contract intent helper that reuses the new world burning damage API so lava damage queues share the centralized implementation.
- [x] Move `NewBloodSplatterIntent` into `internal/effects`, exposing an intent helper that reuses the shared quantization utilities so world callers stop depending on server-level geometry helpers when queuing blood decals.
- [x] Move the blood splatter configuration helpers (`newBloodSplatterParams` and `bloodSplatterColors`) into `internal/effects`, providing adapters so world callers reuse the centralized defaults when instantiating contract-managed decals.
- [x] Move the blood decal instance wiring (`ensureBloodDecalInstance`) into `internal/effects`, exposing a config-driven helper so the server wrapper only supplies runtime lookups and registries.
- [x] Move the runtime effect state helpers (`registerWorldEffect`, `unregisterWorldEffect`, `storeWorldEffect`, `loadWorldEffect`) into `internal/effects`, exposing runtime-driven adapters so the legacy world wrapper delegates to the shared package.
- [x] Extract the contract projectile lifecycle hook into `internal/effects`, introducing a configuration struct so the server wrapper only wires world lookups, registry adapters, and telemetry callbacks.
- [x] Extract the melee spawn hook into `internal/effects`, introducing a configuration seam so the server wiring only supplies actor lookups and impact resolution callbacks.
- [x] Move `resolveMeleeImpact` into `internal/world`, exposing an adapter that accepts the hook's owner reference and impact footprint so `internal/effects` delegates melee collision and telemetry through the centralized world helper.
- [x] Move `applyEffectHitPlayer`/`applyEffectHitNPC` into `internal/world`, returning callbacks so melee resolution and other hooks apply contract damage through the centralized helpers.
- [x] Move the shared actor hit dispatcher (`applyEffectHitActor` and the effect behavior lookup) into a new `internal/combat` package, returning adapters so world callbacks can resolve hits without depending on the legacy `World` type.
- [x] Move melee ability cooldown and action gating into `internal/combat`, exposing helpers so world melee execution reuses centralized combat adapters.
- [x] Move melee intent construction (`NewMeleeIntent` and `meleeAttackRectangle`) into `internal/combat`, exposing geometry helpers so effect staging relies on the centralized combat package.
- [x] Move projectile intent construction (`NewProjectileIntent` and its spawn geometry helpers) into `internal/combat`, injecting quantization and owner adapters so ability staging lives alongside other combat helpers.
- [x] Extract projectile ability gating (`triggerFireball` owner lookup and cooldown checks) into `internal/combat`, returning the staged owner reference alongside the trigger result so world callers reuse the centralized combat adapter.
- [x] Extract the fireball trigger staging into `internal/combat`, introducing a helper that consumes the projectile gate and template to return a ready contract intent so the world only enqueues the resulting effect.
- [x] Extract the melee trigger staging into `internal/combat`, introducing a helper that consumes the melee gate and intent helpers to return a ready contract intent so the world only enqueues the resulting effect.
- [x] Populate the melee and projectile ability owner references with combat intent owners so the staging helpers can drop legacy `*actorState` assertions and rely on sanitized adapters.
- [x] Teach the combat staging helpers to consume typed ability owner references directly so trigger configs can drop the `ExtractOwner` closures and rely on sanitized owners end-to-end.
- [x] Move the actor-to-intent owner conversion helpers (`meleeIntentOwner` and `newProjectileIntentOwner`) into `internal/combat` so the world ability gates pull sanitized owners straight from the combat package.
- [x] Thread `combat.AbilityActor` through `World.abilityOwner` so ability gates and trigger staging stop exposing legacy `*actorState` references.
- [x] Update the `server/effect_intents.go` helpers (`newMeleeIntent`, `NewProjectileIntent`) to accept `combat.AbilityActor` owners so intent staging can drop direct `*actorState` conversions.
- [x] Update `combat.MeleeAbilityGateConfig` and `combat.ProjectileAbilityGateConfig` to accept `combat.AbilityActor` lookups so the world gate wiring can stop converting owner snapshots into intent owners directly.
- [x] Expand the combat ability gate tests to assert ability actor position and facing metadata survive the conversion so the new lookup seam stays lossless.
- [x] Move the world effect hit wrapper (`World.applyEffectHitActor` and its adapter wiring) into `internal/combat`, exposing a dispatcher helper so hit resolution logic lives alongside the other combat helpers while the world keeps its telemetry callbacks.
- [x] Move the world player and NPC effect hit callback wiring (`worldpkg.EffectHitPlayerCallback`/`EffectHitNPCCallback` usage in `server/simulation.go`) into `internal/combat`, exposing constructor helpers so the world initialization only supplies telemetry, blood spawn, and defeat adapters while the combat package owns hit staging.

- [x] Route the burning status effect damage application in `server/status_effects.go` through a combat helper that wraps the world dispatcher so status ticks drop their direct `combat.ApplyEffectHit` calls.
- [x] Add a `server/status_effects_test.go` regression that exercises `World.applyBurningDamage` with a stub dispatcher to ensure it delegates through the combat burning damage callback and flushes telemetry after each hit.
- [x] Move the world effect-hit dispatcher wiring (`World.configureEffectHitAdapter` and friends) into `internal/combat`, exposing a constructor the world calls so telemetry and mutation adapters stay centralized while the server keeps only thin wrappers.
- [x] Move the combat damage and defeat telemetry logging into `internal/combat`, exposing adapters that accept the publisher and entity lookup so the world configuration no longer constructs logging payloads directly.
- [x] Move the combat attack-overlap telemetry into `internal/combat`, exposing an adapter that accepts the publisher, entity lookup, and projectile metadata so multi-target hits reuse the shared logging helper.
- [x] Extract the projectile overlap resolution in `server/effects.go` into `internal/combat`, returning a helper that accepts the projectile state, target iterators, hit callbacks, and telemetry recorder so the world step delegates multi-target scanning.
- [x] Extract the remaining projectile advance logic in `server/effects.go` into `internal/combat`, returning a helper that applies travel, range, and obstacle gating before delegating to the shared overlap resolver so the world wrapper only wires effect state and callbacks.
- [x] Extract the projectile stop helper in `server/effects.go` into `internal/combat`, exposing a callback-driven adapter so the world wrapper only supplies telemetry and explosion hooks.
- [x] Move the area-effect explosion spawn helper (`spawnAreaEffectAt`) into `internal/effects`, introducing a configuration seam so the world wrapper only wires ID allocation, registry adapters, and telemetry callbacks.
- [x] Thread `effects.AreaEffectSpawnConfig` through `combat.StopProjectile` so the combat helper spawns explosions directly and the world wrapper can drop its dedicated shim.
- [x] Thread `effects.AreaEffectSpawnConfig` through `combat.AdvanceProjectile` so impact explosions from hit resolution spawn via the shared helper and `World.advanceProjectile` can drop its direct call.
- [x] Replace the `Stop` callback on `combat.ProjectileAdvanceConfig` with a nested `ProjectileStopConfig` so `AdvanceProjectile` applies stop semantics directly while the world wrapper only supplies telemetry adapters.
- [x] Extract a shared world helper that builds the `combat.ProjectileStopConfig` (and reusable area-effect spawn settings) so `advanceProjectiles`, `maybeExplodeOnExpiry`, and the combat helper all delegate through the same wiring before retiring the bespoke stop wrapper.
- [x] Move the shared projectile stop wiring (`projectileStopConfig`/`areaEffectSpawnConfig`) into `internal/world`, exposing an adapter so the server wrapper only forwards the effect state and current time when stopping or advancing projectiles.
- [x] Move the legacy projectile advancement loop (`advanceProjectiles` and `maybeExplodeOnExpiry`) into `internal/world`, exposing a helper that walks non-contract projectiles and applies stop semantics through the shared adapter.
- [x] Move the legacy projectile step (`advanceProjectile`) into `internal/world`, exposing a helper that wires world geometry, overlap checks, and hit callbacks while delegating to `combat.AdvanceProjectile`.

- [ ] Move the follow-effect helpers (`advanceNonProjectiles` and `updateFollowEffect`) into `internal/world`, exposing thin wrappers on the legacy world so attachment tracking lives alongside the centralized effect helpers.

- [x] Keep the tick loop in `sim/engine`:

  - [x] Maintain the fixed timestep, command queue, and tick progression inside the engine.
  - [x] Use a **ring buffer** (`CommandBuffer`) for deterministic input instead of unbounded channels.
- [ ] Extract subpackages:

  - [x] Carve out `world/` for tiles, spatial index, RNG/time, and map helpers.
  - [x] Carve out `journal/` for write-barriers and diff recording.
  - [x] Carve out `effects/` for authoritative visual events.
  - [ ] Carve out `combat/` for hit and damage rules.
  - [ ] Carve out `stats/` for actor stats.
  - [ ] Carve out `items/` for items and equipment.
  - [ ] Carve out `ai/` for NPC logic and behaviors.
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
