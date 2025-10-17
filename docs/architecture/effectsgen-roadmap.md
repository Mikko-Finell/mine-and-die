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
* 🟡 **Feed renderer from `ContractLifecycleStore`**
  Implement an orchestrator so WebSocket state/lifecycle messages populate `ContractLifecycleStore` and trigger renders that consume store snapshots (no legacy stubs).

### Planned (to finish Phase 4)

* **Transport → Store ingestion**
  Normalize `effect_spawned` / `effect_update` / `effect_ended` (plus cursor hints) into `ContractLifecycleStore`, including resync/reset handling.
* **Renderer scheduling via catalog metadata**
  Use generated catalog/definitions to decide ownership (`managedByClient`) and build draw instructions (layers/animations) instead of placeholders.

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

**Wire the live path *WebSocket → ContractLifecycleStore → Renderer* using generated types/metadata.**

**Acceptance criteria**

* Batches for a known `entryId` flow from network handlers into `ContractLifecycleStore` with cursor/reset handling.
* Renderer consumes only store snapshots; draw scheduling respects `managedByClient` and other generated metadata.
* CI passes with generator drift, schema validation, goldens, and the headless render smoke test.
