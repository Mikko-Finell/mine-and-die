# Simplify Initiative – Consolidated Priorities

The following themes aggregate every proposal captured in the individual `summary_*.md` snapshots within this folder. Each section keeps one canonical description per idea while reflecting the combined intent of the underlying notes.

## Most Common
- **Centralise effect spawn detection in `client/network.js`.** Replace the various `is*Spawn`, `is*Identifier`, and logging helpers with a shared matcher that gathers spawn candidates once and dispatches to per-effect predicates.
- **Unify player and NPC path-following.** Extract a shared navigation controller for the logic currently duplicated across `server/player_path.go` and `server/npc_path.go`, covering waypoint progression, stall detection, and intent updates.
- **Generalise inventory and equipment mutation helpers.** Collapse `mutateActorInventory`, `mutateActorEquipment`, and the related exported wrappers in `server/world_mutators.go` into a single clone/mutate/patch routine used by every actor type.
- **Share effect lifecycle normalisation utilities.** Deduplicate the camelCase fallbacks and parsing helpers so lifecycle batch handling is defined once and reused across the client modules that read effect payloads.

## Highest Impact
- **Build a shared actor path engine.** Merge the player and NPC state machines into a configurable engine so stall thresholds, waypoint advancement, and intent publication stay consistent for every actor.
- **Create a data-driven effect spawn pipeline.** Centralise spawn classification and logging in `client/network.js` to remove hundreds of duplicated lines and keep future transport schema changes in sync.
- **Standardise snapshot payloads and bootstrap defaults.** Define common structs for `joinResponse`, `stateMessage`, and `keyframeMessage`, and serve world configuration defaults from the authoritative server sources to prevent schema/config drift.
- **Unify inventory/equipment mutation flows.** Drive all actor inventory and equipment updates through one parameterised helper to guarantee consistent cloning, versioning, and patch emission.

## Easiest Change
- **Store `resyncPolicy` by value.** Update `server/resync_policy.go` and its callers to drop the optional pointer pattern and delete the unreachable `nil` checks.
- **Gate or remove the hub's always-on spawn logging.** Delete the per-tick substring scans in `server/hub.go`, or hide them behind a debug flag, to trim hot-path work and log noise.
- **Replace bespoke map copy helpers.** Swap `copyIntMap`/`copyBoolMap` and similar helpers for a shared generic implementation or `maps.Clone`.
- **Remove legacy camelCase effect lifecycle fallbacks.** Now that the server emits snake_case arrays, strip the redundant casing branches from the client payload readers.
- **Consolidate clamp helpers on the server.** Provide one shared clamp utility and reuse it across movement, stats, effects, and ground item code to eliminate scattered ad-hoc implementations.
