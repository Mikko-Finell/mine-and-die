# AI System Overview

The Mine & Die server drives all non-player characters through deterministic finite state machines (FSMs). Behaviour is authored in JSON (`server/ai_configs/`) and compiled at startup into compact ID-based tables so the tick loop avoids string comparisons and reflection.

## Authoring Configs

Each config targets a single NPC archetype and contains:

- `npc_type` – The canonical NPC type string (e.g. `"goblin"`).
- `states[]` – An ordered list of states with `id`, optional `tick_every`, `duration_ticks`, `actions[]`, and `transitions[]`.
- `blackboard_defaults` – Optional defaults for fields like `arrive_radius`, `pause_ticks`, or `stuck_epsilon`.

States reference declarative actions and conditions from the shared library:

- **Actions** (`actions[]`) enqueue commands or tweak blackboard data without mutating world state. Examples include `moveToward`, `stop`, `setTimer`, `setWaypoint`, and `useAbility`.
- **Transitions** (`transitions[]`) evaluate conditions in order (`reachedWaypoint`, `timerExpired`, `playerWithin`, `stuck`, etc.). The first condition that returns true selects the next state.

Configs are embedded at build time and compiled by `server/ai_library.go` into typed slices that map names → small enums and parameter blocks. This step validates references and prevents runtime string work.

## Runtime Execution

The executor in `server/ai_executor.go` runs during the AI phase of each tick:

1. NPC IDs are sorted to keep decision order deterministic.
2. The executor skips actors whose `NextDecisionAt` lies in the future, capping total decisions per tick.
3. Transitions are evaluated, emitting `AIStateChanged` events when state IDs change.
4. Actions execute. Ability usage still enqueues `CommandAction` payloads, while movement actions plan path targets that the simulation's path follower resolves into per-tick intents.
5. Blackboard bookkeeping updates waypoints, timers, `StuckCounter`, and schedules the next decision tick.

`moveToward` actions build A* paths across a coarse navigation grid. If a direct path cannot be found the planner probes nearby tiles and selects the closest accessible fallback. Path progress is monitored each tick so pushes or external nudges trigger a replanning cooldown before the NPC resumes travel.

Because actions only enqueue commands, the simulation loop remains the single authority for world mutations.

## Goblin Patrol Example

`server/ai_configs/goblin.json` defines a two-state patrol:

- `Patrol` runs every 5 ticks, issues `moveToward(waypoint)`, and transitions to `Wait` when within `arrive_radius`.
- `Wait` executes every tick, calls `stop()`, sets a timer for `pause_ticks`, advances the waypoint index once on entry, and returns to `Patrol` when the timer expires.

The defaults seed goblins with two waypoints, pause for half a second (~30 ticks), and detect stuck behaviour using a small epsilon. Adding new archetypes follows the same pattern—extend the JSON, cover it with table-driven tests, and the executor requires no modifications.

## Rat Scavenger Behaviour

Rats are lightweight ambience critters implemented directly in Go (`server/rat_ai.go`). They do not use the JSON FSM pipeline—the executor short-circuits to a bespoke wander/flee routine when it sees `type: "rat"`.

- **Wandering.** Each rat remembers its spawn position and periodically picks a random target within ~200 pixels using the world's deterministic RNG. Movement commands are emitted at a reduced speed so the critter meanders instead of sprinting like a player.
- **Threat response.** If any non-rat actor (player or hostile NPC) comes within 140 pixels, the rat stores a flee vector and issues full-speed move commands away from the intruder for roughly four seconds. The flee logic also fires when a goblin walks past, keeping ambience NPCs from clustering around fights.
- **Testing.** `server/rat_test.go` exercises both behaviours: rats eventually leave their spawn point when left alone and gain distance from nearby threats.
