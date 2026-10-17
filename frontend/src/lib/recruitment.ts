/**
 * Lynk Recruitment and Application Helpers
 *
 * Provides helper functions for application status styling, filtering,
 * metric calculations, and applicant metadata extraction.
 */

import type { ApplicationStatus, ProjectApplication } from "../types/project.ts";

export interface StatusBadgeStyle {
  bg: string;
  text: string;
  border: string;
  label: string;
  dot: string;
}

export interface ApplicationMetrics {
  total: number;
  submitted: number;
  shortlisted: number;
  accepted: number;
  rejected: number;
  withdrawn: number;
}

/**
 * Extracts a display name for the applicant using flat or nested attributes.
 */
export function getApplicantDisplayName(app: ProjectApplication): string {
  if (app.applicant_name && app.applicant_name.trim().length > 0) {
    return app.applicant_name.trim();
  }

  if (app.applicant) {
    const fullName = [app.applicant.first_name, app.applicant.last_name]
      .filter(Boolean)
      .join(" ")
      .trim();
    if (fullName.length > 0) {
      return fullName;
    }
    if (app.applicant.email && app.applicant.email.trim().length > 0) {
      return app.applicant.email.trim();
    }
  }

  if (app.applicant_email && app.applicant_email.trim().length > 0) {
    return app.applicant_email.trim();
  }

  return "Applicant";
}

/**
 * Extracts the applicant email address.
 */
export function getApplicantEmail(app: ProjectApplication): string {
  if (app.applicant_email && app.applicant_email.trim().length > 0) {
    return app.applicant_email.trim();
  }
  if (app.applicant?.email && app.applicant.email.trim().length > 0) {
    return app.applicant.email.trim();
  }
  return "";
}

/**
 * Extracts the applicant department.
 */
export function getApplicantDepartment(app: ProjectApplication): string {
  if (app.applicant_department && app.applicant_department.trim().length > 0) {
    return app.applicant_department.trim();
  }
  if (app.applicant?.department && app.applicant.department.trim().length > 0) {
    return app.applicant.department.trim();
  }
  return "General Campus";
}

/**
 * Computes 1-2 character initials for the applicant avatar.
 */
export function getApplicantInitials(name?: string): string {
  if (!name || name.trim().length === 0) {
    return "AP";
  }

  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

/**
 * Returns consistent styling tokens and labels for application statuses.
 */
export function getApplicationStatusBadgeClasses(
  status: ApplicationStatus | string
): StatusBadgeStyle {
  switch (status) {
    case "submitted":
      return {
        bg: "bg-blue-500/10",
        text: "text-blue-600",
        border: "border-blue-500/20",
        label: "Submitted",
        dot: "bg-blue-500",
      };
    case "shortlisted":
      return {
        bg: "bg-amber-500/10",
        text: "text-amber-600",
        border: "border-amber-500/20",
        label: "Shortlisted",
        dot: "bg-amber-500",
      };
    case "accepted":
      return {
        bg: "bg-emerald-500/10",
        text: "text-emerald-600",
        border: "border-emerald-500/20",
        label: "Accepted",
        dot: "bg-emerald-500",
      };
    case "rejected":
      return {
        bg: "bg-rose-500/10",
        text: "text-rose-600",
        border: "border-rose-500/20",
        label: "Rejected",
        dot: "bg-rose-500",
      };
    case "withdrawn":
      return {
        bg: "bg-muted",
        text: "text-muted-foreground",
        border: "border-border",
        label: "Withdrawn",
        dot: "bg-muted-foreground",
      };
    default:
      return {
        bg: "bg-muted",
        text: "text-muted-foreground",
        border: "border-border",
        label: status || "Unknown",
        dot: "bg-muted-foreground",
      };
  }
}

/**
 * Aggregates metric counts across all project applications.
 */
export function computeApplicationMetrics(applications: ProjectApplication[]): ApplicationMetrics {
  const metrics: ApplicationMetrics = {
    total: 0,
    submitted: 0,
    shortlisted: 0,
    accepted: 0,
    rejected: 0,
    withdrawn: 0,
  };

  if (!Array.isArray(applications)) {
    return metrics;
  }

  for (const app of applications) {
    if (!app) continue;
    metrics.total += 1;
    switch (app.status) {
      case "submitted":
        metrics.submitted += 1;
        break;
      case "shortlisted":
        metrics.shortlisted += 1;
        break;
      case "accepted":
        metrics.accepted += 1;
        break;
      case "rejected":
        metrics.rejected += 1;
        break;
      case "withdrawn":
        metrics.withdrawn += 1;
        break;
      default:
        break;
    }
  }

  return metrics;
}

/**
 * Filters a list of applications based on search query and status.
 */
export function filterApplications(
  applications: ProjectApplication[],
  query: string,
  statusFilter: ApplicationStatus | "all"
): ProjectApplication[] {
  if (!Array.isArray(applications)) {
    return [];
  }

  const normalizedQuery = (query || "").trim().toLowerCase();

  return applications.filter((app) => {
    if (!app) return false;

    // Status filter
    if (statusFilter !== "all" && app.status !== statusFilter) {
      return false;
    }

    // Text search filter
    if (normalizedQuery.length > 0) {
      const name = getApplicantDisplayName(app).toLowerCase();
      const email = getApplicantEmail(app).toLowerCase();
      const department = getApplicantDepartment(app).toLowerCase();
      const statement = (app.statement || "").toLowerCase();

      const matchesName = name.includes(normalizedQuery);
      const matchesEmail = email.includes(normalizedQuery);
      const matchesDept = department.includes(normalizedQuery);
      const matchesStatement = statement.includes(normalizedQuery);

      if (!matchesName && !matchesEmail && !matchesDept && !matchesStatement) {
        return false;
      }
    }

    return true;
  });
}
