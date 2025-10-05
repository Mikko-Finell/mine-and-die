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
