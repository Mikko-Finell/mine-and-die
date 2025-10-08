**Crafting & Material Dependency Specification**

* **No artificial gating** (no skills, professions, or facilities).
* **Every craft has direct utility**, not filler progression.
* **Crafted items recycle** with partial loss.
* **All incentives emerge from scarcity and use-value**, not arbitrary design layers.

---

# ⚗️ Crafting & Material Dependency Specification

## 1. Overview

This specification defines the **crafting and material transformation system**.
All players possess identical crafting capability: if the required materials are held, the craft can be performed.
Progression emerges solely from **resource access, trade leverage, and risk tolerance**, not from character level, profession, or recipe unlocking.

The system is designed to maintain **functional purpose for every crafted item**, **recursive material circulation**, and **organic economic interdependence** without NPC mediation.

---

## 2. Material Classes

### 2.1 Primary Materials

* Acquired directly from the environment, creatures, or mining deposits.
* Abundant but regionally constrained.
* Form the base inputs for most transformations.

**Examples:** herbs, ore, wood, tails, fangs, hides.

### 2.2 Processed Materials

* Created by simple transformation or combination of primaries.
* Typically more portable, stable, or potent.
* Serve as the foundation for higher-impact items but also retain direct value in crafting and trade.

**Examples:** extracts, metals, powders, leather, potions.

### 2.3 Advanced Materials

* Produced by combining processed goods with rare or high-risk ingredients.
* Provide tangible gameplay effects such as combat buffs, healing, or enhanced weapon performance.
* Continue to rely on early-tier inputs to preserve economic continuity.

**Examples:** elixirs, poisons, coatings, enchanted reagents.

### 2.4 Catalysts

* Low-tier materials with broad applicability across recipes.
* Ensure sustained demand for early-game drops.

**Examples:** Rat Tails, Herbs, Resin.

---

## 3. Crafting Model

### 3.1 Definition

A recipe is a simple transformation rule:

```
Inputs → Output (+ Effect)
```

| Field            | Type                   | Description                                                                     |
| ---------------- | ---------------------- | ------------------------------------------------------------------------------- |
| **Inputs**       | List[MaterialQuantity] | Materials required for crafting.                                                |
| **Output**       | List[MaterialQuantity] | Resulting items.                                                                |
| **Effect**       | Text                   | The direct functional purpose of the output (e.g., heal, buff, poison, repair). |
| **RecycleValue** | Float[0–1]             | Fraction of materials recoverable if recycled or dismantled (default = 0.75).   |

Crafting is instantaneous and deterministic: if the inputs exist, the output is produced.

---

## 4. Design Principles

### 4.1 Universal Access

* Every player may craft any item without restriction.
* There are no crafting skills, tools, or facilities beyond possession of the required inputs.
* Scarcity, geography, and risk form the only progression barriers.

### 4.2 Functional Output Only

* Every craft must produce an item with immediate, intrinsic purpose:

  * Consumables that restore, buff, or cure.
  * Poisons or coatings that temporarily alter weapons.
  * Enhancements that modify performance in combat or gathering.
* No intermediate or placeholder goods exist purely for further crafting or resale.

  * Economic interdependence arises because **higher-tier effects** require **lower-tier functional items** as ingredients.

### 4.3 Recursive Dependency

Higher-tier items reuse earlier outputs as reagents or catalysts, maintaining enduring relevance across all play stages.

Example:

```
[Herb] + [Rat Tail] → Lesser Healing Potion
[Lesser Healing Potion] + [Imp Eye] → Greater Healing Potion
[Greater Healing Potion] + [Demon Heart] + [Herb] → Elixir of Fortitude
```

Here, *Herbs* and *Rat Tails* remain permanent economic anchors.

### 4.4 Recycling

* Any crafted item can be **dismantled** to reclaim materials.
* Standard recovery = 75 % of original inputs, rounded down.
* This ensures resource circulation and prevents permanent accumulation of obsolete goods.

### 4.5 Determinism

Crafting outcomes are fixed; there are no random bonuses or quality rolls.
All economic variance derives from material scarcity and player valuation, not RNG.

---

## 5. Economic Properties

### 5.1 Tangibility

All materials and crafted items are physical, droppable, and lootable.
They obey the same decay and persistence rules as any other object.

### 5.2 Market Dynamics

