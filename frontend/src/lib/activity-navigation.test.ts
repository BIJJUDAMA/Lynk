import test from "node:test";
import assert from "node:assert/strict";
import { getApplicationStatusBadgeClasses } from "./recruitment.ts";
import type { ApplicationStatus } from "../types/project.ts";

/**
 * Maps query param to valid activity tab.
 */
function resolveActivityTab(rawTab?: string | null): "projects" | "applications" | "postings" {
  if (rawTab === "applications") return "applications";
  if (rawTab === "postings" || rawTab === "jobs") return "postings";
  return "projects";
}

/**
 * Resolves workspace URL slug or ID fallback.
 */
function getWorkspaceHref(project: { id: string; slug?: string }): string {
  return `/projects/${project.slug || project.id}`;
}

test("resolveActivityTab defaults to projects", () => {
  assert.equal(resolveActivityTab(null), "projects");
  assert.equal(resolveActivityTab(""), "projects");
  assert.equal(resolveActivityTab("unknown"), "projects");
  assert.equal(resolveActivityTab("contracts"), "projects");
});

test("resolveActivityTab maps applications and postings correctly", () => {
  assert.equal(resolveActivityTab("applications"), "applications");
  assert.equal(resolveActivityTab("postings"), "postings");
  assert.equal(resolveActivityTab("jobs"), "postings");
});

test("getWorkspaceHref uses slug when present, falling back to id", () => {
  assert.equal(getWorkspaceHref({ id: "uuid-123", slug: "mars-rover" }), "/projects/mars-rover");
  assert.equal(getWorkspaceHref({ id: "uuid-123", slug: "" }), "/projects/uuid-123");
  assert.equal(getWorkspaceHref({ id: "uuid-456" }), "/projects/uuid-456");
});

test("application status styling handles all 5 canonical statuses", () => {
  const statuses: ApplicationStatus[] = [
    "submitted",
    "shortlisted",
    "accepted",
    "rejected",
    "withdrawn",
  ];

  for (const status of statuses) {
    const badge = getApplicationStatusBadgeClasses(status);
    assert.ok(badge.bg, `Missing bg for ${status}`);
    assert.ok(badge.text, `Missing text for ${status}`);
    assert.ok(badge.border, `Missing border for ${status}`);
    assert.ok(badge.dot, `Missing dot for ${status}`);
    assert.ok(badge.label, `Missing label for ${status}`);
  }
});
