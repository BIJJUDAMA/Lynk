import test from "node:test";
import assert from "node:assert/strict";
import { createDebounced, syncSearchDraftFromParent } from "./job-filters.ts";

test("createDebounced collapses rapid calls to the last invocation", async () => {
  const calls: string[] = [];
  const d = createDebounced((v: string) => {
    calls.push(v);
  }, 30);
  d("a");
  d("ab");
  d("abc");
  await new Promise((r) => setTimeout(r, 80));
  assert.deepEqual(calls, ["abc"]);
});

test("createDebounced cancel prevents a pending trailing call", async () => {
  const calls: string[] = [];
  const d = createDebounced((v: string) => {
    calls.push(v);
  }, 30);
  d("abc");
  d.cancel();
  await new Promise((r) => setTimeout(r, 80));
  assert.deepEqual(calls, []);
});

test("syncSearchDraftFromParent cancels when parent search becomes empty while draft is pending", () => {
  assert.deepEqual(syncSearchDraftFromParent("", "react"), {
    cancelPending: true,
    draft: "",
  });
  assert.deepEqual(syncSearchDraftFromParent("", ""), {
    cancelPending: false,
    draft: "",
  });
  assert.deepEqual(syncSearchDraftFromParent("react", "react intern"), {
    cancelPending: false,
    draft: "react",
  });
});
