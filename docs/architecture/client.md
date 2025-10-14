# Client Architecture

The browser client is delivered as a set of ES modules that the Go server serves verbatim.
`index.html` declares the canvas, HUD labels, world reset controls, and mounts a custom
`<game-client-app>` element; loading `main.js` bootstraps every other module without a
bundler or framework build step.【F:client/index.html†L1-L65】【F:client/main.js†L1-L37】

## Module Map

- [`client/main.js`](../../client/main.js) – Owns the shared `store`, renders the Lit-based
  control panels, wires diagnostics, manages the world reset form, and kicks off input,
  rendering, and networking.【F:client/main.js†L1-L210】【F:client/main.js†L689-L793】
- [`client/network.js`](../../client/network.js) – Handles the `/join` handshake, websocket
  lifecycle, patch/keyframe bookkeeping, effect trigger queues, outbound message helpers, and
  world reset requests.【F:client/network.js†L1121-L1320】【F:client/network.js†L1366-L1440】
- [`client/render.js`](../../client/render.js) – Interpolates entity positions, updates the
  shared `EffectManager`, draws the scene, and feeds contract lifecycle data straight into the
  effect definitions now that the legacy snapshot array has been retired.【F:client/render.js†L489-L540】【F:client/render.js†L820-L867】
- [`client/patches.js`](../../client/patches.js) – Maintains the incremental patch baseline,
  applies journal batches, and tracks recovery metrics for diagnostics.【F:client/patches.js†L999-L1108】
- [`client/input.js`](../../client/input.js) – Normalises keyboard input into intents and
  actions, handles click-to-move cancellation, and exposes the camera toggle shortcut.【F:client/input.js†L14-L124】
- [`client/heartbeat.js`](../../client/heartbeat.js) – Provides `computeRtt` and a reusable
  interval wrapper for websocket heartbeats.【F:client/heartbeat.js†L1-L41】
- [`client/effect-lifecycle.js`](../../client/effect-lifecycle.js) & companions manage the
  authoritative contract lifecycle cache, translate events for rendering, and track diagnostics
  when updates arrive out of order.【F:client/effect-lifecycle.js†L224-L415】【F:client/effect-diagnostics.js†L61-L88】
- [`client/effect-lifecycle-translator.js`](../../client/effect-lifecycle-translator.js) and
  [`client/effect-manager-adapter.js`](../../client/effect-manager-adapter.js) convert contract
  payloads into js-effects spawn/update requests and mirror live instances for debugging.【F:client/effect-lifecycle-translator.js†L1-L200】【F:client/effect-manager-adapter.js†L1-L65】
- [`client/render-modes.js`](../../client/render-modes.js) defines the patch-vs-snapshot
  rendering modes exposed in the debug UI.【F:client/render-modes.js†L1-L21】
- [`client/js-effects/`](../../client/js-effects) hosts the generated effect definitions. Edit
  the TypeScript sources under `tools/js-effects/` and rebuild to change them.

## Boot Sequence

1. `index.html` draws the static shell (canvas, HUD labels, world controls) and loads
   `main.js` as an ES module.【F:client/index.html†L11-L65】
2. `main.js` registers the Lit `<game-client-app>` tabs, builds the initial `store`, exposes
   debug helpers on `window`, and defines the DOM-rendering templates for the telemetry,
   world reset, and inventory panels.【F:client/main.js†L29-L433】【F:client/main.js†L689-L807】
3. The `bootstrap()` routine queries the DOM, attaches listeners (debug panel toggle, latency
   override, render mode controls, world reset form, canvas pathing), initialises diagnostics,
   renders the panels, registers keyboard handlers, starts the render loop, and finally calls
   `joinGame(store)` to enter the simulation.【F:client/main.js†L891-L1899】

## Shared Store Structure

The plain-object `store` defined in `main.js` is imported by every module; it gathers mutable
state and helper callbacks so subsystems can cooperate without a framework runtime.【F:client/main.js†L689-L793】

- **UI references:** Elements for status text, latency readouts, diagnostics grid, HUD labels,
  world reset inputs, and inventory slots are cached for quick updates.【F:client/main.js†L689-L763】【F:client/main.js†L1890-L1950】
- **Simulation state:** Authoritative maps for players, NPCs, obstacles, effects, ground items,
  world dimensions, and camera configuration feed both rendering and diagnostics.【F:client/main.js†L728-L787】【F:client/main.js†L960-L1021】
