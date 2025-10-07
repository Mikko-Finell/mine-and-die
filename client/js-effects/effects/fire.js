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
        this.flames = [];
        this.spawnTimer = 0;
        this.flameTimer = 0;
        this.opts = { ...FireEffectDefinition.defaults, ...opts };
        this.id = `fire-${Math.random().toString(36).slice(2)}`;
        this.origin = { x: opts.x, y: opts.y };
        this.aabb.x = this.origin.x - 48;
        this.aabb.y = this.origin.y - 64;
        this.aabb.w = 96;
        this.aabb.h = 96;
    }
    isAlive() {
        return !this.finished;
    }
    getAABB() {
        return this.aabb;
    }
    dispose() {
        this.embers.length = 0;
        this.flames.length = 0;
    }
    handoffToDecal() {
        return null;
    }
    update(frame) {
        if (this.finished) {
            return;
        }
        const dt = Math.max(0, frame.dt);
        if (dt <= 0) {
            return;
        }
        const rand = frame.rng && typeof frame.rng.next === "function"
            ? () => frame.rng.next()
            : Math.random;
        const { spawnInterval, embersPerBurst, flamesPerBurst, riseSpeed, swirl, lifeScale, sizeScale, windX, jitter, } = this.opts;
        this.spawnTimer += dt;
        this.flameTimer += dt;
        while (this.spawnTimer >= spawnInterval) {
            this.spawnTimer -= spawnInterval;
            const n = randInt(rand, Math.max(1, embersPerBurst - 1), embersPerBurst + 1);
            for (let i = 0; i < n; i += 1) {
                const ang = randRange(rand, 0, TAU);
                const r = randRange(rand, 0, 6 * sizeScale);
                const px = this.origin.x + Math.cos(ang) * r;
                const py = this.origin.y + Math.sin(ang) * (r * 0.35);
                const vx = windX + randRange(rand, -jitter, jitter);
                const vy = -riseSpeed * randRange(rand, 0.75, 1.25);
                this.embers.push({
                    x: px,
                    y: py,
                    vx,
                    vy,
                    life: randRange(rand, 0.7, 1.2) * lifeScale,
                    age: 0,
                    size: randRange(rand, 1.25, 2.25) * sizeScale,
                });
            }
        }
        const flameInterval = spawnInterval * 0.5;
        while (this.flameTimer >= flameInterval) {
            this.flameTimer -= flameInterval;
            const n = randInt(rand, Math.max(1, flamesPerBurst - 1), flamesPerBurst + 1);
            for (let i = 0; i < n; i += 1) {
                const baseW = randRange(rand, 6, 12) * sizeScale;
                const baseH = randRange(rand, 18, 28) * sizeScale;
                const px = this.origin.x + randRange(rand, -4, 4) * sizeScale;
                const py = this.origin.y + randRange(rand, -2, 2) * sizeScale;
                this.flames.push({
                    x: px,
                    y: py,
                    vx: windX * 0.5 + randRange(rand, -jitter * 0.5, jitter * 0.5),
                    vy: -riseSpeed * randRange(rand, 0.9, 1.1),
                    life: randRange(rand, 0.28, 0.45) * lifeScale,
                    age: 0,
                    w: baseW,
                    h: baseH,
                    rot: randRange(rand, -0.25, 0.25),
                    swaySeed: randRange(rand, 0, TAU),
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
        for (let i = this.flames.length - 1; i >= 0; i -= 1) {
            const f = this.flames[i];
            f.age += dt;
            const t = clamp01(f.age / f.life);
            const grow = t < 0.4 ? t / 0.4 : 1 - (t - 0.4) / 0.6;
            const sway = Math.sin(f.swaySeed + f.age * 10) * swirl * 0.5;
            f.x += (f.vx + sway) * dt;
            f.y += f.vy * dt;
            f.rot += sway * 0.02;
            f.curW = f.w * (0.65 + 0.35 * grow);
            f.curH = f.h * (0.6 + 0.7 * grow);
            if (f.age >= f.life) {
                const last = this.flames.pop();
                if (last && i < this.flames.length) {
                    this.flames[i] = last;
                }
            }
        }
        const rX = 56 * sizeScale;
        const rY = 84 * sizeScale;
        this.aabb.x = this.origin.x - rX;
        this.aabb.y = this.origin.y - rY;
        this.aabb.w = rX * 2;
        this.aabb.h = rY * 2;
    }
    draw(frame) {
        var _a, _b;
        if (this.finished) {
            return;
        }
        const { ctx, camera } = frame;
        const { baseColor, midColor, hotColor, emberColor, emberAlpha, additive, gradientBias, } = this.opts;
        ctx.save();
        if (additive) {
            ctx.globalCompositeOperation = "lighter";
        }
        for (const f of this.flames) {
            const x = camera.toScreenX(f.x);
            const y = camera.toScreenY(f.y);
            const z = camera.zoom;
            const w = ((_a = f.curW) !== null && _a !== void 0 ? _a : f.w) * z;
            const h = ((_b = f.curH) !== null && _b !== void 0 ? _b : f.h) * z;
            const t = clamp01(f.age / f.life);
            const hotT = clamp01(1 - Math.pow(t, gradientBias));
            ctx.save();
            ctx.translate(x, y);
            ctx.rotate(f.rot);
            ctx.scale(1, 1.1);
            const g = ctx.createRadialGradient(0, h * -0.1, 1, 0, -h * 0.35, Math.max(w, h));
            g.addColorStop(0, hotColor);
            g.addColorStop(0.35, midColor);
            g.addColorStop(1, baseColor);
            ctx.globalAlpha = 0.55 + 0.25 * hotT;
            ctx.fillStyle = g;
            ctx.beginPath();
            ctx.ellipse(0, -h * 0.2, w, h, 0, 0, TAU);
            ctx.fill();
            ctx.restore();
            ctx.save();
            ctx.translate(x, y - h * 0.25);
            ctx.rotate(f.rot * 0.6);
            const innerRadius = Math.max(8, Math.min(w, h) * 0.6);
            const g2 = ctx.createRadialGradient(0, 0, 1, 0, 0, innerRadius);
            g2.addColorStop(0, hotColor);
            g2.addColorStop(1, midColor);
            ctx.globalAlpha = 0.45 + 0.4 * hotT;
            ctx.fillStyle = g2;
            ctx.beginPath();
            ctx.ellipse(0, 0, w * 0.45, h * 0.55, 0, 0, TAU);
            ctx.fill();
            ctx.restore();
        }
        for (const p of this.embers) {
            const x = camera.toScreenX(p.x);
            const y = camera.toScreenY(p.y);
            const r = (p.size || 1.6) * camera.zoom;
            const lifeT = clamp01(p.age / p.life);
            const alpha = emberAlpha * (1 - lifeT) * (0.6 + 0.4 * Math.sin(p.age * 20));
            ctx.save();
            ctx.globalAlpha = alpha;
            ctx.fillStyle = emberColor;
            ctx.beginPath();
            ctx.ellipse(x, y, r, r * 0.9, 0, 0, TAU);
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
        flamesPerBurst: 2,
        riseSpeed: 42,
        windX: 4,
        swirl: 6,
        jitter: 10,
        sizeScale: 1,
        lifeScale: 1,
        baseColor: "rgba(255,140,0,0.45)",
        midColor: "rgba(255,180,60,0.65)",
        hotColor: "rgba(255,255,200,0.95)",
        emberColor: "rgba(255,220,150,1.0)",
        emberAlpha: 0.8,
        gradientBias: 0.65,
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
};
