# Mine & Die

Mine & Die is a browser-based PvP extraction prototype. A Go 1.24 server simulates the world at ~15 Hz, serves static assets, and drives an authoritative WebSocket stream. A lightweight HTML/ESM client renders the arena, captures input, and mirrors simulation snapshots in real time.

## At a Glance
- **Server** – Go HTTP service plus Gorilla WebSocket hub, deterministic world step, finite-state AI executor, and `/diagnostics` telemetry. See [Server Architecture](docs/architecture/server.md) and [AI System](docs/architecture/ai.md).
- **Client** – `<canvas>` renderer, intent/input handlers, latency HUD, and diagnostics drawer delivered as static ES modules. A ground-up replacement is underway; legacy details live in [Client Architecture (deprecated)](docs/old/architecture/client.md).
- **Protocol** – `POST /join` bootstrap followed by a single `/ws?id=…` channel for intents, actions, state snapshots, and heartbeats. Message contracts live in [Networking](docs/architecture/networking.md).

## Repository Layout
- `server/` – Authoritative simulation, hub, HTTP handlers, finite-state AI runtime, and regression tests.
- `client/` – Active modules for the in-flight rewrite; the previous implementation now resides in `client/old/`.
- `docs/` – Architecture notes, gameplay design, and troubleshooting references (start with [docs/README.md](docs/README.md)).
- `tools/js-effects/` – React playground and build tooling for the effect library (outputs synced into `client/js-effects/`).
- `technical_debt.md` – Ongoing cleanup backlog and investigation notes.

## Setup
1. **Install prerequisites** – Go 1.24.x (matches `server/go.mod`). Node.js ≥ 18 is optional for running Vitest or the effects playground.
2. **Run the server**
   ```bash
   cd server
   go run .
   ```
   The process listens on `http://localhost:8080`, serving both the API and static client files.
3. **Open the client** – Visit `http://localhost:8080` in a browser. Refresh to pick up any HTML/CSS/JS edits.
4. **Stop** – Press `Ctrl+C` in the terminal running the server.

## Testing
- Go regression tests:
  ```bash
  cd server
  go test ./...
  ```
- Client/unit tests (optional, via Vitest):
  ```bash
  npm test
  ```

## Runtime Flow
1. Clients call `POST /join` to receive player metadata, world configuration, active actors/effects, and queued one-shot triggers.
2. The browser immediately upgrades to `/ws?id=<player-id>` and receives ~15 Hz `state` snapshots plus incremental diff metadata.
3. Input changes emit `input` commands; click-to-move adds `path`/`cancelPath`; ability keys send `action`; diagnostics helpers issue `console` messages. Each payload includes the client’s last acknowledged tick.
4. The hub drains staged commands, advances AI finite state machines, applies movement, collision, effects, hazards, and inventory, then broadcasts the fresh snapshot to all peers.
5. Heartbeats every ~2 seconds keep latency metrics current and disconnect any client that misses three intervals (~6 s).

For deeper coverage of systems, see [Effects & Conditions](docs/architecture/effects.md), [Items](docs/architecture/items.md), and [Testing & Troubleshooting](docs/architecture/testing.md).

## Tooling Notes
- `npm run dev` runs the effects playground in `tools/js-effects/` for iterating on combat visuals.
- `npm run build` rebuilds the effects workspace and syncs the distributable into `client/js-effects/`.

## Roadmap
High-level milestone tracking lives here for quick reference. Dive into
[`docs/project-milestones.md`](docs/project-milestones.md) for the full scope,
dependencies, and acceptance criteria for each phase.

1. **Milestone 1 – Core Stats & Itemization Backbone** → Establishes the stat
   taxonomy, equippable items, and inventory plumbing that underpin every other
   system.
2. **Milestone 2 – Crafting & Resource Loop** → Introduces resource gathering
   and recipes so materials can be transformed into functional gear.
3. **Milestone 3 – Stat Progression & Boost Items** → Adds long-term character
   growth through crafted boosters that permanently raise stats.
4. **Milestone 4 – Combat MVP** → Delivers deterministic real-time combat with
   hit resolution, NPC encounters, and basic logging for tuning.
5. **Milestone 5 – Economy & Market System** → Connects mining, trading, and
   taxation into a cohesive gold circulation loop.
6. **Milestone 6 – Factions & Tax Hierarchy** → Layers on political structures
   so factions can earn and manage income across the playerbase.
7. **Milestone 7 – Balance & Integration Pass** → Harmonises tuning across
   systems and locks in analytics to support wider playtests.
