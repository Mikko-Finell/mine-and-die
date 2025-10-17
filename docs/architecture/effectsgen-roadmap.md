# Effect Contract Generation Roadmap

This document tracks the engineering work required to deliver the `effectsgen` toolchain and its supporting data sources defined in [`effectsgen-spec.md`](./effectsgen-spec.md). It is the single source of truth for status and the next concrete tasks.

## Roadmap

| Phase | Goal                                     | Exit Criteria                                                                                                                                                                                               | Status         |
| ----- | ---------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | -------------- |
| 1     | Finalise Go contract registry            | Central Go package owns all effect contracts and validation helpers; unit coverage in place.                                                                                                                | 🟢 Done        |
| 2     | JSON schema & catalog resolver           | JSON schema validated; loader in `server/effects` resolves designer `entryId` → contract and caches lookups.                                                                                                | 🟢 Done        |
| 3     | `tools/effectgen` TypeScript emitter     | Deterministic TS output for payloads/enums/catalog metadata with golden-file tests; generator wired to CI drift checks.                                                                                     | 🟢 Done        |
| 4     | Client integration of generated bindings | Client imports generated module; WebSocket lifecycle batches hydrate `ContractLifecycleStore`; renderer draws from store snapshots using generated catalog metadata; CI enforces regeneration drift checks. | 🟡 In progress |

## Active Work

### In progress

* 🟢 **Consume generated catalog metadata on the client**
  Client modules now import canonical catalog data from `client/generated/effect-contracts.ts`; join-time payloads are verified against the generated snapshot and all downstream helpers read from the shared store.
* 🟢 **Feed renderer from `ContractLifecycleStore`**
  Orchestrator now ingests WebSocket `state` batches into `ContractLifecycleStore` and emits render batches derived from generated catalog metadata/layers.
* 🟢 **Lifecycle renderer smoke tests**
  Headless harness replays recorded lifecycle batches and asserts renderer output derives from generated catalog metadata and managed ownership flags.

### Planned (to finish Phase 4)

* _(none)_

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

**Add smoke coverage for client-managed retention and resync handling in the lifecycle renderer harness.**

**Acceptance criteria**

* Recorded batch includes a client-managed catalog entry that should remain retained after an `end` event.
* Harness verifies renderer output marks retained entries as managed and preserves geometry across the resync boundary.
* Resync message clears lifecycle state and renderer batches back to an empty frame.
