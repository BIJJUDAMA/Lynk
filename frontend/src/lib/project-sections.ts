/**
 * Lynk Project Workspace Section Registry and Helpers
 *
 * Implements pure functions for validating platform-trusted workspace sections,
 * sorting by position order, and sanitizing external resource links.
 */

import type { ProjectSection, SectionType } from "../types/project.ts";
import { SECTION_TYPES } from "../types/project.ts";

export const TRUSTED_SECTION_TYPES: readonly SectionType[] = SECTION_TYPES;

const TRUSTED_SECTION_SET = new Set<string>(SECTION_TYPES);

/**
 * Checks if a section type is registered in the platform-approved section registry.
 * Prevents execution or rendering of arbitrary/untrusted section types.
 */
export function isTrustedSectionType(type: string): type is SectionType {
  return TRUSTED_SECTION_SET.has(type);
}

/**
 * Filters sections by enabled status, excludes untrusted types,
 * and sorts strictly in ascending position order.
 */
export function filterAndSortSections(sections?: ProjectSection[] | null): ProjectSection[] {
  if (!sections || !Array.isArray(sections)) {
    return [];
  }

  return sections
    .filter((sec) => sec && sec.enabled && isTrustedSectionType(sec.section_type))
    .slice()
    .sort((a, b) => (a.position ?? 0) - (b.position ?? 0));
}

/**
 * Validates whether an external resource link is safe to open.
 * Strictly requires http: or https: schemes and non-empty hostname.
 * Rejects javascript:, data:, file:, and malformed URIs.
 */
export function isSafeResourceUrl(url?: string | null): boolean {
  if (!url || typeof url !== "string") {
    return false;
  }
  const trimmed = url.trim();
  if (!trimmed) {
    return false;
  }

  try {
    const parsed = new URL(trimmed);
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

/**
 * Extracts a clean hostname or domain label from a URL for display.
 */
export function getResourceDomain(url?: string | null): string {
  if (!url || !isSafeResourceUrl(url)) {
    return "";
  }
  try {
    const parsed = new URL(url.trim());
    return parsed.hostname.replace(/^www\./, "");
  } catch {
    return "";
  }
}

/**
 * Categorizes a resource URL and resource_type into a canonical icon key.
 */
export function getResourceCategory(
  resourceType?: string | null,
  url?: string | null
): "github" | "demo" | "docs" | "link" {
  const normType = (resourceType || "").toLowerCase().trim();
  const normUrl = (url || "").toLowerCase().trim();

  if (normType === "github" || normUrl.includes("github.com")) {
    return "github";
  }
  if (
    normType === "demo" ||
    normType === "preview" ||
    normType === "live" ||
    normUrl.includes("vercel.app") ||
    normUrl.includes("netlify.app")
  ) {
    return "demo";
  }
  if (
    normType === "docs" ||
    normType === "documentation" ||
    normUrl.includes("notion.site") ||
    normUrl.includes("gitbook.io") ||
    normUrl.includes("docs.")
  ) {
    return "docs";
  }
  return "link";
}
