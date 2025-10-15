# Patch Migration Plan

We are incrementally transitioning the Mine & Die client/server sync model from
full-state broadcasts to field-targeted patches. Each patch only includes the
fields that changed, but the payload carries the _authoritative value_ for those
fields (e.g. "hitpoints: 45" rather than "hitpoints: -5"). This convention keeps
terminology aligned with the code: we still talk about "diffs" because the
server journals which properties changed, yet those entries are snapshots of the
latest values rather than arithmetic deltas. This document tracks the current
coverage, the client-side scaffolding introduced for testing, and the next steps
required before the patch pipeline can replace the legacy snapshot flow.

## Current Server Coverage

### Player write barrier coverage

* **Position (`Actor.X`, `Actor.Y`)** – committed via `World.SetPosition`, which
  verifies that the coordinates changed, bumps the per-player version, and emits
  a position patch for clients. [server/world_mutators.go](../../server/world_mutators.go) The main
  simulation resolves movement against scratch copies and then calls
  `applyPlayerPositionMutations` to publish the final coordinates through the
  setter so the journal stays authoritative. [server/simulation.go](../../server/simulation.go)
* **Facing (`Actor.Facing`)** – updated with `World.SetFacing`, which clamps the
  direction, increments the version, and records a facing patch. [server/world_mutators.go](../../server/world_mutators.go)
  Both direct player input handling and the path follower invoke this helper so
  facing changes continue to produce patches. [server/simulation.go](../../server/simulation.go) [server/player_path.go](../../server/player_path.go)
* **Intent vectors (`playerState.intentX`, `playerState.intentY`)** – updated via
  `World.SetIntent`, which skips invalid vectors, bumps the version, and records
  an intent patch for the journal. [server/world_mutators.go](../../server/world_mutators.go) Player
  command handling routes through this setter so intent changes stay journaled
  alongside facing and position. [server/simulation.go](../../server/simulation.go) The path
  follower likewise normalizes its movement updates through the write barrier so
  server-driven navigation emits consistent patches. [server/player_path.go](../../server/player_path.go)
* **Health (`Actor.Health`)** – adjusted through `World.SetHealth`, which clamps
  to `[0, MaxHealth]`, increments the version, and appends a health patch that
  mirrors the new hit points and max health. [server/world_mutators.go](../../server/world_mutators.go)
  Damage and healing effects call this setter whenever the target is a tracked
  player so write barriers capture the change. [server/effects.go](../../server/effects.go)
* **Inventory (`Actor.Inventory`)** – wrapped by `World.MutateInventory`, which
  clones the inventory, executes the provided mutation, rolls back on error, and
  appends an inventory patch if anything actually changed. [server/world_mutators.go](../../server/world_mutators.go)
  Hub flows such as `drop_gold` and `pickup_gold` mutate player inventories via
  this helper. [server/hub.go](../../server/hub.go)
* **Version counter (`playerState.version`)** – only incremented inside the
  setters above, ensuring authoritative snapshots line up with the mutation
  journal. [server/world_mutators.go](../../server/world_mutators.go)

### NPC write barrier coverage

* **Position (`NPC.Actor.X`, `NPC.Actor.Y`)** – committed via `World.SetNPCPosition` so every movement pass funnels through the
  journal. The simulation mirrors the player pipeline by staging NPC positions on scratch copies and then calling
  `applyNPCPositionMutations` to write the results back through the barrier. [server/world_mutators.go](../../server/world_mutators.go) [server/simulation.go](../../server/simulation.go)
* **Facing (`NPC.Actor.Facing`)** – updated with `World.SetNPCFacing`. Command processing, AI actions, and the path follower all call
  this helper so server-driven rotations emit patches instead of mutating struct fields in place. [server/world_mutators.go](../../server/world_mutators.go) [server/simulation.go](../../server/simulation.go) [server/ai_executor.go](../../server/ai_executor.go) [server/npc_path.go](../../server/npc_path.go)
* **Health (`NPC.Actor.Health`)** – routed through `World.SetNPCHealth`, which shares the clamping and patch emission used for
  players. Combat behaviours now detect NPC targets and publish their damage via the write barrier. [server/world_mutators.go](../../server/world_mutators.go) [server/effects.go](../../server/effects.go)
