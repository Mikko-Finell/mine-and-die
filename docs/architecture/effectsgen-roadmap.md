# Effect Contract Generation Roadmap

This document tracks the engineering work needed to deliver the `effectsgen` tool and supporting data sources defined in [`effectsgen-spec.md`](./effectsgen-spec.md). Each section highlights what contributors should focus on next to reach a production-ready pipeline.

## Roadmap

| Phase | Goal | Exit Criteria | Status |
| ----- | ---- | ------------- | ------ |
| 1 | Finalise Go contract registry | All effect contracts registered through a central Go package with validation helpers and unit coverage. | 🟡 In progress |
| 2 | Author JSON schema and catalog resolver | Machine-validated JSON schema covers catalog entries keyed by designer IDs and validates referenced contract IDs; loader in `server/effects` caches entry → contract mappings. | 🟢 Done |
| 3 | Implement `tools/effectgen` TypeScript emitter | CLI reads Go registry and catalog resolver output to generate deterministic bindings that expose both contract payloads and catalog entry metadata with golden-file tests. | 🟡 In progress |
| 4 | Integrate generated bindings into client build | Client imports generated module, gameplay enqueues catalog entry IDs, runtime resolves contract payloads via the loader, and CI enforces regeneration drift checks. | ⚪ Planned |

## Active Work

| Item | Goal | Status | Notes |
| --- | --- | --- | --- |
| Consolidate contract declarations | Move scattered struct definitions into `server/effects/contracts` with compile-time registration. | 🟢 Done | `server/effects/contract` now owns the types, effect IDs, built-in registry, and default effect definitions consumed by `server/effects_manager.go`; legacy aliases removed and callers import the contract package directly. |
| Retire legacy effectState pipeline | Remove `server/effects.go`/`server/simulation.go` shims once contract definitions cover all gameplay behaviours. | 🟡 In progress | Legacy structs now marked with `LEGACY` comments to scope the cleanup. |
| Draft JSON schema | Use `jsonschema` tags on Go structs and export schema to `docs/contracts/effects.schema.json`. | 🟢 Done | `go generate` now emits `docs/contracts/effects.schema.json` from `catalog.EntryDocument`. |
| Build catalog loader | Add runtime loader that merges static JSON compositions, validates contract IDs, and exposes catalog entry lookups to gameplay. | 🟢 Done | `server/effects/catalog` now reads `config/effects/definitions.json`, validates against the Go registry, and feeds `EffectManager` with runtime contract lookups. |
| Align runtime effect queue with catalog IDs | Update `server/effects_manager.go` and related callers so gameplay code enqueues catalog entry IDs while the runtime resolves the linked contract before serialization. | 🟢 Done | Gameplay intents now propagate designer entry IDs through the manager while resolving contracts. |
| Surface catalog entry metadata to client runtime | Ensure generated bindings feed catalog metadata into `client/js-effects` so the effect runner can resolve compositions by entry ID without manual mirrors. | 🟢 Done | Catalog exports now include a required `managedByClient` flag derived from the Go registry; the client normalizer enforces it and exposes a helper for renderers. |
| Adopt catalog-managed ownership in renderer | Use `effectCatalog` metadata to decide when lifecycles stay client-owned instead of falling back to replication hints. | 🟢 Done | New renderer helpers resolve catalog entries by `entryId` and expose `isLifecycleClientManaged` for scheduling logic. |
| Scaffold code generator | Parse Go registry, map to TS AST, and emit modules under `client/generated/effects`. | 🟢 Done | Generator now validates inputs, projects catalog `definition` blocks as `EffectDefinition` literals, emits phantom-typed `effectContracts` metadata with a shared `getContractMeta` accessor for narrowing, wires `go:generate` in `server/effects/contract` via a shared `go.work`, and is invoked automatically by `npm run client:build`, `client:dev`, and the `pretest` drift check so CI fails when bindings are stale. |
| Map Go payload structs to TS interfaces | Emit TypeScript interfaces from `server/effects/contract` payload structs and wire them into the generated module. | 🟢 Done | `tools/effectsgen` now parses the Go registry, generates payload interfaces, and maps spawn/update/end types into `client/generated/effect-contracts.ts`. |
| Lift Go enums into TypeScript unions | Translate `server/effects/contract` enum constants into literal-union aliases so client code narrows on contract fields. | 🟢 Done | `tools/effectsgen/internal/pipeline/contracts.go` now collects typed constants and emits literal-union aliases with coverage in `internal/pipeline/run_test.go`. |
| Add regression tests | Golden snapshots for generated TS and integration tests for loader fallback paths. | 🟢 Done | Loader now rejects duplicate IDs/unknown contracts and survives missing sources. |
| Enforce managed-by-client invariants | Extend `server/effects/catalog` validation so client-managed entries disable updates/end events and use single-tick lifetimes. | 🟢 Done | `server/effects/catalog/resolver.go` now rejects managed entries that enable updates/end payloads or exceed one tick. |
| Build contract lifecycle store | Ingest spawn/update/end batches into a typed client store that retains client-managed lifecycles. | 🟢 Done | `client/effect-lifecycle-store.ts` stores authoritative batches, exposes renderer views, and relies on catalog metadata for retention. |

## Program Goals

* One authoritative Go registry defines every effect contract and its field semantics.
* Designer-owned JSON catalogs describe effect compositions without duplicating struct definitions.
* `tools/effectgen` produces stable TypeScript bindings consumed directly by the client build.
* Continuous integration fails when contracts or catalogs change without regenerating bindings.
* Runtime loaders validate catalog references and resolve entry IDs to contract payloads before effects are sent to clients.

## Suggested Next Task

Integrate the contract lifecycle store into the new renderer pipeline so render scheduling consumes `client/effect-lifecycle-store.ts` instead of legacy lifecycle state.
