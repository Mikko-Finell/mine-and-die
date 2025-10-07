export const createPooled = (factory, onRelease) => {
    const pool = [];
    return {
        acquire: () => {
            var _a, _b;
            const instance = (_a = pool.pop()) !== null && _a !== void 0 ? _a : factory();
            (_b = instance.reset) === null || _b === void 0 ? void 0 : _b.call(instance);
            return instance;
        },
        release: (instance) => {
            onRelease === null || onRelease === void 0 ? void 0 : onRelease(instance);
            pool.push(instance);
        },
        size: () => pool.length,
    };
};
