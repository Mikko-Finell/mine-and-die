# ⚔️ Combat System — Mechanics Specification (v1)

> Defines the real-time combat mechanics governing all interactions between actors in *Mine & Die*.
> The system is deterministic in simulation and probabilistic in resolution, integrating with the existing stat, item, and progression frameworks.

---

## 1. Overview

Combat is continuous and real-time.
All attacks, defenses, and effects evolve through discrete simulation ticks, but outcomes are expressed as smooth, uninterrupted motion and interaction.
Every combat action is defined by **timing**, **resolution**, and **resource** components.

---

## 2. Timing Model

Each action (attack, spell, block, dodge) consists of three temporal phases:

| Phase        | Description                                                  | Primary Modifiers                              |
| ------------ | ------------------------------------------------------------ | ---------------------------------------------- |
| **Wind-up**  | Commitment period before execution.                          | Weapon or spell base time; reduced by *Speed*. |
| **Active**   | Window during which the action can connect or apply effects. | Focus for precision; Speed for cast rate.      |
| **Recovery** | Lockout before another action may start.                     | Reduced by *Speed*.                            |

Actions are continuous; overlapping phases from multiple actors create the flow of combat.
No global turn structure exists.

---

## 3. Resolution Model

Every offensive action resolves through a probabilistic contest between **Accuracy** and **Evasion**, producing one of three outcomes: *Hit*, *Graze*, or *Miss*.

### 3.1 Hit Probability

* Accuracy is derived from the attacker’s **Focus** and the inherent precision of the weapon or ability.
* Evasion is derived from the defender’s **Speed** and any temporary bonuses (dodge, concealment, aura).
* The difference between the two values maps to a **bounded probability curve** rather than a binary threshold.
* Equal Accuracy and Evasion result in a high hit probability (~75–85 %), not 50 %.
* Small stat differences yield smooth, incremental changes in hit rate.
* A narrow *graze band* surrounds the threshold, allowing partially successful strikes with reduced damage.

### 3.2 Criticals

Critical hits represent enhanced precision and force.
Critical chance scales primarily with **Focus**, and critical magnitude scales modestly with **Might** or weapon class.

---

## 4. Damage Determination

### 4.1 Base Power

The starting value comes from the attacking item or ability:

* **Physical Power** = Weapon base × (1 + Might scaling).
* **Magical Power** = Spell base × (1 + Resonance scaling).

### 4.2 Mitigation

Defensive reduction applies by channel:

* **Armor** mitigates physical damage using a diminishing-return curve.
* **Resistance** mitigates magical damage by an analogous curve.
* No defense can reach total immunity; returns flatten asymptotically.

### 4.3 Modifiers

Final damage after mitigation is multiplied by:

* Outcome modifier (Hit = 1.0, Graze ≈ 0.5, Crit ≈ 1.5).
* Active effects such as coatings, auras, buffs, curses, or faction bonuses.
* A narrow bounded random factor (~±3 %) to desynchronize identical hits without introducing volatility.

---

## 5. Stagger and Poise

Each attack carries an **Impact** value proportional to weapon class and *Might*.
Each defender possesses **Poise**, derived from armor weight and *Might*.

When Impact exceeds Poise, the defender is staggered—canceling wind-up or recovery phases but never completed actions.
This creates short, deterministic openings for follow-up attacks.

---

## 6. Resources and Tempo

Combat actions consume and regenerate physical and magical resources.

| Resource    | Consumed By                      | Primary Scaling        | Regeneration Scaling |
| ----------- | -------------------------------- | ---------------------- | -------------------- |
| **Stamina** | Physical attacks, dodges, blocks | *Might* (capacity)     | *Speed* (recovery)   |
| **Mana**    | Spells, channels, auras          | *Resonance* (capacity) | *Focus* (recovery)   |

Attack and cast speeds, recovery times, and cooldown reductions are all continuous functions of *Speed* (tempo) and *Focus* (control).
There are no discrete “haste” breakpoints; scaling is smooth.

---

## 7. Status and Over-Time Effects

Status effects—poison, bleed, burn, slow, regeneration, aura fields—are deterministic in timing and tick at fixed intervals.

* **Intensity** scales with the attacker’s relevant power stat (*Might* or *Resonance*).
* **Duration** scales with *Focus*.
* **Resistance** uses the same mitigation curves as direct damage.
* Effects of the same type stack additively up to configured limits.
* All tick intervals align to the global simulation rate.

---

## 8. Probabilistic Events

Randomness exists only in:

* Hit/graze/miss resolution,
* Critical and status-proc rolls,
* Minor bounded damage variation.

All probabilities are derived from fixed analytic curves based on stat differentials.
No hidden rolls or variable seed rates exist; identical state produces identical expected results.

---

## 9. Interaction Hierarchy

1. **Initiation** – action declared, resources reserved.
2. **Wind-up** – vulnerable commitment period.
3. **Active** – contact check and probability resolution.
4. **Damage and Effects** – mitigation, modifiers, DoT applications.
5. **Recovery** – cooldown and stamina/mana regeneration.

This cycle repeats continuously across all actors, forming the combat simulation.

---

## 10. Integration Summary

* **Might** governs physical output, health, and resistance to stagger.
* **Resonance** governs magical output and mana capacity.
* **Focus** governs accuracy, control, and duration of sustained effects.
* **Speed** governs tempo, movement, evasion, and rate of recovery.
* Equipment defines base power, armor, resistances, and impact classes.
* Consumables and crafted items provide transient modifiers within the same framework.

---

**Result:**
Combat in *Mine & Die* is a continuous probabilistic-deterministic system.
Outcomes are computed from actor stats, equipment parameters, and stateful resources, producing consistent mechanical results without discrete turns or opaque randomness.
