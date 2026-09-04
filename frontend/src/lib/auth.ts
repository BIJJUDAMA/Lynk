import type { ApiResponse, User, UserRole } from "../types/api";

// ==========================================
// Configuration & Constants
// ==========================================

export const KEYCLOAK_CONFIG = {
  url: process.env.NEXT_PUBLIC_KEYCLOAK_URL || "http://localhost:8081",
  realm: process.env.NEXT_PUBLIC_KEYCLOAK_REALM || "lynk",
  clientId: process.env.NEXT_PUBLIC_KEYCLOAK_CLIENT_ID || "lynk-frontend",
  scope: "openid profile email roles",
};

export const API_CONFIG = {
  baseUrl: process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1",
};

export const STORAGE_KEYS = {
  tokens: "lynk_auth_tokens",
  codeVerifier: "lynk_pkce_verifier",
  roleHint: "lynk_auth_role_hint",
  redirectPath: "lynk_auth_redirect_path",
} as const;

// ==========================================
// Types
// ==========================================

export type AuthRole = "student" | "employer" | "admin" | null;

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
  realm_access?: {
    roles?: string[];
  };
  resource_access?: Record<string, { roles?: string[] }>;
  exp?: number;
  iat?: number;
  [key: string]: unknown;
}

export interface AuthUser {
  id: string; // Keycloak UUID (sub)
  email: string;
  name: string;
  preferredUsername?: string;
  isVerified: boolean;
  role: AuthRole;
  roles: string[];
}

// ==========================================
// PKCE Cryptographic Helpers
// ==========================================

/**
 * Generates a high-entropy cryptographically random string for PKCE code_verifier.
 */
export function generateRandomString(length = 64): string {
  const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~";
  let result = "";

  if (typeof window !== "undefined" && window.crypto && window.crypto.getRandomValues) {
    const values = new Uint8Array(length);
    window.crypto.getRandomValues(values);
    for (let i = 0; i < length; i++) {
      result += charset[values[i] % charset.length];
    }
  } else {
    // Fallback for SSR or non-browser test environments
    for (let i = 0; i < length; i++) {
      result += charset[Math.floor(Math.random() * charset.length)];
    }
  }

  return result;
}

/**
 * Computes SHA-256 hash of a plain text string.
 */
export async function sha256(plain: string): Promise<ArrayBuffer> {
  const encoder = new TextEncoder();
  const data = encoder.encode(plain);

  if (typeof window !== "undefined" && window.crypto && window.crypto.subtle) {
    return await window.crypto.subtle.digest("SHA-256", data);
  }

  // Fallback for Node.js / SSR execution
  const nodeCrypto = await import("crypto");
  return nodeCrypto.createHash("sha256").update(data).digest().buffer;
}

/**
 * Encodes an ArrayBuffer or Uint8Array into URL-safe base64 string without padding.
 */
export function base64UrlEncode(buffer: ArrayBuffer | Uint8Array): string {
  const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer);
  let binary = "";
  for (let i = 0; i < bytes.byteLength; i++) {
    binary += String.fromCharCode(bytes[i]);
  }

  let base64 = "";
  if (typeof btoa !== "undefined") {
    base64 = btoa(binary);
  } else {
    base64 = Buffer.from(binary, "binary").toString("base64");
  }

  return base64.replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}

/**
 * Generates an S256 PKCE code challenge from a code verifier.
 */
export async function generateCodeChallenge(verifier: string): Promise<string> {
  const hashed = await sha256(verifier);
  return base64UrlEncode(hashed);
}

// ==========================================
// OIDC URL Builders
// ==========================================

/**
 * Constructs the Keycloak authorization URL with PKCE parameters.
 */
export async function buildLoginUrl(
  redirectUri: string,
  roleHint?: AuthRole
): Promise<{ url: string; codeVerifier: string }> {
  const codeVerifier = generateRandomString(64);
  const codeChallenge = await generateCodeChallenge(codeVerifier);

  const authUrl = new URL(
    `${KEYCLOAK_CONFIG.url}/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/auth`
  );

  authUrl.searchParams.set("client_id", KEYCLOAK_CONFIG.clientId);
  authUrl.searchParams.set("response_type", "code");
  authUrl.searchParams.set("scope", KEYCLOAK_CONFIG.scope);
  authUrl.searchParams.set("redirect_uri", redirectUri);
  authUrl.searchParams.set("code_challenge", codeChallenge);
  authUrl.searchParams.set("code_challenge_method", "S256");

  if (roleHint) {
    saveRoleHint(roleHint);
  }

  return { url: authUrl.toString(), codeVerifier };
}

/**
 * Constructs Keycloak registration URL with PKCE parameters.
 */
export async function buildRegisterUrl(
  redirectUri: string,
  roleHint?: AuthRole
): Promise<{ url: string; codeVerifier: string }> {
  const codeVerifier = generateRandomString(64);
  const codeChallenge = await generateCodeChallenge(codeVerifier);

  const registerUrl = new URL(
    `${KEYCLOAK_CONFIG.url}/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/registrations`
  );

  registerUrl.searchParams.set("client_id", KEYCLOAK_CONFIG.clientId);
  registerUrl.searchParams.set("response_type", "code");
  registerUrl.searchParams.set("scope", KEYCLOAK_CONFIG.scope);
  registerUrl.searchParams.set("redirect_uri", redirectUri);
  registerUrl.searchParams.set("code_challenge", codeChallenge);
  registerUrl.searchParams.set("code_challenge_method", "S256");

  if (roleHint) {
    saveRoleHint(roleHint);
  }

  return { url: registerUrl.toString(), codeVerifier };
}

