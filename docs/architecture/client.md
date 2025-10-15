# Client Architecture

The browser client is delivered as a set of ES modules that the Go server serves verbatim.
`index.html` declares the canvas, HUD labels, world reset controls, and mounts a custom
`<game-client-app>` element; loading `main.js` bootstraps every other module without a
bundler or framework build step. [client/index.html](../../client/index.html) [client/main.js](../../client/main.js)

## Module Map

- [`client/main.js`](../../client/main.js) – Owns the shared `store`, renders the Lit-based
  control panels, wires diagnostics, manages the world reset form, and kicks off input,
  rendering, and networking.
- [`client/network.js`](../../client/network.js) – Handles the `/join` handshake, websocket
  lifecycle, patch/keyframe bookkeeping, effect trigger queues, outbound message helpers, and
  world reset requests.
- [`client/render.js`](../../client/render.js) – Interpolates entity positions, updates the
  shared `EffectManager`, draws the scene, and feeds contract lifecycle data straight into the
  effect definitions now that the legacy snapshot array has been retired.
- [`client/patches.js`](../../client/patches.js) – Maintains the incremental patch baseline,
  applies journal batches, and tracks recovery metrics for diagnostics.
- [`client/input.js`](../../client/input.js) – Normalises keyboard input into intents and
  actions, handles click-to-move cancellation, and exposes the camera toggle shortcut.
- [`client/heartbeat.js`](../../client/heartbeat.js) – Provides `computeRtt` and a reusable
  interval wrapper for websocket heartbeats.
- [`client/effect-lifecycle.js`](../../client/effect-lifecycle.js) & companions manage the
  authoritative contract lifecycle cache, translate events for rendering, and track diagnostics
  when updates arrive out of order. [client/effect-diagnostics.js](../../client/effect-diagnostics.js)
- [`client/effect-lifecycle-translator.js`](../../client/effect-lifecycle-translator.js) and
  [`client/effect-manager-adapter.js`](../../client/effect-manager-adapter.js) convert contract
  payloads into js-effects spawn/update requests and mirror live instances for debugging.
- [`client/render-modes.js`](../../client/render-modes.js) defines the patch-vs-snapshot
  rendering modes exposed in the debug UI.
- [`client/js-effects/`](../../client/js-effects) hosts the generated effect definitions. Edit
  the TypeScript sources under `tools/js-effects/` and rebuild to change them.

## Boot Sequence

1. `index.html` draws the static shell (canvas, HUD labels, world controls) and loads
   `main.js` as an ES module. [client/index.html](../../client/index.html)
2. `main.js` registers the Lit `<game-client-app>` tabs, builds the initial `store`, exposes
   debug helpers on `window`, and defines the DOM-rendering templates for the telemetry,
   world reset, and inventory panels. [client/main.js](../../client/main.js)
3. The `bootstrap()` routine queries the DOM, attaches listeners (debug panel toggle, latency
   override, render mode controls, world reset form, canvas pathing), initialises diagnostics,
   renders the panels, registers keyboard handlers, starts the render loop, and finally calls
   `joinGame(store)` to enter the simulation. [client/main.js](../../client/main.js)

## Shared Store Structure

The plain-object `store` defined in `main.js` is imported by every module; it gathers mutable
state and helper callbacks so subsystems can cooperate without a framework runtime. [client/main.js](../../client/main.js)

- **UI references:** Elements for status text, latency readouts, diagnostics grid, HUD labels,
  world reset inputs, and inventory slots are cached for quick updates. [client/main.js](../../client/main.js)
- **Simulation state:** Authoritative maps for players, NPCs, obstacles, effects, ground items,
  world dimensions, and camera configuration feed both rendering and diagnostics. [client/main.js](../../client/main.js)
- **Networking telemetry:** The store tracks websocket handles, last tick/timestamps, intent
  send times, heartbeat metrics, message counters, and the currently selected render mode so
  diagnostics stay in sync with actual traffic. [client/main.js](../../client/main.js)
- **Effects pipeline:** `effectManager`, lifecycle summary, pending effect triggers, and the
  derived registry of active instances are shared between `network.js` (which queues contract
  data) and `render.js` (which consumes it). [client/main.js](../../client/main.js) [client/network.js](../../client/network.js) [client/render.js](../../client/render.js)
