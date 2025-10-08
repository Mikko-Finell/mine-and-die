import { EffectLayer } from "./types.js";

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
const nowSeconds = () => (typeof performance !== "undefined" ? performance.now() / 1000 : Date.now() / 1000);
const resolveDecalBounds = (spec) => {
    const texture = spec === null || spec === void 0 ? void 0 : spec.texture;
    if (texture && typeof texture === "object") {
        const width = Number.isFinite(texture.width) ? texture.width : 0;
        const height = Number.isFinite(texture.height) ? texture.height : 0;
        if (width > 0 && height > 0) {
            return { width, height };
        }
    }
    const shape = spec === null || spec === void 0 ? void 0 : spec.shape;
    if (shape && typeof shape === "object") {
        if (shape.type === "oval") {
            const rx = Number.isFinite(shape.rx) ? shape.rx : 0;
            const ry = Number.isFinite(shape.ry) ? shape.ry : 0;
            return { width: rx * 2, height: ry * 2 };
        }
        if (shape.type === "rect") {
            const w = Number.isFinite(shape.w) ? shape.w : 0;
            const h = Number.isFinite(shape.h) ? shape.h : 0;
            return { width: w, height: h };
        }
        if (shape.type === "poly" && Array.isArray(shape.points) && shape.points.length >= 4) {
            let minX = 0;
            let maxX = 0;
            let minY = 0;
            let maxY = 0;
            for (let i = 0; i < shape.points.length; i += 2) {
                const px = shape.points[i];
                const py = shape.points[i + 1];
                if (i === 0) {
                    minX = maxX = px;
                    minY = maxY = py;
                }
                else {
                    minX = Math.min(minX, px);
                    maxX = Math.max(maxX, px);
                    minY = Math.min(minY, py);
                    maxY = Math.max(maxY, py);
                }
            }
            return { width: maxX - minX, height: maxY - minY };
        }
    }
    const fallback = Number.isFinite(spec === null || spec === void 0 ? void 0 : spec.size) ? spec.size : 12;
    return { width: fallback, height: fallback };
};
class DecalInstance {
    constructor(spec, timestamp) {
        var _a, _b;
        this.type = "ground-decal";
        this.layer = EffectLayer.GroundDecal;
        this.sublayer = (_a = spec === null || spec === void 0 ? void 0 : spec.sublayer) !== null && _a !== void 0 ? _a : 0;
        this.kind = "static";
        this.finished = false;
        this.spec = spec !== null && spec !== void 0 ? spec : {};
        this.spawnedAt = Number.isFinite(timestamp) ? timestamp : nowSeconds();
        this.ttl = Number.isFinite(spec === null || spec === void 0 ? void 0 : spec.ttl) ? Math.max(0, spec.ttl) : null;
        this.id = typeof (spec === null || spec === void 0 ? void 0 : spec.id) === "string" && spec.id.length > 0
            ? spec.id
            : `decal-${Math.random().toString(36).slice(2)}`;
        const bounds = resolveDecalBounds(spec !== null && spec !== void 0 ? spec : {});
        const centerX = Number.isFinite(spec === null || spec === void 0 ? void 0 : spec.x) ? spec.x : 0;
        const centerY = Number.isFinite(spec === null || spec === void 0 ? void 0 : spec.y) ? spec.y : 0;
        this.aabb = {
            x: centerX - bounds.width / 2,
            y: centerY - bounds.height / 2,
            w: bounds.width,
            h: bounds.height,
        };
        this.averageColor = typeof (spec === null || spec === void 0 ? void 0 : spec.averageColor) === "string" && ((_b = spec === null || spec === void 0 ? void 0 : spec.averageColor) === null || _b === void 0 ? void 0 : _b.length) > 0
            ? spec.averageColor
            : "rgba(127, 29, 29, 0.85)";
    }
    getAABB() {
        return this.aabb;
    }
    isAlive() {
        if (this.ttl === null) {
            return !this.finished;
        }
        const age = nowSeconds() - this.spawnedAt;
        return !this.finished && age < this.ttl;
    }
    update(frame) {
        if (this.ttl === null) {
            return;
        }
        const current = Number.isFinite(frame === null || frame === void 0 ? void 0 : frame.now) ? frame.now : nowSeconds();
        const age = current - this.spawnedAt;
        if (age >= this.ttl) {
            this.finished = true;
        }
    }
    draw(frame) {
        var _a, _b, _c, _d;
        const { ctx } = frame;
        if (!ctx || this.finished) {
            return;
        }
        const spec = this.spec;
        const x = Number.isFinite(spec.x) ? spec.x : 0;
        const y = Number.isFinite(spec.y) ? spec.y : 0;
        const rotation = Number.isFinite(spec.rotation) ? spec.rotation : 0;
        const texture = spec.texture;
        const shape = spec.shape;
        const defaultColor = this.averageColor;
        ctx.save();
        ctx.translate(x, y);
        if (rotation !== 0) {
            ctx.rotate(rotation);
        }
        const hasCanvas = typeof HTMLCanvasElement !== "undefined" && texture instanceof HTMLCanvasElement;
        const hasBitmap = typeof ImageBitmap !== "undefined" && texture instanceof ImageBitmap;
        if (hasCanvas || hasBitmap) {
            const width = Number.isFinite(texture.width) ? texture.width : 0;
            const height = Number.isFinite(texture.height) ? texture.height : 0;
            ctx.drawImage(texture, -width / 2, -height / 2, width, height);
            ctx.restore();
            return;
        }
        if (shape && typeof shape === "object") {
            ctx.fillStyle = defaultColor;
            if (shape.type === "oval") {
                const rx = Number.isFinite(shape.rx) ? shape.rx : 0;
                const ry = Number.isFinite(shape.ry) ? shape.ry : 0;
                if (rx > 0 && ry > 0) {
                    ctx.beginPath();
                    ctx.ellipse(0, 0, rx, ry, 0, 0, Math.PI * 2);
                    ctx.fill();
                }
            }
            else if (shape.type === "rect") {
                const w = Number.isFinite(shape.w) ? shape.w : 0;
                const h = Number.isFinite(shape.h) ? shape.h : 0;
                if (w > 0 && h > 0) {
                    ctx.fillRect(-w / 2, -h / 2, w, h);
                }
            }
            else if (shape.type === "poly" && Array.isArray(shape.points)) {
                const points = shape.points;
                if (points.length >= 4 && points.length % 2 === 0) {
                    ctx.beginPath();
                    ctx.moveTo(points[0], points[1]);
                    for (let i = 2; i < points.length; i += 2) {
                        ctx.lineTo(points[i], points[i + 1]);
                    }
                    ctx.closePath();
                    ctx.fill();
                }
            }
            ctx.restore();
            return;
        }
        if (typeof texture === "string" && texture) {
            ctx.fillStyle = texture;
            const size = Number.isFinite((_b = (_a = this.spec) === null || _a === void 0 ? void 0 : _a.size) !== null && _b !== void 0 ? _b : NaN) ? (_d = (_c = this.spec) === null || _c === void 0 ? void 0 : _c.size) !== null && _d !== void 0 ? _d : 12 : 12;
            ctx.fillRect(-size / 2, -size / 2, size, size);
            ctx.restore();
            return;
        }
        ctx.fillStyle = defaultColor;
        ctx.fillRect(-6, -6, 12, 12);
        ctx.restore();
    }
    dispose() {
        this.finished = true;
        this.spec = {};
    }
}
/**
 * EffectManager owns the runtime for visual effects: spawning, syncing, culling,
 * updating, rendering, trigger dispatch, and decal hand-off.
 */
