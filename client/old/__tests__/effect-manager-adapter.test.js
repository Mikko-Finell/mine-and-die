import { describe, expect, test } from "vitest";
import {
  ensureEffectRegistry,
  mirrorEffectInstances,
  getEffectInstanceById,
  removeEffectInstance,
} from "../effect-manager-adapter.js";

describe("effect-manager-adapter", () => {
  test("ensureEffectRegistry reuses existing map", () => {
    const store = { effectInstancesById: new Map([["alpha", { id: "alpha" }]]) };
    const registry = ensureEffectRegistry(store);
    expect(registry).toBe(store.effectInstancesById);
    expect(registry.get("alpha")).toEqual({ id: "alpha" });
  });

  test("ensureEffectRegistry creates map when missing", () => {
    const store = {};
    const registry = ensureEffectRegistry(store);
    expect(registry).toBeInstanceOf(Map);
    expect(store.effectInstancesById).toBe(registry);
    expect(registry.size).toBe(0);
  });

  test("mirrorEffectInstances copies metadata entries", () => {
    const instanceA = { type: "fire" };
    const instanceB = { type: "melee" };
    const manager = {
      instanceMetadata: new Map([
        [instanceA, { id: "effect-1", type: "fire" }],
        [instanceB, { id: "effect-2", type: "melee" }],
      ]),
    };
    const store = { effectInstancesById: new Map([["stale", { id: "stale" }]]) };
    const mirrored = mirrorEffectInstances(store, manager);
    expect(mirrored).toBe(store.effectInstancesById);
    expect(mirrored.size).toBe(2);
    expect(mirrored.get("effect-1")).toBe(instanceA);
    expect(mirrored.get("effect-2")).toBe(instanceB);
    expect(mirrored.has("stale")).toBe(false);
  });

  test("mirrorEffectInstances tolerates missing metadata", () => {
    const store = { effectInstancesById: new Map([["preserve", {}]]) };
    const mirrored = mirrorEffectInstances(store, { instanceMetadata: null });
    expect(mirrored.size).toBe(0);
  });

  test("getEffectInstanceById reads mirrored entries", () => {
    const instance = { id: "effect-99" };
    const store = { effectInstancesById: new Map([["effect-99", instance]]) };
    expect(getEffectInstanceById(store, "effect-99")).toBe(instance);
    expect(getEffectInstanceById(store, "missing")).toBeNull();
  });

  test("removeEffectInstance deletes entry when present", () => {
    const store = { effectInstancesById: new Map([["effect-7", { id: "effect-7" }]]) };
    expect(removeEffectInstance(store, "effect-7")).toBe(true);
    expect(store.effectInstancesById.size).toBe(0);
    expect(removeEffectInstance(store, "effect-7")).toBe(false);
  });
});
