import { beforeEach, describe, expect, test, vi } from "vitest";

import { EffectLayer } from "@js-effects/effects-lib";

import { GameClientOrchestrator } from "../client-manager";
import * as effectCatalogStore from "../effect-catalog";
import {
  effectCatalog as generatedEffectCatalog,
  type DeliveryKind,
} from "../generated/effect-contracts";
import {
  type ContractLifecycleBatch,
  type ContractLifecycleEndEvent,
  type ContractLifecycleSpawnEvent,
  type ContractLifecycleUpdateEvent,
} from "../effect-lifecycle-store";
import { CanvasRenderer, validateRenderLayers, type StaticGeometry } from "../render";
import { InMemoryWorldStateStore } from "../world-state";
import {
  createHeadlessHarness,
  createJoinResponse,
  defaultNetworkConfiguration,
  defaultOrchestratorConfiguration,
  defaultRendererConfiguration,
  HeadlessNetworkClient,
} from "./helpers/headless-harness";

const expectedRuntimeLayerByDelivery: Record<DeliveryKind, number> = {
  area: EffectLayer.ActorOverlay,
  target: EffectLayer.ActorOverlay,
  visual: EffectLayer.GroundDecal,
};

const filterEffectGeometry = (geometry: readonly StaticGeometry[]): StaticGeometry[] =>
  geometry.filter((entry) => !entry.id.startsWith("world/"));

