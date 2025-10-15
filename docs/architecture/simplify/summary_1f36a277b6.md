# Most Common

- **Unify effect spawn detection in `client/network.js`.** Every survey highlights that helpers such as `isBloodSplatterSpawn`, `isFireSpawn`, `isAttackSpawn`, and their identifier/logging companions all repeat the same nested candidate gathering and predicate checks. Consolidating them into a parameterised utility or data-driven table would slash duplicated traversal logic and keep future payload tweaks consistent across the client.
- **Share the path-following state machine between players and NPCs.** Multiple notes point out that `server/player_path.go` and `server/npc_path.go` implement nearly identical waypoint progression, stall detection, and recalc logic. Extracting a common controller with actor-specific hooks would prevent behavioural drift and make tuning thresholds a single edit.
- **Collapse redundant inventory/equipment mutation wrappers.** Reviews of `server/world_mutators.go` repeatedly call out that `MutateInventory`, `MutateNPCInventory`, `mutateActorInventory`, and `mutateActorEquipment` all clone state, apply closures, and emit patches using near-identical guard rails. A shared helper or unified entrypoint would reduce boilerplate and keep mutation semantics aligned for every entity type.

# Highest Impact

- **Create a shared path-following engine for all actors.** Collapsing the duplicate state machines in `server/player_path.go` and `server/npc_path.go` would simplify stall handling, arrival radius tuning, and intent updates across every entity. It eliminates two large, parallel implementations and makes future navigation fixes propagate automatically.
- **Centralise effect spawn detection and logging.** Replacing the repeated `is*Spawn`, `shouldLog*`, and logging wrappers in `client/network.js` with a unified matcher+logger pipeline removes hundreds of lines of boilerplate and ensures future payload schema updates are reflected everywhere at once.
- **Source default world configuration from the server.** Wiring the client to consume the authoritative values already defined in `server/constants.go` / `server/world_config.go` prevents desyncs around entity counts and world dimensions, while letting balancing changes happen in one place instead of across disconnected hard-coded copies.

# Easiest Change

- **Make `resyncPolicy` a value instead of a pointer.** `newJournal` already initialises the policy, so flipping the methods in `server/resync_policy.go` and `server/patches.go` to value receivers lets us delete the impossible nil guards with minimal risk.
- **Remove always-on spawn logging in `server/hub.go`.** The `broadcastState` instrumentation currently scans for specific substrings each tick just to dump entire payloads; gating or deleting it reclaims hot-path cycles with a simple diff.
- **Replace `copyIntMap`/`copyBoolMap` with a generic helper.** Both `server/effects_manager.go` and `server/patches.go` reimplement the same map cloning logic; switching to a shared generic (or `maps.Clone`) is a straightforward refactor that trims redundant helpers immediately.
