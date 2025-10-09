import { describe, expect, it } from "vitest";
import {
  createPatchState,
  updatePatchState,
  PATCH_KIND_PLAYER_POS,
  PATCH_KIND_PLAYER_FACING,
  PATCH_KIND_PLAYER_INTENT,
  PATCH_KIND_PLAYER_HEALTH,
  PATCH_KIND_PLAYER_INVENTORY,
  PATCH_KIND_NPC_POS,
  PATCH_KIND_NPC_FACING,
  PATCH_KIND_NPC_HEALTH,
  PATCH_KIND_NPC_INVENTORY,
  PATCH_KIND_EFFECT_POS,
  PATCH_KIND_EFFECT_PARAMS,
  PATCH_KIND_GROUND_ITEM_POS,
  PATCH_KIND_GROUND_ITEM_QTY,
  applyPatchesToSnapshot,
  buildBaselineFromSnapshot,
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

function makeNPC(overrides = {}) {
  return {
    id: "npc-1",
    x: 4,
    y: 5,
    facing: "down",
    health: 20,
    maxHealth: 20,
    type: "goblin",
    aiControlled: true,
    experienceReward: 5,
    inventory: { slots: [] },
    ...overrides,
  };
}

function makeEffect(overrides = {}) {
  return {
    id: "effect-1",
    type: "fireball",
    owner: "player-1",
    start: 100,
    duration: 250,
    x: 6,
    y: 7,
    width: 24,
    height: 24,
    params: { remaining: 1.5 },
    ...overrides,
  };
}

function makeGroundItem(overrides = {}) {
  return {
    id: "ground-1",
    type: "gold",
    x: 8,
    y: 9,
    qty: 3,
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
    const payload = deepFreeze({
      t: 1,
      players: [makePlayer()],
      npcs: [makeNPC()],
      effects: [makeEffect()],
      groundItems: [makeGroundItem()],
    });

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
    expect(Object.keys(result.baseline.npcs)).toEqual(["npc-1"]);
    expect(Object.keys(result.patched.npcs)).toEqual(["npc-1"]);
    expect(result.baseline.npcs["npc-1"]).not.toBe(
      result.patched.npcs["npc-1"],
    );
    expect(result.patched.npcs["npc-1"]).toMatchObject({
      type: "goblin",
      facing: "down",
      x: 4,
      y: 5,
    });
    expect(Object.keys(result.baseline.effects)).toEqual(["effect-1"]);
    expect(Object.keys(result.patched.effects)).toEqual(["effect-1"]);
    expect(result.baseline.effects["effect-1"]).not.toBe(
      result.patched.effects["effect-1"],
    );
    expect(result.patched.effects["effect-1"]).toMatchObject({
      type: "fireball",
      x: 6,
      y: 7,
    });
    expect(result.patched.effects["effect-1"].params).toEqual({ remaining: 1.5 });
    expect(Object.keys(result.baseline.groundItems)).toEqual(["ground-1"]);
    expect(Object.keys(result.patched.groundItems)).toEqual(["ground-1"]);
    expect(result.baseline.groundItems["ground-1"]).not.toBe(
      result.patched.groundItems["ground-1"],
    );
    expect(result.patched.groundItems["ground-1"]).toMatchObject({
      type: "gold",
      qty: 3,
      x: 8,
      y: 9,
    });
    expect(result.lastAppliedPatchCount).toBe(0);
    expect(result.errors).toEqual([]);
    expect(result.lastUpdateSource).toBe("join");
    expect(result.lastTick).toBe(1);
    expect(result.lastSequence).toBe(1);
    expect(result.baseline.sequence).toBe(1);
    expect(result.patched.sequence).toBe(1);
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

  it("applies NPC, effect, and ground item patches onto the baseline snapshot", () => {
    const basePayload = deepFreeze({
      t: 6,
      players: [makePlayer()],
      npcs: [makeNPC()],
      effects: [makeEffect()],
      groundItems: [makeGroundItem()],
    });
    const seeded = updatePatchState(createPatchState(), basePayload, { source: "join" });
    freezeState(seeded);

    const payload = deepFreeze({
      t: 7,
      players: [makePlayer()],
      npcs: [makeNPC()],
      effects: [makeEffect()],
      groundItems: [makeGroundItem()],
      patches: [
        { kind: PATCH_KIND_NPC_POS, entityId: "npc-1", payload: { x: 14, y: 16 } },
        {
          kind: PATCH_KIND_NPC_FACING,
          entityId: "npc-1",
          payload: { facing: "left" },
        },
        {
          kind: PATCH_KIND_NPC_HEALTH,
          entityId: "npc-1",
          payload: { health: 12, maxHealth: 22 },
        },
        {
          kind: PATCH_KIND_NPC_INVENTORY,
          entityId: "npc-1",
          payload: { slots: [{ slot: 1, item: { type: "gold", quantity: 9 } }] },
        },
        { kind: PATCH_KIND_EFFECT_POS, entityId: "effect-1", payload: { x: 9, y: 10 } },
        {
          kind: PATCH_KIND_EFFECT_PARAMS,
          entityId: "effect-1",
          payload: { params: { remaining: 0.5, speed: 2 } },
        },
        { kind: PATCH_KIND_GROUND_ITEM_POS, entityId: "ground-1", payload: { x: 2, y: 3 } },
        { kind: PATCH_KIND_GROUND_ITEM_QTY, entityId: "ground-1", payload: { qty: 7 } },
      ],
    });

    const result = updatePatchState(seeded, payload, { source: "state" });

    expect(result.lastAppliedPatchCount).toBe(8);
    const npc = result.patched.npcs["npc-1"];
    expect(npc).toMatchObject({
      x: 14,
      y: 16,
      facing: "left",
      health: 12,
      maxHealth: 22,
    });
    expect(npc.inventory.slots).toEqual([
      { slot: 1, item: { type: "gold", quantity: 9 } },
    ]);
    const effect = result.patched.effects["effect-1"];
    expect(effect).toMatchObject({ x: 9, y: 10 });
    expect(effect.params).toEqual({ remaining: 0.5, speed: 2 });
    const groundItem = result.patched.groundItems["ground-1"];
    expect(groundItem).toMatchObject({ x: 2, y: 3, qty: 7 });
    expect(result.baseline.npcs["npc-1"].x).toBe(4);
    expect(result.baseline.effects["effect-1"].x).toBe(6);
    expect(result.baseline.groundItems["ground-1"].qty).toBe(3);
    expect(result.errors).toEqual([]);
    expect(result.lastUpdateSource).toBe("state");
    expect(result.lastTick).toBe(7);
    expect(result.lastSequence).toBe(7);
    expect(result.patched.sequence).toBe(7);
  });

  it("normalizes entity identifiers when seeding baseline state", () => {
    const payload = deepFreeze({
      t: 11,
      players: [makePlayer({ id: "  player-1  " })],
      npcs: [makeNPC({ id: "\tnpc-2\n" })],
      effects: [makeEffect({ id: " effect-3 " })],
      groundItems: [makeGroundItem({ id: "\nground-4\t" })],
    });

    const baseline = buildBaselineFromSnapshot(payload);

    expect(Object.keys(baseline.players)).toEqual(["player-1"]);
    expect(Object.keys(baseline.npcs)).toEqual(["npc-2"]);
    expect(Object.keys(baseline.effects)).toEqual(["effect-3"]);
    expect(Object.keys(baseline.groundItems)).toEqual(["ground-4"]);
    expect(baseline.players["player-1"].id).toBe("player-1");
    expect(baseline.npcs["npc-2"].id).toBe("npc-2");
    expect(baseline.effects["effect-3"].id).toBe("effect-3");
    expect(baseline.groundItems["ground-4"].id).toBe("ground-4");
    expect(baseline.sequence).toBe(11);
  });

  it("normalizes patch kinds and entity identifiers before applying handlers", () => {
    const base = {
      players: {
        "player-1": makePlayer({ id: "player-1", intentDX: 0, intentDY: 0 }),
      },
      npcs: {
        "npc-1": makeNPC({ id: "npc-1" }),
      },
      effects: {},
      groundItems: {},
    };

    const { players, npcs, errors, appliedCount } = applyPatchesToSnapshot(base, [
      {
        kind: " NPC_POS ",
        entityId: " npc-1 ",
        payload: { x: 32, y: 48 },
      },
      {
        kind: "PLAYER_POS",
        entityId: " player-1 ",
        payload: { x: 5, y: 7 },
      },
    ]);

    expect(errors).toEqual([]);
    expect(appliedCount).toBe(2);
    expect(npcs["npc-1"].x).toBe(32);
    expect(npcs["npc-1"].y).toBe(48);
    expect(players["player-1"].x).toBe(5);
    expect(players["player-1"].y).toBe(7);
  });

  it("normalizes entity identifiers when cloning baseline maps", () => {
    const base = {
      players: {
        "  player-1  ": makePlayer({ id: "  player-1  " }),
      },
      npcs: {
        " npc-2 ": makeNPC({ id: " npc-2 " }),
      },
      effects: {
        " effect-3 ": makeEffect({ id: " effect-3 " }),
      },
      groundItems: {
        " ground-4 ": makeGroundItem({ id: " ground-4 " }),
      },
    };

    const { players, npcs, effects, groundItems } = applyPatchesToSnapshot(base, []);

    expect(Object.keys(players)).toEqual(["player-1"]);
    expect(Object.keys(npcs)).toEqual(["npc-2"]);
    expect(Object.keys(effects)).toEqual(["effect-3"]);
    expect(Object.keys(groundItems)).toEqual(["ground-4"]);
    expect(players["player-1"].id).toBe("player-1");
    expect(npcs["npc-2"].id).toBe("npc-2");
    expect(effects["effect-3"].id).toBe("effect-3");
    expect(groundItems["ground-4"].id).toBe("ground-4");
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
    const joinPayload = deepFreeze({ t: 12, sequence: 99, players: [makePlayer()] });
    const seeded = updatePatchState(createPatchState(), joinPayload, { source: "join" });
    freezeState(seeded);

    const patchPayload = deepFreeze({
      t: 12,
      sequence: 200,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 9, y: 9 } },
      ],
    });

    const first = updatePatchState(seeded, patchPayload, { source: "state" });
    expect(first.lastAppliedPatchCount).toBe(1);
    expect(first.patched.players["player-1"].x).toBe(9);
    expect(first.lastSequence).toBe(200);
    freezeState(first);

    const second = updatePatchState(first, patchPayload, { source: "state" });
    expect(second.lastAppliedPatchCount).toBe(0);
    expect(second.patched.players["player-1"].x).toBe(9);
    expect(second.lastTick).toBe(12);
    expect(second.lastSequence).toBe(200);
  });

  it("accepts legacy seq field for backwards compatibility", () => {
    const payload = deepFreeze({ t: 5, seq: 75, players: [makePlayer()] });
    const state = updatePatchState(createPatchState(), payload, { source: "state" });
    expect(state.lastSequence).toBe(75);
  });

  it("rejects out-of-order batches and leaves prior state untouched", () => {
    const seedPayload = deepFreeze({ t: 20, sequence: 300, players: [makePlayer()] });
    const seeded = updatePatchState(createPatchState(), seedPayload, { source: "state" });
    const live = updatePatchState(seeded, deepFreeze({
      t: 20,
      sequence: 301,
      players: [makePlayer()],
      patches: [
        { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 11, y: 12 } },
      ],
    }), { source: "state" });
    freezeState(live);

    const stale = updatePatchState(live, deepFreeze({
      t: 19,
      sequence: 250,
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
    expect(stale.lastSequence).toBe(301);
  });

  it("rejects batches when the sequence number regresses", () => {
    const base = updatePatchState(
      createPatchState(),
      deepFreeze({ t: 50, sequence: 400, players: [makePlayer()] }),
      { source: "state" },
    );
    const live = updatePatchState(
      base,
      deepFreeze({
        t: 50,
        sequence: 401,
        players: [makePlayer()],
        patches: [
          { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 13, y: 14 } },
        ],
      }),
      { source: "state" },
    );
    freezeState(live);

    const regressed = updatePatchState(
      live,
      deepFreeze({
        t: 50,
        sequence: 400,
        players: [makePlayer()],
        patches: [
          { kind: PATCH_KIND_PLAYER_POS, entityId: "player-1", payload: { x: 1, y: 1 } },
        ],
      }),
      { source: "state" },
    );

    expect(regressed.lastAppliedPatchCount).toBe(0);
    expect(regressed.patched.players["player-1"].x).toBe(13);
    expect(regressed.errors[regressed.errors.length - 1].message).toMatch(
      /out-of-order patch sequence 400 < 401/,
    );
    expect(regressed.lastSequence).toBe(401);
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
    expect(resynced.lastSequence).toBe(5);
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
    expect(fresh.lastSequence).toBe(6);
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
    expect(stale.lastSequence).toBe(6);
  });
});
