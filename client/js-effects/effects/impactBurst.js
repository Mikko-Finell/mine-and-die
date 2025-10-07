import { EffectLayer, } from "../types";
const TAU = Math.PI * 2;
class ImpactBurstInstance {
    constructor(opts) {
        this.id = `impact-burst-${Math.random().toString(36).slice(2)}`;
        this.type = ImpactBurstDefinition.type;
        this.layer = EffectLayer.ActorOverlay;
        this.kind = "once";
        this.particles = [];
        this.aabb = { x: 0, y: 0, w: 0, h: 0 };
        this.elapsed = 0;
        this.finished = false;
        this.decal = null;
        this.opts = { ...ImpactBurstDefinition.defaults, ...opts };
        this.origin = { x: opts.x, y: opts.y };
        this.seedParticles();
        this.updateAABB(0);
    }
    isAlive() {
        return !this.finished;
    }
    getAABB() {
        return this.aabb;
    }
    update(frame) {
        if (this.finished) {
            return;
        }
        const dt = Math.max(0, frame.dt);
        if (dt <= 0) {
            return;
        }
        this.elapsed += dt;
        this.updateAABB(this.elapsed);
        if (this.elapsed >= this.opts.duration) {
            this.finished = true;
            this.decal = {
                x: this.origin.x,
                y: this.origin.y,
                rotation: 0,
                shape: { type: "oval", rx: this.opts.decalRadius, ry: this.opts.decalRadius * 0.6 },
                averageColor: this.opts.color,
                ttl: this.opts.decalTtl,
                layerHint: "GroundDecal",
            };
        }
    }
    draw(frame) {
        const { ctx, camera } = frame;
        const progress = Math.min(1, this.elapsed / Math.max(this.opts.duration, 0.0001));
        const easing = 1 - Math.pow(1 - progress, 3);
        const radius = this.opts.ringRadius * (0.4 + easing * 0.6);
        const screenX = camera.toScreenX(this.origin.x);
        const screenY = camera.toScreenY(this.origin.y);
        ctx.save();
        ctx.translate(screenX, screenY);
        ctx.scale(camera.zoom, camera.zoom);
        ctx.beginPath();
        ctx.strokeStyle = this.opts.color;
        ctx.lineWidth = 4 * (1 - progress * 0.6);
        ctx.globalAlpha = 0.85 * (1 - progress * 0.5);
        ctx.arc(0, 0, radius, 0, TAU);
        ctx.stroke();
        ctx.globalAlpha = 1;
        ctx.fillStyle = this.opts.secondaryColor;
        for (const particle of this.particles) {
            const distance = radius * (0.4 + easing * particle.speed);
            const px = Math.cos(particle.angle) * distance;
            const py = Math.sin(particle.angle) * distance * 0.65;
            ctx.beginPath();
            ctx.ellipse(px, py, particle.radius, particle.radius * 0.55, 0, 0, TAU);
            ctx.fill();
        }
        ctx.restore();
    }
    dispose() {
        this.particles.length = 0;
    }
    handoffToDecal() {
        return this.decal;
    }
    seedParticles() {
        const count = Math.max(3, Math.floor(this.opts.particleCount));
        for (let i = 0; i < count; i += 1) {
            const angle = (i / count) * TAU;
            const offset = (i % 2 === 0 ? 1 : 0.6) + (i / count) * 0.2;
            this.particles.push({
                angle,
                speed: 0.6 + offset * 0.4,
                radius: 6 + (i % 3),
            });
        }
    }
    updateAABB(time) {
        const radius = this.opts.ringRadius * (0.4 + Math.min(1, time / Math.max(this.opts.duration, 0.0001)));
        const reach = radius + this.opts.decalRadius;
        this.aabb.x = this.origin.x - reach;
        this.aabb.y = this.origin.y - reach;
        this.aabb.w = reach * 2;
        this.aabb.h = reach * 2;
    }
}
export const ImpactBurstDefinition = {
    type: "impact-burst",
    defaults: {
        duration: 0.8,
        ringRadius: 72,
        particleCount: 10,
        color: "#ffd166",
        secondaryColor: "#ffb347",
        decalRadius: 36,
        decalTtl: 6,
    },
    create: (opts) => new ImpactBurstInstance(opts),
    createFromPreset: (position, preset, overrides) => {
        var _a;
        const baseOptions = (_a = preset === null || preset === void 0 ? void 0 : preset.options) !== null && _a !== void 0 ? _a : {};
        const merged = {
            ...ImpactBurstDefinition.defaults,
            ...baseOptions,
            ...overrides,
        };
        return new ImpactBurstInstance({ ...merged, x: position.x, y: position.y });
    },
};