- **Networking telemetry:** The store tracks websocket handles, last tick/timestamps, intent
  send times, heartbeat metrics, message counters, and the currently selected render mode so
  diagnostics stay in sync with actual traffic.【F:client/main.js†L748-L776】【F:client/main.js†L1101-L1156】
- **Effects pipeline:** `effectManager`, lifecycle summary, pending effect triggers, and the
  derived registry of active instances are shared between `network.js` (which queues contract
  data) and `render.js` (which consumes it).【F:client/main.js†L744-L781】【F:client/network.js†L812-L870】【F:client/render.js†L820-L835】
- **Patch tester:** `patchState` and related flags (`lastRecovery`, deferred patch counts, keyframe
  cadence) mirror the incremental diff subsystem so the debug panel can display recovery status
  alongside snapshot cadence controls.【F:client/main.js†L789-L793】【F:client/main.js†L1101-L1170】
- **World configuration & inventory:** The current config, dirty-field tracking, inventory slot
  limits, and the last console acknowledgement feed the world reset form and inventory panel.
  `renderInventory` reads player inventory data and regenerates the slot grid whenever the
  authoritative snapshot changes.【F:client/main.js†L786-L793】【F:client/main.js†L1501-L1760】

Helper methods such as `store.setRenderMode`, `store.setLatency`, `store.updateDiagnostics`, and
`store.updateWorldConfigUI` expose the behaviours other modules need without requiring direct DOM
access.【F:client/main.js†L828-L878】【F:client/main.js†L1023-L1099】【F:client/main.js†L1864-L1899】

## Networking Pipeline

- **Join handshake:** `joinGame(store)` POSTs `/join`, normalises players/NPCs/ground items,
  resets lifecycle caches and patch state, mirrors world configuration, seeds display maps, and
  opens the websocket connection once the response succeeds.【F:client/network.js†L1121-L1218】
- **Websocket handling:** `connectEvents` constructs the `/ws` URL, resets telemetry counters,
  listens for `state`, `heartbeat`, `console_ack`, and keyframe messages, and tears down the
  session when errors occur.【F:client/network.js†L1230-L1371】
- **State application:** Each `state` message optionally runs through the patch tester, applies
  the resulting snapshot, queues effect triggers, updates lifecycle caches, reconciles world
  configuration, and refreshes display maps plus diagnostics.【F:client/network.js†L1270-L1339】
- **Outbound messages:** `sendCurrentIntent`, `sendMoveTo`, `sendCancelPath`, `sendAction`,
  `sendConsoleCommand`, and `setKeyframeCadence` all delegate to `sendMessage`, which stamps the
  protocol version, includes the last acknowledged tick, applies simulated latency, and updates
  diagnostics counters.【F:client/network.js†L1070-L1118】【F:client/network.js†L1374-L1434】
- **Heartbeats:** `startHeartbeat(store)` (built on `createHeartbeat`) pings every two seconds;
  acknowledgements update `latencyMs`, RTT history, and HUD labels.【F:client/network.js†L905-L1141】【F:client/heartbeat.js†L1-L41】
- **Keyframes & recovery:** `syncPatchTestingState` and the keyframe retry loop request full
  snapshots when diffs cannot be applied, logging recovery progress and NACK counts so the debug
  panel can report issues.【F:client/network.js†L934-L1057】【F:client/patches.js†L1038-L1108】
- **Effect triggers:** Contract-driven `effectTriggers` batches are deduplicated by ID and queued
  in `store.pendingEffectTriggers`; `render.js` drains and dispatches them through the
  `EffectManager` each frame.【F:client/network.js†L812-L870】【F:client/render.js†L820-L835】
- **World resets:** Submitting the world reset form calls `resetWorld(store, config)`, which posts
  `/world/reset`, normalises the returned config, and refreshes the UI once the server confirms the
  change.【F:client/network.js†L1429-L1489】【F:client/main.js†L1648-L1698】

## Input & Player Interaction

`registerInputHandlers(store)` attaches `keydown`/`keyup` listeners that maintain a pressed-key set,
derive facing, cancel click-to-move when WASD resumes, and dispatch intents or actions (space for
melee, `F` for fireball). The `C` key toggles the camera lock via a store callback.【F:client/input.js†L14-L124】

`initializeCanvasPathing()` listens for pointer clicks on the canvas, translates screen-space
coordinates into world positions, clamps them to map bounds, and invokes `sendMoveTo` for
server-driven pathing.【F:client/main.js†L891-L929】【F:client/network.js†L1391-L1414】

