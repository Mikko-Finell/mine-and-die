import { beforeEach, afterEach, describe, expect, it, vi } from "vitest";

import {
  getEffectCatalog,
  setEffectCatalog,
} from "../effect-catalog";
import { effectCatalogHash } from "../generated/effect-contracts-hash";
import { WebSocketNetworkClient } from "../network";

const createJoinPayload = (overrides: Record<string, unknown> = {}) => ({
  ver: 1,
  id: "player-1",
  config: { seed: "seed", width: 100, height: 100 },
  effectCatalogHash,
  players: [],
  npcs: [],
  obstacles: [],
  groundItems: [],
  patches: [],
  ...overrides,
});

describe("WebSocketNetworkClient join", () => {
  beforeEach(() => {
    setEffectCatalog(null);
  });

  afterEach(() => {
    vi.restoreAllMocks();
    setEffectCatalog(null);
  });

  it("returns the generated effect catalog when hashes match", async () => {
    const payload = createJoinPayload();
    const fetchMock = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValue({
        ok: true,
        status: 200,
        json: async () => payload,
      } as Response);

    const client = new WebSocketNetworkClient({
      joinUrl: "/join",
      websocketUrl: "ws://localhost",
      heartbeatIntervalMs: 5000,
      protocolVersion: 1,
    });

    const result = await client.join();

    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(result.effectCatalogHash).toBe(effectCatalogHash);
    expect(result.effectCatalog).toBe(getEffectCatalog());
  });

  it("throws a compatibility error with rebuild instructions when hashes mismatch", async () => {
    const payload = createJoinPayload({ effectCatalogHash: "mismatch" });
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => payload,
    } as Response);

    const client = new WebSocketNetworkClient({
      joinUrl: "/join",
      websocketUrl: "ws://localhost",
      heartbeatIntervalMs: 5000,
      protocolVersion: 1,
    });

    const error = await client.join().catch((cause) => cause as Error);

    expect(error).toBeInstanceOf(Error);
    expect(error.message).toMatch(
      /Effect catalog hash mismatch between client and server/,
    );
    expect(error.message).toMatch(
      /Rebuild and redeploy both client and server from the same commit/,
    );
  });
});
