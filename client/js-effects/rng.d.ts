import type { RandomGenerator } from "./types";
/**
 * Creates a deterministic pseudo-random generator that can be reseeded from
 * stable string ids at runtime. The generator is intentionally lightweight so
 * hosts can create or reset it per frame without significant overhead.
 */
export declare const createRandomGenerator: (seed?: string | number) => RandomGenerator;
