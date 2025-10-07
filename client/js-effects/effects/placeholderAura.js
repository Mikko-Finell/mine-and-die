import { EffectLayer, } from "../types.js";
const TAU = Math.PI * 2;
class PlaceholderAuraInstance {
    constructor(opts) {
        this.id = `placeholder-aura-${Math.random().toString(36).slice(2)}`;
        this.type = PlaceholderAuraDefinition.type;
        this.layer = EffectLayer.ActorOverlay;
        this.kind = "loop";
        this.elapsed = 0;
        this.currentPulse = 0.5;
        this.currentRadius = 0;
        this.aabb = { x: 0, y: 0, w: 0, h: 0 };
        this.opts = { ...PlaceholderAuraDefinition.defaults, ...opts };
        this.center = { x: opts.x, y: opts.y };
        this.updatePulseState(0);
        this.updateAABB();
    }
    isAlive() {
        return true;
    }
    getAABB() {
        return this.aabb;
    }
    update(frame) {
        const dt = Math.max(0, frame.dt);
        if (dt <= 0) {
            return;
        }
        this.elapsed += dt * this.opts.pulseSpeed;
        this.updatePulseState(this.elapsed);
        this.updateAABB();
    }
    draw(frame) {
        const { ctx, camera } = frame;
        const particles = Math.max(1, Math.floor(this.opts.particleCount));
        const colors = this.opts.colors.length > 0 ? this.opts.colors : ["#ff7f50"];
        const particleRadius = 24 + this.currentPulse * 12;
        const screenX = camera.toScreenX(this.center.x);
        const screenY = camera.toScreenY(this.center.y);
        ctx.save();
        ctx.translate(screenX, screenY);
        ctx.scale(camera.zoom, camera.zoom);
        for (let i = 0; i < particles; i += 1) {
            const angle = (i / particles) * TAU;
            const px = Math.cos(angle) * this.currentRadius;
            const py = Math.sin(angle) * this.currentRadius;
            const colorIndex = i % colors.length;
            const gradient = ctx.createRadialGradient(px, py, 0, px, py, particleRadius);
            gradient.addColorStop(0, colors[colorIndex]);
            gradient.addColorStop(1, "rgba(255, 255, 255, 0)");
            ctx.beginPath();
            ctx.fillStyle = gradient;
            ctx.arc(px, py, particleRadius * 0.8, 0, TAU);
            ctx.fill();
        }
        ctx.restore();
    }
    dispose() { }
    updatePulseState(time) {
        const pulse = Math.sin(time) * 0.5 + 0.5;
        this.currentPulse = pulse;
        this.currentRadius = this.opts.radius * (0.75 + pulse * 0.5);
    }
    updateAABB() {
        const particleRadius = 24 + this.currentPulse * 12;
        const reach = this.currentRadius + particleRadius;
        this.aabb.x = this.center.x - reach;
        this.aabb.y = this.center.y - reach;
        this.aabb.w = reach * 2;
        this.aabb.h = reach * 2;
    }
}
export const PlaceholderAuraDefinition = {
    type: "placeholder-aura",
    defaults: {
        radius: 60,
        pulseSpeed: 2,
        particleCount: 12,
        colors: ["#ff7f50", "#ffa500"],
    },
    create: (opts) => new PlaceholderAuraInstance(opts),
    createFromPreset: (position, preset, overrides) => {
        var _a;
        const baseOptions = (_a = preset === null || preset === void 0 ? void 0 : preset.options) !== null && _a !== void 0 ? _a : {};
        const merged = {
            ...PlaceholderAuraDefinition.defaults,
            ...baseOptions,
            ...overrides,
        };
        return new PlaceholderAuraInstance({ ...merged, x: position.x, y: position.y });
    },
};
