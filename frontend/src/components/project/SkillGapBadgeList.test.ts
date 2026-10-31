import test from "node:test";
import assert from "node:assert";
import { SkillGapBadgeList } from "./SkillGapBadgeList.js";

test("SkillGapBadgeList component exported correctly", () => {
  assert.strictEqual(typeof SkillGapBadgeList, "function");
});
