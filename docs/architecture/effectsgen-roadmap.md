# Effect Contract Generation Roadmap

This document tracks the engineering work required to deliver the `effectsgen` toolchain and its supporting data sources defined in [`effectsgen-spec.md`](./effectsgen-spec.md). It is the single source of truth for status and the next concrete tasks.

## Roadmap

| Phase | Goal | Exit Criteria | Status |
| ----- | ---- | ------------- | ------ |
| 1 | Finalise Go contract registry | Central Go package owns all effect contracts and validation helpers; unit coverage in place. | 🟢 Done |
| 2 | JSON schema & catalog resolver | JSON schema validated; loader in `server/effects` resolves designer `entryId` → contract and caches lookups. | 🟢 Done |
| 3 | `tools/effectgen` TypeScript emitter | Deterministic TS output for payloads/enums/catalog metadata with golden-file tests; generator wired to CI drift checks. | 🟢 Done |
| 4 | Client integration of generated bindings | Generated bindings drive type authority and rendering paths in the live client; CI enforces regeneration drift checks. | 🟡 In progress |
| 5 | Client session orchestration | `client/main.ts` boots a `GameClientOrchestrator` backed by `WebSocketNetworkClient`, `InMemoryWorldStateStore`, and `CanvasRenderer`; UI mounts the renderer output and forwards lifecycle/keyframe events from network handlers. | ⚪ Planned |
| 6 | Input capture & command dispatch | `client/input.ts` implements `KeyboardInputController.register/unregister`; an `InputActionDispatcher` wires player intents/actions into `NetworkClient.send`, updating path/action payloads and honouring resync/ack flows in `client/client-manager.ts`. | 🟢 Done |
| 7 | Effect runtime playback integration | Replace placeholder canvas drawing with the JS effects runtime so lifecycle batches spawn catalog-driven animations via `tools/js-effects` definitions; renderer disposes instances on end events and reconciles `ContractLifecycleStore` state. | ⚪ Planned |

## Phase 4 – Client integration of generated bindings

### Completed Work

* 🟢 **Bootstrap orchestrator inside the live client shell**
  `client/main.ts` now constructs `GameClientOrchestrator` with `WebSocketNetworkClient`, `InMemoryWorldStateStore`, and `CanvasRenderer`, wiring the renderer host in `<game-canvas>` so lifecycle playback flows through the orchestrator entry point.
* 🟢 **Plumb WebSocket lifecycle events into the orchestrator**
  `client/network.ts` now forwards `state`, `keyframe`, `keyframeNack`, and heartbeat envelopes into `client/client-manager.ts`, which surfaces disconnect/errors through orchestrator handlers and client lifecycle callbacks.
* 🟢 **Mount renderer host and expose connection state in the shell**
  `client/main.ts` binds the renderer to `<game-canvas>`, streams batches into the canvas, and updates connection/heartbeat UI + styles in `client/styles.css` so operators can see status and errors in the live shell.
* 🟢 **Consume generated catalog metadata on the client**
  Client modules now import canonical catalog data from `client/generated/effect-contracts.ts`; join-time payloads are verified against the generated snapshot and all downstream helpers read from the shared store.
* 🟢 **Feed renderer from `ContractLifecycleStore`**
  Orchestrator now ingests WebSocket `state` batches into `ContractLifecycleStore` and emits render batches derived from generated catalog metadata/layers.
* 🟢 **Lifecycle renderer smoke tests**
  Headless harness replays recorded lifecycle batches and asserts renderer output derives from generated catalog metadata and managed ownership flags.
* 🟢 **effectsgen Go toolchain compatibility**
  Upgraded `golang.org/x/tools` (and indirect deps) so the generator builds cleanly with Go 1.24.3, restoring `npm run build` and client bundle output.
* 🟢 **Lifecycle smoke coverage for client-managed entries**
  Extended `client/__tests__/lifecycle-render-smoke.test.ts` to replay a `managedByClient` catalog entry and assert renderer metadata retains ownership after lifecycle end.
