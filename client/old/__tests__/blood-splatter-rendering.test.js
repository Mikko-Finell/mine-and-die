import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { ensureEffectLifecycleState } from "../effect-lifecycle.js";
import { startRenderLoop } from "../render.js";
import { EffectManager } from "@js-effects/effects-lib";
import { createEffectTestStore } from "./__helpers__/effect-test-store.js";
import { ingestStateMessage } from "./__helpers__/ingest-state-message.js";

const ATTACK_STATE_MESSAGE =
  '{"ver":1,"type":"state","t":2451,"sequence":2504,"keyframeSeq":2484,"serverTime":1760429765710,' +
  '"config":{"width":2400,"height":1800},"players":[{"id":"player-2","x":904,"y":1238,"facing":"right","maxHealth":100,"health":100}],' +
  '"displayPlayers":{"player-2":{"x":904,"y":1238}},"npcs":[{"id":"npc-rat-3","x":898.8969594381456,"y":1242.443619937173,"facing":"left","type":"rat","maxHealth":60,"health":60}],' +
  '"displayNPCs":{"npc-rat-3":{"x":898.8969594381456,"y":1242.443619937173}},"effect_spawned":[{"tick":2451,"seq":1,' +
  '"instance":{"id":"contract-effect-5","definitionId":"attack","definition":{"typeId":"melee-swing","delivery":"area","shape":"rect","motion":"instant","impact":"first-hit","lifetimeTicks":1,' +
  '"hooks":{"onSpawn":"melee.spawn"},"client":{"sendSpawn":true,"sendUpdates":true,"sendEnd":true,"managedByClient":true},"end":{"kind":1,' +
  '"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}},' +
  '"startTick":2451,"deliveryState":{"geometry":{"shape":"rect","offsetX":-17,"width":22,"height":16},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},' +
  '"behaviorState":{"ticksRemaining":3,"extra":{"healthDelta":-10,"reach":56,"width":40}},"ownerActorId":"player-2",' +
  '"replication":{"sendSpawn":true,"sendUpdates":true,"sendEnd":true,"managedByClient":true},"end":{"kind":1,' +
  '"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}}}],' +
  '"effect_update":[{"tick":2451,"seq":2,"id":"contract-effect-5","deliveryState":{"geometry":{"shape":"rect","offsetX":-17,"width":22,"height":16},' +
  '"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},"behaviorState":{"ticksRemaining":3,' +
  '"extra":{"healthDelta":-10,"reach":56,"width":40}}}],"effect_ended":[{"tick":2451,"seq":3,"id":"contract-effect-5","reason":"expired"}],' +
  '"effect_seq_cursors":{"contract-effect-5":3},"patches":[{"kind":"npc_pos","entityId":"npc-rat-3","payload":{"x":898.8969594381456,"y":1242.443619937173}}]}';

