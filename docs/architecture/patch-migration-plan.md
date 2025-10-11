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
  a position patch for clients.【F:server/world_mutators.go†L16-L140】 The main
  simulation resolves movement against scratch copies and then calls
  `applyPlayerPositionMutations` to publish the final coordinates through the
  setter so the journal stays authoritative.【F:server/simulation.go†L321-L361】【F:server/simulation.go†L424-L447】
* **Facing (`Actor.Facing`)** – updated with `World.SetFacing`, which clamps the
  direction, increments the version, and records a facing patch.【F:server/world_mutators.go†L32-L156】
  Both direct player input handling and the path follower invoke this helper so
  facing changes continue to produce patches.【F:server/simulation.go†L233-L258】【F:server/player_path.go†L81-L83】
* **Intent vectors (`playerState.intentX`, `playerState.intentY`)** – updated via
  `World.SetIntent`, which skips invalid vectors, bumps the version, and records
  an intent patch for the journal.【F:server/world_mutators.go†L51-L176】 Player
  command handling routes through this setter so intent changes stay journaled
  alongside facing and position.【F:server/simulation.go†L233-L315】 The path
  follower likewise normalizes its movement updates through the write barrier so
  server-driven navigation emits consistent patches.【F:server/player_path.go†L7-L158】
* **Health (`Actor.Health`)** – adjusted through `World.SetHealth`, which clamps
  to `[0, MaxHealth]`, increments the version, and appends a health patch that
  mirrors the new hit points and max health.【F:server/world_mutators.go†L67-L192】
  Damage and healing effects call this setter whenever the target is a tracked
  player so write barriers capture the change.【F:server/effects.go†L206-L245】
* **Inventory (`Actor.Inventory`)** – wrapped by `World.MutateInventory`, which
  clones the inventory, executes the provided mutation, rolls back on error, and
  appends an inventory patch if anything actually changed.【F:server/world_mutators.go†L101-L208】
  Hub flows such as `drop_gold` and `pickup_gold` mutate player inventories via
  this helper.【F:server/hub.go†L360-L495】
* **Version counter (`playerState.version`)** – only incremented inside the
  setters above, ensuring authoritative snapshots line up with the mutation
  journal.【F:server/world_mutators.go†L16-L208】

### NPC write barrier coverage

* **Position (`NPC.Actor.X`, `NPC.Actor.Y`)** – committed via `World.SetNPCPosition` so every movement pass funnels through the
  journal. The simulation mirrors the player pipeline by staging NPC positions on scratch copies and then calling
  `applyNPCPositionMutations` to write the results back through the barrier.【F:server/world_mutators.go†L210-L264】【F:server/simulation.go†L321-L372】
* **Facing (`NPC.Actor.Facing`)** – updated with `World.SetNPCFacing`. Command processing, AI actions, and the path follower all call
  this helper so server-driven rotations emit patches instead of mutating struct fields in place.【F:server/world_mutators.go†L224-L236】【F:server/simulation.go†L239-L276】【F:server/ai_executor.go†L417-L421】【F:server/npc_path.go†L70-L95】
* **Health (`NPC.Actor.Health`)** – routed through `World.SetNPCHealth`, which shares the clamping and patch emission used for
  players. Combat behaviours now detect NPC targets and publish their damage via the write barrier.【F:server/world_mutators.go†L238-L264】【F:server/effects.go†L206-L245】
* **Inventory (`NPC.Actor.Inventory`)** – mutated through `World.MutateNPCInventory`. Drops now clone-and-commit via the helper
  so looting and death drops stay journaled.【F:server/world_mutators.go†L252-L264】【F:server/ground_items.go†L64-L180】

### Effect write barrier coverage

* **Effect transforms (`Effect.X`, `Effect.Y`)** – wrapped by `World.SetEffectPosition`. Projectile advancement, follow effects,
  and collision handlers call this helper when moving hitboxes so patches capture the authoritative coordinates.【F:server/world_mutators.go†L266-L301】【F:server/effects.go†L659-L713】【F:server/effects.go†L826-L858】
* **Dynamic parameters (`Effect.Params`)** – funnelled through `World.SetEffectParam`. Remaining range, expiry bookkeeping, and
  other per-tick adjustments now bump the effect version and snapshot the merged parameter map for patches.【F:server/world_mutators.go†L283-L301】【F:server/effects.go†L704-L713】【F:server/effects.go†L873-L882】

### Ground item write barrier coverage

* **Stacks (`GroundItem.Qty`, `GroundItem.X`, `GroundItem.Y`)** – updated by `World.SetGroundItemQuantity`/`World.SetGroundItemPosition`.
  Merge logic, console commands, and death drops route through these helpers so ground loot changes emit patches and bump
  per-stack versions.【F:server/world_mutators.go†L303-L338】【F:server/ground_items.go†L64-L166】

