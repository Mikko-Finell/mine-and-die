# mine-and-die

Mine & Die is an experimental browser-based arena where players race to extract gold, fight over territory, and live with permadeath consequences. A Go backend simulates the world at a steady tick rate, while a lightweight JavaScript client renders the action in real time.

## Documentation Map
- [Project Overview](docs/README.md)
- [Server Architecture](docs/server.md)
- [Client Architecture](docs/client.md)
- [Effects & Conditions](docs/effects.md)
- [Testing & Troubleshooting](docs/testing.md)
- [Contributor Guidelines](AGENTS.md)
- [AI System](docs/ai.md)

Use these documents as the primary reference when extending gameplay, networking, or presentation.

## Effects Playground & Tooling

The JavaScript effects library and its React playground now live under `tools/js-effects/`. Use the npm scripts at the reposito
ry root to work with them:

- `npm run dev` launches the playground for interactive effect iteration.
- `npm run build` compiles all workspace packages and synchronises the ESM build output into `client/js-effects/` for consumpti
on by the game client.

## Server Layout
The Go module under `server/` is now split by responsibility so contributors can jump straight to the area they need:

- `constants.go` – Shared world and timing constants.
- `main.go` – HTTP wiring, endpoint registration, and WebSocket loop bootstrap.
- `hub.go` – Core state container plus join/subscribe/disconnect flows, the command queue, and the simulation ticker.
- `simulation.go` – World data model, per-tick system orchestration, and combat/effect systems.
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
- **Finite Gold Deposits** – Gold mines spawn with fixed capacities, deplete permanently when exhausted, and occasionally respawn elsewhere to keep the global supply scarce.
- **Mining & Loss** – Extracted gold becomes a physical inventory item that drops on death alongside the rest of a player's belongings.
- **Safe Zones & Marketplace** – Trading is only risk-free inside marked tiles; the global market can be browsed remotely but orders require safe-zone presence.
- **Faction Hierarchy** – Four ranks (King → Noble → Knight → Citizen) form a tree. Superiors manage direct subordinates and configure tax rates.
- **Hierarchical Taxation** – Any gold income automatically routes percentage cuts up the faction chain, with succession-by-kill reassigning positions on lethal coups.

## Runtime Contract
1. Clients `POST /join` to receive a snapshot containing their player ID, all known players, obstacles, active effects, and any queued fire-and-forget effect triggers.
2. A WebSocket connection (`/ws?id=<player-id>`) delivers `state` messages ~15× per second, each bundling live effects plus one-shot `effectTriggers` generated since the previous tick.
3. Clients send `{ type: "input", dx, dy, facing }` whenever movement intent changes, `{ type: "path", x, y }` for click-to-move navigation, `{ type: "cancelPath" }` when manual control resumes, and `{ type: "action", action }` for abilities. The hub stages these as simulation commands.
4. Heartbeats (`{ type: "heartbeat", sentAt }`) flow every ~2 seconds; the hub records the timing as a command and missing three in a row disconnects the session.
5. `/diagnostics` exposes a JSON summary of tick rate, heartbeat interval, and per-player timing data.

## Command Pipeline
- **Commands** – Each inbound message becomes a typed command (`Move`, `Action`, `Heartbeat`) stored until the next tick. Commands capture the issuing tick, player ID, and structured payload so the simulation runs deterministically.
- **AI pass** – Before staging player commands, the server runs finite state machines for every NPC whose cadence is due. The executor reads compiled configs from `server/ai_configs/`, evaluates transitions using pre-resolved IDs, and enqueues standard `Command` structs (movement, facing, abilities). This keeps the hot path lock-free and avoids string lookups during the tick.
- **World step** – `World.Step` consumes staged commands, updates intents/heartbeats, advances movement, resolves collisions, executes abilities, applies hazards, and prunes stale actors.
  The hub broadcasts the resulting snapshots to every subscriber each tick.

## AI System
NPC behaviour is authored as declarative finite state machines. Designers write JSON configs (see `server/ai_configs/`) describing states, transition conditions, and action lists. At startup the server compiles these configs into ID-based tables so the runtime never performs string lookups or reflection. Each tick the executor:

1. Sorts NPC IDs for deterministic iteration and checks the `NextDecisionAt` cadence gate.
2. Evaluates transitions in order, applying the first matching condition and updating the NPC's active state when it changes.
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
Run both the client and server suites before submitting changes:
```bash
npm test
cd server
go test ./...
```
The JavaScript tests use Vitest to spot-check brittle client helpers; we are not pursuing full browser coverage and the effects playground tooling does not require dedicated tests. The Go tests continue to exercise join flow, intent handling, collision resolution, effect lifecycles, and heartbeat tracking.

## Roadmap
The core mining loop, melee/projectile combat, and lava-driven conditions are already
implemented (see the server, client, and effects documentation). The remaining
milestones focus on systems that have not shipped yet.

### Milestone 1 – Gold resource loop
- **Server**
  - Represent finite-capacity gold deposits, support depletion/despawning, and broadcast respawn events.
- **Client**
  - Visualise deposit state, depletion, and respawn timings so players can prioritise contested sites.
- **Systems & Economy**
  - Ensure mining actions transfer gold into inventories and trigger the faction tax pipeline.
- **Documentation**
  - Capture depletion rules, respawn cadence, and player-facing scarcity expectations.

### Milestone 2 – Safe zones & market interaction
- **Server**
  - Authoritative safe-zone definitions that disable PvP and gate market interactions.
  - Implement escrowed buy/sell order matching with direct inventory transfers.
- **Client**
  - Surface safe-zone boundaries, market listings, and order-fulfilment flows tied to on-tile presence.
- **Systems & Economy**
  - Enforce remote market browsing with location-locked order execution, keeping transactions synchronous with taxation.
- **Documentation**
  - Expand references for safe-zone behaviour, market usage, and risk expectations when travelling with gold.

### Milestone 3 – Faction hierarchy & succession
- **Server**
  - Persist faction trees with King/Noble/Knight/Citizen ranks, promotion powers, and configurable tax percentages.
  - Handle succession-by-kill to immediately reassign positions and subordinate tax streams.
- **Client**
  - Provide hierarchy management tools, tax visibility, and coup feedback for kill-based promotions.
- **Systems & Economy**
  - Integrate tax routing with every gold acquisition path and maintain subordinate reassignment when members leave.
- **Documentation**
  - Maintain faction governance, taxation configuration, and succession rules in the design docs.
