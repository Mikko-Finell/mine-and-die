import { describe, expect, it } from "vitest";
import { normalizeGroundItems, normalizeWorldConfig } from "./network.js";

describe("normalizeWorldConfig", () => {
  it("fills defaults while normalizing provided values", () => {
    const result = normalizeWorldConfig({
      obstacles: false,
      obstaclesCount: "7.6",
      goldMines: true,
      goldMineCount: -4,
      npcs: true,
      npcCount: 1,
      lavaCount: "3.2",
      seed: "  custom-seed  ",
      width: "900",
      height: null,
    });

    expect(result).toMatchObject({
      obstacles: false,
      obstaclesCount: 7,
      goldMines: true,
      goldMineCount: 0,
      npcs: true,
      goblinCount: 1,
      ratCount: 0,
      npcCount: 1,
      lava: true,
      lavaCount: 3,
      seed: "custom-seed",
      width: 900,
      height: 1800,
    });
  });
});

describe("normalizeGroundItems", () => {
  it("skips invalid entries and defaults missing numbers", () => {
    const result = normalizeGroundItems([
      { id: "ore", x: 10.5, y: "20", qty: "3" },
      null,
      { id: 42, x: "bad", y: 9, qty: {} },
      { id: "gem", x: Infinity, y: -5, qty: -2 },
      { id: "empty" },
    ]);

    expect(result).toEqual({
      ore: { id: "ore", x: 10.5, y: 20, qty: 3 },
      gem: { id: "gem", x: 0, y: -5, qty: -2 },
      empty: { id: "empty", x: 0, y: 0, qty: 0 },
    });
  });
});
