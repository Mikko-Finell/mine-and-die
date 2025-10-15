# Feature Wishlist

## Goblin Patrol Loot Diversion
Patrolling goblins should detect loose gold, temporarily divert to collect it, and then rejoin their original patrol path.

**Implementation outline**
- Extend the goblin patrol AI state machine to add a "collect loot" branch that can be entered from patrol when gold enters line of sight, including path recalculation and resume logic.
- Ensure the perception system publishes gold-on-ground events (position, amount, ownership) for AI consumers.
- Update persistence/network replication so server-authoritative loot pickups propagate to clients and respect patrol scheduling.
- Add QA coverage: unit tests for the new AI state transitions and an integration test validating patrol resume behavior after collection.

**Impacted systems**
- `server/ai_configs/goblin.json` for introducing the patrol-to-loot diversion state and its transition triggers in the compiled goblin behavior tree.
- `server/ai_executor.go` and `server/ai_library.go` to interpret the new collect-loot actions and expose condition hooks for spotting ground gold while patrolling.
- `server/npc_path.go` to add path goal handoffs that let patrol routes pause for loot collection and then resume the original waypoint sequence.
- `server/ground_items.go` and related `World` helpers in `server/simulation.go` for reserving, picking up, and broadcasting ground gold interactions initiated by NPCs.
- Client replication plumbing in `client/patches.js`, `client/network.js`, and `client/render.js` to keep the goblin's temporary path deviation and the disappearing loot stack synchronized for local prediction and visuals.

## Expanded Directional Facing
Increase the supported facing directions from four cardinal orientations to eight, covering diagonals for characters and NPCs.

**Implementation outline**
- Update shared movement and animation enums to represent eight-direction facing and propagate through serialization.
- Adjust character controller interpolation and rotation logic to snap or blend toward the nearest octant without jitter.
- Extend sprite/3D asset packs and animation state machines with diagonal frames, including tooling for export and validation.
- Audit combat hit arc calculations and pathfinding heuristics so new facings preserve gameplay balance and collision checks.

**Impacted systems**
- `server/movement.go` and `server/pathfinding.go` for adding eight-direction vector math, serialization, and movement constraint validation.
- `server/ai_library.go` and `server/ai_executor.go` to ensure behavior nodes and locomotion commands emit the new facing metadata.
- `client/render.js` and `client/main.ts` to drive sprite orientation, animation state selection, and prediction logic for diagonal facings.
- `client/vendor` animation assets alongside `client/styles.css` for supplying diagonal frames and updating atlas metadata consumed at runtime.

## Pursuit Facing Alignment
While chasing the player, goblins must continuously rotate to face the player before executing melee attacks so hit directionality remains correct.

**Implementation outline**
- Extend the chase AI behavior to request continuous facing updates toward the target, respecting existing pathing constraints.
- Refine the locomotion/rotation controller so goblins smoothly interpolate orientation while moving and gate melee attacks until alignment thresholds are met.
- Propagate facing data over the network to keep client-side prediction and hit validation in sync with server authority.
- Expand automated combat tests (or add new ones) to verify goblin melee attacks only trigger when facing alignment criteria are satisfied.

**Impacted systems**
- `server/ai_configs/goblin.json` for encoding the chase-orient behavior and its gating conditions in the compiled behavior tree.
- `server/movement.go` and `server/npc_path.go` to blend pursuit movement with continuous facing updates and to synchronize waypoint steering.
- `server/effects_manager.go` and `server/melee_command_pipeline_test.go` to enforce facing checks before melee intents execute.
- `client/network.js`, `client/patches.js`, and `client/render.js` to stream orientation updates, update prediction buffers, and align attack animations toward the player.

## Persistent Decals
Client rendering should retain permanent decals (e.g., blood splatter) without garbage collection removing them over time.

