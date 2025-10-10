# Testing & Troubleshooting

## Automated Tests
- Run `npm test` from the repository root to execute the Vitest client suite. The cases cover heartbeat utilities, network normalization (payload builders, world config helpers, console serialization), and the patch replay container (baseline construction, diff application, keyframe recovery).【F:client/__tests__/heartbeat.test.js†L1-L136】【F:client/__tests__/network.test.js†L1-L206】【F:client/__tests__/patches.test.js†L1-L200】
- Run `cd server && go test ./...` to exercise the Go regression suite.【F:server/go.mod†L1-L6】
- Current suite highlights:
  - Player joins, snapshot integrity, movement/collision, actions, mining rewards, and heartbeat tracking via `main_test.go`.【F:server/main_test.go†L1-L220】
  - Write barrier coverage for players, NPCs, effects, and ground items in `world_mutators_test.go`.【F:server/world_mutators_test.go†L360-L520】
  - Keyframe journal eviction behaviour in `patches_test.go`.【F:server/patches_test.go†L8-L62】
  - Deterministic goblin patrol and rat flee behaviours in `ai_test.go`.【F:server/ai_test.go†L148-L200】

## Manual Checks
- **Server health:** `curl http://localhost:8080/health` should return `ok`.
- **Diagnostics:** Visit `http://localhost:8080/diagnostics` to inspect heartbeat timings and tick rate.
- **Latency simulation:** Use the diagnostics latency input on the client to emulate network jitter.

## Debug Tips
- Enable verbose logging by adding `log.Printf` calls around simulation branches you are modifying; remember to remove or gate them before committing.
- If players seem to teleport, confirm that `store.displayPlayers` entries exist for each ID and that `startRenderLoop` is running.
- Connection drops usually mean missed heartbeats—check `store.lastHeartbeatSentAt`/`AckAt` values in the diagnostics panel.
- For reproducible obstacle layouts during debugging, use the debug panel's world reset seed input (or POST `/world/reset` with a `seed`) instead of modifying Go code.【F:client/main.js†L78-L210】【F:server/main.go†L72-L172】
