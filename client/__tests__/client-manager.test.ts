import { beforeEach, describe, expect, test, vi } from "vitest";

import { setEffectCatalog } from "../effect-catalog";
import { effectCatalog as generatedEffectCatalog } from "../generated/effect-contracts";
import {
  type ContractLifecycleSpawnEvent,
  type ContractLifecycleBatch,
} from "../effect-lifecycle-store";
import { createHeadlessHarness } from "./helpers/headless-harness";

const findStaticGeometry = (batch: { readonly staticGeometry: readonly { id: string; vertices: readonly [number, number][]; style?: unknown; }[] } | undefined, id: string) =>
  batch?.staticGeometry.find((entry) => entry.id === id) ?? null;

const computeVertexCentroid = (vertices: readonly [number, number][]) => {
  if (vertices.length === 0) {
    return { x: 0, y: 0 };
  }
  let sumX = 0;
  let sumY = 0;
  for (const [x, y] of vertices) {
    sumX += x;
    sumY += y;
  }
  return { x: sumX / vertices.length, y: sumY / vertices.length };
};

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
    const effectGeometry = finalBatch.staticGeometry.filter((entry) => !entry.id.startsWith("world/"));
    expect(effectGeometry[0]?.layer.id).toBe("effect-area");
    expect(effectGeometry[0]?.vertices.length).toBe(4);

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
    const { orchestrator, network, renderer, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const onPathCommand = vi.fn();
    const dispatcher = orchestrator.createInputDispatcher({ onPathCommand });

    expect(onPathCommand).toHaveBeenCalledTimes(1);
    expect(onPathCommand).toHaveBeenLastCalledWith({ active: false, target: null });

    dispatcher.sendAction("attack");
    dispatcher.sendPathCommand({ x: 320, y: 240 });
    expect(network.sentMessages).toHaveLength(0);
    expect(onPathCommand).toHaveBeenCalledTimes(2);
    expect(onPathCommand).toHaveBeenLastCalledWith({ active: true, target: { x: 320, y: 240 } });
    const initialBatch = renderer.batches.at(-1);
    expect(initialBatch?.pathTarget).toEqual({ x: 320, y: 240 });

    emitLifecycleState({ tick: 8, receivedAt: 1000 });

    expect(network.sentMessages).toHaveLength(2);
    expect(network.sentMessages[0]).toEqual({ type: "action", action: "attack", ver: 1, ack: 8, seq: 1 });
    expect(network.sentMessages[1]).toEqual({ type: "path", x: 320, y: 240, ver: 1, ack: 8, seq: 2 });
    dispatcher.handleCommandAck({ sequence: 1, tick: 8 });
    dispatcher.handleCommandAck({ sequence: 2, tick: 8 });

    dispatcher.sendAction("attack");
    expect(network.sentMessages).toHaveLength(3);
    expect(network.sentMessages[2]).toEqual({ type: "action", action: "attack", ver: 1, ack: 8, seq: 3 });
    dispatcher.handleCommandAck({ sequence: 3, tick: 8 });

    dispatcher.sendPathCommand({ x: 320, y: 240 });
    expect(network.sentMessages).toHaveLength(4);
    expect(network.sentMessages[3]).toEqual({ type: "path", x: 320, y: 240, ver: 1, ack: 8, seq: 4 });
    expect(onPathCommand).toHaveBeenCalledTimes(3);
    expect(onPathCommand).toHaveBeenLastCalledWith({ active: true, target: { x: 320, y: 240 } });
    dispatcher.handleCommandAck({ sequence: 4, tick: 8 });

    dispatcher.sendCurrentIntent({ dx: 1.2, dy: 0, facing: "right" });
    expect(network.sentMessages).toHaveLength(5);
    expect(network.sentMessages[4]).toEqual({
      type: "input",
      dx: 1,
      dy: 0,
      facing: "right",
      ver: 1,
      ack: 8,
      seq: 5,
    });
    dispatcher.handleCommandAck({ sequence: 5, tick: 8 });

    dispatcher.cancelPath();
    expect(network.sentMessages).toHaveLength(6);
    expect(network.sentMessages[5]).toEqual({ type: "cancelPath", ver: 1, ack: 8, seq: 6 });
    expect(onPathCommand).toHaveBeenCalledTimes(4);
    expect(onPathCommand).toHaveBeenLastCalledWith({ active: false, target: null });
    expect(renderer.batches.at(-1)?.pathTarget).toBeNull();
    dispatcher.handleCommandAck({ sequence: 6, tick: 8 });

    emitLifecycleState({ resync: true, receivedAt: 1500 });
    dispatcher.sendAction("attack");
    dispatcher.sendPathCommand({ x: 128, y: 96 });
    expect(network.sentMessages).toHaveLength(8);
    expect(network.sentMessages[6]).toEqual({ type: "action", action: "attack", ver: 1, seq: 7 });
    expect(network.sentMessages[6]).not.toHaveProperty("ack");
    expect(network.sentMessages[7]).toEqual({ type: "path", x: 128, y: 96, ver: 1, seq: 8 });
    expect(network.sentMessages[7]).not.toHaveProperty("ack");
    expect(onPathCommand).toHaveBeenCalledTimes(6);
    expect(onPathCommand).toHaveBeenNthCalledWith(5, { active: false, target: null });
    expect(onPathCommand).toHaveBeenNthCalledWith(6, { active: true, target: { x: 128, y: 96 } });
    expect(renderer.batches.at(-1)?.pathTarget).toEqual({ x: 128, y: 96 });

    emitLifecycleState({ tick: 11, receivedAt: 1600 });
    dispatcher.sendAction("attack");
    dispatcher.sendPathCommand({ x: 512, y: 256 });
    expect(network.sentMessages).toHaveLength(10);
    expect(network.sentMessages[8]).toEqual({ type: "action", action: "attack", ver: 1, ack: 11, seq: 9 });
    expect(network.sentMessages[9]).toEqual({ type: "path", x: 512, y: 256, ver: 1, ack: 11, seq: 10 });
    expect(onPathCommand).toHaveBeenCalledTimes(7);
    expect(onPathCommand).toHaveBeenNthCalledWith(7, { active: true, target: { x: 512, y: 256 } });
    expect(renderer.batches.at(-1)?.pathTarget).toEqual({ x: 512, y: 256 });

    await orchestrator.shutdown();
  });

  test("acknowledgement ticks advance dispatch metadata without waiting for state", async () => {
    const { orchestrator, network, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const dispatcher = orchestrator.createInputDispatcher();

    dispatcher.sendAction("attack");
    emitLifecycleState({ tick: 8, receivedAt: 1000 });

    expect(network.sentMessages).toHaveLength(1);
    expect(network.sentMessages[0]).toEqual({ type: "action", action: "attack", ver: 1, ack: 8, seq: 1 });

    network.emit({
      type: "commandAck",
      payload: { seq: 1, tick: 12 },
      receivedAt: 1100,
    });

    dispatcher.sendAction("attack");
    expect(network.sentMessages).toHaveLength(2);
    expect(network.sentMessages[1]).toEqual({ type: "action", action: "attack", ver: 1, ack: 12, seq: 2 });

    network.emit({
      type: "commandAck",
      payload: { seq: 2, tick: 4 },
      receivedAt: 1200,
    });

    dispatcher.sendAction("attack");
    expect(network.sentMessages).toHaveLength(3);
    expect(network.sentMessages[2]).toEqual({ type: "action", action: "attack", ver: 1, ack: 12, seq: 3 });

    await orchestrator.shutdown();
  });

  test("command rejection ticks update acknowledgement metadata", async () => {
    const { orchestrator, network, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const dispatcher = orchestrator.createInputDispatcher();

    dispatcher.sendPathCommand({ x: 64, y: 32 });
    emitLifecycleState({ tick: 5, receivedAt: 900 });

    expect(network.sentMessages).toHaveLength(1);
    expect(network.sentMessages[0]).toEqual({ type: "path", x: 64, y: 32, ver: 1, ack: 5, seq: 1 });

    network.emit({
      type: "commandReject",
      payload: { seq: 1, reason: "queue_limit", retry: false, tick: 9 },
      receivedAt: 950,
    });

    dispatcher.sendPathCommand({ x: 128, y: 96 });
    expect(network.sentMessages).toHaveLength(2);
    expect(network.sentMessages[1]).toEqual({ type: "path", x: 128, y: 96, ver: 1, ack: 9, seq: 2 });

    await orchestrator.shutdown();
  });

  test("replays pending commands immediately after resync and omits stale ack metadata", async () => {
    const { orchestrator, network, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const dispatcher = orchestrator.createInputDispatcher();

    dispatcher.sendAction("attack");
    emitLifecycleState({ tick: 5, receivedAt: 1000 });

    expect(network.sentMessages).toHaveLength(1);
    expect(network.sentMessages[0]).toEqual({ type: "action", action: "attack", ver: 1, ack: 5, seq: 1 });

    emitLifecycleState({ resync: true, receivedAt: 1500 });

    expect(network.sentMessages).toHaveLength(2);
    expect(network.sentMessages[1]).toEqual({ type: "action", action: "attack", ver: 1, seq: 1 });
    expect(network.sentMessages[1]).not.toHaveProperty("ack");

    network.emit({
      type: "commandAck",
      payload: { seq: 1, tick: 12 },
      receivedAt: 1600,
    });

    dispatcher.sendAction("attack");

    expect(network.sentMessages).toHaveLength(3);
    expect(network.sentMessages[2]).toEqual({ type: "action", action: "attack", ver: 1, ack: 12, seq: 2 });

    await orchestrator.shutdown();
  });

  test("retriable command rejections are retried without surfacing telemetry", async () => {
    vi.useFakeTimers();
    const { orchestrator, network, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const onCommandRejectionChanged = vi.fn();
    const dispatcher = orchestrator.createInputDispatcher({ onCommandRejectionChanged });

    try {
      dispatcher.sendAction("attack");
      emitLifecycleState({ tick: 3, receivedAt: 700 });

      expect(network.sentMessages).toHaveLength(1);
      expect(network.sentMessages[0]).toEqual({ type: "action", action: "attack", ver: 1, ack: 3, seq: 1 });

      network.emit({
        type: "commandReject",
        payload: { seq: 1, reason: "queue_limit", retry: true, tick: 7 },
        receivedAt: 750,
      });

      expect(onCommandRejectionChanged).not.toHaveBeenCalled();
      expect(network.sentMessages).toHaveLength(1);

      vi.advanceTimersByTime(50);

      expect(network.sentMessages).toHaveLength(2);
      expect(network.sentMessages[1]).toEqual({ type: "action", action: "attack", ver: 1, ack: 7, seq: 1 });
    } finally {
      await orchestrator.shutdown();
      vi.useRealTimers();
    }
  });

  test("forwards non-retryable command rejections through hooks", async () => {
    const { orchestrator, network, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const onCommandRejectionChanged = vi.fn();
    const dispatcher = orchestrator.createInputDispatcher({ onCommandRejectionChanged });

    dispatcher.sendPathCommand({ x: 48, y: 32 });
    emitLifecycleState({ tick: 4, receivedAt: 800 });

    network.emit({
      type: "commandReject",
      payload: { seq: 1, reason: "queue_limit", retry: false, tick: 6 },
      receivedAt: 850,
    });

    expect(onCommandRejectionChanged).toHaveBeenLastCalledWith({
      sequence: 1,
      reason: "queue_limit",
      retry: false,
      tick: 6,
      kind: "path",
    });

    dispatcher.sendPathCommand({ x: 96, y: 64 });
    network.emit({
      type: "commandAck",
      payload: { seq: 2, tick: 10 },
      receivedAt: 900,
    });

    expect(onCommandRejectionChanged).toHaveBeenLastCalledWith(null);

    await orchestrator.shutdown();
  });

  test("resync clears stored command rejections", async () => {
    const { orchestrator, network, emitLifecycleState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
    });

    await orchestrator.boot({});
    const onCommandRejectionChanged = vi.fn();
    const dispatcher = orchestrator.createInputDispatcher({ onCommandRejectionChanged });

    dispatcher.sendPathCommand({ x: 48, y: 32 });
    emitLifecycleState({ tick: 4, receivedAt: 800 });

    expect(network.sentMessages).toHaveLength(1);
    expect(network.sentMessages[0]).toEqual({ type: "path", x: 48, y: 32, ver: 1, ack: 4, seq: 1 });

    network.emit({
      type: "commandReject",
      payload: { seq: 1, reason: "queue_limit", retry: false, tick: 6 },
      receivedAt: 850,
    });

    expect(onCommandRejectionChanged).toHaveBeenLastCalledWith({
      sequence: 1,
      reason: "queue_limit",
      retry: false,
      tick: 6,
      kind: "path",
    });

    emitLifecycleState({ resync: true, receivedAt: 900 });

    expect(onCommandRejectionChanged).toHaveBeenLastCalledWith(null);

    await orchestrator.shutdown();
  });

  test("applies world patches to update actor geometry", async () => {
    const { orchestrator, renderer, emitLifecycleState, worldState } = createHeadlessHarness({
      catalog: generatedEffectCatalog,
      joinResponseOverrides: {
        players: [
          {
            id: "player-1",
            x: 12,
            y: 18,
            facing: "up",
          },
        ],
        npcs: [
          {
            id: "npc-1",
            x: 24,
            y: 32,
            type: "goblin",
            health: 10,
            maxHealth: 12,
          },
        ],
      },
    });

    await orchestrator.boot({});

    const initialBatch = renderer.batches.at(-1);
    expect(initialBatch).toBeDefined();

    const initialPlayerGeometry = findStaticGeometry(initialBatch, "world/player/player-1");
    expect(initialPlayerGeometry).not.toBeNull();
    const initialPlayerCenter = computeVertexCentroid(initialPlayerGeometry!.vertices);
    expect(initialPlayerCenter.x).toBeCloseTo(12);
    expect(initialPlayerCenter.y).toBeCloseTo(18);

    const initialNpcGeometry = findStaticGeometry(initialBatch, "world/npc/npc-1");
    expect(initialNpcGeometry).not.toBeNull();
    const initialNpcCenter = computeVertexCentroid(initialNpcGeometry!.vertices);
    expect(initialNpcCenter.x).toBeCloseTo(24);
    expect(initialNpcCenter.y).toBeCloseTo(32);

    const patchPayload: Record<string, unknown> = {
      patches: [
        { kind: "player_pos", entityId: "player-1", payload: { x: 40, y: 44 } },
        { kind: "player_facing", entityId: "player-1", payload: { facing: "left" } },
        { kind: "player_intent", entityId: "player-1", payload: { dx: -1, dy: 0 } },
        { kind: "npc_pos", entityId: "npc-1", payload: { x: 52, y: 60 } },
        { kind: "npc_health", entityId: "npc-1", payload: { health: 6, maxHealth: 12 } },
      ],
    };

    emitLifecycleState({
      payload: patchPayload,
      tick: 20,
      receivedAt: 20 * 16,
    });

    const updatedSnapshot = worldState.snapshot();
    const playerState = updatedSnapshot.entities.get("player-1");
    expect(playerState?.position).toEqual([40, 44]);
    expect(playerState?.facing).toBe("left");
    expect(playerState?.intent).toEqual({ dx: -1, dy: 0 });

    const npcState = updatedSnapshot.entities.get("npc-1");
    expect(npcState?.position).toEqual([52, 60]);
    expect(npcState?.health).toBe(6);
    expect(npcState?.maxHealth).toBe(12);

    const finalBatch = renderer.batches.at(-1);
    expect(finalBatch).toBeDefined();

    const updatedPlayerGeometry = findStaticGeometry(finalBatch, "world/player/player-1");
    expect(updatedPlayerGeometry).not.toBeNull();
    const playerCenter = computeVertexCentroid(updatedPlayerGeometry!.vertices);
    expect(playerCenter.x).toBeCloseTo(40);
    expect(playerCenter.y).toBeCloseTo(44);
    expect((updatedPlayerGeometry!.style as Record<string, unknown>).facing).toBe("left");
    expect((updatedPlayerGeometry!.style as Record<string, unknown>).intent).toEqual({ dx: -1, dy: 0 });

    const updatedNpcGeometry = findStaticGeometry(finalBatch, "world/npc/npc-1");
    expect(updatedNpcGeometry).not.toBeNull();
    const npcCenter = computeVertexCentroid(updatedNpcGeometry!.vertices);
    expect(npcCenter.x).toBeCloseTo(52);
    expect(npcCenter.y).toBeCloseTo(60);
    expect((updatedNpcGeometry!.style as Record<string, unknown>).health).toBe(6);
    expect((updatedNpcGeometry!.style as Record<string, unknown>).maxHealth).toBe(12);

    await orchestrator.shutdown();
  });
});
