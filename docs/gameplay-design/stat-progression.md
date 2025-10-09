# **Exponential-Decay Boost System — Mechanics Spec (v1)**

> Defines the deterministic, item-based progression system replacing XP and levels.
> All power growth derives from crafted consumables that permanently modify player attributes.

---

## 1. Overview

Progression in *Mine & Die* is purely **material**.
Players advance by **consuming crafted boosters** — not by earning XP or skill points.

Each booster increases a target stat by an amount that **decays exponentially** as more of that stat is accumulated, ensuring smooth, diminishing returns and infinite scalability without caps or prestige resets.

---

## 2. Core Formulae

### 2.1 Increment

Each consumed booster adds a value determined by:

[
\text{Increment} = \text{base_gain} \times r^{n}
]

where:

* `base_gain` — effect of the first dose
* `r` — global decay ratio (≈ **0.9**)
* `n` — current stack count for that stat

### 2.2 Cumulative Total

After *n* doses:

[
\text{Total} = \text{base_gain} \times \frac{1 - r^{n}}{1 - r}
]

This creates a curve that approaches an asymptotic limit, ensuring **infinite but tapering growth**.

---

## 3. Booster Definition

| Field           | Type  | Description                                                                  |
| --------------- | ----- | ---------------------------------------------------------------------------- |
| **base_gain**   | Float | Base stat gain of the first dose.                                            |
| **potency**     | Int   | Number of stack units consumed by this dose (higher tiers accelerate decay). |
| **target_stat** | Enum  | One of: Might, Resonance, Focus, Speed.                                      |
| **rarity/tier** | Enum  | Determines crafting cost and trade value only.                               |

---

## 4. Server Logic

Each player state includes a dictionary:

```go
stacks[stat] → integer
bonus[stat]  → float
```

On **consume(booster)**:

1. Read `n = stacks[target_stat]`.
2. Compute `increment = base_gain * r^n`.
3. Add `increment` to `bonus[target_stat]`.
4. Increase `stacks[target_stat] += potency`.
5. Destroy the consumed item.

All modifiers are **persistent** until player death or full wipe.

---

## 5. Example Tooltips

```
Elixir of Might
Base Gain: +3.5
Potency: 2
Decay Ratio: 0.90
Current Stacks (Might): 7
→ This Dose Adds: +1.71
→ New Total: +12.34 Might
```

Tooltips are deterministic — all values shown are computed from player state, not hidden rolls.

---

## 6. Progression Dynamics

* Early doses grant **large jumps** in power; later doses taper smoothly.
* Because each stat decays independently, **cross-investment** remains viable.
* Items with higher **potency** advance the decay curve faster, favoring specialization over breadth.

### Example Progression (r = 0.9, base = 3.0)

| Dose | Increment | Total |
| ---- | --------- | ----- |
| 1    | 3.00      | 3.00  |
| 2    | 2.70      | 5.70  |
| 3    | 2.43      | 8.13  |
| 4    | 2.19      | 10.32 |
| 5    | 1.97      | 12.29 |

---

## 7. Item Classes

Booster items follow the **crafting specification**; each is a high-tier **Advanced Material** with functional effect *and* economic role.

| Item Type               | Stat      | Base Gain | Potency | Example Inputs (see Crafting Spec)      |
| ----------------------- | --------- | --------- | ------- | --------------------------------------- |
| **Elixir of Might**     | Might     | +3.5      | 2       | Greater Healing Potion + Demon Heart    |
| **Elixir of Resonance** | Resonance | +3.0      | 2       | Warding Circle Chalk + Imp Eye          |
| **Tincture of Focus**   | Focus     | +2.0      | 1       | Herb Extract + Rat Tail + Spectral Dust |
| **Elixir of Speed**     | Speed     | +3.0      | 2       | Tail Resin + Nightshade + Slime Gel     |

Each recipe ties back into existing crafting loops, ensuring boosters remain resource-sink anchors in the economy.

---

## 8. Integration with Stat System

* Stats and effects are defined in **Stat Taxonomy — Core Attributes (v2.1)**.
* Boosters increase the **base value** of the relevant attribute.
* Derived stats (HP, Mana, Accuracy, etc.) automatically recalc from new base values.

---

## 9. Economic Interaction

* Boosters are **consumables** produced through advanced crafting.
* All materials come from **ecosystem drops** or **player trade**; no NPC vendors.
* Demand is continuous and sink-driven: every death resets progression, creating perpetual consumption.
* Scarcity and decay ratio together stabilize power inflation — no hard caps, but asymptotic costs.

See **Economy — Mechanics Spec (v1)** for persistence and taxation rules affecting booster trade and crafting materials.

---

## 10. Balance Parameters (Server-Configurable)

| Parameter                | Default | Range     | Description                               |
| ------------------------ | ------- | --------- | ----------------------------------------- |
| **r (decay ratio)**      | 0.90    | 0.85–0.95 | Global diminishing-return coefficient     |
| **base_gain multiplier** | 1.0     | per-item  | Adjusts all booster strength              |
| **potency scaling**      | 1       | per-item  | Determines how quickly decay accelerates  |
| **stack cap**            | none    | optional  | Hard limit, if desired for capped servers |

---

## 11. Design Principles

1. **Material Progression Only:** All growth originates from crafted items.
2. **Deterministic Math:** No RNG in progression or effect magnitude.
3. **Diminishing Returns:** Exponential decay enforces balance without arbitrary limits.
4. **Cross-System Coupling:** Boosters tie directly into crafting, economy, and stat systems.

---

**Result:**
A closed-loop, mathematically elegant leveling system that replaces XP and skill trees with tangible, tradable items.
Progression feels continuous and earned, every death re-anchors value into the economy, and balance emerges naturally from exponential decay rather than imposed limits.
