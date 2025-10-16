We are working on a re-write of the client app and to improve the server's contract.

Goals:

- The client shall only send inputs and render the world according to the authoritive commands received from server. The client is allowed to lerp movement for smoothing.
- Any configs or rules the client needs to use must be either read from a shared source that the server also reads from, or it must be sent by the server.
- The client must never use heuristics to infer positions, movement, ids, etc.
- The client must never use normalization of server data, feature flags, combatibility layers, shims, or anything of that nature.
- Do not bloat the client code with safety checks for server data. If the contract states a field has a certain type, that's what it will have.

Do not trust the docs, they are probably outdated.

You are responsible for implementing the **effectsgen initiative** — a project for unifying and generating effect contracts between the Go server and the TypeScript client.

Start by reading `docs/architecture/effectsgen-roadmap.md` and `docs/architecture/effectsgen-spec.md`.
Then continue the work outlined there. If you discover new requirements, inconsistencies, or edge cases during your work, add them directly to `effectsgen-roadmap.md` unless they block the active task.

---

# Using `effectsgen-roadmap.md`

This file is the single source of truth for all ongoing work related to the **effect contract generation pipeline**.
Keep it up to date as you progress — no other tracker or issue list is required.

---

### Updating the document

* When you begin a task, update the **Roadmap** or **Active Work** tables with the current phase, status symbol, and a short one-line summary of what’s happening.
* If you add a new subtask, keep it concise: title, purpose, and where in the code or spec it belongs (e.g., `server/effects/contract/registry.go`, `client/generated/effect-contracts.ts`).
* When a phase or item is complete, change its status to 🟢 and leave the short description intact for auditability.
* If something is blocked or deferred, set 🔴 Blocked and add a short reason (dependency, spec gap, or test coverage missing).
* Always update the file after completing, starting, or reprioritising a task — treat it as the authoritative state of the project.

---

### Status symbols

| Symbol         | Meaning                                            |
| -------------- | -------------------------------------------------- |
| ⚪ Planned      | Logged, not yet started                            |
| 🟡 In progress | Being designed or implemented                      |
| 🟢 Done        | Completed, merged, and validated                   |
| 🔴 Blocked     | Waiting on dependency, spec clarification, or test |

---

### Writing style

* Be **short and concrete** — describe what’s being implemented, not what might be.
* Reference **files or packages**, not line numbers (they change often).
* When relevant, note the **entry point** or **artifact** being produced (e.g. CLI tool, schema file, generated TS output).
* Anyone should be able to open `effectsgen-roadmap.md` and instantly see what’s active, what’s next, and where to contribute.

---

### Quick workflow

1. Read the relevant spec and roadmap section.
2. Make progress on the next 🟡 item.
3. Update the roadmap with your change.
4. Run and verify generator/tests before marking 🟢 Done.
5. Suggest the next logical step at the bottom of the document when you finish a phase.
