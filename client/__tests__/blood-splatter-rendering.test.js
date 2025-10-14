import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { parseServerEvent, handleProtocolVersion, applyStateSnapshot, deriveDisplayMaps } from "../network.js";
import { createPatchState } from "../patches.js";
import {
  applyEffectLifecycleBatch,
  ensureEffectLifecycleState,
} from "../effect-lifecycle.js";
import { startRenderLoop } from "../render.js";
import { EffectManager } from "../js-effects/manager.js";
import { RENDER_MODE_SNAPSHOT } from "../render-modes.js";

const RAW_STATE_MESSAGE =
  "{\"ver\":1,\"type\":\"state\",\"effect_spawned\":[{\"tick\":3003,\"seq\":1,\"instance\":{\"id\":\"contract-effect-17\",\"definitionId\":\"blood-splatter\",\"definition\":{\"typeId\":\"blood-splatter\",\"delivery\":\"visual\",\"shape\":\"rect\",\"motion\":\"none\",\"impact\":\"first-hit\",\"lifetimeTicks\":18,\"hooks\":{\"onSpawn\":\"visual.blood.splatter\",\"onTick\":\"visual.blood.splatter\"},\"client\":{\"sendSpawn\":true,\"sendUpdates\":false,\"sendEnd\":true},\"end\":{\"kind\":0,\"conditions\":{\"onUnequip\":false,\"onOwnerDeath\":false,\"onOwnerLost\":false,\"onZoneChange\":false,\"onExplicitCancel\":false}}},\"startTick\":3003,\"deliveryState\":{\"geometry\":{\"shape\":\"rect\",\"width\":11,\"height\":11},\"motion\":{\"positionX\":0,\"positionY\":0,\"velocityX\":0,\"velocityY\":0},\"follow\":\"none\"},\"behaviorState\":{\"ticksRemaining\":18,\"extra\":{\"centerX\":604,\"centerY\":354}},\"ownerActorId\":\"player-2\",\"replication\":{\"sendSpawn\":true,\"sendUpdates\":false,\"sendEnd\":true},\"end\":{\"kind\":0,\"conditions\":{\"onUnequip\":false,\"onOwnerDeath\":false,\"onOwnerLost\":false,\"onZoneChange\":false,\"onExplicitCancel\":false}}}},{\"tick\":3003,\"seq\":1,\"instance\":{\"id\":\"contract-effect-18\",\"definitionId\":\"blood-splatter\",\"definition\":{\"typeId\":\"blood-splatter\",\"delivery\":\"visual\",\"shape\":\"rect\",\"motion\":\"none\",\"impact\":\"first-hit\",\"lifetimeTicks\":18,\"hooks\":{\"onSpawn\":\"visual.blood.splatter\",\"onTick\":\"visual.blood.splatter\"},\"client\":{\"sendSpawn\":true,\"sendUpdates\":false,\"sendEnd\":true},\"end\":{\"kind\":0,\"conditions\":{\"onUnequip\":false,\"onOwnerDeath\":false,\"onOwnerLost\":false,\"onZoneChange\":false,\"onExplicitCancel\":false}}},\"startTick\":3003,\"deliveryState\":{\"geometry\":{\"shape\":\"rect\",\"width\":11,\"height\":11},\"motion\":{\"positionX\":0,\"positionY\":0,\"velocityX\":0,\"velocityY\":0},\"follow\":\"none\"},\"behaviorState\":{\"ticksRemaining\":18,\"extra\":{\"centerX\":596,\"centerY\":341}},\"ownerActorId\":\"player-2\",\"replication\":{\"sendSpawn\":true,\"sendUpdates\":false,\"sendEnd\":true},\"end\":{\"kind\":0,\"conditions\":{\"onUnequip\":false,\"onOwnerDeath\":false,\"onOwnerLost\":false,\"onZoneChange\":false,\"onExplicitCancel\":false}}}}],\"effect_seq_cursors\":{\"contract-effect-17\":1,\"contract-effect-18\":1},\"patches\":[{\"kind\":\"npc_pos\",\"entityId\":\"npc-rat-3\",\"payload\":{\"x\":1022.5983188067556,\"y\":1022.9362807067404}}],\"t\":3003,\"sequence\":3160,\"keyframeSeq\":3134,\"serverTime\":1760384455798,\"config\":{\"obstacles\":true,\"obstaclesCount\":2,\"goldMines\":true,\"goldMineCount\":1,\"npcs\":true,\"goblinCount\":2,\"ratCount\":1,\"npcCount\":3,\"lava\":true,\"lavaCount\":3,\"seed\":\"prototype\",\"width\":2400,\"height\":1800},\"keyframeInterval\":30}";

