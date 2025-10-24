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

- [x] Objective: Separate concerns without changing runtime behavior.

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

## [DONE] Phase 2 — Simulation Decomposition

- [x] Objective: Split the monolithic simulation into smaller packages with explicit ownership.

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

- [x] Move the follow-effect helpers (`advanceNonProjectiles` and `updateFollowEffect`) into `internal/world`, exposing thin wrappers on the legacy world so attachment tracking lives alongside the centralized effect helpers.
- [x] Move the status-effect lifetime helpers (`extendAttachedEffect` and `expireAttachedEffect`) into `internal/world` so attachment expiration shares the centralized effect utilities.
- [x] Move the status-effect advancement loop (`advanceStatusEffects` and `advanceActorStatusEffects`) into `internal/world`, exposing thin wrappers on the legacy world so tick progression lives alongside the centralized status helpers.
- [x] Move the status-effect application helper (`applyStatusEffect`) into `internal/world`, returning adapter-friendly configuration so the legacy world only wires logging, telemetry, and effect manager dependencies.
- [x] Move the status-effect definition registry (`newStatusEffectDefinitions`) into `internal/world`, exposing builders that return `world.ApplyStatusEffectDefinition` values so the server wrapper only supplies effect manager and telemetry adapters.
- [x] Extend the status-effect definition builder to accept a fallback visual attachment adapter so `World.attachStatusEffectVisual` can delegate through `internal/world` when the effect manager is unavailable.
- [x] Thread the status-effect instance handle through the fallback attachment path so `World.attachStatusEffectVisual` can stop reaching into `actor.statusEffects` when the effect manager is unavailable.
- [x] Update `World.attachStatusEffectVisual` to drive status tagging and lifetime extension through the handle's attachment accessors so the fallback path stops mutating `*statusEffectInstance` fields directly.
- [x] Update `World.statusEffectsAdvanceConfig` to construct status-effect attachment callbacks via `newStatusEffectInstanceHandle` so extend/expire/clear flows reuse the handle's attachment accessors instead of touching `inst.attachedEffect` directly.
- [x] Drop the `inst.attachedEffect` guard in `World.statusEffectsAdvanceConfig` and rely on the handle's attachment helpers to no-op when no visual is attached, eliminating direct reads of the legacy field.
- [x] Remove the attachment closure nil checks in `World.statusEffectsAdvanceConfig` so the extend/expire/clear wrappers call the handle helpers directly and let them absorb missing visuals.
- [x] Drop the attachment helper guards in `World.handleBurningStatusApply` so the fallback path calls `Clear`/`Extend` directly and lets the handle no-op when no visual is present.
- [x] Drop the `handle.SetActor` guard in `World.attachStatusEffectVisual` so fallback attachments update the handle actor reference directly through the accessor.
- [x] Drop the `handle.SetActor` guard in `World.applyStatusEffect` so reused and newly created handles update the actor reference directly through the accessor.
- [x] Drop the `handle.Actor` guard in `World.handleBurningStatusApply` so the fallback path relies on the handle-provided actor reference when attaching visuals.
- [x] Drop the `handle.Actor` guard in `World.attachStatusEffectVisual` so the helper always resolves fallback actors through the handle accessor.
- [x] Add regression coverage proving the fallback attachment path resolves the actor through the handle when callers pass a nil actor pointer.
- [x] Move `World.abilityOwner` and `World.abilityOwnerState` into `internal/world`, returning adapters that expose `combat.AbilityActor` snapshots so combat ability gating lives behind the world package seams.
- [x] Expose ability gate constructors in `internal/world` that build the melee and projectile gate configs from ability owner lookups so the legacy world wrapper only wires telemetry, cooldowns, and templates.
- [x] Move the melee and projectile gate wiring into `internal/world` helpers that accept `combat` gate factories so the legacy world wrapper drops direct `combat.New*AbilityGate` calls.

- [x] Keep the tick loop in `sim/engine`:

  - [x] Maintain the fixed timestep, command queue, and tick progression inside the engine.
  - [x] Use a **ring buffer** (`CommandBuffer`) for deterministic input instead of unbounded channels.
