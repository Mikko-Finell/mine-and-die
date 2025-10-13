import { describe, expect, test, vi } from "vitest";
import {
  applyEffectLifecycleBatch,
  ensureEffectLifecycleState,
  getEffectLifecycleEntry,
  resetEffectLifecycleState,
} from "../effect-lifecycle.js";

function createSpawnEvent({
  id = "effect-1",
  seq = 1,
  tick = 10,
  definitionId = "attack",
} = {}) {
  return {
    tick,
    seq,
    instance: {
      id,
      definitionId,
      deliveryState: { geometry: { shape: "rect", width: 2 } },
      behaviorState: { ticksRemaining: 3 },
      params: { power: 5 },
    },
  };
}

describe("effect-lifecycle", () => {
  test("ensureEffectLifecycleState attaches maps to the store", () => {
    const store = {};
    const state = ensureEffectLifecycleState(store);
    expect(state.instances).toBeInstanceOf(Map);
    expect(state.lastSeqById).toBeInstanceOf(Map);
    const again = ensureEffectLifecycleState(store);
    expect(again).toBe(state);
  });

  test("applyEffectLifecycleBatch processes spawn, update, and end events", () => {
    const store = {};
    const spawnSummary = applyEffectLifecycleBatch(store, {
      effect_spawned: [createSpawnEvent()],
    });
    expect(spawnSummary.spawns).toEqual(["effect-1"]);
    const state = ensureEffectLifecycleState(store);
    const entryAfterSpawn = getEffectLifecycleEntry(store, "effect-1");
    expect(entryAfterSpawn).not.toBeNull();
    expect(entryAfterSpawn.instance.deliveryState).toEqual({
      geometry: { shape: "rect", width: 2 },
    });
    expect(entryAfterSpawn.seq).toBe(1);

    const updateSummary = applyEffectLifecycleBatch(store, {
      effect_update: [
        {
          id: "effect-1",
          seq: 2,
          tick: 11,
          deliveryState: { geometry: { shape: "rect", width: 4 } },
          params: { damage: 12 },
        },
      ],
    });
    expect(updateSummary.updates).toEqual(["effect-1"]);
    const entryAfterUpdate = getEffectLifecycleEntry(store, "effect-1");
    expect(entryAfterUpdate.instance.deliveryState).toEqual({
      geometry: { shape: "rect", width: 4 },
    });
    expect(entryAfterUpdate.instance.params).toEqual({ power: 5, damage: 12 });
    expect(entryAfterUpdate.seq).toBe(2);

    const endSummary = applyEffectLifecycleBatch(store, {
      effect_ended: [
        {
          id: "effect-1",
          seq: 3,
          tick: 12,
          reason: "expired",
        },
      ],
    });
    expect(endSummary.ends).toEqual(["effect-1"]);
    expect(state.instances.size).toBe(0);
    expect(state.lastSeqById.get("effect-1")).toBe(3);
  });

  test("applyEffectLifecycleBatch drops stale events and reports unknown updates", () => {
    const store = {};
    applyEffectLifecycleBatch(store, { effect_spawned: [createSpawnEvent()] });
    const duplicate = applyEffectLifecycleBatch(store, {
      effect_spawned: [createSpawnEvent()],
    });
    expect(duplicate.spawns).toHaveLength(0);
    expect(duplicate.droppedSpawns).toEqual(["effect-1"]);

    const handler = vi.fn();
    const unknown = applyEffectLifecycleBatch(
      {},
      {
        effect_update: [
          {
            id: "ghost",
            seq: 1,
            tick: 9,
          },
        ],
      },
      { onUnknownUpdate: handler },
    );
    expect(unknown.updates).toHaveLength(0);
    expect(unknown.unknownUpdates).toHaveLength(1);
    expect(unknown.unknownUpdates[0]).toMatchObject({ id: "ghost", seq: 1 });
    expect(handler).toHaveBeenCalledTimes(1);
    expect(handler.mock.calls[0][0]).toMatchObject({ id: "ghost", seq: 1 });
  });

  test("effect_seq_cursors update the tracked sequence map", () => {
    const store = {};
    applyEffectLifecycleBatch(store, { effect_spawned: [createSpawnEvent()] });
    applyEffectLifecycleBatch(store, {
      effect_seq_cursors: { "effect-1": 10, orphan: 2 },
    });
    const state = ensureEffectLifecycleState(store);
    expect(state.lastSeqById.get("effect-1")).toBe(10);
    expect(state.lastSeqById.get("orphan")).toBe(2);
  });

  test("resetEffectLifecycleState clears tracked entries", () => {
    const store = {};
    applyEffectLifecycleBatch(store, { effect_spawned: [createSpawnEvent()] });
    resetEffectLifecycleState(store);
    const state = ensureEffectLifecycleState(store);
    expect(state.instances.size).toBe(0);
    expect(state.lastSeqById.size).toBe(0);
  });
});
