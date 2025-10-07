import { EffectLayer, } from "../types.js";
const DEFAULT_MID = "#7a0e12";
const DEFAULT_DARK = "#4a090b";
const TAU = Math.PI * 2;
const randomRange = (rand, min, max) => min + rand() * (max - min);
const randomInt = (rand, min, max) => Math.floor(randomRange(rand, min, max + 1));
const createBlob = (rand, radius, points, jaggedness) => {
    const coords = [];
    const step = TAU / points;
    for (let i = 0; i < points; i += 1) {
        const angle = step * i;
        const variedRadius = radius * (1 - jaggedness * 0.5 + rand() * jaggedness);
        coords.push(Math.cos(angle) * variedRadius, Math.sin(angle) * variedRadius);
    }
    return coords;
};
class BloodSplatterInstance {
    constructor(opts) {
        this.id = `blood-splatter-${Math.random().toString(36).slice(2)}`;
        this.type = BloodSplatterDefinition.type;
        this.layer = EffectLayer.GroundDecal;
        this.sublayer = 0;
        this.kind = "loop";
        this.droplets = [];
        this.stains = [];
        this.spawnTimer = 0;
        this.aabb = { x: 0, y: 0, w: 0, h: 0 };
        this.stainCursor = 0;
        this.burstCount = 0;
        this.finished = false;
        this.finalDecal = null;
        this.opts = { ...BloodSplatterDefinition.defaults, ...opts };
        this.origin = { x: opts.x, y: opts.y };
        const maxBursts = Number.isFinite(this.opts.maxBursts)
            ? Math.max(1, Math.floor(this.opts.maxBursts))
            : Number.POSITIVE_INFINITY;
        this.maxBurstCount = maxBursts;
        this.kind = Number.isFinite(maxBursts) ? "once" : "loop";
        // Seed with an initial burst so the effect is visible immediately.
        const rand = Math.random;
        this.spawnBurst(rand, this.origin.x, this.origin.y);
        this.burstCount = 1;
        this.recalculateAABB();
    }
    isAlive() {
        return !this.finished;
    }
    getAABB() {
        return this.aabb;
    }
    update(frame) {
        var _a, _b;
        if (this.finished) {
            return;
        }
        const rand = (_b = (_a = frame.rng) === null || _a === void 0 ? void 0 : _a.next.bind(frame.rng)) !== null && _b !== void 0 ? _b : Math.random;
        const dt = Math.max(0, frame.dt);
        if (dt <= 0) {
            return;
        }
        const speedMultiplier = Math.max(this.opts.speed, 0.0001);
        const interval = Math.max(0.016, this.opts.spawnInterval / speedMultiplier);
        this.spawnTimer += dt;
        while (this.spawnTimer >= interval &&
            this.burstCount < this.maxBurstCount) {
            this.spawnBurst(rand, this.origin.x, this.origin.y);
            this.spawnTimer -= interval;
            this.burstCount += 1;
        }
        this.updateDroplets(dt, rand);
        this.recalculateAABB();
        if (this.burstCount >= this.maxBurstCount && this.droplets.length === 0) {
            this.captureFinalDecal();
            this.finished = true;
        }
    }
    draw(frame) {
        var _a;
        if (this.finished) {
            return;
        }
        const { ctx, camera } = frame;
        const { dropletRadius, colors } = this.opts;
        const midColor = (_a = colors[0]) !== null && _a !== void 0 ? _a : DEFAULT_MID;
        // Draw stains first so droplets appear on top.
        for (const stain of this.stains) {
            const screenX = camera.toScreenX(stain.x);
            const screenY = camera.toScreenY(stain.y);
            ctx.save();
            ctx.translate(screenX, screenY);
            ctx.scale(camera.zoom, camera.zoom);
            ctx.rotate(stain.rotation);
            ctx.scale(1, stain.squish);
            ctx.beginPath();
            const base = stain.basePath;
            ctx.moveTo(base[0], base[1]);
            for (let i = 2; i < base.length; i += 2) {
                ctx.lineTo(base[i], base[i + 1]);
            }
            ctx.closePath();
            ctx.fillStyle = stain.darkColor;
            ctx.fill();
            ctx.beginPath();
            const mid = stain.midPath;
            ctx.moveTo(mid[0], mid[1]);
            for (let i = 2; i < mid.length; i += 2) {
                ctx.lineTo(mid[i], mid[i + 1]);
            }
            ctx.closePath();
            ctx.fillStyle = stain.midColor;
            ctx.fill();
            ctx.restore();
        }
        const dropletRadiusX = dropletRadius * camera.zoom;
        const dropletRadiusY = dropletRadius * 0.65 * camera.zoom;
        for (const droplet of this.droplets) {
            const remaining = Math.max(0, Math.min(1, droplet.life / 0.9));
            const alpha = 0.7 * (0.4 + 0.6 * remaining);
            const screenX = camera.toScreenX(droplet.x);
            const screenY = camera.toScreenY(droplet.y);
            ctx.save();
            ctx.globalAlpha = alpha;
            ctx.fillStyle = midColor;
            ctx.beginPath();
            ctx.ellipse(screenX, screenY, dropletRadiusX, dropletRadiusY, 0, 0, TAU);
            ctx.fill();
            ctx.restore();
        }
    }
    dispose() {
        this.droplets.length = 0;
        this.stains.length = 0;
    }
    handoffToDecal() {
        return this.finalDecal;
    }
    spawnBurst(rand, cx, cy) {
        const minCount = Math.min(this.opts.minDroplets, this.opts.maxDroplets);
        const maxCount = Math.max(this.opts.minDroplets, this.opts.maxDroplets);
        const count = randomInt(rand, minCount, maxCount);
        const speedFactor = this.opts.speed;
        const baseSpeed = 120 * speedFactor;
        for (let i = 0; i < count; i += 1) {
            const angle = randomRange(rand, 0, TAU);
            const speed = baseSpeed * randomRange(rand, 0.7, 1.3);
            this.droplets.push({
                x: cx,
                y: cy,
                vx: Math.cos(angle) * speed,
                vy: Math.sin(angle) * speed * 0.6,
                life: randomRange(rand, 0.5, 0.9),
                minLife: randomRange(rand, 0.25, 0.35),
                age: 0,
            });
        }
    }
    updateDroplets(dt, rand) {
        if (this.droplets.length === 0) {
            return;
        }
        const dragBase = Math.min(Math.max(this.opts.drag, 0.5), 0.995);
        const dragFactor = Math.pow(dragBase, dt * 60);
        const landingSpeed = 18 * this.opts.speed;
        const landingSpeedSq = landingSpeed * landingSpeed;
        for (let i = this.droplets.length - 1; i >= 0; i -= 1) {
            const droplet = this.droplets[i];
            droplet.age += dt;
            droplet.life -= dt;
            droplet.vx *= dragFactor;
            droplet.vy *= dragFactor;
            droplet.x += droplet.vx * dt;
            droplet.y += droplet.vy * dt;
            const speedSq = droplet.vx * droplet.vx + droplet.vy * droplet.vy;
            const landed = droplet.age > droplet.minLife && speedSq <= landingSpeedSq;
            if (droplet.life <= 0 || landed) {
                this.addStain(droplet.x, droplet.y, rand);
                const last = this.droplets.pop();
                if (last && i < this.droplets.length) {
                    this.droplets[i] = last;
                }
            }
        }
    }
    addStain(x, y, rand) {
        var _a, _b, _c;
        const colors = (_a = this.opts.colors) !== null && _a !== void 0 ? _a : [DEFAULT_MID, DEFAULT_DARK];
        const midColor = (_b = colors[0]) !== null && _b !== void 0 ? _b : DEFAULT_MID;
        const darkColor = (_c = colors[1]) !== null && _c !== void 0 ? _c : DEFAULT_DARK;
        const minRadius = Math.min(this.opts.minStainRadius, this.opts.maxStainRadius);
        const maxRadius = Math.max(this.opts.minStainRadius, this.opts.maxStainRadius);
        const radius = randomRange(rand, minRadius, maxRadius);
        const points = randomInt(rand, 6, 10);
        const rotation = randomRange(rand, -0.7, 0.7);
        const squish = randomRange(rand, 0.55, 0.78);
        const baseRadius = radius * 1.05;
        const stain = {
            x,
            y,
            rotation,
            squish,
            basePath: createBlob(rand, baseRadius, points, 0.4),
            midPath: createBlob(rand, radius, points, 0.35),
            midColor,
            darkColor,
            boundingRadius: baseRadius * 1.5,
        };
        if (this.stains.length < this.opts.maxStains) {
            this.stains.push(stain);
            return;
        }
        // Overwrite the oldest stain in a circular buffer to avoid reallocations.
        this.stains[this.stainCursor] = stain;
        this.stainCursor = (this.stainCursor + 1) % this.stains.length;
    }
    recalculateAABB() {
        const { dropletRadius } = this.opts;
        let minX = this.origin.x;
        let minY = this.origin.y;
        let maxX = this.origin.x;
        let maxY = this.origin.y;
        let hasContent = false;
        for (const droplet of this.droplets) {
            const radius = dropletRadius;
            const dxMin = droplet.x - radius;
            const dxMax = droplet.x + radius;
            const dyMin = droplet.y - radius;
            const dyMax = droplet.y + radius;
            if (!hasContent) {
                minX = dxMin;
                maxX = dxMax;
                minY = dyMin;
                maxY = dyMax;
                hasContent = true;
            }
            else {
                minX = Math.min(minX, dxMin);
                maxX = Math.max(maxX, dxMax);
                minY = Math.min(minY, dyMin);
                maxY = Math.max(maxY, dyMax);
            }
        }
        for (const stain of this.stains) {
            const radius = stain.boundingRadius;
            const dxMin = stain.x - radius;
            const dxMax = stain.x + radius;
            const dyMin = stain.y - radius;
            const dyMax = stain.y + radius;
            if (!hasContent) {
                minX = dxMin;
                maxX = dxMax;
                minY = dyMin;
                maxY = dyMax;
                hasContent = true;
            }
            else {
                minX = Math.min(minX, dxMin);
                maxX = Math.max(maxX, dxMax);
                minY = Math.min(minY, dyMin);
                maxY = Math.max(maxY, dyMax);
            }
        }
        if (!hasContent) {
            const padding = dropletRadius * 2;
            this.aabb.x = this.origin.x - padding;
            this.aabb.y = this.origin.y - padding;
            this.aabb.w = padding * 2;
            this.aabb.h = padding * 2;
            return;
        }
        this.aabb.x = minX;
        this.aabb.y = minY;
        this.aabb.w = maxX - minX;
        this.aabb.h = maxY - minY;
    }
    captureFinalDecal() {
        if (this.finalDecal || typeof document === "undefined") {
            return;
        }
        if (this.stains.length === 0) {
            this.finalDecal = null;
            return;
        }
        const bounds = this.aabb;
        const width = Math.max(1, Math.ceil(bounds.w));
        const height = Math.max(1, Math.ceil(bounds.h));
        const canvas = document.createElement("canvas");
        canvas.width = width + 8;
        canvas.height = height + 8;
        const ctx = canvas.getContext("2d");
        if (!ctx) {
            return;
        }
        const offsetX = bounds.x - 4;
        const offsetY = bounds.y - 4;
        ctx.translate(-offsetX, -offsetY);
        for (const stain of this.stains) {
            ctx.save();
            ctx.translate(stain.x, stain.y);
            ctx.rotate(stain.rotation);
            ctx.scale(1, stain.squish);
            ctx.beginPath();
            const base = stain.basePath;
            ctx.moveTo(base[0], base[1]);
            for (let i = 2; i < base.length; i += 2) {
                ctx.lineTo(base[i], base[i + 1]);
            }
            ctx.closePath();
            ctx.fillStyle = stain.darkColor;
            ctx.fill();
            ctx.beginPath();
            const mid = stain.midPath;
            ctx.moveTo(mid[0], mid[1]);
            for (let i = 2; i < mid.length; i += 2) {
                ctx.lineTo(mid[i], mid[i + 1]);
            }
            ctx.closePath();
            ctx.fillStyle = stain.midColor;
            ctx.fill();
            ctx.restore();
        }
        this.finalDecal = {
            x: bounds.x + bounds.w / 2,
            y: bounds.y + bounds.h / 2,
            texture: canvas,
            layerHint: "GroundDecal",
        };
    }
}
export const BloodSplatterDefinition = {
    type: "blood-splatter",
    defaults: {
        spawnInterval: 1.6,
        minDroplets: 18,
        maxDroplets: 26,
        dropletRadius: 3,
        minStainRadius: 12,
        maxStainRadius: 28,
        drag: 0.94,
        speed: 1,
        colors: [DEFAULT_MID, DEFAULT_DARK],
        maxStains: 140,
        maxBursts: Number.POSITIVE_INFINITY,
    },
    create: (opts) => new BloodSplatterInstance(opts),
    createFromPreset: (position, preset, overrides) => {
        var _a;
        const baseOptions = (_a = preset === null || preset === void 0 ? void 0 : preset.options) !== null && _a !== void 0 ? _a : {};
        const merged = {
            ...BloodSplatterDefinition.defaults,
            ...baseOptions,
            ...overrides,
        };
        return new BloodSplatterInstance({ ...merged, x: position.x, y: position.y });
    },
};
