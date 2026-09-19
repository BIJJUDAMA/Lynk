import test from "node:test";
import assert from "node:assert/strict";
import {
  refreshSessionAfterEmailVerification,
  syncStaleEmailVerification,
} from "./verify-session.ts";

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

test("syncStaleEmailVerification skips server check when claim is already verified", async () => {
  let serverChecked = false;
  let refreshCalled = false;

  const result = await syncStaleEmailVerification(
    true,
    async () => {
      serverChecked = true;
      return { isVerified: true };
    },
    async () => {
      refreshCalled = true;
    }
  );

  assert.equal(result, true);
  assert.equal(serverChecked, false);
  assert.equal(refreshCalled, false);
});

test("syncStaleEmailVerification updates and refreshes session when server confirms verification", async () => {
  let serverChecked = false;
  let refreshCalled = false;

  const result = await syncStaleEmailVerification(
    false,
    async () => {
      serverChecked = true;
      return { isVerified: true };
    },
    async () => {
      refreshCalled = true;
      return true;
    }
  );

  assert.equal(result, true);
  assert.equal(serverChecked, true);
  assert.equal(refreshCalled, true);
});

test("syncStaleEmailVerification remains false when server confirms unverified", async () => {
  let serverChecked = false;
  let refreshCalled = false;

  const result = await syncStaleEmailVerification(
    false,
    async () => {
      serverChecked = true;
      return { isVerified: false };
    },
    async () => {
      refreshCalled = true;
      return true;
    }
  );

  assert.equal(result, false);
  assert.equal(serverChecked, true);
  assert.equal(refreshCalled, false);
});

test("syncStaleEmailVerification handles server check errors gracefully", async () => {
  let refreshCalled = false;

  const result = await syncStaleEmailVerification(
    false,
    async () => {
      throw new Error("network error");
    },
    async () => {
      refreshCalled = true;
      return true;
    }
  );

  assert.equal(result, false);
  assert.equal(refreshCalled, false);
});
