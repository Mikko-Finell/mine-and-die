# Most Common

- Consolidate the effect spawn detection and logging helpers in `client/network.js` into a shared matcher so effect-specific functions stop duplicating the same candidate traversal.
- Unify the player and NPC path-following state machines on the server so navigation, stall detection, and waypoint updates live in one reusable helper instead of two parallel files.
- Collapse the duplicated inventory and equipment mutation wrappers into a generic helper that handles cloning, error rollback, and patch emission for any actor payload.
- Share the effect lifecycle normalization utilities across client modules (or drop deprecated camelCase fallbacks) so payload parsing rules stay consistent and simpler to maintain.

# Highest Impact

- Unify the player and NPC path-following controllers so the movement state machine, stall thresholds, and intent updates are defined once and reused across every actor type.
- Collapse the inventory and equipment mutation flows into a single generic helper that manages cloning, equality checks, and patch emission for any actor state change.
- Introduce a shared world snapshot payload struct for `joinResponse`, `stateMessage`, and `keyframeMessage` so schema updates touch one definition instead of three separate message types.
- Serve default world configuration data from one source (e.g., the server bootstrap payload) so client and server defaults can no longer drift out of sync.

# Easiest Change

- Drop the optional pointer pattern from `resyncPolicy`, store it by value on owners, and delete the impossible `nil` guards that currently wrap every method call.
- Remove the legacy camelCase fallbacks when reading effect lifecycle batches now that the server emits snake_case, trimming repeated optional chaining in the client payload readers.
- Replace `copyIntMap`/`copyBoolMap` with a single generic map clone (or Go's `maps.Clone`) so both callers stop maintaining redundant helpers.
- Introduce shared clamp helpers for the server packages that currently reimplement `clamp` logic, reducing scattered utility duplicates with a lightweight refactor.
