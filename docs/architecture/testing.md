# Testing & Troubleshooting

## Automated Tests
- Run `npm test` from the repository root to execute the Vitest client suite. It now includes join/resync integration specs that assert two-pass lifecycle replay, patch reset scheduling, and forced resync requests in addition to the existing heartbeat and normalization coverage. [client/__tests__/network.test.js](../../client/__tests__/network.test.js)
- Run `cd server && go test ./...` to exercise the Go regression suite. [server/go.mod](../../server/go.mod)
- Current suite highlights:
  - Player joins, snapshot integrity, movement/collision, actions, mining rewards, and heartbeat tracking via `main_test.go`. [server/main_test.go](../../server/main_test.go)
  - Contract lifecycle sequencing by delivery kind in `effects_manager_contract_test.go`, ensuring spawn/update/end ordering, follow modes, and end reasons remain deterministic. [server/effects_manager_contract_test.go](../../server/effects_manager_contract_test.go)
  - Write barrier coverage for players, NPCs, effects, and ground items in `world_mutators_test.go`. [server/world_mutators_test.go](../../server/world_mutators_test.go)
  - Keyframe journal eviction behaviour in `patches_test.go`. [server/patches_test.go](../../server/patches_test.go)
  - Deterministic goblin patrol and rat flee behaviours in `ai_test.go`. [server/ai_test.go](../../server/ai_test.go)

## Manual Checks
- **Server health:** `curl http://localhost:8080/health` should return `ok`.
- **Diagnostics:** Visit `http://localhost:8080/diagnostics` to inspect heartbeat timings and tick rate.
- **Latency simulation:** Use the diagnostics latency input on the client to emulate network jitter.

## Debug Tips
- Enable verbose logging by adding `log.Printf` calls around simulation branches you are modifying; remember to remove or gate them before committing.
- If lifecycle events appear out of order, inspect `store.lastEffectLifecycleSummary` and the lifecycle state helpers (`peekEffectLifecycleState`) to confirm the client received a spawn before each update. [client/network.js](../../client/network.js) [client/effect-lifecycle.js](../../client/effect-lifecycle.js)
- Connection drops usually mean missed heartbeats—check `store.lastHeartbeatSentAt`/`AckAt` values in the diagnostics panel.
- For reproducible obstacle layouts during debugging, use the debug panel's world reset seed input (or POST `/world/reset` with a `seed`) instead of modifying Go code. [client/main.ts](../../client/main.ts) [server/main.go](../../server/main.go)
