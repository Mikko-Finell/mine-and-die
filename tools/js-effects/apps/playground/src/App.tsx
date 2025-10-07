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

const ORIGIN_X = CANVAS_WIDTH / 2;
const ORIGIN_Y = CANVAS_HEIGHT / 2;

type OptionValue = number;

type OptionConfig = {
  key: string;
  label: string;
  step?: number;
};

type ResetOptions = {
  preserveDecals?: boolean;
};

const effectControls: Record<string, OptionConfig[]> = {
  "placeholder-aura": [
    { key: "particleCount", label: "Particles", step: 1 },
    { key: "radius", label: "Radius", step: 1 },
    { key: "pulseSpeed", label: "Pulse Speed", step: 0.1 },
  ],
  "melee-swing": [
    { key: "width", label: "Width", step: 1 },
    { key: "height", label: "Height", step: 1 },
    { key: "duration", label: "Duration (s)", step: 0.01 },
    { key: "fadeExponent", label: "Fade Power", step: 0.1 },
  ],
  "impact-burst": [
    { key: "ringRadius", label: "Ring Radius", step: 1 },
    { key: "particleCount", label: "Particles", step: 1 },
    { key: "duration", label: "Duration (s)", step: 0.05 },
    { key: "decalTtl", label: "Decal TTL (s)", step: 0.5 },
  ],
  "blood-splatter": [
    { key: "speed", label: "Speed", step: 0.1 },
    { key: "spawnInterval", label: "Spawn Interval (s)", step: 0.1 },
    { key: "minDroplets", label: "Min Droplets", step: 1 },
    { key: "maxDroplets", label: "Max Droplets", step: 1 },
    { key: "dropletRadius", label: "Droplet Radius", step: 0.1 },
    { key: "minStainRadius", label: "Min Pool Radius", step: 1 },
    { key: "maxStainRadius", label: "Max Pool Radius", step: 1 },
    { key: "drag", label: "Drag", step: 0.01 },
  ],
};

const isCanvasTexture = (value: unknown): value is HTMLCanvasElement =>
  typeof HTMLCanvasElement !== "undefined" && value instanceof HTMLCanvasElement;

const isImageBitmapTexture = (value: unknown): value is ImageBitmap =>
  typeof ImageBitmap !== "undefined" && value instanceof ImageBitmap;

const deriveDecimalPlaces = (step?: number): number => {
  if (typeof step !== "number") {
    return 0;
  }

  const stepAsString = step.toString();
  if (!stepAsString.includes(".")) {
    return 0;
  }

  return stepAsString.split(".")[1]?.length ?? 0;
};

const formatInputValue = (value: number, config: OptionConfig): string => {
  if (!Number.isFinite(value)) {
    return "0";
  }

  const decimals = deriveDecimalPlaces(config.step);
  if (decimals <= 0) {
    return Math.round(value).toString();
  }

  return value.toFixed(decimals);
};

const formatNumber = (value: number): string => {
  if (!Number.isFinite(value)) {
    return "0";
  }

  if (Number.isInteger(value)) {
    return value.toString();
  }

  const fixed = value.toFixed(4);
  return fixed.replace(/0+$/, "").replace(/\.$/, "");
};

const normalizeValueForConfig = (value: number, config: OptionConfig): number => {
  let result = value;
  const decimals = deriveDecimalPlaces(config.step);
  if (decimals > 0) {
    const factor = 10 ** decimals;
    result = Math.round(result * factor) / factor;
  }

  return result;
};

