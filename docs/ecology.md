# World Ecology — Dynamic Terrain & Biome System (v1)

> Design-level description of how terrain changes over time in *Mine & Die*.
> No parameters or implementation details.

---

## 1. Overview

The world updates as a grid of **cells**.
Each cell looks at its neighbors and changes based on simple rules.
From these local updates, larger patterns form: forests, hot zones, and cave systems.
Player actions feed into the same rules.

---

## 2. Model

Each cell tracks a small state: material type, a notion of stored energy/fertility, and a recent-change marker.
Cells exchange influence with neighbors (e.g., heat spreads, fertility spreads, erosion smooths, plants expand).
Rules are local and deterministic:

* Same seed → same world.
* Simulation can run incrementally.
* Player actions act as disturbances to the same fields.

---

## 3. Example Biome Cycles

### Woodland

Moderate energy + high fertility → plants appear.
Light growth → grass; sustained fertility → bushes/trees.
Overcrowding, drought, or fire → regression to open ground.
This produces stable clusters and natural clearings.

### Volcanic

High energy concentrates → rock heats, melts, and deposits ash.
As energy diffuses, the area cools and hardens.
Later, fertility returns and vegetation can reappear.

### Subterranean

Cooling/compression after high-energy phases can create voids → caves.
Caves collect minerals and support distinct fauna.
Disturbance weakens them; collapse fills them back in over time.

---

## 4. Interactions at Boundaries

Borders between patterns are active.
Heat can trigger nearby fires; moisture/fertility can recover burned ground.
Edges (forest ↔ rock, lava ↔ soil, surface ↔ cave) are high-change zones.
No biome is permanent.

---

## 5. System Stability

To avoid lock-ups or runaway growth, the ecology includes:

* **Local decay:** ordered states drift toward neutral unless reinforced.
* **Global bounds:** extreme conditions (e.g., molten terrain) are capped in total share.
* **Small randomness:** periodic nudges prevent the world from getting stuck.

These keep the system cycling rather than stalling.

---

## 6. Player & System Influence

Players touch the same variables the world uses:

| Influence              | Ecological response                                         |
| ---------------------- | ----------------------------------------------------------- |
| Extraction/destruction | Reduces local stability; can push areas toward high energy. |
| Abandonment/time       | Lets fertility and vegetation recover.                      |
| Construction           | Creates stable pockets that resist change while maintained. |
| Death/decay            | Returns material to fertile or volatile states.             |

Player activity becomes part of the world’s balancing loop.

---

## 7. Celestial Shards (Safe Zones)

**Celestial Shards** are rare world objects that appear occasionally and fade over time.
While active, each shard projects a **PvP-disabled safe zone** with steadier environmental behavior.

**Effects inside the zone**

* Player-vs-player combat is disabled.
* Terrain updates slow down; the area stays relatively stable and flat.
* Hostile activity is reduced or kept outside the boundary.

**Lifecycle**

* Shards spawn infrequently and without player input.
* Their influence **slowly decays**: the zone shrinks and effects weaken over long spans.
* When a shard expires, normal world rules resume.

**Usage and limits**

* Useful as temporary hubs for trade, recovery, or low-risk gathering.
* They do not grant ownership or special rights and cannot be “captured.”
* Only a small number can exist at once, and they avoid overlapping.

---

## 8. Principles

1. **Emergent structure:** large features come from local rules.
2. **Continuous change:** no hard resets; updates are gradual.
3. **Shared variables:** players affect the same fields as nature.
4. **Recovery paths:** damage eventually feeds regeneration.
5. **Deterministic core + variation:** same seed is reproducible; light randomness keeps variety.

---

## Result

The world is a process, not a static map.
Forests grow and burn, rock heats and cools, caves open and collapse, and stabilizers appear and fade.
The system keeps moving toward a shifting balance shaped by both environment and players.