* **Inventory (`NPC.Actor.Inventory`)** – mutated through `World.MutateNPCInventory`. Drops now clone-and-commit via the helper
  so looting and death drops stay journaled. [server/world_mutators.go](../../server/world_mutators.go) [server/ground_items.go](../../server/ground_items.go)

### Effect write barrier coverage

* **Effect transforms (`Effect.X`, `Effect.Y`)** – wrapped by `World.SetEffectPosition`. Projectile advancement, follow effects,
  and collision handlers call this helper when moving hitboxes so patches capture the authoritative coordinates. [server/world_mutators.go](../../server/world_mutators.go) [server/effects.go](../../server/effects.go)
* **Dynamic parameters (`Effect.Params`)** – funnelled through `World.SetEffectParam`. Remaining range, expiry bookkeeping, and
  other per-tick adjustments now bump the effect version and snapshot the merged parameter map for patches. [server/world_mutators.go](../../server/world_mutators.go) [server/effects.go](../../server/effects.go)

### Ground item write barrier coverage

* **Stacks (`GroundItem.Qty`, `GroundItem.X`, `GroundItem.Y`)** – updated by `World.SetGroundItemQuantity`/`World.SetGroundItemPosition`.
  Merge logic, console commands, and death drops route through these helpers so ground loot changes emit patches and bump
  per-stack versions. [server/world_mutators.go](../../server/world_mutators.go) [server/ground_items.go](../../server/ground_items.go)

### Write barrier regression tests

Dedicated unit tests cover the new helpers for every entity type. They assert version increments and patch emission for NPCs,
effects, and ground items alongside the existing player coverage so future refactors can’t silently bypass the journal. [server/world_mutators_test.go](../../server/world_mutators_test.go)

> **Important:** Any server code that mutates broadcast state must call the appropriate `World` setter or mutation helper.
> Writing directly to struct fields will skip version bumps and patch emission; reviewers should reject changes that bypass
> these barriers.

### Player state still mutated directly

The remaining player fields are still updated in place by simulation systems and
hub flows. They currently bypass write barriers and therefore do not emit
patches or version bumps:

* **Path tracking (`playerState.path` fields)** – recalculation and completion
  logic rewrites the struct directly when managing goals, indices, and arrival
  radius. [server/player_path.go](../../server/player_path.go)
* **Input timestamps (`playerState.lastInput`)** – assigned whenever a movement
  or path command is processed so diagnostics can show recent activity. [server/simulation.go](../../server/simulation.go)
* **Heartbeat metadata (`playerState.lastHeartbeat`, `playerState.lastRTT`)** –
  recorded directly on heartbeat commands and when a subscriber reconnects to a
  player slot. [server/simulation.go](../../server/simulation.go) [server/hub.go](../../server/hub.go)
* **Cooldown timers (`playerState.cooldowns`)** – lazily populated and updated
  in the ability helpers to enforce ability reuse delays. [server/effects.go](../../server/effects.go)
* **Status effect map (`actorState.statusEffects`)** – populated, refreshed, and cleaned
  up by the status effect system when effects apply or expire. [server/status_effects.go](../../server/status_effects.go)
* **Scratch movement (`actorState.X`, `actorState.Y`)** – movement integration
  still adjusts actor copies directly while resolving collisions before the
  results are written back through `SetPosition`. These adjustments never touch
  the authoritative map entries directly but are worth noting when auditing the
  pipeline. [server/movement.go](../../server/movement.go) [server/simulation.go](../../server/simulation.go)

## Client instrumentation

The client now mirrors incoming patches in a background state container so we
can validate diff replays while the game continues to rely on the authoritative
snapshot path:

* A new `createPatchState`/`updatePatchState` pair normalises player snapshots,
  enforces monotonic ticks, deduplicates recent patch keys with an LRU cache,
  clamps invalid coordinates, and records replay errors for inspection while
  preserving prior patched values across duplicate batches. [client/patches.js](../../client/patches.js)