Because all crafts produce items with direct gameplay function, **demand is consumption-driven**:

* Combat creates constant need for potions, coatings, and repairs.
* Risky environments increase consumption rates, reinforcing regional trade loops.
* Shortages of low-tier materials propagate upward through crafting dependencies.

### 5.3 Organic Specialization

Although no formal professions exist, players naturally diverge into roles based on access and preference:

* **Gatherers** — harvest abundant primaries for profit.
* **Crafters** — transform materials near safe zones for steady margin.
* **Adventurers** — consume or trade crafted goods in high-risk zones.
* **Traders** — exploit regional price differentials.
  All roles remain player-driven and reversible at any time.

---

## 6. Recycling Procedure

1. Player selects an eligible crafted item.
2. System calculates recoverable materials = ⌊ Input × RecycleValue ⌋.
3. Item is destroyed; recovered materials returned to inventory.
4. No experience or bonuses are granted — recycling is purely material reclamation.

---

## 7. Regional Variation (Optional)

* Resource distribution differs by biome or zone, creating localized supply/demand imbalances.
* No artificial trade restrictions are imposed.
* Transport risk and faction control determine real market boundaries.

---

## 8. Persistence & Balance

* Items persist physically until looted or decayed.
* No automated minting or deletion occurs.
* The system is **closed-loop**: all value flows through player action — gathering, crafting, consumption, recycling, and loss.

---

## 9. Example Recipes (initial concept)

The following examples illustrate a compact, recursive crafting web with **direct-use outputs**, **cross-chain coupling**, and **no artificial gates**.
All items are physical, droppable, and recyclable at **75%** of consumed inputs (rounded down).

### 9.1 Base Consumables (Tier-1)

| Item                      | Inputs               | Output     | Effect (Immediate Use)                                                          | Typical Sources                |
| ------------------------- | -------------------- | ---------- | ------------------------------------------------------------------------------- | ------------------------------ |
| **Herb Extract**          | 2× Herb              | 1× Extract | Consume: minor regeneration for 10s. Also used as a catalyst in higher recipes. | Herb: fields/forest (gather)   |
| **Lesser Healing Potion** | 1× Herb, 1× Rat Tail | 1× Potion  | Consume: restore modest health instantly.                                       | Rat Tail: rats (sewers/plains) |

### 9.2 Processed Reagents (Usable *and* Versatile)

| Item              | Inputs                       | Output   | Effect (Immediate Use)                                                                                  | Typical Sources                              |
| ----------------- | ---------------------------- | -------- | ------------------------------------------------------------------------------------------------------- | -------------------------------------------- |
| **Tail Resin**    | 1× Rat Tail, 1× Herb Extract | 1× Resin | Throw (small radius): 3s slow (“sticky”). Also extends duration of coatings when used as an ingredient. | —                                            |
| **Obsidian Dust** | 1× Obsidian Shard            | 2× Dust  | Sprinkle on weapon: next 10 hits gain +armor penetration.                                               | Obsidian Shard: lava fields/volcano (gather) |

### 9.3 Potions, Elixirs, Coatings, and Throwables

| Item                       | Inputs                                                          | Output       | Effect (Immediate Use)                                                 | Notes                                                                         |
| -------------------------- | --------------------------------------------------------------- | ------------ | ---------------------------------------------------------------------- | ----------------------------------------------------------------------------- |
| **Greater Healing Potion** | 1× Lesser Healing Potion, 1× Imp Eye, 1× Herb                   | 1× Potion    | Consume: large instant heal.                                           | Keeps Tier-1 items in demand. Inputs: Imp Eye (imps, lava caves)              |
| **Elixir of Fortitude**    | 1× Greater Healing Potion, 1× Demon Heart, 1× Herb, 1× Rat Tail | 1× Elixir    | Consume: +max-health buff for 10 min; removes one recent wound effect. | Demon Heart: demons (dungeons/volcanic depths)                                |
| **Venom Coating**          | 1× Nightshade, 1× Rat Tail, 1× Herb Extract                     | 1× Coating   | Apply to weapon: on-hit DoT for 5 min (or 50 hits).                    | Nightshade: nocturnal herb (graveyards/forest at night)                       |
| **Sharpening Paste**       | 2× Obsidian Dust, 1× Slime Gel                                  | 1× Paste     | Apply to weapon: +base damage for 5 min (or 50 hits).                  | Slime Gel: slimes (sewers/swamps)                                             |
| **Poison Glob**            | 1× Venom Coating, 1× Slime Gel                                  | 1× Throwable | Throw: small splash, 8s poison DoT; allies unaffected.                 | Consumes a coating as an input (direct-use recursion).                        |
| **Smoke Bomb**             | 1× Smoke Powder, 1× Rat Tail                                    | 1× Bomb      | Throw: 5s line-of-sight break; enemies lose target lock briefly.       | Smoke Powder: 1× Ash Cap + 1× Slime Gel → 1× Powder. Ash Cap: volcanic fungus |

