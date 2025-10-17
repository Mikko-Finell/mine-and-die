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

  test("hydrates effect catalog snapshot from keyframe responses", async () => {
    const fireballEntry = generatedEffectCatalog.fireball;
    const fireballParameters = {
      ...(fireballEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const { network, renderer, orchestrator } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    const onReady = vi.fn();
    await orchestrator.boot({ onReady });
    expect(onReady).toHaveBeenCalledTimes(1);

    const setCatalogSpy = vi.spyOn(effectCatalogStore, "setEffectCatalog");
    const normalizeCatalogSpy = vi.spyOn(effectCatalogStore, "normalizeEffectCatalog");

    try {
      setCatalogSpy.mockClear();
      normalizeCatalogSpy.mockClear();

      const keyframeCatalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));

      network.emit({
        type: "keyframe",
        payload: {
          config: {
            effectCatalog: keyframeCatalogPayload,
          },
        },
        receivedAt: 2000,
      });

      expect(normalizeCatalogSpy).toHaveBeenCalledTimes(1);
      expect(normalizeCatalogSpy).toHaveBeenLastCalledWith(keyframeCatalogPayload);
      const normalizedCatalog = normalizeCatalogSpy.mock.results.at(-1)?.value;
      expect(setCatalogSpy).toHaveBeenCalledTimes(1);
      expect(setCatalogSpy).toHaveBeenLastCalledWith(normalizedCatalog);

      const fireballSpawn: ContractLifecycleSpawnEvent = {
        seq: 1,
        tick: 64,
        instance: {
          id: "effect-fireball",
          entryId: "fireball",
          definitionId: fireballEntry.contractId,
          definition: fireballEntry.definition,
          startTick: 64,
          deliveryState: {
            geometry: {
              shape: "circle",
              radius: fireballParameters.radius ?? 12,
              offsetX: 0,
              offsetY: 0,
            },
            motion: {
              positionX: 192,
              positionY: 256,
              velocityX: 0,
              velocityY: 0,
            },
          },
          behaviorState: {
            ticksRemaining: fireballEntry.definition.lifetimeTicks,
            tickCadence: 1,
          },
          params: fireballParameters,
          colors: ["#ffaa33"],
          replication: fireballEntry.definition.client,
          end: fireballEntry.definition.end,
        },
      };

      network.emit({
        type: "state",
        payload: {
          effect_spawned: [fireballSpawn],
          effect_seq_cursors: { "effect-fireball": fireballSpawn.seq },
          t: fireballSpawn.tick,
        },
        receivedAt: 2400,
      });

      expect(renderer.batches.length).toBeGreaterThan(0);
      const lastBatch = renderer.batches.at(-1)!;
      const geometry = lastBatch.staticGeometry.find((entry) => entry.id === "effect-fireball");
      expect(geometry).toBeDefined();
      expect(geometry!.style).toMatchObject({
        entryId: "fireball",
        managedByClient: fireballEntry.managedByClient,
      });
      const animation = lastBatch.animations.find((entry) => entry.effectId === "effect-fireball");
      expect(animation).toBeDefined();
      expect(animation!.metadata).toMatchObject({
        contractId: fireballEntry.contractId,
        entryId: "fireball",
        catalog: fireballEntry,
      });
    } finally {
      normalizeCatalogSpy.mockRestore();
      setCatalogSpy.mockRestore();
    }
  });

  test("resets lifecycle store when keyframe requests are nacked and recovers on resync", async () => {
    const fireballEntry = generatedEffectCatalog.fireball;
    const fireballParameters = {
      ...(fireballEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const { network, renderer, orchestrator } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    const onReady = vi.fn();
    await orchestrator.boot({ onReady });
    expect(onReady).toHaveBeenCalledTimes(1);

    const fireballSpawn: ContractLifecycleSpawnEvent = {
      seq: 1,
      tick: 96,
      instance: {
        id: "effect-fireball",
        entryId: "fireball",
        definitionId: fireballEntry.contractId,
        definition: fireballEntry.definition,
        startTick: 96,
        deliveryState: {
          geometry: {
            shape: "circle",
            radius: fireballParameters.radius ?? 12,
          },
          motion: {
            positionX: 192,
            positionY: 256,
            velocityX: 0,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: fireballEntry.definition.lifetimeTicks,
          tickCadence: 1,
        },
        params: fireballParameters,
        colors: ["#ffaa33"],
        replication: fireballEntry.definition.client,
        end: fireballEntry.definition.end,
      },
    };

    network.emit({
      type: "state",
      payload: {
        effect_spawned: [fireballSpawn],
        effect_seq_cursors: { [fireballSpawn.instance.id]: fireballSpawn.seq },
        t: fireballSpawn.tick,
      },
      receivedAt: 2000,
    });

    expect(renderer.batches.length).toBeGreaterThan(0);
    let lastBatch = renderer.batches.at(-1)!;
    expect(lastBatch.animations.some((frame) => frame.effectId === fireballSpawn.instance.id)).toBe(true);

    const setCatalogSpy = vi.spyOn(effectCatalogStore, "setEffectCatalog");
    const normalizeCatalogSpy = vi.spyOn(effectCatalogStore, "normalizeEffectCatalog");

    try {
      setCatalogSpy.mockClear();
      normalizeCatalogSpy.mockClear();

      const resyncCatalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));

      network.emit({
        type: "keyframeNack",
        payload: {
          config: {
            effectCatalog: resyncCatalogPayload,
          },
          resync: true,
        },
        receivedAt: 2600,
      });

      expect(normalizeCatalogSpy).toHaveBeenCalledTimes(1);
      expect(normalizeCatalogSpy).toHaveBeenLastCalledWith(resyncCatalogPayload);
      const normalizedCatalog = normalizeCatalogSpy.mock.results.at(-1)?.value;
      expect(setCatalogSpy).toHaveBeenCalledTimes(1);
      expect(setCatalogSpy).toHaveBeenLastCalledWith(normalizedCatalog);

      lastBatch = renderer.batches.at(-1)!;
      expect(lastBatch.keyframeId).toBe("lifecycle-0");
      expect(lastBatch.staticGeometry).toHaveLength(0);
      expect(lastBatch.animations).toHaveLength(0);

      const resyncTick = 400;
      network.emit({
        type: "state",
        payload: {
          resync: true,
          t: resyncTick,
          config: {
            effectCatalog: resyncCatalogPayload,
          },
        },
        receivedAt: 3000,
      });

      expect(normalizeCatalogSpy).toHaveBeenCalledTimes(2);
      expect(normalizeCatalogSpy).toHaveBeenLastCalledWith(resyncCatalogPayload);
      const resyncedCatalog = normalizeCatalogSpy.mock.results.at(-1)?.value;
      expect(setCatalogSpy).toHaveBeenCalledTimes(2);
      expect(setCatalogSpy).toHaveBeenLastCalledWith(resyncedCatalog);

      lastBatch = renderer.batches.at(-1)!;
      expect(lastBatch.keyframeId).toBe("lifecycle-0");
      expect(lastBatch.staticGeometry).toHaveLength(0);
      expect(lastBatch.animations).toHaveLength(0);
      expect(lastBatch.time).toBe(resyncTick * 16);

      const postResyncSpawn: ContractLifecycleSpawnEvent = {
        seq: 11,
        tick: resyncTick + 4,
        instance: {
          ...fireballSpawn.instance,
          startTick: resyncTick + 4,
        },
      };

      network.emit({
        type: "state",
        payload: {
          effect_spawned: [postResyncSpawn],
          effect_seq_cursors: { [postResyncSpawn.instance.id]: postResyncSpawn.seq },
          t: postResyncSpawn.tick,
        },
        receivedAt: 3400,
      });

      lastBatch = renderer.batches.at(-1)!;
      expect(lastBatch.keyframeId).toBe(`tick-${postResyncSpawn.tick}`);
      const animations = lastBatch.animations.filter((frame) => frame.effectId === postResyncSpawn.instance.id);
      expect(animations).toHaveLength(1);
      expect(animations[0].metadata).toMatchObject({
        entryId: "fireball",
        contractId: fireballEntry.contractId,
        catalog: fireballEntry,
        lastEventKind: "spawn",
      });
    } finally {
      normalizeCatalogSpy.mockRestore();
      setCatalogSpy.mockRestore();
    }
  });

  test("retries keyframe requests after resync fallback before resuming rendering", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(0);

    const fireballEntry = generatedEffectCatalog.fireball;
    const fireballParameters = {
      ...(fireballEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const retryDelayMs = 250;
    const nackTime = 2000;
    const resyncTime = nackTime + 100;
    const timeUntilRetry = retryDelayMs - (resyncTime - nackTime);
    const { network, renderer, orchestrator } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
      orchestratorConfiguration: {
        keyframeRetryDelayMs: retryDelayMs,
        keyframeRetryPolicy: {
          baseMs: retryDelayMs,
          maxMs: retryDelayMs,
          multiplier: 1,
          jitterMs: 0,
        },
      },
    });

    const sendSpy = vi.spyOn(network, "send");

    try {
      const onReady = vi.fn();
      await orchestrator.boot({ onReady });
      expect(onReady).toHaveBeenCalledTimes(1);

      const initialSpawn: ContractLifecycleSpawnEvent = {
        seq: 4,
        tick: 144,
        instance: {
          id: "effect-fireball",
          entryId: "fireball",
          definitionId: fireballEntry.contractId,
          definition: fireballEntry.definition,
          startTick: 144,
          deliveryState: {
            geometry: {
              shape: "circle",
              radius: fireballParameters.radius ?? 12,
            },
            motion: {
              positionX: 160,
              positionY: 208,
              velocityX: 0,
              velocityY: 0,
            },
          },
          behaviorState: {
            ticksRemaining: fireballEntry.definition.lifetimeTicks,
            tickCadence: 1,
          },
          params: fireballParameters,
          colors: ["#ffaa33"],
          replication: fireballEntry.definition.client,
          end: fireballEntry.definition.end,
        },
      };

      network.emit({
        type: "state",
        payload: {
          effect_spawned: [initialSpawn],
          effect_seq_cursors: { [initialSpawn.instance.id]: initialSpawn.seq },
          t: initialSpawn.tick,
          keyframeSeq: 12,
        },
        receivedAt: 1200,
      });

      const initialBatch = renderer.batches.at(-1)!;
      expect(initialBatch.animations.some((frame) => frame.effectId === initialSpawn.instance.id)).toBe(true);

      sendSpy.mockClear();
      vi.setSystemTime(nackTime);

      const nackCatalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));
      network.emit({
        type: "keyframeNack",
        payload: {
          sequence: 12,
          resync: true,
          config: {
            effectCatalog: nackCatalogPayload,
          },
        },
        receivedAt: nackTime + 50,
      });

      expect(sendSpy).not.toHaveBeenCalled();
      const clearedBatch = renderer.batches.at(-1)!;
      expect(clearedBatch.staticGeometry).toHaveLength(0);
      expect(clearedBatch.animations).toHaveLength(0);

      const resyncSequence = 24;
      const resyncTick = 384;
      const resyncCatalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));
      vi.setSystemTime(resyncTime);
      network.emit({
        type: "state",
        payload: {
          resync: true,
          t: resyncTick,
          keyframeSeq: resyncSequence,
          config: {
            effectCatalog: resyncCatalogPayload,
          },
        },
        receivedAt: resyncTime + 50,
      });

      expect(sendSpy).not.toHaveBeenCalled();

      if (timeUntilRetry > 1) {
        vi.advanceTimersByTime(timeUntilRetry - 1);
        expect(sendSpy).not.toHaveBeenCalled();
        vi.advanceTimersByTime(1);
        expect(sendSpy).toHaveBeenCalledTimes(1);
      } else if (timeUntilRetry === 1) {
        vi.advanceTimersByTime(1);
        expect(sendSpy).toHaveBeenCalledTimes(1);
      } else {
        expect(sendSpy).toHaveBeenCalledTimes(1);
      }
      expect(sendSpy).toHaveBeenLastCalledWith({
        type: "keyframeRequest",
        keyframeSeq: resyncSequence,
        ver: 1,
      });

      const keyframeCatalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));
      vi.advanceTimersByTime(50);
      network.emit({
        type: "keyframe",
        payload: {
          sequence: resyncSequence,
          config: {
            effectCatalog: keyframeCatalogPayload,
          },
        },
        receivedAt: Date.now() + 50,
      });

      vi.runOnlyPendingTimers();
      expect(sendSpy).toHaveBeenCalledTimes(1);

      const postResyncSpawn: ContractLifecycleSpawnEvent = {
        seq: 18,
        tick: resyncTick + 6,
        instance: {
          ...initialSpawn.instance,
          startTick: resyncTick + 6,
        },
      };

      vi.advanceTimersByTime(200);
      network.emit({
        type: "state",
        payload: {
          effect_spawned: [postResyncSpawn],
          effect_seq_cursors: { [postResyncSpawn.instance.id]: postResyncSpawn.seq },
          t: postResyncSpawn.tick,
          keyframeSeq: resyncSequence,
        },
        receivedAt: Date.now() + 50,
      });

      const resumedBatch = renderer.batches.at(-1)!;
      expect(resumedBatch.keyframeId).toBe(`tick-${postResyncSpawn.tick}`);
      const resumedFrames = resumedBatch.animations.filter((frame) => frame.effectId === postResyncSpawn.instance.id);
      expect(resumedFrames).toHaveLength(1);
      expect(resumedFrames[0].metadata).toMatchObject({
        entryId: "fireball",
        lastEventKind: "spawn",
      });
    } finally {
      sendSpy.mockRestore();
      vi.useRealTimers();
    }
  });

  test("requests keyframe once when patch sequence skips and clears after catching up", async () => {
    vi.useFakeTimers();
    vi.setSystemTime(0);

    const fireballEntry = generatedEffectCatalog.fireball;
    const fireballParameters = {
      ...(fireballEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const keyframeSequence = 48;
    const { network, renderer, orchestrator, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    const sendSpy = vi.spyOn(network, "send");

    try {
      const onReady = vi.fn();
      await orchestrator.boot({ onReady });
      expect(onReady).toHaveBeenCalledTimes(1);

      const spawnTick = 200;
      const effectId = "effect-fireball";
      const fireballSpawn: ContractLifecycleSpawnEvent = {
        seq: 1,
        tick: spawnTick,
        instance: {
          id: effectId,
          entryId: "fireball",
          definitionId: fireballEntry.contractId,
          definition: fireballEntry.definition,
          startTick: spawnTick,
          deliveryState: {
            geometry: {
              shape: "circle",
              radius: fireballParameters.radius ?? 12,
            },
            motion: {
              positionX: 256,
              positionY: 320,
              velocityX: 64,
              velocityY: 0,
            },
          },
          behaviorState: {
            ticksRemaining: fireballEntry.definition.lifetimeTicks,
            tickCadence: 1,
          },
          params: fireballParameters,
          colors: ["#ffaa33"],
          replication: fireballEntry.definition.client,
          end: fireballEntry.definition.end,
        },
      };

      emitLifecycleState({
        spawns: [fireballSpawn],
        cursors: { [effectId]: fireballSpawn.seq },
        tick: fireballSpawn.tick,
        sequence: fireballSpawn.seq,
        keyframeSequence,
        receivedAt: Date.now(),
      });

      vi.advanceTimersByTime(16);

      const updateTwo: ContractLifecycleUpdateEvent = {
        seq: 2,
        tick: fireballSpawn.tick + 1,
        id: effectId,
        deliveryState: {
          geometry: {
            shape: "circle",
            radius: fireballParameters.radius ?? 12,
          },
          motion: {
            positionX: 288,
            positionY: 320,
            velocityX: 64,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: fireballEntry.definition.lifetimeTicks - 1,
        },
      };

      emitLifecycleState({
        updates: [updateTwo],
        cursors: { [effectId]: updateTwo.seq },
        tick: updateTwo.tick,
        sequence: updateTwo.seq,
        keyframeSequence,
        receivedAt: Date.now(),
      });

      vi.advanceTimersByTime(16);

      emitLifecycleState({
        tick: (updateTwo.tick ?? fireballSpawn.tick) + 3,
        sequence: 5,
        keyframeSequence,
        receivedAt: Date.now(),
      });

      expect(sendSpy).toHaveBeenCalledTimes(1);
      expect(sendSpy).toHaveBeenLastCalledWith({
        type: "keyframeRequest",
        keyframeSeq: keyframeSequence,
        ver: 1,
      });

      const updateThree: ContractLifecycleUpdateEvent = {
        seq: 3,
        tick: fireballSpawn.tick + 2,
        id: effectId,
        deliveryState: {
          geometry: {
            shape: "circle",
            radius: fireballParameters.radius ?? 12,
          },
          motion: {
            positionX: 320,
            positionY: 320,
            velocityX: 64,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: fireballEntry.definition.lifetimeTicks - 2,
        },
      };

      vi.advanceTimersByTime(16);
      emitLifecycleState({
        updates: [updateThree],
        cursors: { [effectId]: updateThree.seq },
        tick: updateThree.tick,
        sequence: updateThree.seq,
        keyframeSequence,
        receivedAt: Date.now(),
      });

      const updateFour: ContractLifecycleUpdateEvent = {
        seq: 4,
        tick: fireballSpawn.tick + 3,
        id: effectId,
        deliveryState: {
          geometry: {
            shape: "circle",
            radius: fireballParameters.radius ?? 12,
          },
          motion: {
            positionX: 352,
            positionY: 320,
            velocityX: 64,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: fireballEntry.definition.lifetimeTicks - 3,
        },
      };

      vi.advanceTimersByTime(16);
      emitLifecycleState({
        updates: [updateFour],
        cursors: { [effectId]: updateFour.seq },
        tick: updateFour.tick,
        sequence: updateFour.seq,
        keyframeSequence,
        receivedAt: Date.now(),
      });

      const updateFive: ContractLifecycleUpdateEvent = {
        seq: 5,
        tick: fireballSpawn.tick + 4,
        id: effectId,
        deliveryState: {
          geometry: {
            shape: "circle",
            radius: fireballParameters.radius ?? 12,
          },
          motion: {
            positionX: 384,
            positionY: 320,
            velocityX: 64,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: fireballEntry.definition.lifetimeTicks - 4,
        },
      };

      vi.advanceTimersByTime(16);
      emitLifecycleState({
        updates: [updateFive],
        cursors: { [effectId]: updateFive.seq },
        tick: updateFive.tick,
        sequence: updateFive.seq,
        keyframeSequence,
        receivedAt: Date.now(),
      });

      vi.runOnlyPendingTimers();
      expect(sendSpy).toHaveBeenCalledTimes(1);

      const finalBatch = renderer.batches.at(-1)!;
      expect(finalBatch.keyframeId).toBe(`tick-${updateFive.tick}`);
      const finalFrames = finalBatch.animations.filter((frame) => frame.effectId === effectId);
      expect(finalFrames).toHaveLength(1);
      expect(finalFrames[0].metadata.lastEventKind).toBe("update");
      expect(finalFrames[0].metadata.instance.deliveryState.motion.positionX).toBe(
        updateFive.deliveryState!.motion!.positionX,
      );

      const geometry = finalBatch.staticGeometry.filter((entry) => entry.id === effectId);
      expect(geometry).toHaveLength(1);
    } finally {
      sendSpy.mockRestore();
      vi.useRealTimers();
    }
  });
});
