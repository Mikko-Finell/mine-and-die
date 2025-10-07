import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  EffectManager,
  createRandomGenerator,
  type DecalSpec,
  type EffectFrameContext,
} from "@js-effects/effects-lib";
import { availableEffects, type AnyEffectCatalogEntry } from "./effects";

const CANVAS_WIDTH = 480;
const CANVAS_HEIGHT = 320;
const STORAGE_KEY = "js-effects-playground:overrides";

type OptionValue = number;

type OptionConfig = {
  key: string;
  label: string;
  min?: number;
  max?: number;
  step?: number;
};

const effectControls: Record<string, OptionConfig[]> = {
  "placeholder-aura": [
    { key: "particleCount", label: "Particles", min: 1, max: 64, step: 1 },
    { key: "radius", label: "Radius", min: 10, max: 160, step: 1 },
    { key: "pulseSpeed", label: "Pulse Speed", min: 0, max: 8, step: 0.1 },
  ],
  "impact-burst": [
    { key: "ringRadius", label: "Ring Radius", min: 20, max: 140, step: 1 },
    { key: "particleCount", label: "Particles", min: 3, max: 32, step: 1 },
    { key: "duration", label: "Duration (s)", min: 0.2, max: 2, step: 0.05 },
    { key: "decalTtl", label: "Decal TTL (s)", min: 1, max: 12, step: 0.5 },
  ],
  "blood-splatter": [
    { key: "speed", label: "Speed", min: 0.2, max: 3, step: 0.1 },
    { key: "spawnInterval", label: "Spawn Interval (s)", min: 0.2, max: 4, step: 0.1 },
    { key: "minDroplets", label: "Min Droplets", min: 1, max: 60, step: 1 },
    { key: "maxDroplets", label: "Max Droplets", min: 1, max: 80, step: 1 },
    { key: "dropletRadius", label: "Droplet Radius", min: 1, max: 8, step: 0.1 },
    { key: "minStainRadius", label: "Min Pool Radius", min: 4, max: 40, step: 1 },
    { key: "maxStainRadius", label: "Max Pool Radius", min: 6, max: 60, step: 1 },
    { key: "drag", label: "Drag", min: 0.7, max: 0.99, step: 0.01 },
  ],
};

const mergeOptions = <T extends Record<string, any>>(
  base: T,
  updates: Record<string, OptionValue>
): T => ({
  ...base,
  ...updates,
});

const loadStoredOverrides = (): Record<string, Record<string, OptionValue>> => {
  if (typeof window === "undefined") return {};

  const raw = window.localStorage.getItem(STORAGE_KEY);
  if (!raw) return {};

  try {
    const parsed = JSON.parse(raw);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, Record<string, OptionValue>>;
    }
  } catch (error) {
    console.warn("Failed to parse stored overrides", error);
  }

  return {};
};

const persistOverrides = (data: Record<string, Record<string, OptionValue>>) => {
  if (typeof window === "undefined") return;

  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(data));
};

