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

## Realtime Simulation Contract

- **Tick loop**: The Go hub advances the world at ~15 Hz and clamps all player positions within the 800×600 arena before broadcasting the authoritative snapshot on every tick.
- **Input payloads**: Clients send `{ "type": "input", "dx": <float>, "dy": <float> }` messages whenever directional intent changes. Vectors are normalized server-side and persisted until a new intent arrives.
- **Heartbeat expectations**:
  - Clients emit `{ "type": "heartbeat", "sentAt": <unixMillis> }` every ~2 seconds.
  - The server responds with `{ "type": "heartbeat", "serverTime": <unixMillis>, "clientTime": <unixMillis>, "rtt": <ms> }` and removes sockets that miss three consecutive heartbeats (~6 seconds).
- **Diagnostics**: `/diagnostics` returns a JSON payload with the current tick rate, heartbeat interval, and per-player heartbeat/latency observations for monitoring round-trip quality.
- **World geometry**: The server seeds a handful of rectangular obstacles at startup. Their coordinates are included in `/join` responses and every realtime `state` payload so clients can render matching blockers. Player movement is resolved server-side with obstacle collisions and mutual player separation to prevent overlap.

### Controls

- **Movement**: WASD keys issue normalized intent vectors to the server.
- **Melee attack**: Tap `Space` to swing a short-range attack in front of your facing.
- **Fireball**: Press `F` to launch a ranged projectile that travels up to five tiles and vanishes on collision.

---

## Roadmap

The project is in its foundational phase. The milestones below outline the intended progression toward the full Mine & Die experience. Each step is scoped to be deliverable, testable, and to build on the work completed in prior milestones.

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

---

## Development

### Requirements
- Go ≥ 1.22
- Node.js (optional, for client tooling and future build steps)
- SQLite (default local persistence)

## Installation & Local Run Guide

Follow the steps below to get the project running on a Unix-like environment (macOS or Linux). Each step assumes you are comfortable with a terminal but may be new to the Go toolchain.

1. **Install Git (if not already installed).**
   - macOS: `xcode-select --install`
   - Debian/Ubuntu: `sudo apt update && sudo apt install git`

2. **Install Go (required to run the game server).**
   - macOS (Homebrew): `brew install go`
   - Debian/Ubuntu: `sudo apt install golang`
   - Alternatively, download an official tarball from [https://go.dev/dl/](https://go.dev/dl/) and follow the instructions provided there.
   - Verify the installation with `go version`; it should report version 1.22 or newer.

3. **(Optional) Install Node.js and npm.** While the current client is served as static files, future tooling may rely on Node.
   - macOS (Homebrew): `brew install node`
   - Debian/Ubuntu: `sudo apt install nodejs npm`
   - Verify with `node --version` and `npm --version`.

4. **Clone the repository and enter it.**
   ```bash
   git clone https://github.com/<your-username>/mine-and-die
   cd mine-and-die
   ```

5. **Run the Go server.**
   ```bash
   cd server
   go run .
   ```
   The terminal should print `server listening on :8080`. Leave this process running; it serves both the API and the static client from the `client` directory.

6. **Open the client.**
   - In a web browser on the same machine, navigate to [http://localhost:8080](http://localhost:8080).
   - You should see the Mine & Die prototype and can start interacting with the local server immediately.

7. **Stopping the server.**
   - Return to the terminal running `go run .` and press `Ctrl+C` to shut down the server.

If you make changes to the client assets, simply refresh the browser; the Go server serves the updated static files automatically. For more advanced client workflows (such as bundling or hot-module reloading), install Node.js and introduce your preferred tooling inside the `client/` directory.
