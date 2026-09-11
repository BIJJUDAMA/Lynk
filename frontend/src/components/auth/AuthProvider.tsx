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
  initSuperTokens,
  Session,
  EmailPassword,
  EmailVerification,
} from "@/lib/supertokens";
import { isEduEmail } from "@/lib/email-validation";
import { AuthRole, AuthTokens, AuthUser } from "@/lib/auth";
import { emailVerifiedFromAccessPayload } from "@/lib/auth-bootstrap";
import { DEFAULT_API_BASE_URL } from "@/lib/api";
import { Profile, User } from "@/types/api";

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
  login: (
    emailOrOptions?: string | { redirectPath?: string; roleHint?: AuthRole },
    password?: string
  ) => Promise<void>;
  register: (
    emailOrOptions?: string | { redirectPath?: string; roleHint?: AuthRole },
    password?: string
  ) => Promise<void>;
  logout: (redirectPath?: string) => Promise<void>;
  getToken: () => Promise<string | null>;
  setSession: (tokens?: AuthTokens, syncedUser?: User | null) => Promise<void>;
  resendVerificationEmail: () => Promise<void>;
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
  const syncGen = React.useRef(0);

  // Sync auth state from SuperTokens session and backend profile
  const syncAuthState = useCallback(async (): Promise<boolean> => {
    const gen = ++syncGen.current;
    try {
      initSuperTokens();
      const sessionExists = await Session.doesSessionExist();
      if (gen !== syncGen.current) return false;
      if (!sessionExists) {
        setUser(null);
        setToken(null);
        setRefreshToken(null);
        setBackendUser(null);
        setIsVerified(false);
        setRole(null);
        return false;
      }

      const userId = await Session.getUserId();
      const payload = await Session.getAccessTokenPayloadSecurely().catch(() => ({}));
      const accessToken = (await Session.getAccessToken().catch(() => null)) ?? null;
      if (gen !== syncGen.current) return false;
      setToken(accessToken);

      const claimed = emailVerifiedFromAccessPayload(payload as Record<string, unknown>);
      let emailVerified = claimed ?? false;
      if (claimed === undefined) {
        try {
          const isVerifiedRes = await EmailVerification.isEmailVerified();
          emailVerified = Boolean(isVerifiedRes?.isVerified);
        } catch (err) {
          console.warn("Failed to check email verification status:", err);
        }
      }
      if (gen !== syncGen.current) return false;

      // Fetch /api/v1/auth/me to sync and retrieve backend user profile
      let bUser: User | null = null;
      let bProfile: Profile | null = null;
      try {
        const apiBase = DEFAULT_API_BASE_URL;
        const headers: Record<string, string> = {
          Accept: "application/json",
        };
        if (accessToken) {
          headers["Authorization"] = `Bearer ${accessToken}`;
        }
        const res = await fetch(`${apiBase}/auth/me`, {
          credentials: "include",
          headers,
        });
        if (gen !== syncGen.current) return false;
        if (res.ok) {
          const json = await res.json();
          if (json?.data?.user) {
            bUser = json.data.user;
            if (gen !== syncGen.current) return false;
            setBackendUser(json.data.user);
          }
          if (json?.data?.profile) {
            bProfile = json.data.profile;
          }
          if (json?.data?.email_verified !== undefined) {
            emailVerified = emailVerified || Boolean(json.data.email_verified);
          }
        }
      } catch (err) {
        console.warn("Failed to fetch /api/v1/auth/me:", err);
      }

      const email: string = bUser?.email || payload?.email || "";
      const name: string =
        (bProfile && (bProfile.first_name || bProfile.last_name))
          ? `${bProfile.first_name || ""} ${bProfile.last_name || ""}`.trim()
          : payload?.name || (email ? email.split("@")[0] : "") || "Campus Member";

      const resolvedRole: AuthRole =
        (bUser?.role as AuthRole) ||
        (payload?.role as AuthRole) ||
        "member";

      const authUser: AuthUser = {
        id: userId,
        email,
        name,
        isVerified: emailVerified,
        role: resolvedRole,
        roles: resolvedRole ? [resolvedRole] : ["member"],
      };

      if (gen !== syncGen.current) return false;
      setUser(authUser);
      setIsVerified(emailVerified);
      setRole(resolvedRole);
      return true;
    } catch (err) {
      if (gen !== syncGen.current) return false;
      console.error("Error syncing auth state:", err);
      setUser(null);
      setToken(null);
      setRefreshToken(null);
      setBackendUser(null);
      setIsVerified(false);
      setRole(null);
      return false;
    }
  }, []);

  // Initialize SuperTokens on mount and inspect session
  useEffect(() => {
    let isMounted = true;

    const initAuth = async () => {
      try {
        initSuperTokens();
        if (isMounted) {
          await syncAuthState();
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
  }, [syncAuthState]);

  // Backward-compatible session sync helper
  const setSession = useCallback(
    async (tokens?: AuthTokens, syncedUser?: User | null) => {
      if (syncedUser !== undefined) {
        setBackendUser(syncedUser);
      }
      if (tokens?.accessToken) {
        setToken(tokens.accessToken);
        setRefreshToken(tokens.refreshToken ?? null);
      }
      await syncAuthState();
    },
    [syncAuthState]
  );

  // Retrieve valid access token directly from SuperTokens session
  const getToken = useCallback(async (): Promise<string | null> => {
    try {
      initSuperTokens();
      const exists = await Session.doesSessionExist();
      if (!exists) {
        return null;
      }
      const accessToken = await Session.getAccessToken();
      return accessToken ?? null;
    } catch (err) {
      console.error("Failed to retrieve SuperTokens access token:", err);
      return null;
    }
  }, []);

  // Sign in via SuperTokens EmailPassword recipe, or legacy redirect if no credentials provided
  const login = useCallback(
    async (
      emailOrOptions?: string | { redirectPath?: string; roleHint?: AuthRole },
      password?: string
    ) => {
      initSuperTokens();

      if (typeof emailOrOptions === "string" && typeof password === "string") {
        const email = emailOrOptions.trim();
        const res = await EmailPassword.signIn({
          formFields: [
            { id: "email", value: email },
            { id: "password", value: password },
          ],
        });

        if (res.status === "WRONG_CREDENTIALS_ERROR") {
          throw new Error("Invalid email or password");
        }

        if (res.status === "FIELD_ERROR") {
          const message =
            res.formFields.map((f) => f.error).join(", ") ||
            "Validation error during sign in";
          throw new Error(message);
        }

        if (res.status === "SIGN_IN_NOT_ALLOWED") {
          throw new Error(res.reason || "Sign in is not allowed at this time");
        }

        if (res.status === "OK") {
          await syncAuthState();
          return;
        }

        throw new Error("Sign in failed");
      }

      // Legacy / redirect call
      if (typeof window !== "undefined") {
        let redirectUrl = "/login";
        if (
          typeof emailOrOptions === "object" &&
          emailOrOptions !== null &&
          emailOrOptions.redirectPath
        ) {
          redirectUrl = `/login?redirect=${encodeURIComponent(emailOrOptions.redirectPath)}`;
        }
        window.location.href = redirectUrl;
      }
    },
    [syncAuthState]
  );

  // Register via SuperTokens EmailPassword recipe with strict .edu validation
  const register = useCallback(
    async (
      emailOrOptions?: string | { redirectPath?: string; roleHint?: AuthRole },
      password?: string
    ) => {
      initSuperTokens();

      if (typeof emailOrOptions === "string" && typeof password === "string") {
        const email = emailOrOptions.trim();

        if (!isEduEmail(email)) {
          throw new Error(
            "Registration rejected: only institutional .edu email addresses are permitted."
          );
        }

        const res = await EmailPassword.signUp({
          formFields: [
            { id: "email", value: email },
            { id: "password", value: password },
          ],
        });

        if (res.status === "FIELD_ERROR") {
          const message =
            res.formFields.map((f) => f.error).join(", ") ||
            "Validation error during registration";
          throw new Error(message);
        }

        if (res.status === "SIGN_UP_NOT_ALLOWED") {
          throw new Error(res.reason || "Sign up is not allowed at this time");
        }

        if (res.status === "OK") {
          try {
            await EmailVerification.sendVerificationEmail();
          } catch (err) {
            console.warn("Failed to send verification email upon sign up:", err);
          }

          await syncAuthState();
          setIsVerified(false);
          return;
        }

        throw new Error("Registration failed");
      }

      // Legacy / redirect call
      if (typeof window !== "undefined") {
        let redirectUrl = "/login";
        if (
          typeof emailOrOptions === "object" &&
          emailOrOptions !== null &&
          emailOrOptions.redirectPath
        ) {
          redirectUrl = `/login?redirect=${encodeURIComponent(emailOrOptions.redirectPath)}`;
        }
        window.location.href = redirectUrl;
      }
    },
    [syncAuthState]
  );

  // Resend email verification link to current user
  const resendVerificationEmail = useCallback(async () => {
    initSuperTokens();
    const res = await EmailVerification.sendVerificationEmail();
    if (res.status === "EMAIL_ALREADY_VERIFIED_ERROR") {
      setIsVerified(true);
      setUser((prev) => (prev ? { ...prev, isVerified: true } : null));
    }
  }, []);

  // Sign out of SuperTokens session & redirect
  const logout = useCallback(async (redirectPath = "/") => {
    syncGen.current++;
    initSuperTokens();
    try {
      await Session.signOut();
    } catch (err) {
      console.warn("Error signing out of SuperTokens session:", err);
    }

    setToken(null);
    setRefreshToken(null);
    setUser(null);
    setRole(null);
    setIsVerified(false);
    setBackendUser(null);

    if (typeof window !== "undefined") {
      window.location.href = redirectPath || "/login";
    }
  }, []);

  const value = useMemo<AuthContextType>(
    () => ({
      user,
      token,
      refreshToken,
      backendUser,
      isLoading,
      isAuthenticated: Boolean(user),
      isVerified,
      role,
      login,
      register,
      logout,
      getToken,
      setSession,
      resendVerificationEmail,
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
      resendVerificationEmail,
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
