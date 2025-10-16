const intersects = (a, b) => a.x < b.x + b.w && a.x + a.w > b.x && a.y < b.y + b.h && a.y + a.h > b.y;
const asPresetOptions = (preset) => {
    if (!preset || typeof preset !== "object") {
        return {};
    }
    const options = preset.options;
    if (options && typeof options === "object") {
        return options;
    }
    return {};
};
export class EffectManager {
    constructor() {
        this.effects = [];
        this.finished = [];
        this.pendingRemovals = new Set();
        this.iterating = false;
        this.creationCounter = 0;
        this.viewBounds = null;
        this.stats = { updated: 0, drawn: 0, culled: 0 };
    }
    spawn(definition, options) {
        const instance = definition.create(options);
        return this.track(instance);
    }
    spawnFromPreset(definition, position, preset, overrides) {
        if (definition.createFromPreset) {
            const instance = definition.createFromPreset(position, preset, overrides);
            return this.track(instance);
        }
        const presetOptions = asPresetOptions(preset);
        const instance = definition.create({
            ...presetOptions,
            ...(overrides !== null && overrides !== void 0 ? overrides : {}),
            x: position.x,
            y: position.y,
        });
        return this.track(instance);
    }
    addInstance(instance) {
        return this.track(instance);
    }
    clear() {
        var _a, _b, _c;
        for (const entry of this.effects) {
            (_b = (_a = entry.instance).dispose) === null || _b === void 0 ? void 0 : _b.call(_a);
        }
        for (const finished of this.finished) {
            (_c = finished.dispose) === null || _c === void 0 ? void 0 : _c.call(finished);
        }
        this.effects = [];
        this.finished = [];
        this.creationCounter = 0;
        this.viewBounds = null;
        this.stats = { updated: 0, drawn: 0, culled: 0 };
    }
    cullByAABB(view) {
        this.viewBounds = view;
        for (const managed of this.effects) {
            const bounds = managed.instance.getAABB();
            managed.culled = !intersects(bounds, view);
        }
    }
    updateAll(frame) {
        var _a, _b, _c, _d;
        this.stats.updated = 0;
        this.stats.culled = 0;
        this.iterating = true;
        for (let i = 0; i < this.effects.length;) {
            const managed = this.effects[i];
            if (this.pendingRemovals.has(managed.instance)) {
                this.pendingRemovals.delete(managed.instance);
                (_b = (_a = managed.instance).dispose) === null || _b === void 0 ? void 0 : _b.call(_a);
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
            managed.sublayer = (_c = managed.instance.sublayer) !== null && _c !== void 0 ? _c : 0;
            i += 1;
        }
        this.iterating = false;
        if (this.pendingRemovals.size > 0) {
            for (const instance of this.pendingRemovals) {
                if (this.removeActiveInstance(instance)) {
                    (_d = instance.dispose) === null || _d === void 0 ? void 0 : _d.call(instance);
                }
            }
            this.pendingRemovals.clear();
        }
    }
    drawAll(frame) {
        this.drawLayerRange(frame);
    }
    drawLayerRange(frame, minLayer = Number.NEGATIVE_INFINITY, maxLayer = Number.POSITIVE_INFINITY, options = {}) {
        const { resetDrawn = true } = options;
        if (resetDrawn) {
            this.stats.drawn = 0;
        }
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
        const clampedMin = Number.isFinite(minLayer)
            ? minLayer
            : Number.NEGATIVE_INFINITY;
        const clampedMax = Number.isFinite(maxLayer)
            ? maxLayer
            : Number.POSITIVE_INFINITY;
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
            const layerValue = managed.layer;
            if (layerValue < clampedMin || layerValue > clampedMax) {
                continue;
            }
            managed.instance.draw(frame);
            this.stats.drawn += 1;
        }
    }
    collectDecals() {
        var _a, _b, _c;
        if (this.finished.length === 0) {
            return [];
        }
        const decals = [];
        for (const instance of this.finished) {
            const decal = (_b = (_a = instance.handoffToDecal) === null || _a === void 0 ? void 0 : _a.call(instance)) !== null && _b !== void 0 ? _b : null;
            if (decal) {
                decals.push(decal);
            }
            (_c = instance.dispose) === null || _c === void 0 ? void 0 : _c.call(instance);
        }
        this.finished = [];
        return decals;
    }
    getLastFrameStats() {
        return { ...this.stats };
    }
    removeInstance(instance) {
        var _a, _b;
        if (!instance) {
            return false;
        }
        let disposed = false;
        const finishedIndex = this.finished.indexOf(instance);
        if (finishedIndex !== -1) {
            this.finished.splice(finishedIndex, 1);
            (_a = instance.dispose) === null || _a === void 0 ? void 0 : _a.call(instance);
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
        }
        else {
            foundActive = this.removeActiveInstance(instance);
            if (foundActive && !disposed) {
                (_b = instance.dispose) === null || _b === void 0 ? void 0 : _b.call(instance);
                disposed = true;
            }
        }
        return disposed || foundActive;
    }
    track(instance) {
        var _a;
        this.effects.push({
            instance,
            layer: instance.layer,
            sublayer: (_a = instance.sublayer) !== null && _a !== void 0 ? _a : 0,
            creationIndex: this.creationCounter++,
            culled: false,
        });
        return instance;
    }
    removeActiveInstance(instance) {
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
