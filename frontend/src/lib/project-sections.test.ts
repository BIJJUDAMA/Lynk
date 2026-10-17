import test from "node:test";
import assert from "node:assert/strict";
import {
  isTrustedSectionType,
  filterAndSortSections,
  isSafeResourceUrl,
  getResourceDomain,
  getResourceCategory,
} from "./project-sections.ts";
import type { ProjectSection } from "../types/project.ts";

test("isTrustedSectionType validates standard platform section types", () => {
  assert.equal(isTrustedSectionType("overview"), true);
  assert.equal(isTrustedSectionType("team"), true);
  assert.equal(isTrustedSectionType("resources"), true);
  assert.equal(isTrustedSectionType("recruitment"), true);
  assert.equal(isTrustedSectionType("updates"), true);
  assert.equal(isTrustedSectionType("docs"), true);

  // Untrusted or arbitrary script types must be rejected
  assert.equal(isTrustedSectionType("custom_script"), false);
  assert.equal(isTrustedSectionType("unknown"), false);
  assert.equal(isTrustedSectionType("<script>"), false);
  assert.equal(isTrustedSectionType(""), false);
});

test("filterAndSortSections ignores disabled and untrusted sections and sorts by position ascending", () => {
  const sections: ProjectSection[] = [
    {
      id: "sec-3",
      project_id: "p1",
      section_type: "updates",
      title: "Updates",
      enabled: true,
      position: 3,
      config: {},
      content: "Latest updates",
    },
    {
      id: "sec-disabled",
      project_id: "p1",
      section_type: "docs",
      title: "Docs",
      enabled: false,
      position: 1,
      config: {},
      content: "Hidden docs",
    },
    {
      id: "sec-untrusted",
      project_id: "p1",
      section_type: "untrusted_type" as unknown as ProjectSection["section_type"],
      title: "Exploit",
      enabled: true,
      position: 0,
      config: {},
      content: "malicious code",
    },
    {
      id: "sec-0",
      project_id: "p1",
      section_type: "overview",
      title: "Overview",
      enabled: true,
      position: 0,
      config: {},
      content: "Welcome to project",
    },
    {
      id: "sec-2",
      project_id: "p1",
      section_type: "team",
      title: "Team",
      enabled: true,
      position: 2,
      config: {},
      content: "",
    },
  ];

  const result = filterAndSortSections(sections);
  assert.equal(result.length, 3);
  assert.equal(result[0].section_type, "overview");
  assert.equal(result[1].section_type, "team");
  assert.equal(result[2].section_type, "updates");
});

test("filterAndSortSections handles empty or null input gracefully", () => {
  assert.deepEqual(filterAndSortSections([]), []);
  assert.deepEqual(filterAndSortSections(null), []);
  assert.deepEqual(filterAndSortSections(undefined), []);
});

test("isSafeResourceUrl accepts valid http and https URLs and rejects unsafe schemes", () => {
  assert.equal(isSafeResourceUrl("https://github.com/lynk/project"), true);
  assert.equal(isSafeResourceUrl("http://localhost:3000/demo"), true);
  assert.equal(isSafeResourceUrl("https://docs.lynk.edu/spec"), true);

  // Unsafe schemes must be rejected
  assert.equal(isSafeResourceUrl("javascript:alert(1)"), false);
  assert.equal(isSafeResourceUrl("data:text/html,<script>alert(1)</script>"), false);
  assert.equal(isSafeResourceUrl("file:///etc/passwd"), false);
  assert.equal(isSafeResourceUrl(""), false);
  assert.equal(isSafeResourceUrl(null), false);
  assert.equal(isSafeResourceUrl("not a url"), false);
});

test("getResourceDomain extracts clean hostname", () => {
  assert.equal(getResourceDomain("https://github.com/organization/repo"), "github.com");
  assert.equal(getResourceDomain("https://www.example.org/project"), "example.org");
  assert.equal(getResourceDomain("javascript:void(0)"), "");
});

test("getResourceCategory categorizes links into icons", () => {
  assert.equal(getResourceCategory("github", "https://github.com/team/repo"), "github");
  assert.equal(getResourceCategory(null, "https://github.com/team/repo"), "github");
  assert.equal(getResourceCategory("demo", "https://demo.vercel.app"), "demo");
  assert.equal(getResourceCategory("docs", "https://notion.site/my-doc"), "docs");
  assert.equal(getResourceCategory("other", "https://example.com"), "link");
});