### Write barrier regression tests

Dedicated unit tests cover the new helpers for every entity type. They assert version increments and patch emission for NPCs,
effects, and ground items alongside the existing player coverage so future refactors can’t silently bypass the journal.【F:server/world_mutators_test.go†L372-L574】

> **Important:** Any server code that mutates broadcast state must call the appropriate `World` setter or mutation helper.
> Writing directly to struct fields will skip version bumps and patch emission; reviewers should reject changes that bypass
> these barriers.

### Player state still mutated directly

The remaining player fields are still updated in place by simulation systems and
hub flows. They currently bypass write barriers and therefore do not emit
patches or version bumps:

* **Path tracking (`playerState.path` fields)** – recalculation and completion
  logic rewrites the struct directly when managing goals, indices, and arrival
  radius.【F:server/player_path.go†L89-L169】
* **Input timestamps (`playerState.lastInput`)** – assigned whenever a movement
  or path command is processed so diagnostics can show recent activity.【F:server/simulation.go†L233-L315】
* **Heartbeat metadata (`playerState.lastHeartbeat`, `playerState.lastRTT`)** –
  recorded directly on heartbeat commands and when a subscriber reconnects to a
  player slot.【F:server/simulation.go†L288-L313】【F:server/hub.go†L169-L190】
* **Cooldown timers (`playerState.cooldowns`)** – lazily populated and updated
  in the ability helpers to enforce ability reuse delays.【F:server/effects.go†L338-L377】
* **Condition map (`actorState.conditions`)** – populated, refreshed, and cleaned
  up by the condition system when status effects apply or expire.【F:server/conditions.go†L87-L158】
* **Scratch movement (`actorState.X`, `actorState.Y`)** – movement integration
  still adjusts actor copies directly while resolving collisions before the
  results are written back through `SetPosition`. These adjustments never touch
  the authoritative map entries directly but are worth noting when auditing the
  pipeline.【F:server/movement.go†L6-L102】【F:server/simulation.go†L321-L361】

## Client instrumentation

The client now mirrors incoming patches in a background state container so we
can validate diff replays while the game continues to rely on the authoritative
snapshot path:

* A new `createPatchState`/`updatePatchState` pair normalises player snapshots,
  enforces monotonic ticks, deduplicates recent patch keys with an LRU cache,
  clamps invalid coordinates, and records replay errors for inspection while
  preserving prior patched values across duplicate batches.【F:client/patches.js†L1-L380】
* The main store instantiates this background state during bootstrap, and the
  network layer refreshes it on `/join` and every `state` broadcast while logging
  new patch replay issues to the console for debugging and resetting the dedupe
  history whenever the server announces a resynchronisation.【F:client/main.js†L9-L110】【F:client/network.js†L1-L214】【F:client/network.js†L702-L744】【F:client/network.js†L804-L851】
* The diagnostics drawer now surfaces the patch baseline tick, replay batch
  summary, and entity counts so QA can compare snapshot and diff pipelines at a
  glance without opening the console.【F:client/index.html†L288-L315】【F:client/main.js†L420-L620】
* NPC, effect, and ground item patches now replay alongside player diffs in the
  background state container, eliminating the temporary unsupported patch
  warnings while exercising dedupe logic for every entity type.【F:client/patches.js†L1-L828】
* Keyframe recovery maintains an on-client cache of recent snapshots, requests
  server keyframes when diffs reference unknown entities, replays deferred
  batches once the baseline arrives, and surfaces recovery status in the
  diagnostics drawer. The server now bounds its keyframe journal using the
  `KEYFRAME_JOURNAL_CAPACITY` and `KEYFRAME_JOURNAL_MAX_AGE_MS` environment
  variables, publishes `keyframe` and `keyframeNack` responses (`expired` / `rate_limited`),
  rate-limits recovery RPCs, emits telemetry on journal size and NACK counts, and the
  client escalates to a resync or schedules retries with jittered backoff while tracking
  diagnostics counters.【F:server/patches.go†L1-L218】【F:server/hub.go†L600-L820】【F:server/main.go†L320-L360】【F:client/patches.js†L900-L1320】【F:client/network.js†L640-L1240】【F:client/main.js†L560-L700】
* Vitest coverage now freezes inputs to guard against mutation, asserts
  idempotent replay counts, validates monotonic tick handling, and exercises the
  resync pathway so future patch types can extend the pipeline with
  confidence.【F:client/__tests__/patches.test.js†L1-L328】
