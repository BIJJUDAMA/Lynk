import test from "node:test";
import assert from "node:assert/strict";
import { refreshSessionAfterEmailVerification } from "./verify-session.ts";

test("refreshSessionAfterEmailVerification invokes attemptRefreshingSession", async () => {
  let n = 0;
  await refreshSessionAfterEmailVerification({
    attemptRefreshingSession: async () => {
      n += 1;
      return true;
    },
  });
  assert.equal(n, 1);
});

test("refreshSessionAfterEmailVerification is a no-op when method absent", async () => {
  await refreshSessionAfterEmailVerification({});
  // should not throw
});
