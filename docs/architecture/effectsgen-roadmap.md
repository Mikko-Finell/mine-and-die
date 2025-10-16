# Effect Contract Generation Roadmap

This document tracks the engineering work needed to deliver the `effectsgen` tool and supporting data sources defined in [`effectsgen-spec.md`](./effectsgen-spec.md). Each section highlights what contributors should focus on next to reach a production-ready pipeline.

## Roadmap

| Phase | Goal | Exit Criteria | Status |
| ----- | ---- | ------------- | ------ |
| 1 | Finalise Go contract registry | All effect contracts registered through a central Go package with validation helpers and unit coverage. | 🟡 In progress |
| 2 | Author JSON schema and catalogs | Machine-validated JSON schema exists for effect components and compositions; initial catalog checked into `server/effects/fixtures`. | ⚪ Planned |
| 3 | Implement `tools/effectgen` TypeScript emitter | CLI reads Go registry and JSON catalogs to generate deterministic TypeScript bindings with golden-file tests. | ⚪ Planned |
| 4 | Integrate generated bindings into client build | Client imports generated module, effect runner resolves animations via catalog IDs, and CI enforces regeneration drift checks. | ⚪ Planned |

## Active Work

| Item | Goal | Status | Notes |
| --- | --- | --- | --- |
| Consolidate contract declarations | Move scattered struct definitions into `server/effects/contracts` with compile-time registration. | 🟡 In progress | Need to replace ad-hoc copies in combat and map packages. |
| Draft JSON schema | Use `jsonschema` tags on Go structs and export schema to `docs/contracts/effects.schema.json`. | ⚪ Planned | Schema will validate designer-authored catalogs. |
| Build catalog loader | Add runtime loader that merges static JSON compositions and ensures referenced contracts exist. | ⚪ Planned | Loader must support hot reload in dev. |
| Scaffold code generator | Parse Go registry, map to TS AST, and emit modules under `client/generated/effects`. | 🟡 In progress | Workspace skeleton added in `tools/effectsgen`; CLI currently returns "not implemented". |
| Add regression tests | Golden snapshots for generated TS and integration tests for loader fallback paths. | ⚪ Planned | Guard against accidental contract drift. |

## Program Goals

* One authoritative Go registry defines every effect contract and its field semantics.
* Designer-owned JSON catalogs describe effect compositions without duplicating struct definitions.
* `tools/effectgen` produces stable TypeScript bindings consumed directly by the client build.
* Continuous integration fails when contracts or catalogs change without regenerating bindings.
* Runtime loaders validate catalog references before effects are sent to clients.

## Suggested Next Task

Stabilise the contract registry API by refactoring existing server packages to register effects through the new central package.
