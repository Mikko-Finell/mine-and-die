import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { ensureEffectLifecycleState } from "../effect-lifecycle.js";
import { startRenderLoop } from "../render.js";
import { createEffectTestStore } from "./__helpers__/effect-test-store.js";
import { ingestStateMessage } from "./__helpers__/ingest-state-message.js";

const SAME_BATCH_MESSAGE =
  '{"ver":1,"type":"state","t":128,"sequence":512,"keyframeSeq":500,"serverTime":1710000000123,' +
  '"config":{"width":2400,"height":1800},"players":[{"id":"player-1","x":544,"y":352,"facing":"down","maxHealth":100,"health":100}],' +
  '"effect_spawned":[{"tick":128,"seq":1,"instance":{"id":"contract-effect-one-tick","definitionId":"attack","ownerActorId":"player-1",' +
  '"deliveryState":{"geometry":{"shape":"rect","width":12,"height":12},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},' +
  '"behaviorState":{"ticksRemaining":1},"params":{"lifetimeTicks":1},' +
  '"replication":{"sendSpawn":true,"sendUpdates":true,"sendEnd":true,"managedByClient":true}}}],"effect_update":[{"tick":128,"seq":2,"id":"contract-effect-one-tick",' +
  '"deliveryState":{"geometry":{"shape":"rect","width":12,"height":12},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"}}],' +
  '"effect_ended":[{"tick":128,"seq":3,"id":"contract-effect-one-tick","reason":"expired"}],"effect_seq_cursors":{"contract-effect-one-tick":3},' +
  '"patches":[{"kind":"player_pos","entityId":"player-1","payload":{"x":544,"y":352}}]}';

describe("effect lifecycle – spawn and end in same batch", () => {
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

  test("one-tick melee effect should survive into the first render frame", () => {
    const store = createEffectTestStore({
      player: { id: "player-1", x: 544, y: 352, facing: "down", maxHealth: 100, health: 100 },
    });

    const { lifecycleSummary } = ingestStateMessage(store, SAME_BATCH_MESSAGE);

    expect(lifecycleSummary.spawns).toEqual(["contract-effect-one-tick"]);
    expect(lifecycleSummary.updates).toEqual(["contract-effect-one-tick"]);
    expect(lifecycleSummary.ends).toEqual(["contract-effect-one-tick"]);

    const lifecycleState = ensureEffectLifecycleState(store);
    expect(lifecycleState.instances.size).toBe(0);

    startRenderLoop(store);
    expect(typeof rafCallback).toBe("function");
    const firstFrameTime = performance.now() + 16;
    rafCallback(firstFrameTime);

    let tracked = store.effectManager.getTrackedInstances("melee-swing");
    expect(tracked.size).toBeGreaterThanOrEqual(1);

    expect(typeof rafCallback).toBe("function");
    rafCallback(firstFrameTime + 16);

    tracked = store.effectManager.getTrackedInstances("melee-swing");
    expect(tracked.size).toBeGreaterThanOrEqual(1);
  });
});
