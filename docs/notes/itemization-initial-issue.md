# Proposed Issue: Define Canonical Item Schema Types on the Server

## Goal
Establish the deterministic item data structures required by the itemization plan so that future work (stats, crafting, economy) can operate on a unified model.

## Motivation
- The current server inventory only tracks a handful of placeholder items with minimal metadata (`Type`, `Name`, `Description`).【F:server/inventory.go†L6-L55】
- The gameplay plan requires a richer schema covering item class, equip slots, stackability, modifiers, and actions.【F:docs/gameplay-design/itemization-and-equipment.md†L9-L108】
- Milestone 1 calls for an "Item schema" and equip hooks before downstream systems can evolve.【F:docs/project-milestones.md†L8-L23】

## Suggested Scope
- Introduce new Go enum types for `ItemClass`, `EquipSlot`, and `ItemAction` that exactly match the taxonomy enumerated in the design doc (seven equip slots, full action list) so downstream code can rely on fixed string/JSON surfaces.
- Define a canonical `ItemDefinition` struct including fields for tier, stackable flag, fungibility key, equip slot, modifier list, and available actions. `Stackable` must only allow stacking when both the flag is `true` **and** two instances share the same `FungibilityKey`.
- Provide a constructor or builder that enforces the fungibility rules (e.g., `fungibility_key` composition) and validates enum inputs.
- Embed a deterministic `modifiers[]` slice (e.g., stable struct with `Type`, `Magnitude`, `Duration` fields even if initially empty) so future equip hooks and stat systems can extend the payload without schema churn.
- Provide serialization helpers so definitions can be shared with the client via catalog fetch or compile-time embedding (explicitly avoid per-tick payload growth).
- Replace the existing `itemCatalog` with seed data representing at least one weapon, armor piece, accessory, consumable, coating/enchant, throwable, and processed material using the new schema.
- Add unit tests to confirm:
  - JSON/Go encoding of definitions is deterministic (stable field and slice order).
  - Invalid enum values are rejected when constructing definitions.
  - Inventory stacking logic respects both `Stackable` and `FungibilityKey` equality.
  - Round-trip serialization/deserialization preserves struct contents.
- Document the schema in `docs/gameplay-design/itemization-and-equipment.md` if additional clarifications are needed (e.g., enum naming) and describe the migration path for presentation fields.

## Out of Scope
- Client UI changes or inventory rendering updates.
- Implementing equipment effects or stat modifications.
- Crafting, loot drops, or economy integrations.
- Network message consumption of new fields (beyond serialization helpers/tests). Keep schema sharing limited to catalog fetch or compile-time embedding to avoid bloating per-tick state snapshots.

## Acceptance Criteria
- New schema types exist with GoDoc comments referencing the design doc.
- Legacy presentation fields (`Name`, `Description`) are marked as deprecated and called out as legacy UI shims—the canonical schema should become the source of truth once the new UI lands.
- Tests under `server/` cover schema validation, serialization determinism, round-tripping, and stacking logic updates.
- Repository documentation reflects the canonical enum names so future tasks (equipping, crafting) can reference them.
