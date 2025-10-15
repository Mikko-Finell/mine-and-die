# Most Common

- Consolidate the effect spawn identifier/logging helpers in `client/network.js` so the `is*Identifier`, `is*Spawn`, and `log*Spawn` families all reuse a single traversal of spawn payloads instead of maintaining near-identical copies for each tracked effect type.
- Unify the player and NPC path-following state machines in `server/player_path.go` and `server/npc_path.go` by extracting shared helpers for stall detection, arrival radius defaults, and intent updates, preventing the duplicated logic from drifting.
- Collapse the repeated inventory and equipment mutation wrappers in `server/world_mutators.go` by sharing the clone/compare/patch boilerplate across actors, keeping inventory updates and version bumps consistent.

# Highest Impact

- Unify the player and NPC path-following systems so every actor ticks through a shared navigation controller, giving us one place to tune stall thresholds, arrival radii, and intent updates without risking divergent behaviour across the simulation.
- Serve a single authoritative world configuration by plumbing the server defaults (counts, dimensions, seeds) into the client instead of hard-coding duplicates, removing an entire class of sync bugs when balance numbers change.
- Centralize the effect spawn classification and logging pipeline in `client/network.js`, collapsing hundreds of duplicated lines and ensuring transport schema updates land in one helper.
- Collapse the repeated inventory/equipment mutation boilerplate in `server/world_mutators.go` into a shared helper so every mutation path benefits from the same cloning, comparison, and patch emission safeguards.

# Easiest Change

- Drop the defensive `nil` checks by storing `resyncPolicy` as a value on the journal instead of a pointer, letting its methods use value receivers and deleting the impossible `if policy == nil` guard clauses.
- Remove the legacy camelCase fallbacks when reading effect lifecycle batches, now that the server always emits snake_case arrays, simplifying the client-side payload parsing.
- Replace the bespoke `clamp` helpers scattered across server packages with a single shared utility so movement, stat formulas, and ground item math all call the same function.
- Switch the duplicated `copyIntMap`/`copyBoolMap` helpers to a generic `copyMap` (or Go's `maps.Clone`) to remove parallel implementations while keeping semantics identical.
