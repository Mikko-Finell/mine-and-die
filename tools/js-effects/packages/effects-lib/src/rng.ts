import type { RandomGenerator } from "./types.js";

const xmur3 = (str: string) => {
  let h = 1779033703 ^ str.length;
  for (let i = 0; i < str.length; i += 1) {
    h = Math.imul(h ^ str.charCodeAt(i), 3432918353);
    h = (h << 13) | (h >>> 19);
  }
  return () => {
    h = Math.imul(h ^ (h >>> 16), 2246822507);
    h = Math.imul(h ^ (h >>> 13), 3266489909);
    h ^= h >>> 16;
    return h >>> 0;
  };
};

const fromSeedValue = (seed: string | number | undefined) => {
  if (typeof seed === "number" && Number.isFinite(seed)) {
    return seed >>> 0;
  }
  if (typeof seed === "string") {
    const seedHash = xmur3(seed);
    return seedHash();
  }
  return Math.floor(Math.random() * 2 ** 32);
};

const mulberry32 = (a: number) => () => {
  let t = (a += 0x6d2b79f5);
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};

/**
 * Creates a deterministic pseudo-random generator that can be reseeded from
 * stable string ids at runtime. The generator is intentionally lightweight so
 * hosts can create or reset it per frame without significant overhead.
 */
export const createRandomGenerator = (
  seed?: string | number
): RandomGenerator => {
  let state = mulberry32(fromSeedValue(seed));

  const reseed = (id: string) => {
    state = mulberry32(fromSeedValue(id));
  };

  return {
    next: () => state(),
    seedFrom: reseed,
  };
};
