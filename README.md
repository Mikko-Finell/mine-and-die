# mine-and-die

Experimental browser-based, PvP-enabled permadeath MMO prototype. Players mine finite gold from neutral deposits, organize into guild hierarchies with configurable taxation, and establish control through direct conflict. The world has no formal ownership system—territory only persists while defended.

---

## Core Concepts

- **Gold Mining**
  - Gold is a finite, halving-schedule resource.
  - Players must equip a pickaxe and mine at designated deposits.
  - Mining requires player action and exposes the miner to PvP risk.
  - Mined gold enters the player’s inventory and triggers guild tax distribution.

- **Permadeath**
  - Player death results in full character deletion and item drop.
  - All inventory, gold, and equipment are dropped on death.
  - No respawn; new character creation required.

- **Guilds**
  - Five-tier hierarchy: King → Noble → Knight → Squire → Citizen.
  - Guilds define custom tax percentages.
  - All gold income is automatically taxed upward through the chain of command.
  - Guilds maintain treasuries and can use funds for recruitment or defense.
  - Peasants are guild-less players operating outside the hierarchy.

- **Economy**
  - No NPC merchants or vendors.
  - Monsters drop items, not gold.
  - All trade, pricing, and supply chains are player-driven.
  - Gold supply follows a halving schedule; scarcity increases over time.

- **World State**
  - Mines are neutral entities; there is no formal ownership.
  - Control is emergent and based purely on manpower and coordination.
  - Players and NPCs share the same simulation space.

---

## Technical Overview

- **Server**
  - Language: Go
  - Protocols: WebSocket (realtime state sync), HTTP (auth, static endpoints)
  - Architecture: Authoritative tick-based simulation (10–20 Hz)
  - State: In-memory world model with periodic persistence to SQLite/Postgres
  - Modules:
    - Player state & inventory
    - Combat resolution (server-authoritative)
    - Mining tick handler & emission schedule
    - Guild and tax ledger
    - Item spawn/despawn system
  - Security: Movement clamping, server-only damage authority, per-session nonces

- **Client**
  - Stack: HTML5 + vanilla JS + `<canvas>` rendering
  - Graphics: Early alpha uses procedurally generated canvas primitives rendered on the client; no sprite sheets are planned for the initial milestone.
  - Networking: WebSocket client for input and state updates
  - Features:
    - Player movement and interpolation
    - Mining and combat interactions
    - Basic UI overlays (HP, gold, inventory)

---

## Development

### Requirements
- Go ≥ 1.22
- Node.js (for client bundling)
- SQLite (default local persistence)

### Setup
```bash
git clone https://github.com/<your-username>/mine-and-die
cd mine-and-die
go run ./server
# in another terminal
npm install && npm run dev

```
