# mine-and-die

Mine & Die is an experimental browser-based arena where players race to extract gold, fight over territory, and live with permadeath consequences. A Go backend simulates the world at a steady tick rate, while a lightweight JavaScript client renders the action in real time.

## Documentation Map
- [Project Overview](docs/README.md)
- [Server Architecture](docs/server.md)
- [Client Architecture](docs/client.md)
- [Testing & Troubleshooting](docs/testing.md)
- [Contributor Guidelines](AGENTS.md)
- [AI System](docs/ai.md)

Use these documents as the primary reference when extending gameplay, networking, or presentation.

## Server Layout
The Go module under `server/` is now split by responsibility so contributors can jump straight to the area they need:

- `constants.go` – Shared world and timing constants.
- `main.go` – HTTP wiring, endpoint registration, and WebSocket loop bootstrap.
- `hub.go` – Core state container plus join/subscribe/disconnect flows, the command queue, and the simulation ticker.
- `simulation.go` – World data model, per-tick system orchestration, and event emission.
- `player.go` – Player-facing types, facing math, and intent bookkeeping.
- `npc.go` – Neutral enemy definitions, snapshots, and seeding helpers.
- `ai_types.go` – Shared AI structs used by the finite state machine executor.
- `ai_library.go` – Loads author-authored FSM configs, validates them, and compiles compact runtime data.
- `ai_executor.go` – Evaluates NPC state machines each tick and enqueues deterministic commands.
- `movement.go` – Movement helpers, collision resolution, and clamp utilities.
- `obstacles.go` – Procedural world generation and geometry helpers.
- `effects.go` – Ability cooldowns, projectiles, environmental hazards, and effect lifecycle management.
- `inventory.go` – Item and stack management utilities.
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
3. Clients send `{ type: "input", dx, dy, facing }` whenever movement intent changes, `{ type: "path", x, y }` for click-to-move navigation, `{ type: "cancelPath" }` when manual control resumes, and `{ type: "action", action }` for abilities. The hub stages these as simulation commands.
4. Heartbeats (`{ type: "heartbeat", sentAt }`) flow every ~2 seconds; the hub records the timing as a command and missing three in a row disconnects the session.
5. `/diagnostics` exposes a JSON summary of tick rate, heartbeat interval, and per-player timing data.

## Command & Event Pipeline
- **Commands** – Each inbound message becomes a typed command (`Move`, `Action`, `Heartbeat`) stored until the next tick. Commands capture the issuing tick, player ID, and structured payload so the simulation runs deterministically.
- **AI pass** – Before staging player commands, the server runs finite state machines for every NPC whose cadence is due. The executor reads compiled configs from `server/ai_configs/`, evaluates transitions using pre-resolved IDs, and enqueues standard `Command` structs (movement, facing, abilities). This keeps the hot path lock-free and avoids string lookups during the tick.
- **World step** – `World.Step` consumes staged commands, updates intents/heartbeats, advances movement, resolves collisions, executes abilities, applies hazards, and prunes stale actors.
- **Events** – Systems append high-level events (movement, health changes, effect spawns, loot awards, despawns) to the step output. The hub can fan these out alongside snapshots for future HUD/analytics features.

## AI System
NPC behaviour is authored as declarative finite state machines. Designers write JSON configs (see `server/ai_configs/`) describing states, transition conditions, and action lists. At startup the server compiles these configs into ID-based tables so the runtime never performs string lookups or reflection. Each tick the executor:

1. Sorts NPC IDs for deterministic iteration and checks the `NextDecisionAt` cadence gate.
2. Evaluates transitions in order, applying the first matching condition and emitting `AIStateChanged` events.
3. Executes the state's actions, which only enqueue standard commands (`CommandMove`, `CommandAction`, etc.).
4. Updates per-NPC blackboards (timer bookkeeping, waypoint indices, stuck counters) and schedules the next evaluation tick.

The initial `goblin` patrol uses a simple `Patrol ↔ Wait` loop (`server/ai_configs/goblin.json`). The new `rat` archetype demonstrates random roaming and flee logic powered by `setRandomDestination`, `nonRatWithin`, and `moveAway` without changing the tick loop—just add a config and tests.

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