const FIREBALL_STATE_MESSAGE =
  '{"ver":1,"type":"state","effect_spawned":[{"tick":755,"seq":1,"instance":{"id":"contract-effect-7","definitionId":"fireball","definition":{"typeId":"fireball","delivery":"area","shape":"circle","motion":"linear","impact":"first-hit","lifetimeTicks":45,"hooks":{"onSpawn":"projectile.fireball.lifecycle","onTick":"projectile.fireball.lifecycle"},"client":{"sendSpawn":true,"sendUpdates":true,"sendEnd":true},"end":{"kind":0,"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}},"startTick":755,"deliveryState":{"geometry":{"shape":"circle","offsetY":-13,"width":10,"height":10,"radius":5},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},"behaviorState":{"ticksRemaining":45,"extra":{"dx":0,"dy":-1,"healthDelta":-15,"radius":12,"range":200,"remainingRange":200,"speed":320}},"ownerActorId":"player-2","replication":{"sendSpawn":true,"sendUpdates":true,"sendEnd":true},"end":{"kind":0,"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}}}],"effect_update":[{"tick":755,"seq":2,"id":"contract-effect-7","deliveryState":{"geometry":{"shape":"circle","offsetY":-22,"width":10,"height":10,"radius":5},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},"behaviorState":{"ticksRemaining":45,"extra":{"dx":0,"dy":-1,"healthDelta":-15,"radius":12,"range":200,"remainingRange":179,"speed":320}}}],"effect_seq_cursors":{"contract-effect-7":2},"patches":[{"kind":"npc_pos","entityId":"npc-rat-3","payload":{"x":1002.6716357990908,"y":1291.4205793376696}},{"kind":"effect_pos","entityId":"contract-effect-7","payload":{"x":1276.5670472790982,"y":754.4748771614693}},{"kind":"effect_params","entityId":"contract-effect-7","payload":{"params":{"dx":0,"dy":-1,"healthDelta":-15,"radius":12.5,"range":200,"remainingRange":178.66666666666666,"speed":320}}}],"t":755,"sequence":829,"keyframeSeq":813,"serverTime":1760387896479,"config":{"obstacles":true,"obstaclesCount":2,"goldMines":true,"goldMineCount":1,"npcs":true,"goblinCount":2,"ratCount":1,"npcCount":3,"lava":true,"lavaCount":3,"seed":"prototype","width":2400,"height":1800},"keyframeInterval":30}';

const FIRE_STATE_MESSAGE =
  '{"ver":1,"type":"state","effect_spawned":[{"tick":283,"seq":1,"instance":{"id":"contract-effect-15","definitionId":"fire","definition":{"typeId":"fire","delivery":"target","shape":"rect","motion":"follow","impact":"first-hit","lifetimeTicks":45,"hooks":{"onSpawn":"status.burning.visual","onTick":"status.burning.visual"},"client":{"sendSpawn":true,"sendUpdates":true,"sendEnd":true},"end":{"kind":0,"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}},"startTick":283,"deliveryState":{"geometry":{"shape":"rect","width":11,"height":11},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"attachedActorId":"player-2","follow":"target"},"behaviorState":{"ticksRemaining":45},"followActorId":"player-2","ownerActorId":"lava-2","replication":{"sendSpawn":true,"sendUpdates":true,"sendEnd":true},"end":{"kind":0,"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}}},{"tick":283,"seq":1,"instance":{"id":"contract-effect-16","definitionId":"burning-tick","definition":{"typeId":"burning-tick","delivery":"target","shape":"rect","motion":"instant","impact":"first-hit","lifetimeTicks":1,"hooks":{"onSpawn":"status.burning.tick"},"client":{"sendSpawn":true,"sendUpdates":false,"sendEnd":true},"end":{"kind":1,"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}},"startTick":283,"deliveryState":{"geometry":{"shape":"rect","width":11,"height":11},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"attachedActorId":"player-2","follow":"target"},"behaviorState":{"ticksRemaining":1,"extra":{"healthDelta":-4}},"followActorId":"player-2","ownerActorId":"lava-2","replication":{"sendSpawn":true,"sendUpdates":false,"sendEnd":true},"end":{"kind":1,"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}}}],"effect_update":[{"tick":283,"seq":2,"id":"contract-effect-15","deliveryState":{"geometry":{"shape":"rect","width":11,"height":11},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"attachedActorId":"player-2","follow":"target"},"behaviorState":{"ticksRemaining":45}}],"effect_ended":[{"tick":283,"seq":2,"id":"contract-effect-16","reason":"expired"}],"effect_seq_cursors":{"contract-effect-15":2,"contract-effect-16":2},"patches":[{"kind":"player_pos","entityId":"player-2","payload":{"x":526.3024428824517,"y":347.8355910446632}},{"kind":"player_health","entityId":"player-2","payload":{"health":96,"maxHealth":100}}],"t":283,"sequence":288,"keyframeSeq":269,"serverTime":1760388654779,"config":{"obstacles":true,"obstaclesCount":2,"goldMines":true,"goldMineCount":1,"npcs":true,"goblinCount":2,"ratCount":1,"npcCount":3,"lava":true,"lavaCount":3,"seed":"prototype","width":2400,"height":1800},"keyframeInterval":30}';

