import { describe, expect, it, vi } from "vitest";

import { InMemoryInputStore } from "../input";

describe("InMemoryInputStore", () => {
  it("tracks state snapshots and emits callbacks", () => {
    const onIntent = vi.fn();
    const onFacing = vi.fn();
    const onPathActive = vi.fn();
    const onCameraLock = vi.fn();

    const store = new InMemoryInputStore({
      initialFacing: "left",
      initialPathActive: true,
      initialPressedKeys: ["A", "w"],
      initialDirectionOrder: ["w", "A"],
      onIntentChanged: onIntent,
      onFacingChanged: onFacing,
      onPathActiveChanged: onPathActive,
      onCameraLockToggle: onCameraLock,
    });

    const initialSnapshot = store.getState();
    expect(initialSnapshot.currentFacing).toBe("left");
    expect(initialSnapshot.pathActive).toBe(true);
    expect(Array.from(initialSnapshot.pressedKeys)).toEqual(["a", "w"]);
    expect(initialSnapshot.directionOrder).toEqual(["w", "a"]);
    expect(store.getState().pressedKeys).not.toBe(initialSnapshot.pressedKeys);

    store.setIntent({ dx: 1, dy: 0, facing: "right" });
    expect(onIntent).toHaveBeenCalledWith({ dx: 1, dy: 0, facing: "right" });
    expect(store.getState().currentFacing).toBe("right");

    store.updateFacing("up");
    expect(onFacing).toHaveBeenCalledWith("up");
    expect(store.getState().currentFacing).toBe("up");

    store.setPathActive(false);
    expect(onPathActive).toHaveBeenCalledWith(false);
    expect(store.getState().pathActive).toBe(false);

    store.toggleCameraLock();
    expect(onCameraLock).toHaveBeenLastCalledWith(true);
    store.toggleCameraLock();
    expect(onCameraLock).toHaveBeenLastCalledWith(false);

    const lastIntent = store.getLastIntent();
    lastIntent.dx = 42;
    expect(store.getLastIntent()).toEqual({ dx: 1, dy: 0, facing: "right" });
  });

  it("normalizes key state snapshots", () => {
    const store = new InMemoryInputStore();

    store.setKeyState?.({
      pressedKeys: new Set(["W", "d"]),
      directionOrder: ["D", "w"],
    });

    const snapshot = store.getState();
    expect(Array.from(snapshot.pressedKeys)).toEqual(["w", "d"]);
    expect(snapshot.directionOrder).toEqual(["d", "w"]);

    const mutated = store.getState();
    const orderCopy = [...mutated.directionOrder];
    orderCopy.push("x");
    expect(store.getState().directionOrder).toEqual(["d", "w"]);
  });
});
