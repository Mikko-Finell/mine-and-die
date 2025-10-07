type Resettable = { reset?(): void };

type PoolFactory<T> = () => T;

type PoolReleaseHook<T> = (instance: T) => void;

export interface InstancePool<T> {
  acquire(): T;
  release(instance: T): void;
  size(): number;
}

export const createPooled = <T extends Resettable>(
  factory: PoolFactory<T>,
  onRelease?: PoolReleaseHook<T>
): InstancePool<T> => {
  const pool: T[] = [];

  return {
    acquire: () => {
      const instance = pool.pop() ?? factory();
      instance.reset?.();
      return instance;
    },
    release: (instance: T) => {
      onRelease?.(instance);
      pool.push(instance);
    },
    size: () => pool.length,
  };
};
