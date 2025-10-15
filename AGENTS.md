We are working on a re-write of the client app and to improve the server's contract.

Goals:
- The client shall only send inputs and render the world according to the authoritive commands received from server.
- Any configs or rules the client needs to use must be either read from a shared source that the server also reads from, or it must be sent by the server.
- The client must never use heuristics to infer positions, movement, ids, or any kind of config that affects the rendered state.
- The client must never use normalization of server data, feature flags, combatibility layers, shims, or anything of that nature.
- The client is allowed to lerp movement for smoothing.
- Do not bloat the client code with safety checks for server data. If the contract states a field has a certain type, that's what it will have.

Notes:
* Do not trust the docs, they are probably outdated.
