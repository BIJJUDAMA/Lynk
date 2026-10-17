import type { ApplicationStatus, ContractStatus, JobStatus } from "@/types/api";
import type { LifecycleStatus, RecruitmentStatus } from "@/types/project";

/**
 * Formats ISO date string into human-readable date.
 */
export function formatJobDate(dateStr?: string | null): string {
  if (!dateStr) return "Flexible";
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return dateStr;
    return d.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  } catch {
    return dateStr;
  }
}

/**
 * Returns color classes for job status badges.
 */
export function getStatusBadgeClasses(status: JobStatus): {
  bg: string;
  text: string;
  dot: string;
} {
  switch (status) {
    case "open":
      return {
        bg: "bg-pastel-green border-pastel-greenText/20",
        text: "text-pastel-greenText",
        dot: "bg-pastel-greenText",
      };
    case "in_progress":
      return {
        bg: "bg-pastel-blue border-pastel-blueText/20",
        text: "text-pastel-blueText",
        dot: "bg-pastel-blueText",
      };
    case "closed":
      return {
        bg: "bg-muted border-border",
        text: "text-muted-foreground",
        dot: "bg-muted-foreground",
      };
    case "cancelled":
      return {
        bg: "bg-pastel-red border-pastel-redText/20",
        text: "text-pastel-redText",
        dot: "bg-pastel-redText",
      };
    default:
      return {
        bg: "bg-muted border-border",
        text: "text-muted-foreground",
        dot: "bg-muted-foreground",
      };
  }
}

export const COMMON_DEPARTMENTS = [
  "Computer Science & Engineering",
  "Data Science & AI",
  "Design & Creative Arts",
  "Business & Marketing",
  "Biology & Life Sciences",
  "Mathematics & Statistics",
  "Writing & Communications",
  "Economics & Finance",
  "Psychology & Social Sciences",
];

export const POPULAR_SKILLS = [
  "React",
  "Python",
  "TypeScript",
  "Figma",
  "Data Analysis",
  "SQL",
  "Content Writing",
  "Machine Learning",
  "UI/UX",
];

/**
 * Formats a file size in bytes to a human-readable string (KB or MB).
 */
