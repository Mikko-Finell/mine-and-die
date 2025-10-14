import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { startRenderLoop } from "../render.js";
import { createEffectTestStore } from "./__helpers__/effect-test-store.js";
import { ingestStateMessage } from "./__helpers__/ingest-state-message.js";
import { EffectManager } from "../js-effects/manager.js";

const ZERO_MOTION_MESSAGE =
  '{"ver":1,"type":"state","t":256,"sequence":812,"keyframeSeq":804,"serverTime":1710000000456,' +
  '"config":{"width":2400,"height":1800},"players":[{"id":"player-2","x":672,"y":416,"facing":"right","maxHealth":100,"health":100}],' +
  '"displayPlayers":{"player-2":{"x":672,"y":416}},"effect_spawned":[{"tick":256,"seq":1,' +
  '"instance":{"id":"contract-effect-zero-motion","definitionId":"attack","ownerActorId":"player-2","deliveryState":{' +
  '"geometry":{"shape":"rect","width":16,"height":16},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},' +
  '"behaviorState":{"ticksRemaining":8},"params":{"lifetimeTicks":8}}}],"effect_seq_cursors":{"contract-effect-zero-motion":1},' +
  '"patches":[{"kind":"player_pos","entityId":"player-2","payload":{"x":672,"y":416}}]}';

describe("effect anchoring – zero motion melee", () => {
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

  test("zero-motion melee effects should anchor near their owner", () => {
    const store = createEffectTestStore({
      player: { id: "player-2", x: 672, y: 416, facing: "right", maxHealth: 100, health: 100 },
    });

    ingestStateMessage(store, ZERO_MOTION_MESSAGE);

    const spawnSpy = vi.spyOn(EffectManager.prototype, "spawn");

    try {
      startRenderLoop(store);
      expect(typeof rafCallback).toBe("function");
      rafCallback(performance.now() + 16);

      expect(spawnSpy).toHaveBeenCalledTimes(1);
      const [, options] = spawnSpy.mock.calls[0] ?? [];
      expect(options?.x).toBeGreaterThan(600);
      expect(options?.y).toBeGreaterThan(350);
    } finally {
      spawnSpy.mockRestore();
    }
  });
});