* Patch batches now carry authoritative `sequence` counters and explicit `resync`
  markers so the client can reset history and deduplicate against the server's
  metadata instead of inferring behaviour from tick values.【F:server/hub.go†L617-L664】【F:server/messages.go†L13-L35】【F:client/patches.js†L720-L964】【F:client/__tests__/patches.test.js†L1-L520】
  `sequence` is the globally monotonic message counter for state broadcasts;
  clients still tolerate legacy aliases such as `seq`/`sequenceNumber`, but the
  canonical field is now `sequence`. The `resync` flag continues to delineate
  authoritative snapshot boundaries like initial joins and world resets.

## Completed steps

* ✅ **Expand patch coverage** – client-side NPC, effect, and ground item patch
  handlers mirror the server journals so replay validation covers every
  broadcast entity without console noise.【F:client/patches.js†L1-L828】【F:client/__tests__/patches.test.js†L1-L328】
* ✅ **Patch sequence plumbing** – state broadcasts now include monotonic
  sequence numbers plus a `resync` flag, and the client dedupe cache consumes
  those fields to discard duplicates and protect against out-of-order batches
  without guessing from tick counters.【F:server/hub.go†L617-L664】【F:server/messages.go†L13-L35】【F:client/patches.js†L720-L964】【F:client/__tests__/patches.test.js†L1-L520】
* ✅ **Replay validation tooling** – the diagnostics drawer surfaces patch
  baseline ticks, applied patch counts, error summaries, and entity totals by
  reading from the background patch state, letting QA compare snapshot and diff
  pipelines without inspecting the console.【F:client/index.html†L288-L341】【F:client/main.js†L401-L620】
* ✅ **Keyframe recovery** – the server journals recent snapshots alongside patch
  batches, exposes them via `keyframeSeq` references plus a `keyframeRequest`
  websocket flow, and the client consumes those frames to heal missing-entity
  diffs without console noise.【F:server/hub.go†L600-L720】【F:server/main.go†L200-L360】【F:client/patches.js†L900-L1158】【F:client/network.js†L640-L820】
* ✅ **Switch-over rehearsal** – the render loop can now target either the
  authoritative snapshots or the patch-driven state. Console helpers
  (`debugSetRenderMode`, `debugToggleRenderMode`, or `store.setRenderMode`)
  flip the mode at runtime, share a centralised enum so diagnostics stay in sync,
  and the renderer reads from the patch container for players, NPCs, effects,
  and ground items when patch mode is active so QA can smoke test the diff
  pipeline without code edits.【F:client/main.js†L13-L314】【F:client/render.js†L1-L618】【F:client/render-modes.js†L1-L24】
* ✅ **Patch-first broadcasts** – steady-state `state` messages now rely on journalled diffs with configurable keyframe intervals, and the client mirrors patch baselines when snapshots are omitted. 【F:server/main.go†L35-L122】【F:server/hub.go†L707-L1078】【F:client/network.js†L557-L1218】

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

The issue arises from how the client rebuilds its authoritative baseline between
keyframes. When a new `state` message arrives without a full snapshot:

1. `updatePatchState` reuses the cached keyframe referenced by `keyframeSeq` as
   the working baseline.【F:client/patches.js†L1268-L1445】
2. Patches are applied against this static baseline rather than the cumulative
   world state. The baseline’s `sequence` remains frozen, so deduplication may
   skip legitimate updates and any properties omitted from the patch (for
   example, position during a facing change) revert to the cached keyframe
   values.【F:client/patches.js†L1510-L1640】
3. The renderer reads this partially updated view and interpolates toward stale
   coordinates until the next positional patch or keyframe arrives, producing
   the visible rewind and delayed or missing ability effects.【F:client/network.js†L1221-L1259】

This is an architectural flaw in the client’s patch-replay model, not a server
serialization error. The server correctly emits monotonic sequences and
consistent keyframe pointers.

### Supporting regression

`client/__tests__/patches.test.js` now includes
`it("regresses to the cached keyframe when cadence skips snapshots", …)` which:

1. Seeds a keyframe baseline and advances one player via a positional patch.
2. Applies a facing-only patch while reusing the same `keyframeSeq`.
3. Asserts that the player’s position regresses to the keyframe coordinates
   before advancing again once a new positional patch arrives.【F:client/__tests__/patches.test.js†L829-L919】

The test reliably reproduces the rewind-return cycle in isolation and guards
against accidental regression masking.

### Next steps

* Rework patch replay to maintain a cumulative, entity-scoped baseline instead of
  cloning from the last keyframe for each message.
* Treat missing entities in patch batches as keyframe recovery candidates rather
  than hard errors.
* Advance the baseline’s sequence with each successful patch application so
  deduplication compares against the latest state, not the static keyframe.

These adjustments should eliminate positional flicker, restore ability-effect
visibility under high cadence, and bring patch-only playback to parity with
snapshot-every-tick behaviour.
