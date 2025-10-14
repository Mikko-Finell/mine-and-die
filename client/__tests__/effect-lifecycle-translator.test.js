import { describe, expect, test } from "vitest";
import { contractLifecycleToEffect } from "../effect-lifecycle-translator.js";

describe("contractLifecycleToEffect", () => {
  test("translates geometry, duration, and params from lifecycle entry", () => {
    const lifecycleEntry = {
      instance: {
        id: "contract-1",
        definitionId: "attack",
        deliveryState: {
          geometry: { shape: "rect", width: 32, height: 16 },
          motion: { positionX: 160, positionY: 200 },
        },
        behaviorState: { ticksRemaining: 3, extra: { damage: 12 } },
        params: { fadeExponent: 2 },
      },
    };
    const fallbackEffect = {
      id: "legacy-attack",
      width: 48,
      height: 60,
      params: { fadeExponent: 1, jitter: 4 },
    };
    const result = contractLifecycleToEffect(lifecycleEntry, {
      store: { TILE_SIZE: 40 },
      fallbackEffect,
    });

    expect(result.id).toBe("contract-1");
    expect(result.type).toBe("attack");
    expect(result.width).toBeCloseTo(80);
    expect(result.height).toBeCloseTo(40);
    expect(result.x).toBeCloseTo(360);
    expect(result.y).toBeCloseTo(480);
    expect(result.duration).toBeCloseTo((1000 / 15) * 3, 5);
    expect(result.params).toEqual({ fadeExponent: 2, jitter: 4, damage: 12 });
  });

  test("falls back to cloning the legacy effect when lifecycle data missing", () => {
    const fallbackEffect = { id: "legacy-fire", x: 10, y: 20, params: { size: 2 } };
    const result = contractLifecycleToEffect(null, {
      store: { TILE_SIZE: 40 },
      fallbackEffect,
    });

    expect(result).toEqual(fallbackEffect);
    expect(result).not.toBe(fallbackEffect);
  });

  test("derives position from anchor offsets when motion is absent", () => {
    const lifecycleEntry = {
      instance: {
        id: "contract-anchor",
        definitionId: "fireball",
        ownerActorId: "npc-1",
        deliveryState: {
          geometry: {
            shape: "rect",
            width: 16,
            height: 16,
            offsetX: 8,
            offsetY: -4,
          },
        },
        behaviorState: {},
      },
    };
    const store = {
      TILE_SIZE: 40,
      npcs: { "npc-1": { x: 200, y: 300 } },
    };
    const renderState = {
      npcs: { "npc-1": { x: 220, y: 320 } },
    };

    const result = contractLifecycleToEffect(lifecycleEntry, {
      store,
      renderState,
    });

    expect(result.width).toBeCloseTo(40);
    expect(result.height).toBeCloseTo(40);
    expect(result.x).toBeCloseTo(220);
    expect(result.y).toBeCloseTo(290);
  });

  test("prefers lifecycle colors and trims fallback palettes", () => {
    const lifecycleEntry = {
      instance: {
        id: "contract-colors",
        definitionId: "blood-splatter",
        colors: [" #7a0e12 ", "", "#4a090b", null],
      },
    };
    const fallbackEffect = {
      id: "legacy-colors",
      colors: ["#123456", "#654321"],
    };

    const result = contractLifecycleToEffect(lifecycleEntry, {
      fallbackEffect,
    });

    expect(result.colors).toEqual(["#7a0e12", "#4a090b"]);
  });

  test("resolves actor anchors stored in Maps", () => {
    const lifecycleEntry = {
      instance: {
        id: "contract-map-attack",
        definitionId: "attack",
        ownerActorId: "player-map",
        deliveryState: {
          geometry: {
            shape: "rect",
            width: 16,
            height: 16,
          },
        },
        behaviorState: {
          ticksRemaining: 1,
        },
      },
    };

    const renderState = {
      players: new Map([["player-map", { x: 320, y: 160 }]]),
    };

    const result = contractLifecycleToEffect(lifecycleEntry, {
      store: { TILE_SIZE: 40 },
      renderState,
    });

    expect(result.width).toBeCloseTo(40);
    expect(result.height).toBeCloseTo(40);
    expect(result.x).toBeCloseTo(300);
    expect(result.y).toBeCloseTo(140);
  });
});
