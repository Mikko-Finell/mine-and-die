import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import {
  getEffectCatalog,
  normalizeEffectCatalog,
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

  test("normalizes join payloads against generated metadata", () => {
    const payload = JSON.parse(JSON.stringify(generatedEffectCatalog));

    const normalized = normalizeEffectCatalog(payload);
    expect(normalized).toEqual(generatedEffectCatalog);
    expect(normalized).not.toBe(payload);
    expect(Object.isFrozen(normalized)).toBe(true);

    const subsequent = normalizeEffectCatalog(payload);
    expect(subsequent).not.toBe(normalized);

    expect(() => normalizeEffectCatalog({})).toThrowError(/effect catalog mismatch/i);
    expect(() => normalizeEffectCatalog(null)).toThrowError(/must be an object/i);

    const mutated = JSON.parse(JSON.stringify(generatedEffectCatalog));
    mutated.attack.managedByClient = false;
    expect(() => normalizeEffectCatalog(mutated)).toThrowError(/does not match generated metadata/i);
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
