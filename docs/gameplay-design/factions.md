# Faction Hierarchy — Mechanics Spec (v1)

> Minimal, enforceable rules only. No NPC mediation or intent statements.

---

## 1) Structure

* A **Faction** is a rooted tree with exactly one **King** (root).
* Every member except the King has exactly **one superior**.
* Default engine ranks (labels for permissions/tax steps):
  `King → Noble → Knight → Citizen`

---

## 2) Authority

* **King**

  * May promote, demote, or reassign **any member**.
  * Configures **tax percentages** for each rank-to-superior step (server-bounded).
* **Any superior**

  * May promote/demote **direct subordinates** within the next rank down.
  * May reassign subordinates among their own children.

---

## 3) Taxation

* When gold enters a member’s inventory (mining, trade, loot, transfer), a configured percentage routes to their **immediate superior**; each superior forwards their configured percentage upward, ending at the King.
* Taxes are taken from the incoming amount (no duplication or minting).
* Tax rates are stored on the Faction and editable by the King; changes apply to subsequent income.

---

## 4) Deaths, Coups, and Succession (Permadeath)

All deaths are **permanent**. The hierarchy **self-repairs** immediately so no rank is left vacant.

### 4.1 Internal Coup (intra-Faction)

**Condition:** Killer and victim are in the same Faction, and the victim is higher in the killer’s chain.
**Effect:**

1. Killer is **promoted to the victim’s rank and position**.
2. All of the victim’s subordinates (and their tax streams) reattach under the killer.
3. Victim’s inventory drops as world loot.

### 4.2 External Kill (inter-Faction or unaffiliated killer)

**Condition:** Killer is not in the victim’s Faction (or is in a different branch with no superior/subordinate relation).
**Effect:**

1. The victim’s **highest-priority subordinate** is **promoted** into the victim’s rank and position (see 4.4 Promotion Rule).
2. All other subordinates reattach beneath that promoted member.
3. Victim’s inventory drops as world loot.
4. The killer gains **no** membership or authority.

### 4.3 King’s Death

* The new King is chosen by the **Promotion Rule** among the King’s direct subordinates.
* The tree remains intact; tax routes update instantly.

### 4.4 Promotion Rule (deterministic)

When automatic promotion is required (external kill, King death, voluntary departure):

1. Prefer **highest character level** among the victim’s direct subordinates.
2. If tied, prefer **oldest recruit date** into the Faction.
3. If still tied, choose deterministically by **lowest account ID**.

> Internal coups (4.1) **bypass** this rule: the **killer** is promoted directly.

### 4.5 Leaving / Ejection

* Voluntary departure or ejection removes the member’s node and triggers the **Promotion Rule** to fill the vacancy; subordinates reattach beneath the promoted member.

---

## 5) Membership & Integrity

* **Joining:** an authorized superior attaches the recruit as a direct subordinate.
* The tree is always valid: **no orphan nodes, no vacant ranks**.
* Outsiders cannot gain membership or control via combat.
* All hierarchy changes (join/leave/promotion/demotion/death/coup) update tax routing **immediately**.

---

## 6) Offline & Inactivity Interactions (Hierarchy-Relevant Only)

* If a member dies while offline (per world/offline rules): apply **4.2/4.3** as appropriate.
* **Dormancy:** after configured inactivity thresholds, superiors (or the King) may **eject** a dormant member; treat as **Leaving** (5), triggering the **Promotion Rule**.

---

## 7) Event Logging (Audit)

The engine records immutable events for transparency and dispute resolution:

* `Join`, `Leave/Eject`, `Promote`, `Demote`, `Reassign`, `Death`, `Coup`, `King Succession`, `Tax-Rate Change`.

---

## 8) Parameters (implementation knobs)

* Allowed **tax percentage** ranges per rank step.
* **Rank set** (labels and step order), if customization is enabled.
* **Dormancy** thresholds (warn/eject).
* Tie-break ordering for the **Promotion Rule** (if server wants alternative criteria).

---

**Interface Notes (cross-doc references):**

* Economy doc defines what constitutes “gold income” and physical drop rules.
* Crafting doc is independent; faction membership does not alter recipe access.
