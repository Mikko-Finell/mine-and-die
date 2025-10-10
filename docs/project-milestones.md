# Mine & Die — Project Milestones

_Companion to the [Product Management Plan](./project-plan.md) and its dependency graph (see Section 9)._

## 🧱 Milestone 1 — Core Stats & Itemization Backbone

**Goal:** establish the foundation for all progression and combat math.
**Depends on:** nothing.
**Includes:**

- Implement **Stat Taxonomy** (base + derived stats; serialization; zero-point balance)
- Implement **Item schema** (type, slot, modifiers, rarity, etc.)
- Basic **inventory container** system (server + client sync)
- Equip/unequip hooks that modify stats
- Minimal **test fixtures** for stats + items

**Playable outcome:** a character can equip items and see their stats change.

↳ Supports the "Itemization & Equipment" and "Stat Taxonomy" streams outlined in the [build order guidance](./project-plan.md#9-dependency-graph).

---

## 🔨 Milestone 2 — Crafting & Resource Loop

**Goal:** turn raw materials into usable items.
**Depends on:** Milestone 1 (item definitions).
**Includes:**

- Define **crafting recipes** and input/output item types
- Add **gathered resource items** (ore, herbs, reagents)
- Implement crafting UI stub or console command
- Hook item creation to inventory system
- Begin tracking **material sinks** and byproducts

**Playable outcome:** players can gather or spawn materials and craft basic gear/consumables.

↳ Extends the Crafting workstream and unlocks the economy objectives described in [Section 4 of the plan](./project-plan.md#4-strategic-objectives-next-two-quarters).

---

## 🧪 Milestone 3 — Stat Progression & Boost Items

**Goal:** introduce permanent progression via crafted elixirs/tinctures.
**Depends on:** Milestone 2 (crafting), Stat Taxonomy.
**Includes:**

- Implement exponential-decay progression formula
- Define booster items and their potency/target stat fields
- Track consumed boosters server-side
- Display updated stats post-consumption

**Playable outcome:** players can improve stats persistently through crafted boosters.

↳ Builds on the "Stat Progression" system in the [dependency graph](./project-plan.md#9-dependency-graph) and feeds the combat balancing roadmap.

---

## ⚔️ Milestone 4 — Combat MVP

**Goal:** basic deterministic real-time combat.
**Depends on:** Milestone 1 + 3.
**Includes:**

- Implement hit, evade, and damage equations using current stat model
- Add attack intent & target resolution loop
- Include weapon/armor effects and coatings integration
- Add test combat arena with a few NPCs
- Log damage events for later balancing

**Playable outcome:** two entities can fight and kill each other; basic balancing possible.

↳ Aligns with the Combat pillar prioritized in the plan's [suggested build order](./project-plan.md#9-dependency-graph).

---

## 💰 Milestone 5 — Economy & Market System

**Goal:** create the item circulation loop.
**Depends on:** Milestone 2 + 4.
**Includes:**

- Implement **gold mining / loot drops**
- Add **player trade / escrowed market** interface
- Define **item pricing, taxes, and destruction sinks**
- Hook in **crafting costs and repair fees**
- Minimal safe-zone handling for peaceful trading

**Playable outcome:** players can earn, trade, and spend gold; items have value.

↳ Directly supports the "Gold Economy" and "Safe Zones & Market" initiatives listed in [Section 5](./project-plan.md#5-execution-framework).

---

## 🏰 Milestone 6 — Factions & Tax Hierarchy

**Goal:** add the political-economic layer.
**Depends on:** Milestone 5.
**Includes:**

- Implement faction data model (hierarchy, leader, ranks)
- Wire tax routing from market & loot income
- Add basic promotion, inheritance, and overthrow mechanics
- Display faction earnings in a summary interface

**Playable outcome:** factions earn passive income from member activity.

↳ Advances the Faction Governance objectives captured in the plan's [strategic goals](./project-plan.md#4-strategic-objectives-next-two-quarters).

---

## 🔄 Milestone 7 — Balance & Integration Pass

**Goal:** make everything coherent and ready for wider playtests.
**Depends on:** all previous.
**Includes:**

- Normalize stat curves and crafting yields
- Re-tune combat damage equations
- Adjust market fees and faction tax rates
- Add data logging for player economy and combat analytics
- Define regression tests for each subsystem

**Playable outcome:** a self-contained alpha loop where players gather, craft, fight, trade, and grow stronger.

↳ Closes the loop envisioned for Milestone D (Faction Governance Alpha) and ensures roadmap alignment with the [plan metrics](./project-plan.md#7-metrics-for-success).
