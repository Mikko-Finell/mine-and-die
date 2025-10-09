# Patch Migration Plan

We are incrementally transitioning the Mine & Die client/server sync model from
full-state broadcasts to diff-based patches. This document tracks the current
coverage, the client-side scaffolding introduced for testing, and the next steps
required before the patch pipeline can replace the legacy snapshot flow.

## Current Server Coverage

### Player write barrier coverage

* **Position (`Actor.X`, `Actor.Y`)** – committed via `World.SetPosition`, which
  verifies that the coordinates changed, bumps the per-player version, and emits
  a position patch for clients.【F:server/world_mutators.go†L14-L43】 The main
  simulation resolves movement against scratch copies and then calls
  `applyPlayerPositionMutations` to publish the final coordinates through the
  setter so the journal stays authoritative.【F:server/simulation.go†L330-L357】【F:server/simulation.go†L410-L434】
* **Facing (`Actor.Facing`)** – updated with `World.SetFacing`, which clamps the
  direction, increments the version, and records a facing patch.【F:server/world_mutators.go†L45-L75】
  Both direct player input handling and the path follower invoke this helper so
  facing changes continue to produce patches.【F:server/simulation.go†L239-L258】【F:server/player_path.go†L81-L82】
* **Intent vectors (`playerState.intentX`, `playerState.intentY`)** – updated via
  `World.SetIntent`, which skips invalid vectors, bumps the version, and records
  an intent patch for the journal.【F:server/world_mutators.go†L78-L110】 Player
  command handling routes through this setter so intent changes stay journaled
  alongside facing and position.【F:server/simulation.go†L239-L315】 The path
  follower likewise normalizes its movement updates through the write barrier so
  server-driven navigation emits consistent patches.【F:server/player_path.go†L18-L155】
* **Health (`Actor.Health`)** – adjusted through `World.SetHealth`, which clamps
  to `[0, MaxHealth]`, increments the version, and appends a health patch that
  mirrors the new hit points and max health.【F:server/world_mutators.go†L113-L156】
  Damage and healing effects call this setter whenever the target is a tracked
  player so write barriers capture the change.【F:server/effects.go†L205-L228】
* **Inventory (`Actor.Inventory`)** – wrapped by `World.MutateInventory`, which
  clones the inventory, executes the provided mutation, rolls back on error, and
  appends an inventory patch if anything actually changed.【F:server/world_mutators.go†L123-L156】
  Hub flows such as `drop_gold` and `pickup_gold` mutate player inventories via
  this helper.【F:server/hub.go†L360-L495】
* **Version counter (`playerState.version`)** – only incremented inside the
  setters above, ensuring authoritative snapshots line up with the mutation
  journal.【F:server/world_mutators.go†L30-L147】

### Player state still mutated directly

The remaining player fields are still updated in place by simulation systems and
hub flows. They currently bypass write barriers and therefore do not emit
patches or version bumps:

* **Path tracking (`playerState.path` fields)** – recalculation and completion
  logic rewrites the struct directly when managing goals, indices, and arrival
  radius.【F:server/player_path.go†L88-L168】
* **Input timestamps (`playerState.lastInput`)** – assigned whenever a movement
  or path command is processed so diagnostics can show recent activity.【F:server/simulation.go†L259-L315】
* **Heartbeat metadata (`playerState.lastHeartbeat`, `playerState.lastRTT`)** –
  recorded directly on heartbeat commands and when a subscriber reconnects to a
  player slot.【F:server/simulation.go†L287-L293】【F:server/hub.go†L169-L190】
* **Cooldown timers (`playerState.cooldowns`)** – lazily populated and updated
  in the ability helpers to enforce ability reuse delays.【F:server/effects.go†L341-L367】
* **Condition map (`actorState.conditions`)** – populated, refreshed, and cleaned
  up by the condition system when status effects apply or expire.【F:server/conditions.go†L87-L158】
* **Scratch movement (`actorState.X`, `actorState.Y`)** – movement integration
  still adjusts actor copies directly while resolving collisions before the
  results are written back through `SetPosition`. These adjustments never touch
  the authoritative map entries directly but are worth noting when auditing the
  pipeline.【F:server/movement.go†L6-L102】【F:server/simulation.go†L331-L359】

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
* Vitest coverage now freezes inputs to guard against mutation, asserts
  idempotent replay counts, validates monotonic tick handling, and exercises the
  resync pathway so future patch types can extend the pipeline with
  confidence.【F:client/__tests__/patches.test.js†L1-L216】

## Suggested next steps

1. **Expand patch coverage** – add NPC, effect, and ground item patch handlers on
   the client once the server emits them, mirroring the player helper structure.
2. **Replay validation tooling** – surface the background patch state in the
   diagnostics drawer so QA can compare snapshot-vs-diff outputs without opening
   the console.
3. **Patch sequence plumbing** – expose per-batch sequence numbers and explicit
   resync markers in the websocket payload so the client dedupe cache can rely on
   authoritative metadata instead of inferred ticks.
4. **Keyframe recovery** – plumb the server's journal keyframes through to the
   client and teach the patch runner to resynchronise from a full snapshot when a
   diff references an unknown entity.
5. **Switch-over rehearsal** – gate the render loop behind a feature flag that
   can swap between full snapshots and the patch-driven state to smoke test the
   final migration path.
