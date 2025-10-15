# Feature Wishlist

## Goblin Patrol Loot Diversion
Patrolling goblins should detect loose gold, temporarily divert to collect it, and then rejoin their original patrol path.

**Implementation outline**
- Extend the goblin patrol AI state machine to add a "collect loot" branch that can be entered from patrol when gold enters line of sight, including path recalculation and resume logic.
- Ensure the perception system publishes gold-on-ground events (position, amount, ownership) for AI consumers.
- Update persistence/network replication so server-authoritative loot pickups propagate to clients and respect patrol scheduling.
- Add QA coverage: unit tests for the new AI state transitions and an integration test validating patrol resume behavior after collection.

## Expanded Directional Facing
Increase the supported facing directions from four cardinal orientations to eight, covering diagonals for characters and NPCs.

## Pursuit Facing Alignment
While chasing the player, goblins must continuously rotate to face the player before executing melee attacks so hit directionality remains correct.

## Persistent Decals
Client rendering should retain permanent decals (e.g., blood splatter) without garbage collection removing them over time.

## Arrow Attack Type
Introduce a dedicated `arrow` attack type for ranged combat balancing and future content.

## Equipment Slots
Add equipment slots for head, torso, gloves, boots, and accessory items to drive player progression and inventory depth.

## Item Equipping
Allow characters to equip items into the defined slots with proper stat application and validation.