const filterWorldGeometry = (geometry: readonly StaticGeometry[]): StaticGeometry[] =>
  geometry.filter((entry) => entry.id.startsWith("world/"));

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

    expect(() => validateRenderLayers(renderer.configuration.layers)).not.toThrow();
    const misorderedLayers = renderer.configuration.layers.map((layer) => {
      if (layer.id === "effect-area") {
        return { ...layer, zIndex: layer.zIndex + 10 };
      }
      if (layer.id === "effect-target") {
        return { ...layer, zIndex: layer.zIndex - 10 };
      }
      return { ...layer };
    });
    expect(() => validateRenderLayers(misorderedLayers)).toThrowError(/runtime layer ordering/);

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
    expect(filterWorldGeometry(activeBatch.staticGeometry).length).toBeGreaterThan(0);
    expect(filterEffectGeometry(activeBatch.staticGeometry).length).toBeGreaterThan(0);
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

    expect(activeBatch.runtimeEffects).toBeDefined();
    const fireballRuntime = activeBatch.runtimeEffects!.find((entry) => entry.effectId === "effect-fireball");
    expect(fireballRuntime).toBeDefined();
    expect(fireballRuntime!.intent).not.toBeNull();
    expect(fireballRuntime!.intent!.definition.type).toBe("fireball");
    expect(fireballRuntime!.intent!.state).toBe("active");
    expect(fireballRuntime!.intent!.retained).toBe(false);
    const fireballInstance = fireballRuntime!.intent!.definition.create(
      fireballRuntime!.intent!.options,
    );
    expect(fireballInstance.layer).toBe(
      expectedRuntimeLayerByDelivery[fireballEntry.definition.delivery as DeliveryKind],
    );
    fireballInstance.dispose?.();

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

    const attackRuntime = activeBatch.runtimeEffects!.find((entry) => entry.effectId === "effect-attack");
    expect(attackRuntime).toBeDefined();
    expect(attackRuntime!.intent).not.toBeNull();
    expect(attackRuntime!.intent!.definition.type).toBe("melee-swing");
    expect(attackRuntime!.intent!.state).toBe("active");
    expect(attackRuntime!.intent!.retained).toBe(false);
    const attackInstance = attackRuntime!.intent!.definition.create(
      attackRuntime!.intent!.options,
    );
    expect(attackInstance.layer).toBe(
      expectedRuntimeLayerByDelivery[attackEntry.definition.delivery as DeliveryKind],
    );
    attackInstance.dispose?.();

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
    expect(filterEffectGeometry(endedBatch.staticGeometry).length).toBeGreaterThan(0);
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

    const endedRuntime = endedBatch.runtimeEffects!.find((entry) => entry.effectId === "effect-fireball");
    expect(endedRuntime).toBeDefined();
    expect(endedRuntime!.intent).not.toBeNull();
    expect(endedRuntime!.intent!.state).toBe("ended");
    expect(endedRuntime!.intent!.retained).toBe(false);

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

    const retainedRuntime = endedBatch.runtimeEffects!.find((entry) => entry.effectId === "effect-attack");
    expect(retainedRuntime).toBeDefined();
    expect(retainedRuntime!.intent).not.toBeNull();
    expect(retainedRuntime!.intent!.state).toBe("ended");
    expect(retainedRuntime!.intent!.retained).toBe(true);

    const resyncTick = 130;

    network.emit({
      type: "state",
      payload: {
        resync: true,
        t: resyncTick,
      },
      receivedAt: 3600,
    });

    expect(renderer.batches.length).toBeGreaterThanOrEqual(4);
    const resyncBatch = renderer.batches.at(-1)!;
    expect(resyncBatch.keyframeId).toBe("lifecycle-0");
    expect(resyncBatch.time).toBe(resyncTick * 16);
    expect(filterEffectGeometry(resyncBatch.staticGeometry)).toHaveLength(0);
    expect(resyncBatch.animations).toHaveLength(0);
    expect(resyncBatch.runtimeEffects).toHaveLength(0);

    const activeCatalog = effectCatalogStore.getEffectCatalog();
    expect(activeCatalog).toEqual(generatedEffectCatalog);
    expect(Object.isFrozen(activeCatalog)).toBe(true);
  });

  test("renders keyframe responses using the local catalog snapshot", async () => {
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

    const resyncTick = 400;

    network.emit({
      type: "keyframeNack",
      payload: {
        resync: true,
      },
      receivedAt: 2600,
    });

    lastBatch = renderer.batches.at(-1)!;
    expect(lastBatch.keyframeId).toBe("lifecycle-0");
    expect(filterEffectGeometry(lastBatch.staticGeometry)).toHaveLength(0);
    expect(lastBatch.animations).toHaveLength(0);

    network.emit({
      type: "state",
      payload: {
        resync: true,
        t: resyncTick,
      },
      receivedAt: 3000,
    });

    lastBatch = renderer.batches.at(-1)!;
    expect(lastBatch.keyframeId).toBe("lifecycle-0");
    expect(filterEffectGeometry(lastBatch.staticGeometry)).toHaveLength(0);
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
      expect(filterEffectGeometry(clearedBatch.staticGeometry)).toHaveLength(0);
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

  test("clears lifecycle state when the network disconnects after rendering", async () => {
    const fireballEntry = generatedEffectCatalog.fireball;
    const fireballParameters = {
      ...(fireballEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const { network, renderer, orchestrator, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    const onReady = vi.fn();
    const onError = vi.fn();
    await orchestrator.boot({ onReady, onError });
    expect(onReady).toHaveBeenCalledTimes(1);

    const spawnTick = 48;
    const fireballSpawn: ContractLifecycleSpawnEvent = {
      seq: 1,
      tick: spawnTick,
      instance: {
        id: "effect-fireball",
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
            positionX: 320,
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

    emitLifecycleState({
      spawns: [fireballSpawn],
      cursors: { "effect-fireball": fireballSpawn.seq },
      tick: fireballSpawn.tick,
      sequence: fireballSpawn.seq,
      receivedAt: Date.now(),
    });

    expect(renderer.batches.length).toBeGreaterThanOrEqual(2);
    const activeBatch = renderer.batches.at(-1)!;
    expect(activeBatch.keyframeId).toBe(`tick-${spawnTick}`);
    expect(filterWorldGeometry(activeBatch.staticGeometry).length).toBeGreaterThan(0);
    expect(filterEffectGeometry(activeBatch.staticGeometry).length).toBeGreaterThan(0);
    expect(activeBatch.animations.length).toBeGreaterThan(0);

    const batchesBeforeDisconnect = renderer.batches.length;
    network.simulateDisconnect(4000, "closing-time");

    expect(onError).toHaveBeenCalledTimes(1);
    const disconnectError = onError.mock.calls.at(-1)?.at(0);
    expect(disconnectError).toBeInstanceOf(Error);
    expect((disconnectError as Error).message).toBe("Disconnected from server (closing-time) [4000]");

    expect(renderer.batches.length).toBe(batchesBeforeDisconnect + 1);
    const disconnectedBatch = renderer.batches.at(-1)!;
    expect(disconnectedBatch.keyframeId).toBe("lifecycle-0");
    expect(filterEffectGeometry(disconnectedBatch.staticGeometry)).toHaveLength(0);
    expect(disconnectedBatch.animations).toHaveLength(0);
    expect(disconnectedBatch.pathTarget).toBeNull();
  });

  test("disposes runtime instances on resync and disconnect replays", async () => {
    const fireballEntry = generatedEffectCatalog.fireball;
    const fireballParameters = {
      ...(fireballEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const attackEntry = generatedEffectCatalog.attack;
    expect(attackEntry.managedByClient).toBe(true);
    const attackParameters = {
      ...(attackEntry.blocks.parameters as Record<string, number> | undefined),
    } as Readonly<Record<string, number>>;
    const attackReach = attackParameters.reach ?? 56;
    const attackWidth = attackParameters.width ?? 40;
    const attackLifetimeTicks = Math.max(1, attackEntry.definition.lifetimeTicks ?? 0);
    const renderer = new CanvasRenderer({
      dimensions: { ...defaultRendererConfiguration.dimensions },
      layers: defaultRendererConfiguration.layers.map((layer) => ({ ...layer })),
    });
    const fireballEffectId = "effect-fireball";
    const attackEffectId = "effect-attack";
    const fireballRuntimeInstance = { effectId: fireballEffectId };
    const attackRuntimeInstance = { effectId: attackEffectId };
    const effectManager = {
      spawn: vi.fn((_definition: unknown, options: Record<string, unknown>) => {
        const effectId = options.effectId as string | undefined;
        if (effectId === fireballEffectId) {
          return fireballRuntimeInstance;
        }
        if (effectId === attackEffectId) {
          return attackRuntimeInstance;
        }
        return {};
      }),
      removeInstance: vi.fn(),
      clear: vi.fn(),
      cullByAABB: vi.fn(),
      updateAll: vi.fn(),
      drawAll: vi.fn(),
    };
    const rendererInternals = renderer as unknown as {
      effectManager: typeof effectManager;
      activeEffects: Map<string, { retained: boolean }>;
    };
    rendererInternals.effectManager = effectManager;
    const activeEffects = rendererInternals.activeEffects;

    const joinResponse = createJoinResponse(generatedEffectCatalog);
    const network = new HeadlessNetworkClient(
      { ...defaultNetworkConfiguration },
      joinResponse,
    );
    const worldState = new InMemoryWorldStateStore();
    const orchestrator = new GameClientOrchestrator(
      { ...defaultOrchestratorConfiguration },
      { network, renderer, worldState },
    );

    const emitFireballSpawn = (seq: number, tick: number): void => {
      const spawn: ContractLifecycleSpawnEvent = {
        seq,
        tick,
        instance: {
          id: fireballEffectId,
          entryId: "fireball",
          definitionId: fireballEntry.contractId,
          definition: fireballEntry.definition,
          startTick: tick,
          deliveryState: {
            geometry: {
              shape: "circle",
              radius: fireballParameters.radius ?? 12,
              offsetX: 0,
              offsetY: 0,
            },
            motion: {
              positionX: 256,
              positionY: 320,
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
          effect_spawned: [spawn],
          effect_seq_cursors: { [fireballEffectId]: seq },
          t: tick,
        },
        receivedAt: tick * 16,
      });
    };

    const emitAttackSpawn = (seq: number, tick: number): void => {
      const spawn: ContractLifecycleSpawnEvent = {
        seq,
        tick,
        instance: {
          id: attackEffectId,
          entryId: "attack",
          definitionId: attackEntry.contractId,
          definition: attackEntry.definition,
          startTick: tick,
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
            ticksRemaining: attackLifetimeTicks,
            tickCadence: 1,
          },
          params: attackParameters,
          colors: ["#ffffff"],
          replication: attackEntry.definition.client,
          end: attackEntry.definition.end,
        },
      };

      network.emit({
        type: "state",
        payload: {
          effect_spawned: [spawn],
          effect_seq_cursors: { [attackEffectId]: seq },
          t: tick,
        },
        receivedAt: tick * 16,
      });
    };

    const emitAttackEnd = (seq: number, tick: number): void => {
      const endEvent: ContractLifecycleEndEvent = {
        seq,
        tick,
        id: attackEffectId,
      };

      network.emit({
        type: "state",
        payload: {
          effect_ended: [endEvent],
          effect_seq_cursors: { [attackEffectId]: seq },
          t: tick,
        },
        receivedAt: tick * 16,
      });
    };

    try {
      const onReady = vi.fn();
      await orchestrator.boot({ onReady });
      expect(onReady).toHaveBeenCalledTimes(1);

      emitFireballSpawn(1, 120);
      emitAttackSpawn(1, 120);
      expect(effectManager.spawn).toHaveBeenCalledTimes(2);
      expect(activeEffects.size).toBe(2);

      emitAttackEnd(2, 124);
      expect(effectManager.removeInstance).not.toHaveBeenCalled();
      expect(activeEffects.get(attackEffectId)?.retained).toBe(true);

      const resyncCatalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));
      network.emit({
        type: "state",
        payload: {
          resync: true,
          t: 180,
          config: { effectCatalog: resyncCatalogPayload },
        },
        receivedAt: 3600,
      });

      expect(effectManager.removeInstance).toHaveBeenCalledTimes(2);
      expect(new Set(effectManager.removeInstance.mock.calls.map((call) => call[0]))).toEqual(
        new Set([fireballRuntimeInstance, attackRuntimeInstance]),
      );
      expect(activeEffects.size).toBe(0);

      emitAttackSpawn(3, 200);
      emitAttackEnd(4, 204);
      expect(effectManager.spawn).toHaveBeenCalledTimes(3);
      expect(activeEffects.size).toBe(1);
      expect(activeEffects.get(attackEffectId)?.retained).toBe(true);

      network.simulateDisconnect();
      expect(effectManager.removeInstance).toHaveBeenCalledTimes(3);
      expect(activeEffects.size).toBe(0);
    } finally {
      await orchestrator.shutdown();
    }
  });
});
