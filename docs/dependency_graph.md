# Server System Dependency Graph

The table below summarizes how the major server-side systems described in the architecture docs depend on one another. Dependencies highlight which packages must expose clean interfaces before we can peel systems into standalone modules; rows with a dependency of `—` mark current leaf nodes.

| System | Dependency | Reason |
| --- | --- | --- |
| Items & Catalog | — | Item definitions only use the Go standard library, so the data model stands alone.【F:server/items.go†L1-L46】 |
| Stats Engine | — | The stats package depends solely on math/sort helpers, making it self-contained today.【F:server/stats/stats.go†L1-L49】 |
| Inventory | Items & Catalog | Inventory merges stacks by calling `ItemDefinitionFor` to validate fungibility keys from the catalog.【F:server/inventory.go†L41-L66】 |
| Equipment | Items & Catalog | Equipment stores `ItemStack` entries and inspects item modifiers when equipping gear.【F:server/equipment.go†L12-L61】【F:server/equipment.go†L159-L180】 |
| Equipment | Stats Engine | Equipping translates item modifiers into `stats.StatDelta` payloads, so the stat component must be available.【F:server/equipment.go†L3-L8】【F:server/equipment.go†L159-L180】 |
| Ground Items & Economy | Items & Catalog | Ground stacks backfill fungibility keys from the item catalog before merging quantities.【F:server/ground_items.go†L71-L110】 |
| Combat & Damage | Inventory | Mining rewards and combat loot mutate inventories through the world’s inventory helper.【F:server/effects.go†L135-L154】 |
| Combat & Damage | Items & Catalog | Combat routines spawn gold using the canonical `ItemTypeGold` identifier.【F:server/effects.go†L135-L153】 |
| Status Effects | Effects Contract Runtime | Burning’s `OnApply` handler enqueues status visuals via the unified effect manager.【F:server/status_effects.go†L46-L67】 |
| Effects Contract Runtime | World Core / Simulation | The effect manager owns a world handle and invokes world hooks to resolve melee, projectiles, and status visuals.【F:server/effects_manager.go†L13-L34】【F:server/effects_manager.go†L376-L433】 |
| AI | Combat & Damage | AI ability lookups map directly to combat effect identifiers such as melee swings and fireballs.【F:server/ai_executor.go†L16-L19】 |
| AI | World Core / Simulation | AI execution iterates NPC state from the world and schedules commands each tick.【F:server/ai_executor.go†L21-L93】 |
| Movement & Pathfinding | World Obstacles | Movement helpers clamp against obstacle geometry supplied by the world configuration.【F:server/movement.go†L5-L91】 |
| World Core / Simulation | Stats Engine | Each tick resolves actor stats and clamps health via the stat component.【F:server/simulation.go†L93-L114】 |
| World Core / Simulation | Inventory | Player inventory mutations flow through the world mutator to emit patches.【F:server/world_mutators.go†L237-L250】 |
| World Core / Simulation | Equipment | Equipment mutations similarly rely on the world mutator for versioning and patches.【F:server/world_mutators.go†L253-L268】 |
| World Core / Simulation | Effects Contract Runtime | The world embeds the effect manager, effect behaviors, and projectile templates to drive combat.【F:server/simulation.go†L64-L76】 |
| World Core / Simulation | Status Effects | Status effect definitions live on the world for per-actor state transitions.【F:server/simulation.go†L75-L76】 |
| World Core / Simulation | AI | The world holds the compiled AI library to drive NPC behaviour each tick.【F:server/simulation.go†L64-L83】 |
| World Core / Simulation | Ground Items & Economy | Ground item maps live on the world, so drops and pickups run through this state.【F:server/simulation.go†L88-L90】 |
| Networking & Hub | World Core / Simulation | The hub owns the world instance, stages commands, and wires telemetry for transport.【F:server/hub.go†L24-L156】 |

Leaf systems ready for extraction include the **Items & Catalog** and **Stats Engine**, since they currently expose no intra-repo dependencies beyond the standard library.【F:server/items.go†L1-L46】【F:server/stats/stats.go†L1-L49】
