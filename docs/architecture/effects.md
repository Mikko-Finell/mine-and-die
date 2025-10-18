# Effects Contract Architecture

This document explains how Mine & Die defines, generates, and consumes effect
contracts across the Go server and the TypeScript client. The system keeps the
Go definitions authoritative, distributes catalog metadata through generated
artifacts, and guarantees that gameplay and rendering share the same
vocabulary.

## Authoritative Sources

| Responsibility | Artifact | Notes |
| --- | --- | --- |
| Payload structs & registry | `server/effects/contract` | Owns contract structs, lifecycle ownership metadata, and the authoritative registry. |
| Designer catalog | `config/effects/definitions.json` | Maps catalog IDs to contract IDs and runtime parameters consumed by the server and client. |
| Generated client bindings | `client/generated/effect-contracts.ts` | Produced by the generator; provides TypeScript interfaces plus typed catalog snapshots. |
| Generator & tooling | `tools/effectsgen` | Loads the registry and catalog, emits bindings, and enforces validation rules. |

## Contract Declarations in Go

Effect payloads live in `server/effects/contract`. Each struct embeds
`ContractPayload` to satisfy the `Payload` interface so it can be registered.
The `Registry` value enumerates every contract ID with its spawn, update, and
end payload types plus a `LifecycleOwner` flag describing which runtime (server
or client) manages the instance after spawn.

The registry must validate before use. Validation ensures:

1. Every definition has a non-empty ID and unique registry entry.
2. Payload declarations are pointers to structs (or `NoPayload`).
3. Ownership flags stay within the supported enum (`LifecycleOwnerServer` or
   `LifecycleOwnerClient`).

Callers typically materialise an index with `Registry.Index()` when the server
boots; the generator repeats the same validation before emitting artifacts.

### Built-in definitions

`definitions_default.go` provides ready-made `EffectDefinition` templates for
core gameplay effects. Callers receive fresh struct instances to customise
lifetimes, hooks, or client replication policies without mutating the package
state. This keeps gameplay logic and generation in sync while allowing the
runtime to specialise behaviour per effect ID.

## Designer Catalog JSON

Designers author catalog entries in `config/effects/definitions.json`. Each
entry links a catalog `id` (such as `fireball`) to a `contractId` present in the
Go registry. Additional keys capture runtime options: delivery kind, geometry,
motion profile, client replication flags, hook names, and per-effect
parameters. The server loader validates that every catalog entry references a
known contract and caches the expanded metadata for runtime lookups.

Because the generator reads the same JSON file, any catalog change becomes part
of the generated TypeScript snapshot. Client code receives literal types for
fields like `jsEffect` or `managedByClient` and can only reference catalog IDs
that exist in the JSON file.

## Generation Pipeline

The `effectsgen` command in `tools/effectsgen` ties everything together.
Invocation accepts flags pointing at the Go contracts package and the catalog
JSON. The pipeline performs the following steps:

1. **Load contracts** – Uses Go packages tooling to load `server/effects/contract`
and discover exported structs that satisfy `Payload`.
2. **Parse registry** – Evaluates the `Registry` variable, capturing the spawn,
   update, and end payload types plus lifecycle ownership for each contract ID.
3. **Build a type graph** – Walks every struct recursively, translating Go types
   into TypeScript equivalents. Enums declared as `type Foo string` produce
   literal union types, slices become arrays, and optional pointers become
   `field?: Type`.
4. **Read catalog JSON** – Loads `config/effects/definitions.json`, infers the
   literal types of each entry, and confirms that every `contractId` matches the
   registry.
5. **Emit artifacts** – Writes `client/generated/effect-contracts.ts` with an
   auto-generated banner, interface definitions, a typed `effectContracts`
   mapping, and an exported `effectDefinitions` constant wrapping the catalog
   data.
6. **Format output** – Runs Prettier when available so diffs remain deterministic
   for CI drift checks.

Generation fails fast on invalid payload declarations, unknown contract IDs, or
unsupported Go field types so the CI pipeline catches mismatches before they
reach the game client.

## Server Runtime Integration

The server compiles the contract package during startup, validates the registry,
and registers it with runtime systems that enqueue gameplay effects. When
content code wants to emit an effect, it references the catalog entry ID. The
loader in `server/effects/catalog` resolves the entry, looks up the linked
contract ID, and serialises the appropriate payload struct. Because both the
registry and catalog flow through the generator, the server and client share
identical payload shapes and catalog metadata.

## Client Consumption

The client imports `client/generated/effect-contracts.ts` to access:

- TypeScript interfaces describing spawn, update, and end payloads for each
  contract.
- The typed `effectContracts` map for runtime validation.
- The literal `effectDefinitions` catalog snapshot for rendering metadata.

`client/client-manager.ts` hydrates a `ContractLifecycleStore` from WebSocket
payloads. Render batches produced by the orchestrator include
`runtimeEffects` derived from the catalog metadata and payloads. The canvas
renderer translates those batches into effect runtime calls, spawning instances
on `spawn`, reconciling `update` payloads, and disposing instances on `end` or
when the lifecycle store resets.

### Catalog distribution policy

The generated TypeScript artifacts are the only catalog payloads the client
needs. We do **not** stream catalog snapshots or updates from the server; the
checked-in output from `effectsgen` already contains the authoritative data for
runtime usage. Until we intentionally revisit the contract, assume that shipping
a fresh build (with regenerated artifacts) is the required workflow for catalog
changes.

Future niceties like hot reload, live catalog streaming, or runtime editing are
explicitly out of scope for the current initiative. The architecture should not
carry extra code paths, toggleable transports, or invalidation hooks in
anticipation of those features. If we ever need them, we will design the
contract extensions deliberately when that roadmap becomes active.

## Lifecycle and Resync Semantics

- **Spawn** – Creates a runtime instance keyed by catalog ID and payload
  signature. Client-managed entries (`LifecycleOwnerClient`) persist until the
  renderer decides to release them.
- **Update** – Changes in payload signatures trigger instance replacement so
  option changes (such as animation variants) apply immediately.
- **End** – Marks the effect for removal. Non-retained entries dispose
  immediately; client-managed entries wait for renderer confirmation.
- **Resync / Disconnect** – Clearing the lifecycle store removes any active
  runtime instances and reloads catalog metadata from the latest snapshot so the
  client stays in lockstep with the server after reconnects.

## Adding or Updating Effects

1. Author or modify Go payload structs in `server/effects/contract` and register
   them in `Registry` with the desired lifecycle owner.
2. Create catalog entries in `config/effects/definitions.json` that reference the
   contract ID and supply runtime parameters.
3. Run the generator (`go run ./tools/effectsgen ...`) to refresh the TypeScript
   artifacts.
4. Regenerate the client bundle or run TypeScript compilation to confirm types
   align with the new payloads.
5. Execute server tests or smoke replays to verify runtime behaviour and
   lifecycle teardown semantics.

## Validation and Drift Checks

- CI should run the generator and compare the output against committed files to
  catch drift.
- Server loaders validate catalog entries against the registry on startup.
- Client smoke tests replay recorded lifecycle batches to ensure the renderer
  honours spawn/update/end transitions, resync clears, and client-managed
  retention rules.

Keeping the generator authoritative means server and client code do not hand
maintain parallel structures; any schema change flows from the Go contracts to
all downstream consumers automatically.
