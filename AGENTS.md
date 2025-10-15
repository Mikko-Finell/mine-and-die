You are responsible for implementing the bugs initiative: A project for stabilizing the codebase.

Start by reading `docs/bugs.md` 

Then continue the work outlined there. If you discover new bugs during your work, add them to `bugs.md` unless they are blocking for the active task.

# Using `docs/bugs.md`

This file is the single source of truth for known bugs and fixes in flight. Keep it current as you work; no other bug tracker is required.

---

### Updating the document

* When you log a bug, add a row in **Active Bugs** with a concise title, impact/severity tag (e.g., [crit]/[high]/[med]/[low]), status, and a one-line repro or clue (command, scenario, or test name).
* When you start fixing it, switch status to 🟡 Doing and add a short note if helpful (e.g., file/function names, linked PR, test added). Avoid line numbers.
* When it’s fixed and merged, set 🟢 Done and keep the one-line repro so readers know what was addressed. If it’s obsolete/duplicate, strike it through and add a brief note (e.g., “duplicate of #12”).
* Always update the file whenever a bug’s state changes or a related PR merges.

---

### Status symbols

| Symbol     | Meaning                                     |
| ---------- | ------------------------------------------- |
| ⚪ Planned  | Logged, not started                         |
| 🟡 Doing   | Under investigation / being fixed           |
| 🟢 Done    | Fixed, merged, and verified                 |
| 🔴 Blocked | Waiting on prerequisite, env, or dependency |

---

### Writing style

* Be brief and factual: **Observed → Expected → Minimal clue** (test name, command, or module/function).
* Filenames/functions are fine; avoid line numbers (they churn).
* Prefer concrete repros over prose; link to failing test if one exists.
* Anyone should be able to scan the table and know what’s broken, why it matters, and what’s happening next.
