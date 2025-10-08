# ⚒ Game-Economy & Faction Hierarchy Specification

## 1. Gold Resource System

### 1.1 Deposits

* The world contains **finite gold-mine deposits**.
* Each deposit has a fixed **capacity** (total gold units).
* When its capacity reaches 0, the deposit is permanently exhausted and disappears.
* New deposits may appear at engine-defined intervals or through procedural world-growth events, but total global gold remains scarce.

### 1.2 Mining

* Any player standing within mining range of an active deposit may perform a **mining action** that:

  * removes `x` gold from the deposit (reducing its remaining capacity),
  * adds `x` gold to the miner’s carried inventory,
  * triggers the automatic **tax flow** described in § 3.

### 1.3 Ownership & Control

* Deposits have **no built-in ownership** flag.

  * Access control or defense is purely physical (player enforcement).
  * The engine does not reserve deposits for any Faction or player.

### 1.4 Transport & Loss

* Gold exists as a **physical item** in inventory.
* On player death:

  * The entire inventory, including gold, drops on the ground as lootable items.
  * Nothing is deleted or protected by the system.

---

## 2. Safe Zones & Market

### 2.1 Safe Zones

* Certain map tiles are designated as **safe zones**.

  * PvP combat and item damage are disabled inside these zones.
  * Items can be transferred freely between players here.

### 2.2 Marketplace Interface

* All players can **view** the global market data (listings and prices) from anywhere.
* **Creating or fulfilling** market orders requires physical presence inside a safe zone.
* The engine provides:

  * Item listing (sell orders).
  * Optional buy orders for **fungible** resources (same type & quality).
* The marketplace acts only as an **escrow and matching service**.

  * Gold and items are transferred directly between player inventories when a match occurs.
  * No system-owned gold or items are created or destroyed.

---

## 3. Faction Hierarchy System

### 3.1 Terminology

* The player organization is called a **Faction**.
* A Faction is a rooted **hierarchical tree** of members.

### 3.2 Roles

Default rank structure (engine-provided):

```
King  →  Noble  →  Knight  →  Citizen
```

All members except the King have exactly one **superior** node.

### 3.3 Authority

* The **King** possesses unrestricted executive control over the entire Faction tree:

  * May promote, demote, or reassign any member to any rank.
  * May configure **tax percentages** for each rank-to-superior step.
* Any **superior** node (at any rank) has management rights over its **direct subordinates**:

  * May promote or demote them within the immediate next rank below its own level.
  * May reassign subordinates among its children.

### 3.4 Taxation Flow

* When a player receives gold by any method (mining, trade, loot, etc.), the system automatically routes a percentage of that gold upward through the hierarchy:

  Example with configured rates:

  ```
  Citizen → Knight : 10 %
  Knight  → Noble  : 5 %
  Noble   → King   : 2 %
  ```
* Taxes are transferred instantly at the time of income.
* All taxed gold is drawn from the transaction amount (not duplicated).
* Tax rates are stored as Faction parameters editable by the King.

### 3.5 Succession by Kill

* If a player kills another player who is **above them within their Faction chain**:

  1. The killer is immediately assigned to the victim’s former rank and position.
  2. All subordinates and their tax streams are reassigned to the killer.
  3. The victim is removed from the Faction (status = dead).
* Killing across unrelated branches or outside the Faction has no hierarchical effect.

### 3.6 Membership Changes

* Joining: a member is attached as a subordinate to any existing member authorized to accept recruits.
* Leaving: voluntary departure or removal deletes the member’s node; subordinates are automatically reattached to the departing member’s superior.

---

## 4. Item & Resource Framework

### 4.1 Material Categories

* **Primary materials:** wood, stone, herbs, hides, basic ores—abundant, low value.
* **Refined materials:** smelted metals, processed hides, potions—require primary materials.
* **Gold:** universal currency item, finite supply.

### 4.2 Gathering & Crafting

* Gathering actions yield primary materials; no system limit other than player time.
* Crafting consumes materials to create higher-tier items; recipes define inputs → outputs.
* Crafted items are tradable through the marketplace or directly between players.

---

## 5. Combat & Loot

* PvP combat can occur anywhere except safe zones.
* On death:

  * The defeated player’s entire inventory is dropped as lootable items.
  * No experience, levels, or stats are modified by the engine beyond death state.
* The killer gains whatever they can physically pick up.

---

## 6. Persistence & Scarcity

* Gold and items persist in the world until looted or decayed by standard item-lifetime rules.
* No routine deletion or minting occurs.
* Global scarcity and redistribution emerge from mining depletion, trade, taxation, and PvP loss.

---

**Result:**
These mechanics define a closed-loop, player-governed economy with finite monetary supply, full-loot PvP, and hierarchical taxation.  All political and social behavior—alliances, coups, governance style, or enforcement—is emergent from player actions rather than system enforcement.
