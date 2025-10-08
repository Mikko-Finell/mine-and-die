import { describe, expect, it } from "vitest";
import {
  DEFAULT_WORLD_HEIGHT,
  DEFAULT_WORLD_SEED,
  DEFAULT_WORLD_WIDTH,
  normalizeCount,
  normalizeGroundItems,
  normalizeWorldConfig,
  splitNpcCounts,
} from "../network.js";

const DEFAULT_COUNTS = {
  goblinCount: 2,
  ratCount: 1,
  npcCount: 3,
};

describe("splitNpcCounts", () => {
  it.each([
    { total: 0, expected: { goblinCount: 0, ratCount: 0, npcCount: 0 } },
    { total: 1, expected: { goblinCount: 1, ratCount: 0, npcCount: 1 } },
    { total: 2, expected: { goblinCount: 2, ratCount: 0, npcCount: 2 } },
    { total: 3, expected: { goblinCount: 2, ratCount: 1, npcCount: 3 } },
    { total: 10, expected: { goblinCount: 2, ratCount: 8, npcCount: 10 } },
  ])(
    "splits totals across goblins and rats when only npcCount = $total",
    ({ total, expected }) => {
      const result = splitNpcCounts({ npcCount: total }, DEFAULT_COUNTS);
      expect(result).toEqual(expected);
      expect(result.npcCount).toBe(result.goblinCount + result.ratCount);
    },
  );

  it.each([
    {
      label: "only goblin count provided",
      input: { goblinCount: 5 },
      expected: { goblinCount: 5, ratCount: 1, npcCount: 6 },
    },
    {
      label: "only rat count provided",
      input: { ratCount: 4 },
      expected: { goblinCount: 2, ratCount: 4, npcCount: 6 },
    },
    {
      label: "both goblin and rat provided",
      input: { goblinCount: 3, ratCount: 7 },
      expected: { goblinCount: 3, ratCount: 7, npcCount: 10 },
    },
  ])("respects explicit per-type counts when $label", ({ input, expected }) => {
    const result = splitNpcCounts(input, DEFAULT_COUNTS);
    expect(result).toEqual(expected);
    expect(result.npcCount).toBe(result.goblinCount + result.ratCount);
  });

  it.each([
    {
      label: "negative goblin count",
      input: { goblinCount: -5 },
      expected: { goblinCount: 0, ratCount: 1, npcCount: 1 },
    },
    {
      label: "float goblin count",
      input: { goblinCount: 2.9 },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
    {
      label: "string rat count",
      input: { ratCount: "4" },
      expected: { goblinCount: 2, ratCount: 4, npcCount: 6 },
    },
    {
      label: "null npc count",
      input: { npcCount: null },
      expected: { goblinCount: 0, ratCount: 0, npcCount: 0 },
    },
    {
      label: "undefined npc count",
      input: { npcCount: undefined },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
    {
      label: "NaN npc count",
      input: { npcCount: Number.NaN },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
    {
      label: "float npc count",
      input: { npcCount: 3.7 },
      expected: { goblinCount: 2, ratCount: 1, npcCount: 3 },
    },
  ])("normalizes $label", ({ input, expected }) => {
    const result = splitNpcCounts(input, DEFAULT_COUNTS);
    expect(result).toEqual(expected);
    expect(result.npcCount).toBe(result.goblinCount + result.ratCount);
  });
});

describe("normalizeWorldConfig", () => {
  it("respects boolean toggles and coerces the seed", () => {
    const result = normalizeWorldConfig({
      obstacles: false,
      obstaclesCount: "3.8",
      goldMines: false,
      goldMineCount: "5",
      npcs: false,
      goblinCount: "4",
      ratCount: "1",
      lava: false,
      lavaCount: "7",
      seed: 42,
    });

    expect(result.obstacles).toBe(false);
    expect(result.goldMines).toBe(false);
    expect(result.npcs).toBe(false);
    expect(result.lava).toBe(false);
    expect(result.seed).toBe("42");
    expect(result.goblinCount + result.ratCount).toBe(result.npcCount);
  });

  it("trims string seeds and falls back when empty", () => {
    const result = normalizeWorldConfig({ seed: "  custom-seed  " });
    expect(result.seed).toBe("custom-seed");

    const fallback = normalizeWorldConfig({ seed: "   " });
    expect(fallback.seed).toBe(DEFAULT_WORLD_SEED);
  });

  it.each([
    { width: 1200, height: 800 },
    { width: "3600", height: "2400" },
  ])("accepts positive finite width/height values", ({ width, height }) => {
    const result = normalizeWorldConfig({ width, height });
    expect(result.width).toBe(Number(width));
    expect(result.height).toBe(Number(height));
  });

  it.each([
    { width: 0, height: 0 },
    { width: -1, height: -2 },
    { width: "invalid", height: "bad" },
    { width: Infinity, height: -Infinity },
  ])("falls back to defaults for invalid dimensions", ({ width, height }) => {
    const result = normalizeWorldConfig({ width, height });
    expect(result.width).toBe(DEFAULT_WORLD_WIDTH);
    expect(result.height).toBe(DEFAULT_WORLD_HEIGHT);
  });

  it("normalizes npc totals even with partial inputs", () => {
    const result = normalizeWorldConfig({ npcCount: 1, ratCount: 4 });
    expect(result.goblinCount + result.ratCount).toBe(result.npcCount);
    expect(result.ratCount).toBe(4);
  });
});

describe("normalizeCount", () => {
  it.each([
    { value: 5, fallback: 2, expected: 5 },
    { value: "3.9", fallback: 2, expected: 3 },
    { value: -2, fallback: 4, expected: 0 },
    { value: Number.NaN, fallback: 7, expected: 7 },
  ])("normalizes $value with fallback $fallback", ({ value, fallback, expected }) => {
    expect(normalizeCount(value, fallback)).toBe(expected);
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