const formatValue = (value: unknown, indent: number): string => {
  if (Array.isArray(value)) {
    if (value.length === 0) {
      return "[]";
    }

    const nextIndent = indent + 2;
    const closingIndent = " ".repeat(indent);
    const items = value.map((entry) => formatValue(entry, nextIndent));
    const fitsInline =
      items.every((entry) => !entry.includes("\n")) &&
      items.join(", ").length <= 60;

    if (fitsInline) {
      return `[${items.join(", ")}]`;
    }

    const padding = " ".repeat(nextIndent);
    return `[\n${items.map((entry) => `${padding}${entry}`).join(",\n")}\n${closingIndent}]`;
  }

  if (value && typeof value === "object") {
    const entries = Object.entries(value as Record<string, unknown>);
    if (entries.length === 0) {
      return "{}";
    }

    const nextIndent = indent + 2;
    const padding = " ".repeat(nextIndent);
    const closingIndent = " ".repeat(indent);

    return `{\n${entries
      .map(([key, entry]) => `${padding}${key}: ${formatValue(entry, nextIndent)}`)
      .join(",\n")}\n${closingIndent}}`;
  }

  if (typeof value === "string") {
    return JSON.stringify(value);
  }

  if (typeof value === "number") {
    return formatNumber(value);
  }

  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }

  return "null";
};

