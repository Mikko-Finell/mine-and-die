# B.U.G.S. — Behavioral Unification & General Stabilization

This document tracks the ongoing effort to reduce defects and keep the game reliable. Developers can continue the work by using the roadmap and active bugs tables below — no other documents are needed.

## Roadmap

| Phase | Goal                                      | Exit Criteria                                                 | Status         |
| ----- | ----------------------------------------- | ------------------------------------------------------------- | -------------- |
| 1     |  |     | 🟡 In progress |
| 2     |            |          | ⚪ Planned      |
| 3     |                        |  | ⚪ Planned      |
| 4     |                        |         | ⚪ Planned      |

## Active Bugs

| Bug                               | Impact                                  | Status    | Notes                                                          |
| --------------------------------- | --------------------------------------- | --------- | -------------------------------------------------------------- |

(Add new rows as bugs are logged. When you start one, set 🟡 Doing; when merged and verified, set 🟢 Done. If obsolete or duplicate, strike through with a short note.)

## Quality Goals

* Reproducible: every bug entry includes a minimal repro (command, test name, or scenario).
* Deterministic: simulation/replication paths avoid nondeterministic branches.
* No zombies: entities/items removed on server are removed on clients without keyframe reliance.
* Tests with fixes: every fix lands with a failing test turned green.
* Minimal surface area: prefer single code paths per behavior to reduce bug vectors.