const BLOOD_SPLATTER_STATE_MESSAGE =
  '{"ver":1,"type":"state","t":2452,"sequence":2505,"keyframeSeq":2484,"serverTime":1760429765775,' +
  '"config":{"width":2400,"height":1800},"players":[{"id":"player-2","x":904,"y":1238,"facing":"right","maxHealth":100,"health":100}],' +
  '"displayPlayers":{"player-2":{"x":904,"y":1238}},"npcs":[{"id":"npc-rat-3","x":907.2384585405665,"y":1235.7951290070935,"facing":"left","type":"rat","maxHealth":60,"health":60}],' +
  '"displayNPCs":{"npc-rat-3":{"x":907.2384585405665,"y":1235.7951290070935}},' +
  '"effect_spawned":[{"tick":2452,"seq":1,' +
  '"instance":{"id":"contract-effect-6","definitionId":"blood-splatter","definition":{"typeId":"blood-splatter","delivery":"visual","shape":"rect","motion":"none","impact":"first-hit","lifetimeTicks":18,' +
  '"hooks":{"onSpawn":"visual.blood.splatter","onTick":"visual.blood.splatter"},"client":{"sendSpawn":true,"sendUpdates":false,"sendEnd":true,"managedByClient":true},"end":{"kind":0,' +
  '"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}},' +
  '"startTick":2452,"deliveryState":{"geometry":{"shape":"rect","width":11,"height":11},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},' +
  '"behaviorState":{"ticksRemaining":18,"extra":{"centerX":18,"centerY":71}},"params":{"drag":0.92,"dropletRadius":3,' +
  '"maxBursts":0,"maxDroplets":33,"maxStainRadius":6,"maxStains":140,"minDroplets":4,"minStainRadius":4,' +
  '"spawnInterval":1.1,"speed":3},"ownerActorId":"player-2",' +
  '"colors":["#7a0e12","#4a090b"],"replication":{"sendSpawn":true,"sendUpdates":false,"sendEnd":true,"managedByClient":true},"end":{"kind":0,' +
  '"conditions":{"onUnequip":false,"onOwnerDeath":false,"onOwnerLost":false,"onZoneChange":false,"onExplicitCancel":false}}}}],' +
  '"effect_seq_cursors":{"contract-effect-6":1},"patches":[{"kind":"npc_pos","entityId":"npc-rat-3","payload":{"x":907.2384585405665,"y":1235.7951290070935}}]}';

describe("blood-splatter visual effects", () => {
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

  test("blood-splatter lifecycle entries spawn render instances", () => {
    const store = createEffectTestStore({
      player: { id: "player-2", x: 904, y: 1238, facing: "right", maxHealth: 100, health: 100 },
      world: { width: 2400, height: 1800 },
    });

    const attackSummary = ingestStateMessage(store, ATTACK_STATE_MESSAGE).lifecycleSummary;
    expect(attackSummary.spawns).toEqual(["contract-effect-5"]);

    const { lifecycleSummary } = ingestStateMessage(store, BLOOD_SPLATTER_STATE_MESSAGE);
    expect(lifecycleSummary.spawns).toEqual(["contract-effect-6"]);

    const lifecycleState = ensureEffectLifecycleState(store);
    const entry = lifecycleState.instances.get("contract-effect-6");
    expect(entry?.instance?.definitionId).toBe("blood-splatter");

    const spawnSpy = vi.spyOn(EffectManager.prototype, "spawn");

    try {
      startRenderLoop(store);
      expect(typeof rafCallback).toBe("function");
      rafCallback(performance.now() + 16);

      const tracked = store.effectManager.getTrackedInstances("blood-splatter");
      // Regression guard: prepareEffectPass should hand blood-splatter lifecycle entries to the
      // effect manager so ground decals render. When the transfer is broken the spawn hook never
      // fires and the manager reports zero tracked instances, leaving the splatter invisible.
      expect(spawnSpy).toHaveBeenCalledTimes(1);
      const [, options] = spawnSpy.mock.calls[0];
      expect(options?.colors).toEqual(["#7a0e12", "#4a090b"]);
      expect(options?.drag).toBeCloseTo(0.92, 5);
      expect(options?.dropletRadius).toBeCloseTo(3, 5);
      expect(options?.maxBursts).toBeCloseTo(0, 5);
      expect(options?.maxDroplets).toBeCloseTo(33, 5);
      expect(options?.maxStainRadius).toBeCloseTo(6, 5);
      expect(options?.maxStains).toBeCloseTo(140, 5);
      expect(options?.minDroplets).toBeCloseTo(4, 5);
      expect(options?.minStainRadius).toBeCloseTo(4, 5);
      expect(options?.spawnInterval).toBeCloseTo(1.1, 5);
      expect(options?.speed).toBeCloseTo(3, 5);
      expect(tracked.size).toBeGreaterThanOrEqual(1);
    } finally {
      spawnSpy.mockRestore();
    }
  });
});
