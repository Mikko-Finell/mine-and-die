# **Itemization & Equipment — Item Classes (v1.2)**

> Defines the canonical item classes, their equip surfaces, functional roles, and market identity for *Mine & Die*.
> The framework unifies combat, crafting, and economy under a single deterministic item model.

---

## 1) Global Structure

All in-game objects share a single deterministic schema:

| Field               | Description                                                                                        |
| ------------------- | -------------------------------------------------------------------------------------------------- |
| **type**            | One of the classes below.                                                                          |
| **tier**            | Integer defining relative material and trade value.                                                |
| **stackable**       | Whether identical items merge into stacks.                                                         |
| **fungibility_key** | Item type + tier + quality tags. Defines stack identity and market buy-order grouping.             |
| **equip_slot**      | Slot for equippable items (`MainHand`, `OffHand`, `Head`, `Body`, `Gloves`, `Boots`, `Accessory`). |
| **modifiers[]**     | Deterministic effects on actor stats or abilities.                                                 |
| **actions[]**       | Verbs the item introduces or enables (`attack`, `block`, `consume`, `apply`, `throw`, `place`).    |
| **recycle_value**   | Fraction of crafting inputs recoverable when dismantled (default = 0.75).                          |

All items are physical and tradable unless placed or consumed.
Stacking and trade rely entirely on deterministic attributes — no hidden quality states.

---

## 2) Item Classes

### 2.1 Weapon

Weapons define the actor’s offensive capability.
They scale with **Might** for physical or **Resonance** for magical archetypes.

| Property    | Description                                                                                     |
| ----------- | ----------------------------------------------------------------------------------------------- |
| **Slots**   | `MainHand` (one-handed) or `MainHand+OffHand` (two-handed).                                     |
| **Surface** | Base power, impact class, critical multiplier, and optional elemental or conditional modifiers. |
| **Actions** | `attack`, optional `special` or `charge`.                                                       |

Subtypes use tags (not subclasses): *dagger, sword, axe, bow, tome, staff*, etc.

---

### 2.2 Shield

Provides directional defense and counter-pressure.

| Property    | Description                                                      |
| ----------- | ---------------------------------------------------------------- |
| **Slot**    | `OffHand`.                                                       |
| **Surface** | Block window, poise bonus, physical and elemental resist values. |
| **Actions** | `block(hold/timed)`, `bash`.                                     |

---

### 2.3 Armor

Protective equipment defining passive mitigation and stat reinforcement.

| Property    | Description                                                                                                |
| ----------- | ---------------------------------------------------------------------------------------------------------- |
| **Slots**   | `Head`, `Body`, `Gloves`, `Boots`.                                                                         |
| **Surface** | Armor rating, resistances by damage type, and auxiliary modifiers such as HP flat, poise, or regeneration. |

Each piece contributes independently; there are no set bonuses unless defined in crafting data.

---

### 2.4 Accessory

Augments the player with supportive or thematic bonuses.

| Property    | Description                                                                                        |
| ----------- | -------------------------------------------------------------------------------------------------- |
| **Slot**    | `Accessory` (multiple concurrent slots if configured).                                             |
| **Surface** | Effects such as regeneration, duration scaling, evasion, or conditional bonuses versus enemy tags. |
| **Actions** | Optional `activate` for time-limited passives or toggles.                                          |

---

### 2.5 Coating / Enchantment

Applies a permanent elemental or conditional property to a weapon or armor piece.

| Property        | Description                                                                          |
| --------------- | ------------------------------------------------------------------------------------ |
| **Application** | `apply(source_item, target_item)`; host item gains additional modifiers permanently. |
| **Persistence** | Coatings and enchantments remain until the item is recycled or overwritten.          |
| **Surface**     | Typical effects: on-hit damage type, penetration, resistance bonus, or aura field.   |
| **Stacking**    | One active coating or enchantment per host. Applying a new one replaces the old.     |

Coatings are treated as crafted **Advanced Materials** and trade independently on the market.

---

### 2.6 Consumable

Single-use items that produce an immediate or permanent effect.

| Property        | Description                                                                                                     |
| --------------- | --------------------------------------------------------------------------------------------------------------- |
| **Stackable**   | Yes.                                                                                                            |
| **Actions**     | `consume()` or `apply(target)` if directional.                                                                  |
| **Examples**    | Healing potions, elixirs, draughts, tinctures, ritual powders, thrown vials.                                    |
| **Integration** | Permanent boosters modify base stats (see *Stat Progression*). Others grant temporary buffs, heals, or debuffs. |

Ritual items follow this same model—activated from inventory and consumed on use.