- [ ] Extract subpackages:

  - [x] Carve out `world/` for tiles, spatial index, RNG/time, and map helpers.
  - [x] Carve out `journal/` for write-barriers and diff recording.
  - [x] Carve out `effects/` for authoritative visual events.
  - [x] Carve out `combat/` for hit and damage rules.
  - [x] Carve out `stats/` for actor stats.
  - [x] Carve out `items/` for items and equipment.
  - [x] Carve out `ai/` for NPC logic and behaviors.
  - [x] Move NPC spawn configuration and AI library bootstrapping into `internal/ai` so world construction only wires adapters and defaults.
  - [x] Move the legacy `worldNPCSpawner` adapter and associated spawn entry points into `internal/ai` so world construction simply forwards spawn callbacks and inventory defaults.
  - [x] Move the actor inventory state and mutation helpers (`inventory.go`, `equipment.go`) into a new `internal/items` package so the legacy world wrapper delegates item bookkeeping to the shared adapters.
  - [x] Move the ground item mutation and lifecycle helpers (`ground_items.go`) into `internal/items` so drop/pickup flows reuse the shared item adapters.
  - [x] Update the legacy world wrapper to delegate ground item drops and pickups through the `internal/items` helpers so journal emission and telemetry wiring live behind the shared adapters.
  - [x] Move the ground drop delegate assembly (`buildGroundDropDelegates`, `invokeGroundDrop`, and associated helpers) into `internal/items` so the legacy world wrapper only injects inventory drains, RNG hooks, and telemetry callbacks.
  - [x] Move the ground drop inventory drain helpers (`removeStacksFunc`, `removeGoldQuantityFunc`, `inventoryDrainFunc`, and `equipmentDrainFunc`) into `internal/items`, exposing config-driven adapters so the world wrapper just supplies mutate closures and actor lookups.
