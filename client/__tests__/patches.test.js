import { describe, expect, it } from "vitest";
import {
  createPatchState,
  updatePatchState,
  PATCH_KIND_PLAYER_POS,
  PATCH_KIND_PLAYER_FACING,
  PATCH_KIND_PLAYER_INTENT,
  PATCH_KIND_PLAYER_HEALTH,
  PATCH_KIND_PLAYER_INVENTORY,
  PATCH_KIND_PLAYER_EQUIPMENT,
  PATCH_KIND_PLAYER_REMOVED,
  PATCH_KIND_NPC_POS,
  PATCH_KIND_NPC_FACING,
  PATCH_KIND_NPC_HEALTH,
  PATCH_KIND_NPC_INVENTORY,
  PATCH_KIND_NPC_EQUIPMENT,
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
    equipment: { slots: [] },
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
    equipment: { slots: [] },
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

function makeEffect(overrides = {}) {
  return {
    id: "effect-1",
    x: 0,
    y: 0,
    params: { width: 10 },
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

function withRetryAttempts(state, sequence, attempts) {
  const pendingRequests =
    state.pendingKeyframeRequests instanceof Map
      ? new Map(state.pendingKeyframeRequests)
      : new Map();
  const normalizedSeq = Math.floor(sequence);
  const existing = pendingRequests.get(normalizedSeq) || {};
  pendingRequests.set(normalizedSeq, {
    attempts,
    nextRetryAt: null,
    firstRequestedAt:
      typeof existing.firstRequestedAt === "number" && Number.isFinite(existing.firstRequestedAt)
        ? existing.firstRequestedAt
        : Date.now(),
  });
  const pendingReplays = Array.isArray(state.pendingReplays)
    ? state.pendingReplays.map((entry) => {
        if (!entry || entry.sequence !== normalizedSeq) {
          return entry ? { ...entry } : entry;
        }
        const requests = Math.max(attempts, Number.isFinite(entry.requests) ? entry.requests : 1);
        return { ...entry, requests };
      })
    : [];
  return {
    ...state,
    pendingKeyframeRequests: pendingRequests,
    pendingReplays,
  };
}

describe("updatePatchState", () => {
  it("builds a baseline and patched snapshot when no patches are provided", () => {
    const initial = freezeState(createPatchState());
    const payload = deepFreeze({
      t: 1,
      players: [makePlayer()],
      npcs: [makeNPC()],
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
    expect(Object.keys(result.baseline.effects)).toEqual([]);
    expect(Object.keys(result.patched.effects)).toEqual([]);
    expect(result.baseline.effects).not.toBe(result.patched.effects);
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

  it("preserves fungibility metadata when hydrating player inventories", () => {
    const fungibilityKey = "iron_dagger::tier1";
    const payload = deepFreeze({
      t: 2,
      players: [
        makePlayer({
          inventory: {
            slots: [
              {
                slot: 0,
                item: { type: "iron_dagger", quantity: 1, fungibility_key: fungibilityKey },
              },
            ],
          },
        }),
      ],
    });

    const baseline = buildBaselineFromSnapshot(payload);

    expect(baseline.players["player-1"].inventory.slots).toEqual([
      { slot: 0, item: { type: "iron_dagger", quantity: 1, fungibility_key: fungibilityKey } },
    ]);
  });

  it("hydrates equipment slots for players and NPCs", () => {
    const payload = deepFreeze({
      t: 3,
      players: [
        makePlayer({
          equipment: {
            slots: [
              {
                slot: "MainHand",
                item: { type: "iron_sword", quantity: 1, fungibility_key: "iron_sword::tier1" },
              },
            ],
          },
        }),
      ],
      npcs: [
        makeNPC({
          equipment: {
            slots: [
              {
                slot: "Head",
                item: { type: "bronze_helm", quantity: 1 },
              },
            ],
          },
        }),
      ],
    });

    const baseline = buildBaselineFromSnapshot(payload);

    expect(baseline.players["player-1"].equipment).toEqual({
      slots: [
        { slot: "MainHand", item: { type: "iron_sword", quantity: 1, fungibility_key: "iron_sword::tier1" } },
      ],
    });
    expect(baseline.npcs["npc-1"].equipment).toEqual({
      slots: [{ slot: "Head", item: { type: "bronze_helm", quantity: 1 } }],
    });
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
    expect(result.baseline.players["player-1"].x).toBe(5);
    expect(result.errors).toEqual([]);
    expect(result.lastUpdateSource).toBe("state");
    expect(result.lastTick).toBe(3);
  });

  it("removes players when removal patches arrive", () => {
    const seeded = updatePatchState(createPatchState(), deepFreeze({ t: 2, sequence: 2, players: [makePlayer()] }), {
      source: "join",
    });
    freezeState(seeded);

    const payload = deepFreeze({
      t: 4,
      sequence: 4,
      patches: [{ kind: PATCH_KIND_PLAYER_REMOVED, entityId: "player-1" }],
    });

    const result = updatePatchState(seeded, payload, { source: "broadcast" });

    expect(result.errors).toEqual([]);
    expect(result.lastAppliedPatchCount).toBe(1);
    expect(result.patched.players).not.toHaveProperty("player-1");
    expect(result.baseline.players).not.toHaveProperty("player-1");
  });

  it("preserves fungibility metadata when applying inventory patches", () => {
    const base = buildBaselineFromSnapshot(
      deepFreeze({
        t: 4,
        players: [makePlayer()],
      }),
    );
    const patchKey = "iron_dagger::unique";

    const { players, errors } = applyPatchesToSnapshot(base, [
      {
        kind: PATCH_KIND_PLAYER_INVENTORY,
        entityId: "player-1",
        payload: {
          slots: [
            {
              slot: 0,
              item: { type: "iron_dagger", quantity: 1, fungibilityKey: patchKey },
            },
          ],
        },
      },
    ]);

    expect(errors).toEqual([]);
    expect(players["player-1"].inventory.slots).toEqual([
      { slot: 0, item: { type: "iron_dagger", quantity: 1, fungibility_key: patchKey } },
    ]);
  });

  it("applies equipment patches for players and NPCs", () => {
    const base = buildBaselineFromSnapshot(
      deepFreeze({
        t: 5,
        players: [makePlayer()],
        npcs: [makeNPC()],
      }),
    );

    const patches = [
      {
        kind: PATCH_KIND_PLAYER_EQUIPMENT,
        entityId: "player-1",
        payload: {
          slots: [
            {
              slot: "MainHand",
              item: { type: "steel_sword", quantity: 1, fungibility_key: "steel_sword::tier2" },
            },
          ],
        },
      },
      {
        kind: PATCH_KIND_NPC_EQUIPMENT,
        entityId: "npc-1",
        payload: {
          slots: [
            {
              slot: "Body",
              item: { type: "chain_mail", quantity: 1 },
            },
          ],
        },
      },
    ];

    const { players, npcs, errors, appliedCount } = applyPatchesToSnapshot(base, patches);

    expect(errors).toEqual([]);
    expect(appliedCount).toBe(2);
    expect(players["player-1"].equipment.slots).toEqual([
      { slot: "MainHand", item: { type: "steel_sword", quantity: 1, fungibility_key: "steel_sword::tier2" } },
    ]);
    expect(npcs["npc-1"].equipment.slots).toEqual([
      { slot: "Body", item: { type: "chain_mail", quantity: 1 } },
    ]);
  });

  it("applies NPC and ground item patches onto the baseline snapshot", () => {
    const basePayload = deepFreeze({
      t: 6,
      players: [makePlayer()],
      npcs: [makeNPC()],
      groundItems: [makeGroundItem()],
    });
    const seeded = updatePatchState(createPatchState(), basePayload, { source: "join" });
    freezeState(seeded);

    const payload = deepFreeze({
      t: 7,
      players: [makePlayer()],
      npcs: [makeNPC()],
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
        { kind: PATCH_KIND_GROUND_ITEM_POS, entityId: "ground-1", payload: { x: 2, y: 3 } },
        { kind: PATCH_KIND_GROUND_ITEM_QTY, entityId: "ground-1", payload: { qty: 7 } },
      ],
    });

    const result = updatePatchState(seeded, payload, { source: "state" });

    expect(result.lastAppliedPatchCount).toBe(6);
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
    const groundItem = result.patched.groundItems["ground-1"];
    expect(groundItem).toMatchObject({ x: 2, y: 3, qty: 7 });
    expect(result.baseline.npcs["npc-1"].x).toBe(14);
    expect(result.baseline.groundItems["ground-1"].qty).toBe(7);
    expect(result.errors).toEqual([]);
    expect(result.lastUpdateSource).toBe("state");
    expect(result.lastTick).toBe(7);
    expect(result.lastSequence).toBe(7);
    expect(result.patched.sequence).toBe(7);
  });

  it("hydrates effect position and parameter patches", () => {
    const seeded = updatePatchState(
      createPatchState(),
      deepFreeze({ t: 5, players: [makePlayer()] }),
      { source: "join" },
    );
    freezeState(seeded);

    const payload = deepFreeze({
      t: 6,
      patches: [
        { kind: PATCH_KIND_EFFECT_POS, entityId: "effect-new", payload: { x: 19.5, y: 27.25 } },
        {
          kind: PATCH_KIND_EFFECT_PARAMS,
          entityId: "effect-new",
          payload: { params: { width: 40, height: 12.5, junk: "ignored" } },
        },
      ],
    });

    const result = updatePatchState(seeded, payload, { source: "state" });

    expect(result.lastAppliedPatchCount).toBe(2);
    const effect = result.patched.effects["effect-new"];
    expect(effect).toBeDefined();
    expect(effect.x).toBe(19.5);
    expect(effect.y).toBe(27.25);
    expect(effect.params).toEqual({ width: 40, height: 12.5 });
    expect(result.baseline.effects["effect-new"].params).toEqual({ width: 40, height: 12.5 });
    expect(result.errors).toEqual([]);
  });

  it("hydrates effect parameter patches when params are provided directly", () => {
    const seeded = updatePatchState(
      createPatchState(),
      deepFreeze({ t: 8, players: [makePlayer()] }),
      { source: "join" },
    );
    freezeState(seeded);

    const payload = deepFreeze({
      t: 9,
      patches: [
        {
          kind: PATCH_KIND_EFFECT_PARAMS,
          entityId: "effect-direct",
          payload: { width: "5.25", height: 11, junk: "ignored" },
        },
      ],
    });

    const result = updatePatchState(seeded, payload, { source: "state" });

    expect(result.lastAppliedPatchCount).toBe(1);
    const effect = result.patched.effects["effect-direct"];
    expect(effect).toBeDefined();
    expect(effect.x).toBe(0);
    expect(effect.y).toBe(0);
    expect(effect.params).toEqual({ width: 5.25, height: 11 });
    expect(result.baseline.effects["effect-direct"].params).toEqual({ width: 5.25, height: 11 });
    expect(result.errors).toEqual([]);
  });

  it("clamps regressing patch sequences for incremental effect updates", () => {
    const previousTick = 50;
    const seeded = updatePatchState(
      createPatchState(),
      deepFreeze({ t: previousTick, sequence: 218, players: [makePlayer()] }),
      { source: "join" },
    );
    freezeState(seeded);

    const payload = deepFreeze({
      t: previousTick + 1,
      sequence: 209,
      patches: [
        {
          kind: PATCH_KIND_EFFECT_PARAMS,
          entityId: "effect-regressed",
          payload: { params: { width: 7, height: 9 } },
        },
      ],
    });

    const result = updatePatchState(seeded, payload, { source: "state" });

    expect(result.lastAppliedPatchCount).toBe(1);
    expect(result.lastSequence).toBe(218);
    expect(result.lastTick).toBeGreaterThanOrEqual(previousTick);
    const effect = result.patched.effects["effect-regressed"];
    expect(effect).toBeDefined();
    expect(effect.params).toEqual({ width: 7, height: 9 });
    expect(Array.isArray(result.errors)).toBe(true);
  });

  it("normalizes entity identifiers when seeding baseline state", () => {
    const payload = deepFreeze({
      t: 11,
      players: [makePlayer({ id: "  player-1  " })],
      npcs: [makeNPC({ id: "\tnpc-2\n" })],
      groundItems: [makeGroundItem({ id: "\nground-4\t" })],
    });

    const baseline = buildBaselineFromSnapshot(payload);

    expect(Object.keys(baseline.players)).toEqual(["player-1"]);
    expect(Object.keys(baseline.npcs)).toEqual(["npc-2"]);
    expect(Object.keys(baseline.effects)).toEqual([]);
    expect(Object.keys(baseline.groundItems)).toEqual(["ground-4"]);
    expect(baseline.players["player-1"].id).toBe("player-1");
    expect(baseline.npcs["npc-2"].id).toBe("npc-2");
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
      groundItems: {
        " ground-4 ": makeGroundItem({ id: " ground-4 " }),
      },
      effects: {
        " effect-7 ": makeEffect({ id: " effect-7 ", x: 12.5, y: 9.25 }),
      },
    };

    const { players, npcs, groundItems, effects } = applyPatchesToSnapshot(base, []);

    expect(Object.keys(players)).toEqual(["player-1"]);
    expect(Object.keys(npcs)).toEqual(["npc-2"]);
    expect(Object.keys(groundItems)).toEqual(["ground-4"]);
    expect(Object.keys(effects)).toEqual(["effect-7"]);
    expect(players["player-1"].id).toBe("player-1");
    expect(npcs["npc-2"].id).toBe("npc-2");
    expect(groundItems["ground-4"].id).toBe("ground-4");
    expect(effects["effect-7"].id).toBe("effect-7");
    expect(effects["effect-7"].x).toBe(12.5);
    expect(effects["effect-7"].y).toBe(9.25);
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

  it("requests keyframes for unknown entities and replays once the snapshot arrives", () => {
    const requestLog = [];
    const seeded = updatePatchState(
      createPatchState(),
      deepFreeze({ t: 10, sequence: 10, players: [makePlayer()] }),
      { source: "join" },
    );
    freezeState(seeded);

    const missingNPCPatch = deepFreeze({
      sequence: 11,
      keyframeSeq: 11,
      patches: [
        {
          kind: PATCH_KIND_NPC_HEALTH,
          entityId: "npc-99",
          payload: { health: 7, maxHealth: 14 },
        },
      ],
    });

    const deferred = updatePatchState(seeded, missingNPCPatch, {
      source: "state",
      requestKeyframe: (sequence) => requestLog.push(sequence),
      now: 100,
    });
    freezeState(deferred);

    expect(requestLog).toEqual([11]);
    expect(deferred.lastAppliedPatchCount).toBe(0);
    expect(deferred.pendingKeyframeRequests instanceof Map).toBe(true);
    const pendingMeta =
      deferred.pendingKeyframeRequests instanceof Map
        ? deferred.pendingKeyframeRequests.get(11)
        : null;
    expect(pendingMeta && typeof pendingMeta.firstRequestedAt === "number").toBe(true);
    expect(pendingMeta?.attempts).toBe(1);
    expect(pendingMeta?.nextRetryAt).toBe(null);
    expect(Array.isArray(deferred.pendingReplays) ? deferred.pendingReplays.length : 0).toBe(1);
    expect(Object.keys(deferred.patched.npcs)).toEqual([]);
    expect(deferred.deferredPatchCount).toBe(1);
    expect(deferred.totalDeferredPatchCount).toBe(1);
    expect(deferred.lastDeferredReplayLatencyMs).toBeNull();

    const keyframePayload = deepFreeze({
      type: "keyframe",
      sequence: 11,
      t: 11,
      players: [makePlayer()],
      npcs: [makeNPC({ id: "npc-99", health: 9, maxHealth: 14 })],
      groundItems: [],
    });

    const replayed = updatePatchState(deferred, keyframePayload, {
      source: "keyframe",
      requestKeyframe: (sequence) => requestLog.push(sequence),
      now: 160,
    });

    expect(requestLog).toEqual([11]);
    expect(replayed.lastAppliedPatchCount).toBe(1);
    expect(
      replayed.pendingKeyframeRequests instanceof Map ? replayed.pendingKeyframeRequests.size : 0,
    ).toBe(0);
    expect(Array.isArray(replayed.pendingReplays) ? replayed.pendingReplays.length : 0).toBe(0);
    expect(replayed.baseline.npcs["npc-99"].health).toBe(7);
    expect(replayed.patched.npcs["npc-99"].health).toBe(7);
    expect(replayed.patched.npcs["npc-99"].maxHealth).toBe(14);
    expect(replayed.lastRecovery && replayed.lastRecovery.status).toBe("recovered");
    expect(replayed.errors).toEqual([]);
    expect(replayed.deferredPatchCount).toBe(0);
    expect(replayed.totalDeferredPatchCount).toBe(1);
    expect(replayed.lastDeferredReplayLatencyMs).toBe(60);
  });

  it("keeps tick and sequence monotonic when replaying from a fallback snapshot", () => {
    const seeded = updatePatchState(
      createPatchState(),
      deepFreeze({ t: 20, sequence: 20, players: [makePlayer({ x: 5, y: 5 })] }),
      { source: "join" },
    );

    const mutated = {
      ...seeded,
      baseline: {
        ...seeded.baseline,
        tick: 18,
        sequence: 18,
        players: { ...seeded.baseline.players },
        npcs: { ...seeded.baseline.npcs },
        groundItems: { ...seeded.baseline.groundItems },
        effects: { ...seeded.baseline.effects },
      },
      patched: {
        ...seeded.patched,
        tick: 19,
        sequence: 19,
        players: { ...seeded.patched.players },
        npcs: { ...seeded.patched.npcs },
        groundItems: { ...seeded.patched.groundItems },
        effects: { ...seeded.patched.effects },
      },
      lastTick: 20,
      lastSequence: 20,
    };

    const payload = deepFreeze({
      sequence: 21,
      keyframeSeq: 21,
      patches: [
        {
          kind: PATCH_KIND_PLAYER_POS,
          entityId: "player-1",
          payload: { x: 33, y: 44 },
        },
      ],
    });

    const next = updatePatchState(mutated, payload, {
      source: "state",
      requestKeyframe: () => {},
      now: 900,
    });

    expect(next.patched.players["player-1"].x).toBe(33);
    expect(next.patched.players["player-1"].y).toBe(44);
    expect(next.lastTick).toBe(20);
    expect(next.lastSequence).toBe(20);
    expect(next.deferredPatchCount).toBe(0);
    expect(next.totalDeferredPatchCount).toBe(0);
  });

  it("regresses to the cached keyframe when cadence skips snapshots", () => {
    const seeded = freezeState(
      updatePatchState(
        createPatchState(),
        deepFreeze({
          t: 90,
          sequence: 90,
          players: [makePlayer({ x: 10, y: 10, facing: "right" })],
        }),
        { source: "join" },
      ),
    );

    const forwardStep = freezeState(
      updatePatchState(
        seeded,
        deepFreeze({
          t: 91,
          sequence: 91,
          keyframeSeq: 90,
          patches: [
            {
              kind: PATCH_KIND_PLAYER_POS,
              entityId: "player-1",
              payload: { x: 12, y: 10 },
            },
          ],
        }),
        { source: "state" },
      ),
    );

    expect(forwardStep.patched.players["player-1"].x).toBe(12);

    const diagonal = freezeState(
      updatePatchState(
        forwardStep,
        deepFreeze({
          t: 92,
          sequence: 92,
          keyframeSeq: 90,
          patches: [
            {
              kind: PATCH_KIND_PLAYER_POS,
              entityId: "player-1",
              payload: { x: 14, y: 12 },
            },
          ],
        }),
        { source: "state" },
      ),
    );

    expect(diagonal.patched.players["player-1"].x).toBe(14);
    expect(diagonal.patched.players["player-1"].y).toBe(12);

    const facingPatch = deepFreeze({
      t: 93,
      sequence: 93,
      keyframeSeq: 90,
      patches: [
        {
          kind: PATCH_KIND_PLAYER_FACING,
          entityId: "player-1",
          payload: { facing: "up" },
        },
      ],
    });

    const regressed = updatePatchState(diagonal, facingPatch, { source: "state" });

    expect(regressed.patched.players["player-1"].facing).toBe("up");
    expect(regressed.patched.players["player-1"].x).toBe(14);
    expect(regressed.patched.players["player-1"].y).toBe(12);

    const recovery = updatePatchState(
      regressed,
      deepFreeze({
        t: 94,
        sequence: 94,
        keyframeSeq: 90,
        patches: [
          {
            kind: PATCH_KIND_PLAYER_POS,
            entityId: "player-1",
            payload: { x: 16, y: 14 },
          },
        ],
      }),
      { source: "state" },
    );

    expect(recovery.patched.players["player-1"].x).toBe(16);
    expect(recovery.patched.players["player-1"].y).toBe(14);
  });

  it("applies patches against the latest view while waiting for a keyframe", () => {
    const requests = [];
    let baseline = updatePatchState(
      createPatchState(),
      deepFreeze({ t: 12, sequence: 12, players: [makePlayer({ x: 4, y: 6 })] }),
      { source: "state" },
    );
    if (baseline?.keyframes && baseline.keyframes.map instanceof Map) {
      baseline = {
        ...baseline,
        keyframes: { ...baseline.keyframes, map: new Map() },
      };
    }

    const patchPayload = deepFreeze({
      t: 13,
      sequence: 13,
      keyframeSeq: 12,
      patches: [
        {
          kind: PATCH_KIND_PLAYER_POS,
          entityId: "player-1",
          payload: { x: 9, y: 11 },
        },
      ],
    });

    const next = updatePatchState(baseline, patchPayload, {
      source: "state",
      now: 1000,
      requestKeyframe: (sequence, tick) => {
        requests.push({ sequence, tick });
      },
    });

    expect(next.patched.players["player-1"].x).toBe(9);
    expect(next.patched.players["player-1"].y).toBe(11);
    expect(requests).toEqual([{ sequence: 12, tick: 13 }]);
    const pending =
      next.pendingKeyframeRequests instanceof Map
        ? next.pendingKeyframeRequests.get(12)
        : null;
    expect(pending?.attempts).toBe(1);
    expect(Array.isArray(next.pendingReplays) ? next.pendingReplays.length : 0).toBe(1);
    expect(next.deferredPatchCount).toBe(0);
    expect(next.totalDeferredPatchCount).toBe(0);
    expect(next.lastTick).toBe(12);
    expect(next.lastSequence).toBe(12);
  });

  it("ignores duplicate keyframes after applying the deferred replay", () => {
    const seeded = freezeState(
      updatePatchState(createPatchState(), deepFreeze({ t: 4, sequence: 4, players: [makePlayer()] }), {
        source: "join",
      }),
    );

    const missingPatch = deepFreeze({
      sequence: 5,
      keyframeSeq: 5,
      patches: [
        {
          kind: PATCH_KIND_PLAYER_HEALTH,
          entityId: "player-1",
          payload: { health: 4, maxHealth: 10 },
        },
      ],
    });

    const deferred = freezeState(
      updatePatchState(seeded, missingPatch, {
        source: "state",
        requestKeyframe: () => {},
      }),
    );

    const keyframePayload = deepFreeze({
      type: "keyframe",
      sequence: 5,
      t: 5,
      players: [makePlayer({ id: "player-1", health: 7, maxHealth: 10 })],
      npcs: [],
      groundItems: [],
    });

    const replayed = freezeState(
      updatePatchState(deferred, keyframePayload, { source: "keyframe", requestKeyframe: () => {} }),
    );
    const duplicate = updatePatchState(replayed, keyframePayload, {
      source: "keyframe",
      requestKeyframe: () => {},
    });

    expect(
      duplicate.pendingKeyframeRequests instanceof Map ? duplicate.pendingKeyframeRequests.size : 0,
    ).toBe(0);
    expect(Array.isArray(duplicate.pendingReplays) ? duplicate.pendingReplays.length : 0).toBe(0);
    expect(duplicate.lastAppliedPatchCount).toBe(replayed.lastAppliedPatchCount);
    expect(Array.isArray(duplicate.recoveryLog) ? duplicate.recoveryLog.length : 0).toBe(
      Array.isArray(replayed.recoveryLog) ? replayed.recoveryLog.length : 0,
    );
    expect(
      duplicate.resolvedKeyframeSequences instanceof Set
        ? duplicate.resolvedKeyframeSequences.has(5)
        : false,
    ).toBe(true);
  });

  it("safely ignores patch payloads without snapshot data when baseline is empty", () => {
    const state = createPatchState();
    const payload = deepFreeze({ sequence: 2, keyframeSeq: 2, patches: [] });
    const result = updatePatchState(state, payload, { source: "state" });

    expect(result).toBeTruthy();
    expect(result.lastAppliedPatchCount).toBe(0);
    expect(result.errors).toEqual([]);
    expect(result.baseline && typeof result.baseline).toBe("object");
    expect(Object.keys(result.baseline?.players || {})).toEqual([]);
  });

  it("escalates to resync when a keyframe nack reports expiry", () => {
    const seeded = freezeState(
      updatePatchState(
        createPatchState(),
        deepFreeze({ t: 2, sequence: 2, players: [makePlayer()] }),
        { source: "join" },
      ),
    );

    const pending = freezeState(
      updatePatchState(
        seeded,
        deepFreeze({
          sequence: 6,
          keyframeSeq: 6,
          patches: [
            {
              kind: PATCH_KIND_NPC_POS,
              entityId: "npc-missing",
              payload: { x: 3, y: 4 },
            },
          ],
        }),
        { source: "state", requestKeyframe: () => {} },
      ),
    );

    const nack = freezeState(
      updatePatchState(
        pending,
        deepFreeze({ type: "keyframeNack", sequence: 6, reason: "expired" }),
        { source: "keyframe" },
      ),
    );

    expect(nack.resyncRequested).toBe(true);
    expect(nack.keyframeNackCounts?.expired).toBe(1);
    expect(
      nack.pendingKeyframeRequests instanceof Map ? nack.pendingKeyframeRequests.size : 0,
    ).toBe(0);
    expect(Array.isArray(nack.pendingReplays) ? nack.pendingReplays.length : 0).toBe(0);
    expect(nack.lastRecovery?.status).toBe("expired");
  });

  it("tracks rate-limited nacks without clearing pending replays", () => {
    const seeded = freezeState(
      updatePatchState(
        createPatchState(),
        deepFreeze({ t: 3, sequence: 3, players: [makePlayer()] }),
        { source: "join" },
      ),
    );

    const pending = freezeState(
      updatePatchState(
        seeded,
        deepFreeze({
          sequence: 8,
          keyframeSeq: 8,
          patches: [
            {
              kind: PATCH_KIND_PLAYER_HEALTH,
              entityId: "player-1",
              payload: { health: 9, maxHealth: 10 },
            },
          ],
        }),
        { source: "state", requestKeyframe: () => {} },
      ),
    );

    const nack = freezeState(
      updatePatchState(
        pending,
        deepFreeze({ type: "keyframeNack", sequence: 8, reason: "rate_limited" }),
        { source: "keyframe" },
      ),
    );

    expect(nack.resyncRequested).toBe(false);
    expect(nack.keyframeNackCounts?.rate_limited).toBe(1);
    expect(nack.pendingKeyframeRequests instanceof Map).toBe(true);
    const pendingMeta =
      nack.pendingKeyframeRequests instanceof Map
        ? nack.pendingKeyframeRequests.get(8)
        : null;
    expect(pendingMeta).not.toBeNull();
    expect(pendingMeta?.attempts).toBe(1);
    expect(typeof pendingMeta?.nextRetryAt).toBe("number");
    expect(pendingMeta && pendingMeta.nextRetryAt > pendingMeta.firstRequestedAt).toBe(true);
    expect(Array.isArray(nack.pendingReplays) ? nack.pendingReplays.length : 0).toBe(1);
    expect(nack.pendingReplays[0].sequence).toBe(8);
    expect(nack.lastRecovery?.status).toBe("rate_limited");
  });

  it("escalates to resync when rate-limited retries exceed the cap", () => {
    const seeded = freezeState(
      updatePatchState(
        createPatchState(),
        deepFreeze({ t: 3, sequence: 3, players: [makePlayer()] }),
        { source: "join" },
      ),
    );

    const pending = freezeState(
      updatePatchState(
        seeded,
        deepFreeze({
          sequence: 8,
          keyframeSeq: 8,
          patches: [
            {
              kind: PATCH_KIND_PLAYER_HEALTH,
              entityId: "player-1",
              payload: { health: 9, maxHealth: 10 },
            },
          ],
        }),
        { source: "state", requestKeyframe: () => {} },
      ),
    );

    const nackPayload = deepFreeze({ type: "keyframeNack", sequence: 8, reason: "rate_limited" });
    const nack1 = freezeState(updatePatchState(pending, nackPayload, { source: "keyframe" }));
    const retriedOnce = freezeState(withRetryAttempts(nack1, 8, 2));
    const nack2 = freezeState(updatePatchState(retriedOnce, nackPayload, { source: "keyframe" }));
    const retriedTwice = freezeState(withRetryAttempts(nack2, 8, 3));
    const nack3 = updatePatchState(retriedTwice, nackPayload, { source: "keyframe" });

    expect(nack3.resyncRequested).toBe(true);
    expect(nack3.keyframeNackCounts?.rate_limited).toBe(3);
    expect(Array.isArray(nack3.pendingReplays) ? nack3.pendingReplays.length : 0).toBe(0);
    expect(nack3.pendingKeyframeRequests instanceof Map ? nack3.pendingKeyframeRequests.size : 0).toBe(0);
  });

  it("maintains forward motion between sparse keyframes", () => {
    const keyframePayload = deepFreeze({
      type: "keyframe",
      keyframeSeq: 10,
      sequence: 10,
      t: 10,
      players: [makePlayer({ x: 0, y: 0, facing: "right" })],
    });
    const seeded = updatePatchState(createPatchState(), keyframePayload, { source: "broadcast" });

    const movePayload = deepFreeze({
      type: "patch",
      keyframeSeq: 10,
      sequence: 11,
      patches: [
        {
          kind: PATCH_KIND_PLAYER_POS,
          entityId: "player-1",
          payload: { x: 2, y: 3 },
        },
      ],
    });
    const afterMove = updatePatchState(seeded, movePayload, { source: "broadcast" });

    expect(afterMove.baseline.players["player-1"]).toMatchObject({ x: 2, y: 3 });
    expect(afterMove.patched.players["player-1"]).toMatchObject({ x: 2, y: 3 });
    expect(afterMove.baseline.sequence).toBe(11);
    expect(afterMove.patched.sequence).toBe(11);

    const facingPayload = deepFreeze({
      type: "patch",
      keyframeSeq: 10,
      sequence: 12,
      patches: [
        {
          kind: PATCH_KIND_PLAYER_FACING,
          entityId: "player-1",
          payload: { facing: "down" },
        },
      ],
    });
    const afterFacing = updatePatchState(afterMove, facingPayload, { source: "broadcast" });

    expect(afterFacing.baseline.players["player-1"]).toMatchObject({
      x: 2,
      y: 3,
      facing: "down",
    });
    expect(afterFacing.patched.players["player-1"]).toMatchObject({
      x: 2,
      y: 3,
      facing: "down",
    });
    expect(afterFacing.baseline.sequence).toBe(12);
    expect(afterFacing.patched.sequence).toBe(12);
  });
});
