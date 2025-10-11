# NPC AI System

Mine & Die drives every non-player character (NPC) from deterministic finite state machines (FSMs). Behaviour is authored in JSON under `server/ai_configs/`, compiled at startup into ID-based tables, and evaluated every simulation tick inside `World.runAI`. The executor only emits standard commands (`CommandMove`, `CommandAction`, etc.), so gameplay rules continue to flow through the core simulation systems.

## Asset pipeline

1. JSON configs are embedded at build time (`//go:embed ai_configs/*.json`).
2. `loadAILibrary` deserialises each file, validates references, and calls `compileAIConfig` to translate author-friendly names into compact enums and parameter slices (`ai_library.go`).
3. Each compiled config receives a monotonically increasing ID and is stored both by NPC type and by ID for quick lookup during runtime.
4. When NPCs spawn, their `AIConfigID`, initial state, and blackboard defaults are fetched from the shared library (`ai_library.go`, `npc.go`).

This compilation step removes string comparisons and reflection from the hot path—every action, condition, and ability is resolved to an integer ID before the simulation starts.

## Authoring configs

Each config targets one archetype and contains:

- `npc_type` – Matches the `NPCType` string used when seeding NPCs (`NPCTypeGoblin`, `NPCTypeRat`, etc.).
- `blackboard_defaults` – Optional defaults applied on spawn. Supported fields are `waypoint_index`, `arrive_radius`, `pause_ticks`, `patrol_speed`, and `stuck_epsilon`. These become the baseline values for the runtime blackboard (`ai_library.go`).
- `states[]` – Ordered list of states. The array index doubles as the numeric state ID used at runtime.
  - `id` – Human-readable name used in configs/tests.
  - `tick_every` – Cadence (in ticks) between evaluations. `0` forces evaluation next tick.
  - `duration_ticks` – Optional enter timer that blocks transitions until the delay expires.
  - `actions[]` – Declarative actions executed in order whenever the state runs.
  - `transitions[]` – Ordered conditions. The first condition that returns `true` selects the next state.

### Actions

`compileAIConfig` currently recognises the following actions (`ai_library.go`):

| Action | Purpose | Key parameters |
| ------ | ------- | -------------- |
| `moveToward` | Request navigation toward a waypoint, the tracked player target, or an offset vector. | `target` (`waypoint`/`player`/`vector`), optional `vector` payload. |
| `stop` | Clears any active path and emits a zeroed movement command so the NPC halts immediately. | – |
| `useAbility` | Queues a `CommandAction` for the mapped ability and starts its cooldown. | `ability` (matches effect names such as `attack`, `fireball`). |
| `face` | Rotates the NPC to look toward the current waypoint or tracked player without moving. | `target` (same options as `moveToward`). |
| `setTimer` | On state entry, schedules `WaitUntil = now + duration` for later `timerExpired` checks. | `duration_ticks`. Defaults to `pause_ticks` when omitted. |
| `setWaypoint` | On state entry, either advance to the next waypoint or jump to a specific index. | `advance` flag or explicit `waypoint`. |
| `setRandomDestination` | Picks a roam point around the NPC’s `Home` vector and seeds a navigation request. | `radius`, optional `min_radius`. |
| `moveAway` | Plans a path away from the stored `TargetActorID`, respecting minimum/maximum flee distance. | `distance`, optional `min_distance`. |

All actions only mutate the blackboard or enqueue commands; world state changes still happen in the simulation step.

### Transitions

Transition conditions are parsed into integer IDs with optional parameter blocks (`ai_library.go`):

| Condition | Behaviour |
| --------- | --------- |
| `reachedWaypoint` | Succeeds when the NPC is within `ArriveRadius` (overridable per transition) of the active waypoint, with stall-sensitive relaxation to avoid getting stuck (`ai_executor.go`). |
| `timerExpired` | Checks if the state entry timer (`WaitUntil`) has elapsed. |
| `playerWithin` | Locks onto the closest player within the supplied radius and stores their ID on the blackboard. |
| `nonRatWithin` | Similar to `playerWithin` but excludes rats and the NPC itself; used by the rat behaviour. |
| `lostSight` | Returns `true` when the tracked target drifts beyond a distance threshold or disappears. |
| `cooldownReady` | Gates state changes on ability cooldown availability. |
| `stuck` | Fires if the NPC’s recent movement fell below `epsilon` for `decisions` consecutive evaluations, signalling a stalled path. |

