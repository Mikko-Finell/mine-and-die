import { beforeEach, describe, expect, test, vi } from "vitest";

import * as effectCatalogStore from "../effect-catalog";
import {
  effectCatalog as generatedEffectCatalog,
} from "../generated/effect-contracts";
import {
  type ContractLifecycleBatch,
  type ContractLifecycleEndEvent,
  type ContractLifecycleSpawnEvent,
  type ContractLifecycleUpdateEvent,
} from "../effect-lifecycle-store";
import { createHeadlessHarness } from "./helpers/headless-harness";

describe("Lifecycle renderer smoke test", () => {
  beforeEach(() => {
    effectCatalogStore.setEffectCatalog(null);
  });

  test("replays recorded lifecycle batches and renders frames from generated metadata", async () => {
    const fireballEntry = generatedEffectCatalog.fireball;
    const fireballParameters = {
      ...(fireballEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const attackEntry = generatedEffectCatalog.attack;
    const attackParameters = {
      ...(attackEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const fireballRadius = fireballParameters.radius ?? 12;
    const { network, renderer, orchestrator } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    const onReady = vi.fn();
    await orchestrator.boot({ onReady });
    expect(onReady).toHaveBeenCalledTimes(1);

    const fireballSpawn: ContractLifecycleSpawnEvent = {
      seq: 1,
      tick: 120,
      instance: {
        id: "effect-fireball",
        entryId: "fireball",
        definitionId: fireballEntry.contractId,
        definition: fireballEntry.definition,
        startTick: 120,
        deliveryState: {
          geometry: {
            shape: "circle",
            radius: fireballRadius,
            offsetX: 4,
          },
          motion: {
            positionX: 256,
            positionY: 320,
            velocityX: 64,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: 30,
          tickCadence: 1,
        },
        params: fireballParameters,
        colors: ["#ffaa33"],
        replication: fireballEntry.definition.client,
        end: fireballEntry.definition.end,
      },
    };

    const attackReach = attackParameters.reach ?? 56;
    const attackWidth = attackParameters.width ?? 40;
    const attackSpawn: ContractLifecycleSpawnEvent = {
      seq: 1,
      tick: 120,
      instance: {
        id: "effect-attack",
        entryId: "attack",
        definitionId: attackEntry.contractId,
        definition: attackEntry.definition,
        startTick: 120,
        deliveryState: {
          geometry: {
            shape: "rect",
            width: attackWidth,
            height: attackReach,
            offsetX: attackWidth / 2,
            offsetY: -(attackReach / 2),
          },
          motion: {
            positionX: 208,
            positionY: 320,
            velocityX: 0,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: 1,
          tickCadence: 1,
        },
        params: attackParameters,
        colors: ["#ffffff"],
        replication: attackEntry.definition.client,
        end: attackEntry.definition.end,
      },
    };

    const fireballUpdate: ContractLifecycleUpdateEvent = {
      seq: 2,
      tick: 121,
      id: "effect-fireball",
      deliveryState: {
        geometry: {
          shape: "circle",
          radius: fireballRadius,
          offsetX: 12,
        },
        motion: {
          positionX: 272,
          positionY: 320,
          velocityX: 64,
          velocityY: 0,
        },
      },
      behaviorState: {
        ticksRemaining: 28,
      },
      params: { ...fireballParameters, range: 180 } as Readonly<Record<string, number>>,
    };

    const attackUpdate: ContractLifecycleUpdateEvent = {
      seq: 2,
      tick: 121,
      id: "effect-attack",
      deliveryState: {
        geometry: {
          shape: "rect",
          width: attackWidth,
          height: attackReach,
          offsetX: attackWidth,
          offsetY: -(attackReach / 2),
        },
        motion: {
          positionX: 224,
          positionY: 320,
          velocityX: 32,
          velocityY: 0,
        },
      },
      behaviorState: {
        ticksRemaining: 0,
      },
      params: attackParameters,
    };

    const fireballEnd: ContractLifecycleEndEvent = {
      seq: 3,
      tick: 124,
      id: "effect-fireball",
      reason: "expired",
    };

    const attackEnd: ContractLifecycleEndEvent = {
      seq: 3,
      tick: 122,
      id: "effect-attack",
      reason: "expired",
    };

    const recordedBatch: ContractLifecycleBatch = {
      spawns: [fireballSpawn, attackSpawn],
      updates: [fireballUpdate, attackUpdate],
      cursors: { "effect-fireball": 2, "effect-attack": 2 },
    };

    network.emit({
      type: "state",
      payload: {
        effect_spawned: recordedBatch.spawns,
        effect_update: recordedBatch.updates,
        effect_seq_cursors: recordedBatch.cursors,
        t: 121,
      },
      receivedAt: 2500,
    });

    expect(renderer.batches.length).toBeGreaterThanOrEqual(2);
    const activeBatch = renderer.batches.at(-1)!;
    expect(activeBatch.keyframeId).toBe("tick-121");
    expect(activeBatch.staticGeometry.length).toBeGreaterThan(0);
    expect(activeBatch.animations.length).toBeGreaterThan(0);

    const geometry = activeBatch.staticGeometry.find((entry) => entry.id === "effect-fireball");
    expect(geometry).toBeDefined();
    expect(geometry!.layer.id).toBe("effect-area");
    expect(geometry!.style).toMatchObject({
      entryId: "fireball",
      managedByClient: fireballEntry.managedByClient,
    });

    const animation = activeBatch.animations.find((entry) => entry.effectId === "effect-fireball");
    expect(animation).toBeDefined();
    expect(animation!.metadata).toMatchObject({
      contractId: fireballEntry.contractId,
      entryId: "fireball",
      managedByClient: fireballEntry.managedByClient,
      lastEventKind: "update",
      catalog: fireballEntry,
      blocks: fireballEntry.blocks,
    });
    expect(animation!.metadata.instance.deliveryState.motion.positionX).toBe(272);
    expect(animation!.metadata.retained).toBe(false);

    const attackGeometry = activeBatch.staticGeometry.find((entry) => entry.id === "effect-attack");
    expect(attackGeometry).toBeDefined();
    expect(attackGeometry!.layer.id).toBe("effect-area");
    expect(attackGeometry!.style).toMatchObject({
      entryId: "attack",
      managedByClient: attackEntry.managedByClient,
    });

    const attackAnimation = activeBatch.animations.find((entry) => entry.effectId === "effect-attack");
    expect(attackAnimation).toBeDefined();
    expect(attackAnimation!.metadata).toMatchObject({
      contractId: attackEntry.contractId,
      entryId: "attack",
      managedByClient: attackEntry.managedByClient,
      lastEventKind: "update",
      catalog: attackEntry,
      blocks: attackEntry.blocks,
    });
    expect(attackAnimation!.metadata.retained).toBe(false);

    const endBatch: ContractLifecycleBatch = {
      ends: [fireballEnd, attackEnd],
      cursors: { "effect-fireball": 3, "effect-attack": 3 },
    };

    network.emit({
      type: "state",
      payload: {
        effect_ended: endBatch.ends,
        effect_seq_cursors: endBatch.cursors,
        t: 124,
      },
      receivedAt: 3000,
    });

    expect(renderer.batches.length).toBeGreaterThanOrEqual(3);
    const endedBatch = renderer.batches.at(-1)!;
    expect(endedBatch.keyframeId).toBe("tick-124");
    expect(endedBatch.staticGeometry.length).toBeGreaterThan(0);
    expect(endedBatch.animations.length).toBeGreaterThan(0);

    const endedAnimation = endedBatch.animations.find((entry) => entry.effectId === "effect-fireball");
    expect(endedAnimation).toBeDefined();
    expect(endedAnimation!.metadata).toMatchObject({
      entryId: "fireball",
      contractId: fireballEntry.contractId,
      managedByClient: fireballEntry.managedByClient,
      state: "ended",
      lastEventKind: "end",
      retained: false,
      catalog: fireballEntry,
    });

    expect(endedBatch.staticGeometry.find((entry) => entry.id === "effect-fireball")).toBeUndefined();

    const retainedGeometry = endedBatch.staticGeometry.find((entry) => entry.id === "effect-attack");
    expect(retainedGeometry).toBeDefined();
    expect(retainedGeometry!.style).toMatchObject({
      entryId: "attack",
      managedByClient: attackEntry.managedByClient,
    });

    const retainedAnimation = endedBatch.animations.find((entry) => entry.effectId === "effect-attack");
    expect(retainedAnimation).toBeDefined();
    expect(retainedAnimation!.metadata).toMatchObject({
      entryId: "attack",
      contractId: attackEntry.contractId,
      managedByClient: attackEntry.managedByClient,
      lastEventKind: "end",
      catalog: attackEntry,
    });
    expect(retainedAnimation!.metadata.retained).toBe(true);

    const setCatalogSpy = vi.spyOn(effectCatalogStore, "setEffectCatalog");
    const normalizeCatalogSpy = vi.spyOn(effectCatalogStore, "normalizeEffectCatalog");

    try {
      const resyncTick = 130;
      const resyncCatalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));

      network.emit({
        type: "state",
        payload: {
          resync: true,
          t: resyncTick,
          config: {
            effectCatalog: resyncCatalogPayload,
          },
        },
        receivedAt: 3600,
      });

      expect(renderer.batches.length).toBeGreaterThanOrEqual(4);
      const resyncBatch = renderer.batches.at(-1)!;
      expect(resyncBatch.keyframeId).toBe("lifecycle-0");
      expect(resyncBatch.time).toBe(resyncTick * 16);
      expect(resyncBatch.staticGeometry).toHaveLength(0);
      expect(resyncBatch.animations).toHaveLength(0);

      expect(normalizeCatalogSpy).toHaveBeenCalledTimes(1);
      expect(normalizeCatalogSpy).toHaveBeenLastCalledWith(resyncCatalogPayload);
      const normalizedCatalog = normalizeCatalogSpy.mock.results.at(-1)?.value;
      expect(setCatalogSpy).toHaveBeenCalledTimes(1);
      expect(setCatalogSpy).toHaveBeenLastCalledWith(normalizedCatalog);

      const activeCatalog = effectCatalogStore.getEffectCatalog();
      expect(activeCatalog).toEqual(normalizedCatalog);
      expect(Object.isFrozen(activeCatalog)).toBe(true);
    } finally {
      normalizeCatalogSpy.mockRestore();
      setCatalogSpy.mockRestore();
    }
  });
});
