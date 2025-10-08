import { EffectLayer } from "../types.js";
class MeleeSwingInstance {
    constructor(opts) {
        this.type = MeleeSwingEffectDefinition.type;
        this.layer = EffectLayer.ActorOverlay;
        this.aabb = { x: 0, y: 0, w: 0, h: 0 };
        this.elapsed = 0;
        this.finished = false;
        const width = Number.isFinite(opts.width)
            ? opts.width
            : MeleeSwingEffectDefinition.defaults.width;
        const height = Number.isFinite(opts.height)
            ? opts.height
            : MeleeSwingEffectDefinition.defaults.height;
        const duration = Math.max(0.05, Number.isFinite(opts.duration)
            ? opts.duration
            : MeleeSwingEffectDefinition.defaults.duration);
        const strokeWidth = Number.isFinite(opts.strokeWidth)
            ? opts.strokeWidth
            : MeleeSwingEffectDefinition.defaults.strokeWidth;
        const innerInset = Number.isFinite(opts.innerInset)
            ? opts.innerInset
            : MeleeSwingEffectDefinition.defaults.innerInset;
        const fadeExponent = Number.isFinite(opts.fadeExponent)
            ? opts.fadeExponent
            : MeleeSwingEffectDefinition.defaults.fadeExponent;
        this.opts = {
            ...MeleeSwingEffectDefinition.defaults,
            ...opts,
            width,
            height,
            duration,
            strokeWidth,
            innerInset,
            fadeExponent,
        };
        this.id =
            typeof opts.effectId === "string"
                ? opts.effectId
                : `melee-swing-${Math.random().toString(36).slice(2)}`;
        const originX = Number.isFinite(opts.x) ? opts.x : 0;
        const originY = Number.isFinite(opts.y) ? opts.y : 0;
        this.origin = { x: originX, y: originY };
        this.aabb.x = originX;
        this.aabb.y = originY;
        this.aabb.w = width;
        this.aabb.h = height;
    }
    isAlive() {
        return !this.finished;
    }
    getAABB() {
        return this.aabb;
    }
    update(frame) {
        var _a;
        if (this.finished) {
            return;
        }
        const dt = Math.max(0, (_a = frame.dt) !== null && _a !== void 0 ? _a : 0);
        if (dt <= 0) {
            return;
        }
        this.elapsed += dt;
        if (this.elapsed >= this.opts.duration) {
            this.finished = true;
        }
    }
    draw(frame) {
        var _a, _b;
        if (this.finished) {
            return;
        }
        const { ctx, camera } = frame;
        const duration = Math.max(this.opts.duration, 0.0001);
        const progress = Math.min(1, this.elapsed / duration);
        const intensity = 1 - Math.pow(progress, this.opts.fadeExponent);
        const screenX = camera.toScreenX(this.origin.x);
        const screenY = camera.toScreenY(this.origin.y);
        ctx.save();
        ctx.translate(screenX, screenY);
        ctx.scale((_a = camera.zoom) !== null && _a !== void 0 ? _a : 1, (_b = camera.zoom) !== null && _b !== void 0 ? _b : 1);
        ctx.globalAlpha = intensity;
        ctx.fillStyle = this.opts.fill;
        ctx.fillRect(0, 0, this.opts.width, this.opts.height);
        const inset = Math.max(0, Math.min(this.opts.innerInset, Math.min(this.opts.width, this.opts.height) / 2));
        if (inset > 0) {
            ctx.globalAlpha = intensity * 0.65;
            ctx.fillStyle = this.opts.innerFill;
            ctx.fillRect(inset, inset, Math.max(0, this.opts.width - inset * 2), Math.max(0, this.opts.height - inset * 2));
        }
        ctx.globalAlpha = Math.min(1, intensity * 1.1);
        ctx.lineWidth = this.opts.strokeWidth;
        ctx.strokeStyle = this.opts.stroke;
        ctx.strokeRect(0, 0, this.opts.width, this.opts.height);
        ctx.restore();
    }
    dispose() {
        // no-op; included for symmetry with other effect instances
    }
}
export const MeleeSwingEffectDefinition = {
    type: "melee-swing",
    defaults: {
        duration: 0.18,
        width: 48,
        height: 48,
        fill: "rgba(239, 68, 68, 0.32)",
        stroke: "rgba(239, 68, 68, 0.9)",
        strokeWidth: 3,
        innerFill: "rgba(254, 242, 242, 0.9)",
        innerInset: 6,
        fadeExponent: 1.5,
    },
    create: (opts) => new MeleeSwingInstance(opts),
    fromEffect: (effect, store) => {
        var _a, _b, _c, _d;
        if (!effect || typeof effect !== "object") {
            return null;
        }
        const fallbackSize = Number.isFinite(store === null || store === void 0 ? void 0 : store.TILE_SIZE)
            ? store.TILE_SIZE
            : MeleeSwingEffectDefinition.defaults.width;
        const width = Number.isFinite(effect.width) ? effect.width : fallbackSize;
        const height = Number.isFinite(effect.height) ? effect.height : fallbackSize;
        const x = Number.isFinite(effect.x) ? effect.x : 0;
        const y = Number.isFinite(effect.y) ? effect.y : 0;
        const durationMs = Number.isFinite(effect.duration) ? effect.duration : 150;
        const durationSeconds = Math.max(0.05, durationMs / 1000 + 0.05);
        const strokeWidth = Math.max(2, Math.min(4, Math.min(width, height) * 0.08));
        const innerInset = Math.max(3, Math.min(width, height) * 0.22);
        const fadeExponent = Number.isFinite((_a = effect.params) === null || _a === void 0 ? void 0 : _a.fadeExponent)
            ? (_b = effect.params) === null || _b === void 0 ? void 0 : _b.fadeExponent
            : MeleeSwingEffectDefinition.defaults.fadeExponent;
        return {
            effectId: typeof effect.id === "string" ? effect.id : undefined,
            x,
            y,
            width,
            height,
            duration: durationSeconds,
            strokeWidth,
            innerInset,
            fadeExponent,
            fill: (_c = effect.fill) !== null && _c !== void 0 ? _c : MeleeSwingEffectDefinition.defaults.fill,
            stroke: (_d = effect.stroke) !== null && _d !== void 0 ? _d : MeleeSwingEffectDefinition.defaults.stroke,
        };
    },
};
