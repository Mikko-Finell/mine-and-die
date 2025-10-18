import { describe, expect, test } from "vitest";

import { translateRenderAnimation } from "../effect-runtime-adapter";
import { effectCatalog as generatedEffectCatalog } from "../generated/effect-contracts";
import type { AnimationFrame } from "../render";

const FIREBALL_TICK = 120;
const TICK_DURATION_MS = 16;

const createFireballAnimation = (): AnimationFrame => {
  const fireballEntry = generatedEffectCatalog.fireball;
  const fireballParameters = fireballEntry.blocks.parameters as Record<string, number> | undefined;
  const radius = fireballParameters?.radius ?? 12;

  return {
    effectId: "effect-fireball",
    startedAt: FIREBALL_TICK * TICK_DURATION_MS,
    durationMs: 30 * TICK_DURATION_MS,
    metadata: {
      state: "active",
      contractId: fireballEntry.contractId,
      entryId: fireballEntry.contractId,
      managedByClient: fireballEntry.managedByClient,
      lastEventKind: "update",
      retained: false,
      catalog: fireballEntry,
      blocks: fireballEntry.blocks,
      instance: {
        id: "effect-fireball",
        entryId: fireballEntry.contractId,
        definitionId: fireballEntry.contractId,
        definition: fireballEntry.definition,
        startTick: FIREBALL_TICK,
        deliveryState: {
          geometry: {
            shape: "circle",
            radius,
            offsetX: 12,
          },
          motion: {
            positionX: 272,
            positionY: 320,
            velocityX: 64,
            velocityY: 0,
          },
        },
        behaviorState: {
          ticksRemaining: 28,
        },
        params: fireballParameters,
        colors: ["#ffaa33", "#ff8800"],
        replication: fireballEntry.definition.client,
        end: fireballEntry.definition.end,
      },
    },
  } satisfies AnimationFrame;
};

describe("effect-runtime-adapter", () => {
  test("translates fireball lifecycle frames into runtime spawn intent", () => {
    const fireballFrame = createFireballAnimation();

    const intent = translateRenderAnimation(fireballFrame);

    expect(intent).not.toBeNull();
    expect(intent!.effectId).toBe("effect-fireball");
    expect(intent!.definition.type).toBe("fireball");
    expect(intent!.state).toBe("active");
    expect(intent!.retained).toBe(false);

    const options = intent!.options as Record<string, unknown>;
    expect(options.x).toBeCloseTo(272 + 12, 3);
    expect(options.y).toBeCloseTo(320, 3);
    expect(options.speed).toBeGreaterThan(0);
    expect(options.range).toBeGreaterThan(0);
    expect(options.radius).toBeCloseTo((fireballFrame.metadata.instance as any).deliveryState.geometry.radius, 3);
  });

  test("marks ended lifecycle frames for removal", () => {
    const fireballFrame = createFireballAnimation();
    fireballFrame.metadata.state = "ended";
    fireballFrame.metadata.lastEventKind = "end";
    fireballFrame.metadata.retained = false;

    const intent = translateRenderAnimation(fireballFrame);

    expect(intent).not.toBeNull();
    expect(intent!.state).toBe("ended");
    expect(intent!.retained).toBe(false);
  });

  test("falls back to placeholder definition for unknown catalog entries", () => {
    const frame: AnimationFrame = {
      effectId: "unknown-effect",
      startedAt: 0,
      durationMs: 0,
      metadata: {
        state: "active",
        contractId: "placeholder",
        lastEventKind: "spawn",
        retained: false,
        blocks: {},
        instance: {
          id: "unknown-effect",
          definitionId: "placeholder",
          startTick: 0,
          deliveryState: {
            geometry: {
              shape: "circle",
              radius: 24,
            },
            motion: {
              positionX: 100,
              positionY: 120,
              velocityX: 0,
              velocityY: 0,
            },
          },
          behaviorState: {
            ticksRemaining: 1,
          },
          replication: {
            sendSpawn: true,
            sendUpdates: false,
            sendEnd: false,
          },
          end: {
            kind: 0,
          },
        },
      },
    };

    const intent = translateRenderAnimation(frame);

    expect(intent).not.toBeNull();
    expect(intent!.definition.type).toBe("placeholder-aura");
    expect(intent!.options.x).toBe(100);
    expect(intent!.options.y).toBe(120);
  });
});

