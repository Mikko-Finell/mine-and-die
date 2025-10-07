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
        var _a;
        this.stats.updated = 0;
        this.stats.culled = 0;
        for (let i = 0; i < this.effects.length;) {
            const managed = this.effects[i];
            if (managed.culled) {
                this.stats.culled += 1;
                i += 1;
                continue;
            }
            managed.instance.update(frame);
            this.stats.updated += 1;
            if (!managed.instance.isAlive()) {
                this.finished.push(managed.instance);
                this.effects.splice(i, 1);
                continue;
            }
            managed.layer = managed.instance.layer;
            managed.sublayer = (_a = managed.instance.sublayer) !== null && _a !== void 0 ? _a : 0;
            i += 1;
        }
    }
    drawAll(frame) {
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
        var _a;
        if (!instance) {
            return false;
        }
        let removed = false;
        for (let index = this.effects.length - 1; index >= 0; index -= 1) {
            const managed = this.effects[index];
            if (managed.instance === instance) {
                this.effects.splice(index, 1);
                removed = true;
            }
        }
        const finishedIndex = this.finished.indexOf(instance);
        if (finishedIndex !== -1) {
            this.finished.splice(finishedIndex, 1);
            removed = true;
        }
        if (removed) {
            (_a = instance.dispose) === null || _a === void 0 ? void 0 : _a.call(instance);
        }
        return removed;
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
}
