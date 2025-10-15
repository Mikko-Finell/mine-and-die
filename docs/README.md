# Mine & Die Documentation Hub

Welcome to the documentation set for the Mine & Die prototype. The project explores a browser-based, PvP-focused mining arena with an authoritative Go backend and a vanilla JavaScript client. This hub gives you the broad context and points you toward deeper dives for each part of the stack.

## Repository Layout
- `server/` – Authoritative simulation, HTTP handlers, and WebSocket hub. See [Server Architecture](./architecture/server.md).
- `client/` – UI, input, networking, and rendering modules served as static assets. The legacy implementation is archived in [Client Architecture (deprecated)](./old/architecture/client.md) while the new client takes shape.
- `docs/` – Living documentation (this folder).
  - `architecture/` – Technical specs and engineering-focused docs.
  - `gameplay-design/` – Concept pillars, combat intentions, and progression loops.
- `README.md` – Quickstart instructions and high-level pitch.
- `AGENTS.md` – Contributor guidelines and conventions.

## Additional References
- [Networking](./architecture/networking.md) – Join flow, message contracts, diff metadata, and heartbeat policy.
- [Server Architecture](./architecture/server.md) – Tick loop, data structures, actions, and diagnostics.
- [Client Architecture (deprecated)](./old/architecture/client.md) – Store layout, rendering flow, and network helpers from the legacy stack.
- [Effects & Conditions](./architecture/effects.md) – How the authoritative effect pipeline maps to client visuals.
- [Item System](./architecture/items.md) – Canonical definitions, catalog export, and fungibility-aware runtime integration.
- [Logging](./architecture/logging.md) – Publisher/router design plus event catalog.
- [AI System](./architecture/ai.md) – Behaviour configs, executor flow, and extension tips.
- [Stats System](./architecture/stats.md) – Layered attributes, mutation flow, and integration points for combat.
- [Testing & Troubleshooting](./architecture/testing.md) – Available test suites and common debugging tips.
- [Patch Migration Plan](./architecture/patch-migration-plan.md) – Current coverage and roadmap for the diff pipeline.
- [Playground Animations](./architecture/playground-animations.md) – How to author effect definitions and surface them inside the playground tool.

Keep this folder updated as the simulation evolves so newcomers can get productive quickly.
