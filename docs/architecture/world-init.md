# Roadmap: “Hydrate world → translate to geometry → draw → show movement → layer effects”

This plan stitches together what your team wrote, in a clean sequence. It only references names/parts they already mentioned.

## [IN PROGRESS] Phase 1 — Ingest the authoritative world stream

### Next task

**Goal**
Client actually stores the world the server already sends (100×100 field, players, NPCs, obstacles, ground items, patches).

**Exit criteria**

* `JoinResponse` / `WebSocketNetworkClient.join` parse and return `players`, `npcs`, `obstacles`, `groundItems`, and any initial `patches` from the server (per `server/messages.go`).
* `GameClientOrchestrator.handleStatePayload` no longer drops world data: it forwards entity lists and the `patches` array to the world store.
* `InMemoryWorldStateStore.applyKeyframe` is called on join/keyframe payloads; `InMemoryWorldStateStore.applyPatchBatch` is called for each state tick.
* After applying a keyframe or patch batch, the client takes a fresh `worldState.snapshot()` and triggers a render request.

**Notes / places your team already pointed to**

* Parsing in `client/network.ts` (extend `JoinResponse`, update `WebSocketNetworkClient.join`).
* Wiring in `client/client-manager.ts` (helpers to build `WorldKeyframe` / `WorldPatchBatch` and to call `applyKeyframe` / `applyPatchBatch`).
* Patch types include `PatchPlayerPos` / `PatchPlayerIntent` (per `server/world_mutators.go`).
* World dimensions come from join (100×100).

---

## [TODO] Phase 2 — Turn world state into renderable geometry

### Next task

**Goal**
Translate `worldState.snapshot()` into geometry the renderer can consume, then merge with effect output.

**Exit criteria**

* `GameClientOrchestrator.buildRenderBatch` reads `worldState.snapshot()` each frame and produces `RenderBatch.staticGeometry` for:

  * a background/tile grid sized from the join’s world dimensions,
  * silhouettes/sprites for players and NPCs,
  * primitives for obstacles and any ground items.
* The result is *merged* with existing per-effect geometry and lifecycle animation intents (the effect path remains intact).

**Notes / places your team already pointed to**

* All of this happens in `client/client-manager.ts` inside `buildRenderBatch`.
* The batch already has effect data; the missing piece is populating `staticGeometry` from the world snapshot.

---

## [TODO] Phase 3 — Draw the static geometry (before the effects)

### Next task

**Goal**
Make the canvas actually show the world and entities, then layer effects above.

**Exit criteria**

* `CanvasRenderer.stepFrame` iterates `RenderBatch.staticGeometry` and rasterizes those primitives (fills/strokes, polygons/rects/circles as appropriate).
* The canvas is sized to the join’s `worldWidth` / `worldHeight` so the 100×100 map scales correctly.
* The existing effect runtime still runs (after or before, per desired layering), and the waypoint ring draw remains intact.

**Notes / places your team already pointed to**

* Work in `client/render.ts` inside `CanvasRenderer.stepFrame`.
* Today, `staticGeometry` is ignored; this phase changes that.

---

## [TODO] Phase 4 — Show server-driven movement

### Next task

**Goal**
Make the pawn move according to server-authoritative patches and acknowledged paths.

**Exit criteria**

* World patches from state envelopes (e.g., `PatchPlayerPos`, `PatchPlayerIntent`) are actually applied to the store each tick.
* `buildRenderBatch` pulls positions from `worldState.snapshot()` so the player/NPC geometry updates as patches land.
* Clicking to set a path (already wired through the client input → hub) results in visible movement as soon as the server accepts waypoints and emits position patches.

**Notes / places your team already pointed to**

* Path command plumbing is already in place server-side (`CommandSetPath`), with collision/path resolution and patch emission each tick.
* The missing piece was the client storing patches and pushing updated coordinates into the batch.

---

## [TODO] Phase 5 — Validate effect layering and effect-driven geometry

### Next task

**Goal**
Ensure generated contracts render correctly on top of the newly visible world.

**Exit criteria**

* The runtime adapter continues to map catalog entries (e.g., `attack → melee/swing`) to `@js-effects/effects-lib` definitions.
* Effect-driven geometry produced by the orchestrator remains visible with the new static pass (ordering is correct).
* Swing/impact effects render above actors on the 100×100 field during normal play.

**Notes / places your team already pointed to**

* The effect pipeline from Phase 7 (earlier work) is already live; this is a layering/visibility check after Phases 2–3.

---

## [TODO] Phase 6 — Frame cadence, sizing, and basic observability

### Next task

**Goal**
Keep the loop smooth and make debugging straightforward.

**Exit criteria**

* On join, log world dimensions and received entity counts; on state ticks, log applied patch batch sizes (lightweight dev logs).
* Verify canvas resize behavior when world dimensions change (if/when you change defaults).
* Confirm no regressions to effect playback cadence when the static pass is enabled.

**Notes**

* Keep this minimal (the team already has the necessary hooks); focus is on clarity during bring-up.

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
