"use client";

import { create } from "zustand";
import type { User } from "../types/user";
import { clearTokens, loadTokens, saveTokens, type TokenPair } from "./persist";

type AuthState = {
  user: User | null;
  accessToken: string | null;
  refreshToken: string | null;
  isBootstrapped: boolean;
  isLoading: boolean;
  setUser: (user: User | null) => void;
  setTokens: (pair: TokenPair) => void;
  setSession: (user: User, pair: TokenPair) => void;
  rotateAccess: (accessToken: string) => void;
  hydrateFromStorage: () => void;
  setBootstrapped: (v: boolean) => void;
  setLoading: (v: boolean) => void;
  logout: () => void;
};

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  accessToken: null,
  refreshToken: null,
  isBootstrapped: false,
  isLoading: false,
  setUser: (user) => set({ user }),
  setTokens: (pair) => {
    saveTokens(pair);
    set({ accessToken: pair.accessToken, refreshToken: pair.refreshToken });
  },
  setSession: (user, pair) => {
    saveTokens(pair);
    set({
      user,
      accessToken: pair.accessToken,
      refreshToken: pair.refreshToken,
    });
  },
  rotateAccess: (accessToken) => {
    const refresh = get().refreshToken;
    if (refresh) saveTokens({ accessToken, refreshToken: refresh });
    set({ accessToken });
  },
  hydrateFromStorage: () => {
    const pair = loadTokens();
    if (pair) {
      set({ accessToken: pair.accessToken, refreshToken: pair.refreshToken });
    }
  },
  setBootstrapped: (isBootstrapped) => set({ isBootstrapped }),
  setLoading: (isLoading) => set({ isLoading }),
  logout: () => {
    clearTokens();
    set({ user: null, accessToken: null, refreshToken: null });
  },
}));

export function getAccessToken(): string | null {
  return useAuthStore.getState().accessToken;
}

export function getRefreshToken(): string | null {
  return useAuthStore.getState().refreshToken;
}

export function getCurrentUser(): User | null {
  return useAuthStore.getState().user;
}