Conditions run in the order declared in the JSON, so place higher-priority transitions first.

## Runtime execution

`World.runAI` (invoked from the main tick loop) evaluates up to 64 NPCs per tick to keep frame times predictable (`ai_executor.go`). The flow is:

1. Gather and lexicographically sort NPC IDs to maintain deterministic iteration.
2. Skip NPCs whose next decision tick (`NextDecisionAt`) lies in the future or whose config/state table is missing.
3. Clamp the current state index and fall back to the initial state if the stored value is invalid.
4. Walk the state’s transitions in order, using `evaluateCondition` helpers. Matching a new state resets `StateEnteredTick`, applies any `enterTimer`, and keeps evaluation within the freshly selected state.
5. Execute the state’s actions. Movement-related actions call into navigation helpers to (re)build paths, while ability usage emits a `CommandAction` using `abilityIDToCommand` and stamps the corresponding cooldown.
6. Schedule the next decision tick based on the state cadence and record bookkeeping timestamps. The blackboard is then updated with positional deltas and waypoint progress.

Because commands are enqueued rather than applied immediately, the simulation step remains the single authority for collision resolution, damage, and effect lifecycles.

## Movement planning and blackboard maintenance

Navigation uses a coarse grid planner shared with players (`npc_path.go`):

- `ensureNPCPath` builds an A* path toward the desired target and stores it on the blackboard, retrying with relaxed targets when necessary. Failed plans impose a short cooldown before the next attempt.
- `followNPCPath` is called each tick to advance along the stored path. It updates facing, intent vectors, and triggers replans when the NPC stops making progress (pushed by other actors, blocked by obstacles, etc.).
- `updateBlackboard` tracks per-waypoint distance, the best progress achieved so far, and stall counters. If progress stagnates, `reachedWaypoint` gradually relaxes the acceptable radius so patrols keep moving even when geometry interferes (`ai_executor.go`).
- Separate fields (`PathLastDistance`, `PathStallTicks`, `PathRecalcTick`) prevent thrashing by delaying replans until meaningful time has passed.

The navigation grid now evaluates all eight neighboring cells during search, applying an octile heuristic and `sqrt(2)` step cost to diagonals. Diagonal hops are only permitted when the orthogonal flank tiles are free of static obstacles and dynamic blockers, preventing agents from cutting corners through occupied gaps. Player and NPC path followers already normalize arbitrary movement vectors, so the resulting diagonal waypoints translate directly into diagonal intents.

## Ability and timer management

Abilities are mapped from config strings to internal IDs during compilation. When `useAbility` runs, the executor:

1. Emits a `CommandAction` for the configured ability name (`attack`, `fireball`, etc.).
2. Uses `abilityCooldownTicks` to convert cooldown durations into simulation ticks and records `nextAbilityReady[ability]` on the blackboard (`ai_executor.go`).
3. Later transitions can query `cooldownReady` to branch into follow-up states only when the ability is available again.

Timers set via `setTimer` (or defaults) populate `WaitUntil`. The `timerExpired` condition and the `enterTimer` field give designers two timing tools: one for general-purpose waits and another for a guaranteed dwell time immediately after entering a state.

## Existing behaviours

Two configs ship by default (`server/ai_configs/`):

- **Goblin patrol & pursuit** – Alternates between `Patrol` and `Wait`, marching through fixed waypoints. Reached-waypoint detection uses stall-aware radius relaxation so the patrol resumes even when nudged off path. If a player crosses within roughly eight tiles (320 world units), the `playerWithin` transition promotes the goblin into a `Pursue` state that re-targets the tracked player each tick. The goblin continues chasing until `lostSight` fires at ~360 units or the player despawns, at which point it drops back to its patrol loop.
- **Rat wander & flee** – Roams around its home point, pauses periodically, and switches into a `Flee` state when players or hostile NPCs enter the configured radius. `moveAway` keeps rats backing off until `lostSight` or timers allow calmer behaviour.

Both behaviours are covered by regression tests in `server/ai_test.go`, which simulate hundreds of ticks to validate patrol loops, stall recovery, and flee logic.

## Extending the system

To add a new archetype:

1. Author a JSON config describing states, actions, and transitions.
2. Add regression coverage in `server/ai_test.go` so deterministic patrols/flee/ability usage stay protected.
3. Update documentation if new actions or conditions are introduced.

The executor and planner are already resilient to multiple NPC types—new behaviours typically require only config and test updates.