const createExampleCall = (
  effect: AnyEffectCatalogEntry,
  options: Record<string, unknown>,
  origin: { x: number; y: number }
): string => {
  const mergedEntries: [string, unknown][] = [
    ["x", origin.x],
    ["y", origin.y],
    ...Object.entries(options).sort(([a], [b]) => a.localeCompare(b)),
  ];

  const lines = mergedEntries.map(([key, value]) => {
    const formatted = formatValue(value, 2);
    if (formatted.includes("\n")) {
      const [firstLine, ...rest] = formatted.split("\n");
      return [`  ${key}: ${firstLine}`, ...rest].join("\n");
    }
    return `  ${key}: ${formatted}`;
  });

  return `effectManager.spawn(${effect.definitionName}, {\n${lines.join(",\n")}\n});`;
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
  const previousActiveRef = useRef(false);
  const resetEffectRef = useRef<((options?: ResetOptions) => void) | null>(
    null
  );
  const copyTimeoutRef = useRef<number | null>(null);

  const [selectedEffect, setSelectedEffect] = useState<AnyEffectCatalogEntry>(
    () => availableEffects[0]
  );
  const [optionOverrides, setOptionOverrides] = useState<Record<string, OptionValue>>(() => {
    const stored = storedOverridesRef.current[selectedEffect.id];
    return stored ? { ...stored } : {};
  });
  const [inputValues, setInputValues] = useState<Record<string, string>>({});
  const [isLooping, setIsLooping] = useState(false);
  const [copyStatus, setCopyStatus] = useState<"idle" | "copied" | "error">(
    "idle"
  );

  const options = useMemo(
    () =>
      mergeOptions(
        selectedEffect.definition.defaults,
        optionOverrides
      ) as typeof selectedEffect.definition.defaults,
    [selectedEffect, optionOverrides]
  );

  const resolvedOptions = options as unknown as Record<string, OptionValue>;

  const controls = useMemo(
    () => effectControls[selectedEffect.id] ?? [],
    [selectedEffect.id]
  );

  const exampleCall = useMemo(
    () =>
      createExampleCall(
        selectedEffect,
        options as unknown as Record<string, unknown>,
        { x: ORIGIN_X, y: ORIGIN_Y }
      ),
    [selectedEffect, options]
  );

  useEffect(() => {
    const stored = storedOverridesRef.current[selectedEffect.id];
    setOptionOverrides(stored ? { ...stored } : {});
  }, [selectedEffect.id]);

  useEffect(() => {
    const next: Record<string, string> = {};

    for (const control of controls) {
      const numericValue = Number(resolvedOptions[control.key] ?? 0);
      next[control.key] = formatInputValue(numericValue, control);
    }

    setInputValues(next);
  }, [controls, options]);

  useEffect(() => {
    return () => {
      if (copyTimeoutRef.current !== null) {
        window.clearTimeout(copyTimeoutRef.current);
      }
    };
  }, []);

  useEffect(() => {
    setCopyStatus("idle");
  }, [exampleCall]);

  const requestReset = (options?: ResetOptions) => {
    resetEffectRef.current?.(options);
  };

  useEffect(() => {
    const canvas = canvasRef.current;
    const ctx = canvas?.getContext("2d");

    if (!canvas || !ctx) return undefined;

    const manager = effectManagerRef.current ?? new EffectManager();
    effectManagerRef.current = manager;

    const view = { x: 0, y: 0, w: CANVAS_WIDTH, h: CANVAS_HEIGHT };
    const camera = {
      toScreenX: (x: number) => x,
      toScreenY: (y: number) => y,
      zoom: 1,
    };

    let lastTimestamp = performance.now();

    const resetEffect = ({ preserveDecals = false }: ResetOptions = {}) => {
      manager.clear();
      if (!preserveDecals) {
        decalsRef.current = [];
      }
      frameIndexRef.current = 0;
      previousActiveRef.current = false;
      rngRef.current.seedFrom?.(
        `${selectedEffect.id}:reset:${isLooping ? "loop" : "single"}`
      );
      if (!preserveDecals) {
        ctx.clearRect(0, 0, canvas.width, canvas.height);
      }

      const spawnOptions: Partial<typeof selectedEffect.definition.defaults> & {
        x: number;
        y: number;
      } = {
        ...(options as Partial<typeof selectedEffect.definition.defaults>),
        x: ORIGIN_X,
        y: ORIGIN_Y,
      };

      if (selectedEffect.id === "blood-splatter" && !isLooping) {
        spawnOptions.maxBursts = 1;
      }

      manager.spawn(selectedEffect.definition, spawnOptions);
    };

    resetEffect();
    resetEffectRef.current = resetEffect;

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

        const texture = spec.texture;
        const shape = spec.shape;
        const color = spec.averageColor ?? "#ffffff";

        let handled = false;

        if (texture) {
          if (isCanvasTexture(texture)) {
            ctx.drawImage(texture, -texture.width / 2, -texture.height / 2);
            handled = true;
          } else if (isImageBitmapTexture(texture)) {
            ctx.drawImage(texture, -texture.width / 2, -texture.height / 2);
            handled = true;
          }
        }

        if (!handled) {
          ctx.fillStyle = color;

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
        }

        ctx.restore();
        survivors.push(entry);
      }

      decalsRef.current = survivors;
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

      ctx.clearRect(0, 0, canvas.width, canvas.height);

      drawDecals(timestamp);
      manager.drawAll(frame);

      const metrics = manager.getLastFrameStats();
      const hasActive = metrics.updated > 0 || metrics.drawn > 0;

      if (isLooping && !hasActive && previousActiveRef.current) {
        resetEffect({ preserveDecals: true });
        lastTimestamp = timestamp;
      }

      previousActiveRef.current = hasActive;

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
      decalsRef.current = [];
      previousActiveRef.current = false;
      resetEffectRef.current = null;
    };
  }, [selectedEffect, options, isLooping]);

  const commitControlValue = (key: string, value: number) => {
    let didChange = false;

    setOptionOverrides((prev) => {
      const defaults =
        (selectedEffect.definition.defaults as Record<string, OptionValue>) ?? {};
      const defaultValue = defaults[key];
      const updated = { ...prev };

      if (typeof defaultValue === "number" && Math.abs(defaultValue - value) < 1e-6) {
        if (key in updated) {
          delete updated[key];
          didChange = true;
        } else {
          return prev;
        }
      } else if (updated[key] !== value) {
        updated[key] = value;
        didChange = true;
      } else {
        return prev;
      }

      const stored = { ...storedOverridesRef.current };
      if (Object.keys(updated).length === 0) {
        delete stored[selectedEffect.id];
      } else {
        stored[selectedEffect.id] = updated;
      }
      storedOverridesRef.current = stored;
      persistOverrides(stored);

      return updated;
    });

    if (didChange) {
      requestReset();
    }
  };

  const handleLoopChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    setIsLooping(event.target.checked);
    requestReset();
  };

  const handleReset = () => {
    requestReset();
  };

  const scheduleCopyStatusReset = () => {
    if (copyTimeoutRef.current !== null) {
      window.clearTimeout(copyTimeoutRef.current);
    }
    copyTimeoutRef.current = window.setTimeout(() => {
      setCopyStatus("idle");
      copyTimeoutRef.current = null;
    }, 2000);
  };

  const handleCopyCall = async () => {
    const text = exampleCall;

    try {
      if (
        typeof navigator !== "undefined" &&
        navigator.clipboard &&
        typeof navigator.clipboard.writeText === "function"
      ) {
        await navigator.clipboard.writeText(text);
      } else if (typeof document !== "undefined") {
        const textarea = document.createElement("textarea");
        textarea.value = text;
        textarea.setAttribute("readonly", "true");
        textarea.style.position = "absolute";
        textarea.style.left = "-9999px";
        document.body.appendChild(textarea);
        textarea.select();
        document.execCommand("copy");
        document.body.removeChild(textarea);
      } else {
        throw new Error("Clipboard APIs are unavailable");
      }

      setCopyStatus("copied");
      scheduleCopyStatusReset();
    } catch (error) {
      console.warn("Failed to copy example call", error);
      setCopyStatus("error");
      scheduleCopyStatusReset();
    }
  };

  const handleInputChange = (control: OptionConfig) => (
    event: React.ChangeEvent<HTMLInputElement>
  ) => {
    const { value } = event.target;
    setInputValues((prev) => ({
      ...prev,
      [control.key]: value,
    }));

    const trimmed = value.trim();
    if (trimmed === "") {
      return;
    }

    const parsed = Number(trimmed);
    if (!Number.isNaN(parsed) && Number.isFinite(parsed)) {
      commitControlValue(control.key, parsed);
    }
  };

  const handleInputBlur = (control: OptionConfig) => (
    event: React.FocusEvent<HTMLInputElement>
  ) => {
    const parsed = Number(event.target.value);
    if (Number.isNaN(parsed)) {
      const fallback = Number(resolvedOptions[control.key] ?? 0);
      setInputValues((prev) => ({
        ...prev,
        [control.key]: formatInputValue(fallback, control),
      }));
      return;
    }

    const clamped = normalizeValueForConfig(parsed, control);
    setInputValues((prev) => ({
      ...prev,
      [control.key]: formatInputValue(clamped, control),
    }));
    commitControlValue(control.key, clamped);
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
          <div className="app__canvas-stack">
            <div className="app__canvas-frame">
              <canvas ref={canvasRef} width={CANVAS_WIDTH} height={CANVAS_HEIGHT} />
            </div>
            <div className="app__canvas-actions">
              <label className="app__loop-toggle">
                <input
                  type="checkbox"
                  checked={isLooping}
                  onChange={handleLoopChange}
                />
                Loop animation
              </label>
              <button
                type="button"
                className="app__reset-button"
                onClick={handleReset}
              >
                Reset
              </button>
            </div>
            <div className="app__example">
              <div className="app__example-header">
                <h3>Effect call</h3>
                <button
                  type="button"
                  className="app__copy-button"
                  onClick={handleCopyCall}
                >
                  {copyStatus === "copied"
                    ? "Copied!"
                    : copyStatus === "error"
                    ? "Copy failed"
                    : "Copy call"}
                </button>
              </div>
              <pre className="app__example-code">
                <code>{exampleCall}</code>
              </pre>
            </div>
          </div>
        </section>
        <aside className="app__controls-panel">
          <h3 className="app__controls-title">Controls</h3>
          <div className="controls__list">
            {controls.length === 0 ? (
              <p className="controls__empty">No tunable parameters for this effect.</p>
            ) : (
              controls.map((control) => {
                const numericValue = Number(resolvedOptions[control.key] ?? 0);
                const displayValue =
                  inputValues[control.key] ?? formatInputValue(numericValue, control);
                const hint =
                  control.step !== undefined
                    ? `step ${formatNumber(control.step)}`
                    : null;

                return (
                  <label key={control.key} className="controls__item">
                    <span className="controls__label">{control.label}</span>
                    <div className="controls__input-group">
                      <input
                        type="number"
                        inputMode="decimal"
                        step={control.step ?? "any"}
                        value={displayValue}
                        onChange={handleInputChange(control)}
                        onBlur={handleInputBlur(control)}
                      />
                      {hint ? (
                        <span className="controls__hint">{hint}</span>
                      ) : null}
                    </div>
                  </label>
                );
              })
            )}
          </div>
        </aside>
      </main>
    </div>
  );
};

export default App;

