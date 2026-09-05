import test from "node:test";
import assert from "node:assert/strict";
import { isEduEmail } from "./email-validation.ts";

test("isEduEmail accepts valid institutional .edu emails", () => {
  assert.equal(isEduEmail("student@stanford.edu"), true);
  assert.equal(isEduEmail("jordan.lee@cs.berkeley.edu"), true);
  assert.equal(isEduEmail("faculty@mit.edu"), true);
  assert.equal(isEduEmail("USER@HARVARD.EDU"), true);
  assert.equal(isEduEmail("grad.student@alumni.univ.edu"), true);
});

test("isEduEmail rejects non-.edu emails", () => {
  assert.equal(isEduEmail("user@gmail.com"), false);
  assert.equal(isEduEmail("user@yahoo.co.uk"), false);
  assert.equal(isEduEmail("hacker@edu.com"), false);
  assert.equal(isEduEmail("test@fake-edu.org"), false);
  assert.equal(isEduEmail("user@stanford.edu.attacker.com"), false);
  assert.equal(isEduEmail(""), false);
  assert.equal(isEduEmail("notanemail"), false);
  assert.equal(isEduEmail("@stanford.edu"), false);
  assert.equal(isEduEmail("student@"), false);
});