/**
 * Constructs Keycloak RP-initiated logout URL.
 */
export function buildLogoutUrl(redirectUri: string, idToken?: string): string {
  const logoutUrl = new URL(
    `${KEYCLOAK_CONFIG.url}/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/logout`
  );

  logoutUrl.searchParams.set("client_id", KEYCLOAK_CONFIG.clientId);
  logoutUrl.searchParams.set("post_logout_redirect_uri", redirectUri);
  if (idToken) {
    logoutUrl.searchParams.set("id_token_hint", idToken);
  }

  return logoutUrl.toString();
}

// ==========================================
// Token Exchange & Refresh
// ==========================================

/**
 * Exchanges authorization code and PKCE code_verifier for access and refresh tokens.
 */
export async function exchangeCodeForTokens(
  code: string,
  codeVerifier: string,
  redirectUri: string
): Promise<AuthTokens> {
  const tokenUrl = `${KEYCLOAK_CONFIG.url}/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/token`;

  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: KEYCLOAK_CONFIG.clientId,
    code,
    code_verifier: codeVerifier,
    redirect_uri: redirectUri,
  });

  const response = await fetch(tokenUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: body.toString(),
  });

  if (!response.ok) {
    const errorText = await response.text();
    let errorMsg = `Token exchange failed with HTTP ${response.status}`;
    try {
      const errJson = JSON.parse(errorText);
      errorMsg = errJson.error_description || errJson.error || errorMsg;
    } catch {
      // Keep fallback errorMsg
    }
    throw new Error(errorMsg);
  }

  const data = await response.json();
  const expiresIn = data.expires_in ?? 300;

  return {
    accessToken: data.access_token,
    refreshToken: data.refresh_token,
    idToken: data.id_token,
    tokenType: data.token_type,
    expiresIn,
    expiresAt: Date.now() + expiresIn * 1000,
    scope: data.scope,
  };
}

/**
 * Refreshes an expired or expiring access token using the refresh token.
 */
export async function refreshTokens(refreshToken: string): Promise<AuthTokens> {
  const tokenUrl = `${KEYCLOAK_CONFIG.url}/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/token`;

  const body = new URLSearchParams({
    grant_type: "refresh_token",
    client_id: KEYCLOAK_CONFIG.clientId,
    refresh_token: refreshToken,
  });

  const response = await fetch(tokenUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body: body.toString(),
  });

  if (!response.ok) {
    const errorText = await response.text();
    let errorMsg = `Token refresh failed with HTTP ${response.status}`;
    try {
      const errJson = JSON.parse(errorText);
      errorMsg = errJson.error_description || errJson.error || errorMsg;
    } catch {
      // Keep fallback errorMsg
    }
    throw new Error(errorMsg);
  }

  const data = await response.json();
  const expiresIn = data.expires_in ?? 300;

  return {
    accessToken: data.access_token,
    refreshToken: data.refresh_token || refreshToken,
    idToken: data.id_token,
    tokenType: data.token_type,
    expiresIn,
    expiresAt: Date.now() + expiresIn * 1000,
    scope: data.scope,
  };
}

// ==========================================
// JWT Decoding & User Extraction
// ==========================================

/**
 * Decodes the payload of a JWT token without verifying cryptographic signature (signature validated by backend via JWKS).
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
  const roles = claims.realm_access?.roles ?? [];
  let role: AuthRole = null;

  if (roles.includes("admin")) {
    role = "admin";
  } else if (roles.includes("employer")) {
    role = "employer";
  } else if (roles.includes("student")) {
    role = "student";
  }

  const displayName =
    claims.name ||
    claims.preferred_username ||
    (claims.email ? claims.email.split("@")[0] : "User");

  return {
    id: claims.sub,
    email: claims.email ?? "",
    name: displayName,
    preferredUsername: claims.preferred_username,
    isVerified: Boolean(claims.email_verified),
    role,
    roles,
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

export function saveCodeVerifier(verifier: string): void {
  if (typeof window === "undefined") return;
  sessionStorage.setItem(STORAGE_KEYS.codeVerifier, verifier);
}

export function getStoredCodeVerifier(): string | null {
  if (typeof window === "undefined") return null;
  return sessionStorage.getItem(STORAGE_KEYS.codeVerifier);
}

export function clearCodeVerifier(): void {
  if (typeof window === "undefined") return;
  sessionStorage.removeItem(STORAGE_KEYS.codeVerifier);
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
 * Synchronizes the Keycloak authenticated user with the Go backend PostgreSQL database.
 * Calls POST /api/v1/auth/sync with Bearer token.
 */
export async function syncUserWithBackend(
  accessToken: string,
  roleHint?: string
): Promise<ApiResponse<User>> {
  const syncUrl = `${API_CONFIG.baseUrl}/auth/sync`;
  const body = roleHint ? JSON.stringify({ role: roleHint }) : undefined;

  const response = await fetch(syncUrl, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      Authorization: `Bearer ${accessToken}`,
    },
    body,
  });

  const data = (await response.json()) as ApiResponse<User>;
  return data;
}
