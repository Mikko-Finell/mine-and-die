# Economy — Mechanics Spec (v1)

> Minimal, enforceable rules only. No NPC vendors, no artificial sinks, no intent statements.

---

## 1) Scope & Currency

* The **economy** consists of: resource emission (mining), transport, trade, taxation routing (see Faction spec), and PvP loss.
* **Gold** is the sole currency item recognized by engine systems.
* Engine does not mint/burn gold outside of deposit extraction and physical loss via player action.

---

## 2) Gold Emission (Finite Deposits)

**2.1 Deposits**

* World contains **finite gold deposits**; each has a fixed **capacity** (total gold units).
* When capacity reaches 0, the deposit is **permanently exhausted** and removed from the world.

**2.2 Mining Action**

* A mining action within range:

  * removes `x` gold from the deposit (bounded by remaining capacity),
  * adds `x` gold to the miner’s inventory as **Gold (item)**,
  * triggers taxation routing (see §5).

**2.3 Ownership**

* Deposits have **no engine-level ownership**; access control is physical only (player enforcement).

**2.4 World Growth (optional)**

* Servers may introduce **new regions** containing fresh deposits.
* Total configured world cap may expand only by administrator action; engine itself does not inflate supply.

---

## 3) Transport & Loss

* Gold is a **physical, tradable item** in inventories and containers.
* On player **death**: the **entire inventory drops** as world loot; nothing is protected or deleted by the system.
* Dropped items obey standard item-lifetime rules (despawn timers or container persistence), never burning gold implicitly.

---

## 4) Market

**4.1 Visibility & Execution**

* Market data (listings/prices) is **read-only globally** from any location.
* Creating or fulfilling orders requires physical presence in a **safe zone** (see world rules).

**4.2 Order Types**

* **Sell listings** for concrete items (non-fungible or fungible).
* **Buy orders (optional)** for **fungible** resources of identical type/quality (e.g., ore grade N).

  * Fungibility is defined by item type + quality tags; items that do not match exactly cannot auto-settle into fungible orders.

**4.3 Escrow & Settlement**

* When listing or matching inside a safe zone, the market acts as **escrow** only:

  * Seller’s item and buyer’s gold are locked until the match finalizes.
  * On match, the engine transfers **item ↔ gold** directly **between player inventories**.
* **No system fees**; if servers enable fees, they must route to a player-owned entity (never burned).

**4.4 No NPC Logistics**

* Engine provides no NPC caravans, banks, or insurance.
* All item movement is performed by players.

---

## 5) Tax Interaction (Cross-Doc)

* When gold **enters** a member’s inventory by mining, trade, loot, or transfer, **tax percentages** route upward through the member’s Faction chain **immediately** (see *Faction Hierarchy — Taxation*).
* Taxes are taken from the incoming amount; no duplication.

---

## 6) PvP Effects on Economy

* PvP is enabled everywhere except safe zones.
* Economic effects of PvP are limited to **inventory drops** and **subsequent trades**; the engine does not alter prices or supply in response to kills.

---

## 7) Persistence & Scarcity

* Gold/items persist until looted or removed by configured **item-lifetime** rules.
* No routine mint/burn; **scarcity** arises from:

  * finite deposits (§2),
  * physical loss and redistribution via PvP (§3, §6),
  * taxation routing (§5),
  * player hoarding and trade.

---

## 8) Anti-Exploit Constraints

* All market settlements are **atomic** (item and gold transfer succeeds together or not at all).
* Escrowed items/gold cannot be moved, dropped, or taxed until settlement or cancellation completes.
* Duplicate-creation prevention: item and gold identifiers are **unique and conserved** across transfers; engine enforces single-writer semantics for deposit capacity.

---

## 9) Parameters (Implementation Knobs)

* Deposit capacity distributions; world cap; spawn placement rules (if world growth is enabled).
* Mining action rate limits (per-tool, per-node concurrency).
* Safe-zone definitions for market execution.
* Item-lifetime/despawn timers for world drops and containers.
* Market feature toggles (buy orders on/off), and any **player-routed fees** (recipient entity).

---

### Cross-Document References

* **Faction Hierarchy**: taxation percentages, hierarchy updates on death/leave/coup, and event logging.
* **Crafting**: resource categories, quality tags (define fungibility), and recipe graphs that determine downstream demand.
