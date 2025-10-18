import { describe, expect, it, vi } from "vitest";

import { InMemoryInputStore } from "../input";

describe("InMemoryInputStore", () => {
  it("tracks state snapshots, path targets, and emits callbacks", () => {
    const onIntent = vi.fn();
    const onFacing = vi.fn();
    const onPathActive = vi.fn();
    const onPathTarget = vi.fn();
    const onCameraLock = vi.fn();

    const store = new InMemoryInputStore({
      initialFacing: "left",
      initialPathActive: true,
      initialPathTarget: { x: 160, y: 208 },
      initialPressedKeys: ["A", "w"],
      initialDirectionOrder: ["w", "A"],
      onIntentChanged: onIntent,
      onFacingChanged: onFacing,
      onPathActiveChanged: onPathActive,
      onPathTargetChanged: onPathTarget,
      onCameraLockToggle: onCameraLock,
    });

    const initialSnapshot = store.getState();
    expect(initialSnapshot.currentFacing).toBe("left");
    expect(initialSnapshot.pathActive).toBe(true);
    expect(initialSnapshot.pathTarget).toEqual({ x: 160, y: 208 });
    expect(Array.from(initialSnapshot.pressedKeys)).toEqual(["a", "w"]);
    expect(initialSnapshot.directionOrder).toEqual(["w", "a"]);
    expect(store.getState().pressedKeys).not.toBe(initialSnapshot.pressedKeys);
    expect(store.getPathTarget()).toEqual({ x: 160, y: 208 });

    store.setIntent({ dx: 1, dy: 0, facing: "right" });
    expect(onIntent).toHaveBeenCalledWith({ dx: 1, dy: 0, facing: "right" });
    expect(store.getState().currentFacing).toBe("right");

    store.updateFacing("up");
    expect(onFacing).toHaveBeenCalledWith("up");
    expect(store.getState().currentFacing).toBe("up");

    store.setPathTarget?.({ x: 512, y: 384 });
    expect(onPathTarget).toHaveBeenCalledWith({ x: 512, y: 384 });
    const targetSnapshot = store.getPathTarget();
    expect(targetSnapshot).toEqual({ x: 512, y: 384 });
    if (targetSnapshot) {
      targetSnapshot.x = 0;
    }
    expect(store.getPathTarget()).toEqual({ x: 512, y: 384 });

    store.setPathActive(false);
    expect(onPathActive).toHaveBeenCalledWith(false);
    expect(store.getState().pathActive).toBe(false);
    expect(onPathTarget).toHaveBeenLastCalledWith(null);
    expect(store.getPathTarget()).toBeNull();

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

  it("avoids emitting redundant path target updates", () => {
    const onPathTarget = vi.fn();
    const store = new InMemoryInputStore({ onPathTargetChanged: onPathTarget });

    store.setPathTarget?.(null);
    expect(onPathTarget).not.toHaveBeenCalled();

    store.setPathTarget?.({ x: 32, y: 64 });
    expect(onPathTarget).toHaveBeenLastCalledWith({ x: 32, y: 64 });

    store.setPathTarget?.({ x: 32, y: 64 });
    expect(onPathTarget).toHaveBeenCalledTimes(1);
  });

  it("tracks and clears command rejection details", () => {
    const onCommandRejection = vi.fn();
    const store = new InMemoryInputStore({ onCommandRejectionChanged: onCommandRejection });

    store.setCommandRejection?.({
      sequence: 3,
      reason: "queue_limit",
      retry: false,
      tick: 12,
      kind: "path",
    });

    const snapshot = store.getState();
    expect(snapshot.lastCommandRejection).toEqual({
      sequence: 3,
      reason: "queue_limit",
      retry: false,
      tick: 12,
      kind: "path",
    });
    expect(onCommandRejection).toHaveBeenCalledWith({
      sequence: 3,
      reason: "queue_limit",
      retry: false,
      tick: 12,
      kind: "path",
    });

    const mutationTarget = snapshot.lastCommandRejection;
    if (mutationTarget) {
      mutationTarget.reason = "mutated";
    }
    expect(store.getState().lastCommandRejection).toEqual({
      sequence: 3,
      reason: "queue_limit",
      retry: false,
      tick: 12,
      kind: "path",
    });

    store.clearCommandRejection?.("action");
    expect(store.getState().lastCommandRejection).not.toBeNull();

    store.clearCommandRejection?.("path");
    expect(store.getState().lastCommandRejection).toBeNull();
    expect(onCommandRejection).toHaveBeenLastCalledWith(null);
  });
});
