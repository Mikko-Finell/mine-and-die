# Agent Guidelines

This file applies to the entire repository.

## Repository layout
- `server/`: Go 1.24.3 service that exposes `/join`, `/move`, `/events`, `/health` endpoints. Clients receive realtime snapshots via Server-Sent Events and the server acts as the authoritative state hub. Static assets are served from `client/`.
- `client/`: Static HTML, CSS, and vanilla JS canvas prototype. Networking uses `fetch` for writes and `EventSource` for state streaming. Keep the code dependency-free and organized by feature (DOM bindings, game loop, networking) inside `main.js`.

## Design notes
- Keep communication between client and server minimal and JSON based; avoid introducing new transports without discussion.
- The world simulation is intentionally simple (shared player positions only). Favor clarity over premature abstraction when extending logic.
- Client visuals are intentionally lightweight grid-and-square rendering. Preserve the clean canvas rendering style unless a larger art direction change is planned.

## Contribution tips
- Document new endpoints or significant client interactions in `README.md` when they alter gameplay expectations.
- Prefer small, incremental commits that align with features or fixes; include succinct messages describing the change.
