import { describe, expect, test } from "vitest";
import { contractLifecycleToEffect } from "../effect-lifecycle-translator.js";

describe("contractLifecycleToEffect", () => {
  test("translates geometry, duration, and params from lifecycle entry", () => {
    const lifecycleEntry = {
      instance: {
        id: "contract-1",
        definitionId: "attack",
        definition: { typeId: "melee-swing" },
        deliveryState: {
          geometry: { shape: "rect", width: 32, height: 16 },
          motion: { positionX: 160, positionY: 200 },
        },
        behaviorState: { ticksRemaining: 3, extra: { damage: 12 } },
        params: { fadeExponent: 2 },
      },
    };
    const result = contractLifecycleToEffect(lifecycleEntry, {
      store: { TILE_SIZE: 40 },
    });

    expect(result.id).toBe("contract-1");
    expect(result.type).toBe("melee-swing");
    expect(result.width).toBeCloseTo(80);
    expect(result.height).toBeCloseTo(40);
    expect(result.x).toBeCloseTo(360);
    expect(result.y).toBeCloseTo(480);
    expect(result.duration).toBeCloseTo((1000 / 15) * 3, 5);
    expect(result.params).toEqual({ fadeExponent: 2, damage: 12 });
  });

  test("returns null when lifecycle entry missing", () => {
    const result = contractLifecycleToEffect(null, {
      store: { TILE_SIZE: 40 },
    });

    expect(result).toBeNull();
  });

  test("derives position from anchor offsets when motion is absent", () => {
    const lifecycleEntry = {
      instance: {
        id: "contract-anchor",
        definitionId: "fireball",
        definition: { typeId: "fireball" },
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
    const result = contractLifecycleToEffect(lifecycleEntry, {});

    expect(result.colors).toEqual(["#7a0e12", "#4a090b"]);
  });

  test("uses quantized center coordinates when owner anchor differs", () => {
    const tileSize = 40;
    const quantize = (world) => Math.round((world / tileSize) * 16);
    const lifecycleEntry = {
      instance: {
        id: "contract-blood", 
        definitionId: "blood-splatter",
        ownerActorId: "player-1",
        deliveryState: {
          geometry: { shape: "rect", width: quantize(40), height: quantize(40) },
          motion: { positionX: 0, positionY: 0 },
        },
        behaviorState: {
          ticksRemaining: 10,
          extra: { centerX: quantize(400), centerY: quantize(560) },
        },
      },
    };
    const store = {
      TILE_SIZE: tileSize,
      players: { "player-1": { x: 1000, y: 1200 } },
    };

    const result = contractLifecycleToEffect(lifecycleEntry, { store });

    expect(result.width).toBeCloseTo(40);
    expect(result.height).toBeCloseTo(40);
    expect(result.x).toBeCloseTo(380);
    expect(result.y).toBeCloseTo(540);
    expect(result.params?.centerX).toBeCloseTo(400);
    expect(result.params?.centerY).toBeCloseTo(560);
  });
});
