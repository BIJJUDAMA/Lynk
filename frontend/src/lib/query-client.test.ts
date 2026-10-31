import test from "node:test";
import assert from "node:assert/strict";
import { createQueryClient } from "./query-client.ts";

test("createQueryClient configures 60s staleTime, 10m gcTime, retry 1, and refetchOnWindowFocus false", () => {
  const client = createQueryClient();
  const defaults = client.getDefaultOptions();
  assert.equal(defaults.queries?.staleTime, 60 * 1000);
  assert.equal(defaults.queries?.gcTime, 10 * 60 * 1000);
  assert.equal(defaults.queries?.retry, 1);
  assert.equal(defaults.queries?.refetchOnWindowFocus, false);
});
