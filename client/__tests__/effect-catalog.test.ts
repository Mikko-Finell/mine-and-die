import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import {
  getEffectCatalog,
  setEffectCatalog,
  subscribeEffectCatalog,
  type EffectCatalogSnapshot,
} from "../effect-catalog";
import { effectCatalog as generatedEffectCatalog } from "../generated/effect-contracts";

describe("effect catalog store", () => {
  beforeEach(() => {
    setEffectCatalog(null);
  });

  afterEach(() => {
    setEffectCatalog(null);
  });

  test("exposes a frozen snapshot of the generated metadata", () => {
    const snapshot = getEffectCatalog();
    expect(snapshot).toEqual(generatedEffectCatalog);
    expect(Object.isFrozen(snapshot)).toBe(true);

    const clone = JSON.parse(JSON.stringify(snapshot));
    clone.attack.blocks.jsEffect = "mutated";
    expect(snapshot.attack.blocks.jsEffect).toBe("melee/swing");
  });

  test("notifies subscribers immediately and on subsequent updates", () => {
    const received: ReturnType<typeof getEffectCatalog>[] = [];
    const unsubscribe = subscribeEffectCatalog((catalog) => {
      received.push(catalog);
    });

    setEffectCatalog({ attack: generatedEffectCatalog.attack });
    setEffectCatalog({
      attack: generatedEffectCatalog.attack,
      fireball: generatedEffectCatalog.fireball,
    });

    unsubscribe();

    expect(received).toHaveLength(3);
    expect(Object.keys(received[0]).sort()).toEqual(Object.keys(generatedEffectCatalog));
    expect(Object.keys(received[1])).toEqual(["attack"]);
    expect(Object.keys(received[2]).sort()).toEqual(["attack", "fireball"]);
    expect(Object.isFrozen(received[1])).toBe(true);
    expect(Object.isFrozen(received[2])).toBe(true);
  });

  test("stops notifying listeners after unsubscribe", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeEffectCatalog(listener);
    unsubscribe();

    setEffectCatalog({ attack: generatedEffectCatalog.attack });

    expect(listener).toHaveBeenCalledTimes(1);
  });

  test("throws when subscribing with a non-function listener", () => {
    expect(() => subscribeEffectCatalog(null as unknown as () => void)).toThrowError(
      /listener must be a function/i,
    );
  });

  test("does not retain references to mutable inputs", () => {
    const mutable = JSON.parse(
      JSON.stringify({ attack: generatedEffectCatalog.attack }),
    ) as Record<string, any>;

    setEffectCatalog(mutable as unknown as EffectCatalogSnapshot);
    mutable.attack.blocks.jsEffect = "mutated";

    const snapshot = getEffectCatalog();
    expect(snapshot.attack.blocks.jsEffect).toBe("melee/swing");
  });
});
