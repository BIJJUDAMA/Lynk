import test from "node:test";
import assert from "node:assert/strict";
import {
  generateRandomString,
  sha256,
  base64UrlEncode,
  generateCodeChallenge,
  decodeJwtClaims,
  extractUserFromClaims,
  buildLoginUrl,
  buildRegisterUrl,
  buildLogoutUrl,
  KEYCLOAK_CONFIG,
} from "./auth.ts";
import type { JwtClaims } from "./auth.ts";

test("generateRandomString produces expected length and valid characters", () => {
  const str = generateRandomString(64);
  assert.equal(str.length, 64);
  assert.match(str, /^[A-Za-z0-9\-._~]+$/);

  const customStr = generateRandomString(128);
  assert.equal(customStr.length, 128);
  assert.match(customStr, /^[A-Za-z0-9\-._~]+$/);
});

test("RFC 7636 PKCE S256 test vector validation", async () => {
  // Appendix B of RFC 7636
  const rfcVerifier = "dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk";
  const expectedChallenge = "E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM";

  const challenge = await generateCodeChallenge(rfcVerifier);
  assert.equal(challenge, expectedChallenge);
});

test("decodeJwtClaims decodes valid JWT payload and handles malformed strings", () => {
  // Sample JWT payload: {"sub":"user-123","email":"student@university.edu","email_verified":true,"realm_access":{"roles":["student"]}}
  const header = Buffer.from(JSON.stringify({ alg: "RS256", typ: "JWT" })).toString("base64url");
  const payload = Buffer.from(
    JSON.stringify({
      sub: "c82b0e9b-98f5-4148-bb83-c90a19c78271",
      email: "student@university.edu",
      email_verified: true,
      preferred_username: "alex.student",
      name: "Alex Rivera",
      realm_access: {
        roles: ["student", "offline_access", "uma_authorization"],
      },
    })
  ).toString("base64url");
  const signature = "dummy_signature";
  const token = `${header}.${payload}.${signature}`;

  const claims = decodeJwtClaims(token);
  assert.ok(claims);
  assert.equal(claims.sub, "c82b0e9b-98f5-4148-bb83-c90a19c78271");
  assert.equal(claims.email, "student@university.edu");
  assert.equal(claims.email_verified, true);
  assert.equal(claims.preferred_username, "alex.student");
  assert.equal(claims.name, "Alex Rivera");

  // Malformed tokens
  assert.equal(decodeJwtClaims("not-a-token"), null);
  assert.equal(decodeJwtClaims("a.b"), null); // invalid json
});

test("extractUserFromClaims resolves roles and institutional email verification gate", () => {
  // Student with verified .edu email
  const studentClaims: JwtClaims = {
    sub: "user-student-1",
    email: "sarah@mit.edu",
    email_verified: true,
    name: "Sarah Chen",
    realm_access: {
      roles: ["student", "default-roles-lynk"],
    },
  };

  const studentUser = extractUserFromClaims(studentClaims);
  assert.equal(studentUser.id, "user-student-1");
  assert.equal(studentUser.email, "sarah@mit.edu");
  assert.equal(studentUser.name, "Sarah Chen");
  assert.equal(studentUser.role, "student");
  assert.equal(studentUser.isVerified, true);

  // Student with UNVERIFIED email (Institutional Email Gate trigger)
  const unverifiedClaims: JwtClaims = {
    sub: "user-student-2",
    email: "unverified@harvard.edu",
    email_verified: false,
    realm_access: {
      roles: ["student"],
    },
  };

  const unverifiedUser = extractUserFromClaims(unverifiedClaims);
  assert.equal(unverifiedUser.role, "student");
  assert.equal(unverifiedUser.isVerified, false);

  // Employer user
  const employerClaims: JwtClaims = {
    sub: "user-employer-1",
    email: "recruiter@techcorp.com",
    email_verified: true,
    realm_access: {
      roles: ["employer"],
    },
  };

  const employerUser = extractUserFromClaims(employerClaims);
  assert.equal(employerUser.role, "employer");
  assert.equal(employerUser.isVerified, true);

  // Admin user precedence over other roles
  const adminClaims: JwtClaims = {
    sub: "user-admin-1",
    email: "admin@lynk.local",
    email_verified: true,
    realm_access: {
      roles: ["student", "employer", "admin"],
    },
  };

  const adminUser = extractUserFromClaims(adminClaims);
  assert.equal(adminUser.role, "admin");
});

test("buildLogoutUrl creates correct Keycloak RP-initiated logout URL", () => {
  const redirectUri = "http://localhost:3000/";
  const idToken = "sample-id-token-123";

  const url = buildLogoutUrl(redirectUri, idToken);
  const parsed = new URL(url);

  assert.equal(parsed.origin, KEYCLOAK_CONFIG.url);
  assert.equal(
    parsed.pathname,
    `/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/logout`
  );
  assert.equal(parsed.searchParams.get("client_id"), KEYCLOAK_CONFIG.clientId);
  assert.equal(parsed.searchParams.get("post_logout_redirect_uri"), redirectUri);
  assert.equal(parsed.searchParams.get("id_token_hint"), idToken);
});

test("buildLoginUrl and buildRegisterUrl generate valid PKCE authorization URLs", async () => {
  const redirectUri = "http://localhost:3000/callback";

  // Login URL
  const loginResult = await buildLoginUrl(redirectUri, "student");
  assert.ok(loginResult.codeVerifier);
  assert.equal(loginResult.codeVerifier.length, 64);

  const loginParsed = new URL(loginResult.url);
  assert.equal(loginParsed.origin, KEYCLOAK_CONFIG.url);
  assert.equal(
    loginParsed.pathname,
    `/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/auth`
  );
  assert.equal(loginParsed.searchParams.get("client_id"), KEYCLOAK_CONFIG.clientId);
  assert.equal(loginParsed.searchParams.get("response_type"), "code");
  assert.equal(loginParsed.searchParams.get("scope"), KEYCLOAK_CONFIG.scope);
  assert.equal(loginParsed.searchParams.get("redirect_uri"), redirectUri);
  assert.equal(loginParsed.searchParams.get("code_challenge_method"), "S256");
  assert.ok(loginParsed.searchParams.get("code_challenge"));

  // Register URL
  const registerResult = await buildRegisterUrl(redirectUri, "employer");
  assert.ok(registerResult.codeVerifier);

  const registerParsed = new URL(registerResult.url);
  assert.equal(registerParsed.origin, KEYCLOAK_CONFIG.url);
  assert.equal(
    registerParsed.pathname,
    `/realms/${KEYCLOAK_CONFIG.realm}/protocol/openid-connect/registrations`
  );
  assert.equal(registerParsed.searchParams.get("client_id"), KEYCLOAK_CONFIG.clientId);
  assert.equal(registerParsed.searchParams.get("response_type"), "code");
  assert.equal(registerParsed.searchParams.get("scope"), KEYCLOAK_CONFIG.scope);
  assert.equal(registerParsed.searchParams.get("redirect_uri"), redirectUri);
  assert.equal(registerParsed.searchParams.get("code_challenge_method"), "S256");
  assert.ok(registerParsed.searchParams.get("code_challenge"));
});

