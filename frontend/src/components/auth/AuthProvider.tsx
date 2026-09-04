"use client";

import React, {
  createContext,
  useContext,
  useEffect,
  useState,
  useCallback,
  useMemo,
} from "react";
import {
  AuthRole,
  AuthTokens,
  AuthUser,
  buildLoginUrl,
  buildLogoutUrl,
  buildRegisterUrl,
  clearCodeVerifier,
  clearRedirectPath,
  clearRoleHint,
  clearTokens,
  decodeJwtClaims,
  extractUserFromClaims,
  getStoredTokens,
  refreshTokens,
  saveCodeVerifier,
  saveRedirectPath,
  saveRoleHint,
  saveTokens,
} from "@/lib/auth";
import { User } from "@/types/api";

// ==========================================
// Context Interface
// ==========================================

export interface AuthContextType {
  user: AuthUser | null;
  token: string | null;
  refreshToken: string | null;
  backendUser: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  isVerified: boolean;
  role: AuthRole;
  login: (options?: { redirectPath?: string; roleHint?: AuthRole }) => Promise<void>;
  register: (options?: { redirectPath?: string; roleHint?: AuthRole }) => Promise<void>;
  logout: (redirectPath?: string) => Promise<void>;
  getToken: () => Promise<string | null>;
  setSession: (tokens: AuthTokens, syncedUser?: User | null) => void;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

// ==========================================
// AuthProvider Component
// ==========================================

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null);
  const [token, setToken] = useState<string | null>(null);
  const [refreshToken, setRefreshToken] = useState<string | null>(null);
  const [backendUser, setBackendUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isVerified, setIsVerified] = useState<boolean>(false);
  const [role, setRole] = useState<AuthRole>(null);

  // Initialize session from localStorage on mount
  useEffect(() => {
    let isMounted = true;

    const initAuth = async () => {
      try {
        const stored = getStoredTokens();
        if (!stored || !stored.accessToken) {
          if (isMounted) {
            setIsLoading(false);
          }
          return;
        }

        // Check if token is expired or expires in < 30 seconds
        const isExpiringSoon =
          stored.expiresAt !== undefined &&
          Date.now() >= stored.expiresAt - 30000;

        let activeAccessToken = stored.accessToken;
        let activeRefreshToken = stored.refreshToken ?? null;

        if (isExpiringSoon && stored.refreshToken) {
          try {
            const refreshed = await refreshTokens(stored.refreshToken);
            saveTokens(refreshed);
            activeAccessToken = refreshed.accessToken;
            activeRefreshToken = refreshed.refreshToken ?? stored.refreshToken;
          } catch (refreshErr) {
            console.warn("Stored token expired and refresh failed:", refreshErr);
            clearTokens();
            if (isMounted) {
              setIsLoading(false);
            }
            return;
          }
        }

        const claims = decodeJwtClaims(activeAccessToken);
        if (claims && isMounted) {
          let u = extractUserFromClaims(claims);
          setToken(activeAccessToken);
          setRefreshToken(activeRefreshToken);

          // Rehydrate backend user profile from /api/v1/auth/me if accessible
          try {
            const apiBase = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";
            const res = await fetch(`${apiBase}/auth/me`, {
              headers: { Authorization: `Bearer ${activeAccessToken}` },
            });
            if (res.ok) {
              const json = await res.json();
              if (json?.data?.user) {
                setBackendUser(json.data.user);
                if (!u.role && json.data.user.role) {
                  u = { ...u, role: json.data.user.role as AuthRole };
                }
              }
            }
          } catch {
            // Non-fatal if API is temporarily unavailable
          }

          setUser(u);
          setRole(u.role);
          setIsVerified(u.isVerified);
        }
      } catch (err) {
        console.error("Error initializing auth state:", err);
      } finally {
        if (isMounted) {
          setIsLoading(false);
        }
      }
    };

    initAuth();

    return () => {
      isMounted = false;
    };
  }, []);

  // Update session directly (e.g. from callback page after token exchange & backend sync)
  const setSession = useCallback((tokens: AuthTokens, syncedUser?: User | null) => {
    saveTokens(tokens);
    setToken(tokens.accessToken);
    setRefreshToken(tokens.refreshToken ?? null);
    if (syncedUser !== undefined) {
      setBackendUser(syncedUser);
    }

    const claims = decodeJwtClaims(tokens.accessToken);
    if (claims) {
      let u = extractUserFromClaims(claims);
      if (!u.role && syncedUser?.role) {
        u = { ...u, role: syncedUser.role as AuthRole };
      }
      setUser(u);
      setRole(u.role);
      setIsVerified(u.isVerified);
    }
  }, []);

  // Retrieves valid access token, auto-refreshes if within 30s of expiry
  const getToken = useCallback(async (): Promise<string | null> => {
    const currentStored = getStoredTokens();
    if (!currentStored || !currentStored.accessToken) {
      return null;
    }

    let isExpiringSoon = false;
    if (currentStored.expiresAt !== undefined) {
      isExpiringSoon = Date.now() >= currentStored.expiresAt - 30000;
    } else {
      const claims = decodeJwtClaims(currentStored.accessToken);
      if (claims?.exp) {
        isExpiringSoon = Date.now() >= claims.exp * 1000 - 30000;
      }
    }

    if (!isExpiringSoon) {
      return currentStored.accessToken;
    }

    if (!currentStored.refreshToken) {
      return null;
    }

    try {
      const refreshed = await refreshTokens(currentStored.refreshToken);
      saveTokens(refreshed);
      setToken(refreshed.accessToken);
      setRefreshToken(refreshed.refreshToken ?? currentStored.refreshToken);

      const claims = decodeJwtClaims(refreshed.accessToken);
      if (claims) {
        const u = extractUserFromClaims(claims);
        setUser(u);
        setRole(u.role);
        setIsVerified(u.isVerified);
      }
      return refreshed.accessToken;
    } catch (err) {
      console.error("Failed to auto-refresh access token:", err);
      clearTokens();
      setToken(null);
      setRefreshToken(null);
      setUser(null);
      setRole(null);
      setIsVerified(false);
      setBackendUser(null);
      return null;
    }
  }, []);

  // Initiate PKCE authorization flow for login
  const login = useCallback(
    async (options?: { redirectPath?: string; roleHint?: AuthRole }) => {
      if (typeof window === "undefined") return;
      const redirectUri = `${window.location.origin}/callback`;

      if (options?.redirectPath) {
        saveRedirectPath(options.redirectPath);
      }
      if (options?.roleHint) {
        saveRoleHint(options.roleHint);
      }

      const { url, codeVerifier } = await buildLoginUrl(redirectUri, options?.roleHint);
      saveCodeVerifier(codeVerifier);
      window.location.href = url;
    },
    []
  );

  // Initiate PKCE authorization flow directly for registration
  const register = useCallback(
    async (options?: { redirectPath?: string; roleHint?: AuthRole }) => {
      if (typeof window === "undefined") return;
      const redirectUri = `${window.location.origin}/callback`;

      if (options?.redirectPath) {
        saveRedirectPath(options.redirectPath);
      }
      if (options?.roleHint) {
        saveRoleHint(options.roleHint);
      }

      const { url, codeVerifier } = await buildRegisterUrl(redirectUri, options?.roleHint);
      saveCodeVerifier(codeVerifier);
      window.location.href = url;
    },
    []
  );

  // Clear local session & redirect to Keycloak RP-initiated logout
  const logout = useCallback(async (redirectPath = "/") => {
    if (typeof window === "undefined") return;
    const stored = getStoredTokens();
    const idToken = stored?.idToken;

    clearTokens();
    clearCodeVerifier();
    clearRoleHint();
    clearRedirectPath();

    setToken(null);
    setRefreshToken(null);
    setUser(null);
    setRole(null);
    setIsVerified(false);
    setBackendUser(null);

    const postLogoutRedirectUri = `${window.location.origin}${redirectPath}`;
    const logoutUrl = buildLogoutUrl(postLogoutRedirectUri, idToken);
    window.location.href = logoutUrl;
  }, []);

  const value = useMemo<AuthContextType>(
    () => ({
      user,
      token,
      refreshToken,
      backendUser,
      isLoading,
      isAuthenticated: Boolean(user && token),
      isVerified,
      role,
      login,
      register,
      logout,
      getToken,
      setSession,
    }),
    [
      user,
      token,
      refreshToken,
      backendUser,
      isLoading,
      isVerified,
      role,
      login,
      register,
      logout,
      getToken,
      setSession,
    ]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

// ==========================================
// Custom Hook
// ==========================================

export function useAuth(): AuthContextType {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within an <AuthProvider>");
  }
  return context;
}
