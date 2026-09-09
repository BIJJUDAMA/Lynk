import test from "node:test";
import assert from "node:assert/strict";
import {
  generateRandomString,
  sha256,
  base64UrlEncode,
  generateCodeChallenge,
  decodeJwtClaims,
  extractUserFromClaims,
  syncUserWithBackend,
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
  // Sample JWT payload: {"sub":"user-123","email":"member@university.edu","email_verified":true,"roles":["member"]}
  const header = Buffer.from(JSON.stringify({ alg: "RS256", typ: "JWT" })).toString("base64url");
  const payload = Buffer.from(
    JSON.stringify({
      sub: "c82b0e9b-98f5-4148-bb83-c90a19c78271",
      email: "member@university.edu",
      email_verified: true,
      preferred_username: "alex.member",
      name: "Alex Rivera",
      roles: ["member"],
    })
  ).toString("base64url");
  const signature = "dummy_signature";
  const token = `${header}.${payload}.${signature}`;

  const claims = decodeJwtClaims(token);
  assert.ok(claims);
  assert.equal(claims.sub, "c82b0e9b-98f5-4148-bb83-c90a19c78271");
  assert.equal(claims.email, "member@university.edu");
  assert.equal(claims.email_verified, true);
  assert.equal(claims.preferred_username, "alex.member");
  assert.equal(claims.name, "Alex Rivera");

  // Malformed tokens
  assert.equal(decodeJwtClaims("not-a-token"), null);
  assert.equal(decodeJwtClaims("a.b"), null); // invalid json
});

test("extractUserFromClaims resolves member and admin roles and email verification", () => {
  // Verified Campus Member
  const memberClaims: JwtClaims = {
    sub: "user-member-1",
    email: "sarah@mit.edu",
    email_verified: true,
    name: "Sarah Chen",
    roles: ["member"],
  };

  const memberUser = extractUserFromClaims(memberClaims);
  assert.equal(memberUser.id, "user-member-1");
  assert.equal(memberUser.email, "sarah@mit.edu");
  assert.equal(memberUser.name, "Sarah Chen");
  assert.equal(memberUser.role, "member");
  assert.equal(memberUser.isVerified, true);

  // Unverified Campus Member
  const unverifiedClaims: JwtClaims = {
    sub: "user-member-2",
    email: "unverified@harvard.edu",
    email_verified: false,
    roles: ["member"],
  };

  const unverifiedUser = extractUserFromClaims(unverifiedClaims);
  assert.equal(unverifiedUser.role, "member");
  assert.equal(unverifiedUser.isVerified, false);

  // Admin user precedence over member role
  const adminClaims: JwtClaims = {
    sub: "user-admin-1",
    email: "admin@lynk.local",
    email_verified: true,
    roles: ["member", "admin"],
  };

  const adminUser = extractUserFromClaims(adminClaims);
  assert.equal(adminUser.role, "admin");
  assert.equal(adminUser.isVerified, true);
});

test("syncUserWithBackend posts profile payload without role field", async () => {
  const originalFetch = global.fetch;
  let capturedUrl = "";
  let capturedBody: unknown = null;
  let capturedHeaders: Record<string, string> = {};

  global.fetch = async (url: RequestInfo | URL, init?: RequestInit) => {
    capturedUrl = String(url);
    capturedBody = init?.body ? JSON.parse(String(init.body)) : null;
    capturedHeaders = (init?.headers as Record<string, string>) || {};
    return new Response(
      JSON.stringify({
        success: true,
        data: {
          id: "user-123",
          email: "student@stanford.edu",
          role: "member",
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        },
        error: null,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } }
    );
  };

  try {
    const res = await syncUserWithBackend("fake-token", {
      first_name: "Taylor",
      last_name: "Swift",
    });
    assert.equal(res.success, true);
    assert.ok(capturedUrl.endsWith("/auth/sync"));
    assert.equal(capturedHeaders.Authorization, "Bearer fake-token");
    assert.deepEqual(capturedBody, {
      first_name: "Taylor",
      last_name: "Swift",
    });
    // Ensure 'role' is never sent in the request body
    assert.equal((capturedBody as Record<string, unknown>).role, undefined);
  } finally {
    global.fetch = originalFetch;
  }
});


