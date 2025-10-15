const isRecord = (value) => typeof value === "object" && value !== null && !Array.isArray(value);
const normalizePreset = (input) => {
    var _a;
    return {
        name: input.name,
        version: input.version,
        options: { ...((_a = input.options) !== null && _a !== void 0 ? _a : {}) },
        meta: input.meta ? { ...input.meta } : undefined,
    };
};
const validatePresetShape = (candidate) => {
    if (!isRecord(candidate)) {
        throw new Error("Preset must be an object");
    }
    const { options, name, version, meta } = candidate;
    if (!isRecord(options)) {
        throw new Error("Preset.options must be an object");
    }
    if (name !== undefined && typeof name !== "string") {
        throw new Error("Preset.name must be a string if provided");
    }
    if (version !== undefined && typeof version !== "string") {
        throw new Error("Preset.version must be a string if provided");
    }
    if (meta !== undefined) {
        if (!isRecord(meta)) {
            throw new Error("Preset.meta must be an object if provided");
        }
        const { author, notes } = meta;
        if (author !== undefined && typeof author !== "string") {
            throw new Error("Preset.meta.author must be a string if provided");
        }
        if (notes !== undefined && typeof notes !== "string") {
            throw new Error("Preset.meta.notes must be a string if provided");
        }
    }
    return normalizePreset({
        name: name,
        version: version,
        options: options,
        meta: meta,
    });
};
export const loadPreset = async (source) => {
    if (typeof source === "string") {
        const response = await fetch(source);
        if (!response.ok) {
            throw new Error(`Failed to load preset from ${source}: ${response.status}`);
        }
        const data = await response.json();
        return validatePresetShape(data);
    }
    if (!isRecord(source)) {
        throw new Error("Preset source must be a string or object");
    }
    return validatePresetShape(source);
};
