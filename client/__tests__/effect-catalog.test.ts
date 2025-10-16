import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";
import {
  getEffectCatalog,
  normalizeEffectCatalog,
  setEffectCatalog,
  subscribeEffectCatalog,
  type EffectCatalogSnapshot,
} from "../effect-catalog";

describe("effect catalog store", () => {
  beforeEach(() => {
    setEffectCatalog(null);
  });

  afterEach(() => {
    setEffectCatalog(null);
  });

  test("normalizes catalog input and provides immutable snapshots", () => {
    const normalized = normalizeEffectCatalog({
      slash: {
        contractId: "attack",
        blocks: { damage: 12 },
      },
    });

    setEffectCatalog(normalized);
    const snapshot = getEffectCatalog();

    expect(Object.isFrozen(snapshot)).toBe(true);
    expect(snapshot).not.toBe(normalized);
    expect(snapshot.slash?.contractId).toBe("attack");
    expect(snapshot.slash?.blocks.damage).toBe(12);
  });

  test("notifies subscribers immediately and on subsequent updates", () => {
    const received: EffectCatalogSnapshot[] = [];
    const unsubscribe = subscribeEffectCatalog((catalog) => {
      received.push(catalog);
    });

    const first = normalizeEffectCatalog({
      slash: { contractId: "attack" },
    });
    setEffectCatalog(first);

    const second = normalizeEffectCatalog({
      slash: { contractId: "attack" },
      frostbite: { contractId: "frost" },
    });
    setEffectCatalog(second);

    unsubscribe();

    expect(received).toHaveLength(3);
    expect(Object.keys(received[0])).toHaveLength(0);
    expect(Object.keys(received[1])).toEqual(["slash"]);
    expect(Object.keys(received[2]).sort()).toEqual(["frostbite", "slash"]);
  });

  test("stops notifying listeners after unsubscribe", () => {
    const listener = vi.fn();
    const unsubscribe = subscribeEffectCatalog(listener);
    unsubscribe();

    const payload = normalizeEffectCatalog({
      slash: { contractId: "attack" },
    });
    setEffectCatalog(payload);

    expect(listener).toHaveBeenCalledTimes(1);
  });

  test("throws when subscribing with a non-function listener", () => {
    expect(() => subscribeEffectCatalog(null as unknown as () => void)).toThrowError(
      /listener must be a function/i,
    );
  });
});
