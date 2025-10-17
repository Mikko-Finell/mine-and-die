# Effect Contract Generation Roadmap

This document tracks the engineering work required to deliver the `effectsgen` toolchain and its supporting data sources defined in [`effectsgen-spec.md`](./effectsgen-spec.md). It is the single source of truth for status and the next concrete tasks.

## Roadmap

| Phase | Goal | Exit Criteria | Status |
| ----- | ---- | ------------- | ------ |
| 1 | Finalise Go contract registry | Central Go package owns all effect contracts and validation helpers; unit coverage in place. | 🟢 Done |
| 2 | JSON schema & catalog resolver | JSON schema validated; loader in `server/effects` resolves designer `entryId` → contract and caches lookups. | 🟢 Done |
| 3 | `tools/effectgen` TypeScript emitter | Deterministic TS output for payloads/enums/catalog metadata with golden-file tests; generator wired to CI drift checks. | 🟢 Done |
| 4 | Client integration of generated bindings | Client imports generated module; WebSocket lifecycle batches hydrate `ContractLifecycleStore`; renderer draws from store snapshots using generated catalog metadata; CI enforces regeneration drift checks. | 🟡 In progress |
| 5 | Client session orchestration | `client/main.ts` boots a `GameClientOrchestrator` backed by `WebSocketNetworkClient`, `InMemoryWorldStateStore`, and `CanvasRenderer`; UI mounts the renderer output and forwards lifecycle/keyframe events from network handlers. | ⚪ Planned |
| 6 | Input capture & command dispatch | `client/input.ts` implements `KeyboardInputController.register/unregister`; an `InputActionDispatcher` wires player intents/actions into `NetworkClient.send`, updating path/action payloads and honouring resync/ack flows in `client/client-manager.ts`. | 🟡 In progress |
| 7 | Effect runtime playback integration | Replace placeholder canvas drawing with the JS effects runtime so lifecycle batches spawn catalog-driven animations via `tools/js-effects` definitions; renderer disposes instances on end events and reconciles `ContractLifecycleStore` state. | ⚪ Planned |

## Active Work

### In progress

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

* 🟡 **Keyboard input capture and dispatcher plumbing**
  `client/input.ts` now registers DOM keyboard listeners, normalises intents, and streams actions through `client/client-manager.ts` so payloads include protocol metadata and pause during resync.

### Planned (to finish Phase 4)


* _None — monitoring for regressions only; follow-up items are scoped under Phases 5–7._


## Definition of Done (Phase 4)

Phase 4 is complete when all of the following hold:

1. **Type authority**: All client type narrowing and catalog metadata originate from generated code in `client/generated/*`; no manual mirrors.
2. **Network→Store**: WebSocket lifecycle batches are parsed and inserted into `ContractLifecycleStore` with correct cursor semantics and resync handling.
3. **Store→Renderer**: The renderer pulls only from `ContractLifecycleStore` snapshots and generated catalog metadata for scheduling/ownership; no legacy paths.
4. **CI gates**:

   * Generator drift check fails CI if bindings are stale.
   * Golden tests for generated TS pass.
   * JSON schema validation passes for all catalogs.
   * A headless render smoke test asserts that at least one lifecycle-driven frame is produced for a known `entryId`.

## Reference Map (authoritative paths)

* **Contracts & schema**: `server/effects/contract`, `docs/contracts/effects.schema.json`
* **Catalog**: `server/effects/catalog` (loader/validation), `config/effects/definitions.json`
* **Generator**: `tools/effectsgen` → `client/generated/effect-contracts.ts` (payloads, enums, catalog metadata)
* **Client runtime**:

  * Store: `client/effect-lifecycle-store.ts`
  * Orchestrator: `client/client-manager.ts` (hydrates store from network)
  * Network plumbing: `client/network.ts`, `client/main.ts`
  * Rendering: `client/render.ts` (reads store snapshots & catalog metadata)

## Suggested Next Task

**Kick off Phase 6 by implementing keyboard input capture and command dispatch.**

**Acceptance criteria**

* `client/input.ts` wires `KeyboardInputController.register/unregister` to DOM events and exposes an `InputActionDispatcher` that normalizes intents/actions.
* `client/client-manager.ts` routes dispatcher output through `NetworkClient.send`, preserving protocol version + ack metadata and pausing dispatch while resyncing.
* A headless harness or unit tests exercise the dispatcher to confirm movement/action payloads reach the mock network client with the expected shape.
