# Client Architecture

The client is a lightweight ES module bundle served directly from the Go server. No bundler or framework is required—open `index.html` and the modules bootstrap themselves.

## Module Overview
- `index.html` – Declares the canvas, status text, diagnostics panel, and loads `main.js` via `<script type="module">`.
- `main.js` – Builds the shared `store` object, wires diagnostics UI (including the inventory grid and world reset form), seeds the patch-testing container, and starts input, render, and networking flows.【F:client/main.js†L1-L210】 The debug panel exposes toggles, spawn counts, and a deterministic seed input so QA can regenerate identical layouts on demand.【F:client/main.js†L17-L210】
- `network.js` – Handles the `/join` handshake, WebSocket lifecycle, diff + keyframe plumbing, heartbeat timers, and outbound message helpers (including console commands). It also normalises snapshots into store dictionaries, applies contract lifecycle batches, and keeps diagnostics in sync.【F:client/network.js†L905-L1341】
- `effect-lifecycle.js` – Owns the client-side registry of active contract effects, applies lifecycle batches in a two-pass order, and exposes helpers for diagnostics and rendering integration.【F:client/effect-lifecycle.js†L224-L415】
- `heartbeat.js` – Provides `computeRtt` and the reusable `createHeartbeat` interval helper for latency tracking.【F:client/heartbeat.js†L1-L41】
- `patches.js` – Maintains a background snapshot, applies server patch batches, tracks keyframe recovery, and surfaces replay diagnostics for the debug panel.【F:client/patches.js†L1-L200】【F:client/patches.js†L720-L964】
- `input.js` – Converts keyboard events into normalized intents and action messages.
- `render.js` – Performs `<canvas>` drawing, lerps network state to display positions, and renders effects/obstacles.
- `styles.css` – Minimal styling for layout and diagnostics readouts.
- `vendor/` – Drop-in ES modules for third-party helpers. Import them from client code via `import … from "./vendor/<file>.js"`.

## Store Shape
`main.js` creates a plain object (`store`) shared across modules. Key fields:
- DOM references (canvas, status text, diagnostics elements).
- Simulation constants (`TILE_SIZE`, `PLAYER_SIZE`, etc.).
- Connection state (`socket`, `playerId`, heartbeat timestamps, last authoritative tick).
  - Player dictionaries: `players` (authoritative) and `displayPlayers` (interpolated positions).
  - NPC dictionaries: `npcs` mirrors neutral enemies from the server, `displayNPCs` lerps their positions for rendering.
  - Arrays for `obstacles` and `effects` plus a `groundItems` map so the renderer can draw dropped loot with quantities.【F:client/main.js†L153-L210】【F:client/render.js†L338-L405】
  - Effect runtime: a shared `effectManager` instance drives all combat visuals while
    `pendingEffectTriggers` / `processedEffectTriggerIds` ensure fire-and-forget payloads are
    applied exactly once. Contract batches feed `lastEffectLifecycleSummary` and
    `effectDiagnostics` so the diagnostics drawer can report dropped or unknown events.【F:client/main.js†L728-L777】【F:client/network.js†L1126-L1341】
  - Patch testing: `patchState` stores the diff baseline, error history, pending keyframe requests, and replay queues surfaced in the diagnostics panel.【F:client/main.js†L193-L210】【F:client/network.js†L841-L899】【F:client/index.html†L288-L315】
  - Heartbeat + latency metadata (`lastHeartbeatSentAt`, `lastHeartbeatAckAt`, `latencyMs`, `hudNetworkEls`) keeps HUD chips and diagnostics current.【F:client/main.js†L161-L210】【F:client/network.js†L905-L1141】
  - `worldConfig` mirrors the server's toggles, counts, and seed so the world reset form reflects authoritative values.【F:client/main.js†L78-L210】【F:client/network.js†L943-L1086】

## Initialization Sequence
1. `main.js` prepares UI helpers, attaches diagnostics toggles, initialises the patch-testing container, and registers the latency override input.【F:client/main.js†L1-L210】
2. `registerInputHandlers(store)` tracks pressed keys and sends intents/actions.
3. `startRenderLoop(store)` animates the canvas, interpolating toward server positions every frame.
4. `joinGame(store)` POSTs `/join`, normalises the response into the store (players, NPCs, ground items, config, effect triggers), seeds the patch baseline, and opens the WebSocket.【F:client/network.js†L943-L1050】
5. `connectEvents(store)` sets up WebSocket callbacks, starts heartbeats, and keeps patch/keyframe bookkeeping in sync with every message.【F:client/network.js†L1001-L1141】