* The main store instantiates this background state during bootstrap, and the
  network layer refreshes it on `/join` and every `state` broadcast while logging
  new patch replay issues to the console for debugging and resetting the dedupe
  history whenever the server announces a resynchronisation. [client/main.js](../../client/main.js) [client/network.js](../../client/network.js)
* The diagnostics drawer now surfaces the patch baseline tick, replay batch
  summary, and entity counts so QA can compare snapshot and diff pipelines at a
  glance without opening the console. [client/index.html](../../client/index.html) [client/main.js](../../client/main.js)
* NPC, effect, and ground item patches now replay alongside player diffs in the
  background state container, eliminating the temporary unsupported patch
  warnings while exercising dedupe logic for every entity type. [client/patches.js](../../client/patches.js)
* Keyframe recovery maintains an on-client cache of recent snapshots, requests
  server keyframes when diffs reference unknown entities, replays deferred
  batches once the baseline arrives, and surfaces recovery status in the
  diagnostics drawer. The server now bounds its keyframe journal using the
  `KEYFRAME_JOURNAL_CAPACITY` and `KEYFRAME_JOURNAL_MAX_AGE_MS` environment
  variables, publishes `keyframe` and `keyframeNack` responses (`expired` / `rate_limited`),
  rate-limits recovery RPCs, emits telemetry on journal size and NACK counts, and the
  client escalates to a resync or schedules retries with jittered backoff while tracking
  diagnostics counters. [server/patches.go](../../server/patches.go) [server/hub.go](../../server/hub.go) [server/main.go](../../server/main.go) [client/patches.js](../../client/patches.js) [client/network.js](../../client/network.js) [client/main.js](../../client/main.js)
* Vitest coverage now freezes inputs to guard against mutation, asserts
  idempotent replay counts, validates monotonic tick handling, and exercises the
  resync pathway so future patch types can extend the pipeline with
  confidence. [client/__tests__/patches.test.js](../../client/__tests__/patches.test.js)
* Patch batches now carry authoritative `sequence` counters and explicit `resync`
  markers so the client can reset history and deduplicate against the server's
  metadata instead of inferring behaviour from tick values. [server/hub.go](../../server/hub.go) [server/messages.go](../../server/messages.go) [client/patches.js](../../client/patches.js) [client/__tests__/patches.test.js](../../client/__tests__/patches.test.js)
  `sequence` is the globally monotonic message counter for state broadcasts;
  clients now require this canonical field instead of tolerating legacy aliases
  such as `seq` or `sequenceNumber`. The `resync` flag continues to delineate
  authoritative snapshot boundaries like initial joins and world resets.

## Completed steps

* ✅ **Expand patch coverage** – client-side NPC, effect, and ground item patch
  handlers mirror the server journals so replay validation covers every
  broadcast entity without console noise. [client/patches.js](../../client/patches.js) [client/__tests__/patches.test.js](../../client/__tests__/patches.test.js)
* ✅ **Patch sequence plumbing** – state broadcasts now include monotonic
  sequence numbers plus a `resync` flag, and the client dedupe cache consumes
  those fields to discard duplicates and protect against out-of-order batches
  without guessing from tick counters. [server/hub.go](../../server/hub.go) [server/messages.go](../../server/messages.go) [client/patches.js](../../client/patches.js) [client/__tests__/patches.test.js](../../client/__tests__/patches.test.js)
* ✅ **Replay validation tooling** – the diagnostics drawer surfaces patch
  baseline ticks, applied patch counts, error summaries, and entity totals by
  reading from the background patch state, letting QA compare snapshot and diff
  pipelines without inspecting the console. [client/index.html](../../client/index.html) [client/main.js](../../client/main.js)
* ✅ **Keyframe recovery** – the server journals recent snapshots alongside patch
  batches, exposes them via `keyframeSeq` references plus a `keyframeRequest`
  websocket flow, and the client consumes those frames to heal missing-entity
  diffs without console noise. [server/hub.go](../../server/hub.go) [server/main.go](../../server/main.go) [client/patches.js](../../client/patches.js) [client/network.js](../../client/network.js)
