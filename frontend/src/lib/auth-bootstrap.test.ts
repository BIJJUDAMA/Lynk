import test from "node:test";
import assert from "node:assert/strict";
import {
  emailVerifiedFromAccessPayload,
  shouldPostAuthSyncOnSessionRestore,
} from "./auth-bootstrap.ts";

test("session restore does not POST /auth/sync", () => {
  assert.equal(shouldPostAuthSyncOnSessionRestore(), false);
});

test("emailVerified claim is preferred", () => {
  assert.equal(emailVerifiedFromAccessPayload({ emailVerified: true }), true);
  assert.equal(emailVerifiedFromAccessPayload({ email_verified: false }), false);
  assert.equal(emailVerifiedFromAccessPayload({}), undefined);
});