- [x] Thread journal-aware quantity and position setters through `items.GroundDropConfig` so drop flows record patches via shared `internal/journal` adapters instead of calling the legacy world helpers directly.
- [x] Update ground item removal to route through the journal-aware quantity setter so `items.RemoveGroundItem` records patches without invoking the legacy world helper.
- [x] Swap `World.upsertGroundItem` over to the journal-aware quantity and position setters so ground item merges stop calling the legacy world helpers.
- [x] Retire the legacy `World.SetGroundItemQuantity`/`SetGroundItemPosition` helpers by updating remaining call sites and tests to rely on the journal-aware setters directly.
- [x] Route mutations only through `journal` APIs to record diffs.
- [x] Collapse the `items.GroundDropConfig` setter hooks into a shared journal appender so `BuildGroundDropDelegates` constructs the patch-recording setters internally and callers cannot bypass diff emission.
- [x] Update `items.RemoveGroundItem` to accept a journal append callback and construct its own quantity setter so deletions always emit patches through the shared helpers.
- [x] Update `items.PickupNearestItem` to require the tile index and journal appender so it removes depleted stacks via `RemoveGroundItem` and always records quantity patches.
- [x] Retire the world `removeGroundItem` wrapper by updating tests and call sites to invoke `items.RemoveGroundItem` directly with the shared journal dependency.
- [x] Replace the `groundItemState` alias with `items.GroundItemState` throughout the server so world code works with the shared item types directly.
- [x] Replace the remaining `GroundItem` alias with `items.GroundItem` so server tests and helpers operate on the shared item structs.
- [x] Replace the `groundTileKey` alias with `items.GroundTileKey` so server code uses the shared tile metadata type directly.
- [x] Collapse the `sim_engine_adapter` ground item conversions to work with `[]itemspkg.GroundItem` end-to-end so hub snapshots and broadcasts stop hopping between legacy and shared item types.
- [x] Update hub state messages and marshaling helpers to encode `[]itemspkg.GroundItem` directly so the network layer no longer depends on `sim` ground item wrappers.
- [x] Add regression coverage proving `Hub.marshalState` emits `groundItems` using the shared `internal/items` schema so the new payload shape stays locked for network consumers.
- [x] Extend the websocket subscribe/resubscribe tests to assert the initial state payload carries the shared ground item schema and trim any remaining `sim` ground item clones from the handler.
- [x] Replace `simutil.CloneGroundItems` with an `items.CloneGroundItems` helper and update hub/keyframe call sites to rely on the shared item package for snapshot cloning.
- [x] Replace the remaining ad-hoc ground item slice clones in determinism and marshaling tests with `items.CloneGroundItems` so coverage exercises the shared helper path.
- [x] Replace `simutil.CloneEffectTriggers` with an `effects.CloneEffectTriggers` helper and route hub/journal packaging through the shared effects package so the simulation utilities keep shrinking.
- [x] Move the effect trigger conversion helpers (`simEffectTriggersFromLegacy`/`legacyEffectTriggersFromSim`) into `internal/effects` so hub and adapter callers reuse the shared clone path and the remaining map/string cloning can leave `simutil`.
- [x] Move the effect params patch payload conversions (`EffectParamsPayload` cases in `sim_engine_adapter.go`) into `internal/effects` so the adapter clones effect parameter maps through the shared helpers and `simutil` can drop its float map clone.
- [x] Move the effect event batch and resync signal conversions out of `sim_engine_adapter.go` into `internal/effects` so the adapter and hub rely on the shared helpers when cloning journal batches.
- [x] Route alive effect ID cloning through `internal/effects` so the adapter, hub, and tests stop depending on the server-local helper and `simutil` can drop its dedicated clone function.
- [x] Move the inventory and equipment patch payload conversions into `internal/items` so the adapter reuses the shared item helpers when cloning patch payloads.
- [x] Move the actor inventory and equipment snapshot conversions into `internal/items` so adapter and world callers share the same helpers when translating actors.
- [x] Add `internal/items` helpers that wrap the slot mappers to build full `sim.Inventory` and `sim.Equipment` snapshots (and the reverse) so the adapter and tests can drop their struct-assembly loops.
- [x] Update `internal/sim/patches` player replay helpers to use the shared inventory/equipment snapshot builders so clone/apply flows drop their bespoke slice copy logic.
- [x] Update `internal/simutil` clone helpers to use the shared inventory/equipment snapshot builders so the utilities drop their `items/simpayloads` slice copy dependency.
- [x] Update `internal/items/simpayloads` inventory and equipment payload cloning to reuse the shared snapshot builders so patch conversions drop their bespoke slice copy logic.
- [x] Add dedicated `internal/items/simsnapshots` helpers that assemble `sim.Inventory` and `sim.Equipment` snapshots from `[]sim` slots and update the `sim/patches`, `simutil`, and `items/simpayloads` callers to use them so identity mapper closures disappear.
- [x] Promote the slot→snapshot helpers to `internal/items` and update the existing call sites so snapshot assembly lives alongside the shared item clones without changing schemas.
- [x] Expose slot clone helpers from `internal/items` (for example `CloneInventorySlots` and `CloneEquippedItems`) and update `simutil` and `items/simpayloads` to call them directly so the struct assemblers no longer reach into `.Slots` for cloning.
- [x] Route the `sim_engine_adapter` inventory and equipment conversions through the promoted helpers so adapter cloning reuses the shared snapshot assembly.
- [x] Promote the slot conversion reflect helpers (`SimInventorySlotsFromAny` / `SimEquippedItemsFromAny`) into `internal/items` and delegate the `items/simpayloads` wrappers to them so slice normalization lives with the shared clone utilities.
- [x] Update remaining call sites to reference `items.SimInventorySlotsFromAny` / `items.SimEquippedItemsFromAny` directly and delete the pass-through wrappers from `items/simpayloads` once no external callers remain.
- [x] Update `items/simpayloads` clone helpers to call `items.CloneInventorySlots` / `items.CloneEquippedItems` directly at call sites and remove the wrappers once unused.
- [x] Remove `internal/items/simpayloads` now that patch conversions rely on the shared item helpers and add adapter coverage for the pointer-based payload cases.
- [x] Move the legacy inventory and equipment payload assembly helpers from `sim_engine_adapter.go` into `internal/items` so patch and snapshot conversions share the same adapters.
- [x] Move the legacy inventory and equipment slot conversion closures (`inventorySlotFromSim` / `equippedItemFromSim`) into `internal/items` so adapter and payload assembly callers reuse shared mapping helpers.
- [x] Move the legacy inventory and equipment assembler helpers (`inventoryFromSlots`, `inventoryPayloadFromSlots`, `equipmentFromSlots`, `equipmentPayloadFromSlots`) into `internal/items` so snapshot and payload conversions share the centralized constructors.
- [x] Update the `internal/items` snapshot and payload tests to exercise the new assembler helpers so the package relies on the shared constructors end-to-end.
- [x] Replace the remaining inventory and equipment payload struct literals in `simutil` and server tests with the `items` assembler helpers so payload cloning stays centralized.
- [x] Update `world_mutators.go` to construct inventory and equipment patch payloads via the shared `items` assembler helpers so runtime diff emission uses the centralized constructors.
- [x] Convert the remaining inventory and equipment payload constructions in `world_mutators_test.go` and other world tests to the shared helpers so test scaffolding relies on the centralized constructors.
- [x] Add world equipment mutation coverage in `world_equipment_test.go` (or neighboring world tests) that verifies patch payloads through `items.EquipmentPayloadFromSlots` so equipment scaffolding uses the centralized constructors.
- [x] Add `sim_engine_adapter` equipment patch conversion coverage that asserts `items.EquipmentPayloadFromSlots` assembles the payload so adapter cloning stays centralized.
- [x] Add `sim_engine_adapter` equipment snapshot conversion coverage that asserts `items.EquipmentValueFromSlots` assembles the player and NPC equipment so snapshot cloning stays centralized.
- [x] Add `sim_engine_adapter` equipment keyframe conversion coverage that asserts the shared item assemblers build keyframe payloads so diff archival stays centralized.
- [x] Add `sim_engine_adapter` inventory keyframe conversion coverage that asserts the shared item assemblers build keyframe payloads so diff archival stays centralized.
- [x] Add `sim_engine_adapter` equipment keyframe conversion coverage that asserts the shared item assemblers build simulation keyframe payloads so inbound journal restores stay centralized.
- [x] Add `sim_engine_adapter` inventory keyframe conversion coverage that asserts the shared item assemblers build simulation keyframe payloads so inbound journal restores stay centralized.
- [x] Add `sim_engine_adapter` keyframe ground item conversion coverage that asserts the shared clone helpers build snapshots in both directions so journal archival keeps using `items.CloneGroundItems`.
- [x] Add `sim_engine_adapter` keyframe recording coverage that proves `adapter.RecordKeyframe` clones ground items via `items.CloneGroundItems` before appending to the journal.
- [x] Add `sim_engine_adapter` keyframe lookup coverage that proves `adapter.KeyframeBySequence` clones ground items via `items.CloneGroundItems` when serving callers.
- [x] Add hub keyframe lookup coverage that proves `Hub.Keyframe` clones ground items via `items.CloneGroundItems` so snapshots cannot mutate the journal.
- [x] Add hub keyframe request coverage that proves `HandleKeyframeRequest` clones ground items via `items.CloneGroundItems` before responding to clients.
- [x] Add hub keyframe request coverage that proves player and NPC slices are deep-cloned so client mutations cannot affect the journal state.
- [x] Add hub keyframe lookup coverage that proves player and NPC slices are deep-cloned so snapshots cannot mutate the journal state.
- [x] Add hub keyframe request coverage that proves obstacle slices are deep-cloned so client mutations cannot affect the journal state.
- [x] Add hub keyframe lookup coverage that proves obstacle slices are deep-cloned so snapshots cannot mutate the journal state.
- [x] Add hub keyframe request coverage that proves world config metadata is copied before responding so client mutations cannot affect the journal state.
- [x] Add hub keyframe lookup coverage that proves world config metadata is copied before returning so snapshots cannot mutate the journal state.
- [x] Add adapter keyframe lookup coverage that proves `sim.Engine.KeyframeBySequence` copies world config metadata before returning so hub callers cannot mutate the journal state.
- [x] Add adapter keyframe recording coverage that proves `sim.Engine.RecordKeyframe` copies world config metadata before appending so journal state cannot be mutated by callers.
- [x] Add journal keyframe lookup coverage that proves `internal/journal.Journal.KeyframeBySequence` returns cloned world config metadata so downstream adapters cannot mutate stored frames.
- [x] Add journal keyframe recording coverage that proves `internal/journal.Journal.RecordKeyframe` stores cloned world config metadata so subsequent lookups cannot mutate prior entries.

