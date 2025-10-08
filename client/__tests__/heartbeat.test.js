import { afterEach, describe, expect, it, vi } from "vitest";
import { computeRtt, createHeartbeat } from "../heartbeat.js";

describe("computeRtt", () => {
  it.each([
    {
      label: "uses payload RTT when finite",
      payload: { rtt: 42 },
      now: 1000,
      expected: 42,
    },
    {
      label: "clamps negative RTT",
      payload: { rtt: -5 },
      now: 1000,
      expected: 0,
    },
    {
      label: "derives from client time when RTT missing",
      payload: { clientTime: 100 },
      now: 250,
      expected: 150,
    },
    {
      label: "clamps derived RTT to zero",
      payload: { clientTime: 400 },
      now: 350,
      expected: 0,
    },
    {
      label: "ignores non-finite numbers",
      payload: { rtt: Number.POSITIVE_INFINITY, clientTime: Number.NaN },
      now: 300,
      expected: null,
    },
    {
      label: "returns null for missing fields",
      payload: {},
      now: 100,
      expected: null,
    },
  ])("$label", ({ payload, now, expected }) => {
    expect(computeRtt(payload, now)).toBe(expected);
  });
});

describe("createHeartbeat", () => {
  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.clearAllMocks();
  });

  it("clears previous intervals before starting again", () => {
    let nextId = 1;
    const timers = new Map();
    const setIntervalMock = vi.fn((fn) => {
      const id = nextId++;
      timers.set(id, fn);
      return id;
    });
    const clearIntervalMock = vi.fn((id) => {
      timers.delete(id);
    });
    const sendMock = vi.fn();
    const nowMock = vi.fn(() => 0);

    const controller = createHeartbeat({
      now: nowMock,
      setInterval: setIntervalMock,
      clearInterval: clearIntervalMock,
      send: sendMock,
    });

    controller.start(100);
    const firstId = setIntervalMock.mock.results[0]?.value;
    expect(setIntervalMock).toHaveBeenCalledTimes(1);
    expect(controller.isRunning()).toBe(true);

    controller.start(100);
    expect(clearIntervalMock).toHaveBeenCalledWith(firstId);
    expect(setIntervalMock).toHaveBeenCalledTimes(2);
    expect(controller.isRunning()).toBe(true);
  });

  it("sends immediately and on the configured interval", () => {
    vi.useFakeTimers();
    vi.setSystemTime(0);

    const sendMock = vi.fn();
    const controller = createHeartbeat({
      now: () => Date.now(),
      setInterval: (fn, ms) => setInterval(fn, ms),
      clearInterval: (id) => clearInterval(id),
      send: sendMock,
    });

    controller.start(1000);
    expect(sendMock).toHaveBeenCalledTimes(1);
    expect(sendMock).toHaveBeenLastCalledWith(0);

    vi.advanceTimersByTime(1000);
    expect(sendMock).toHaveBeenCalledTimes(2);
    expect(sendMock).toHaveBeenLastCalledWith(1000);

    vi.advanceTimersByTime(2000);
    expect(sendMock).toHaveBeenCalledTimes(4);
    expect(sendMock).toHaveBeenNthCalledWith(3, 2000);
    expect(sendMock).toHaveBeenNthCalledWith(4, 3000);
  });

  it("stops scheduling when stop is called", () => {
    vi.useFakeTimers();
    vi.setSystemTime(0);

    const sendMock = vi.fn();
    const clearIntervalSpy = vi.spyOn(globalThis, "clearInterval");
    const controller = createHeartbeat({
      now: () => Date.now(),
      setInterval: (fn, ms) => setInterval(fn, ms),
      clearInterval: (id) => clearInterval(id),
      send: sendMock,
    });

    controller.start(1000);
    vi.advanceTimersByTime(1000);
    expect(sendMock).toHaveBeenCalledTimes(2);

    controller.stop();
    expect(controller.isRunning()).toBe(false);

    vi.advanceTimersByTime(5000);
    expect(sendMock).toHaveBeenCalledTimes(2);
    expect(clearIntervalSpy).toHaveBeenCalled();
    clearIntervalSpy.mockRestore();
  });
});