* 🟢 **Resync clearing coverage in renderer harness**
  Added a resync replay to `client/__tests__/lifecycle-render-smoke.test.ts` that asserts the lifecycle store clears retained entries and the renderer emits an empty frame when `payload.resync = true`.
* 🟢 **Shared harness helper reuse**
  Extracted reusable headless orchestrator helpers in `client/__tests__/helpers/headless-harness.ts` so both server- and client-managed smoke coverage share the same catalog-driven setup.

* 🟢 **Snapshot and tooling updates for client-managed flows**
  Surfaced `managedByClient` from `server/effect_catalog_metadata.go` and snapshot utilities so join payloads match the generated catalog bindings without manual overrides.

* 🟢 **Resync catalog snapshots mirror server metadata**
  `server/hub.go` now attaches `snapshotEffectCatalog` output to resync/keyframe configs and the client orchestrator reuses the network payload; smoke coverage asserts the renderer hydrates with the server-sent snapshot.

* 🟢 **Keyframe catalog hydration**
  `hub.HandleKeyframeRequest` now returns `snapshotEffectCatalog` payloads and the client orchestrator normalizes catalog snapshots from WebSocket keyframe replies with headless harness coverage.

* 🟢 **Keyframe NACK resync fallback**
  `server/hub.go` and `server/messages.go` attach `snapshotEffectCatalog` metadata to `keyframeNack` responses, schedule the next broadcast as a resync, and `client/client-manager.ts` resets the lifecycle store on NACK before rehydrating from the resync payload with smoke coverage in `client/__tests__/lifecycle-render-smoke.test.ts`.

* 🟢 **Keyframe retry scheduling after resync fallback**
  `client/client-manager.ts` defers keyframe re-requests until resync payloads are applied, throttles retries with an exponential backoff+jitter policy exposed via `ClientManagerConfiguration.keyframeRetryPolicy`, and harness coverage asserts a single retry is issued before rendering resumes.

* 🟢 **Keyframe request triggers for patch sequence gaps**
  `client/client-manager.ts` tracks lifecycle patch sequences, raises a keyframe request when the hub skips ahead, and the headless harness asserts only one retry is issued before playback catches up.

### Next Task

* _None queued; remain focused on Phase 6 command acknowledgement._

## Phase 5 – Client session orchestration

### Completed Work

* _Not yet started._

### Next Task

* _Pending Phase 4 exit._

## Phase 6 – Input capture & command dispatch

### Completed Work

* 🟢 **Keyboard input capture and dispatcher plumbing**
  `client/main.ts` boots `KeyboardInputController` with `InMemoryInputStore`, handing dispatch to `GameClientOrchestrator.createInputDispatcher`; tests in `client/__tests__/client-manager.test.ts` assert protocol metadata, pause-on-resync, and hook notifications for intents/path cancels.
* 🟢 **Pointer navigation command hooks**
  `client/main.ts` now routes canvas pointer interactions through the input dispatcher to emit `path` and `cancelPath` commands, updating `InMemoryInputStore` path state via dispatcher hooks with harness coverage validating resync pause behaviour.

* 🟢 **Track pointer path targets for UI feedback**
  `client/input.ts` persists the latest path targets, the orchestrator mirrors them into render batches, and resync handling clears stored targets so the canvas marker stays in sync with dispatcher state.
* 🟢 **Server tests align with command acknowledgement contract**
  `server/main_test.go` and `server/melee_command_pipeline_test.go` now destructure `(Command, bool, string)` so the command pipeline compiles against acknowledgement and rejection envelopes.

* 🟢 **Persist command rejection telemetry in the shell**
  `client/main.ts` now mirrors stored rejection state into the telemetry panel with styled feedback in `client/styles.css`, keeping the server-provided reason visible after the log scrolls by.
* 🟢 **Regression coverage for command sequencing and resync retries**
  Added harness scenarios in `client/__tests__/client-manager.test.ts` that replay pending commands across resync, verify retry metadata on rejection envelopes, and ensure dispatcher sequencing continues before accepting new input.

### Next Task

* _Phase 6 closed; prepare Phase 7 – Effect runtime playback integration._

**Acceptance criteria**

