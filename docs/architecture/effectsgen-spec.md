# Effect Contract Generation Specification

This spec defines the `effectsgen` pipeline that keeps the Mine & Die effect
contract authoritative in Go while exporting TypeScript bindings and
configuration for the client runtime.

## Goals

1. **Single contract source** – All spawn/update/end payloads are declared once
   in Go and registered in a central registry. Field names, enums, and nesting
   originate from these structs.
2. **Data-driven compositions** – Effect compositions ("fireball", "ray of frost",
   "blood splatter") are authored in JSON that both the server and client read.
   Designers modify JSON without recompiling Go or TypeScript.
3. **Generated client bindings** – Running `tools/effectgen` emits TypeScript
   interfaces plus strongly typed views of the JSON compositions so the client
   never hand-maintains mirrors of Go contracts.
4. **Runtime assembly on the server** – The server compiles Go contract types but
   loads JSON compositions at startup or hot reloads them for live editing.

## Authoritative Sources

| Responsibility            | Location / Artifact                                                | Notes |
| ------------------------- | ------------------------------------------------------------------ | ----- |
| Contract payload structs  | `server/effects/contract` Go package                               | Only place structs are defined. |
| Contract registry         | `server/effects/contract/registry.go`                              | Exports a `Registry` describing each contract and its payload types. |
| Composition catalog       | `config/effects/definitions.json`                                  | Designers map effect IDs to shared contract IDs plus animation/status params. |
| Optional shared constants | `config/effects/constants.json`                                    | (Future) Non-struct data like palette names; also surfaced by generator. |
| Generated client API      | `client/generated/effect-contracts.ts` (tool output)               | Never hand-edited. |

## Contract Declarations in Go

### Struct authoring

* Each payload struct lives in the `server/effects/contract` package (or a
  sub-package). Example:

  ```go
  package contract

  type ProjectileSpawn struct {
      ProjectileID   string `effect:"projectileId"`
      Origin         Vec2   `effect:"origin"`
      Velocity       Vec2
      Damage         int16
      ImpactEffectID string `effect:"impactEffectId"`
      Variant        string `effect:"variant"`
  }
  ```

  The projectile contract is intentionally generic. Designers build `fireball`,
  `ray-of-frost`, and similar payloads by configuring catalog entries that
  reference this shared struct rather than creating bespoke contract IDs.

* Supported field types: numeric primitives, `string`, `bool`, arrays/slices,
  fixed-size structs (referencing other contract structs), maps with string keys,
  and tagged enums implemented as Go `type Foo string` with constant values.

* Optional fields are represented as pointers; the generator emits
  `field?: Type` in TypeScript when the tag `effect:"optional"` is present or
  when the Go field type is a pointer.

### Contract registry

* `registry.go` exports a single `var Registry = contract.Registry{...}`. Each
  entry associates an ID with the Go types used for spawn, update, and end
  payloads.

  ```go
  var Registry = []Definition{
      {
          ID: "projectile",
          Spawn: (*ProjectileSpawn)(nil),
          Update: (*ProjectileUpdate)(nil),
          End: (*ProjectileEnd)(nil),
      },
      {
          ID: "blood-splatter",
          Spawn: (*BloodSplatterSpawn)(nil),
          Update: NoPayload,
          End: NoPayload,
      },
  }
  ```

* `Definition` is defined in `server/effects/contract/registry.go` as:

  ```go
  type Definition struct {
      ID     string
      Spawn  Payload
      Update Payload
      End    Payload
  }

  type Payload interface{ payloadMarker() }

  var NoPayload payloadSentinel
  ```

  The `Payload` interface is satisfied by pointer-to-struct types that embed a
  `ContractPayload` marker or alias to `NoPayload`.

* The registry is consumed by both the runtime (`EffectManager`) and by the
  generator. IDs must be unique. The generator treats `NoPayload` as `null` in
  the output TypeScript types.

## Composition JSON Format

* `config/effects/definitions.json` is the designer-facing catalog. It is an
  array (or object keyed by ID) with entries structured as:

  ```json
  {
      "id": "fireball",
      "contractId": "projectile",
      "jsEffect": "projectile/fireball",
      "parameters": {
          "trail": "ember-sparks",
          "impact": "fire-explosion-large",
          "variant": "fire"
      }
  },
  {
      "id": "ray-of-frost",
      "contractId": "projectile",
      "jsEffect": "projectile/ray-of-frost",
      "parameters": {
          "trail": "ice-shards",
          "impact": "frost-burst-medium",
          "variant": "ice"
      }
  }
  ```

* `contractId` must match an ID from the registry. Designers create new variants
  (e.g., `fireball`, `ray-of-frost`, `arrow`) by pointing them at the shared
  contract (`projectile` in the example above) and providing variant-specific
  metadata. Server-side loaders validate `contractId` values at startup using the
  registry.

* Additional optional blocks (`audio`, `cameraShake`, etc.) are free-form objects
  that designers can extend. The generator will surface them as TypeScript types
  derived from JSON schema inference (see below).