export function formatFileSize(bytes?: number | null): string {
  if (!bytes || bytes <= 0) return "0 KB";
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(2)} MB`;
}

/**
 * Returns color classes and labels for application status badges.
 */
export function getApplicationStatusBadgeClasses(status: ApplicationStatus): {
  bg: string;
  text: string;
  dot: string;
  label: string;
} {
  switch (status) {
    case "pending":
      return {
        bg: "bg-pastel-yellow border-pastel-yellowText/20",
        text: "text-pastel-yellowText",
        dot: "bg-pastel-yellowText",
        label: "Pending Review",
      };
    case "accepted":
      return {
        bg: "bg-pastel-green border-pastel-greenText/20",
        text: "text-pastel-greenText",
        dot: "bg-pastel-greenText",
        label: "Accepted",
      };
    case "rejected":
      return {
        bg: "bg-pastel-red border-pastel-redText/20",
        text: "text-pastel-redText",
        dot: "bg-pastel-redText",
        label: "Not Selected",
      };
    default:
      return {
        bg: "bg-muted border-border",
        text: "text-muted-foreground",
        dot: "bg-muted-foreground",
        label: status,
      };
  }
}

/**
 * Validates whether a string is a valid web URL starting with http:// or https://.
 */
export function isValidUrl(url: string): boolean {
  if (!url || typeof url !== "string") return false;
  try {
    const parsed = new URL(url.trim());
    return parsed.protocol === "http:" || parsed.protocol === "https:";
  } catch {
    return false;
  }
}

/**
 * Returns color classes and labels for contract status badges.
 */
export function getContractStatusBadgeClasses(status: ContractStatus): {
  bg: string;
  text: string;
  dot: string;
  label: string;
} {
  switch (status) {
    case "draft":
      return {
        bg: "bg-muted border-border",
        text: "text-muted-foreground",
        dot: "bg-muted-foreground",
        label: "Draft",
      };
    case "active":
      return {
        bg: "bg-pastel-blue border-pastel-blueText/20",
        text: "text-pastel-blueText",
        dot: "bg-pastel-blueText",
        label: "Active",
      };
    case "completed":
      return {
        bg: "bg-pastel-green border-pastel-greenText/20",
        text: "text-pastel-greenText",
        dot: "bg-pastel-greenText",
        label: "Completed",
      };
    case "cancelled":
      return {
        bg: "bg-pastel-red border-pastel-redText/20",
        text: "text-pastel-redText",
        dot: "bg-pastel-redText",
        label: "Cancelled",
      };
    default:
      return {
        bg: "bg-muted border-border",
        text: "text-muted-foreground",
        dot: "bg-muted-foreground",
        label: status,
      };
  }
}

/**
 * Formats ISO date string into human-readable project timestamp.
 * Returns relative time or formatted date with configurable prefix.
 */
export function formatProjectDate(dateStr?: string | null, prefix: string = "Updated"): string {
  if (!dateStr) return `${prefix} recently`;
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return `${prefix} ${dateStr}`;
    const now = Date.now();
    const diffMs = now - d.getTime();
    if (diffMs < 0) {
      const formatted = d.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      });
      return `${prefix} ${formatted}`;
    }
    const diffSecs = Math.floor(diffMs / 1000);
    const diffMins = Math.floor(diffSecs / 60);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    if (diffDays > 30) {
      const formatted = d.toLocaleDateString("en-US", {
        month: "short",
        day: "numeric",
        year: "numeric",
      });
      return `${prefix} ${formatted}`;
    }
    if (diffDays >= 1) {
      return `${prefix} ${diffDays} ${diffDays === 1 ? "day" : "days"} ago`;
    }
    if (diffHours >= 1) {
      return `${prefix} ${diffHours} ${diffHours === 1 ? "hour" : "hours"} ago`;
    }
    if (diffMins >= 1) {
      return `${prefix} ${diffMins} ${diffMins === 1 ? "min" : "mins"} ago`;
    }
    return `${prefix} just now`;
  } catch {
    return `${prefix} ${dateStr}`;
  }
}

/**
 * Returns color classes and dot animation tokens for project recruitment status badges.
 */
export function getRecruitmentStatusBadgeClasses(status: RecruitmentStatus): {
  bg: string;
  text: string;
  border: string;
  dot: string;
  label: string;
} {
  switch (status) {
    case "open":
      return {
        bg: "bg-emerald-500/10",
        text: "text-emerald-600",
        border: "border-emerald-500/20",
        dot: "bg-emerald-500 animate-pulse",
        label: "Recruiting",
      };
    case "paused":
      return {
        bg: "bg-amber-500/10",
        text: "text-amber-600",
        border: "border-amber-500/20",
        dot: "bg-amber-500",
        label: "Paused",
      };
    case "closed":
      return {
        bg: "bg-muted",
        text: "text-muted-foreground",
        border: "border-border",
        dot: "bg-muted-foreground",
        label: "Closed",
      };
    default:
      return {
        bg: "bg-muted",
        text: "text-muted-foreground",
        border: "border-border",
        dot: "bg-muted-foreground",
        label: status,
      };
  }
}

/**
 * Returns color classes and labels for project lifecycle status badges.
 */
export function getLifecycleBadgeClasses(status: LifecycleStatus): {
  bg: string;
  text: string;
  border: string;
  label: string;
} {
  switch (status) {
    case "idea":
      return {
        bg: "bg-muted/40",
        text: "text-muted-foreground",
        border: "border-border/60",
        label: "Idea Stage",
      };
    case "active":
      return {
        bg: "bg-surface",
        text: "text-foreground",
        border: "border-border/60",
        label: "Active Build",
      };
    case "completed":
      return {
        bg: "bg-muted/50",
        text: "text-muted-foreground",
        border: "border-border/60",
        label: "Completed",
      };
    case "archived":
      return {
        bg: "bg-muted/20",
        text: "text-muted-foreground/80",
        border: "border-border/40",
        label: "Archived",
      };
    default:
      return {
        bg: "bg-surface",
        text: "text-muted-foreground",
        border: "border-border/60",
        label: status,
      };
  }
}

/**
 * Canonical project category options for discovery filters and project creation.
 */
export const PROJECT_CATEGORIES = [
  { id: "all", label: "All Categories", value: "" },
  { id: "ai-robotics", label: "AI / Robotics", value: "AI / Robotics" },
  { id: "web-mobile", label: "Web & Mobile", value: "Web & Mobile" },
  { id: "research", label: "Research", value: "Research" },
  { id: "design", label: "Design", value: "Design" },
  { id: "hardware", label: "Hardware", value: "Hardware" },
  { id: "systems", label: "Systems", value: "Systems" },
] as const;

/**
 * Generates a clean URL slug preview from a project title.
 * Replaces non-alphanumeric characters with hyphens and trims leading/trailing hyphens.
 */
export function slugifyProjectTitle(title: string): string {
  const lower = title.toLowerCase().trim();
  const cleaned = lower.replace(/[^a-z0-9]+/g, "-");
  const trimmed = cleaned.replace(/^-+|-+$/g, "");
  if (trimmed.length > 100) {
    return trimmed.slice(0, 100).replace(/-+$/, "");
  }
  return trimmed;
}
