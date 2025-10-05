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
- **Gold Mining** – Finite deposits follow a halving schedule. Mining requires player action and exposes you to PvP risk.
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
### Milestone 1 – Authoritative tick loop & robust networking
- Introduce a fixed-rate simulation ticker (10–20 Hz) that applies stored player intents, clamps movement, and rebroadcasts authoritative positions from the server.
- Switch the WebSocket contract to accept input payloads, persisting them per player for consumption in the tick loop.
- Add heartbeat and disconnect handling plus round-trip diagnostics so inactive clients are cleaned up reliably.
- Update the web client to publish input changes, lerp toward server positions, and surface connection quality details.
- Document the authoritative movement flow and heartbeat expectations for contributors.

### Milestone 2 – Implement mines and gold extraction
- Model mine nodes on the server (location, remaining ore, respawn timers) and include their status in state broadcasts.
- Resolve mining actions server-side, emitting gold into inventories while depleting mine reserves and scheduling respawns.
- Extend `/join` responses and realtime payloads so clients receive mine metadata for rendering.
- Render mines on the client canvas, allow players to start/stop mining within range, and display progress feedback.
- Capture mining rules, depletion mechanics, and respawn cadence in the documentation.

### Milestone 3 – Server-authoritative combat & permadeath
- Expand the player model with health, damage, equipment, and cooldown tracking, resolving attacks in the authoritative tick.
- Handle permadeath cleanup server-side, dropping inventories into the world and removing dead sessions.
- Broadcast combat events (attack requests, damage notifications, death/drop announcements) over WebSocket.
- Visualize HP, damage feedback, respawn/creation flows, and dropped loot on the client.
- Document combat rules, permadeath consequences, and loot retrieval expectations.

### Milestone 4 – Guild hierarchy with automated taxes
- Define guild data structures with tiered roles, parent-child relationships, and treasury balances, persisting them as needed.
- Provide APIs or WebSocket commands for guild creation, invites, promotions, demotions, and tax configuration within the five-tier limit.
- Route mining rewards through the tax pipeline so gold is distributed up the hierarchy before reaching players.
- Build client UI for guild management, treasury summaries, and tax notifications.
- Expand the README with guild roles, taxation mechanics, and the experience for guild-less players.

### Milestone 5 – Persistent economy & item lifecycle
- Integrate SQLite/Postgres persistence covering players, guilds, mines, and item drops with crash-safe recovery.
- Implement the halving-schedule gold emission so scarcity increases over time.
- Add item spawn/despawn systems for NPCs/monsters, integrating drops into combat resolution and world state broadcasts.
- Support player-to-player trade or guild treasury withdrawals with accompanying client UI.
- Document persistence setup, the economic halving schedule, and trading expectations, including migration steps.
