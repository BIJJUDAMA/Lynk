import { test } from "node:test";
import assert from "node:assert/strict";
import {
  formatJobDate,
  getStatusBadgeClasses,
  COMMON_DEPARTMENTS,
  POPULAR_SKILLS,
  formatFileSize,
  getApplicationStatusBadgeClasses,
  getContractStatusBadgeClasses,
  isValidUrl,
} from "./formatters.ts";

test("formatJobDate formats ISO date strings and handles null/undefined", () => {
  assert.equal(formatJobDate(null), "Flexible");
  assert.equal(formatJobDate(undefined), "Flexible");
  assert.equal(formatJobDate(""), "Flexible");

  const formatted = formatJobDate("2026-10-15T00:00:00Z");
  assert.ok(
    formatted.includes("Oct 15, 2026") ||
      formatted.includes("10/15/2026") ||
      formatted.includes("2026")
  );
});

test("getStatusBadgeClasses returns distinctive style tokens for all job statuses", () => {
  const openStyles = getStatusBadgeClasses("open");
  assert.ok(openStyles.bg.includes("emerald"));
  assert.ok(openStyles.text.includes("emerald"));

  const inProgressStyles = getStatusBadgeClasses("in_progress");
  assert.ok(inProgressStyles.bg.includes("sky"));

  const closedStyles = getStatusBadgeClasses("closed");
  assert.ok(closedStyles.bg.includes("slate"));

  const cancelledStyles = getStatusBadgeClasses("cancelled");
  assert.ok(cancelledStyles.bg.includes("rose"));
});

test("COMMON_DEPARTMENTS and POPULAR_SKILLS contain academic presets", () => {
  assert.ok(COMMON_DEPARTMENTS.length >= 5);
  assert.ok(COMMON_DEPARTMENTS.includes("Computer Science & Engineering"));
  assert.ok(COMMON_DEPARTMENTS.includes("Design & Creative Arts"));

  assert.ok(POPULAR_SKILLS.length >= 5);
  assert.ok(POPULAR_SKILLS.includes("React"));
  assert.ok(POPULAR_SKILLS.includes("Python"));
});

test("formatFileSize formats bytes to human-readable strings", () => {
  assert.equal(formatFileSize(0), "0 KB");
  assert.equal(formatFileSize(null), "0 KB");
  assert.equal(formatFileSize(500), "500 B");
  assert.equal(formatFileSize(1024), "1.0 KB");
  assert.equal(formatFileSize(1024 * 512), "512.0 KB");
  assert.equal(formatFileSize(1024 * 1024 * 2.5), "2.50 MB");
});

test("getApplicationStatusBadgeClasses returns styling for all application statuses", () => {
  const pending = getApplicationStatusBadgeClasses("pending");
  assert.ok(pending.bg.includes("amber"));
  assert.equal(pending.label, "Pending Review");

  const accepted = getApplicationStatusBadgeClasses("accepted");
  assert.ok(accepted.bg.includes("emerald"));
  assert.equal(accepted.label, "Accepted");

  const rejected = getApplicationStatusBadgeClasses("rejected");
  assert.ok(rejected.bg.includes("rose"));
  assert.equal(rejected.label, "Not Selected");
});

test("isValidUrl validates web URLs with http/https protocols", () => {
  assert.equal(isValidUrl("https://github.com/student"), true);
  assert.equal(isValidUrl("http://portfolio.dev"), true);
  assert.equal(isValidUrl("ftp://file.server"), false);
  assert.equal(isValidUrl("javascript:alert(1)"), false);
  assert.equal(isValidUrl("data:text/html,test"), false);
  assert.equal(isValidUrl("file:///etc/passwd"), false);
  assert.equal(isValidUrl("invalid-url"), false);
  assert.equal(isValidUrl(""), false);
});

test("getContractStatusBadgeClasses returns styling for all contract statuses", () => {
  const draft = getContractStatusBadgeClasses("draft");
  assert.ok(draft.bg.includes("slate"));
  assert.equal(draft.label, "Draft");

  const active = getContractStatusBadgeClasses("active");
  assert.ok(active.bg.includes("blue"));
  assert.equal(active.label, "Active");

  const completed = getContractStatusBadgeClasses("completed");
  assert.ok(completed.bg.includes("emerald"));
  assert.equal(completed.label, "Completed");

  const cancelled = getContractStatusBadgeClasses("cancelled");
  assert.ok(cancelled.bg.includes("rose"));
  assert.equal(cancelled.label, "Cancelled");
});

