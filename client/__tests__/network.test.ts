import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { WebSocketNetworkClient } from "../network";
import { effectCatalog as generatedEffectCatalog } from "../generated/effect-contracts";

const NETWORK_CONFIGURATION = {
  joinUrl: "/join",
  websocketUrl: "ws://example.test/ws",
  heartbeatIntervalMs: 1000,
  protocolVersion: 1,
} as const;

describe("WebSocketNetworkClient.join", () => {
  let originalFetch: typeof fetch;

  beforeEach(() => {
    originalFetch = globalThis.fetch;
  });

  afterEach(() => {
    globalThis.fetch = originalFetch;
    vi.restoreAllMocks();
  });

  it("hydrates the effect catalog from the top-level join payload", async () => {
    const catalogPayload = JSON.parse(JSON.stringify(generatedEffectCatalog));
    const fetchMock = vi.fn().mockResolvedValue({
      ok: true,
      status: 200,
      json: async () => ({
        ver: NETWORK_CONFIGURATION.protocolVersion,
        id: "player-1",
        config: { seed: "seed", width: 100, height: 100 },
        effectCatalog: catalogPayload,
        players: [],
        npcs: [],
        obstacles: [],
        groundItems: [],
        patches: [],
      }),
    });
    globalThis.fetch = fetchMock as unknown as typeof fetch;

    const client = new WebSocketNetworkClient(NETWORK_CONFIGURATION);
    const join = await client.join();

    expect(fetchMock).toHaveBeenCalledWith(NETWORK_CONFIGURATION.joinUrl, {
      method: "POST",
      cache: "no-store",
    });
    expect(Object.keys(join.effectCatalog)).toEqual(Object.keys(generatedEffectCatalog));
  });
});
