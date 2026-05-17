"use client";

// Persistence for the access/refresh token pair. Uses StorageAdapter so the
// same code works on web (localStorage), desktop (electron-store), and RN
// (AsyncStorage) — no direct localStorage in packages/core.

import { storage } from "../platform/storage";

const ACCESS_KEY = "medigt_access_token";
const REFRESH_KEY = "medigt_refresh_token";

export type TokenPair = {
  accessToken: string;
  refreshToken: string;
};

export function loadTokens(): TokenPair | null {
  const accessToken = storage().get(ACCESS_KEY);
  const refreshToken = storage().get(REFRESH_KEY);
  if (!accessToken || !refreshToken) return null;
  return { accessToken, refreshToken };
}

export function saveTokens(pair: TokenPair): void {
  storage().set(ACCESS_KEY, pair.accessToken);
  storage().set(REFRESH_KEY, pair.refreshToken);
}

export function clearTokens(): void {
  storage().remove(ACCESS_KEY);
  storage().remove(REFRESH_KEY);
}
