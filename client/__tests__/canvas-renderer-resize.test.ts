import { afterEach, beforeEach, describe, expect, test, vi } from "vitest";

import { CanvasRenderer, type RenderBatch, type RenderLayer, type StaticGeometry } from "../render";

type WindowStub = {
  requestAnimationFrame?: (callback: FrameRequestCallback) => number;
  cancelAnimationFrame?: (handle: number) => void;
  [key: string]: unknown;
};

const windowContainer = globalThis as { window?: WindowStub };
const originalWindow = windowContainer.window;

let requestAnimationFrameMock: ReturnType<typeof vi.fn<[FrameRequestCallback], number>>;
let cancelAnimationFrameMock: ReturnType<typeof vi.fn<[number], void>>;
let rafCallbacks: FrameRequestCallback[];

const worldLayer: RenderLayer = { id: "world-background", zIndex: -200 };
const effectVisualLayer: RenderLayer = { id: "effect-visual", zIndex: 10 };
const effectAreaLayer: RenderLayer = { id: "effect-area", zIndex: 20 };
const effectTargetLayer: RenderLayer = { id: "effect-target", zIndex: 30 };

const createWorldBackground = (width: number, height: number): StaticGeometry => ({
  id: "world/background",
  layer: worldLayer,
  vertices: [
    [0, 0],
    [width, 0],
    [width, height],
    [0, height],
  ],
  style: {
    kind: "world-background",
    width,
    height,
  },
});

const createBatch = (width: number, height: number): RenderBatch => ({
  keyframeId: `world-${width}x${height}`,
  time: Date.now(),
  staticGeometry: [createWorldBackground(width, height)],
  animations: [],
  pathTarget: null,
});

const createContext = (canvas: HTMLCanvasElement): CanvasRenderingContext2D => ({
  canvas,
  save: vi.fn(),
  restore: vi.fn(),
  clearRect: vi.fn(),
  beginPath: vi.fn(),
  moveTo: vi.fn(),
  lineTo: vi.fn(),
  stroke: vi.fn(),
  fill: vi.fn(),
  closePath: vi.fn(),
  arc: vi.fn(),
  fillRect: vi.fn(),
  strokeStyle: "",
  fillStyle: "",
  lineWidth: 1,
}) as unknown as CanvasRenderingContext2D;

beforeEach(() => {
  rafCallbacks = [];
  requestAnimationFrameMock = vi.fn<[FrameRequestCallback], number>((callback: FrameRequestCallback) => {
    rafCallbacks.push(callback);
    return rafCallbacks.length;
  });
  cancelAnimationFrameMock = vi.fn<[number], void>();
  windowContainer.window = {
    ...(originalWindow ?? {}),
    requestAnimationFrame: requestAnimationFrameMock,
    cancelAnimationFrame: cancelAnimationFrameMock,
  };
});

afterEach(() => {
  if (originalWindow) {
    windowContainer.window = originalWindow;
  } else {
    delete windowContainer.window;
  }
  vi.restoreAllMocks();
});

describe("CanvasRenderer", () => {
  test("resizes canvas and preserves effect cadence when world dimensions change", () => {
    const renderer = new CanvasRenderer({
      dimensions: { width: 100, height: 100 },
      layers: [worldLayer, effectVisualLayer, effectAreaLayer, effectTargetLayer],
    });

    const updateAll = vi.fn();
    const drawAll = vi.fn();
    const effectManager = {
      spawn: vi.fn(),
      removeInstance: vi.fn(),
      clear: vi.fn(),
      cullByAABB: vi.fn(),
      updateAll,
      drawAll,
    };
    (renderer as unknown as { effectManager: typeof effectManager }).effectManager = effectManager;

    const canvas = {
      width: 0,
      height: 0,
    } as HTMLCanvasElement;
    const context = createContext(canvas);
    (canvas as unknown as { getContext: (type: string) => CanvasRenderingContext2D | null }).getContext = vi.fn(
      () => context,
    );

    renderer.renderBatch(createBatch(100, 100));
    renderer.mount({ canvas, context });

    expect(canvas.width).toBe(100);
    expect(canvas.height).toBe(100);

    const stepFrame = (timestamp: number): void => {
      const callback = rafCallbacks.shift();
      expect(callback).toBeDefined();
      callback?.(timestamp);
    };

    // Run first frame to establish baseline timing.
    stepFrame(0);

    renderer.renderBatch(createBatch(320, 256));
    stepFrame(32);

    expect(canvas.width).toBe(320);
    expect(canvas.height).toBe(256);

    expect(updateAll).toHaveBeenCalledTimes(2);
    const firstDt = updateAll.mock.calls[0]?.[0]?.dt;
    const secondDt = updateAll.mock.calls[1]?.[0]?.dt;
    expect(firstDt).toBe(0);
    expect(secondDt).toBeGreaterThan(0);
    expect(secondDt).toBeLessThanOrEqual(0.25);

    expect(drawAll).toHaveBeenCalledTimes(2);

    renderer.unmount();
    expect(cancelAnimationFrameMock).toHaveBeenCalled();
  });
});