export class EffectManager {
    constructor() {
        this.effects = [];
        this.finished = [];
        this.pendingRemovals = new Set();
        this.iterating = false;
        this.creationCounter = 0;
        this.viewBounds = null;
        this.stats = { updated: 0, drawn: 0, culled: 0 };
        this.instancesByType = new Map();
        this.instanceMetadata = new Map();
        this.triggerHandlers = new Map();
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
        this.instancesByType.clear();
        this.instanceMetadata.clear();
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
                this.unregisterInstance(managed.instance);
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
                this.unregisterInstance(managed.instance);
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
        const clampedMin = Number.isFinite(minLayer) ? minLayer : Number.NEGATIVE_INFINITY;
        const clampedMax = Number.isFinite(maxLayer) ? maxLayer : Number.POSITIVE_INFINITY;
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
    collectDecals(timestamp) {
        var _a, _b, _c;
        if (this.finished.length === 0) {
            return [];
        }
        const decals = [];
        for (const instance of this.finished) {
            const decal = (_b = (_a = instance.handoffToDecal) === null || _a === void 0 ? void 0 : _a.call(instance)) !== null && _b !== void 0 ? _b : null;
            if (decal) {
                const decalInstance = new DecalInstance(decal, Number.isFinite(timestamp) ? timestamp : nowSeconds());
                this.track(decalInstance);
                decals.push(decalInstance);
            }
            (_c = instance.dispose) === null || _c === void 0 ? void 0 : _c.call(instance);
        }
        this.finished = [];
        return decals;
    }
    getLastFrameStats() {
        return { ...this.stats };
    }
    getTrackedInstances(type) {
        if (!type) {
            return new Map();
        }
        const existing = this.instancesByType.get(type);
        if (existing) {
            return existing;
        }
        const created = new Map();
        this.instancesByType.set(type, created);
        return created;
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
            this.unregisterInstance(instance);
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
                this.unregisterInstance(instance);
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
        this.registerInstance(instance);
        return instance;
    }
    removeActiveInstance(instance) {
        let removed = false;
        for (let index = this.effects.length - 1; index >= 0; index -= 1) {
            if (this.effects[index].instance === instance) {
                this.effects.splice(index, 1);
                this.unregisterInstance(instance);
                removed = true;
            }
        }
        return removed;
    }
    registerTrigger(type, handler) {
        if (typeof type !== "string" || type.length === 0) {
            return;
        }
        if (typeof handler !== "function") {
            return;
        }
        this.triggerHandlers.set(type, handler);
    }
    trigger(type, payload, context) {
        if (typeof type !== "string" || type.length === 0) {
            return;
        }
        const handler = this.triggerHandlers.get(type);
        if (!handler) {
            return;
        }
        handler({ manager: this, trigger: payload, context });
    }
    triggerAll(triggers, context) {
        if (!Array.isArray(triggers) || triggers.length === 0) {
            return;
        }
        for (const trigger of triggers) {
            if (!trigger || typeof trigger !== "object") {
                continue;
            }
            const type = typeof trigger.type === "string" ? trigger.type : "";
            if (!type) {
                continue;
            }
            this.trigger(type, trigger, context);
        }
    }
    registerInstance(instance) {
        if (!instance || typeof instance !== "object") {
            return;
        }
        const type = typeof instance.type === "string" ? instance.type : "";
        const id = typeof instance.id === "string" ? instance.id : "";
        if (!type || !id) {
            return;
        }
        const typed = this.getTrackedInstances(type);
        typed.set(id, instance);
        this.instanceMetadata.set(instance, { type, id });
    }
    unregisterInstance(instance) {
        const meta = this.instanceMetadata.get(instance);
        if (!meta) {
            return;
        }
        const map = this.instancesByType.get(meta.type);
        if (map) {
            map.delete(meta.id);
            if (map.size === 0) {
                this.instancesByType.delete(meta.type);
            }
        }
        this.instanceMetadata.delete(instance);
    }
}
