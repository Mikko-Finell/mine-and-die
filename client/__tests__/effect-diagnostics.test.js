import { describe, expect, test } from "vitest";
import {
  createEffectDiagnosticsState,
  recordUnknownEffectUpdate,
  resetEffectDiagnosticsState,
} from "../effect-diagnostics.js";

describe("effect-diagnostics", () => {
  test("createEffectDiagnosticsState returns empty counters", () => {
    const state = createEffectDiagnosticsState();
    expect(state).toMatchObject({
      unknownUpdateCount: 0,
      lastUnknownUpdateAt: null,
      lastUnknownUpdate: null,
    });
  });

  test("recordUnknownEffectUpdate increments count and normalizes values", () => {
    const state = createEffectDiagnosticsState();
    const now = Date.now();
    recordUnknownEffectUpdate(state, { id: " effect-7 ", seq: "4", tick: 12.9 }, { timestamp: now });

    expect(state.unknownUpdateCount).toBe(1);
    expect(state.lastUnknownUpdateAt).toBe(now);
    expect(state.lastUnknownUpdate).toEqual({ id: "effect-7", seq: 4, tick: 12 });

    recordUnknownEffectUpdate(state, { id: null, seq: -2, tick: "NaN" }, { timestamp: now + 5 });
    expect(state.unknownUpdateCount).toBe(2);
    expect(state.lastUnknownUpdateAt).toBe(now + 5);
    expect(state.lastUnknownUpdate).toEqual({ id: null, seq: null, tick: null });
  });

  test("resetEffectDiagnosticsState clears the existing object", () => {
    const state = createEffectDiagnosticsState();
    recordUnknownEffectUpdate(state, { id: "ghost", seq: 3 });
    expect(state.unknownUpdateCount).toBe(1);

    const reset = resetEffectDiagnosticsState(state);
    expect(reset).toBe(state);
    expect(state.unknownUpdateCount).toBe(0);
    expect(state.lastUnknownUpdateAt).toBeNull();
    expect(state.lastUnknownUpdate).toBeNull();
  });

  test("recordUnknownEffectUpdate tolerates missing initial state", () => {
    const result = recordUnknownEffectUpdate(null, { id: "ghost" }, { timestamp: 123 });
    expect(result.unknownUpdateCount).toBe(1);
    expect(result.lastUnknownUpdateAt).toBe(123);
    expect(result.lastUnknownUpdate).toEqual({ id: "ghost", seq: null, tick: null });
  });
});
