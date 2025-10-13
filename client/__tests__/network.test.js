import { describe, expect, it, vi } from "vitest";
import { computeRtt } from "../heartbeat.js";
import {
  createPatchState,
  updatePatchState,
  PATCH_KIND_PLAYER_POS,
} from "../patches.js";
import {
  buildActionPayload,
  buildCancelPathPayload,
  buildConsolePayload,
  buildHeartbeatPayload,
  buildInputPayload,
  buildPathPayload,
  handleProtocolVersion,
  DEFAULT_WORLD_HEIGHT,
  DEFAULT_WORLD_SEED,
  DEFAULT_WORLD_WIDTH,
  applyStateSnapshot,
  clampToWorld,
  deriveDisplayMaps,
  enqueueEffectTriggers,
  getWorldDims,
  PROTOCOL_VERSION,
  readProtocolVersion,
  sendMessage,
  normalizeCount,
  normalizeGroundItems,
  normalizeWorldConfig,
  parseServerEvent,
  splitNpcCounts,
  __networkInternals,
} from "../network.js";

const DEFAULT_COUNTS = {
  goblinCount: 2,
  ratCount: 1,
  npcCount: 3,
};

function deepFreeze(value) {
  if (value && typeof value === "object") {
    Object.freeze(value);
    for (const key of Object.keys(value)) {
      deepFreeze(value[key]);
    }
  }
  return value;
}

function makePlayer(overrides = {}) {
  return {
    id: "player-1",
    x: 1,
    y: 2,
    facing: "down",
    health: 10,
    maxHealth: 10,
    ...overrides,
  };
}