**Definition of done:**

- [x] Ensure the engine depends downward (`engine → world → journal`).
- [x] Keep subpackages acyclic.
- [x] Keep the golden determinism test passing.

---

## [DONE] Phase 3 — IO and Concurrency Cleanup

- [x] Objective: Push all concurrency to the perimeter.

### Next task

- [x] Instrument websocket send queues with telemetry counters (queue depth, drops) and surface them through the existing telemetry interfaces.
 - [x] Surface websocket queue telemetry metrics through the diagnostics HTTP response so operators can track depth and drop rates in real time.
 - [x] Add diagnostics coverage that forces a subscriber queue overflow to verify the telemetry payload reports non-zero depth and drop counters.
- [x] Document the next logical follow-up step.

- [x] Give each client connection its own writer goroutine and bounded send queue.
- [x] Replace ad-hoc broadcast loops with metrics-backed fan-out queues (queue depth, drops).
- [x] Trigger resync scheduling when the broadcast fan-out queue drops updates so subscribers recover quickly. *(Waived by project owner; resync scheduling will not be implemented.)*

**Definition of done:**

- [x] Keep the simulation tick single-threaded.
- [x] Keep WS and HTTP in separate goroutines with clear boundaries.
- [x] Keep the golden test passing with tick latency at or below baseline.

