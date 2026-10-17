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
  formatProjectDate,
  getRecruitmentStatusBadgeClasses,
  getLifecycleBadgeClasses,
  PROJECT_CATEGORIES,
  slugifyProjectTitle,
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
  assert.ok(openStyles.bg.includes("pastel-green"));
  assert.ok(openStyles.text.includes("pastel-greenText"));

  const inProgressStyles = getStatusBadgeClasses("in_progress");
  assert.ok(inProgressStyles.bg.includes("pastel-blue"));

  const closedStyles = getStatusBadgeClasses("closed");
  assert.ok(closedStyles.bg.includes("muted"));

  const cancelledStyles = getStatusBadgeClasses("cancelled");
  assert.ok(cancelledStyles.bg.includes("pastel-red"));
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
  assert.ok(pending.bg.includes("pastel-yellow"));
  assert.equal(pending.label, "Pending Review");

  const accepted = getApplicationStatusBadgeClasses("accepted");
  assert.ok(accepted.bg.includes("pastel-green"));
  assert.equal(accepted.label, "Accepted");

  const rejected = getApplicationStatusBadgeClasses("rejected");
  assert.ok(rejected.bg.includes("pastel-red"));
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
  assert.ok(draft.bg.includes("muted"));
  assert.equal(draft.label, "Draft");

  const active = getContractStatusBadgeClasses("active");
  assert.ok(active.bg.includes("pastel-blue"));
  assert.equal(active.label, "Active");

  const completed = getContractStatusBadgeClasses("completed");
  assert.ok(completed.bg.includes("pastel-green"));
  assert.equal(completed.label, "Completed");

  const cancelled = getContractStatusBadgeClasses("cancelled");
  assert.ok(cancelled.bg.includes("pastel-red"));
  assert.equal(cancelled.label, "Cancelled");
});

test("formatProjectDate formats timestamps to human-readable strings", () => {
  assert.equal(formatProjectDate(null), "Updated recently");
  assert.equal(formatProjectDate(undefined, "Created"), "Created recently");

  const nowIso = new Date().toISOString();
  assert.equal(formatProjectDate(nowIso), "Updated just now");

  const tenDaysAgo = new Date(Date.now() - 10 * 24 * 60 * 60 * 1000).toISOString();
  assert.equal(formatProjectDate(tenDaysAgo), "Updated 10 days ago");

  const oneDayAgo = new Date(Date.now() - 25 * 60 * 60 * 1000).toISOString();
  assert.equal(formatProjectDate(oneDayAgo), "Updated 1 day ago");

  const oldDate = "2024-01-15T00:00:00Z";
  const formatted = formatProjectDate(oldDate, "Created");
  assert.ok(formatted.includes("Created"));
  assert.ok(formatted.includes("2024"));
});

test("getRecruitmentStatusBadgeClasses returns specified badge tokens", () => {
  const open = getRecruitmentStatusBadgeClasses("open");
  assert.ok(open.bg.includes("emerald-500"));
  assert.ok(open.text.includes("emerald-600"));
  assert.ok(open.dot.includes("animate-pulse"));
  assert.equal(open.label, "Recruiting");

  const paused = getRecruitmentStatusBadgeClasses("paused");
  assert.ok(paused.bg.includes("amber-500"));
  assert.ok(paused.text.includes("amber-600"));
  assert.equal(paused.label, "Paused");

  const closed = getRecruitmentStatusBadgeClasses("closed");
  assert.ok(closed.bg.includes("muted"));
  assert.equal(closed.label, "Closed");
});

test("getLifecycleBadgeClasses returns correct labels and styling", () => {
  const idea = getLifecycleBadgeClasses("idea");
  assert.equal(idea.label, "Idea Stage");

  const active = getLifecycleBadgeClasses("active");
  assert.equal(active.label, "Active Build");

  const completed = getLifecycleBadgeClasses("completed");
  assert.equal(completed.label, "Completed");

  const archived = getLifecycleBadgeClasses("archived");
  assert.equal(archived.label, "Archived");
});

test("PROJECT_CATEGORIES includes all canonical campus project tracks", () => {
  assert.ok(PROJECT_CATEGORIES.length >= 6);
  const values = PROJECT_CATEGORIES.map((c) => c.value);
  assert.ok(values.includes("AI / Robotics"));
  assert.ok(values.includes("Web & Mobile"));
  assert.ok(values.includes("Research"));
  assert.ok(values.includes("Design"));
  assert.ok(values.includes("Hardware"));
  assert.ok(values.includes("Systems"));
});

test("slugifyProjectTitle produces sanitized URL slugs matching backend specifications", () => {
  assert.equal(
    slugifyProjectTitle("Autonomous Campus Delivery Rover"),
    "autonomous-campus-delivery-rover"
  );
  assert.equal(slugifyProjectTitle("AI & Robotics: Lab 2026! (V2)"), "ai-robotics-lab-2026-v2");
  assert.equal(slugifyProjectTitle("  ---Hello World---  "), "hello-world");
  assert.equal(slugifyProjectTitle(""), "");
  assert.equal(slugifyProjectTitle("   "), "");

  const longTitle = "a".repeat(150);
  const slugified = slugifyProjectTitle(longTitle);
  assert.equal(slugified.length, 100);
  assert.equal(slugified, "a".repeat(100));
});
