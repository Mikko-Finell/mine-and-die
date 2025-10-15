import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import { ensureEffectLifecycleState } from "../effect-lifecycle.js";
import { startRenderLoop } from "../render.js";
import { createEffectTestStore } from "./__helpers__/effect-test-store.js";
import { ingestStateMessage } from "./__helpers__/ingest-state-message.js";

const FIRST_LIFECYCLE_MESSAGE =
  '{"ver":1,"type":"state","t":512,"sequence":1024,"keyframeSeq":1010,"serverTime":1710000000789,' +
  '"config":{"width":2400,"height":1800},"players":[{"id":"player-legacy","x":608,"y":288,"facing":"up","maxHealth":100,"health":100}],' +
  '"effect_spawned":[{"tick":512,"seq":1,"instance":{"id":"effect-legacy","definitionId":"attack","definition":{"typeId":"melee-swing"},"ownerActorId":"player-legacy",' +
  '"deliveryState":{"geometry":{"shape":"rect","width":14,"height":14},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},' +
  '"behaviorState":{"ticksRemaining":3},"params":{"lifetimeTicks":3}}}],"effect_update":[{"tick":513,"seq":2,"id":"effect-legacy",' +
  '"deliveryState":{"geometry":{"shape":"rect","width":14,"height":14},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"}}],' +
  '"effect_ended":[{"tick":514,"seq":3,"id":"effect-legacy","reason":"expired"}],"effect_seq_cursors":{"effect-legacy":3},' +
  '"patches":[{"kind":"player_pos","entityId":"player-legacy","payload":{"x":608,"y":288}}]}';

const REUSED_ID_MESSAGE =
  '{"ver":1,"type":"state","t":520,"sequence":1032,"keyframeSeq":1018,"serverTime":1710000000999,' +
  '"config":{"width":2400,"height":1800},"players":[{"id":"player-legacy","x":608,"y":288,"facing":"up","maxHealth":100,"health":100}],' +
  '"effect_spawned":[{"tick":520,"seq":1,"instance":{"id":"effect-legacy","definitionId":"attack","definition":{"typeId":"melee-swing"},"ownerActorId":"player-legacy",' +
  '"deliveryState":{"geometry":{"shape":"rect","width":14,"height":14},"motion":{"positionX":0,"positionY":0,"velocityX":0,"velocityY":0},"follow":"none"},' +
  '"behaviorState":{"ticksRemaining":4},"params":{"lifetimeTicks":4}}}],"effect_seq_cursors":{"effect-legacy":1}}';

describe("effect lifecycle – reused identifiers", () => {
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

  test("client rejects reused effect IDs", () => {
    const store = createEffectTestStore({
      player: { id: "player-legacy", x: 608, y: 288, facing: "up", maxHealth: 100, health: 100 },
    });

    ingestStateMessage(store, FIRST_LIFECYCLE_MESSAGE);

    const lifecycleAfterFirst = ensureEffectLifecycleState(store);
    expect(lifecycleAfterFirst.lastSeqById.get("effect-legacy")).toBe(3);
    expect(lifecycleAfterFirst.instances.size).toBe(0);

    const consoleError = vi.spyOn(console, "error").mockImplementation(() => {});

    try {
      const { lifecycleSummary } = ingestStateMessage(store, REUSED_ID_MESSAGE);
      expect(lifecycleSummary.spawns).toEqual([]);
      expect(lifecycleSummary.droppedSpawns).toEqual(["effect-legacy"]);

      const lifecycleAfterReuse = ensureEffectLifecycleState(store);
      expect(lifecycleAfterReuse.instances.has("effect-legacy")).toBe(false);

      startRenderLoop(store);
      expect(typeof rafCallback).toBe("function");
      rafCallback(performance.now() + 16);

      const tracked = store.effectManager.getTrackedInstances("melee-swing");
      expect(tracked.size).toBe(0);
      expect(consoleError).toHaveBeenCalled();
    } finally {
      consoleError.mockRestore();
    }
  });
});