function createMockContext() {
  const noop = () => {};
  const gradient = { addColorStop: noop };
  return {
    canvas: { width: 1280, height: 720 },
    fillStyle: "#000000",
    strokeStyle: "#000000",
    lineWidth: 1,
    globalAlpha: 1,
    font: "",
    textAlign: "left",
    textBaseline: "alphabetic",
    save: noop,
    restore: noop,
    translate: noop,
    rotate: noop,
    scale: noop,
    beginPath: noop,
    closePath: noop,
    moveTo: noop,
    lineTo: noop,
    stroke: noop,
    fill: noop,
    fillRect: noop,
    strokeRect: noop,
    clearRect: noop,
    arc: noop,
    ellipse: noop,
    quadraticCurveTo: noop,
    fillText: noop,
    createLinearGradient: () => gradient,
    drawImage: noop,
  };
}

function createClientStore() {
  const ctx = createMockContext();
  const canvas = ctx.canvas;
  const effectManager = new EffectManager();
  const store = {
    statusEl: null,
    canvas,
    ctx,
    TILE_SIZE: 40,
    GRID_WIDTH: 60,
    GRID_HEIGHT: 45,
    WORLD_WIDTH: 2400,
    WORLD_HEIGHT: 1800,
    PLAYER_SIZE: 28,
    PLAYER_HALF: 14,
    LERP_RATE: 12,
    statusBaseText: "",
    latencyMs: null,
    simulatedLatencyMs: 0,
    playerId: "player-2",
    currentFacing: "down",
    isPathActive: false,
    activePathTarget: null,
    renderMode: RENDER_MODE_SNAPSHOT,
    keyframeInterval: null,
    defaultKeyframeInterval: null,
    lastTimestamp: performance.now(),
    keys: new Set(),
    directionOrder: [],
    lastStateReceivedAt: null,
    lastTick: 3002,
    effects: [],
    effectManager,
    effectInstancesById: new Map(),
    pendingEffectTriggers: [],
    processedEffectTriggerIds: new Set(),
    patchState: createPatchState(),
    camera: { x: 0, y: 0, lockOnPlayer: true },
    inventorySlotCount: 20,
    worldConfig: { width: 2400, height: 1800 },
    obstacles: [],
    groundItems: {},
    renderInventory: null,
    updateDiagnostics: () => {},
    setStatusBase: () => {},
    setLatency: () => {},
    updateWorldConfigUI: () => {},
    updateRenderModeUI: () => {},
    lastEffectLifecycleSummary: null,
    displayPlayers: {},
    displayNPCs: {},
    players: {},
    npcs: {},
  };

  const player = {
    id: "player-2",
    x: 604,
    y: 354,
    facing: "down",
    maxHealth: 100,
    health: 100,
  };
  store.players[player.id] = player;
  store.displayPlayers[player.id] = { x: player.x, y: player.y };

  const npc = {
    id: "npc-rat-3",
    x: 990,
    y: 990,
    facing: "down",
    type: "rat",
    maxHealth: 60,
    health: 60,
  };
  store.npcs[npc.id] = npc;
  store.displayNPCs[npc.id] = { x: npc.x, y: npc.y };

  return store;
}

