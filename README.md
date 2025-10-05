# mine-and-die

Mine & Die is an experimental browser-based arena where players race to extract gold, fight over territory, and live with permadeath consequences. A Go backend simulates the world at a steady tick rate, while a lightweight JavaScript client renders the action in real time.

## Documentation Map
- [Project Overview](docs/README.md)
- [Server Architecture](docs/server.md)
- [Client Architecture](docs/client.md)
- [Testing & Troubleshooting](docs/testing.md)
- [Contributor Guidelines](AGENTS.md)

Use these documents as the primary reference when extending gameplay, networking, or presentation.

## Server Layout
The Go module under `server/` is now split by responsibility so contributors can jump straight to the area they need:

- `constants.go` – Shared world and timing constants.
- `main.go` – HTTP wiring, endpoint registration, and WebSocket loop bootstrap.
- `hub.go` – Core state container plus join/subscribe/disconnect flows and the simulation ticker.
- `player.go` – Player-facing types, facing math, and intent bookkeeping.
- `movement.go` – Movement helpers, collision resolution, and clamp utilities.
- `obstacles.go` – Procedural world generation and geometry helpers.
- `effects.go` – Ability cooldowns, projectiles, and effect lifecycle management.
- `messages.go` – JSON payload contracts for `/join`, `/ws`, and heartbeat acknowledgements.

## Core Concepts
- **Gold Mining** – Swinging a melee attack into a gold ore block currently awards a single gold coin; richer economy systems remain aspirational.
- **Permadeath** – Death deletes the character and drops everything. Create a new avatar to rejoin the fray.
- **Guild Hierarchy** – Five roles (King → Noble → Knight → Squire → Citizen) with configurable taxes that flow upward.
- **Player-Driven Economy** – No NPC merchants; scarcity and pricing are dictated by players. Monsters drop items, not gold.
- **Emergent Territory** – Mines are neutral. Control exists only while actively defended by players.

## Runtime Contract
1. Clients `POST /join` to receive a snapshot containing their player ID, all known players, obstacles, and active effects.
2. A WebSocket connection (`/ws?id=<player-id>`) delivers `state` messages ~15× per second.
3. Clients send `{ type: "input", dx, dy, facing }` whenever movement intent changes and `{ type: "action", action }` for abilities.
4. Heartbeats (`{ type: "heartbeat", sentAt }`) flow every ~2 seconds; missing three in a row disconnects the session.
5. `/diagnostics` exposes a JSON summary of tick rate, heartbeat interval, and per-player timing data.

## Getting Started
1. **Install dependencies** – Go ≥ 1.22 is required. Node.js is optional for future tooling.
2. **Run the server**
   ```bash
   cd server
   go run .
   ```
3. **Open the client** – Visit [http://localhost:8080](http://localhost:8080) in your browser.
4. **Stop the server** – Press `Ctrl+C` in the terminal running `go run .`.

The Go server serves static assets straight from `client/`, so refreshing the browser picks up any changes immediately.

## Testing
Run the Go suite from the server module:
```bash
cd server
go test ./...
```
The tests exercise join flow, intent handling, collision resolution, effect lifecycles, and heartbeat tracking.

## Roadmap
### Milestone 1 – Mines & resource extraction
- **Server**
  - Model mine nodes (location, remaining ore, respawn timers) and ship their state in authoritative broadcasts.
  - Resolve mining intents server-side, awarding gold, depleting reserves, and scheduling respawns.
- **Client**
  - Render mines on the canvas, allow players to start/stop mining within range, and surface progress feedback.
- **Systems & Economy**
  - Wire mining payouts into player inventories and ensure depletion pacing supports contested hotspots.
- **Documentation**
  - Capture mining rules, depletion mechanics, and respawn cadence for contributors.

### Milestone 2 – Combat & permadeath
- **Server**
  - Expand the player model with health, damage, equipment, and cooldown tracking, resolving combat in the tick loop.
  - Handle permadeath cleanup, dropping inventories, removing dead sessions, and broadcasting events.
- **Client**
  - Visualize HP, damage feedback, respawn/creation flows, and dropped loot within the HUD.
- **Systems & Economy**
  - Define loot tables and drop behaviour that reinforce risk/reward loops.
- **Documentation**
  - Document combat rules, permadeath consequences, loot retrieval, and player re-entry expectations.

### Milestone 3 – Guild hierarchy & taxation
- **Server**
  - Define guild data structures with tiered roles, relationships, and treasury balances backed by persistence-ready storage.
  - Expose APIs or WebSocket commands for creation, invites, promotions, demotions, and configurable taxes.
- **Client**
  - Build UI for guild management, treasury summaries, and tax notifications.
- **Systems & Economy**
  - Route mining rewards through the tax pipeline so gold distributes up the hierarchy before reaching players.
- **Documentation**
  - Expand contributor docs with guild roles, taxation mechanics, and guidance for guild-less players.

### Milestone 4 – Persistent economy & item lifecycle
- **Server**
  - Integrate SQLite/Postgres persistence covering players, guilds, mines, and item drops with crash-safe recovery.
  - Implement halving-schedule gold emission so scarcity increases over time.
- **Client**
  - Support player-to-player trade or guild treasury withdrawals with appropriate UI flows.
- **Systems & Economy**
  - Add item spawn/despawn systems for NPCs/monsters, integrating drops into combat resolution and world broadcasts.
- **Documentation**
  - Document persistence setup, the economic halving schedule, trading expectations, and required migrations.
