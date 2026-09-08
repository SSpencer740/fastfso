import { create } from "zustand";
import type { Identity, SSOOption, User } from "../api/auth";
import * as authApi from "../api/auth";
import { ApiError } from "../api/client";

export type AuthState =
  | "loading"
  | "unauthenticated"
  | "pre_auth"
  | "setup_2fa"
  | "pre_tenant"
  | "authenticated";

interface AuthStore {
  state: AuthState;
  identity: Identity | null;
  user: User | null;
  isSuperAdmin: boolean;
  canAccessAdminPanel: boolean;
  canSwitchContext: boolean;
  available2FA: string[];
  mustSetup2FA: boolean;
  error: string | null;

  // Identifier-first login state
  loginEmail: string | null;
  loginMethods: string[];
  ssoOptions: SSOOption[];

  // Actions
  initialize: () => Promise<void>;
  identify: (email: string) => Promise<void>;
  resetLogin: () => void;
  login: (email: string, password: string) => Promise<void>;
  challengeComplete: (state: AuthState) => void;
  setup2FAComplete: () => void;
  selectTenant: (tenantId: string) => Promise<void>;
  selectAdmin: () => Promise<void>;
  switchContext: () => Promise<void>;
  logout: () => Promise<void>;
  clearError: () => void;
}

export const useAuthStore = create<AuthStore>((set, get) => ({
  state: "loading",
  identity: null,
  user: null,
  isSuperAdmin: false,
  canAccessAdminPanel: false,
  canSwitchContext: false,
  available2FA: [],
  mustSetup2FA: false,
  error: null,
  loginEmail: null,
  loginMethods: [],
  ssoOptions: [],

  initialize: async () => {
    try {
      const me = await authApi.getMe();
      set({
        state: me.state as AuthState,
        identity: me.identity,
        user: me.user ?? null,
        isSuperAdmin: me.is_super_admin,
        canSwitchContext: me.can_switch_context ?? false,
      });
    } catch {
      set({ state: "unauthenticated" });
    }
  },

  identify: async (email) => {
    set({ error: null });
    try {
      const res = await authApi.identify(email);
      set({
        loginEmail: email,
        loginMethods: res.methods,
        ssoOptions: res.sso_options,
      });
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Failed to identify account";
      set({ error: message });
      throw err;
    }
  },

  resetLogin: () =>
    set({
      loginEmail: null,
      loginMethods: [],
      ssoOptions: [],
      error: null,
    }),

  login: async (email, password) => {
    set({ error: null });
    try {
      const res = await authApi.login(email, password);
      set({
        state: res.state,
        available2FA: res.available_2fa ?? [],
        mustSetup2FA: res.setup_2fa_required ?? false,
      });
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Login failed";
      set({ error: message });
      throw err;
    }
  },

  challengeComplete: (state) => set({ state }),

  setup2FAComplete: () => set({ mustSetup2FA: false }),

  selectTenant: async (tenantId) => {
    try {
      await authApi.selectTenant(tenantId);
      const me = await authApi.getMe();
      set({
        state: me.state as AuthState,
        identity: me.identity,
        user: me.user ?? null,
        isSuperAdmin: me.is_super_admin,
        canSwitchContext: me.can_switch_context ?? false,
      });
      await get().initialize();
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Failed to select tenant";
      set({ error: message });
      throw err;
    }
  },

  selectAdmin: async () => {
    try {
      await authApi.selectAdmin();
      const me = await authApi.getMe();
      set({
        state: me.state as AuthState,
        identity: me.identity,
        user: me.user ?? null,
        isSuperAdmin: me.is_super_admin,
        canSwitchContext: me.can_switch_context ?? false,
      });
      await get().initialize();
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Failed to access admin panel";
      set({ error: message });
      throw err;
    }
  },

  switchContext: async () => {
    try {
      await authApi.switchContext();
      set({
        state: "pre_tenant",
        user: null,
        isSuperAdmin: false,
        canSwitchContext: false,
      });
    } catch (err) {
      const message =
        err instanceof ApiError ? err.message : "Failed to switch context";
      set({ error: message });
      throw err;
    }
  },

  logout: async () => {
    try {
      await authApi.logout();
    } catch {
      // Clear state even if the API call fails
    }
    set({
      state: "unauthenticated",
      identity: null,
      user: null,
      isSuperAdmin: false,
      canAccessAdminPanel: false,
      canSwitchContext: false,
      available2FA: [],
      mustSetup2FA: false,
      error: null,
      loginEmail: null,
      loginMethods: [],
      ssoOptions: [],
    });
  },

  clearError: () => set({ error: null }),
}));