### 9.4 Weapons and Armor (Recursive Bases)

| Item                     | Inputs                                            | Output        | Effect (Immediate Use)                                              | Typical Sources                                  |
| ------------------------ | ------------------------------------------------- | ------------- | ------------------------------------------------------------------- | ------------------------------------------------ |
| **Iron Dagger**          | 2× Ore, 1× Wood, 1× Rat Tail                      | 1× Dagger     | Equip: basic melee weapon.                                          | Ore: mines/caves (gather); Wood: forest (gather) |
| **Leather Jerkin**       | 2× Wolf Hide, 1× Rat Tail, 1× Herb Extract        | 1× Jerkin     | Equip: light armor; +bleed resistance.                              | Wolf Hide: wolves (forest)                       |
| **Fire-Forged Sword**    | 1× Iron Dagger, 1× Imp Eye, 1× Obsidian Shard     | 1× Sword      | Equip: adds fire damage on hit (minor burn).                        | Reuses Tier-1 base; cross-tier inputs            |
| **Demonbane Greatsword** | 1× Fire-Forged Sword, 1× Demon Heart, 2× Rat Tail | 1× Greatsword | Equip: heavy weapon; bonus vs demons; on-hit holy flare (cooldown). | Preserves Rat Tail demand in late game           |

### 9.5 Placeables, Rituals, and Traps

| Item                     | Inputs                                           | Output    | Effect (Immediate Use)                                                  | Typical Sources                                                   |
| ------------------------ | ------------------------------------------------ | --------- | ----------------------------------------------------------------------- | ----------------------------------------------------------------- |
| **Warding Circle Chalk** | 1× Bone Shard, 1× Herb Extract, 1× Obsidian Dust | 1× Chalk  | Place: 60s ground aura; +elemental resistance to allies in radius.      | Bone Shard: skeletons (ruins)                                     |
| **Hex Fetish**           | 1× Spectral Dust, 1× Rat Tail, 1× Nightshade     | 1× Fetish | Activate: next successful hit applies 20s armor-shred curse.            | Spectral Dust: ghosts (graveyards)                                |
| **Caltrop Kit**          | 1× Iron Bar, 1× Tail Resin                       | 1× Trap   | Place: slows and applies minor bleed to first 3 enemies stepping on it. | Iron Bar: 2× Ore → 1× Bar (also throwable spike if used directly) |
| **Sticky Snare**         | 1× Spider Silk, 1× Tail Resin                    | 1× Trap   | Place: roots first enemy for 2s; then 6s slow.                          | Spider Silk: spiders (caves)                                      |

---

### 9.6 Dependency Highlights

* **Early items persist upward:** Rat Tail and Herb recur in *Greater Healing Potion*, *Elixir of Fortitude*, *weapons*, *rituals*, and *traps*.
* **Processed reagents remain useful on their own:** *Herb Extract*, *Tail Resin*, and *Obsidian Dust* are direct-use consumables **and** versatile ingredients.
* **Cross-chain coupling:** *Obsidian Dust* links weapon damage (Sharpening Paste) with defensive rituals (Warding Chalk). *Venom Coating* is both a buff and an input to *Poison Glob*.
* **No filler crafts:** Every output confers an immediate advantage (healing, damage, control, escape, or defense).
* **Recycling:** Any listed item can be dismantled to reclaim **75%** of its inputs, ensuring material recirculation and limiting dead stock.

---

**Result:**
This specification defines a **self-contained, economically coherent crafting framework** with no artificial progression systems.
Every item has practical use, every resource retains value through recursion, and every transaction contributes to the continuous material circulation that underpins the world economy.
