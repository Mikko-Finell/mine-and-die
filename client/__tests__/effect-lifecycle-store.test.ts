import { beforeEach, describe, expect, test } from "vitest";

import {
  ContractLifecycleStore,
  type ContractLifecycleEndEvent,
  type ContractLifecycleSpawnEvent,
  type ContractLifecycleUpdateEvent,
} from "../effect-lifecycle-store";
import { setEffectCatalog } from "../effect-catalog";

const createInstance = ({
  id,
  entryId,
  definitionId,
}: {
  readonly id: string;
  readonly entryId: string | null;
  readonly definitionId: string;
}) => ({
  id,
  entryId: entryId ?? undefined,
  definitionId,
  startTick: 10,
  deliveryState: {
    geometry: { shape: "circle" as const },
    motion: {
      positionX: 1,
      positionY: 2,
      velocityX: 0,
      velocityY: 0,
    },
  },
  behaviorState: { ticksRemaining: 5 },
  replication: { sendSpawn: true, sendUpdates: true, sendEnd: true },
  end: { kind: 0 as const },
});

const createSpawn = (options: {
  readonly seq: number;
  readonly tick?: number;
  readonly id: string;
  readonly entryId: string | null;
  readonly definitionId: string;
}): ContractLifecycleSpawnEvent => ({
  seq: options.seq,
  tick: options.tick ?? null,
  instance: createInstance({
    id: options.id,
    entryId: options.entryId,
    definitionId: options.definitionId,
  }),
});

const createUpdate = (options: {
  readonly seq: number;
  readonly tick?: number;
  readonly id: string;
  readonly offsetX?: number;
  readonly ticksRemaining?: number;
  readonly params?: Readonly<Record<string, number>>;
}): ContractLifecycleUpdateEvent => ({
  seq: options.seq,
  tick: options.tick ?? null,
  id: options.id,
  deliveryState: {
    geometry: {
      shape: "circle" as const,
      offsetX: options.offsetX ?? 0,
    },
    motion: {
      positionX: 1,
      positionY: 2,
      velocityX: 3,
      velocityY: 4,
    },
  },
  behaviorState: {
    ticksRemaining: options.ticksRemaining ?? 4,
  },
  params: options.params,
});

const createEnd = (options: {
  readonly seq: number;
  readonly tick?: number;
  readonly id: string;
  readonly reason?: "cancelled" | "expired" | "mapChange" | "ownerLost";
}): ContractLifecycleEndEvent => ({
  seq: options.seq,
  tick: options.tick ?? null,
  id: options.id,
  reason: options.reason ?? "expired",
});

describe("ContractLifecycleStore", () => {
  beforeEach(() => {
    setEffectCatalog({});
  });

  test("tracks spawn, update, and end for server-managed entries", () => {
    setEffectCatalog({
      "fireball-entry": {
        contractId: "fireball",
        managedByClient: false,
        blocks: {},
      },
    });

    const store = new ContractLifecycleStore();

    const spawn = createSpawn({
      seq: 1,
      tick: 10,
      id: "effect-1",
      entryId: "fireball-entry",
      definitionId: "fireball",
    });

    const spawnSummary = store.applyBatch({ spawns: [spawn] });
    expect(spawnSummary.spawns).toEqual(["effect-1"]);
    expect(store.snapshot().version).toBeGreaterThan(0);

    const update = createUpdate({
      seq: 2,
      tick: 11,
      id: "effect-1",
      offsetX: 8,
      ticksRemaining: 3,
      params: { damage: 12 },
    });
    const updateSummary = store.applyBatch({ updates: [update] });
    expect(updateSummary.updates).toEqual(["effect-1"]);

    const afterUpdate = store.snapshot();
    const entry = afterUpdate.entries.get("effect-1");
    expect(entry).not.toBeUndefined();
    expect(entry?.lastEventKind).toBe("update");
    expect(entry?.instance.deliveryState.geometry.offsetX).toBe(8);
    expect(entry?.instance.behaviorState.ticksRemaining).toBe(3);
    expect(entry?.instance.params).toEqual({ damage: 12 });

    const end = createEnd({ seq: 3, tick: 12, id: "effect-1", reason: "expired" });
    const endSummary = store.applyBatch({ ends: [end] });
    expect(endSummary.ends).toEqual(["effect-1"]);
    expect(endSummary.retained).toEqual([]);

    const afterEnd = store.snapshot();
    expect(afterEnd.entries.size).toBe(0);
    const ended = afterEnd.recentlyEnded.get("effect-1");
    expect(ended).not.toBeUndefined();
    expect(ended?.retained).toBe(false);
    expect(ended?.end?.reason).toBe("expired");
  });

  test("retains client-managed entries when they end", () => {
    setEffectCatalog({
      "managed-entry": {
        contractId: "fireball",
        managedByClient: true,
        blocks: {},
      },
    });

    const store = new ContractLifecycleStore();

    const spawn = createSpawn({
      seq: 1,
      id: "effect-retained",
      entryId: "managed-entry",
      definitionId: "fireball",
    });
    store.applyBatch({ spawns: [spawn] });

    const end = createEnd({ seq: 2, id: "effect-retained", reason: "expired" });
    const endSummary = store.applyBatch({ ends: [end] });
    expect(endSummary.retained).toEqual(["effect-retained"]);

    const view = store.snapshot();
    const entry = view.entries.get("effect-retained");
    expect(entry).not.toBeUndefined();
    expect(entry?.managedByClient).toBe(true);
    expect(entry?.retained).toBe(true);
    expect(view.recentlyEnded.size).toBe(0);
  });

  test("reports dropped and unknown events", () => {
    const store = new ContractLifecycleStore();

    const orphanUpdate = createUpdate({ seq: 1, id: "ghost" });
    const firstSummary = store.applyBatch({ updates: [orphanUpdate] });
    expect(firstSummary.updates).toEqual([]);
    expect(firstSummary.unknownUpdates).toEqual([orphanUpdate]);

    setEffectCatalog({
      "fireball-entry": {
        contractId: "fireball",
        managedByClient: false,
        blocks: {},
      },
    });

    const spawn = createSpawn({
      seq: 1,
      id: "effect-dup",
      entryId: "fireball-entry",
      definitionId: "fireball",
    });
    store.applyBatch({ spawns: [spawn] });

    const duplicateSpawnSummary = store.applyBatch({ spawns: [spawn] });
    expect(duplicateSpawnSummary.spawns).toEqual([]);
    expect(duplicateSpawnSummary.droppedSpawns).toEqual(["effect-dup"]);

    const staleUpdate = createUpdate({ seq: 1, id: "effect-dup" });
    const staleSummary = store.applyBatch({ updates: [staleUpdate] });
    expect(staleSummary.updates).toEqual([]);
    expect(staleSummary.droppedUpdates).toEqual(["effect-dup"]);
  });
});

