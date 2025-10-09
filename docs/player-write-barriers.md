# Player Write Barrier Coverage

This note documents which pieces of player state are currently funneled through
write barrier helpers on the server and which fields are still mutated
in-place throughout the simulation code.

## State that already flows through write barriers

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

## State still mutated directly

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

When new write barriers are introduced for the fields above, the existing direct
assignments will need to be routed through the new helpers so versioning and
patch emission continue to stay in sync.

## Prioritising future write barriers

We eventually want every player mutation that surfaces in outbound snapshots to
flow through a write barrier so we can derive patches instead of resending full
state dumps. Not every field carried on `playerState` needs that treatment,
though, and sequencing the work will keep the change manageable:

* **Client-facing actor fields** – Anything that ships as part of the
  authoritative `Actor` snapshot should ultimately move behind a setter so
  deltas can be journaled. At the moment the remaining `playerState` fields that
  reach the client are limited to the inventory, health, facing, position, and
  intents we have already covered.【F:server/player.go†L1-L116】【F:server/player.go†L151-L175】
  If we ever add more HUD-visible attributes (for example, an exposed cooldown
  tracker), we should introduce a dedicated setter at the same time so patches
  stay consistent.
* **Server-only simulation metadata** – Fields like path progress, last input
  timestamps, and heartbeat bookkeeping never leave the server or influence the
  derived `Player` snapshot.【F:server/player.go†L151-L175】【F:server/player_path.go†L18-L168】【F:server/simulation.go†L239-L315】
  These can remain direct mutations for now because patching them would not
  unlock network savings. If any of this data becomes relevant to clients (for
  example, exposing next waypoint data for debugging), we can wrap the new
  outward-facing representation in a barrier without moving the internal
  bookkeeping structures.
* **Condition and cooldown maps** – The condition map already powers gameplay
  decisions on the server and could eventually influence remote visuals. The
  cooldown table behaves similarly, gating ability reuse and affecting several
  regression tests.【F:server/effects.go†L320-L367】【F:server/player.go†L97-L175】 Both
  should move behind helpers once we are ready to stream these values to the
  client. Until then, the churn is purely server-side so the benefit of a write
  barrier is limited to consistency checks.

As a result, we do **not** need to wrap every `playerState` field immediately.
Focusing on the client-visible attributes keeps the barrier surface area small
while we iterate on patch ingestion. Once the client can consume those patches
reliably, we can expand outward to any remaining data we decide to expose.
