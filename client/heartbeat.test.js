import { describe, expect, test, vi, beforeEach, afterEach } from "vitest";
import { computeRtt, createHeartbeat } from "./heartbeat.js";

describe("computeRtt", () => {
  const NOW = 10_000;

  const cases = [
    { name: "returns clamped payload rtt", payload: { rtt: 42 }, expected: 42 },
    { name: "clamps negative payload rtt", payload: { rtt: -5 }, expected: 0 },
    {
      name: "ignores non-finite payload rtt",
      payload: { rtt: Number.NaN, clientTime: NOW - 30 },
      expected: 30,
    },
    {
      name: "falls back to clientTime",
      payload: { clientTime: NOW - 75 },
      expected: 75,
    },
    {
      name: "clamps clientTime diff",
      payload: { clientTime: NOW + 10 },
      expected: 0,
    },
    {
      name: "returns null for missing fields",
      payload: {},
      expected: null,
    },
    {
      name: "returns null for null payload",
      payload: null,
      expected: null,
    },
    {
      name: "returns null when now is not finite",
      payload: { clientTime: NOW - 5 },
      now: Number.NaN,
      expected: null,
    },
  ];

  cases.forEach(({ name, payload, expected, now }) => {
    test(name, () => {
      const result = computeRtt(payload, now ?? NOW);
      expect(result).toBe(expected);
    });
  });
});

describe("createHeartbeat", () => {
  let send;

  beforeEach(() => {
    send = vi.fn();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  test("start only keeps a single interval", () => {
    let nextId = 1;
    const timers = new Map();
    const clearSpy = vi.fn((id) => {
      timers.delete(id);
    });

    const controller = createHeartbeat({
      now: () => 0,
      setInterval: (cb, ms) => {
        const id = nextId++;
        timers.set(id, { cb, ms });
        return id;
      },
      clearInterval: clearSpy,
      send,
    });

    controller.start(1000);
    expect(send).toHaveBeenCalledTimes(1);
    expect(timers.size).toBe(1);
    const firstId = Array.from(timers.keys())[0];

    controller.start(1000);
    expect(clearSpy).toHaveBeenCalledWith(firstId);
    expect(timers.size).toBe(1);
    const secondId = Array.from(timers.keys())[0];
    expect(secondId).not.toBe(firstId);
  });

  test("start sends immediately and on interval", () => {
    vi.useFakeTimers();

    const controller = createHeartbeat({
      now: () => 0,
      setInterval: (cb, ms) => setInterval(cb, ms),
      clearInterval: (id) => clearInterval(id),
      send,
    });

    controller.start(200);
    expect(send).toHaveBeenCalledTimes(1);

    vi.advanceTimersByTime(200);
    expect(send).toHaveBeenCalledTimes(2);

    vi.advanceTimersByTime(600);
    expect(send).toHaveBeenCalledTimes(5);
  });

  test("stop cancels future sends", () => {
    vi.useFakeTimers();

    const controller = createHeartbeat({
      now: () => 0,
      setInterval: (cb, ms) => setInterval(cb, ms),
      clearInterval: (id) => clearInterval(id),
      send,
    });

    controller.start(300);
    expect(controller.isRunning()).toBe(true);

    vi.advanceTimersByTime(300);
    expect(send).toHaveBeenCalledTimes(2);

    controller.stop();
    expect(controller.isRunning()).toBe(false);

    vi.advanceTimersByTime(1000);
    expect(send).toHaveBeenCalledTimes(2);
  });
});