- **Patch tester:** `patchState` and related flags (`lastRecovery`, deferred patch counts, keyframe
  cadence) mirror the incremental diff subsystem so the debug panel can display recovery status
  alongside snapshot cadence controls. [client/main.js](../../client/main.js)
- **World configuration & inventory:** The current config, dirty-field tracking, inventory slot
  limits, and the last console acknowledgement feed the world reset form and inventory panel.
  `renderInventory` reads player inventory data and regenerates the slot grid whenever the
  authoritative snapshot changes. [client/main.js](../../client/main.js)

Helper methods such as `store.setRenderMode`, `store.setLatency`, `store.updateDiagnostics`, and
`store.updateWorldConfigUI` expose the behaviours other modules need without requiring direct DOM
access. [client/main.js](../../client/main.js)

## Networking Pipeline

- **Join handshake:** `joinGame(store)` POSTs `/join`, normalises players/NPCs/ground items,
  resets lifecycle caches and patch state, mirrors world configuration, seeds display maps, and
  opens the websocket connection once the response succeeds. [client/network.js](../../client/network.js)
- **Websocket handling:** `connectEvents` constructs the `/ws` URL, resets telemetry counters,
  listens for `state`, `heartbeat`, `console_ack`, and keyframe messages, and tears down the
  session when errors occur. [client/network.js](../../client/network.js)
- **State application:** Each `state` message optionally runs through the patch tester, applies
  the resulting snapshot, queues effect triggers, updates lifecycle caches, reconciles world
  configuration, and refreshes display maps plus diagnostics. [client/network.js](../../client/network.js)
- **Outbound messages:** `sendCurrentIntent`, `sendMoveTo`, `sendCancelPath`, `sendAction`,
  `sendConsoleCommand`, and `setKeyframeCadence` all delegate to `sendMessage`, which stamps the
  protocol version, includes the last acknowledged tick, applies simulated latency, and updates
  diagnostics counters. [client/network.js](../../client/network.js)
- **Heartbeats:** `startHeartbeat(store)` (built on `createHeartbeat`) pings every two seconds;
  acknowledgements update `latencyMs`, RTT history, and HUD labels. [client/network.js](../../client/network.js) [client/heartbeat.js](../../client/heartbeat.js)
- **Keyframes & recovery:** `syncPatchTestingState` and the keyframe retry loop request full
  snapshots when diffs cannot be applied, logging recovery progress and NACK counts so the debug
  panel can report issues. [client/network.js](../../client/network.js) [client/patches.js](../../client/patches.js)
- **Effect triggers:** Contract-driven `effectTriggers` batches are deduplicated by ID and queued
  in `store.pendingEffectTriggers`; `render.js` drains and dispatches them through the
  `EffectManager` each frame. [client/network.js](../../client/network.js) [client/render.js](../../client/render.js)
- **World resets:** Submitting the world reset form calls `resetWorld(store, config)`, which posts
  `/world/reset`, normalises the returned config, and refreshes the UI once the server confirms the
  change. [client/network.js](../../client/network.js) [client/main.js](../../client/main.js)

## Input & Player Interaction

`registerInputHandlers(store)` attaches `keydown`/`keyup` listeners that maintain a pressed-key set,
derive facing, cancel click-to-move when WASD resumes, and dispatch intents or actions (space for
melee, `F` for fireball). The `C` key toggles the camera lock via a store callback. [client/input.js](../../client/input.js)

`initializeCanvasPathing()` listens for pointer clicks on the canvas, translates screen-space
coordinates into world positions, clamps them to map bounds, and invokes `sendMoveTo` for
server-driven pathing. [client/main.js](../../client/main.js) [client/network.js](../../client/network.js)

## Rendering & Effects

`startRenderLoop(store)` records animation timestamps, lerps display maps toward authoritative
positions, keeps the optional camera lock centred on the local player, and calls `drawScene` each
frame. [client/render.js](../../client/render.js) The renderer delegates combat visuals to a shared
`EffectManager` instance, ensuring contract lifecycle entries are mirrored into js-effects
definitions:

- `collectEffectRenderBuckets` builds per-type render queues straight from lifecycle entries,
  including the `recentlyEnded` cache carried between frames. `contractLifecycleToEffect` and
  `contractLifecycleToUpdatePayload` translate contract metadata into spawn/update payloads that
  definitions such as melee swings, fire zones, and fireballs understand. [client/render.js](../../client/render.js) [client/effect-lifecycle-translator.js](../../client/effect-lifecycle-translator.js)
- `mirrorEffectInstances` keeps a map of active `EffectManager` instances keyed by contract ID for
  debugging, while `ensureEffectLifecycleState` tracks authoritative sequences and provides
  summaries for diagnostics. [client/effect-manager-adapter.js](../../client/effect-manager-adapter.js) [client/effect-lifecycle.js](../../client/effect-lifecycle.js)
- `drainPendingEffectTriggers` forwards queued triggers (such as blood splatter decals) to the
  manager before syncing by type, guaranteeing fire-and-forget effects run once even if the
  websocket batches arrive late. [client/render.js](../../client/render.js)

When extending visuals, add or update js-effects definitions under `tools/js-effects/`, rebuild the
bundle, and wire the new type into `render.js` via `syncEffectsByType` and any trigger handlers
required for custom behaviours. [client/render.js](../../client/render.js)

## Diagnostics & Control Panels

The `<game-client-app>` Lit element renders the three-tab interface (Telemetry, World Controls,
Inventory), persists the active tab to `localStorage`, and exposes ARIA-friendly keyboard navigation
for the tablist. [client/main.js](../../client/main.js) The telemetry panel hosts the debug toggle, live
diagnostics grid, render mode buttons, and keyframe cadence input driven by store callbacks. [client/main.js](../../client/main.js)

`updateDiagnostics()` assembles status strings for connection state, tick counters, input timing,
heartbeats, patch status, and effect lifecycle anomalies; HUD chips share the same data so the
canvas overlay and drawer stay aligned. [client/main.js](../../client/main.js) Console helpers on `window`
(`debugDropGold`, `debugPickupGold`, `debugNetworkStats`, render mode toggles) reuse the networking
helpers to exercise server-side admin commands. [client/main.js](../../client/main.js)

The world controls tab reflects the authoritative configuration, tracks user edits, and invokes
`resetWorld`; dirty-state tracking ensures server responses overwrite the form only after successful
updates. [client/main.js](../../client/main.js) The inventory tab renders the local player's slots with
icons and quantities, expanding automatically when the server grants more inventory positions.
[client/main.js](../../client/main.js)

## Patch & Snapshot Processing

`createPatchState()` and `updatePatchState()` maintain both the baseline snapshot and the patched
view, deduplicate entity updates by sequence, cache recent keyframes, and record recovery attempts.
`network.js` calls `syncPatchTestingState()` for every join/state/keyframe message so diagnostics can
show baseline ticks, batch counts, and recovery outcomes alongside the main simulation state.
[client/patches.js](../../client/patches.js) [client/network.js](../../client/network.js)

## Extending the Client Safely

- **New UI controls:** Render the markup inside `main.js` templates, register references in the
  store, and update `updateDiagnostics()` or other helpers so state changes propagate.
- **Protocol changes:** Mirror new payload fields in `joinGame` and `applyStateSnapshot`, then
  expose them through the store before updating the renderer or diagnostics. [client/network.js](../../client/network.js)
- **Gameplay visuals:** Add js-effects definitions, translate any new contract metadata in
  `effect-lifecycle-translator.js`, and register sync/trigger handlers in `render.js`.
- **Input bindings:** Extend `registerInputHandlers()` to keep intent normalisation consistent and
  update diagnostics so testers can see the new actions firing. [client/input.js](../../client/input.js)
- **Patch tooling:** When adjusting diff semantics, update `patches.js`, surface new telemetry in
  the diagnostics grid, and ensure the request/retry helpers stay in sync with the server logic.

Keeping the documentation aligned with these modules makes it easier for contributors to orient
themselves and evolve the Mine & Die client confidently.