* ✅ **Switch-over rehearsal** – the render loop can now target either the
  authoritative snapshots or the patch-driven state. Console helpers
  (`debugSetRenderMode`, `debugToggleRenderMode`, or `store.setRenderMode`)
  flip the mode at runtime, share a centralised enum so diagnostics stay in sync,
  and the renderer reads from the patch container for players, NPCs, effects,
  and ground items when patch mode is active so QA can smoke test the diff
  pipeline without code edits. [client/main.js](../../client/main.js) [client/render.js](../../client/render.js) [client/render-modes.js](../../client/render-modes.js)
* ✅ **Patch-first broadcasts** – steady-state `state` messages now rely on journalled diffs with configurable keyframe intervals, and the client mirrors patch baselines when snapshots are omitted. [server/main.go](../../server/main.go) [server/hub.go](../../server/hub.go) [client/network.js](../../client/network.js)

## Suggested next steps

No open items. Continue exercising the patch renderer during playtests and add
new tasks here as follow-up issues surface.

## Keyframe cadence regression analysis

### Context

Increasing the keyframe cadence above one tick introduces a visible
"rewind-return" effect: players momentarily snap backward to a prior position or
lose transient visuals such as melee and projectile effects before snapping back
to the correct state. The defect is reproducible in patch-only broadcast mode
and disappears when full snapshots are sent every tick.

### Root cause

Early patch-mode builds reused the cached keyframe referenced by each broadcast
as the working baseline. That static snapshot carried the keyframe’s
coordinates and `sequence` value forward, so facing-only or effect-only diffs
would roll entities back to the cached position until a positional patch or new
keyframe arrived. [client/network.js](../../client/network.js) The server stream itself was
sound—the rewind came entirely from the client’s replay model.

### Resolution

The client now keeps an entity-scoped baseline that survives between broadcasts:

* `updatePatchState` prefers the cumulative baseline stored on `state.baseline`
  when applying new patches, only seeding missing entities from the cached
  keyframe instead of replacing the entire snapshot. [client/patches.js](../../client/patches.js)
* After each replay the patched view is cloned back into the baseline and its
  tick/sequence counters advance so deduplication keys compare against the
  latest state rather than the original keyframe metadata. [client/patches.js](../../client/patches.js)
* When hydrating from a cached keyframe the client now preserves the broadcast’s
  sequence and tick hints, keeping recovery semantics intact without freezing
  the baseline metadata. [client/patches.js](../../client/patches.js)

Together these adjustments eliminate the rewind-return artefact while preserving
keyframe recovery behaviour.

### Regression coverage

`client/__tests__/patches.test.js` now locks in the forward-only behaviour with
`it("maintains forward motion between sparse keyframes", …)` and updates the
cadence regression scenario to confirm that facing-only patches keep cumulative
coordinates instead of snapping back to the cached keyframe. [client/__tests__/patches.test.js](../../client/__tests__/patches.test.js)

### Outstanding issue: effect patches that precede their keyframe

Sparse keyframes expose a second gap that remains unfixed: effect parameter
patches can arrive before the client has ever seen the matching effect entity.
`updatePatchState` currently logs an `unknown entity for patch` error and drops
the payload, leaving the effect invisible until a later keyframe repopulates
the baseline. [client/__tests__/patches.test.js](../../client/__tests__/patches.test.js) This behaviour is
intentional for the moment—the new regression test ensures we can reliably
reproduce the condition while iterating on a fix.

#### Why it happens

The server emits effect parameter diffs (e.g. remaining travel time, tint, or
velocity hints) the moment an effect spawns. When keyframes are sparse, the
client may only have the cached player/NPC baseline available; the effect
itself has not yet been introduced via a keyframe or an entity-creation patch.
Because the replay model refuses to materialise entities from deltas alone, the
incoming patch finds no baseline entry and is rejected as a defensive guard.

#### Proposed solution

When the patch stream references an unknown effect, we should stage a placeholder
entity in the baseline and request the authoritative keyframe in parallel.
Future diffs can then layer onto the placeholder once the keyframe arrives or
retrofit the patch when the recovery response materialises, keeping effect
visuals alive without forcing a full resynchronisation. Coordinated fixes will
either add a dedicated creation patch from the server or teach the client to
promote effect metadata from `effect_params` payloads so the renderer receives
something meaningful immediately.
