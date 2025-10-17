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
    });
  });

  it("skips dispatch when paused", () => {
    const sendMessage = vi.fn();
    const dispatcher = new NetworkInputActionDispatcher({
      getProtocolVersion: () => 1,
      getAcknowledgedTick: () => 10,
      sendMessage,
      isDispatchPaused: () => true,
    });

    dispatcher.sendAction("attack");
    dispatcher.sendCurrentIntent({ dx: 0, dy: 1, facing: "down" });
    dispatcher.cancelPath();

    expect(sendMessage).not.toHaveBeenCalled();
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

    expect(onIntent).toHaveBeenCalledTimes(1);
    expect(onIntent).toHaveBeenCalledWith({ dx: 0.5, dy: 0, facing: "right" });
    expect(onPathCommand).toHaveBeenCalledTimes(1);
    expect(onPathCommand).toHaveBeenCalledWith(false);
  });
});