describe("blood-splatter rendering integration", () => {
  let rafCallback;

  beforeEach(() => {
    rafCallback = null;
    globalThis.requestAnimationFrame = (cb) => {
      rafCallback = cb;
      return 1;
    };
  });

  afterEach(() => {
    rafCallback = null;
    delete globalThis.requestAnimationFrame;
  });

  test("blood-splatter state messages should render through the effect pipeline", () => {
    const store = createClientStore();

    const parsed = parseServerEvent(RAW_STATE_MESSAGE);
    expect(parsed).not.toBeNull();
    expect(parsed.type).toBe("state");

    const spawns = parsed.data.effect_spawned ?? [];
    expect(spawns).toHaveLength(2);
    expect(spawns.every((spawn) => spawn?.instance?.definitionId === "blood-splatter")).toBe(true);

    handleProtocolVersion(parsed.data, "state message");

    const snapshot = applyStateSnapshot(store, parsed.data, null);
    store.players = snapshot.players;
    store.npcs = snapshot.npcs;
    store.obstacles = snapshot.obstacles;
    store.effects = snapshot.effects;
    store.groundItems = snapshot.groundItems;
    store.lastTick = snapshot.lastTick;
    if (snapshot.worldConfig) {
      store.worldConfig = snapshot.worldConfig;
      store.WORLD_WIDTH = snapshot.worldConfig.width;
      store.WORLD_HEIGHT = snapshot.worldConfig.height;
    }
    if (Number.isFinite(snapshot.keyframeInterval)) {
      store.keyframeInterval = snapshot.keyframeInterval;
    }
    if (snapshot.currentFacing) {
      store.currentFacing = snapshot.currentFacing;
    }

    const { displayPlayers, displayNPCs } = deriveDisplayMaps(
      store.players,
      store.npcs,
      store.displayPlayers,
      store.displayNPCs,
    );
    store.displayPlayers = displayPlayers;
    store.displayNPCs = displayNPCs;

    const lifecycleSummary = applyEffectLifecycleBatch(store, parsed.data);
    expect(lifecycleSummary.spawns).toEqual(["contract-effect-17", "contract-effect-18"]);
    store.lastEffectLifecycleSummary = lifecycleSummary;

    const lifecycleState = ensureEffectLifecycleState(store);
    const firstEntry = lifecycleState.instances.get("contract-effect-17");
    const secondEntry = lifecycleState.instances.get("contract-effect-18");
    expect(firstEntry?.instance?.definitionId).toBe("blood-splatter");
    expect(secondEntry?.instance?.definitionId).toBe("blood-splatter");

    startRenderLoop(store);
    expect(typeof rafCallback).toBe("function");
    rafCallback(performance.now() + 16);

    expect(store.__effectLifecycleView?.entries?.size ?? 0).toBe(2);

    const tracked = store.effectManager.getTrackedInstances("blood-splatter");
    expect(tracked.size).toBe(2);
  });

  test("blood-splatter lifecycle entries should spawn render instances during prepareEffectPass", () => {
    const store = createClientStore();

    const parsed = parseServerEvent(RAW_STATE_MESSAGE);
    expect(parsed).not.toBeNull();
    expect(parsed.type).toBe("state");

    handleProtocolVersion(parsed.data, "state message");

    const snapshot = applyStateSnapshot(store, parsed.data, null);
    store.players = snapshot.players;
    store.npcs = snapshot.npcs;
    store.effects = snapshot.effects;
    store.groundItems = snapshot.groundItems;
    store.lastTick = snapshot.lastTick;

    const lifecycleSummary = applyEffectLifecycleBatch(store, parsed.data);
    expect(lifecycleSummary.spawns).toEqual(["contract-effect-17", "contract-effect-18"]);

    const lifecycleState = ensureEffectLifecycleState(store);
    expect(lifecycleState.instances.size).toBe(2);

    const spawnSpy = vi.spyOn(EffectManager.prototype, "spawn");

    try {
      startRenderLoop(store);
      expect(typeof rafCallback).toBe("function");
      rafCallback(performance.now() + 16);

      const tracked = store.effectManager.getTrackedInstances("blood-splatter");
      // This expectation documents the precise regression point: prepareEffectPass should spawn
      // contract-derived blood-splatter instances so they are registered with the EffectManager.
      // When the sync is missing the manager still reports zero tracked entries at this stage,
      // meaning spawnSpy never observes the two expected spawn calls.
      expect(spawnSpy).toHaveBeenCalledTimes(2);
      expect(tracked.size).toBe(2);
    } finally {
      spawnSpy.mockRestore();
    }
  });

  test("fireball lifecycle entries should spawn render instances during prepareEffectPass", () => {
    const store = createClientStore();

    const parsed = parseServerEvent(FIREBALL_STATE_MESSAGE);
    expect(parsed).not.toBeNull();
    expect(parsed.type).toBe("state");

    handleProtocolVersion(parsed.data, "state message");

    const snapshot = applyStateSnapshot(store, parsed.data, null);
    store.players = snapshot.players;
    store.npcs = snapshot.npcs;
    store.effects = snapshot.effects;
    store.groundItems = snapshot.groundItems;
    store.lastTick = snapshot.lastTick;

    const lifecycleSummary = applyEffectLifecycleBatch(store, parsed.data);
    expect(lifecycleSummary.spawns).toEqual(["contract-effect-7"]);
    expect(lifecycleSummary.updates).toEqual(["contract-effect-7"]);

    const lifecycleState = ensureEffectLifecycleState(store);
    expect(lifecycleState.instances.size).toBe(1);

    const spawnSpy = vi.spyOn(EffectManager.prototype, "spawn");

    try {
      startRenderLoop(store);
      expect(typeof rafCallback).toBe("function");
      rafCallback(performance.now() + 16);

      const tracked = store.effectManager.getTrackedInstances("fireball");
      // This expectation mirrors the blood-splatter regression documentation: prepareEffectPass
      // should hand fireball lifecycle entries to the EffectManager so projectiles render on the
      // canvas. If the spawn hand-off regresses, spawnSpy would stay at zero and the tracked map
      // would remain empty, immediately surfacing the failure point.
      expect(spawnSpy).toHaveBeenCalledTimes(1);
      expect(tracked.size).toBe(1);
    } finally {
      spawnSpy.mockRestore();
    }
  });

  test("fireball state messages should render through the effect pipeline", () => {
    const store = createClientStore();

    const parsed = parseServerEvent(FIREBALL_STATE_MESSAGE);
    expect(parsed).not.toBeNull();
    expect(parsed.type).toBe("state");

    const spawns = parsed.data.effect_spawned ?? [];
    expect(spawns).toHaveLength(1);
    expect(spawns[0]?.instance?.definitionId).toBe("fireball");

    handleProtocolVersion(parsed.data, "state message");

    const snapshot = applyStateSnapshot(store, parsed.data, null);
    store.players = snapshot.players;
    store.npcs = snapshot.npcs;
    store.obstacles = snapshot.obstacles;
    store.effects = snapshot.effects;
    store.groundItems = snapshot.groundItems;
    store.lastTick = snapshot.lastTick;
    if (snapshot.worldConfig) {
      store.worldConfig = snapshot.worldConfig;
      store.WORLD_WIDTH = snapshot.worldConfig.width;
      store.WORLD_HEIGHT = snapshot.worldConfig.height;
    }
    if (Number.isFinite(snapshot.keyframeInterval)) {
      store.keyframeInterval = snapshot.keyframeInterval;
    }
    if (snapshot.currentFacing) {
      store.currentFacing = snapshot.currentFacing;
    }

    const { displayPlayers, displayNPCs } = deriveDisplayMaps(
      store.players,
      store.npcs,
      store.displayPlayers,
      store.displayNPCs,
    );
    store.displayPlayers = displayPlayers;
    store.displayNPCs = displayNPCs;

    const lifecycleSummary = applyEffectLifecycleBatch(store, parsed.data);
    expect(lifecycleSummary.spawns).toEqual(["contract-effect-7"]);
    expect(lifecycleSummary.updates).toEqual(["contract-effect-7"]);
    store.lastEffectLifecycleSummary = lifecycleSummary;

    const lifecycleState = ensureEffectLifecycleState(store);
    const entry = lifecycleState.instances.get("contract-effect-7");
    expect(entry?.instance?.definitionId).toBe("fireball");

    startRenderLoop(store);
    expect(typeof rafCallback).toBe("function");
    rafCallback(performance.now() + 16);

    expect(store.__effectLifecycleView?.entries?.size ?? 0).toBe(1);

    const tracked = store.effectManager.getTrackedInstances("fireball");
    expect(tracked.size).toBe(1);
  });

  test("fire state messages should render through the effect pipeline", () => {
    const store = createClientStore();

    const parsed = parseServerEvent(FIRE_STATE_MESSAGE);
    expect(parsed).not.toBeNull();
    expect(parsed.type).toBe("state");

    const spawns = parsed.data.effect_spawned ?? [];
    expect(spawns).toHaveLength(2);
    expect(
      spawns
        .map((spawn) => spawn?.instance?.definitionId)
        .filter((definitionId) => typeof definitionId === "string")
        .sort(),
    ).toEqual(["burning-tick", "fire"]);

    handleProtocolVersion(parsed.data, "state message");

    const snapshot = applyStateSnapshot(store, parsed.data, null);
    store.players = snapshot.players;
    store.npcs = snapshot.npcs;
    store.obstacles = snapshot.obstacles;
    store.effects = snapshot.effects;
    store.groundItems = snapshot.groundItems;
    store.lastTick = snapshot.lastTick;
    if (snapshot.worldConfig) {
      store.worldConfig = snapshot.worldConfig;
      store.WORLD_WIDTH = snapshot.worldConfig.width;
      store.WORLD_HEIGHT = snapshot.worldConfig.height;
    }
    if (Number.isFinite(snapshot.keyframeInterval)) {
      store.keyframeInterval = snapshot.keyframeInterval;
    }
    if (snapshot.currentFacing) {
      store.currentFacing = snapshot.currentFacing;
    }

    const { displayPlayers, displayNPCs } = deriveDisplayMaps(
      store.players,
      store.npcs,
      store.displayPlayers,
      store.displayNPCs,
    );
    store.displayPlayers = displayPlayers;
    store.displayNPCs = displayNPCs;

    const lifecycleSummary = applyEffectLifecycleBatch(store, parsed.data);
    expect(lifecycleSummary.spawns).toEqual(["contract-effect-15", "contract-effect-16"]);
    expect(lifecycleSummary.updates).toEqual(["contract-effect-15"]);
    expect(lifecycleSummary.ends).toEqual(["contract-effect-16"]);
    store.lastEffectLifecycleSummary = lifecycleSummary;

    const lifecycleState = ensureEffectLifecycleState(store);
    const fireEntry = lifecycleState.instances.get("contract-effect-15");
    const burningTickEntry = lifecycleState.instances.get("contract-effect-16");
    expect(fireEntry?.instance?.definitionId).toBe("fire");
    expect(burningTickEntry).toBeUndefined();

    startRenderLoop(store);
    expect(typeof rafCallback).toBe("function");
    rafCallback(performance.now() + 16);

    expect(store.__effectLifecycleView?.entries?.size ?? 0).toBe(1);

    const trackedFire = store.effectManager.getTrackedInstances("fire");
    expect(trackedFire.size).toBe(1);
    const trackedBurningTick = store.effectManager.getTrackedInstances("burning-tick");
    expect(trackedBurningTick.size).toBe(0);
  });

  test("fire lifecycle entries should spawn render instances during prepareEffectPass", () => {
    const store = createClientStore();

    const parsed = parseServerEvent(FIRE_STATE_MESSAGE);
    expect(parsed).not.toBeNull();
    expect(parsed.type).toBe("state");

    handleProtocolVersion(parsed.data, "state message");

    const snapshot = applyStateSnapshot(store, parsed.data, null);
    store.players = snapshot.players;
    store.npcs = snapshot.npcs;
    store.effects = snapshot.effects;
    store.groundItems = snapshot.groundItems;
    store.lastTick = snapshot.lastTick;

    const lifecycleSummary = applyEffectLifecycleBatch(store, parsed.data);
    expect(lifecycleSummary.spawns).toEqual(["contract-effect-15", "contract-effect-16"]);
    expect(lifecycleSummary.updates).toEqual(["contract-effect-15"]);
    expect(lifecycleSummary.ends).toEqual(["contract-effect-16"]);

    const lifecycleState = ensureEffectLifecycleState(store);
    expect(lifecycleState.instances.size).toBe(1);
    expect(lifecycleState.instances.has("contract-effect-15")).toBe(true);

    const spawnSpy = vi.spyOn(EffectManager.prototype, "spawn");

    try {
      startRenderLoop(store);
      expect(typeof rafCallback).toBe("function");
      rafCallback(performance.now() + 16);

      const trackedFire = store.effectManager.getTrackedInstances("fire");
      const trackedBurningTick = store.effectManager.getTrackedInstances("burning-tick");
      // Fire visuals persist while the burning tick expires immediately. This regression test
      // guards the prepareEffectPass hand-off so the persistent fire instance reaches the
      // EffectManager; if the spawn path breaks the spy would never observe a call and the tracked
      // flames map would remain empty. The expectation also confirms that burning-tick stays absent
      // because it ended before rendering.
      expect(spawnSpy).toHaveBeenCalledTimes(1);
      expect(trackedFire.size).toBe(1);
      expect(trackedBurningTick.size).toBe(0);
    } finally {
      spawnSpy.mockRestore();
    }
  });
});