describe("splitNpcCounts", () => {
  it.each([
    { total: 0, expected: { goblinCount: 0, ratCount: 0, npcCount: 0 } },
    { total: 1, expected: { goblinCount: 1, ratCount: 0, npcCount: 1 } },
    { total: 2, expected: { goblinCount: 2, ratCount: 0, npcCount: 2 } },
    { total: 3, expected: { goblinCount: 2, ratCount: 1, npcCount: 3 } },
    { total: 10, expected: { goblinCount: 2, ratCount: 8, npcCount: 10 } },
  ])(
    "splits totals across goblins and rats when only npcCount = $total",
    ({ total, expected }) => {
      const result = splitNpcCounts({ npcCount: total }, DEFAULT_COUNTS);
      expect(result).toEqual(expected);
      expect(result.npcCount).toBe(result.goblinCount + result.ratCount);
    },
  );

  it.each([
    {
      label: "only goblin count provided",
      input: { goblinCount: 5 },
      expected: { goblinCount: 5, ratCount: 1, npcCount: 6 },
    },
    {
      label: "only rat count provided",
      input: { ratCount: 4 },
      expected: { goblinCount: 2, ratCount: 4, npcCount: 6 },
    },
    {
      label: "both goblin and rat provided",
      input: { goblinCount: 3, ratCount: 7 },
      expected: { goblinCount: 3, ratCount: 7, npcCount: 10 },
    },
  ])("respects explicit per-type counts when $label", ({ input, expected }) => {
    const result = splitNpcCounts(input, DEFAULT_COUNTS);
    expect(result).toEqual(expected);
    expect(result.npcCount).toBe(result.goblinCount + result.ratCount);
  });

  it.each([
    {
      label: "negative goblin count",
      input: { goblinCount: -5 },
      expected: { goblinCount: 0, ratCount: 1, npcCount: 1 },
    },
    {
      label: "float goblin count",
      input: { goblinCount: 2.9 },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
    {
      label: "string rat count",
      input: { ratCount: "4" },
      expected: { goblinCount: 2, ratCount: 4, npcCount: 6 },
    },
    {
      label: "null npc count",
      input: { npcCount: null },
      expected: { goblinCount: 0, ratCount: 0, npcCount: 0 },
    },
    {
      label: "undefined npc count",
      input: { npcCount: undefined },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
    {
      label: "NaN npc count",
      input: { npcCount: Number.NaN },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
    {
      label: "float npc count",
      input: { npcCount: 3.7 },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
  ])("normalizes $label", ({ input, expected }) => {
    const result = splitNpcCounts(input, DEFAULT_COUNTS);
    expect(result).toEqual(expected);
    expect(result.npcCount).toBe(result.goblinCount + result.ratCount);
  });
});

describe("getWorldDims", () => {
  it("prefers explicit WORLD dimensions when valid", () => {
    const dims = getWorldDims({
      WORLD_WIDTH: "1024",
      WORLD_HEIGHT: 768,
      canvas: { width: 800, height: 600 },
      GRID_WIDTH: 30,
      GRID_HEIGHT: 20,
      TILE_SIZE: 32,
    });

    expect(dims).toEqual({ width: 1024, height: 768 });
  });

  it("falls back to canvas size when WORLD dimensions are invalid", () => {
    const dims = getWorldDims({
      WORLD_WIDTH: Infinity,
      WORLD_HEIGHT: -1,
      canvas: { width: 640, height: 480 },
    });

    expect(dims).toEqual({ width: 640, height: 480 });
  });

  it("derives dimensions from grid metrics when canvas is missing", () => {
    const dims = getWorldDims({
      GRID_WIDTH: 25,
      GRID_HEIGHT: 15,
      TILE_SIZE: 32,
    });

    expect(dims).toEqual({ width: 800, height: 480 });
  });

  it("returns defaults when no inputs are usable", () => {
    const dims = getWorldDims({});
    expect(dims).toEqual({
      width: DEFAULT_WORLD_WIDTH,
      height: DEFAULT_WORLD_HEIGHT,
    });
  });

  it("handles non-object inputs by returning defaults", () => {
    expect(getWorldDims(null)).toEqual({
      width: DEFAULT_WORLD_WIDTH,
      height: DEFAULT_WORLD_HEIGHT,
    });
  });
});

describe("clampToWorld", () => {
  it("clamps coordinates within world bounds", () => {
    const result = clampToWorld(-50, 999, { width: 500, height: 400 }, 20);
    expect(result).toEqual({ x: 20, y: 380 });
  });

  it("collapses to edges when the map is smaller than the player", () => {
    const result = clampToWorld(10, -15, { width: 40, height: 30 }, 30);
    expect(result).toEqual({ x: 30, y: 30 });
  });

  it("treats invalid dimensions and player sizes as zero", () => {
    const result = clampToWorld(Number.NaN, Infinity, { width: Infinity }, NaN);
    expect(result).toEqual({ x: 0, y: 0 });
  });

  it("respects zero-sized worlds with large player halves", () => {
    const result = clampToWorld(200, -50, { width: 0, height: 0 }, 25);
    expect(result).toEqual({ x: 25, y: 25 });
  });
});

describe("normalizeWorldConfig", () => {
  it("respects boolean toggles and coerces the seed", () => {
    const result = normalizeWorldConfig({
      obstacles: false,
      obstaclesCount: "3.8",
      goldMines: false,
      goldMineCount: "5",
      npcs: false,
      goblinCount: "4",
      ratCount: "1",
      lava: false,
      lavaCount: "7",
      seed: 42,
    });

    expect(result.obstacles).toBe(false);
    expect(result.goldMines).toBe(false);
    expect(result.npcs).toBe(false);
    expect(result.lava).toBe(false);
    expect(result.seed).toBe("42");
    expect(result.goblinCount + result.ratCount).toBe(result.npcCount);
  });

  it("trims string seeds and falls back when empty", () => {
    const result = normalizeWorldConfig({ seed: "  custom-seed  " });
    expect(result.seed).toBe("custom-seed");

    const fallback = normalizeWorldConfig({ seed: "   " });
    expect(fallback.seed).toBe(DEFAULT_WORLD_SEED);
  });

  it.each([
    { width: 1200, height: 800 },
    { width: "3600", height: "2400" },
  ])("accepts positive finite width/height values", ({ width, height }) => {
    const result = normalizeWorldConfig({ width, height });
    expect(result.width).toBe(Number(width));
    expect(result.height).toBe(Number(height));
  });

  it.each([
    { width: 0, height: 0 },
    { width: -1, height: -2 },
    { width: "invalid", height: "bad" },
    { width: Infinity, height: -Infinity },
  ])("falls back to defaults for invalid dimensions", ({ width, height }) => {
    const result = normalizeWorldConfig({ width, height });
    expect(result.width).toBe(DEFAULT_WORLD_WIDTH);
    expect(result.height).toBe(DEFAULT_WORLD_HEIGHT);
  });

  it("normalizes npc totals even with partial inputs", () => {
    const result = normalizeWorldConfig({ npcCount: 1, ratCount: 4 });
    expect(result.goblinCount + result.ratCount).toBe(result.npcCount);
    expect(result.ratCount).toBe(4);
  });
});

describe("normalizeCount", () => {
  it.each([
    { value: 5, fallback: 2, expected: 5 },
    { value: "3.9", fallback: 2, expected: 3 },
    { value: -2, fallback: 4, expected: 0 },
    { value: Number.NaN, fallback: 7, expected: 7 },
  ])("normalizes $value with fallback $fallback", ({ value, fallback, expected }) => {
    expect(normalizeCount(value, fallback)).toBe(expected);
  });
});

describe("normalizeGroundItems", () => {
  it("builds an object of sanitized entries", () => {
    const result = normalizeGroundItems([
      { id: "ore-1", x: "12", y: 7.5, qty: "3" },
      { id: "ore-2", x: null, y: undefined, qty: Infinity },
      { id: 0, x: 4, y: 5, qty: 1 },
      null,
    ]);

    expect(result).toEqual({
      "ore-1": { id: "ore-1", type: "gold", x: 12, y: 7.5, qty: 3 },
      "ore-2": { id: "ore-2", type: "gold", x: 0, y: 0, qty: 0 },
    });
  });

  it("returns an empty object when input is not an array", () => {
    expect(normalizeGroundItems(undefined)).toEqual({});
  });
});

describe("applyStateSnapshot", () => {
  it("normalizes entity facings and optional arrays", () => {
    const payload = {
      players: [
        { id: "local", x: 10, y: 20, facing: "invalid" },
        { id: "ally", x: 30, y: 40, facing: "left" },
        { id: null, x: 0, y: 0, facing: "right" },
      ],
      npcs: [
        { id: "npc-1", x: 50, y: 60, facing: "bad" },
        { id: "npc-2", x: 70, y: 80, facing: "up" },
      ],
      obstacles: [{ id: "rock" }],
      effects: [{ id: "beam" }],
      config: { width: 1200, height: 900 },
    };

    const result = applyStateSnapshot({ playerId: "local" }, payload, null);

    expect(result.players.local.facing).toBe("down");
    expect(result.players.ally.facing).toBe("left");
    expect(result.npcs["npc-1"].facing).toBe("down");
    expect(result.npcs["npc-2"].facing).toBe("up");
    expect(result.obstacles).toEqual(payload.obstacles);
    expect(result.obstacles).not.toBe(payload.obstacles);
    expect(result.effects).toEqual(payload.effects);
    expect(result.effects).not.toBe(payload.effects);
    expect(result.worldConfig.width).toBe(1200);
    expect(result.worldConfig.height).toBe(900);
    expect(result.hasLocalPlayer).toBe(true);
    expect(result.currentFacing).toBe("down");
    expect(result.players).not.toBe(payload.players);
    expect(result.npcs).not.toBe(payload.npcs);
  });

  it("defaults to empty collections when snapshot arrays are invalid", () => {
    const result = applyStateSnapshot({ playerId: "player-1" }, {
      players: null,
      npcs: undefined,
      obstacles: "invalid",
      effects: 42,
    }, null);

    expect(result.players).toEqual({});
    expect(result.npcs).toEqual({});
    expect(result.obstacles).toEqual([]);
    expect(result.effects).toEqual([]);
    expect(result.hasLocalPlayer).toBe(false);
    expect(result.currentFacing).toBeUndefined();
    expect(result.worldConfig).toBeUndefined();
  });

  it("surfaces missing local player without throwing", () => {
    const payload = { players: [{ id: "other", facing: "up" }] };
    const result = applyStateSnapshot({ playerId: "ghost" }, payload, null);

    expect(result.hasLocalPlayer).toBe(false);
    expect(result.currentFacing).toBeUndefined();
  });

  it("applies tick from state", () => {
    const result = applyStateSnapshot({ playerId: "local", lastTick: 7 }, {
      players: [],
      t: 42,
    }, null);

    expect(result.lastTick).toBe(42);
  });

  it("falls back to patched state when snapshot omits arrays", () => {
    const patched = {
      players: {
        alpha: { id: "alpha", x: 1, y: 2, facing: "left" },
      },
      npcs: {
        goblin: { id: "goblin", x: 3, y: 4, facing: "up" },
      },
      effects: {
        spark: { id: "spark", ttl: 5 },
      },
      groundItems: {
        stack: { id: "stack", x: 10, y: 20, qty: 3 },
      },
      tick: 99,
    };

    const previous = {
      playerId: "alpha",
      worldConfig: { width: 800, height: 600 },
    };

    const result = applyStateSnapshot(previous, { t: null }, patched);

    expect(result.players.alpha.facing).toBe("left");
    expect(result.npcs.goblin.facing).toBe("up");
    expect(result.effects).toEqual([{ id: "spark", ttl: 5 }]);
    expect(result.groundItems.stack.qty).toBe(3);
    expect(result.lastTick).toBe(99);
    expect(result.worldConfig.width).toBe(800);
    expect(result.hasLocalPlayer).toBe(true);
  });
});

describe("join and resync integration", () => {
  it("resets patch history and seeds baseline from join snapshots", () => {
    const seededState = updatePatchState(
      createPatchState(),
      deepFreeze({
        type: "state",
        t: 5,
        sequence: 5,
        keyframeSeq: 5,
        players: [makePlayer({ x: 1, y: 2 })],
        npcs: [],
        effects: [],
        groundItems: [],
      }),
      { source: "state" },
    );

    const liveState = updatePatchState(
      seededState,
      deepFreeze({
        type: "state",
        sequence: 6,
        patches: [
          {
            kind: PATCH_KIND_PLAYER_POS,
            entityId: "player-1",
            payload: { x: 7, y: 8 },
          },
        ],
      }),
      { source: "state" },
    );

    expect(liveState.patchHistory.map.size).toBeGreaterThan(0);

    const joinPayload = deepFreeze({
      type: "state",
      t: 12,
      sequence: 12,
      keyframeSeq: 12,
      players: [makePlayer({ x: 4, y: 6 })],
      npcs: [],
      effects: [],
      groundItems: [],
    });

    const joinState = updatePatchState(liveState, joinPayload, {
      source: "join",
    });

    const pendingReplayCount = Array.isArray(joinState.pendingReplays)
      ? joinState.pendingReplays.length
      : 0;
    expect(joinState.lastAppliedPatchCount).toBe(0);
    expect(joinState.patchHistory.map.size).toBe(0);
    expect(pendingReplayCount).toBe(1);

    const replayEntry = Array.isArray(joinState.pendingReplays)
      ? joinState.pendingReplays[0]
      : null;
    expect(replayEntry).not.toBeNull();
    expect(replayEntry?.deferredCount).toBe(0);

    const snapshot = applyStateSnapshot({ playerId: "player-1" }, joinPayload, joinState.patched);

    expect(snapshot.players["player-1"].x).toBe(4);
    expect(snapshot.players["player-1"].y).toBe(6);
    expect(snapshot.lastTick).toBe(12);
    expect(snapshot.hasLocalPlayer).toBe(true);
  });

  it("replays resync payloads in two passes so patches backfill the snapshot", () => {
    const seededState = updatePatchState(
      createPatchState(),
      deepFreeze({
        type: "state",
        t: 8,
        sequence: 8,
        keyframeSeq: 8,
        players: [makePlayer({ x: 2, y: 3 })],
        npcs: [],
        effects: [],
        groundItems: [],
      }),
      { source: "join" },
    );

    const resyncSnapshot = deepFreeze({
      type: "state",
      resync: true,
      t: 20,
      sequence: 20,
      keyframeSeq: 20,
      players: [makePlayer({ x: 5, y: 7 })],
      npcs: [],
      effects: [],
      groundItems: [],
    });

    const requestKeyframe = vi.fn();
    const snapshotState = updatePatchState(seededState, resyncSnapshot, {
      source: "state",
      resetHistory: true,
      requestKeyframe,
      now: 1_000,
    });

    expect(requestKeyframe).not.toHaveBeenCalled();
    expect(snapshotState.lastAppliedPatchCount).toBe(0);
    const snapshotView = applyStateSnapshot(
      { playerId: "player-1", lastTick: seededState.lastTick },
      resyncSnapshot,
      snapshotState.patched,
    );
    expect(snapshotView.players["player-1"].x).toBe(5);
    expect(snapshotView.players["player-1"].y).toBe(7);
    expect(snapshotView.lastTick).toBe(20);

    const catchupPatch = deepFreeze({
      type: "state",
      t: 21,
      sequence: 21,
      patches: [
        {
          kind: PATCH_KIND_PLAYER_POS,
          entityId: "player-1",
          payload: { x: 14, y: 15 },
        },
      ],
    });

    const replayedState = updatePatchState(snapshotState, catchupPatch, {
      source: "state",
    });

    expect(replayedState.lastAppliedPatchCount).toBe(1);
    expect(replayedState.patched.players["player-1"].x).toBe(14);
    expect(replayedState.patched.players["player-1"].y).toBe(15);
    const catchupView = applyStateSnapshot(
      { playerId: "player-1", lastTick: snapshotView.lastTick },
      catchupPatch,
      replayedState.patched,
    );

    expect(catchupView.players["player-1"].x).toBe(14);
    expect(catchupView.players["player-1"].y).toBe(15);
    expect(catchupView.lastTick).toBe(21);
    expect(catchupView.hasLocalPlayer).toBe(true);
  });
});

describe("deriveDisplayMaps", () => {
  it("maps positions and prunes removed entities", () => {
    const players = {
      alpha: { id: "alpha", x: 1, y: 2 },
      beta: { id: "beta", x: 3, y: 4 },
      invalid: { id: null, x: 5, y: 6 },
    };
    const npcs = {
      goblin: { id: "goblin", x: 7, y: 8 },
      rat: { id: "rat", x: 9, y: 10 },
      bad: { id: 123, x: 0, y: 0 },
    };

    const { displayPlayers, displayNPCs } = deriveDisplayMaps(players, npcs);

    expect(displayPlayers).toEqual({
      alpha: { x: 1, y: 2 },
      beta: { x: 3, y: 4 },
    });
    expect(displayNPCs).toEqual({
      goblin: { x: 7, y: 8 },
      rat: { x: 9, y: 10 },
    });

    const { displayPlayers: prunedPlayers, displayNPCs: prunedNPCs } = deriveDisplayMaps(
      { alpha: players.alpha },
      { goblin: npcs.goblin },
      displayPlayers,
      displayNPCs,
    );

    expect(prunedPlayers).toEqual({ alpha: { x: 1, y: 2 } });
    expect(prunedNPCs).toEqual({ goblin: { x: 7, y: 8 } });
    expect(prunedPlayers.alpha).toBe(displayPlayers.alpha);
    expect(prunedNPCs.goblin).toBe(displayNPCs.goblin);
  });

  it("handles non-object inputs without throwing", () => {
    const { displayPlayers, displayNPCs } = deriveDisplayMaps(null, 5);
    expect(displayPlayers).toEqual({});
    expect(displayNPCs).toEqual({});
  });

  it("creates fresh entries when no previous display coordinates are available", () => {
    const players = { alpha: { id: "alpha", x: 10, y: 20 } };
    const npcs = { goblin: { id: "goblin", x: 30, y: 40 } };

    const { displayPlayers, displayNPCs } = deriveDisplayMaps(players, npcs, {
      alpha: { x: NaN, y: 0 },
    });

    expect(displayPlayers.alpha).toEqual({ x: 10, y: 20 });
    expect(displayNPCs.goblin).toEqual({ x: 30, y: 40 });
  });
});

describe("enqueueEffectTriggers", () => {
  it.each([
    { label: "triggers are undefined", triggers: undefined },
    { label: "triggers are null", triggers: null },
    { label: "triggers are an empty array", triggers: [] },
  ])("returns clones of previous state when $label", ({ triggers }) => {
    const prev = {
      pending: [{ id: "existing", effect: "alpha" }],
      processedIds: new Set(["existing"]),
    };
    const originalPending = prev.pending.slice();
    const originalProcessed = new Set(prev.processedIds);

    const result = enqueueEffectTriggers(prev, triggers);

    expect(result.pending).toEqual(originalPending);
    expect(result.pending).not.toBe(prev.pending);
    expect([...result.processedIds]).toEqual([...originalProcessed]);
    expect(result.processedIds).not.toBe(prev.processedIds);
    expect(prev.pending).toEqual(originalPending);
    expect(prev.processedIds.size).toBe(originalProcessed.size);
  });

  it("ignores invalid entries and deduplicates by id", () => {
    const prev = { pending: [], processedIds: new Set() };
    const triggers = [
      null,
      "string",
      { effect: "ambient" },
      { id: "spell-1", effect: "fire" },
      { id: "spell-1", effect: "duplicate" },
      { id: "spell-2", effect: "ice" },
      { id: 123, effect: "numeric" },
      { effect: "buff" },
    ];

    const result = enqueueEffectTriggers(prev, triggers);

    expect(result.pending).toEqual([
      triggers[2],
      triggers[3],
      triggers[5],
      triggers[6],
      triggers[7],
    ]);
    expect([...result.processedIds]).toEqual(["spell-1", "spell-2"]);
  });

  it("skips triggers already processed across batches", () => {
    const existing = { id: "alpha", effect: "existing" };
    const prev = {
      pending: [existing],
      processedIds: new Set(["alpha"]),
    };
    const triggers = [
      { id: "alpha", effect: "duplicate" },
      { id: "beta", effect: "new" },
      { id: "gamma", effect: "also-new" },
      { id: "beta", effect: "duplicate" },
    ];

    const result = enqueueEffectTriggers(prev, triggers);

    expect(result.pending).toEqual([existing, triggers[1], triggers[2]]);
    expect([...result.processedIds]).toEqual(["alpha", "beta", "gamma"]);
    expect(prev.pending).toEqual([existing]);
    expect([...prev.processedIds]).toEqual(["alpha"]);
  });

  it("always enqueues triggers without ids while preserving order", () => {
    const triggers = [
      { effect: "first" },
      { id: "unique", effect: "with-id" },
      { effect: "second" },
      { id: "unique", effect: "duplicate" },
      { effect: "third" },
    ];

    const result = enqueueEffectTriggers({ pending: [], processedIds: new Set() }, triggers);

    expect(result.pending).toEqual([triggers[0], triggers[1], triggers[2], triggers[4]]);
    expect([...result.processedIds]).toEqual(["unique"]);
  });

  it("deduplicates large batches without quadratic growth", () => {
    const triggers = Array.from({ length: 1000 }, (_, index) => ({
      id: `effect-${index % 500}`,
    }));

    const result = enqueueEffectTriggers({ pending: [], processedIds: new Set() }, triggers);

    expect(result.pending).toHaveLength(500);
    expect(result.pending).toEqual(triggers.slice(0, 500));
    expect(result.processedIds.size).toBe(500);
  });
});

describe("message payload builders", () => {
  it.each([
    {
      label: "preserves provided intent values",
      intent: { dx: 1, dy: -0.5 },
      facing: "left",
      expected: { type: "input", dx: 1, dy: -0.5, facing: "left" },
    },
    {
      label: "normalizes invalid intent and facing",
      intent: null,
      facing: "north-east",
      expected: { type: "input", dx: 0, dy: 0, facing: "down" },
    },
    {
      label: "coerces non-finite deltas",
      intent: { dx: Number.NaN, dy: Infinity },
      facing: "right",
      expected: { type: "input", dx: 0, dy: 0, facing: "right" },
    },
    {
      label: "coerces string deltas",
      intent: { dx: "5", dy: "-4.5" },
      facing: "up",
      expected: { type: "input", dx: 5, dy: -4.5, facing: "up" },
    },
  ])("buildInputPayload $label", ({ intent, facing, expected }) => {
    expect(buildInputPayload(intent, facing)).toEqual(expected);
  });

  it("buildPathPayload preserves coordinates without mutation", () => {
    const payload = buildPathPayload(123.45, -67.89);
    expect(payload).toEqual({ type: "path", x: 123.45, y: -67.89 });
  });

  it("buildCancelPathPayload returns only the command type", () => {
    expect(buildCancelPathPayload()).toEqual({ type: "cancelPath" });
  });

  it.each([
    {
      label: "includes params when provided",
      params: { slot: 2 },
      expected: { type: "action", action: "use", params: { slot: 2 } },
    },
    {
      label: "omits params when null",
      params: null,
      expected: { type: "action", action: "use" },
    },
    {
      label: "omits params when empty",
      params: {},
      expected: { type: "action", action: "use" },
    },
  ])("buildActionPayload $label", ({ params, expected }) => {
    expect(buildActionPayload("use", params)).toEqual(expected);
  });

  it("buildHeartbeatPayload preserves the provided timestamp", () => {
    expect(buildHeartbeatPayload(1234)).toEqual({ type: "heartbeat", sentAt: 1234 });
  });

  it.each([
    {
      label: "includes qty when provided as an integer",
      params: { qty: 5 },
      expected: { type: "console", cmd: "spawn", qty: 5 },
    },
    {
      label: "truncates finite float qty",
      params: { qty: 3.9 },
      expected: { type: "console", cmd: "spawn", qty: 3 },
    },
    {
      label: "coerces numeric strings",
      params: { qty: "7" },
      expected: { type: "console", cmd: "spawn", qty: 7 },
    },
    {
      label: "omits qty when null",
      params: { qty: null },
      expected: { type: "console", cmd: "spawn" },
    },
    {
      label: "omits qty when not finite",
      params: { qty: Infinity },
      expected: { type: "console", cmd: "spawn" },
    },
    {
      label: "omits qty when NaN",
      params: { qty: Number.NaN },
      expected: { type: "console", cmd: "spawn" },
    },
    {
      label: "omits qty when string is non-numeric",
      params: { qty: "many" },
      expected: { type: "console", cmd: "spawn" },
    },
    {
      label: "omits qty when params absent",
      params: undefined,
      expected: { type: "console", cmd: "spawn" },
    },
  ])("buildConsolePayload $label", ({ params, expected }) => {
    expect(buildConsolePayload("spawn", params)).toEqual(expected);
  });
});

describe("protocol version helpers", () => {
  it("extracts version from numeric ver fields", () => {
    expect(readProtocolVersion({ ver: 3 })).toBe(3);
  });

  it("coerces string versions and falls back to protocol field", () => {
    expect(readProtocolVersion({ ver: "2" })).toBe(2);
    expect(readProtocolVersion({ protocol: "5" })).toBe(5);
  });

  it("returns null when no version fields are present", () => {
    expect(readProtocolVersion({})).toBeNull();
    expect(readProtocolVersion(null)).toBeNull();
  });

  it("invokes mismatch handler when versions differ", () => {
    const onMismatch = vi.fn();
    const version = handleProtocolVersion(
      { ver: PROTOCOL_VERSION + 2 },
      "state update",
      { onMismatch },
    );

    expect(version).toBe(PROTOCOL_VERSION + 2);
    expect(onMismatch).toHaveBeenCalledTimes(1);
    expect(onMismatch).toHaveBeenCalledWith({
      expected: PROTOCOL_VERSION,
      received: PROTOCOL_VERSION + 2,
      context: "state update",
      message: expect.stringContaining("Protocol version mismatch"),
    });
  });

  it("does not invoke mismatch handler when versions match", () => {
    const onMismatch = vi.fn();
    const version = handleProtocolVersion(
      { ver: PROTOCOL_VERSION },
      "state update",
      { onMismatch },
    );

    expect(version).toBe(PROTOCOL_VERSION);
    expect(onMismatch).not.toHaveBeenCalled();
  });

  it("does not invoke mismatch handler when version missing", () => {
    const onMismatch = vi.fn();
    const version = handleProtocolVersion({}, "join", { onMismatch });

    expect(version).toBeNull();
    expect(onMismatch).not.toHaveBeenCalled();
  });
});

describe("sendMessage", () => {
  it("attaches protocol version to outbound payloads", () => {
    const originalWebSocket = globalThis.WebSocket;
    globalThis.WebSocket = { OPEN: 1 };

    try {
      const send = vi.fn();
      const store = {
        socket: { readyState: 1, send },
        messagesSent: 0,
        bytesSent: 0,
        lastMessageSentAt: null,
        updateDiagnostics: vi.fn(),
      };

      sendMessage(store, { type: "input", dx: 0, dy: 0, facing: "down" });

      expect(send).toHaveBeenCalledTimes(1);
      const encoded = send.mock.calls[0][0];
      const decoded = JSON.parse(encoded);
      expect(decoded).toEqual({
        type: "input",
        dx: 0,
        dy: 0,
        facing: "down",
        ver: PROTOCOL_VERSION,
      });
    } finally {
      globalThis.WebSocket = originalWebSocket;
    }
  });

  it("includes the latest applied tick as ack when available", () => {
    const originalWebSocket = globalThis.WebSocket;
    globalThis.WebSocket = { OPEN: 1 };

    try {
      const send = vi.fn();
      const store = {
        socket: { readyState: 1, send },
        messagesSent: 0,
        bytesSent: 0,
        lastMessageSentAt: null,
        updateDiagnostics: vi.fn(),
        lastTick: 73.8,
      };

      sendMessage(store, { type: "heartbeat", sentAt: 42 });

      const encoded = send.mock.calls[0][0];
      const decoded = JSON.parse(encoded);
      expect(decoded.ack).toBe(73);
      expect(decoded.ver).toBe(PROTOCOL_VERSION);
    } finally {
      globalThis.WebSocket = originalWebSocket;
    }
  });

  it("omits ack when the last tick is unavailable", () => {
    const originalWebSocket = globalThis.WebSocket;
    globalThis.WebSocket = { OPEN: 1 };

    try {
      const send = vi.fn();
      const store = {
        socket: { readyState: 1, send },
        messagesSent: 0,
        bytesSent: 0,
        lastMessageSentAt: null,
        updateDiagnostics: vi.fn(),
        lastTick: null,
      };

      sendMessage(store, { type: "action", action: "attack" });

      const encoded = send.mock.calls[0][0];
      const decoded = JSON.parse(encoded);
      expect(decoded).not.toHaveProperty("ack");
    } finally {
      globalThis.WebSocket = originalWebSocket;
    }
  });
});

describe("parseServerEvent", () => {
  it("returns an envelope for valid state events", () => {
    const encoded = JSON.stringify({ type: "state", players: [], sequence: 5 });

    const result = parseServerEvent(encoded);

    expect(result).not.toBeNull();
    expect(result).toEqual({
      type: "state",
      data: { type: "state", players: [], sequence: 5 },
    });
  });

  it("preserves extra fields for heartbeat events", () => {
    const encoded = JSON.stringify({ type: "heartbeat", clientTime: 50, rtt: 20 });

    const result = parseServerEvent(encoded);

    expect(result).not.toBeNull();
    expect(result.type).toBe("heartbeat");
    expect(result?.data).toEqual({ type: "heartbeat", clientTime: 50, rtt: 20 });
  });

  it.each([
    { label: "non-string input", input: null },
    { label: "number input", input: 42 },
    { label: "boolean input", input: true },
    { label: "invalid json", input: "{" },
    { label: "missing type", input: JSON.stringify({}) },
    { label: "non-string type", input: JSON.stringify({ type: 7 }) },
    { label: "array payload", input: JSON.stringify([]) },
    {
      label: "array with type-like object",
      input: JSON.stringify([{ type: "state" }]),
    },
    { label: "empty string", input: "" },
  ])("returns null for $label", ({ input }) => {
    expect(parseServerEvent(input)).toBeNull();
  });

  it("remains compatible with computeRtt for heartbeat payloads", () => {
    const encoded = JSON.stringify({ type: "heartbeat", clientTime: 100 });
    const result = parseServerEvent(encoded);

    expect(result?.type).toBe("heartbeat");
    expect(computeRtt(result?.data, 175)).toBe(75);
  });
});

describe("keyframe retry loop", () => {
  it("backs off before dispatching another request", () => {
    const originalWebSocket = globalThis.WebSocket;
    globalThis.WebSocket = { OPEN: 1 };

    vi.useFakeTimers();
    try {
      const now = Date.UTC(2024, 0, 1, 0, 0, 0);
      vi.setSystemTime(now);

      const send = vi.fn();
      const store = {
        patchState: {
          pendingKeyframeRequests: new Map(),
          pendingReplays: [],
        },
        simulatedLatencyMs: 0,
        socket: { readyState: 1, send },
        messagesSent: 0,
        bytesSent: 0,
        lastMessageSentAt: null,
        updateDiagnostics: vi.fn(),
        keyframeRetryTimer: null,
      };

      __networkInternals.requestKeyframeSnapshot(store, 7, null, {
        incrementAttempt: false,
        firstRequestedAt: now,
      });

      if (store.keyframeRetryTimer !== null) {
        clearInterval(store.keyframeRetryTimer);
        store.keyframeRetryTimer = null;
      }

      const pendingEntry =
        store.patchState.pendingKeyframeRequests.get(7) || null;
      expect(pendingEntry).not.toBeNull();
      expect(pendingEntry?.attempts).toBe(1);
      expect(pendingEntry?.nextRetryAt).toBe(
        now + __networkInternals.computeKeyframeRetryDelay(1),
      );

      __networkInternals.processKeyframeRetryLoop(store);
      expect(send).toHaveBeenCalledTimes(1);

      vi.setSystemTime(
        now + __networkInternals.computeKeyframeRetryDelay(1) + 1,
      );
      __networkInternals.processKeyframeRetryLoop(store);
      expect(send).toHaveBeenCalledTimes(2);
    } finally {
      vi.useRealTimers();
      globalThis.WebSocket = originalWebSocket;
    }
  });

  it("requests a resync when retries are exhausted", () => {
    vi.useFakeTimers();
    try {
      const now = Date.UTC(2024, 0, 1, 0, 0, 0);
      vi.setSystemTime(now);

      const pendingEntry = {
        attempts: __networkInternals.KEYFRAME_RETRY_CONSTANTS.MAX_ATTEMPTS,
        nextRetryAt: now - 1,
        firstRequestedAt: now - 5000,
      };

      const originalPatchState = {
        pendingKeyframeRequests: new Map([[11, pendingEntry]]),
        pendingReplays: [],
      };

      const setLatency = vi.fn();
      const setStatusBase = vi.fn();
      const store = {
        patchState: originalPatchState,
        keyframeRetryTimer: null,
        socket: null,
        setLatency,
        updateDiagnostics: vi.fn(),
        playerId: null,
        players: {},
        displayPlayers: {},
        npcs: {},
        displayNPCs: {},
        directionOrder: [],
        currentIntent: { dx: 0, dy: 0 },
        setStatusBase,
        effectManager: null,
        reconnectTimeout: null,
      };

      __networkInternals.processKeyframeRetryLoop(store);

      expect(store.patchState).not.toBe(originalPatchState);
      expect(store.patchState?.pendingKeyframeRequests instanceof Map).toBe(true);
      expect(
        store.patchState?.pendingKeyframeRequests?.size ?? 0,
      ).toBe(0);
      expect(setLatency).toHaveBeenCalledWith(null);
      expect(store.keyframeRetryTimer).toBeNull();
    } finally {
      vi.useRealTimers();
    }
  });
});