---

## [DONE] Phase 4 — Typed Contracts & Versioning

- [x] Objective: Solidify data interchange formats and backward compatibility.

### Next task

- [x] Document the next logical follow-up step.
  - Prioritize the patch typing effort by defining a canonical `Patch` struct inside `internal/sim/patches`, mirroring the current map fields so legacy callers can be updated incrementally.

- [x] Replace untyped patch maps with typed structs under `sim/patches`.
- [x] Update `server/patches.go` re-exports to source from `sim/patches` instead of `internal/journal` so callers adopt the canonical patch definitions.
- [x] Switch `internal/items` ground item journal setters to emit `sim/patches` typed patch structs instead of `internal/journal` aliases.
- [x] Switch `internal/effects` patch payload helpers to emit `sim/patches` typed patch structs so effect journal emission no longer depends on `internal/journal` aliases.
- [x] Switch `internal/effects` effect event batch conversions to consume typed effect events instead of `internal/journal` aliases.
- [x] Update `server/patches.go` effect event re-exports to point at the typed definitions so downstream callers stop depending on `internal/journal` structs.
- [x] Sketch `internal/net/proto` versioned snapshot/patch encoders that wrap the typed contract structs while keeping the existing JSON shape for the compatibility window.
- [x] Thread the new versioned encoders through the hub state/keyframe/join marshaling paths while defaulting to version 1 payloads.
- [x] Freeze serialization format and validate via property tests (decode→encode→decode).
  - Lock version 1 snapshot/join/keyframe JSON fixtures under `internal/net/proto/testdata` and add round-trip tests so encoding stays stable.
- [x] Introduce `Version` field in client protocol messages.
  - Thread the websocket client payload version through `ClientMessage` decode/encode helpers while defaulting to version 1 when absent.
- [x] Update CI to fail on incompatible schema changes unless a migration flag is set.

**Definition of done:**

- [x] Keep patches, snapshots, and messages typed and versioned.
- [x] Keep compatibility mode available for older clients.
- [x] Keep the golden test producing identical checksums.

---

## [NOT STARTED] Phase 5 — Observability, Style, and Maintenance

- [ ] Objective: Prevent regression and ensure future maintainability.

### Next task

- [x] Document the next logical follow-up step for Phase 5, outlining how the observability tooling will be staged alongside new `/debug/pprof/` endpoints.
  - Centralize the debug HTTP wiring inside `internal/net` so `/debug/pprof/` registration stays co-located with the REST surface while gating the expensive trace handler behind a dedicated toggle.
  - Follow up by shaping an `ObservabilityConfig` wrapper on `app.Config` so future tooling (trace exporters, metrics scrapers) can reuse the same seam without revisiting every call site.

- [x] Integrate `pprof` and optional tracing endpoints under `/debug/pprof/`.
  - Register the standard profile handlers in `internal/net` and expose `/debug/pprof/trace` only when `EnablePprofTrace` (or the `ENABLE_PPROF_TRACE` env var) opts in, keeping the expensive recorder disabled by default.
  - Cover the new routes with handler tests so the default (disabled) and opt-in cases stay locked as we tune observability.

- [x] Promote the `EnablePprofTrace` flag into a reusable `ObservabilityConfig` struct so upcoming observability hooks can share the same injection point without bloating `HTTPHandlerConfig`.
- [ ] Add `make deps-check` to enforce import boundaries (`net/*` must not import `sim/*` internals).
- [x] Add `make deps-check` to enforce import boundaries (`net/*` must not import `sim/*` internals).
- [ ] Configure `golangci-lint` with cyclomatic limits and forbid package cycles.
  - Introduce a `make lint` entry that runs `golangci-lint` with `gocyclo` thresholds for Go code and a `depguard` rule that blocks `internal/sim/internal` imports from networking packages before wiring it into CI.
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
