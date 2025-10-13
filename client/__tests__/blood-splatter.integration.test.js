import { describe, expect, test, afterEach, beforeEach } from "vitest";
import { applyStateSnapshot } from "../network.js";
import { applyEffectLifecycleBatch, peekEffectLifecycleState } from "../effect-lifecycle.js";
import { startRenderLoop, RENDER_MODE_SNAPSHOT } from "../render.js";
import { EffectManager } from "../js-effects/manager.js";

const originalSpawn = EffectManager.prototype.spawn;
let renderInvocations = [];

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
    scale: noop,
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
    PLAYER_SIZE: PLAYER_HALF * 2,
    PLAYER_HALF,
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

const BLOOD_EFFECT_ID = "contract-effect-2";
const MELEE_EFFECT_ID = "contract-effect-3";
const FIRE_EFFECT_ID = "contract-effect-4";
const FIREBALL_EFFECT_ID = "contract-effect-5";

const TILE_SIZE = 40;
const COORD_SCALE = 16;
const PLAYER_HALF = 14;

const quantizeWorldCoord = (value) =>
  Math.round((value / TILE_SIZE) * COORD_SCALE);

function createContractSnapshotPayload() {
  const playerX = 180;
  const playerY = 200;
  const playerFacing = "right";

  const bloodCenterXWorld = 240;
  const bloodCenterYWorld = 200;
  const bloodFootprintWorld = 28;
  const bloodCenterXQuant = quantizeWorldCoord(bloodCenterXWorld);
  const bloodCenterYQuant = quantizeWorldCoord(bloodCenterYWorld);
  const bloodFootprintQuant = quantizeWorldCoord(bloodFootprintWorld);

  const meleeReach = 56;
  const meleeWidth = 40;
  const meleeCenterXWorld = playerX + PLAYER_HALF + meleeReach / 2;
  const meleeCenterYWorld = playerY;
  const meleeCenterXQuant = quantizeWorldCoord(meleeCenterXWorld);
  const meleeCenterYQuant = quantizeWorldCoord(meleeCenterYWorld);
  const meleeWidthQuant = quantizeWorldCoord(meleeReach);
  const meleeHeightQuant = quantizeWorldCoord(meleeWidth);
  const meleeOffsetXQuant = quantizeWorldCoord(meleeCenterXWorld - playerX);
  const meleeOffsetYQuant = quantizeWorldCoord(meleeCenterYWorld - playerY);

  const fireLifetimeTicks = 45;
  const fireFootprintQuant = quantizeWorldCoord(PLAYER_HALF * 2);

  const fireballRadiusWorld = 12;
  const fireballOffsetWorld = PLAYER_HALF + 6 + fireballRadiusWorld;
  const fireballCenterXWorld = playerX + fireballOffsetWorld;
  const fireballCenterYWorld = playerY;
  const fireballCenterXQuant = quantizeWorldCoord(fireballCenterXWorld);
  const fireballCenterYQuant = quantizeWorldCoord(fireballCenterYWorld);
  const fireballRadiusQuant = quantizeWorldCoord(fireballRadiusWorld);
  const fireballDiameterQuant = quantizeWorldCoord(fireballRadiusWorld * 2);
  const fireballOffsetXQuant = quantizeWorldCoord(fireballOffsetWorld);

  const baseEndConditions = () => ({
    onUnequip: false,
    onOwnerDeath: false,
    onOwnerLost: false,
    onZoneChange: false,
    onExplicitCancel: false,
  });

  return {
    ver: 1,
    type: "state",
    players: [
      {
        id: "player-1",
        x: playerX,
        y: playerY,
        facing: playerFacing,
        health: 20,
        maxHealth: 20,
      },
    ],
    npcs: [
      {
        id: "npc-1",
        x: bloodCenterXWorld,
        y: bloodCenterYWorld,
        facing: "left",
        health: 10,
        maxHealth: 10,
        type: "goblin",
      },
    ],
    effects: [
      {
        id: BLOOD_EFFECT_ID,
        type: "blood-splatter",
        owner: "player-1",
        start: 1000,
        duration: 1200,
        x: bloodCenterXWorld - bloodFootprintWorld / 2,
        y: bloodCenterYWorld - bloodFootprintWorld / 2,
        width: bloodFootprintWorld,
        height: bloodFootprintWorld,
        params: {
          centerX: bloodCenterXQuant,
          centerY: bloodCenterYQuant,
          dropletRadius: 3,
          maxBursts: 1,
        },
      },
      {
        id: MELEE_EFFECT_ID,
        type: "attack",
        owner: "player-1",
        start: 1000,
        duration: 150,
        x: meleeCenterXWorld - meleeReach / 2,
        y: meleeCenterYWorld - meleeWidth / 2,
        width: meleeReach,
        height: meleeWidth,
        params: {
          healthDelta: -10,
          reach: meleeReach,
          width: meleeWidth,
        },
      },
      {
        id: FIRE_EFFECT_ID,
        type: "fire",
        owner: "player-1",
        start: 1000,
        duration: 3000,
        x: playerX - PLAYER_HALF,
        y: playerY - PLAYER_HALF,
        width: PLAYER_HALF * 2,
        height: PLAYER_HALF * 2,
      },
      {
        id: FIREBALL_EFFECT_ID,
        type: "fireball",
        owner: "player-1",
        start: 1000,
        duration: 3000,
        x: fireballCenterXWorld - fireballRadiusWorld,
        y: fireballCenterYWorld - fireballRadiusWorld,
        width: fireballRadiusWorld * 2,
        height: fireballRadiusWorld * 2,
        params: {
          dx: 1,
          dy: 0,
          radius: fireballRadiusWorld,
          speed: 320,
          range: 200,
          healthDelta: -15,
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
          definition: {
            typeId: "blood-splatter",
            delivery: "visual",
            shape: "rect",
            motion: "none",
            impact: "first-hit",
            lifetimeTicks: 18,
            hooks: {
              onSpawn: "visual.blood.splatter",
              onTick: "visual.blood.splatter",
            },
            client: {
              sendSpawn: true,
              sendUpdates: false,
              sendEnd: true,
            },
            end: {
              kind: 0,
              conditions: baseEndConditions(),
            },
          },
          startTick: 100,
          deliveryState: {
            geometry: {
              shape: "rect",
              width: bloodFootprintQuant,
              height: bloodFootprintQuant,
            },
            motion: {
              positionX: bloodCenterXQuant,
              positionY: bloodCenterYQuant,
              velocityX: 0,
              velocityY: 0,
            },
            follow: "none",
          },
          behaviorState: {
            ticksRemaining: 18,
            extra: {
              centerX: bloodCenterXQuant,
              centerY: bloodCenterYQuant,
            },
          },
          ownerActorId: "player-1",
          replication: {
            sendSpawn: true,
            sendUpdates: false,
            sendEnd: true,
          },
          end: {
            kind: 0,
            conditions: baseEndConditions(),
          },
        },
      },
      {
        tick: 100,
        seq: 1,
        instance: {
          id: MELEE_EFFECT_ID,
          definitionId: "attack",
          definition: {
            typeId: "attack",
            delivery: "area",
            shape: "rect",
            motion: "instant",
            impact: "first-hit",
            lifetimeTicks: 1,
            hooks: {
              onSpawn: "melee.spawn",
            },
            client: {
              sendSpawn: true,
              sendUpdates: true,
              sendEnd: true,
            },
            end: {
              kind: 1,
              conditions: baseEndConditions(),
            },
          },
          startTick: 100,
          deliveryState: {
            geometry: {
              shape: "rect",
              width: meleeWidthQuant,
              height: meleeHeightQuant,
              offsetX: meleeOffsetXQuant,
              offsetY: meleeOffsetYQuant,
            },
            motion: {
              positionX: meleeCenterXQuant,
              positionY: meleeCenterYQuant,
              velocityX: 0,
              velocityY: 0,
            },
            follow: "none",
          },
          behaviorState: {
            ticksRemaining: 3,
            extra: {
              healthDelta: -10,
              reach: meleeReach,
              width: meleeWidth,
            },
          },
          ownerActorId: "player-1",
          replication: {
            sendSpawn: true,
            sendUpdates: true,
            sendEnd: true,
          },
          end: {
            kind: 1,
            conditions: baseEndConditions(),
          },
        },
      },
      {
        tick: 100,
        seq: 1,
        instance: {
          id: FIRE_EFFECT_ID,
          definitionId: "fire",
          definition: {
            typeId: "fire",
            delivery: "target",
            shape: "rect",
            motion: "follow",
            impact: "first-hit",
            lifetimeTicks: fireLifetimeTicks,
            hooks: {
              onSpawn: "status.burning.visual",
              onTick: "status.burning.visual",
            },
            client: {
              sendSpawn: true,
              sendUpdates: true,
              sendEnd: true,
            },
            end: {
              kind: 0,
              conditions: baseEndConditions(),
            },
          },
          startTick: 100,
          deliveryState: {
            geometry: {
              shape: "rect",
              width: fireFootprintQuant,
              height: fireFootprintQuant,
            },
            motion: {
              positionX: 0,
              positionY: 0,
              velocityX: 0,
              velocityY: 0,
            },
            follow: "target",
          },
          behaviorState: {
            ticksRemaining: fireLifetimeTicks,
          },
          ownerActorId: "player-1",
          followActorId: "player-1",
          replication: {
            sendSpawn: true,
            sendUpdates: true,
            sendEnd: true,
          },
          end: {
            kind: 0,
            conditions: baseEndConditions(),
          },
        },
      },
      {
        tick: 100,
        seq: 1,
        instance: {
          id: FIREBALL_EFFECT_ID,
          definitionId: "fireball",
          definition: {
            typeId: "fireball",
            delivery: "area",
            shape: "circle",
            motion: "linear",
            impact: "first-hit",
            lifetimeTicks: 45,
            hooks: {
              onSpawn: "projectile.fireball.lifecycle",
              onTick: "projectile.fireball.lifecycle",
            },
            client: {
              sendSpawn: true,
              sendUpdates: true,
              sendEnd: true,
            },
            end: {
              kind: 0,
              conditions: baseEndConditions(),
            },
          },
          startTick: 100,
          deliveryState: {
            geometry: {
              shape: "circle",
              radius: fireballRadiusQuant,
              width: fireballDiameterQuant,
              height: fireballDiameterQuant,
              offsetX: fireballOffsetXQuant,
              offsetY: 0,
            },
            motion: {
              positionX: fireballCenterXQuant,
              positionY: fireballCenterYQuant,
              velocityX: 0,
              velocityY: 0,
            },
            follow: "none",
          },
          behaviorState: {
            ticksRemaining: 45,
            extra: {
              dx: 1,
              dy: 0,
              radius: fireballRadiusWorld,
              speed: 320,
              range: 200,
              healthDelta: -15,
            },
          },
          ownerActorId: "player-1",
          replication: {
            sendSpawn: true,
            sendUpdates: true,
            sendEnd: true,
          },
          end: {
            kind: 0,
            conditions: baseEndConditions(),
          },
        },
      },
    ],
    effect_seq_cursors: {
      [BLOOD_EFFECT_ID]: 1,
      [MELEE_EFFECT_ID]: 1,
      [FIRE_EFFECT_ID]: 1,
      [FIREBALL_EFFECT_ID]: 1,
    },
    effectTriggers: [],
    t: 100,
    sequence: 10,
  };
}

