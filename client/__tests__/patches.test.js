import { describe, expect, it } from "vitest";
import {
  createPatchState,
  updatePatchState,
  PATCH_KIND_PLAYER_POS,
  PATCH_KIND_PLAYER_FACING,
  PATCH_KIND_PLAYER_INTENT,
  PATCH_KIND_PLAYER_HEALTH,
  PATCH_KIND_PLAYER_INVENTORY,
} from "../patches.js";

function makePlayer(overrides = {}) {
  return {
    id: "player-1",
    x: 1,
    y: 2,
    facing: "up",
    health: 10,
    maxHealth: 10,
    inventory: { slots: [] },
    ...overrides,
  };
}

function deepFreeze(value) {
  if (!value || typeof value !== "object") {
    return value;
  }
  if (value instanceof Map || value instanceof Set) {
    return value;
  }
  Object.freeze(value);
  for (const key of Object.keys(value)) {
    deepFreeze(value[key]);
  }
  return value;
}

function freezeState(state) {
  deepFreeze(state);
  if (state.patchHistory) {
    Object.freeze(state.patchHistory);
  }
  return state;
}

describe("updatePatchState", () => {
  it("builds a baseline and patched snapshot when no patches are provided", () => {
    const initial = freezeState(createPatchState());
    const payload = deepFreeze({ t: 1, players: [makePlayer()] });

    const result = updatePatchState(initial, payload, { source: "join" });

    expect(Object.keys(result.baseline.players)).toEqual(["player-1"]);
    expect(Object.keys(result.patched.players)).toEqual(["player-1"]);
    expect(result.baseline.players["player-1"]).not.toBe(
      result.patched.players["player-1"],
    );
    expect(result.baseline.players["player-1"]).toMatchObject({
      id: "player-1",
      x: 1,
      y: 2,
      facing: "up",
    });
    expect(result.lastAppliedPatchCount).toBe(0);
    expect(result.errors).toEqual([]);
    expect(result.lastUpdateSource).toBe("join");
    expect(result.lastTick).toBe(1);
  });

  it("applies player patches onto the baseline snapshot", () => {
    const seeded = updatePatchState(createPatchState(), deepFreeze({ t: 2, players: [makePlayer()] }), {
      source: "join",
    });
    freezeState(seeded);
    const payload = deepFreeze({
      t: 3,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 5, y: 6 } },
        {
          kind: PATCH_KIND_PLAYER_FACING,
          entityId: "player-1",
          payload: { facing: "left" },
        },
        {
          kind: PATCH_KIND_PLAYER_INTENT,
          entityId: "player-1",
          payload: { dx: 1.5, dy: -0.25 },
        },
        {
          kind: PATCH_KIND_PLAYER_HEALTH,
          entityId: "player-1",
          payload: { health: 7, maxHealth: 12 },
        },
        {
          kind: PATCH_KIND_PLAYER_INVENTORY,
          entityId: "player-1",
          payload: { slots: [{ slot: 0, item: { type: "gold", quantity: 3 } }] },
        },
      ],
    });

    const result = updatePatchState(seeded, payload, { source: "state" });
    const patched = result.patched.players["player-1"];

    expect(result.lastAppliedPatchCount).toBe(5);
    expect(patched).toMatchObject({
      x: 5,
      y: 6,
      facing: "left",
      intentDX: 1.5,
      intentDY: -0.25,
      health: 7,
      maxHealth: 12,
    });
    expect(patched.inventory.slots).toEqual([
      { slot: 0, item: { type: "gold", quantity: 3 } },
    ]);
    expect(result.baseline.players["player-1"].x).toBe(1);
    expect(result.errors).toEqual([]);
    expect(result.lastUpdateSource).toBe("state");
    expect(result.lastTick).toBe(3);
  });

  it("records errors for invalid patch envelopes and respects the history limit", () => {
    const seeded = updatePatchState(createPatchState(), deepFreeze({ t: 4, players: [makePlayer()] }), {
      source: "join",
    });
    expect(seeded.errors).toEqual([]);
    freezeState(seeded);

    const payload = deepFreeze({
      t: 5,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "missing", payload: { x: 8, y: 9 } },
        { kind: "npc_update", entityId: "player-1", payload: { hp: 5 } },
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: "oops" } },
      ],
    });

    const result = updatePatchState(seeded, payload, {
      source: "state",
      errorLimit: 2,
    });

    expect(result.lastAppliedPatchCount).toBe(0);
    expect(result.errors.length).toBe(2);
    expect(result.errors.map((entry) => entry.message)).toEqual([
      "unsupported patch kind: npc_update",
      "invalid position payload",
    ]);
    expect(result.errors.every((entry) => entry.source === "state")).toBe(true);
    expect(result.lastError.message).toBe("invalid position payload");
    expect(result.lastError.kind).toBe(PATCH_KIND_PLAYER_POS);
    expect(seeded.errors).toEqual([]);
  });

  it("deduplicates repeated patch batches at the same tick", () => {
    const joinPayload = deepFreeze({ t: 12, players: [makePlayer()] });
    const seeded = updatePatchState(createPatchState(), joinPayload, { source: "join" });
    freezeState(seeded);

    const patchPayload = deepFreeze({
      t: 12,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 9, y: 9 } },
      ],
    });

    const first = updatePatchState(seeded, patchPayload, { source: "state" });
    expect(first.lastAppliedPatchCount).toBe(1);
    expect(first.patched.players["player-1"].x).toBe(9);
    freezeState(first);

    const second = updatePatchState(first, patchPayload, { source: "state" });
    expect(second.lastAppliedPatchCount).toBe(0);
    expect(second.patched.players["player-1"].x).toBe(9);
    expect(second.lastTick).toBe(12);
  });

  it("rejects out-of-order batches and leaves prior state untouched", () => {
    const seedPayload = deepFreeze({ t: 20, players: [makePlayer()] });
    const seeded = updatePatchState(createPatchState(), seedPayload, { source: "state" });
    const live = updatePatchState(seeded, deepFreeze({
      t: 20,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 11, y: 12 } },
      ],
    }), { source: "state" });
    freezeState(live);

    const stale = updatePatchState(live, deepFreeze({
      t: 19,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 1, y: 1 } },
      ],
    }), { source: "state" });

    expect(stale.lastAppliedPatchCount).toBe(0);
    expect(stale.patched.players["player-1"].x).toBe(11);
    expect(stale.errors[stale.errors.length - 1].message).toMatch(
      /out-of-order patch tick 19 < 20/,
    );
    expect(stale.lastTick).toBe(20);
  });

  it("hard rejects non-finite position payloads", () => {
    const seeded = updatePatchState(createPatchState(), deepFreeze({ t: 30, players: [makePlayer()] }), {
      source: "join",
    });
    freezeState(seeded);

    const result = updatePatchState(seeded, deepFreeze({
      t: 30,
      players: [makePlayer()],
      patches: [
        {
          kind: PATCH_KIND_PLAYER_POS,
          entityId: "player-1",
          payload: { x: Infinity, y: NaN },
        },
      ],
    }), { source: "state" });

    expect(result.lastAppliedPatchCount).toBe(0);
    expect(result.errors[result.errors.length - 1].message).toBe(
      "invalid position payload",
    );
    expect(result.patched.players["player-1"].x).toBe(1);
  });

  it("resets patch history on resync and drops stale inflight patches", () => {
    const seeded = updatePatchState(createPatchState(), deepFreeze({ t: 40, players: [makePlayer()] }), {
      source: "state",
    });
    const withLivePatch = updatePatchState(seeded, deepFreeze({
      t: 40,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 21, y: 22 } },
      ],
    }), { source: "state" });
    freezeState(withLivePatch);

    const resynced = updatePatchState(withLivePatch, deepFreeze({
      t: 5,
      players: [makePlayer({ x: 2, y: 3 })],
    }), { source: "state", resetHistory: true });

    expect(resynced.lastTick).toBe(5);
    expect(resynced.patched.players["player-1"].x).toBe(2);

    const fresh = updatePatchState(resynced, deepFreeze({
      t: 6,
      players: [makePlayer({ x: 2, y: 3 })],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 7, y: 8 } },
      ],
    }), { source: "state" });
    expect(fresh.patched.players["player-1"].x).toBe(7);
    expect(fresh.lastAppliedPatchCount).toBe(1);
    freezeState(fresh);

    const stale = updatePatchState(fresh, deepFreeze({
      t: 5,
      players: [makePlayer({ x: 2, y: 3 })],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 99, y: 99 } },
      ],
    }), { source: "state" });

    expect(stale.patched.players["player-1"].x).toBe(7);
    expect(stale.lastAppliedPatchCount).toBe(0);
    expect(stale.errors[stale.errors.length - 1].message).toMatch(
      /out-of-order patch tick 5 < 6/,
    );
  });
});
