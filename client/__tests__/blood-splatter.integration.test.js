import { describe, expect, test, afterEach, beforeEach } from "vitest";
import { applyStateSnapshot } from "../network.js";
import { applyEffectLifecycleBatch, peekEffectLifecycleState } from "../effect-lifecycle.js";
import { startRenderLoop, RENDER_MODE_SNAPSHOT } from "../render.js";
import { EffectManager } from "../js-effects/manager.js";

function createMockContext() {
  const noop = () => {};
  return {
    save: noop,
    restore: noop,
    translate: noop,
    fillRect: noop,
    strokeRect: noop,
    beginPath: noop,
    moveTo: noop,
    lineTo: noop,
    stroke: noop,
    fill: noop,
    arc: noop,
    ellipse: noop,
    quadraticCurveTo: noop,
    fillText: noop,
    drawImage: noop,
    closePath: noop,
    clearRect: noop,
    set lineWidth(value) {},
    set fillStyle(value) {},
    set strokeStyle(value) {},
    set font(value) {},
    set textAlign(value) {},
    set textBaseline(value) {},
    set globalAlpha(value) {},
  };
}

function createStore() {
  return {
    renderMode: RENDER_MODE_SNAPSHOT,
    players: {},
    displayPlayers: {},
    npcs: {},
    displayNPCs: {},
    effects: [],
    groundItems: {},
    obstacles: [],
    pendingEffectTriggers: [],
    processedEffectTriggerIds: new Set(),
    effectManager: null,
    effectInstancesById: null,
    camera: { x: 0, y: 0, lockOnPlayer: true },
    canvas: { width: 800, height: 600 },
    ctx: createMockContext(),
    TILE_SIZE: 40,
    PLAYER_SIZE: 28,
    PLAYER_HALF: 14,
    LERP_RATE: 12,
    WORLD_WIDTH: 800,
    WORLD_HEIGHT: 600,
    GRID_WIDTH: 20,
    GRID_HEIGHT: 15,
    updateDiagnostics: () => {},
    setStatusBase: () => {},
    setLatency: () => {},
    renderInventory: null,
    patchState: null,
    lastTimestamp: performance.now(),
  };
}

const BLOOD_EFFECT_ID = "contract-blood-1";

function createContractSnapshotPayload() {
  const centerXWorld = 240;
  const centerYWorld = 200;
  const widthWorld = 40;
  const heightWorld = 40;
  const quantize = (value) => Math.floor((value / 40) * 16);
  const centerXQuant = quantize(centerXWorld);
  const centerYQuant = quantize(centerYWorld);
  const widthQuant = quantize(widthWorld);
  const heightQuant = quantize(heightWorld);

  return {
    type: "state",
    players: [
      { id: "player-1", x: 180, y: 200, facing: "right", health: 20, maxHealth: 20 },
    ],
    npcs: [
      { id: "npc-1", x: centerXWorld, y: centerYWorld, facing: "left", health: 10, maxHealth: 10, type: "goblin" },
    ],
    effects: [
      {
        id: BLOOD_EFFECT_ID,
        type: "blood-splatter",
        owner: "player-1",
        start: 1000,
        duration: 1200,
        x: centerXWorld - widthWorld / 2,
        y: centerYWorld - heightWorld / 2,
        width: widthWorld,
        height: heightWorld,
        params: {
          dropletRadius: 3,
          drag: 0.92,
          speed: 1,
          spawnInterval: 1.1,
          minDroplets: 4,
          maxDroplets: 8,
          minStainRadius: 4,
          maxStainRadius: 6,
          maxStains: 120,
          maxBursts: 1,
        },
      },
    ],
    effect_spawned: [
      {
        tick: 100,
        seq: 1,
        instance: {
          id: BLOOD_EFFECT_ID,
          definitionId: "blood-splatter",
          startTick: 100,
          deliveryState: {
            geometry: {
              width: widthQuant,
              height: heightQuant,
              offsetX: 0,
              offsetY: 0,
            },
            motion: {
              positionX: centerXQuant,
              positionY: centerYQuant,
            },
          },
          behaviorState: {
            ticksRemaining: 18,
            extra: {
              centerX: centerXQuant,
              centerY: centerYQuant,
            },
          },
          ownerActorId: "player-1",
          replication: {
            sendSpawn: true,
            sendUpdates: false,
            sendEnd: true,
          },
        },
      },
    ],
    effectTriggers: [],
    t: 100,
    sequence: 10,
  };
}

function createContractEndPayload() {
  return {
    type: "state",
    effect_ended: [
      {
        tick: 140,
        seq: 2,
        id: BLOOD_EFFECT_ID,
        reason: "expired",
      },
    ],
    effect_seq_cursors: {
      [BLOOD_EFFECT_ID]: 2,
    },
  };
}

function drainRaf(queue, stepMs) {
  if (queue.length === 0) {
    throw new Error("no scheduled frame to run");
  }
  const callback = queue.shift();
  drainRaf.now += stepMs;
  callback(drainRaf.now);
}

beforeEach(() => {
  drainRaf.now = 0;
});

afterEach(() => {
  drainRaf.now = 0;
  delete globalThis.requestAnimationFrame;
});

describe("blood splatter lifecycle integration", () => {
  test("renders contract-managed blood splatter through to expiry", () => {
    const store = createStore();
    store.playerId = "player-1";
    store.pendingEffectTriggers = [];
    store.processedEffectTriggerIds = new Set();

    const payload = createContractSnapshotPayload();
    const snapshot = applyStateSnapshot(store, payload, null);
    store.players = snapshot.players;
    store.npcs = snapshot.npcs;
    store.effects = snapshot.effects;
    store.groundItems = snapshot.groundItems;
    store.lastTick = snapshot.lastTick;

    const lifecycleSummary = applyEffectLifecycleBatch(store, payload);
    expect(lifecycleSummary.spawns).toContain(BLOOD_EFFECT_ID);

    const lifecycle = peekEffectLifecycleState(store);
    expect(lifecycle).not.toBeNull();
    expect(lifecycle.instances.has(BLOOD_EFFECT_ID)).toBe(true);

    const rafQueue = [];
    globalThis.requestAnimationFrame = (cb) => {
      rafQueue.push(cb);
      return rafQueue.length;
    };

    startRenderLoop(store);

    drainRaf(rafQueue, 16);

    const manager = store.effectManager;
    expect(manager).toBeInstanceOf(EffectManager);
    const tracked = manager.getTrackedInstances("blood-splatter");
    // At present this assertion fails because render.js never mirrors
    // contract-derived "blood-splatter" lifecycle entries into the
    // EffectManager. prepareEffectPass only calls syncEffectsByType for
    // melee, fire, and fireball buckets, so the manager never receives the
    // splatter instance. We keep the expectation as the desired behaviour and
    // document the gap here rather than papering over it.
    expect(tracked.size).toBeGreaterThan(0);

    drainRaf(rafQueue, 16);
    drainRaf(rafQueue, 16);
    drainRaf(rafQueue, 1600);

    const remaining = manager.getTrackedInstances("blood-splatter");
    expect(Array.from(remaining.values()).every((instance) => instance.finished)).toBe(true);

    const endPayload = createContractEndPayload();
    const endSummary = applyEffectLifecycleBatch(store, endPayload);
    expect(endSummary.ends).toContain(BLOOD_EFFECT_ID);

    drainRaf(rafQueue, 16);

    const finalTracked = manager.getTrackedInstances("blood-splatter");
    expect(finalTracked.size).toBe(0);
  });
});