* The server reads the JSON during startup (`server/effects/definitions_loader.go`)
  and caches parsed structures keyed by the catalog entry ID. Runtime spawning
  logic combines authoritative contract payloads with designer-provided
  animation metadata when sending commands to clients.

## `tools/effectgen` Pipeline

### Invocation

```
go run ./tools/effectgen \
    --contracts=server/effects/contract \
    --registry=server/effects/contract/registry.go \
    --definitions=config/effects/definitions.json \
    --out=client/generated/effect-contracts.ts
```

The tool will also support `go:generate` directives inside the contract package
for reproducibility.

### Processing steps

1. **Load Go package** – Use `golang.org/x/tools/go/packages` to load the
   `--contracts` package with `TypesInfo`. Collect all exported structs that
   embed `ContractPayload` or satisfy the `Payload` interface.
2. **Parse registry** – Evaluate the `Registry` variable via `go/types` and
   reflection. Each entry yields concrete Go types for spawn/update/end.
3. **Build type graph** – Traverse struct fields recursively. Supported
   translations:
   - `int`, `int8`, `int16`, `int32`, `float32`, etc. → `number`
   - `string` → `string`
   - `bool` → `boolean`
   - slices → `Type[]`
   - maps → `Record<string, Type>`
   - enums (`type Foo string` with consts) → TypeScript union of literal strings
   - structs → interfaces (with optional field support)
4. **Read composition JSON** – Load `--definitions` and run schema inference: for
   each object, determine the union of keys and their value types. Emit literal
   types (`as const`) so the client sees exact strings for `jsEffect`, etc.
5. **Emit files** – Generate a single TypeScript module with sections:
   - `// Generated by effectsgen. DO NOT EDIT.` banner.
   - Interfaces for each payload struct.
   - `type EffectContractMap = { ... }` mapping IDs to `{ spawn: X; update: Y; end: Z }`.
   - Exported constants `effectContracts` (typed) and
     `effectDefinitions` (JSON-derived data with literal types).
6. **Formatting** – Run the emitted code through `prettier` (bundled via
   `node_modules/.bin/prettier`) if available; otherwise fall back to simple
   indentation to avoid toolchain coupling.

### Error handling

* Missing registry entries, duplicate IDs, or structs outside the contract
  package cause the generator to exit with a non-zero status.
* JSON definitions referencing an unknown `contractId` fail generation.
* Unsupported Go field types report a descriptive error listing the offending
  struct and field.

## Server Runtime Integration

1. On startup, the server imports `server/effects/contract` to compile payload
   structs and registers them by calling `contract.Register(Registry)`.
2. `server/effects/definitions_loader.go` reads
   `config/effects/definitions.json`, validates `contractId` keys against the
   registry, and stores the compositions in memory.
3. When gameplay code enqueues an effect, it references the **catalog entry ID**
   (e.g., `fireball`, `ray-of-frost`). Runtime assembly uses that ID to load the
   composition, then reads the linked `contractId` (`projectile` in the example)
   from the catalog entry to determine which payload struct to serialize. The
   payload structs remain authoritative and are serialized via the shared
   contract.

## Client Consumption

* Client imports `client/generated/effect-contracts.ts` to access:
  - `EffectContracts` type definitions for spawn/update/end payloads.
  - `effectDefinitions` constant describing js-effects hooks.
* Rendering code uses `effectDefinitions[id].jsEffect` to look up animation
  assets while relying on TypeScript types for payload field access.
* When designers update JSON and rerun `effectsgen`, the client automatically
  receives new literal types; any outdated property access fails TypeScript
  compilation.

## Workflow for Adding Effects

*To add a new variant of an existing contract (most common):*

1. **Author composition** – Add or edit an entry in `config/effects/definitions.json`
   that references an existing contract ID (`projectile`, `status-effect`, etc.)
   and sets variant-specific animation and status parameters.
2. **Regenerate bindings** – Run `go run ./tools/effectgen ...` (or `go generate`).
3. **Verify** – Run server tests to ensure runtime loading succeeds and TypeScript
   compilation passes with the regenerated file.

*To introduce a brand-new contract type:*

1. **Define payloads** – Add Go structs in `server/effects/contract` with
   appropriate fields.
2. **Register contract** – Append an entry to `Registry` with a unique ID.
3. **Author composition** – Add one or more entries to
   `config/effects/definitions.json` that reference the new contract ID.
4. **Regenerate bindings** – Run `go run ./tools/effectgen ...` (or `go generate`).
5. **Verify** – Run server tests to ensure runtime loading succeeds and TypeScript
   compilation passes with the regenerated file.

## Future Enhancements

* **Hot reload** – Add file watchers that trigger `effectsgen` and server reloads
  when JSON changes in development.
* **Schema assertions** – Optionally emit JSON Schema from the Go structs for
  external tooling.
* **Playground validation** – Provide a CLI flag to dump sample payloads from Go
  structs to help designers preview required fields.
