# Effect Contract Generation Roadmap

This document tracks the engineering work needed to deliver the `effectsgen` tool and supporting data sources defined in [`effectsgen-spec.md`](./effectsgen-spec.md). Each section highlights what contributors should focus on next to reach a production-ready pipeline.

## Roadmap

| Phase | Goal | Exit Criteria | Status |
| ----- | ---- | ------------- | ------ |
| 1 | Finalise Go contract registry | All effect contracts registered through a central Go package with validation helpers and unit coverage. | 🟡 In progress |
| 2 | Author JSON schema and catalog resolver | Machine-validated JSON schema covers catalog entries keyed by designer IDs and validates referenced contract IDs; loader in `server/effects` caches entry → contract mappings. | ⚪ Planned |
| 3 | Implement `tools/effectgen` TypeScript emitter | CLI reads Go registry and catalog resolver output to generate deterministic bindings that expose both contract payloads and catalog entry metadata with golden-file tests. | ⚪ Planned |
| 4 | Integrate generated bindings into client build | Client imports generated module, gameplay enqueues catalog entry IDs, runtime resolves contract payloads via the loader, and CI enforces regeneration drift checks. | ⚪ Planned |

## Active Work

| Item | Goal | Status | Notes |
| --- | --- | --- | --- |
| Consolidate contract declarations | Move scattered struct definitions into `server/effects/contracts` with compile-time registration. | 🟡 In progress | `server/effects/contract` now owns the types, effect IDs, and a built-in registry backed by shared lifecycle payload structs; remaining callers still import the legacy aliases. |
| Retire legacy effectState pipeline | Remove `server/effects.go`/`server/simulation.go` shims once contract definitions cover all gameplay behaviours. | 🟡 In progress | Legacy structs now marked with `LEGACY` comments to scope the cleanup. |
| Draft JSON schema | Use `jsonschema` tags on Go structs and export schema to `docs/contracts/effects.schema.json`. | ⚪ Planned | Schema will validate designer-authored catalogs. |
| Build catalog loader | Add runtime loader that merges static JSON compositions, validates contract IDs, and exposes catalog entry lookups to gameplay. | 🟢 Done | `server/effects/catalog` now reads `config/effects/definitions.json`, validates against the Go registry, and feeds `EffectManager` with runtime contract lookups. |
| Align runtime effect queue with catalog IDs | Update `server/effects_manager.go` and related callers so gameplay code enqueues catalog entry IDs while the runtime resolves the linked contract before serialization. | 🟢 Done | Gameplay intents now propagate designer entry IDs through the manager while resolving contracts. |
| Surface catalog entry metadata to client runtime | Ensure generated bindings feed catalog metadata into `client/js-effects` so the effect runner can resolve compositions by entry ID without manual mirrors. | 🟡 In progress | Server now exposes catalog snapshot via `/join` and `/effects/catalog`; client wiring and generator output still pending. |
| Scaffold code generator | Parse Go registry, map to TS AST, and emit modules under `client/generated/effects`. | 🟡 In progress | Workspace skeleton added in `tools/effectsgen`; CLI currently returns "not implemented". |
| Add regression tests | Golden snapshots for generated TS and integration tests for loader fallback paths. | ⚪ Planned | Guard against accidental contract drift. |
| Enforce managed-by-client invariants | Extend `server/effects/catalog` validation so client-managed entries disable updates/end events and use single-tick lifetimes. | ⚪ Planned | Catch catalog mistakes like `attack`/`blood-splatter` before they ship. |

## Program Goals

* One authoritative Go registry defines every effect contract and its field semantics.
* Designer-owned JSON catalogs describe effect compositions without duplicating struct definitions.
* `tools/effectgen` produces stable TypeScript bindings consumed directly by the client build.
* Continuous integration fails when contracts or catalogs change without regenerating bindings.
* Runtime loaders validate catalog references and resolve entry IDs to contract payloads before effects are sent to clients.

## Suggested Next Task

Draft JSON schema for designer-authored catalogs so tooling can validate `config/effects/definitions.json` locally.
