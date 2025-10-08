import { EffectLayer, } from "../types.js";
const TAU = Math.PI * 2;
const clamp01 = (x) => Math.max(0, Math.min(1, x));
const randRange = (rand, a, b) => a + rand() * (b - a);
const randInt = (rand, a, b) => Math.floor(randRange(rand, a, b + 1));
class FireInstance {
    constructor(opts) {
        this.type = FireEffectDefinition.type;
        this.layer = EffectLayer.ActorOverlay;
        this.sublayer = 0;
        this.kind = "loop";
        this.finished = false;
        this.aabb = { x: 0, y: 0, w: 0, h: 0 };
        this.embers = [];
        this.spawnTimer = 0;
        this.opts = { ...FireEffectDefinition.defaults, ...opts };
        this.id = typeof opts.effectId === "string" && opts.effectId.length > 0
            ? opts.effectId
            : `fire-${Math.random().toString(36).slice(2)}`;
        this.origin = { x: opts.x, y: opts.y };
        const rX = 56 * this.opts.sizeScale;
        const rY = 84 * this.opts.sizeScale;
        this.aabb.x = this.origin.x - rX;
        this.aabb.y = this.origin.y - rY;
        this.aabb.w = rX * 2;
        this.aabb.h = rY * 2;
    }
    isAlive() {
        return !this.finished;
    }
    getAABB() {
        return this.aabb;
    }
    dispose() {
        this.embers.length = 0;
    }
    handoffToDecal() {
        return null;
    }
    update(frame) {
        var _a, _b, _c;
        if (this.finished) {
            return;
        }
        const dt = Math.max(0, frame.dt);
        if (dt <= 0) {
            return;
        }
        const rand = (_c = (_b = (_a = frame.rng) === null || _a === void 0 ? void 0 : _a.next) === null || _b === void 0 ? void 0 : _b.bind(frame.rng)) !== null && _c !== void 0 ? _c : Math.random;
        const { spawnInterval, embersPerBurst, riseSpeed, windX, swirl, jitter, lifeScale, sizeScale, spawnRadius, concentration, emberPalette, } = this.opts;
        this.spawnTimer += dt;
        while (this.spawnTimer >= spawnInterval) {
            this.spawnTimer -= spawnInterval;
            const n = randInt(rand, Math.max(1, embersPerBurst - 1), embersPerBurst + 1);
            for (let i = 0; i < n; i += 1) {
                const biasPow = 1 + concentration * 6;
                const r = spawnRadius * Math.pow(rand(), biasPow);
                const ang = randRange(rand, 0, TAU);
                const px = this.origin.x +
                    Math.cos(ang) * r * (0.9 + 0.2 * rand());
                const py = this.origin.y +
                    Math.sin(ang) * r * (0.35 + 0.15 * rand());
                const vx = windX + randRange(rand, -jitter, jitter);
                const vy = -riseSpeed * randRange(rand, 0.8, 1.25);
                const color = emberPalette.length
                    ? emberPalette[Math.floor(rand() * emberPalette.length) % emberPalette.length]
                    : "white";
                this.embers.push({
                    x: px,
                    y: py,
                    vx,
                    vy,
                    life: randRange(rand, 0.7, 1.2) * lifeScale,
                    age: 0,
                    size: randRange(rand, 1.25, 2.25) * sizeScale,
                    color,
                });
            }
        }
        for (let i = this.embers.length - 1; i >= 0; i -= 1) {
            const p = this.embers[i];
            p.age += dt;
            const s = Math.sin(p.age * 8) * swirl;
            p.vx += s * dt;
            p.x += p.vx * dt;
            p.y += p.vy * dt;
            if (p.age >= p.life) {
                const last = this.embers.pop();
                if (last && i < this.embers.length) {
                    this.embers[i] = last;
                }
            }
        }
    }
    draw(frame) {
        if (this.finished) {
            return;
        }
        const { ctx, camera } = frame;
        const { emberAlpha, additive } = this.opts;
        ctx.save();
        if (additive) {
            ctx.globalCompositeOperation = "lighter";
        }
        for (const p of this.embers) {
            const x = camera.toScreenX(p.x);
            const y = camera.toScreenY(p.y);
            const r = (p.size || 1.6) * camera.zoom;
            const t = clamp01(p.age / p.life);
            const alpha = emberAlpha * (1 - t) * (0.65 + 0.35 * Math.sin(p.age * 18));
            ctx.save();
            ctx.globalAlpha = alpha;
            ctx.fillStyle = p.color;
            ctx.beginPath();
            ctx.arc(x, y, r, 0, TAU);
            ctx.fill();
            ctx.restore();
        }
        ctx.restore();
    }
}
export const FireEffectDefinition = {
    type: "fire",
    defaults: {
        spawnInterval: 0.08,
        embersPerBurst: 6,
        riseSpeed: 42,
        windX: 4,
        swirl: 6,
        jitter: 10,
        lifeScale: 1,
        sizeScale: 1,
        spawnRadius: 10,
        concentration: 0.6,
        emberPalette: [
            "rgba(255, 220, 150, 1.0)",
            "rgba(255, 180, 60, 1.0)",
            "rgba(255, 245, 200, 1.0)",
        ],
        emberAlpha: 0.9,
        additive: true,
    },
    create: (opts) => new FireInstance(opts),
    createFromPreset: (position, preset, overrides) => {
        var _a;
        const baseOptions = (_a = preset === null || preset === void 0 ? void 0 : preset.options) !== null && _a !== void 0 ? _a : {};
        const merged = {
            ...FireEffectDefinition.defaults,
            ...baseOptions,
            ...overrides,
        };
        return new FireInstance({ ...merged, x: position.x, y: position.y });
    },
    fromEffect: (effect, store) => {
        if (!effect || typeof effect !== "object") {
            return null;
        }
        const tileSize = Number.isFinite(store === null || store === void 0 ? void 0 : store.TILE_SIZE) ? store.TILE_SIZE : 40;
        const width = Number.isFinite(effect.width) ? effect.width : tileSize;
        const height = Number.isFinite(effect.height) ? effect.height : tileSize;
        const baseX = Number.isFinite(effect.x) ? effect.x : 0;
        const baseY = Number.isFinite(effect.y) ? effect.y : 0;
        const centerX = baseX + width / 2;
        const centerY = baseY + height / 2;
        const params = (effect === null || effect === void 0 ? void 0 : effect.params) && typeof effect.params === "object"
            ? effect.params
            : {};
        const readNumber = (key, fallback) => {
            const value = params[key];
            return Number.isFinite(value) ? value : fallback;
        };
        return {
            effectId: typeof effect.id === "string" ? effect.id : undefined,
            x: centerX,
            y: centerY,
            additive: true,
            concentration: readNumber("concentration", 0.25),
            emberAlpha: readNumber("emberAlpha", 1),
            embersPerBurst: readNumber("embersPerBurst", 24),
            flamesPerBurst: readNumber("flamesPerBurst", 1),
            gradientBias: readNumber("gradientBias", 1.65),
            jitter: readNumber("jitter", 22.5),
            lifeScale: readNumber("lifeScale", 1.1),
            riseSpeed: readNumber("riseSpeed", 35),
            sizeScale: readNumber("sizeScale", 1.3),
            spawnInterval: readNumber("spawnInterval", 0.06),
            spawnRadius: readNumber("spawnRadius", 15.5),
            swirl: readNumber("swirl", 0.5),
            windX: readNumber("windX", 0),
            emberPalette: Array.isArray(effect.palette) ? effect.palette : FireEffectDefinition.defaults.emberPalette,
        };
    },
};
