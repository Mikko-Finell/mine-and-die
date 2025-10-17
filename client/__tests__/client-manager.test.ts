import { beforeEach, describe, expect, test, vi } from "vitest";

import { setEffectCatalog } from "../effect-catalog";
import { effectCatalog as generatedEffectCatalog } from "../generated/effect-contracts";
import {
  type ContractLifecycleSpawnEvent,
  type ContractLifecycleBatch,
} from "../effect-lifecycle-store";
import { createHeadlessHarness } from "./helpers/headless-harness";

describe("GameClientOrchestrator", () => {
  beforeEach(() => {
    setEffectCatalog(null);
  });

  test("hydrates lifecycle store and renders batches from catalog metadata", async () => {
    const { network, renderer, orchestrator } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
      rendererConfiguration: {
        layers: [
          { id: "effect-area", zIndex: 1 },
          { id: "effect-visual", zIndex: 2 },
        ],
        dimensions: {
          width: 800,
          height: 600,
        },
      },
    });

    const ready = vi.fn();
    await orchestrator.boot({ onReady: ready });
    expect(ready).toHaveBeenCalledTimes(1);

    const attackEntry = generatedEffectCatalog.attack;
    const spawn: ContractLifecycleSpawnEvent = {
      seq: 1,
      tick: 10,
      instance: {
        id: "effect-attack",
        entryId: "attack",
        definitionId: attackEntry.contractId,
        definition: attackEntry.definition,
        startTick: 10,
        deliveryState: {
          geometry: {
            shape: "rect",
            width: 40,
            height: 24,
          },
          motion: {
            positionX: 128,
            positionY: 256,
            velocityX: 0,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: 6,
        },
        params: attackEntry.blocks.parameters as Readonly<Record<string, number>>,
        replication: attackEntry.definition.client,
        end: attackEntry.definition.end,
      },
    };

    const batch: ContractLifecycleBatch = {
      spawns: [spawn],
      cursors: { "effect-attack": 1 },
    };

    network.emit({
      type: "state",
      payload: {
        effect_spawned: batch.spawns,
        effect_seq_cursors: batch.cursors,
        t: 10,
      },
      receivedAt: 500,
    });

    expect(renderer.batches.length).toBeGreaterThanOrEqual(2);
    const rendered = renderer.batches.at(-1);
    expect(rendered).toBeDefined();
    const finalBatch = rendered!;
    expect(finalBatch.animations.length).toBeGreaterThan(0);
    const animation = finalBatch.animations[0];
    expect(animation.metadata).toMatchObject({
      entryId: "attack",
      managedByClient: attackEntry.managedByClient,
    });
    expect(animation.metadata.catalog).toMatchObject({
      contractId: attackEntry.contractId,
    });
    expect(finalBatch.staticGeometry[0]?.layer.id).toBe("effect-area");
    expect(finalBatch.staticGeometry[0]?.vertices.length).toBe(4);

    network.emit({
      type: "state",
      payload: {
        resync: true,
      },
      receivedAt: 750,
    });

    const cleared = renderer.batches.at(-1);
    expect(cleared).toBeDefined();
    expect(cleared!.animations.length).toBe(0);
  });

  test("input dispatcher attaches metadata, respects pause, and notifies hooks", async () => {
    const { orchestrator, network, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const onPathCommand = vi.fn();
    const dispatcher = orchestrator.createInputDispatcher({ onPathCommand });

    dispatcher.sendAction("attack");
    expect(network.sentMessages).toHaveLength(0);

    emitLifecycleState({ tick: 8, receivedAt: 1000 });

    dispatcher.sendAction("attack");
    expect(network.sentMessages).toHaveLength(1);
    expect(network.sentMessages[0]).toEqual({ type: "action", action: "attack", ver: 1, ack: 8 });

    dispatcher.sendCurrentIntent({ dx: 1.2, dy: 0, facing: "right" });
    expect(network.sentMessages).toHaveLength(2);
    expect(network.sentMessages[1]).toEqual({
      type: "input",
      dx: 1,
      dy: 0,
      facing: "right",
      ver: 1,
      ack: 8,
    });

    dispatcher.cancelPath();
    expect(network.sentMessages).toHaveLength(3);
    expect(network.sentMessages[2]).toEqual({ type: "cancelPath", ver: 1, ack: 8 });
    expect(onPathCommand).toHaveBeenCalledWith(false);

    emitLifecycleState({ resync: true, receivedAt: 1500 });
    dispatcher.sendAction("attack");
    expect(network.sentMessages).toHaveLength(4);
    expect(network.sentMessages[3]).toEqual({ type: "action", action: "attack", ver: 1 });
    expect(network.sentMessages[3]).not.toHaveProperty("ack");

    emitLifecycleState({ tick: 11, receivedAt: 1600 });
    dispatcher.sendAction("attack");
    expect(network.sentMessages).toHaveLength(5);
    expect(network.sentMessages[4]).toEqual({ type: "action", action: "attack", ver: 1, ack: 11 });

    await orchestrator.shutdown();
  });
});
