import { describe, expect, it, vi } from "vitest";

import { NetworkInputActionDispatcher } from "../input";

describe("NetworkInputActionDispatcher", () => {
  it("attaches protocol metadata to intent payloads", () => {
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => 42,
      sendMessage,
    });

    dispatcher.sendCurrentIntent({ dx: 1, dy: 0, facing: "right" });

    expect(sendMessage).toHaveBeenCalledTimes(1);
    expect(sendMessage).toHaveBeenCalledWith({
      type: "input",
      dx: 1,
      dy: 0,
      facing: "right",
      ver: 1,
      ack: 42,
      seq: 1,
    });
  });

  it("skips dispatch when paused", () => {
    const sendMessage = vi.fn();
    let paused = true;
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => 10,
      sendMessage,
      isDispatchPaused: () => paused,
    });

    dispatcher.sendAction("attack");
    dispatcher.sendCurrentIntent({ dx: 0, dy: 1, facing: "down" });
    dispatcher.cancelPath();
    dispatcher.sendPathCommand({ x: 100, y: 200 });

    expect(sendMessage).not.toHaveBeenCalled();

    paused = false;
    dispatcher.handleDispatchResume();

    expect(sendMessage).toHaveBeenCalledTimes(4);
    expect(sendMessage).toHaveBeenNthCalledWith(1, expect.objectContaining({ type: "action", seq: 1 }));
    expect(sendMessage).toHaveBeenNthCalledWith(2, expect.objectContaining({ type: "input", seq: 2 }));
    expect(sendMessage).toHaveBeenNthCalledWith(3, expect.objectContaining({ type: "cancelPath", seq: 3 }));
    expect(sendMessage).toHaveBeenNthCalledWith(4, expect.objectContaining({ type: "path", seq: 4 }));
  });

  it("omits ack when no tick is available", () => {
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 3,
      getAcknowledgedTick: () => null,
      sendMessage,
    });

    dispatcher.sendCurrentIntent({ dx: 0, dy: -1, facing: "up" });

    expect(sendMessage).toHaveBeenCalledWith({
      type: "input",
      dx: 0,
      dy: -1,
      facing: "up",
      ver: 3,
      seq: 1,
    });
  });

  it("dispatches cancelPath commands with metadata", () => {
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 5,
      getAcknowledgedTick: () => 128,
      sendMessage,
    });

    dispatcher.cancelPath();

    expect(sendMessage).toHaveBeenCalledWith({
      type: "cancelPath",
      ver: 5,
      ack: 128,
      seq: 1,
    });
  });

  it("dispatches path commands with metadata", () => {
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 7,
      getAcknowledgedTick: () => 256,
      sendMessage,
    });

    dispatcher.sendPathCommand({ x: 320, y: 144 });

    expect(sendMessage).toHaveBeenCalledWith({
      type: "path",
      x: 320,
      y: 144,
      ver: 7,
      ack: 256,
      seq: 1,
    });
  });

  it("notifies hooks when intents and path commands are dispatched", () => {
    const sendMessage = vi.fn();
    const onIntent = vi.fn();
    const onPathCommand = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => null,
      getAcknowledgedTick: () => null,
      sendMessage,
      onIntentDispatched: onIntent,
      onPathCommand,
    });

    dispatcher.sendCurrentIntent({ dx: 0.5, dy: 0, facing: "right" });
    dispatcher.cancelPath();
    dispatcher.sendPathCommand({ x: 12, y: 24 });

    expect(sendMessage).toHaveBeenCalledTimes(3);
    expect(sendMessage).toHaveBeenNthCalledWith(1, expect.objectContaining({ type: "input", seq: 1 }));
    expect(sendMessage).toHaveBeenNthCalledWith(2, expect.objectContaining({ type: "cancelPath", seq: 2 }));
    expect(sendMessage).toHaveBeenNthCalledWith(3, expect.objectContaining({ type: "path", seq: 3 }));
    expect(onIntent).toHaveBeenCalledTimes(1);
    expect(onIntent).toHaveBeenCalledWith({ dx: 0.5, dy: 0, facing: "right" });
    expect(onPathCommand).toHaveBeenCalledTimes(2);
    expect(onPathCommand).toHaveBeenNthCalledWith(1, { active: false, target: null });
    expect(onPathCommand).toHaveBeenNthCalledWith(2, { active: true, target: { x: 12, y: 24 } });
  });

  it("notifies path command hooks even when dispatch is paused", () => {
    const sendMessage = vi.fn();
    const onPathCommand = vi.fn();
    let paused = true;
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => null,
      sendMessage,
      isDispatchPaused: () => paused,
      onPathCommand,
    });

    dispatcher.sendPathCommand({ x: 48, y: 96 });
    dispatcher.cancelPath();

    expect(sendMessage).not.toHaveBeenCalled();
    expect(onPathCommand).toHaveBeenCalledTimes(2);
    expect(onPathCommand).toHaveBeenNthCalledWith(1, { active: true, target: { x: 48, y: 96 } });
    expect(onPathCommand).toHaveBeenNthCalledWith(2, { active: false, target: null });

    paused = false;
    dispatcher.handleDispatchResume();
    expect(sendMessage).toHaveBeenCalledTimes(2);
    expect(sendMessage).toHaveBeenNthCalledWith(1, expect.objectContaining({ type: "path", seq: 1 }));
    expect(sendMessage).toHaveBeenNthCalledWith(2, expect.objectContaining({ type: "cancelPath", seq: 2 }));
  });

  it("increments sequence identifiers and clears pending commands on acknowledgement", () => {
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => 5,
      sendMessage,
    });

    dispatcher.sendAction("attack");
    dispatcher.sendPathCommand({ x: 10, y: 20 });

    expect(sendMessage).toHaveBeenNthCalledWith(1, expect.objectContaining({ type: "action", seq: 1 }));
    expect(sendMessage).toHaveBeenNthCalledWith(2, expect.objectContaining({ type: "path", seq: 2 }));

    dispatcher.handleCommandAck({ sequence: 1, tick: 8 });

    dispatcher.sendCurrentIntent({ dx: 0, dy: 1, facing: "down" });

    expect(sendMessage).toHaveBeenNthCalledWith(3, expect.objectContaining({ type: "input", seq: 3 }));
  });

  it("replays pending commands after resync before sending new input", () => {
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 2,
      getAcknowledgedTick: () => 11,
      sendMessage,
    });

    dispatcher.sendAction("attack");
    dispatcher.sendPathCommand({ x: 64, y: 32 });

    expect(sendMessage).toHaveBeenCalledTimes(2);
    sendMessage.mockClear();

    dispatcher.handleResync();
    dispatcher.handleDispatchResume();

    expect(sendMessage).toHaveBeenCalledTimes(2);
    expect(sendMessage).toHaveBeenNthCalledWith(1, expect.objectContaining({ type: "action", seq: 1 }));
    expect(sendMessage).toHaveBeenNthCalledWith(2, expect.objectContaining({ type: "path", seq: 2 }));

    dispatcher.sendCurrentIntent({ dx: 1, dy: 0, facing: "right" });
    expect(sendMessage).toHaveBeenNthCalledWith(3, expect.objectContaining({ type: "input", seq: 3 }));
  });

  it("retries rejected commands when instructed to retry", () => {
    vi.useFakeTimers();
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => 9,
      sendMessage,
    });

    dispatcher.sendAction("attack");
    expect(sendMessage).toHaveBeenCalledTimes(1);
    sendMessage.mockClear();

    dispatcher.handleCommandReject({ sequence: 1, reason: "queue_limit", retry: true });
    expect(sendMessage).not.toHaveBeenCalled();

    vi.advanceTimersByTime(50);

    expect(sendMessage).toHaveBeenCalledTimes(1);
    expect(sendMessage).toHaveBeenCalledWith(expect.objectContaining({ type: "action", seq: 1 }));

    vi.useRealTimers();
  });

  it("notifies rejection listeners with command metadata", () => {
    const sendMessage = vi.fn();
    const onCommandRejected = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => 0,
      sendMessage,
      onCommandRejected,
    });

    dispatcher.sendPathCommand({ x: 5, y: 10 });
    dispatcher.handleCommandReject({ sequence: 1, reason: "queue_limit", retry: false, tick: 7 });

    expect(onCommandRejected).toHaveBeenCalledWith({
      sequence: 1,
      reason: "queue_limit",
      retry: false,
      tick: 7,
      kind: "path",
    });
  });

  it("notifies acknowledgement listeners with command metadata", () => {
    const sendMessage = vi.fn();
    const onCommandAcknowledged = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => 0,
      sendMessage,
      onCommandAcknowledged,
    });

    dispatcher.sendAction("attack");
    dispatcher.handleCommandAck({ sequence: 1, tick: 9 });

    expect(onCommandAcknowledged).toHaveBeenCalledWith({ sequence: 1, tick: 9, kind: "action" });
  });
});
