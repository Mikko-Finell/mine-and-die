# Item System Architecture

The item system defines the canonical data that downstream equipment, crafting, and economy features consume. The Go server own
s item definitions, enforces fungibility-aware stacking, and exposes deterministic catalog data so the client and tooling can s
tay aligned.

## Responsibilities
1. **Canonical schema** – Provide an authoritative `ItemDefinition` model with stable enums (`ItemClass`, `EquipSlot`, `ItemAct
ion`) that mirror the gameplay taxonomy.【F:server/items.go†L15-L144】
2. **Fungibility management** – Compose deterministic fungibility keys so inventories, ground drops, and future markets agree o
n when items stack.【F:server/items.go†L101-L198】
3. **Catalog export** – Marshal definitions into a deterministic JSON payload for catalog embedding or fetch-based delivery wit
hout polluting per-tick snapshots.【F:server/items.go†L200-L206】
4. **Runtime integration** – Keep inventory slots and world ground stacks consistent with schema rules while preserving legacy 
presentation fields until the UI migrates.【F:server/inventory.go†L5-L218】【F:server/ground_items.go†L1-L140】

## Canonical Schema
- `server/items.go` declares enum sets for classes, equip slots, and item actions. Construction helpers validate identifiers aga
inst those sets so downstream systems can rely on a closed surface area.【F:server/items.go†L15-L144】
- `ItemDefinition` captures:
  - Structural metadata (`Class`, `EquipSlot`, `Stackable`, `Actions`).
  - Modifiers in a deterministic order so later stat and equip systems can consume a stable payload.【F:server/items.go†L147-L1
98】
  - Legacy presentation (`Name`, `Description`) maintained only for the current UI; contributors should migrate clients to cons
ume schema-driven rendering when available.【F:server/items.go†L147-L198】
- `NewItemDefinition` centralizes validation:
  - Rejects unknown enum values and missing equip slots for wearable classes.
  - Sorts and deduplicates actions and modifiers.
  - Applies default recycle values and copies input slices to maintain immutability guarantees.【F:server/items.go†L147-L198】
- `ComposeFungibilityKey` builds colon-delimited keys from identifiers, tiers, and sorted quality tags so identical items collap
se while bespoke instances remain distinct.【F:server/items.go†L101-L141】

## Definition Catalog
- `server/item_catalog.go` registers the authoritative definition map at module init time and exposes lookups via `ItemDefiniti
ons()` / `ItemDefinitionFor()`.【F:server/item_catalog.go†L1-L142】
- The seed catalog covers at least one weapon, armor, accessory, consumable, coating, throwable, and processed material entry s
o serialization, stacking, and equip metadata receive end-to-end exercise.【F:server/item_catalog.go†L51-L142】
- Future tooling can call `MarshalItemDefinitions` to export the catalog in deterministic order for client fetch or compile-time
 embedding.【F:server/items.go†L200-L206】

## Runtime Integration
### Inventories
- `ItemStack` tracks both `ItemType` and `FungibilityKey`. `Inventory.AddStack` backfills missing keys from the definition, reje
cts mismatches, and only merges stacks when the definition is stackable **and** the keys match.【F:server/inventory.go†L5-L120】
- Slot operations (`MoveStacks`, `Drain`, `RemoveQuantity`) propagate stored keys so later merges remain consistent across the h
ub, player state, and networking payloads.【F:server/inventory.go†L168-L218】

### Equipment
- `Equipment` maintains a deterministic slice of `EquippedItem` entries ordered by slot, enabling snapshots and patches to emit stable payloads.【F:server/equipment.go†L10-L120】
- `equipmentDeltaForDefinition` maps canonical item modifiers onto `stats.LayerEquipment`, allowing equipping and unequipping to feed through the existing stat engine without bespoke maths.【F:server/equipment.go†L122-L145】【F:server/world_equipment.go†L19-L112】
- `World.MutateEquipment` centralises patch-aware equipment mutations for both players and NPCs so downstream flows (death drops, scripted swaps) do not need separate helpers.【F:server/world_mutators.go†L230-L272】
- `World.EquipFromInventory`/`World.UnequipToInventory` remove items from inventories, update the equipment container, adjust stats, and emit patches; helper commands `equip_slot` / `unequip_slot` surface the flow for diagnostics via the console handler.【F:server/world_equipment.go†L19-L112】【F:server/hub.go†L438-L520】
- Death drops now drain equipped slots alongside inventories so ground stacks reflect the complete loadout state.【F:server/ground_items.go†L195-L232】

### Ground Items
- `World.upsertGroundItem` indexes stacks by fungibility key, auto-populating keys from the catalog and merging counts in place 
when definitions permit stacking.【F:server/ground_items.go†L1-L140】
- Ground state mirrors inventory semantics so snapshot consumers observe consistent stack behaviour regardless of source.

## Testing
- `server/items_test.go` guards constructor validation, fungibility key ordering, deterministic JSON encoding, and round-trip serialization of definitions.【F:server/items_test.go†L1-L55】
- `server/inventory_test.go` exercises stacking based on the combined `stackable` flag and fungibility key while ensuring helper operations preserve stored keys.【F:server/inventory_test.go†L1-L95】

## Extension Roadmap
- **Client schema consumption** – Decide whether to serve the catalog via HTTP or embed it during build so the UI stops reading legacy presentation fields directly.
- **Equipment passives** – Layer triggered abilities and conditional bonuses on top of the existing modifier plumbing.
- **Catalog authoring** – Expand coverage beyond the seed set and consider external authoring sources if hand-written definitions become cumbersome.
