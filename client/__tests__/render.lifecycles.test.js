import { describe, expect, test } from "vitest";
import { EffectManager } from "../js-effects/manager.js";
import { __testing__ } from "../render.js";

const { syncEffectsByType, CLIENT_MANAGED_EFFECT_MAX_AGE_MS } = __testing__;

function createTestDefinition(type) {
  return {
    type,
    create: (opts = {}) => {
      const effectId =
        typeof opts.effectId === "string" && opts.effectId.length > 0
          ? opts.effectId
          : `${type}-${Math.random().toString(36).slice(2)}`;
      return {
        id: effectId,
        type,
        layer: 0,
        aabb: { x: 0, y: 0, w: 1, h: 1 },
        isAlive: () => true,
        getAABB() {
          return this.aabb;
        },
        update() {},
        draw() {},
        dispose() {},
      };
    },
    fromEffect: (effect) => ({
      effectId: typeof effect?.id === "string" ? effect.id : undefined,
    }),
  };
}

function createLifecycleEntry({ id, definitionId, managedByClient }) {
  return {
    instance: {
      id,
      definitionId,
      definition: {
        client: { managedByClient: !!managedByClient },
      },
      behaviorState: {},
      deliveryState: {},
    },
  };
}

function createLifecycleView(entriesMap) {
  const entries = entriesMap ?? new Map();
  const recentlyEnded = new Map();
  return {
    entries,
    recentlyEnded,
    getEntry(effectId) {
      if (!effectId) {
        return null;
      }
      return entries.get(effectId) ?? recentlyEnded.get(effectId) ?? null;
    },
  };
}

function createContractEffect(id, type, { managedByClient = false } = {}) {
  const effect = { id, type };
  Object.defineProperty(effect, "__contractDerived", {
    value: true,
    enumerable: false,
    configurable: true,
  });
  Object.defineProperty(effect, "__contractDefinitionId", {
    value: type,
    enumerable: false,
    configurable: true,
  });
  if (managedByClient) {
    Object.defineProperty(effect, "__contractManagedByClient", {
      value: true,
      enumerable: false,
      configurable: true,
    });
  }
  return effect;
}

describe("syncEffectsByType contract lifecycle handling", () => {
  test("client-managed contract effects persist after contract end until GC cap", () => {
    const manager = new EffectManager();
    const definition = createTestDefinition("melee-swing");
    const effectId = "contract-effect-1";
    const lifecycle = createLifecycleView(
      new Map([
        [
          effectId,
          createLifecycleEntry({
            id: effectId,
            definitionId: "attack",
            managedByClient: true,
          }),
        ],
      ]),
    );
    const effect = createContractEffect(effectId, "attack", {
      managedByClient: true,
    });

    const initialFrame = 1000;
    const midFrame = initialFrame + CLIENT_MANAGED_EFFECT_MAX_AGE_MS / 2;
    const expiryFrame =
      initialFrame + CLIENT_MANAGED_EFFECT_MAX_AGE_MS * 2 + 10;

    syncEffectsByType({}, manager, "attack", definition, undefined, [effect], {
      lifecycle,
      frameNow: initialFrame,
    });

    let tracked = manager.getTrackedInstances(definition.type);
    expect(tracked.size).toBe(1);
    const instance = tracked.get(effectId);
    expect(instance).toBeDefined();
    expect(instance.__effectLifecycleClientManaged).toBe(true);

    lifecycle.entries.clear();
    syncEffectsByType({}, manager, "attack", definition, undefined, [], {
      lifecycle,
      frameNow: midFrame,
    });

    tracked = manager.getTrackedInstances(definition.type);
    expect(tracked.has(effectId)).toBe(true);

    syncEffectsByType({}, manager, "attack", definition, undefined, [], {
      lifecycle,
      frameNow: expiryFrame,
    });

    tracked = manager.getTrackedInstances(definition.type);
    expect(tracked.has(effectId)).toBe(false);
  });

  test("server-managed contract effects are removed when their contract entry ends", () => {
    const manager = new EffectManager();
    const definition = createTestDefinition("fireball");
    const effectId = "contract-effect-2";
    const lifecycle = createLifecycleView(
      new Map([
        [
          effectId,
          createLifecycleEntry({
            id: effectId,
            definitionId: "fireball",
            managedByClient: false,
          }),
        ],
      ]),
    );
    const effect = createContractEffect(effectId, "fireball");

    syncEffectsByType({}, manager, "fireball", definition, undefined, [effect], {
      lifecycle,
      frameNow: 2000,
    });

    lifecycle.entries.clear();
    syncEffectsByType({}, manager, "fireball", definition, undefined, [], {
      lifecycle,
      frameNow: 2010,
    });

    const tracked = manager.getTrackedInstances(definition.type);
    expect(tracked.size).toBe(0);
  });

  test("resync reuses client-managed instances and clears stale GC deadlines", () => {
    const manager = new EffectManager();
    const definition = createTestDefinition("melee-swing");
    const effectId = "contract-effect-3";
    const lifecycle = createLifecycleView(
      new Map([
        [
          effectId,
          createLifecycleEntry({
            id: effectId,
            definitionId: "attack",
            managedByClient: true,
          }),
        ],
      ]),
    );
    const effect = createContractEffect(effectId, "attack", {
      managedByClient: true,
    });

    syncEffectsByType({}, manager, "attack", definition, undefined, [effect], {
      lifecycle,
      frameNow: 3000,
    });

    const trackedBefore = manager.getTrackedInstances(definition.type);
    const instance = trackedBefore.get(effectId);
    expect(instance).toBeDefined();

    lifecycle.entries.clear();
    syncEffectsByType({}, manager, "attack", definition, undefined, [], {
      lifecycle,
      frameNow: 3000 + CLIENT_MANAGED_EFFECT_MAX_AGE_MS / 2,
    });
    expect(typeof instance.__effectLifecycleGcDeadline).toBe("number");

    lifecycle.entries.set(
      effectId,
      createLifecycleEntry({
        id: effectId,
        definitionId: "attack",
        managedByClient: true,
      }),
    );
    const refreshedEffect = createContractEffect(effectId, "attack", {
      managedByClient: true,
    });

    syncEffectsByType(
      {},
      manager,
      "attack",
      definition,
      undefined,
      [refreshedEffect],
      {
        lifecycle,
        frameNow: 3000 + CLIENT_MANAGED_EFFECT_MAX_AGE_MS,
      },
    );

    const trackedAfter = manager.getTrackedInstances(definition.type);
    const reusedInstance = trackedAfter.get(effectId);
    expect(reusedInstance).toBe(instance);
    expect(reusedInstance.__effectLifecycleManaged).toBe(true);
    expect(reusedInstance.__effectLifecycleClientManaged).toBe(true);
    expect("__effectLifecycleGcDeadline" in reusedInstance).toBe(false);
  });
});