function createContractEndPayload() {
  return {
    ver: 1,
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
  renderInvocations = [];
  EffectManager.prototype.spawn = function spawnWithRenderTracking(
    definition,
    options,
  ) {
    const instance = originalSpawn.call(this, definition, options);
    if (instance && typeof instance === "object" && typeof instance.draw === "function") {
      if (!instance.__testWrappedDraw) {
        const originalDraw = instance.draw;
        instance.draw = function trackedDraw(frame) {
          renderInvocations.push({
            id: typeof this.id === "string" ? this.id : null,
            type: typeof this.type === "string" ? this.type : null,
          });
          return originalDraw.call(this, frame);
        };
        instance.__testWrappedDraw = true;
      }
    }
    return instance;
  };
});

afterEach(() => {
  drainRaf.now = 0;
  delete globalThis.requestAnimationFrame;
  EffectManager.prototype.spawn = originalSpawn;
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
    const trackedMelee = manager.getTrackedInstances("melee-swing");
    const trackedFire = manager.getTrackedInstances("fire");
    const trackedFireball = manager.getTrackedInstances("fireball");
    expect(trackedMelee.size).toBeGreaterThan(0);
    expect(trackedFire.size).toBeGreaterThan(0);
    expect(trackedFireball.size).toBeGreaterThan(0);
    const invocationsFor = (effectId) =>
      renderInvocations.filter((entry) => entry.id === effectId);
    expect(invocationsFor(MELEE_EFFECT_ID).length).toBeGreaterThan(0);
    expect(invocationsFor(FIRE_EFFECT_ID).length).toBeGreaterThan(0);
    expect(invocationsFor(FIREBALL_EFFECT_ID).length).toBeGreaterThan(0);
    // The added attack (melee-swing), fire, and fireball lifecycle entries are mirrored into
    // the EffectManager as expected and render during the frame, but render.js still never
    // mirrors the contract-derived "blood-splatter" entry. We keep the expectation as the
    // desired behaviour and document the gap here rather than papering over it.
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
