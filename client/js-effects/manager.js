import { EffectLayer, } from "./types.js";
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
const resolveLayerFromHint = (hint) => {
    if (typeof hint !== "string") {
        return EffectLayer.GroundDecal;
    }
    const normalized = hint.trim().toLowerCase();
    if (normalized === "actoroverlay" || normalized === "actor-overlay") {
        return EffectLayer.ActorOverlay;
    }
    return EffectLayer.GroundDecal;
};
const buildDecalAABB = (spec) => {
    var _a, _b;
    const centerX = Number.isFinite(spec.x) ? spec.x : 0;
    const centerY = Number.isFinite(spec.y) ? spec.y : 0;
    const fromShape = () => {
        const shape = spec.shape;
        if (!shape || typeof shape !== "object") {
            return null;
        }
        if (shape.type === "oval") {
            const rx = Number.isFinite(shape.rx) ? shape.rx : 0;
            const ry = Number.isFinite(shape.ry) ? shape.ry : 0;
            return { x: centerX - rx, y: centerY - ry, w: rx * 2, h: ry * 2 };
        }
        if (shape.type === "rect") {
            const w = Number.isFinite(shape.w) ? shape.w : 0;
            const h = Number.isFinite(shape.h) ? shape.h : 0;
            return { x: centerX - w / 2, y: centerY - h / 2, w, h };
        }
        if (shape.type === "poly" && Array.isArray(shape.points)) {
            const points = shape.points;
            if (points.length >= 4 && points.length % 2 === 0) {
                let minX = Infinity;
                let minY = Infinity;
                let maxX = -Infinity;
                let maxY = -Infinity;
                for (let i = 0; i < points.length; i += 2) {
                    const px = Number(points[i]);
                    const py = Number(points[i + 1]);
                    if (Number.isFinite(px) && Number.isFinite(py)) {
                        if (px < minX)
                            minX = px;
                        if (py < minY)
                            minY = py;
                        if (px > maxX)
                            maxX = px;
                        if (py > maxY)
                            maxY = py;
                    }
                }
                if (minX !== Infinity && minY !== Infinity && maxX !== -Infinity && maxY !== -Infinity) {
                    return { x: centerX + minX, y: centerY + minY, w: maxX - minX, h: maxY - minY };
                }
            }
        }
        return null;
    };
    const fromTexture = () => {
        const texture = spec.texture;
        if (!texture || typeof texture === "string") {
            return null;
        }
        const width = Number.isFinite(texture.width)
            ? texture.width
            : Number.isFinite(texture.width)
                ? texture.width
                : 0;
        const height = Number.isFinite(texture.height)
            ? texture.height
            : Number.isFinite(texture.height)
                ? texture.height
                : 0;
        return { x: centerX - width / 2, y: centerY - height / 2, w: width, h: height };
    };
    return ((_b = (_a = fromShape()) !== null && _a !== void 0 ? _a : fromTexture()) !== null && _b !== void 0 ? _b : {
        x: centerX - 6,
        y: centerY - 6,
        w: 12,
        h: 12,
    });
};
const drawDecal = (frame, decal) => {
    const { ctx, camera } = frame;
    const spec = decal.spec;
    const x = Number.isFinite(spec.x) ? spec.x : 0;
    const y = Number.isFinite(spec.y) ? spec.y : 0;
    const rotation = Number.isFinite(spec.rotation) ? spec.rotation : 0;
    const texture = spec.texture;
    const shape = spec.shape;
    const defaultColor = typeof spec.averageColor === "string" && spec.averageColor
        ? spec.averageColor
        : "rgba(127, 29, 29, 0.85)";
    ctx.save();
    ctx.translate(camera.toScreenX(x), camera.toScreenY(y));
    ctx.scale(camera.zoom, camera.zoom);
    if (rotation !== 0) {
        ctx.rotate(rotation);
    }
    const hasCanvas = typeof HTMLCanvasElement !== "undefined" && texture instanceof HTMLCanvasElement;
    const hasBitmap = typeof ImageBitmap !== "undefined" && texture instanceof ImageBitmap;
    if (hasCanvas || hasBitmap) {
        const width = Number.isFinite(texture.width)
            ? texture.width
            : 0;
        const height = Number.isFinite(texture.height)
            ? texture.height
            : 0;
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
        const size = 12;
        ctx.fillRect(-size / 2, -size / 2, size, size);
        ctx.restore();
        return;
    }
    ctx.fillStyle = defaultColor;
    ctx.beginPath();
    ctx.arc(0, 0, 6, 0, Math.PI * 2);
    ctx.fill();
    ctx.restore();
};
const getTimestamp = (nowSeconds) => {
    if (typeof nowSeconds === "number" && Number.isFinite(nowSeconds)) {
        return nowSeconds;
    }
    return Date.now() / 1000;
};
/**
 * EffectManager owns the lifecycle of all visual EffectInstance objects.
 * Hosts feed it simulation state, triggers, and frame context; the manager
 * handles spawning, culling, updating, drawing, and decal ownership so
 * callers never track effect instances themselves.
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
        this.effectIndex = new Map();
        this.decals = [];
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
        this.effectIndex.clear();
        this.decals = [];
    }
    cullByAABB(view) {
        this.viewBounds = view;
        for (const managed of this.effects) {
            const bounds = managed.instance.getAABB();
            managed.culled = !intersects(bounds, view);
        }
        for (const decal of this.decals) {
            decal.culled = !intersects(decal.aabb, view);
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
                if (managed.instance.id) {
                    this.effectIndex.delete(managed.instance.id);
                }
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
        var _a;
        this.stats.drawn = 0;
        const view = this.viewBounds;
        const nowSeconds = (_a = frame.now) !== null && _a !== void 0 ? _a : Date.now() / 1000;
        this.decals = this.decals.filter((decal) => {
            if (nowSeconds >= decal.expiresAt) {
                return false;
            }
            if (view && !intersects(decal.aabb, view)) {
                decal.culled = true;
            }
            return true;
        });
        const sorted = [...this.effects, ...this.decals].sort((a, b) => {
            const layerA = "instance" in a ? a.layer : a.layer;
            const layerB = "instance" in b ? b.layer : b.layer;
            if (layerA !== layerB) {
                return layerA - layerB;
            }
            const sublayerA = "instance" in a ? a.sublayer : a.sublayer;
            const sublayerB = "instance" in b ? b.sublayer : b.sublayer;
            if (sublayerA !== sublayerB) {
                return sublayerA - sublayerB;
            }
            return a.creationIndex - b.creationIndex;
        });
        for (const managed of sorted) {
            if ("instance" in managed) {
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
                continue;
            }
            if (managed.culled && view) {
                this.stats.culled += 1;
                continue;
            }
            drawDecal(frame, managed);
        }
    }
    collectDecals(nowSeconds) {
        var _a, _b, _c;
        if (this.finished.length === 0) {
            return [];
        }
        const decals = [];
        const timestamp = getTimestamp(nowSeconds);
        for (const instance of this.finished) {
            const decal = (_b = (_a = instance.handoffToDecal) === null || _a === void 0 ? void 0 : _a.call(instance)) !== null && _b !== void 0 ? _b : null;
            if (decal) {
                decals.push(decal);
                this.enqueueDecal(decal, timestamp);
            }
            (_c = instance.dispose) === null || _c === void 0 ? void 0 : _c.call(instance);
            if (instance.id) {
                this.effectIndex.delete(instance.id);
            }
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
        if ((disposed || foundActive) && (instance === null || instance === void 0 ? void 0 : instance.id)) {
            this.effectIndex.delete(instance.id);
        }
        return disposed || foundActive;
    }
    getInstanceById(id) {
        var _a;
        const managed = this.effectIndex.get(id);
        return (_a = managed === null || managed === void 0 ? void 0 : managed.instance) !== null && _a !== void 0 ? _a : null;
    }
    getInstancesByType(type) {
        return this.effects
            .filter((entry) => entry.instance.type === type)
            .map((entry) => entry.instance);
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
    trigger(type, trigger, context) {
        if (typeof type !== "string" || type.length === 0) {
            return;
        }
        const handler = this.triggerHandlers.get(type);
        if (!handler) {
            return;
        }
        try {
            handler({ manager: this, trigger, context: context !== null && context !== void 0 ? context : null });
        }
        catch (err) {
            console.error("effect trigger handler failed", err);
        }
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
    track(instance) {
        var _a;
        const managed = {
            instance,
            layer: instance.layer,
            sublayer: (_a = instance.sublayer) !== null && _a !== void 0 ? _a : 0,
            creationIndex: this.creationCounter++,
            culled: false,
        };
        this.effects.push(managed);
        if (typeof instance.id === "string" && instance.id.length > 0) {
            this.effectIndex.set(instance.id, managed);
        }
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
    enqueueDecal(spec, timestamp) {
        const ttl = typeof spec.ttl === "number" && Number.isFinite(spec.ttl) && spec.ttl >= 0
            ? spec.ttl
            : null;
        const layer = resolveLayerFromHint(spec.layerHint);
        const managed = {
            spec,
            layer,
            sublayer: 0,
            creationIndex: this.creationCounter++,
            culled: false,
            expiresAt: ttl === null ? Number.POSITIVE_INFINITY : timestamp + ttl,
            aabb: buildDecalAABB(spec),
        };
        this.decals.push(managed);
    }
}
