import { afterEach, beforeEach, describe, expect, test } from "vitest";
import { BloodSplatterDefinition } from "../js-effects/effects/bloodSplatter.js";
import { createEffectTestStore } from "./__helpers__/effect-test-store.js";

function createCanvasContextStub() {
  return {
    save() {},
    restore() {},
    translate() {},
    rotate() {},
    scale() {},
    beginPath() {},
    closePath() {},
    moveTo() {},
    lineTo() {},
    fill() {},
    fillStyle: "",
  };
}

function createDocumentStub() {
  return {
    createdCanvases: [],
    createElement(type) {
      if (type !== "canvas") {
        throw new Error(`Unexpected element: ${type}`);
      }
      const context = createCanvasContextStub();
      const canvas = {
        width: 0,
        height: 0,
        getContext(kind) {
          if (kind === "2d") {
            return context;
          }
          return null;
        },
      };
      this.createdCanvases.push(canvas);
      return canvas;
    },
  };
}

function computeStainBounds(stain) {
  if (!stain || typeof stain !== "object") {
    return { minX: 0, minY: 0, maxX: 0, maxY: 0 };
  }

  const rotation = Number.isFinite(stain.rotation) ? stain.rotation : 0;
  const squish = Number.isFinite(stain.squish) ? stain.squish : 1;
  const cos = Math.cos(rotation);
  const sin = Math.sin(rotation);

  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  let seeded = false;

  const includePath = (path) => {
    if (!Array.isArray(path)) {
      return;
    }
    for (let i = 0; i < path.length; i += 2) {
      const px = path[i];
      const py = path[i + 1];
      if (!Number.isFinite(px) || !Number.isFinite(py)) {
        continue;
      }
      const scaledX = px;
      const scaledY = py * squish;
      const rotX = scaledX * cos - scaledY * sin;
      const rotY = scaledX * sin + scaledY * cos;
      const worldX = Number(stain.x) + rotX;
      const worldY = Number(stain.y) + rotY;

      if (!seeded) {
        minX = maxX = worldX;
        minY = maxY = worldY;
        seeded = true;
        continue;
      }

      if (worldX < minX) minX = worldX;
      if (worldX > maxX) maxX = worldX;
      if (worldY < minY) minY = worldY;
      if (worldY > maxY) maxY = worldY;
    }
  };

  includePath(stain.basePath);
  includePath(stain.midPath);

  if (!seeded) {
    const fallbackX = Number(stain.x) || 0;
    const fallbackY = Number(stain.y) || 0;
    return { minX: fallbackX, minY: fallbackY, maxX: fallbackX, maxY: fallbackY };
  }

  return { minX, minY, maxX, maxY };
}

function unionStainBounds(stains) {
  let minX = Infinity;
  let minY = Infinity;
  let maxX = -Infinity;
  let maxY = -Infinity;
  let seeded = false;

  for (const stain of stains) {
    const bounds = computeStainBounds(stain);
    if (!Number.isFinite(bounds.minX) || !Number.isFinite(bounds.maxX) || !Number.isFinite(bounds.minY) || !Number.isFinite(bounds.maxY)) {
      continue;
    }
    if (!seeded) {
      minX = bounds.minX;
      minY = bounds.minY;
      maxX = bounds.maxX;
      maxY = bounds.maxY;
      seeded = true;
      continue;
    }

    if (bounds.minX < minX) minX = bounds.minX;
    if (bounds.maxX > maxX) maxX = bounds.maxX;
    if (bounds.minY < minY) minY = bounds.minY;
    if (bounds.maxY > maxY) maxY = bounds.maxY;
  }

  if (!seeded) {
    return { minX: 0, minY: 0, maxX: 0, maxY: 0 };
  }

  return { minX, minY, maxX, maxY };
}

describe("blood-splatter decal regression", () => {
  let originalDocument;
  let documentStub;

  beforeEach(() => {
    originalDocument = globalThis.document;
    documentStub = createDocumentStub();
    globalThis.document = documentStub;
  });

  afterEach(() => {
    if (typeof originalDocument === "undefined") {
      delete globalThis.document;
    } else {
      globalThis.document = originalDocument;
    }
    originalDocument = undefined;
    documentStub = undefined;
  });

  test("handoff should respect stain radius and persist decals", () => {
    const store = createEffectTestStore();
    const manager = store.effectManager;

    const options = {
      x: 240,
      y: 160,
      colors: ["#7a0e12", "#4a090b"],
      drag: 0.92,
      dropletRadius: 3,
      maxBursts: 0,
      maxDroplets: 33,
      maxStainRadius: 6,
      maxStains: 140,
      minDroplets: 4,
      minStainRadius: 4,
      spawnInterval: 1.1,
      speed: 3,
    };

    const instance = manager.spawn(BloodSplatterDefinition, options);

    let iterations = 0;
    const dt = 1 / 30;
    const nowBase = performance.now() / 1000;
    while (instance.isAlive() && iterations < 900) {
      manager.updateAll({ dt, now: nowBase + iterations * dt });
      iterations += 1;
    }

    expect(instance.isAlive()).toBe(false);

    const stainsSnapshot = Array.isArray(instance.stains) ? instance.stains.slice() : [];
    const tightBounds = unionStainBounds(stainsSnapshot);

    const decalSpec = instance.handoffToDecal();
    expect(decalSpec).not.toBeNull();
    expect(decalSpec && decalSpec.texture).toBeDefined();

    const texture = decalSpec.texture;
    const expectedWidth = Math.max(1, Math.ceil(tightBounds.maxX - tightBounds.minX)) + 8;
    const expectedHeight = Math.max(1, Math.ceil(tightBounds.maxY - tightBounds.minY)) + 8;
    expect(texture.width).toBe(expectedWidth);
    expect(texture.height).toBe(expectedHeight);

    const expectedCenterX = (tightBounds.minX + tightBounds.maxX) / 2;
    const expectedCenterY = (tightBounds.minY + tightBounds.maxY) / 2;
    expect(decalSpec.x).toBeCloseTo(expectedCenterX, 5);
    expect(decalSpec.y).toBeCloseTo(expectedCenterY, 5);

    const decals = manager.collectDecals(performance.now() / 1000);
    expect(decals.length).toBe(1);

    const [decal] = decals;
    const bounds = decal.getAABB();
    expect(bounds.w).toBe(expectedWidth);
    expect(bounds.h).toBe(expectedHeight);

    const tracked = manager.getTrackedInstances("ground-decal");
    expect(tracked.has(decal.id)).toBe(true);

    manager.updateAll({ dt, now: nowBase + iterations * dt + dt });
    expect(tracked.has(decal.id)).toBe(true);
  });
});
