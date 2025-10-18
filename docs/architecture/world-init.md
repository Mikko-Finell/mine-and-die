# Roadmap: “Hydrate world → translate to geometry → draw → show movement → layer effects”

This plan stitches together what your team wrote, in a clean sequence. It only references names/parts they already mentioned.

## [DONE] Phase 1 — Ingest the authoritative world stream

### Summary

* Client join now parses full world payloads and hydrates the world store immediately.
* State/keyframe messages build `WorldKeyframe` / `WorldPatchBatch` objects and update the store before rendering.

### Next task

Move to Phase 2.

---

## [DONE] Phase 2 — Turn world state into renderable geometry

### Summary

* `GameClientOrchestrator.buildRenderBatch` now merges `worldState.snapshot()` output into `staticGeometry`, layering a world background/grid and world entities ahead of effect geometry.
* Static geometry entries for players, NPCs, obstacles, and ground items carry metadata (facing, health, size, quantity) so the renderer can paint accurate silhouettes on the 100×100 board.

### Next task

Move to Phase 3.

---

## [DONE] Phase 3 — Draw the static geometry (before the effects)

### Summary

* `CanvasRenderer.stepFrame` now sorts render layers and rasterizes world/static geometry before delegating to runtime effects, keeping the waypoint ring overlay intact.
* The renderer resizes the canvas using authoritative world dimensions derived from the background geometry so the 100×100 field renders at the correct scale.

### Next task

Move to Phase 4.

---

## [DONE] Phase 4 — Show server-driven movement

### Summary

* State envelopes now translate player and NPC patches (`player_pos`, `player_facing`, `player_intent`, `npc_pos`, `npc_health`) into world-store updates every tick, keeping the authoritative snapshot in sync.
* `buildRenderBatch` consumes the refreshed snapshot so static geometry for actors moves immediately when the server emits new coordinates and facing data.
* Regression coverage (`client/__tests__/client-manager.test.ts`) verifies that applying movement patches updates both the store and rendered geometry for players and NPCs.

### Next task

Move to Phase 5.

---

## [DONE] Phase 5 — Validate effect layering and effect-driven geometry

### Next task

**Goal**
Ensure generated contracts render correctly on top of the newly visible world.

**Exit criteria**

* The runtime adapter continues to map catalog entries (e.g., `attack → melee/swing`) to `@js-effects/effects-lib` definitions.
* Effect-driven geometry produced by the orchestrator remains visible with the new static pass (ordering is correct).
* Swing/impact effects render above actors on the 100×100 field during normal play.

**Notes / places your team already pointed to**

* The effect pipeline from Phase 7 (earlier work) is already live; this is a layering/visibility check after Phases 2–3.

### Summary

* `GameClientOrchestrator` now requires explicit renderer layer mappings per delivery kind, preventing area/target effects from silently falling back to a lower z-index layer.
* Regression coverage confirms effect-driven static geometry sorts above hydrated world actors and that missing layer configurations throw immediately.

---

## [IN PROGRESS] Phase 6 — Frame cadence, sizing, and basic observability

### Next task

**Goal**
Keep the loop smooth and make debugging straightforward.

**Exit criteria**

* On join, log world dimensions and received entity counts; on state ticks, log applied patch batch sizes (lightweight dev logs).
* Verify canvas resize behavior when world dimensions change (if/when you change defaults).
* Confirm no regressions to effect playback cadence when the static pass is enabled.

**Notes**

* Keep this minimal (the team already has the necessary hooks); focus is on clarity during bring-up.

### Next task

Instrument join and state handlers to emit debug logs for world dimensions, entity counts, and patch batch sizes so we can trace hydration/resync behavior while testing cadence tweaks.

---

## [TODO] Phase 7 — Harness and regression coverage

### Next task

**Goal**
Lock in the behavior that makes the scene playable.

**Exit criteria**

* Headless harness covers:

  * join → `applyKeyframe` hydrates players/NPCs/obstacles/ground items,
  * state ticks → `applyPatchBatch` moves actors,
  * `buildRenderBatch` includes both world and effect geometry,
  * `CanvasRenderer.stepFrame` draws `staticGeometry` and effects in the intended order.
* Tests exercise movement acknowledgement ticks and ensure actor positions advance only forward (no regressions).

**Notes / places your team already pointed to**

* You’ve already been using a headless harness for lifecycle/effect paths; extend it to assert world hydration and batch composition.

---

## [TODO] Phase 8 — Wrap-up and ready state

### Next task

**Goal**
Reach “playable scene” as described by the team.

**Exit criteria**

* On a clean client boot into a default world:

  * The 100×100 board is visible,
  * A player avatar appears,
  * Clicking sets a path; after server acknowledgement, the pawn walks along server-driven waypoints,
  * Contract-driven effects (e.g., melee/swing for `attack`) animate above the actor.
* The roadmap/docs are updated to mark the above phases complete.

---

## Implementation order (dependency chain)

1. **Phase 1** (ingest) → 2) **Phase 2** (translate to geometry) → 3) **Phase 3** (draw) → 4) **Phase 4** (movement) → 5) **Phase 5** (effects layering) → 6) **Phase 6** (observability) → 7) **Phase 7** (tests) → 8) **Phase 8** (wrap-up).

This keeps progress “visible” as soon as possible: once Phases 1–3 land, you immediately see the world; Phase 4 makes it feel alive; Phase 5 confirms effects remain correct; Phases 6–7 harden and verify; Phase 8 declares success.
