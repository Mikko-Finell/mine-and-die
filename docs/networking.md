# Networking Overview

This document describes how the Mine & Die client and server communicate today. It covers the HTTP preamble, the WebSocket session, and the message payloads exchanged during play.

## Connection Lifecycle

1. **Join handshake** – The browser performs a `POST /join` to create a player slot and fetch an initial snapshot. The response includes the assigned `id`, the current actors (`players`, `npcs`), static world data (`obstacles`), active effects, pending one-shot `effectTriggers`, any dropped `groundItems`, and the authoritative world configuration object. 【F:server/messages.go†L4-L12】【F:client/network.js†L662-L690】
2. **WebSocket upgrade** – After storing the join payload the client opens `/ws?id={playerId}` using `ws://` or `wss://` based on the page protocol. On success the server immediately publishes a `state` message so late joiners start from the latest tick. 【F:server/main.go†L211-L233】【F:client/network.js†L698-L726】
3. **Steady state** – The hub advances the simulation at 15 Hz and sends snapshot broadcasts. Clients keep their session alive by submitting input/ability/console messages plus heartbeat pings every two seconds. Players missing three intervals (~6 s) are culled from the world. 【F:server/hub.go†L504-L567】【F:server/constants.go†L7-L17】【F:client/network.js†L916-L999】【F:server/simulation.go†L344-L372】

If the socket drops the client tears down local state and re-runs the join flow after a short delay. 【F:client/network.js†L1030-L1070】

## HTTP Endpoints

- `POST /join` – Returns the join payload described above. Clients send no body.
- `POST /world/reset` – Accepts JSON toggles for obstacles, NPCs, lava, and counts, plus an optional deterministic `seed`. The server normalises the request, rebuilds the world, and rebroadcasts a fresh snapshot. 【F:server/main.go†L63-L150】【F:client/network.js†L1088-L1156】
- `GET /ws?id=...` – Upgrades to the WebSocket connection and streams all real-time messages. 【F:server/main.go†L200-L302】
- `GET /diagnostics` – Serves heartbeat metadata (player IDs, last heartbeat times, RTT) alongside tick and heartbeat intervals for dashboards, plus each subscriber's latest acknowledged tick. 【F:server/main.go†L33-L61】【F:server/hub.go†L534-L560】
- `GET /health` – Returns `ok` for liveness checks. 【F:server/main.go†L24-L31】

## Server → Client Messages

| Type | Description |
| --- | --- |
| `state` | Primary snapshot broadcast. Includes `players`, `npcs`, `obstacles`, `effects`, optional `effectTriggers`, optional `groundItems`, the current tick (`t`), the `serverTime` of emission, and the world configuration. Sent on every tick and immediately after subscribe/reset events. 【F:server/messages.go†L14-L25】【F:server/hub.go†L547-L581】 |
| `heartbeat` | Reply to client heartbeats containing `serverTime`, the echoed `clientTime`, and the computed `rtt` (milliseconds). 【F:server/messages.go†L40-L45】【F:server/main.go†L276-L315】 |
| `console_ack` | Response for debug console commands with `status`, optional `reason`, the affected quantity, and the target ground stack ID when relevant. Triggered by `drop_gold` / `pickup_gold`. 【F:server/messages.go†L31-L38】【F:server/hub.go†L312-L444】 |

The client queues transient `effectTriggers` to drive visuals once and ignores duplicates using a processed-ID set. 【F:client/network.js†L577-L620】

## Client → Server Messages

All payloads are JSON objects with a `type` string:

| Type | Fields | Notes |
| --- | --- | --- |
| `input` | `dx`, `dy`, `facing` | Movement intent (unit vector components) plus optional facing override. Processed every tick. 【F:client/network.js†L17-L39】【F:server/hub.go†L212-L241】 |
| `path` | `x`, `y` | Requests server-driven navigation toward a clamped world coordinate. 【F:client/network.js†L41-L90】【F:server/hub.go†L243-L268】 |
| `cancelPath` | _(none)_ | Cancels any active server pathing. 【F:client/network.js†L92-L116】【F:server/hub.go†L270-L286】 |
| `action` | `action` | Fires an ability; currently `attack` and `fireball` are accepted. Extra `params` are ignored. 【F:client/network.js†L118-L158】【F:server/hub.go†L288-L310】 |
| `heartbeat` | `sentAt` | Millisecond timestamp used to compute round-trip time and timeout tracking. 【F:client/network.js†L918-L999】【F:server/hub.go†L450-L483】 |
| `console` | `cmd`, optional `qty` | Debug commands for dropping/picking gold piles. 【F:client/network.js†L160-L187】【F:server/hub.go†L312-L444】 |

The helper `sendMessage` centralises JSON serialization, simulated latency, diagnostics counters, and tags every payload with the client's latest applied tick (`ack`). 【F:client/network.js†L623-L657】

All client messages include `ack` when the browser has processed at least one server tick. The hub records the highest value observed per subscriber, logging monotonic progress and exposing the latest acknowledgement through `/diagnostics`. 【F:client/network.js†L623-L657】【F:server/main.go†L257-L320】【F:server/hub.go†L212-L241】【F:server/hub.go†L522-L560】

## World Configuration Broadcasts

Snapshots (join responses and `state` messages) always include a `config` object mirroring the server’s world toggles—obstacle flags and counts, NPC counts, lava toggles, the `seed`, and world `width`/`height`. Clients normalise the data, update diagnostics, and expose the active seed in the debug UI. 【F:server/messages.go†L4-L24】【F:server/world_config.go†L7-L46】【F:client/network.js†L662-L690】【F:client/network.js†L735-L789】

## Heartbeats and Timeouts

Heartbeats fire immediately on connection open and then every 2000 ms. On each send the client records the timestamp and updates diagnostics; when an acknowledgement arrives it computes the round-trip latency, stores the latest values, and keeps the HUD `Tick: ####` / `RTT: ## ms` badges in sync. The server enqueues heartbeat commands with the computed RTT and removes players whose last heartbeat is older than six seconds. 【F:client/network.js†L732-L999】【F:server/constants.go†L15-L17】【F:server/hub.go†L450-L483】【F:server/simulation.go†L344-L372】

## Reconnect Behaviour

When the socket closes or errors, the client stops heartbeats, clears diagnostics counters, drops local actor snapshots, and schedules a fresh `/join` after one second. This keeps reconnect attempts bounded while ensuring players rehydrate the world state cleanly. 【F:client/network.js†L1001-L1070】