const App: React.FC = () => {
  const canvasRef = useRef<HTMLCanvasElement | null>(null);
  const animationRef = useRef<number>();
  const effectManagerRef = useRef<EffectManager>();
  const storedOverridesRef = useRef<Record<string, Record<string, OptionValue>>>(
    loadStoredOverrides()
  );
  const rngRef = useRef(createRandomGenerator());
  const frameIndexRef = useRef(0);
  const decalsRef = useRef<{ spec: DecalSpec; spawnedAt: number }[]>([]);
  const statsRef = useRef({ updated: 0, drawn: 0, culled: 0 });
  const decalCountRef = useRef(0);

  const [stats, setStats] = useState(statsRef.current);
  const [decalCount, setDecalCount] = useState(0);

  const [selectedEffect, setSelectedEffect] = useState<AnyEffectCatalogEntry>(
    () => availableEffects[0]
  );
  const [optionOverrides, setOptionOverrides] = useState<Record<string, OptionValue>>(() => {
    const stored = storedOverridesRef.current[selectedEffect.id];
    return stored ? { ...stored } : {};
  });

  const options = useMemo(
    () =>
      mergeOptions(
        selectedEffect.definition.defaults,
        optionOverrides
      ) as typeof selectedEffect.definition.defaults,
    [selectedEffect, optionOverrides]
  );

  useEffect(() => {
    const stored = storedOverridesRef.current[selectedEffect.id];
    setOptionOverrides(stored ? { ...stored } : {});
  }, [selectedEffect.id]);

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");

    if (!canvas || !ctx) return undefined;

    const manager = effectManagerRef.current ?? new EffectManager();
    manager.clear();
    effectManagerRef.current = manager;

    decalsRef.current = [];
    decalCountRef.current = 0;
    frameIndexRef.current = 0;
    rngRef.current.seedFrom?.(`${selectedEffect.id}:reset`);
    setDecalCount(0);

    const origin = { x: CANVAS_WIDTH / 2, y: CANVAS_HEIGHT / 2 };
    manager.spawn(selectedEffect.definition, {
      ...(options as Record<string, unknown>),
      x: origin.x,
      y: origin.y,
    });

    const view = { x: 0, y: 0, w: CANVAS_WIDTH, h: CANVAS_HEIGHT };
    const camera = {
      toScreenX: (x: number) => x,
      toScreenY: (y: number) => y,
      zoom: 1,
    };

    let lastTimestamp = performance.now();

    const drawDecals = (timestamp: number) => {
      const survivors: { spec: DecalSpec; spawnedAt: number }[] = [];
      const nowSeconds = timestamp / 1000;

      for (const entry of decalsRef.current) {
        const { spec, spawnedAt } = entry;
        const ttl = spec.ttl ?? Infinity;
        const age = nowSeconds - spawnedAt;

        if (Number.isFinite(ttl) && ttl > 0 && age > ttl) {
          continue;
        }

        const alpha =
          Number.isFinite(ttl) && ttl > 0 ? Math.max(0, 1 - age / ttl) : 1;

        ctx.save();
        ctx.globalAlpha = alpha;
        const screenX = camera.toScreenX(spec.x);
        const screenY = camera.toScreenY(spec.y);
        ctx.translate(screenX, screenY);
        ctx.scale(camera.zoom, camera.zoom);
        ctx.rotate(spec.rotation ?? 0);

        const color = spec.averageColor ?? "#ffffff";
        ctx.fillStyle = color;

        const shape = spec.shape;

        if (shape?.type === "rect") {
          const { w, h } = shape;
          ctx.fillRect(-w / 2, -h / 2, w, h);
        } else if (shape?.type === "poly" && shape.points.length >= 4) {
          const pts = shape.points;
          ctx.beginPath();
          ctx.moveTo(pts[0], pts[1]);
          for (let i = 2; i < pts.length; i += 2) {
            ctx.lineTo(pts[i], pts[i + 1]);
          }
          ctx.closePath();
          ctx.fill();
        } else {
          const rx = shape?.type === "oval" ? shape.rx : 32;
          const ry = shape?.type === "oval" ? shape.ry : rx * 0.6;
          ctx.beginPath();
          ctx.ellipse(0, 0, rx, ry, 0, 0, Math.PI * 2);
          ctx.fill();
        }

        ctx.restore();
        survivors.push(entry);
      }

      decalsRef.current = survivors;
      if (decalCountRef.current !== survivors.length) {
        decalCountRef.current = survivors.length;
        setDecalCount(survivors.length);
      }
    };

    const step = (timestamp: number) => {
      const dt = Math.min(0.05, Math.max(0, (timestamp - lastTimestamp) / 1000));
      lastTimestamp = timestamp;

      const rng = rngRef.current;
      rng.seedFrom?.(`${selectedEffect.id}:${frameIndexRef.current}`);
      frameIndexRef.current += 1;

      const frame: EffectFrameContext = {
        ctx,
        dt,
        now: timestamp,
        camera,
        rng,
      };

      manager.cullByAABB(view);
      manager.updateAll(frame);

      const newDecals = manager.collectDecals();
      if (newDecals.length > 0) {
        for (const decal of newDecals) {
          decalsRef.current.push({ spec: decal, spawnedAt: timestamp / 1000 });
        }
      }

      const metrics = manager.getLastFrameStats();
      if (
        metrics.updated !== statsRef.current.updated ||
        metrics.drawn !== statsRef.current.drawn ||
        metrics.culled !== statsRef.current.culled
      ) {
        statsRef.current = metrics;
        setStats(metrics);
      }

      ctx.clearRect(0, 0, canvas.width, canvas.height);

      drawDecals(timestamp);
      manager.drawAll(frame);

      animationRef.current = requestAnimationFrame(step);
    };

    animationRef.current = requestAnimationFrame((timestamp) => {
      lastTimestamp = timestamp;
      step(timestamp);
    });

    return () => {
      if (animationRef.current) {
        cancelAnimationFrame(animationRef.current);
      }
      manager.clear();
      ctx.clearRect(0, 0, canvas.width, canvas.height);
    };
  }, [selectedEffect, options]);

  const controls = effectControls[selectedEffect.id] ?? [];

  const handleControlChange = (key: string, value: number) => {
    setOptionOverrides((prev) => {
      const next = {
        ...prev,
        [key]: value,
      };

      storedOverridesRef.current = {
        ...storedOverridesRef.current,
        [selectedEffect.id]: next,
      };
      persistOverrides(storedOverridesRef.current);

      return next;
    });
  };

  return (
    <div className="app">
      <header className="app__header">
        <h1>JS Effects Playground</h1>
      </header>
      <main className="app__main">
        <aside className="app__sidebar">
          <h2 className="app__sidebar-title">Effects</h2>
          <ul className="effect-list">
            {availableEffects.map((effect) => (
              <li key={effect.id}>
                <button
                  type="button"
                  className={`effect-list__button${
                    effect.id === selectedEffect.id ? " effect-list__button--active" : ""
                  }`}
                  onClick={() => setSelectedEffect(effect)}
                >
                  <span className="effect-list__name">{effect.name}</span>
                  <span className="effect-list__description">{effect.description}</span>
                </button>
              </li>
            ))}
          </ul>
        </aside>
        <section className="app__canvas-area">
          <div className="app__canvas-frame">
            <canvas ref={canvasRef} width={CANVAS_WIDTH} height={CANVAS_HEIGHT} />
          </div>
          <div className="app__metrics">
            <p>
              Frame stats – updated: {stats.updated}, drawn: {stats.drawn}, culled: {" "}
              {stats.culled}
            </p>
            <p>Active decals: {decalCount}</p>
            <p className="app__metrics-note">
              Deterministic mode seeds the RNG with the effect id and frame index.
            </p>
          </div>
        </section>
        <aside className="app__controls-panel">
          <h3 className="app__controls-title">Controls</h3>
          <div className="controls__list">
            {controls.map((control) => {
              const value = Number(
                (options as Record<string, OptionValue>)[control.key] ?? 0
              );

              return (
                <label key={control.key} className="controls__item">
                  <span className="controls__label">{control.label}</span>
                  <div className="controls__slider">
                    <input
                      type="range"
                      min={control.min ?? 0}
                      max={control.max ?? 10}
                      step={control.step ?? 1}
                      value={value}
                      onChange={(event) =>
                        handleControlChange(control.key, Number(event.target.value))
                      }
                    />
                    <output>
                      {value.toFixed((control.step ?? 1) >= 1 ? 0 : 2)}
                    </output>
                  </div>
                </label>
              );
            })}
          </div>
        </aside>
      </main>
    </div>
  );
};

export default App;

