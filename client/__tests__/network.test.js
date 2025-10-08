import { describe, expect, it } from "vitest";
import {
  DEFAULT_WORLD_WIDTH,
  normalizeGroundItems,
  normalizeWorldConfig,
} from "../network.js";

describe("normalizeWorldConfig", () => {
  it("fills defaults and sanitizes provided values", () => {
    const result = normalizeWorldConfig({
      obstacles: false,
      goldMineCount: "3.8",
      goblinCount: -5,
      ratCount: "2",
      npcCount: 1,
      lava: false,
      seed: "  custom-seed  ",
      width: "invalid",
      height: 4200,
    });

    expect(result).toEqual({
      obstacles: false,
      obstaclesCount: 2,
      goldMines: true,
      goldMineCount: 3,
      npcs: true,
      goblinCount: 0,
      ratCount: 2,
      npcCount: 2,
      lava: false,
      lavaCount: 3,
      seed: "custom-seed",
      width: DEFAULT_WORLD_WIDTH,
      height: 4200,
    });
  });
});

describe("normalizeGroundItems", () => {
  it("builds an object of sanitized entries", () => {
    const result = normalizeGroundItems([
      { id: "ore-1", x: "12", y: 7.5, qty: "3" },
      { id: "ore-2", x: null, y: undefined, qty: Infinity },
      { id: 0, x: 4, y: 5, qty: 1 },
      null,
    ]);

    expect(result).toEqual({
      "ore-1": { id: "ore-1", x: 12, y: 7.5, qty: 3 },
      "ore-2": { id: "ore-2", x: 0, y: 0, qty: 0 },
    });
  });

  it("returns an empty object when input is not an array", () => {
    expect(normalizeGroundItems(undefined)).toEqual({});
  });
});
