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
* Vitest coverage now freezes inputs to guard against mutation, asserts
  idempotent replay counts, validates monotonic tick handling, and exercises the
  resync pathway so future patch types can extend the pipeline with
  confidence.【F:client/__tests__/patches.test.js†L1-L328】
* Patch batches now carry authoritative `seq` counters and explicit `resync`
  markers so the client can reset history and deduplicate against the server's
  metadata instead of inferring behaviour from tick values.【F:server/hub.go†L617-L664】【F:server/messages.go†L13-L35】【F:client/patches.js†L720-L964】【F:client/__tests__/patches.test.js†L1-L520】

## Completed steps

* ✅ **Expand patch coverage** – client-side NPC, effect, and ground item patch
  handlers mirror the server journals so replay validation covers every
  broadcast entity without console noise.【F:client/patches.js†L1-L828】【F:client/__tests__/patches.test.js†L1-L328】
* ✅ **Patch sequence plumbing** – state broadcasts now include monotonic
  sequence numbers plus a `resync` flag, and the client dedupe cache consumes
  those fields to discard duplicates and protect against out-of-order batches
  without guessing from tick counters.【F:server/hub.go†L617-L664】【F:server/messages.go†L13-L35】【F:client/patches.js†L720-L964】【F:client/__tests__/patches.test.js†L1-L520】

## Suggested next steps

1. **Replay validation tooling** – surface the background patch state in the
   diagnostics drawer so QA can compare snapshot-vs-diff outputs without opening
   the console.
2. **Keyframe recovery** – plumb the server's journal keyframes through to the
   client and teach the patch runner to resynchronise from a full snapshot when a
   diff references an unknown entity.
3. **Switch-over rehearsal** – gate the render loop behind a feature flag that
   can swap between full snapshots and the patch-driven state to smoke test the
   final migration path.