## Rendering & Effects

`startRenderLoop(store)` records animation timestamps, lerps display maps toward authoritative
positions, keeps the optional camera lock centred on the local player, and calls `drawScene` each
frame.【F:client/render.js†L488-L540】 The renderer delegates combat visuals to a shared
`EffectManager` instance, ensuring contract lifecycle entries are mirrored into js-effects
definitions:

- `collectEffectRenderBuckets` builds per-type render queues straight from lifecycle entries,
  including the `recentlyEnded` cache carried between frames. `contractLifecycleToEffect` and
  `contractLifecycleToUpdatePayload` translate contract metadata into spawn/update payloads that
  definitions such as melee swings, fire zones, and fireballs understand.【F:client/render.js†L306-L421】【F:client/effect-lifecycle-translator.js†L93-L200】
- `mirrorEffectInstances` keeps a map of active `EffectManager` instances keyed by contract ID for
  debugging, while `ensureEffectLifecycleState` tracks authoritative sequences and provides
  summaries for diagnostics.【F:client/effect-manager-adapter.js†L7-L43】【F:client/effect-lifecycle.js†L224-L405】
- `drainPendingEffectTriggers` forwards queued triggers (such as blood splatter decals) to the
  manager before syncing by type, guaranteeing fire-and-forget effects run once even if the
  websocket batches arrive late.【F:client/render.js†L820-L948】

When extending visuals, add or update js-effects definitions under `tools/js-effects/`, rebuild the
bundle, and wire the new type into `render.js` via `syncEffectsByType` and any trigger handlers
required for custom behaviours.【F:client/render.js†L839-L867】

## Diagnostics & Control Panels

The `<game-client-app>` Lit element renders the three-tab interface (Telemetry, World Controls,
Inventory), persists the active tab to `localStorage`, and exposes ARIA-friendly keyboard navigation
for the tablist.【F:client/main.js†L436-L540】 The telemetry panel hosts the debug toggle, live
diagnostics grid, render mode buttons, and keyframe cadence input driven by store callbacks.【F:client/main.js†L45-L210】【F:client/main.js†L1142-L1200】

`updateDiagnostics()` assembles status strings for connection state, tick counters, input timing,
heartbeats, patch status, and effect lifecycle anomalies; HUD chips share the same data so the
canvas overlay and drawer stay aligned.【F:client/main.js†L1023-L1170】 Console helpers on `window`
(`debugDropGold`, `debugPickupGold`, `debugNetworkStats`, render mode toggles) reuse the networking
helpers to exercise server-side admin commands.【F:client/main.js†L795-L888】

The world controls tab reflects the authoritative configuration, tracks user edits, and invokes
`resetWorld`; dirty-state tracking ensures server responses overwrite the form only after successful
updates.【F:client/main.js†L1501-L1698】 The inventory tab renders the local player's slots with
icons and quantities, expanding automatically when the server grants more inventory positions.
【F:client/main.js†L1701-L1760】

## Patch & Snapshot Processing

`createPatchState()` and `updatePatchState()` maintain both the baseline snapshot and the patched
view, deduplicate entity updates by sequence, cache recent keyframes, and record recovery attempts.
`network.js` calls `syncPatchTestingState()` for every join/state/keyframe message so diagnostics can
show baseline ticks, batch counts, and recovery outcomes alongside the main simulation state.
【F:client/patches.js†L999-L1108】【F:client/network.js†L934-L1057】

## Extending the Client Safely

- **New UI controls:** Render the markup inside `main.js` templates, register references in the
  store, and update `updateDiagnostics()` or other helpers so state changes propagate.
- **Protocol changes:** Mirror new payload fields in `joinGame` and `applyStateSnapshot`, then
  expose them through the store before updating the renderer or diagnostics.【F:client/network.js†L1121-L1339】
- **Gameplay visuals:** Add js-effects definitions, translate any new contract metadata in
  `effect-lifecycle-translator.js`, and register sync/trigger handlers in `render.js`.
- **Input bindings:** Extend `registerInputHandlers()` to keep intent normalisation consistent and
  update diagnostics so testers can see the new actions firing.【F:client/input.js†L14-L124】
- **Patch tooling:** When adjusting diff semantics, update `patches.js`, surface new telemetry in
  the diagnostics grid, and ensure the request/retry helpers stay in sync with the server logic.

Keeping the documentation aligned with these modules makes it easier for contributors to orient
themselves and evolve the Mine & Die client confidently.
