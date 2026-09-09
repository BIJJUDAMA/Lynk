import type { ApiResponse, SyncUserRequest, User, UserRole } from "../types/api";

// ==========================================
// Configuration & Constants
// ==========================================

const rawApiUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/+$/, "");
export const API_CONFIG = {
  baseUrl: rawApiUrl.endsWith("/api/v1") ? rawApiUrl : `${rawApiUrl}/api/v1`,
};

export const STORAGE_KEYS = {
  tokens: "lynk_auth_tokens",
  roleHint: "lynk_auth_role_hint",
  redirectPath: "lynk_auth_redirect_path",
} as const;

// ==========================================
// Types
// ==========================================

export type AuthRole = "member" | "admin" | null;

export interface AuthTokens {
  accessToken: string;
  refreshToken?: string;
  idToken?: string;
  tokenType?: string;
  expiresIn?: number;
  expiresAt?: number; // epoch ms
  scope?: string;
}

export interface JwtClaims {
  sub: string;
  email?: string;
  email_verified?: boolean;
  preferred_username?: string;
  name?: string;
  given_name?: string;
  family_name?: string;
  roles?: string[];
  realm_access?: {
    roles?: string[];
  };
  resource_access?: Record<string, { roles?: string[] }>;
  exp?: number;
  iat?: number;
  [key: string]: unknown;
}

export interface AuthUser {
  id: string; // User ID / sub
  email: string;
  name: string;
  preferredUsername?: string;
  isVerified: boolean;
  role: AuthRole;
  roles: string[];
}

// ==========================================
// JWT Decoding & User Extraction
// ==========================================

/**
 * Decodes the payload of a SuperTokens access token without verifying the signature.
 * Verification is performed by SuperTokens Core / Go session middleware — not JWKS/Keycloak.
 */
export function decodeJwtClaims(token: string): JwtClaims | null {
  try {
    const parts = token.split(".");
    if (parts.length < 2) return null;

    const base64Url = parts[1];
    const base64 = base64Url.replace(/-/g, "+").replace(/_/g, "/");
    const jsonPayload = decodeURIComponent(
      (typeof atob !== "undefined"
        ? atob(base64)
        : Buffer.from(base64, "base64").toString("binary")
      )
        .split("")
        .map((c) => "%" + ("00" + c.charCodeAt(0).toString(16)).slice(-2))
        .join("")
    );

    return JSON.parse(jsonPayload) as JwtClaims;
  } catch {
    return null;
  }
}

/**
 * Extracts a normalized AuthUser model from decoded JWT claims.
 */
export function extractUserFromClaims(claims: JwtClaims): AuthUser {
  const rawRoles = [
    ...(claims.roles ?? []),
    ...(claims.realm_access?.roles ?? []),
  ];
  let role: AuthRole = "member";

  if (rawRoles.includes("admin")) {
    role = "admin";
  } else if (rawRoles.includes("member")) {
    role = "member";
  }

  const displayName =
    claims.name ||
    claims.preferred_username ||
    (claims.email ? claims.email.split("@")[0] : "Campus Member");

  return {
    id: claims.sub,
    email: claims.email ?? "",
    name: displayName,
    preferredUsername: claims.preferred_username,
    isVerified: Boolean(claims.email_verified),
    role,
    roles: rawRoles,
  };
}

// ==========================================
// Token & Session Storage
// ==========================================

export function saveTokens(tokens: AuthTokens): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.setItem(STORAGE_KEYS.tokens, JSON.stringify(tokens));
  } catch (err) {
    console.error("Failed to save tokens to localStorage:", err);
  }
}

export function getStoredTokens(): AuthTokens | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = localStorage.getItem(STORAGE_KEYS.tokens);
    return raw ? (JSON.parse(raw) as AuthTokens) : null;
  } catch {
    return null;
  }
}

export function clearTokens(): void {
  if (typeof window === "undefined") return;
  try {
    localStorage.removeItem(STORAGE_KEYS.tokens);
  } catch (err) {
    console.error("Failed to remove tokens from localStorage:", err);
  }
}

export function saveRoleHint(role: string): void {
  if (typeof window === "undefined") return;
  sessionStorage.setItem(STORAGE_KEYS.roleHint, role);
}

export function getStoredRoleHint(): string | null {
  if (typeof window === "undefined") return null;
  return sessionStorage.getItem(STORAGE_KEYS.roleHint);
}

export function clearRoleHint(): void {
  if (typeof window === "undefined") return;
  sessionStorage.removeItem(STORAGE_KEYS.roleHint);
}

export function saveRedirectPath(path: string): void {
  if (typeof window === "undefined") return;
  sessionStorage.setItem(STORAGE_KEYS.redirectPath, path);
}

export function getStoredRedirectPath(): string | null {
  if (typeof window === "undefined") return null;
  return sessionStorage.getItem(STORAGE_KEYS.redirectPath);
}

export function clearRedirectPath(): void {
  if (typeof window === "undefined") return;
  sessionStorage.removeItem(STORAGE_KEYS.redirectPath);
}

// ==========================================
// Go Backend Synchronization
// ==========================================

/**
 * Synchronizes the SuperTokens authenticated user with the Go backend PostgreSQL database.
 * Calls POST /api/v1/auth/sync with Bearer token.
 */
export async function syncUserWithBackend(
  accessToken: string,
  profileData?: SyncUserRequest
): Promise<ApiResponse<User>> {
  const syncUrl = `${API_CONFIG.baseUrl}/auth/sync`;
  const body = profileData ? JSON.stringify(profileData) : JSON.stringify({});

  const response = await fetch(syncUrl, {
    method: "POST",
    credentials: "include",
    headers: {
      "Content-Type": "application/json",
      Accept: "application/json",
      ...(accessToken ? { Authorization: `Bearer ${accessToken}` } : {}),
    },
    body,
  });

  const data = (await response.json()) as ApiResponse<User>;
  return data;
}
