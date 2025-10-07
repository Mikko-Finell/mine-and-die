# Client Architecture

The client is a lightweight ES module bundle served directly from the Go server. No bundler or framework is required—open `index.html` and the modules bootstrap themselves.

## Module Overview
- `index.html` – Declares the canvas, status text, diagnostics panel, and loads `main.js` via `<script type="module">`.
- `main.js` – Builds the shared `store` object, wires diagnostics UI, and starts input, render, and networking flows. The debug panel world reset form exposes toggles, per-system spawn counts, and a deterministic seed input so QA can regenerate identical layouts on demand.
- `network.js` – Handles the `/join` handshake, WebSocket lifecycle, heartbeat timers, and outbound message helpers.
- `input.js` – Converts keyboard events into normalized intents and action messages.
- `render.js` – Performs `<canvas>` drawing, lerps network state to display positions, and renders effects/obstacles.
- `styles.css` – Minimal styling for layout and diagnostics readouts.
- `vendor/` – Drop-in ES modules for third-party helpers. Import them from client code via `import … from "./vendor/<file>.js"`.

## Store Shape
`main.js` creates a plain object (`store`) shared across modules. Key fields:
- DOM references (canvas, status text, diagnostics elements).
- Simulation constants (`TILE_SIZE`, `PLAYER_SIZE`, etc.).
- Connection state (`socket`, `playerId`, heartbeat timestamps).
- Player dictionaries: `players` (authoritative) and `displayPlayers` (interpolated positions).
- NPC dictionaries: `npcs` mirrors neutral enemies from the server, `displayNPCs` lerps their positions for rendering.
- Arrays for `obstacles` and `effects` mirrored from server payloads.
- `worldConfig` mirrors the server's toggles along with the deterministic `seed` string used when restarting the world from the debug panel.

## Initialization Sequence
1. `main.js` prepares UI helpers, attaches diagnostics toggles, and registers the latency override input.
2. `registerInputHandlers(store)` tracks pressed keys and sends intents/actions.
3. `startRenderLoop(store)` animates the canvas, interpolating toward server positions every frame.
4. `joinGame(store)` POSTs `/join`, seeds the store, and opens the WebSocket.
5. `connectEvents(store)` sets up WebSocket callbacks and kicks off the heartbeat loop.

## Networking Details
- **State updates:** The server emits `state` messages containing players, NPCs, obstacles, effects, and `serverTime`. The client overwrites `store.players`, `store.npcs`, merges the display caches, and keeps diagnostics fresh.
- **Intents:** `sendCurrentIntent` serializes `{ type: "input", dx, dy, facing }` whenever movement or facing changes.
- **Path navigation:** `sendMoveTo` sends `{ type: "path", x, y }` for click-to-move requests while `sendCancelPath` clears the server-driven route when WASD input resumes.
- **Actions:** `sendAction` is used by `input.js` for melee and fireball triggers.
- **Heartbeats:** `startHeartbeat` sets an interval that calls `sendHeartbeat`; acknowledgements update latency displays.
- **Reconnects:** Socket closure funnels through `handleConnectionLoss`, which resets state and schedules `joinGame` again.

## Rendering Notes
- Grid + background are redrawn each frame for clarity.
- Players are drawn as colored squares with a facing indicator line; the local player uses cyan/white, others orange/cream.
- NPCs are drawn in violet with their facing indicator and optional type label.
- Obstacles use either a stone block style or a gold ore treatment with deterministic pseudo-random nuggets.
- Melee swings are rendered through the js-effects `EffectManager` using the shared
  `MeleeSwingEffectDefinition` (synced to `client/js-effects/effects/meleeSwing.js` from the
  TypeScript source in `tools/js-effects/packages/effects-lib`). This keeps the in-game red hitbox
  identical to the playground entry and lets contributors tweak it from a single definition while
  other effect types continue to fall back to simple rectangles.

## Extending the Client
- Add new HUD elements to `index.html`, register them in the `store`, and update `main.js` diagnostics helpers.
- Mirror new server fields by adjusting the payload handling inside `network.js` (state, actions, heartbeat logic).
- Expand rendering logic by extending `render.js`; prefer pure functions that read from the `store` to keep coordination simple.
- Route new combat visuals through js-effects. Reuse the existing melee swing integration as a template and update the
  helper maps/cleanup logic in `network.js` if additional tracked instances are required.
- New input bindings belong in `input.js`; keep the derived intent normalized before sending.

## Troubleshooting Tips
- Use the diagnostics panel toggle to watch connection state, latency, and outbound message counts.
- Heartbeat issues usually show up as missing `ack` timestamps—ensure the WebSocket stays open and `sendHeartbeat` is firing.
- If the client loses track of its player record, the status text will prompt a reconnect; inspect `/diagnostics` on the server for confirmation.