* Dispatcher assigns monotonic sequence identifiers to outbound commands and preserves order across reconnects.
* After a resync hydrate, the dispatcher replays buffered commands before accepting fresh input and requests any missing acknowledgements.
* Acknowledgement envelopes clear pending entries; rejects trigger targeted retries with updated payload state.

## Phase 7 – Effect runtime playback integration

### Completed Work

* _Not yet started._

### Next Task

* _Pending Phase 6 completion._

## Definition of Done

### Phase 4 – Client integration of generated bindings

Phase 4 is complete when all of the following hold:

1. **Generated contract authority**: All lifecycle types, catalog metadata, and enums used by the client originate from `client/generated/*`; no manually maintained mirrors or fallbacks remain.
2. **Network ingestion**: WebSocket lifecycle envelopes hydrate `ContractLifecycleStore` with correct cursor semantics, resync clears, and nack recovery driven by `client/client-manager.ts`.
3. **Renderer wiring**: Rendering paths consume only `ContractLifecycleStore` snapshots and generated catalog metadata; no legacy data flow or bespoke catalog parsing is exercised at runtime.
4. **CI gate**: Golden TypeScript comparisons, drift enforcement, JSON schema validation, and headless smoke playback all run in CI.

### Phase 5 – Client session orchestration

Phase 5 is complete when all of the following hold:

1. **Orchestrator boot flow**: `client/main.ts` initialises `GameClientOrchestrator` with a real `WebSocketNetworkClient`, `InMemoryWorldStateStore`, and `CanvasRenderer`, wiring lifecycle callbacks for connection state.
2. **Shell integration**: The live client shell mounts the renderer host, displays connection/heartbeat status, and surfaces disconnect/errors using orchestrator callbacks.
3. **Lifecycle forwarding**: WebSocket handlers route lifecycle/keyframe/nack/resync events into the orchestrator, which in turn maintains the world state store without manual glue modules.
4. **Smoke validation**: Automated smoke or integration coverage exercises a connect→render→disconnect loop and asserts renderer frames are produced from orchestrator-managed state.

### Phase 6 – Input capture & command dispatch

Phase 6 is complete when all of the following hold:

1. **Input device coverage**: `KeyboardInputController` and pointer hooks in `client/main.ts` forward intents/actions to an `InputActionDispatcher` without bypassing orchestrator state.
2. **Command lifecycle**: `InputActionDispatcher` tracks pending commands, clears them on acknowledgement, and halts/flushes dispatch on resync events from `client/client-manager.ts`.
3. **UI feedback loop**: `InMemoryInputStore` (or successor) exposes the data required for UI feedback (active paths, pending actions, rejection reasons) and keeps it consistent through retries.
4. **Regression coverage**: Tests under `client/__tests__` cover happy-path dispatch, rejection retries, and resync pause behaviour for both keyboard and pointer flows.

### Phase 7 – Effect runtime playback integration

Phase 7 is complete when all of the following hold:

1. **Runtime swap**: The canvas renderer swaps placeholder drawing routines for the JS effects runtime backed by `tools/js-effects` definitions.
2. **Lifecycle ownership**: Lifecycle batches spawn and dispose runtime instances according to `ContractLifecycleStore` data, including managed-by-client entries.
3. **Resource hygiene**: Renderer disposes runtime resources on lifecycle end, resync, and disconnect, preventing leaks across frames.
4. **End-to-end tests**: Automated playback harnesses validate that catalog-driven animations appear and settle as expected for representative `entryId` scenarios.

## Reference Map (authoritative paths)

* **Contracts & schema**: `server/effects/contract`, `docs/contracts/effects.schema.json`
* **Catalog**: `server/effects/catalog` (loader/validation), `config/effects/definitions.json`
* **Generator**: `tools/effectsgen` → `client/generated/effect-contracts.ts` (payloads, enums, catalog metadata)
* **Client runtime**:

  * Store: `client/effect-lifecycle-store.ts`
  * Orchestrator: `client/client-manager.ts` (hydrates store from network)
  * Network plumbing: `client/network.ts`, `client/main.ts`
  * Rendering: `client/render.ts` (reads store snapshots & catalog metadata)

