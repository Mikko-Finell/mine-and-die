type Resettable = {
    reset?(): void;
};
type PoolFactory<T> = () => T;
type PoolReleaseHook<T> = (instance: T) => void;
export interface InstancePool<T> {
    acquire(): T;
    release(instance: T): void;
    size(): number;
}
export declare const createPooled: <T extends Resettable>(factory: PoolFactory<T>, onRelease?: PoolReleaseHook<T>) => InstancePool<T>;
export {};
