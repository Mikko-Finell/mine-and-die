### Instructions (minimal, mission-driving)

We are executing the **IDIOM mission** — progressively refactoring the server into small, idiomatic Go packages while preserving exact simulation behavior.

**`docs/architecture/idiomaticity-mission.md` is the source of truth.**
All coordination happens there — phases, order, progress markers, and next task.

---

### How to work

* Open the mission file. Find the phase marked **`[TODO`** or **`[IN PROGRESS]`**.
* Take the **top item under "Next task"**. Do not skip ahead unless explicitly stated.
* Implement the task **without changing behavior**. Golden determinism tests must pass unchanged.
* When you finish:

  * Mark that exact task done.
  * If the change you made was relatively small and you see a series of similarly small changes with similar patterns in the mission file, go ahead and **finish the "Next task" too**. The current work is very repetetive and simple, and we have good tests to rely on, no need to individually commit every small change. Repeat this until you've accumulated a decently sized commit (at least 500 LOC changes).
 
* If blocked on something external, mark the phase `[BLOCKED]` and state why.
* Try to avoid adding shims or feature flags or anything like that if you can. If the task requires it, mark them with code comments.
* If a task is large and complicated so that it can only be executed over several smaller tasks and you have to add a flag for it, then clearly mark the flag with a code comment `// TEMPORARY FLAG REMOVE WHEN DONE`
* When a change is complete remove flags and shims that had to be added, do not leave legacy code or tests around to confuse future contributors.

---

### Core discipline

* Do not normalize / reshape server state to make the code “convenient”.
* No globals, no hidden concurrency in simulation.
* Behavior parity is absolute — golden determinism must not change.
* Don't add line number citations to commit messages.

---

**That’s it.** The file drives the work.
You read the plan, do the next task, update the plan, commit.
