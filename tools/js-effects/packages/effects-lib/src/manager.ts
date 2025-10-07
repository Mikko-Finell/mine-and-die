import {
  type DecalSpec,
  type EffectDefinition,
  type EffectFrameContext,
  type EffectInstance,
  EffectLayer,
  type EffectPreset,
} from "./types";

interface ManagedEffect {
  instance: EffectInstance<any>;
  layer: EffectLayer;
  sublayer: number;
  creationIndex: number;
  culled: boolean;
}

interface ViewBounds {
  x: number;
  y: number;
  w: number;
  h: number;
}

interface FrameStats {
  updated: number;
  drawn: number;
  culled: number;
}

const intersects = (a: ViewBounds, b: ViewBounds): boolean =>
  a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;

const asPresetOptions = (
  preset?: EffectPreset | Partial<EffectPreset>
): Record<string, unknown> => {
  if (!preset || typeof preset !== "object") {
    return {};
  }
  const options = (preset as EffectPreset).options;
  if (options && typeof options === "object") {
    return options as Record<string, unknown>;
  }
  return {};
};

export class EffectManager {
  private effects: ManagedEffect[] = [];

  private finished: EffectInstance[] = [];

  private pendingRemovals: Set<EffectInstance<any>> = new Set();

  private iterating = false;

  private creationCounter = 0;

  private viewBounds: ViewBounds | null = null;

  private stats: FrameStats = { updated: 0, drawn: 0, culled: 0 };

  spawn<TOptions>(
    definition: EffectDefinition<TOptions>,
    options: Partial<TOptions> & { x: number; y: number }
  ): EffectInstance<TOptions> {
    const instance = definition.create(options);
    return this.track(instance);
  }

  spawnFromPreset<TOptions>(
    definition: EffectDefinition<TOptions>,
    position: { x: number; y: number },
    preset?: EffectPreset | Partial<EffectPreset>,
    overrides?: Record<string, unknown>
  ): EffectInstance<TOptions> {
    if (definition.createFromPreset) {
      const instance = definition.createFromPreset(position, preset, overrides);
      return this.track(instance);
    }

    const presetOptions = asPresetOptions(preset);
    const instance = definition.create({
      ...presetOptions,
      ...(overrides ?? {}),
      x: position.x,
      y: position.y,
    } as Partial<TOptions> & { x: number; y: number });
    return this.track(instance);
  }

  addInstance<TOptions>(instance: EffectInstance<TOptions>): EffectInstance<TOptions> {
    return this.track(instance);
  }

  clear(): void {
    for (const entry of this.effects) {
      entry.instance.dispose?.();
    }
    for (const finished of this.finished) {
      finished.dispose?.();
    }
    this.effects = [];
    this.finished = [];
    this.creationCounter = 0;
    this.viewBounds = null;
    this.stats = { updated: 0, drawn: 0, culled: 0 };
  }

  cullByAABB(view: ViewBounds): void {
    this.viewBounds = view;
    for (const managed of this.effects) {
      const bounds = managed.instance.getAABB();
      managed.culled = !intersects(bounds, view);
    }
  }

  updateAll(frame: EffectFrameContext): void {
    this.stats.updated = 0;
    this.stats.culled = 0;
    this.iterating = true;

    for (let i = 0; i < this.effects.length; ) {
      const managed = this.effects[i];

      if (this.pendingRemovals.has(managed.instance)) {
        this.pendingRemovals.delete(managed.instance);
        managed.instance.dispose?.();
        this.effects.splice(i, 1);
        continue;
      }

      if (managed.culled) {
        this.stats.culled += 1;
        i += 1;
        continue;
      }

      managed.instance.update(frame);
      this.stats.updated += 1;

      if (!managed.instance.isAlive()) {
        this.finished.push(managed.instance);
        this.pendingRemovals.delete(managed.instance);
        this.effects.splice(i, 1);
        continue;
      }

      managed.layer = managed.instance.layer;
      managed.sublayer = managed.instance.sublayer ?? 0;
      i += 1;
    }

    this.iterating = false;

    if (this.pendingRemovals.size > 0) {
      for (const instance of this.pendingRemovals) {
        if (this.removeActiveInstance(instance)) {
          instance.dispose?.();
        }
      }
      this.pendingRemovals.clear();
    }
  }

  drawAll(frame: EffectFrameContext): void {
    this.stats.drawn = 0;
    const view = this.viewBounds;
    const sorted = [...this.effects].sort((a, b) => {
      if (a.layer !== b.layer) {
        return a.layer - b.layer;
      }
      if (a.sublayer !== b.sublayer) {
        return a.sublayer - b.sublayer;
      }
      return a.creationIndex - b.creationIndex;
    });

    for (const managed of sorted) {
      if (managed.culled) {
        this.stats.culled += 1;
        continue;
      }
      if (view && !intersects(managed.instance.getAABB(), view)) {
        managed.culled = true;
        this.stats.culled += 1;
        continue;
      }
      managed.instance.draw(frame);
      this.stats.drawn += 1;
    }
  }

  collectDecals(): DecalSpec[] {
    if (this.finished.length === 0) {
      return [];
    }
    const decals: DecalSpec[] = [];
    for (const instance of this.finished) {
      const decal = instance.handoffToDecal?.() ?? null;
      if (decal) {
        decals.push(decal);
      }
      instance.dispose?.();
    }
    this.finished = [];
    return decals;
  }

  getLastFrameStats(): FrameStats {
    return { ...this.stats };
  }

  removeInstance<TOptions>(instance: EffectInstance<TOptions> | null | undefined): boolean {
    if (!instance) {
      return false;
    }

    let disposed = false;

    const finishedIndex = this.finished.indexOf(instance);
    if (finishedIndex !== -1) {
      this.finished.splice(finishedIndex, 1);
      instance.dispose?.();
      disposed = true;
    }

    let foundActive = false;

    if (this.iterating) {
      for (const managed of this.effects) {
        if (managed.instance === instance) {
          this.pendingRemovals.add(instance);
          foundActive = true;
          break;
        }
      }
    } else {
      foundActive = this.removeActiveInstance(instance);
      if (foundActive && !disposed) {
        instance.dispose?.();
        disposed = true;
      }
    }

    return disposed || foundActive;
  }

  private track<TOptions>(
    instance: EffectInstance<TOptions>
  ): EffectInstance<TOptions> {
    this.effects.push({
      instance,
      layer: instance.layer,
      sublayer: instance.sublayer ?? 0,
      creationIndex: this.creationCounter++,
      culled: false,
    });
    return instance;
  }

  private removeActiveInstance(instance: EffectInstance<any>): boolean {
    let removed = false;
    for (let index = this.effects.length - 1; index >= 0; index -= 1) {
      if (this.effects[index].instance === instance) {
        this.effects.splice(index, 1);
        removed = true;
      }
    }
    return removed;
  }
}
