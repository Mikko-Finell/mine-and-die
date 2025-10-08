# Testing & Troubleshooting

## Automated Tests
- Run `go test ./...` from the repository root (or inside `server/`).
- Run `npm test` to execute the lightweight Vitest suite covering the client utilities. These checks focus on brittle parsing
  helpers; we are not aiming for exhaustive UI coverage and tooling playgrounds do not require tests.
- Current suite covers:
  - Player joins and snapshot correctness.
  - Intent normalization and facing behaviour.
  - Movement bounds, obstacle collisions, and player separation.
  - Action cooldowns plus melee/fireball effect lifecycles.
  - Mining rewards when melee swings contact gold ore.
  - Heartbeat bookkeeping and diagnostics snapshots.

## Manual Checks
- **Server health:** `curl http://localhost:8080/health` should return `ok`.
- **Diagnostics:** Visit `http://localhost:8080/diagnostics` to inspect heartbeat timings and tick rate.
- **Latency simulation:** Use the diagnostics latency input on the client to emulate network jitter.

## Debug Tips
- Enable verbose logging by adding `log.Printf` calls around simulation branches you are modifying; remember to remove or gate them before committing.
- If players seem to teleport, confirm that `store.displayPlayers` entries exist for each ID and that `startRenderLoop` is running.
- Connection drops usually mean missed heartbeats—check `store.lastHeartbeatSentAt`/`AckAt` values in the diagnostics panel.
- For reproducible obstacle layouts during debugging, consider seeding the RNG in `generateObstacles` temporarily.