---

### 2.7 Throwable

Projectile consumables that deliver area or on-impact effects.

| Property      | Description                                              |                                                                                              |
| ------------- | -------------------------------------------------------- | -------------------------------------------------------------------------------------------- |
| **Stackable** | Yes.                                                     |                                                                                              |
| **Actions**   | `throw(arc                                               | line)` resulting in deterministic area effects such as poison clouds, smoke, or fire bursts. |
| **Surface**   | Defines radius, duration, and status parameters by tier. |                                                                                              |

---

### 2.8 Trap / Placeable

Deployable items that create temporary world entities with reactive behavior.

| Property        | Description                                                                            |
| --------------- | -------------------------------------------------------------------------------------- |
| **Stackable**   | Yes in inventory; unique once placed.                                                  |
| **Actions**     | `place(tile)`; world entity responds to `trigger(actor)` according to defined script.  |
| **Persistence** | Exists until triggered or dismantled; dismantling returns materials per recycle ratio. |

Examples: caltrops, snares, warding circles.

---

### 2.9 Block / Structure Piece

Physical construction element for player-built fortifications.

| Property        | Description                                     |
| --------------- | ----------------------------------------------- |
| **Stackable**   | Yes.                                            |
| **Actions**     | `place/remove/repair` (material cost).          |
| **Surface**     | Hit points, resistances, and placement tags.    |
| **Persistence** | Remains in world until destroyed or dismantled. |

---

### 2.10 Container

Inventory-extending or world-placed storage.

| Property        | Description                                    |
| --------------- | ---------------------------------------------- |
| **Stackable**   | No.                                            |
| **Actions**     | `open/lock/unlock/place/pickup`.               |
| **Surface**     | Capacity and access control parameters.        |
| **Persistence** | Retains contents until destroyed or reclaimed. |

---

### 2.11 Tool

Implements that enable world interactions.

| Property     | Description                                                                                |
| ------------ | ------------------------------------------------------------------------------------------ |
| **Slots**    | Usually `MainHand`.                                                                        |
| **Actions**  | `mine(node)`, `harvest(corpse)`, or similar.                                               |
| **Function** | Grants access to the associated action; outcome and yield are deterministic per node type. |

---

## 3) Slot Summary

| Category  | Slot(s)                             |
| --------- | ----------------------------------- |
| Weapons   | MainHand / MainHand+OffHand         |
| Shield    | OffHand                             |
| Armor     | Head / Body / Gloves / Boots        |
| Accessory | Accessory (configurable count)      |
| Tool      | MainHand                            |
| Others    | N/A (used or placed from inventory) |

---

## 4) Market Representation

| Item Kind                                                                       | Market Behavior                                                                                                               |
| ------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- |
| **Fungible** (plain gear, consumables, coatings, materials, blocks, throwables) | Eligible for buy orders using `fungibility_key`.                                                                              |
| **Non-fungible** (uniquely enchanted or otherwise modified gear)                | Listed as individual sell items with serialized modifier data.                                                                |
| **Permanent coatings**                                                          | Traded separately as standalone items prior to application. Once applied, the host item’s identity updates deterministically. |

---

## 5) Example Table

| Item                    | Class           | Slot     | Stack | Example Modifiers / Actions   |
| ----------------------- | --------------- | -------- | ----- | ----------------------------- |
| Iron Dagger             | Weapon          | MainHand | Yes   | +Physical Damage, +Impact     |
| Fire-Forged Sword       | Weapon (ench.)  | MainHand | No    | +Fire Damage on Hit           |
| Kite Shield             | Shield          | OffHand  | No    | +Block Window, +Poise         |
| Leather Jerkin          | Armor           | Body     | Yes   | +Armor, +Bleed Resist         |
| Warding Circle Chalk    | Trap/Plac.      | —        | Yes   | place → area resistance aura  |
| Elixir of Fortitude     | Consumable      | —        | Yes   | consume → +Max HP             |
| Poison Glob             | Throwable       | —        | Yes   | throw → AoE Poison            |
| Stone Block             | Block           | —        | Yes   | place → obstacle entity       |
| Mining Pick             | Tool            | MainHand | No    | mine(node)                    |
| Venom Enchantment Stone | Coating/Enchant | —        | Yes   | apply → weapon +Poison on Hit |

---

**Result:**
This model defines every physical and functional item class used in *Mine & Die*.
All systems—crafting, combat, economy, and world simulation—operate over this unified, deterministic item layer.
Permanent coatings eliminate fractional item states, maintain clean market fungibility, and preserve a stable closed-loop material economy.
