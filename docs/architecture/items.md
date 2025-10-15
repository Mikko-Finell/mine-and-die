# Item System Architecture

The item system defines the canonical data that downstream equipment, crafting, and economy features consume. The Go server own
s item definitions, enforces fungibility-aware stacking, and exposes deterministic catalog data so the client and tooling can s
tay aligned.

## Responsibilities
1. **Canonical schema** – Provide an authoritative `ItemDefinition` model with stable enums (`ItemClass`, `EquipSlot`, `ItemAct
ion`) that mirror the gameplay taxonomy. [server/items.go](../../server/items.go)
2. **Fungibility management** – Compose deterministic fungibility keys so inventories, ground drops, and future markets agree o
n when items stack. [server/items.go](../../server/items.go)
3. **Catalog export** – Marshal definitions into a deterministic JSON payload for catalog embedding or fetch-based delivery wit
hout polluting per-tick snapshots. [server/items.go](../../server/items.go)
4. **Runtime integration** – Keep inventory slots and world ground stacks consistent with schema rules while preserving legacy 
presentation fields until the UI migrates. [server/inventory.go](../../server/inventory.go) [server/ground_items.go](../../server/ground_items.go)

## Canonical Schema
- `server/items.go` declares enum sets for classes, equip slots, and item actions. Construction helpers validate identifiers aga
inst those sets so downstream systems can rely on a closed surface area. [server/items.go](../../server/items.go)
- `ItemDefinition` captures:
  - Structural metadata (`Class`, `EquipSlot`, `Stackable`, `Actions`).
  - Modifiers in a deterministic order so later stat and equip systems can consume a stable payload. [server/items.go](../../server/items.go)
  - Legacy presentation (`Name`, `Description`) maintained only for the current UI; contributors should migrate clients to cons
ume schema-driven rendering when available. [server/items.go](../../server/items.go)
- `NewItemDefinition` centralizes validation:
  - Rejects unknown enum values and missing equip slots for wearable classes.
  - Sorts and deduplicates actions and modifiers.
  - Applies default recycle values and copies input slices to maintain immutability guarantees. [server/items.go](../../server/items.go)
- `ComposeFungibilityKey` builds colon-delimited keys from identifiers, tiers, and sorted quality tags so identical items collap
se while bespoke instances remain distinct. [server/items.go](../../server/items.go)

## Definition Catalog
- `server/item_catalog.go` registers the authoritative definition map at module init time and exposes lookups via `ItemDefiniti
ons()` / `ItemDefinitionFor()`. [server/item_catalog.go](../../server/item_catalog.go)
- The seed catalog covers at least one weapon, armor, accessory, consumable, coating, throwable, and processed material entry s
o serialization, stacking, and equip metadata receive end-to-end exercise. [server/item_catalog.go](../../server/item_catalog.go)
- Future tooling can call `MarshalItemDefinitions` to export the catalog in deterministic order for client fetch or compile-time
 embedding. [server/items.go](../../server/items.go)

## Runtime Integration
### Inventories
- `ItemStack` tracks both `ItemType` and `FungibilityKey`. `Inventory.AddStack` backfills missing keys from the definition, reje
cts mismatches, and only merges stacks when the definition is stackable **and** the keys match. [server/inventory.go](../../server/inventory.go)
- Slot operations (`MoveStacks`, `Drain`, `RemoveQuantity`) propagate stored keys so later merges remain consistent across the h
ub, player state, and networking payloads. [server/inventory.go](../../server/inventory.go)

### Equipment
- `Equipment` maintains a deterministic slice of `EquippedItem` entries ordered by slot, enabling snapshots and patches to emit stable payloads. [server/equipment.go](../../server/equipment.go)
- `equipmentDeltaForDefinition` maps canonical item modifiers onto `stats.LayerEquipment`, allowing equipping and unequipping to feed through the existing stat engine without bespoke maths. [server/equipment.go](../../server/equipment.go) [server/world_equipment.go](../../server/world_equipment.go)
- `World.MutateEquipment` centralises patch-aware equipment mutations for both players and NPCs so downstream flows (death drops, scripted swaps) do not need separate helpers. [server/world_mutators.go](../../server/world_mutators.go)
- `World.EquipFromInventory`/`World.UnequipToInventory` remove items from inventories, update the equipment container, adjust stats, and emit patches; helper commands `equip_slot` / `unequip_slot` surface the flow for diagnostics via the console handler. [server/world_equipment.go](../../server/world_equipment.go) [server/hub.go](../../server/hub.go)
- Death drops now drain equipped slots alongside inventories so ground stacks reflect the complete loadout state. [server/ground_items.go](../../server/ground_items.go)

### Ground Items
- `World.upsertGroundItem` indexes stacks by fungibility key, auto-populating keys from the catalog and merging counts in place 
when definitions permit stacking. [server/ground_items.go](../../server/ground_items.go)
- Ground state mirrors inventory semantics so snapshot consumers observe consistent stack behaviour regardless of source.

## Testing
- `server/items_test.go` guards constructor validation, fungibility key ordering, deterministic JSON encoding, and round-trip serialization of definitions. [server/items_test.go](../../server/items_test.go)
- `server/inventory_test.go` exercises stacking based on the combined `stackable` flag and fungibility key while ensuring helper operations preserve stored keys. [server/inventory_test.go](../../server/inventory_test.go)

## Extension Roadmap
- **Client schema consumption** – Decide whether to serve the catalog via HTTP or embed it during build so the UI stops reading legacy presentation fields directly.
- **Equipment passives** – Layer triggered abilities and conditional bonuses on top of the existing modifier plumbing.
- **Catalog authoring** – Expand coverage beyond the seed set and consider external authoring sources if hand-written definitions become cumbersome.