## Networking Details
- **State updates:** The server emits `state` messages containing players, NPCs, obstacles, optional `effectTriggers`, optional `groundItems`, lifecycle arrays (`effect_spawned`/`effect_update`/`effect_ended` with cursors), journaled `patches`, the current tick (`t`), monotonic `sequence`/`keyframeSeq`, `serverTime`, the active world `config`, and a `resync` flag. The legacy `effects` array is no longer sent; the client rebuilds render payloads from the lifecycle state. The client applies the snapshot, queues effect triggers, runs `applyEffectLifecycleBatch`, advances the patch baseline, and updates diagnostics + HUD tick counters.【F:server/messages.go†L18-L39】【F:client/network.js†L1231-L1339】【F:client/effect-lifecycle.js†L272-L415】
- **Intents:** `sendCurrentIntent` serializes `{ type: "input", dx, dy, facing }` whenever movement or facing changes.
- **Path navigation:** `sendMoveTo` sends `{ type: "path", x, y }` for click-to-move requests while `sendCancelPath` clears the server-driven route when WASD input resumes.
- **Actions:** `sendAction` is used by `input.js` for melee and fireball triggers.
- **Console:** `sendConsoleCommand` dispatches `{ type: "console", cmd, qty? }` so testers can drop or pick up gold via the debug overlay; the handler logs acknowledgements with quantities and stack IDs for traceability.【F:client/network.js†L160-L187】【F:client/network.js†L611-L645】
- **Heartbeats:** `startHeartbeat` sets an interval that calls `sendHeartbeat`; acknowledgements update latency displays, including the HUD's `RTT: ## ms` chip fed by the latest round-trip measurement.【F:client/network.js†L905-L1141】【F:client/heartbeat.js†L15-L41】
- **Keyframes:** When the diff pipeline needs recovery, `requestKeyframeSnapshot` emits `{ type: "keyframeRequest", keyframeSeq, keyframeTick? }` and retry loops keep asking until the journal serves the frame or forces a resync.【F:client/network.js†L790-L839】
- Every outbound payload passes through `sendMessage`, which stamps the negotiated protocol version and the last applied tick acknowledgement so the hub can track client progress.【F:client/network.js†L905-L940】
- **Reconnects:** Socket closure funnels through `handleConnectionLoss`, which resets state and schedules `joinGame` again.

## Rendering Notes
- Grid + background are redrawn each frame for clarity.
- Players are drawn as colored squares with a facing indicator line; the local player uses cyan/white, others orange/cream.
- NPCs are drawn in violet with their facing indicator and optional type label.
- Obstacles use either a stone block style or a gold ore treatment with deterministic pseudo-random nuggets.
- All combat visuals run through the shared js-effects `EffectManager`. `render.js` calls the
  generic `syncEffectsByType` helper for each definition (melee swings, lingering fire, and the
  rectangular fireball trail) then delegates culling, sorting, updates, and drawing entirely to the
  manager.
- Definitions in `client/js-effects/effects/` expose a `fromEffect` helper used by
  `syncEffectsByType` to translate authoritative payloads into spawn options. Add new types by
  shipping a definition with `fromEffect` plus any custom `onUpdate` callback required to keep the
  instance aligned with simulation state.
- Fire-and-forget triggers drain from `store.pendingEffectTriggers` each frame and are dispatched
  through `EffectManager.triggerAll`. Registered handlers only receive `(manager, trigger, context)`
  and should call `manager.spawn()` directly—no ad-hoc maps or stores are needed.
- Effects that hand off decals call `handoffToDecal()` when they expire. `EffectManager.collectDecals`
  converts those into long-lived decal instances on the ground layer so `render.js` no longer keeps a
  separate TTL queue.
- Ground items draw after actors so loot stacks remain legible; the renderer formats coins with quantities based on `store.groundItems` and the active tile size.【F:client/render.js†L338-L405】
- When extending the js-effects runtime (new definitions, manager helpers, etc.), make the changes
  in the TypeScript sources under `tools/js-effects/packages/effects-lib` and run `npm run build`
  from the repository root. This regenerates the vendored modules in `client/js-effects/`, so edits
  made directly in the client copy will be overwritten.

## Extending the Client
- Add new HUD elements to `index.html`, register them in the `store`, and update `main.js` diagnostics helpers.
- Mirror new server fields by adjusting the payload handling inside `network.js` (state, actions, heartbeat logic).
- Expand rendering logic by extending `render.js`; prefer pure functions that read from the `store` to keep coordination simple.
- Route new combat visuals through js-effects. Reuse the existing melee swing integration as a template and update the
  helper maps/cleanup logic in `network.js` if additional tracked instances are required.
- New input bindings belong in `input.js`; keep the derived intent normalized before sending.

## Troubleshooting Tips
- Use the diagnostics panel toggle to watch connection state, latency, outbound message counts, and patch replay stats surfaced from `patchState`.【F:client/index.html†L288-L315】【F:client/main.js†L420-L620】
- The diagnostics drawer also mirrors the background patch baseline, replay summary, and entity counts so you can compare snapshot and diff pipelines at a glance without inspecting the console.【F:client/index.html†L288-L315】【F:client/main.js†L420-L620】
- Contract lifecycle metrics live next to patch stats—expand the diagnostics drawer to inspect `lastEffectLifecycleSummary` and confirm spawns/updates/ends are flowing for each tick.【F:client/main.js†L728-L777】【F:client/network.js†L1126-L1341】
- Heartbeat issues usually show up as missing `ack` timestamps—ensure the WebSocket stays open and `sendHeartbeat` is firing.
- If the client loses track of its player record, the status text will prompt a reconnect; inspect `/diagnostics` on the server for confirmation.
