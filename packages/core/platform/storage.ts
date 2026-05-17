// Storage adapter so packages/core/ stays free of browser-specific APIs.
// Each app injects either localStorage (web), electron-store wrapper, or
// AsyncStorage (RN).

export type StorageAdapter = {
  get: (key: string) => string | null;
  set: (key: string, value: string) => void;
  remove: (key: string) => void;
};

let _storage: StorageAdapter = {
  get: () => null,
  set: () => {},
  remove: () => {},
};

export function configureStorage(adapter: StorageAdapter) {
  _storage = adapter;
}

export function storage(): StorageAdapter {
  return _storage;
}