**Implementation outline**
- Refactor the client decal manager to separate permanent decals from pooled/transient instances, with explicit lifetime policies.
- Extend render graph resources (textures, buffers) to support persistent decal layers, including streaming, batching, and save/load considerations.
- Audit performance budgets: profile memory, draw calls, and GPU state changes to ensure permanent decals do not regress frame times.
- Backfill automated and manual test cases covering decal persistence across sessions, map reloads, and multiplayer synchronization.

**Impacted systems**
- `client/render.js` and `client/render-modes.js` to introduce persistent decal collections, render ordering, and draw call batching.
- `client/effect-manager-adapter.js` and `client/effect-lifecycle.js` to tag decal effects with permanence metadata and prevent pooling cleanup.
- `client/effect-diagnostics.js` for updating debugging overlays so engineers can inspect long-lived decals and performance counters.
- Frontend profiling harnesses under `client/__tests__/` to validate that decal retention does not introduce regressions across reloads.

## Arrow Attack Type
Introduce a dedicated `arrow` attack type for ranged combat balancing and future content.

**Implementation outline**
- Define the `arrow` attack enum/type within combat metadata and propagate through serialization contracts for both client and server.
- Hook projectile logic to reference the new type, including damage formulas, resistances, and effect hooks for AI reactions.
- Update combat logging, UI indicators, and analytics pipelines to recognize and display the arrow classification.
- Expand automated combat scenarios to cover arrow attacks, ensuring regression coverage for damage, status effects, and networking.

**Impacted systems**
- `server/effects.go`, `server/effects_manager.go`, and `server/effect_intents.go` to register the arrow attack type and resolve its damage pipelines.
- `server/world_equipment.go` and `server/items.go` to tag bow-class weapons and ammo with arrow metadata for loot generation.
- `client/effect-lifecycle-translator.js` and `client/effect-lifecycle.js` to surface arrow combat events, impact visuals, and resistances on the HUD.
- Telemetry surfaces in `server/telemetry.go` and analytics hooks in `client/main.ts` for logging arrow usage and balancing data.

## Equipment Slots
Add equipment slots for head, torso, gloves, boots, and accessory items to drive player progression and inventory depth.

**Implementation outline**
- Extend player and NPC inventory schemas with explicit slot metadata and persistence support.
- Adjust UI layouts and input bindings to surface the new slot structure, including tooltip updates and controller navigation.
- Update loot tables, item definitions, and drop generation to tag slot compatibility for existing and future gear.
- Add validation tests to confirm slot constraints, equip/unequip flows, and backward compatibility with legacy save data.

**Impacted systems**
- `server/inventory.go`, `server/equipment.go`, and `server/world_equipment.go` to add slot definitions, persistence fields, and validation rules.
- `server/item_catalog.go` and `server/items.go` so generated loot declares compatible slots and upgrade paths.
- `client/main.ts`, `client/input.js`, and UI components in `client/styles.css` to render the expanded equipment grid and interaction affordances.
- Save/load coverage in `server/world_mutators.go` and regression tests in `server/inventory_test.go` to protect existing characters.

## Item Equipping
Allow characters to equip items into the defined slots with proper stat application and validation.

**Implementation outline**
- Implement equip/unequip commands in gameplay controllers with server authority, client prediction, and rollback handling.
- Wire stat recalculation and buff/debuff hooks to trigger on equip events, ensuring UI refresh and combat recalibration.
- Integrate animation/state updates (e.g., visible gear) and audio cues when equipment changes occur.
- Provide regression tests covering equipping edge cases: invalid slots, duplicates, hot-swapping during combat, and multiplayer sync.

**Impacted systems**
- `server/equipment.go`, `server/player.go`, and `server/effects_manager.go` to process equip commands, recompute stats, and broadcast resulting buffs.
- `server/hub.go` and `server/messages.go` for RPC contracts that allow clients to request and receive equip state changes.
- `client/network.js`, `client/patches.js`, and `client/render.js` to apply equip deltas, trigger gear visuals, and refresh HUD stats.
- QA automation in `server/inventory_test.go` and end-to-end flows in `client/__tests__/` validating equip edge cases and rollback.
