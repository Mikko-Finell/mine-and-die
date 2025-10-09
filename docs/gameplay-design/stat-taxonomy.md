# **Stat Taxonomy — Core Attributes (v2.1)**

> Defines the four primary attributes governing physical and magical performance.
> All attributes are numeric, physical, and derived solely from player-consumed items or equipment.
> There is no XP or skill progression — all capability is material and reversible through death.

---

## 1. Overview

Each player has four **primary attributes**.
Two determine **power type**, two provide **shared utility**.

| Stat          | Domain               | Primary Role                                            |
| ------------- | -------------------- | ------------------------------------------------------- |
| **Might**     | Physical Power       | Physical damage scaling, Health pool                    |
| **Resonance** | Magical Power        | Spell and aura scaling, Mana pool                       |
| **Focus**     | Discipline & Control | Accuracy, block/parry timing, cast stability, durations |
| **Speed**     | Tempo & Reaction     | Movement, attack/cast speed, evasion                    |

All secondary and derived stats are deterministic functions of these four.

---

## 2. Attribute Definitions

### **Might**

* Increases **melee and ranged physical damage**.
* Increases **maximum Health (HP)**.
* Improves **stagger resistance** and **carry capacity**.
* No effect on spells or Mana.

### **Resonance**

* Increases **spell and healing power**.
* Increases **maximum Mana**.
* Governs **aura and buff intensity**.
* No effect on physical damage or Health.

### **Focus**

* Increases **accuracy** for both physical and magical actions.
* Improves **parry/block timing** and reduces execution variance.
* Increases **cast stability** and **resistance to interruption**.
* Extends **aura/buff/curse duration**.
* Synergizes with Resonance for **cooldown reduction**.

### **Speed**

* Increases **movement speed**, **attack rate**, and **cast speed**.
* Improves **evasion** (reduces hit probability).
* Reduces recovery and lockout times after actions.

---

## 3. Derived Attributes

| Derived Stat       | Function of         | Description                     |
| ------------------ | ------------------- | ------------------------------- |
| **Health (HP)**    | f(Might)            | Maximum physical endurance      |
| **Mana**           | f(Resonance)        | Maximum magical energy          |
| **Accuracy**       | f(Focus)            | Hit reliability for all attacks |
| **Evasion**        | f(Speed)            | Chance to avoid incoming hits   |
| **Cast Stability** | f(Focus)            | Resistance to channel break     |
| **Cast Speed**     | f(Speed)            | Spell execution rate            |
| **Cooldown Rate**  | f(Speed, Focus)     | Ability recovery efficiency     |

All functions are monotonic with diminishing returns.
No random factors are used in their evaluation.

---

## 4. Combat Resolution (Deterministic)

| Mechanic             | Formula (conceptual)           |
| -------------------- | ------------------------------ |
| **Hit Chance**       | attacker Focus vs target Speed |
| **Evasion**          | Speed-based threshold check    |
| **Block/Parry**      | Focus-based timing window      |
| **Cast Speed**       | base / Speed modifier          |
| **Interrupt Resist** | Focus threshold test           |

---

## 5. Economic Integration

* Attributes are raised via **permanent consumables** (Elixirs, Draughts, Tinctures).
* Diminishing returns follow a global **exponential decay curve** with ratio *r*.
* All attributes are **lost on death** and must be reacquired.
* Attributes are serialized in player state for deterministic combat and balance.

| Item                | Stat      | Tier | Base Gain | Potency |
| ------------------- | --------- | ---- | --------- | ------- |
| Elixir of Might     | Might     | 2    | +3.5      | 2       |
| Elixir of Resonance | Resonance | 2    | +3.0      | 2       |
| Tincture of Focus   | Focus     | 1    | +2.0      | 1       |
| Elixir of Speed     | Speed     | 2    | +3.0      | 2       |

---

## 6. Principles

1. **Independent Power Axes:** Might and Resonance scale physical and magical systems separately.
2. **Shared Utility:** Focus governs control; Speed governs tempo; both apply universally.
3. **Determinism:** All calculations are reproducible and RNG-free.
4. **Material Progression:** Every attribute increase originates from physical items.
