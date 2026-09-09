import type { ApplicationStatus, ContractStatus, JobPayType, JobStatus } from "@/types/api";

/**
 * Formats integer cents as USD currency string.
 */
export function formatUsdFromCents(cents: number): string {
  const n = Number.isFinite(cents) ? cents : 0;
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
  }).format(n / 100);
}

/**
 * Formats budget cents with pay type badge (e.g. "$500.00 Fixed" or "$25.00/hr").
 */
export function formatBudget(budgetCents: number, payType: JobPayType): string {
  const formatted = formatUsdFromCents(budgetCents);
  return payType === "hourly" ? `${formatted}/hr` : `${formatted} Fixed`;
}

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
        bg: "bg-emerald-50 dark:bg-emerald-950/50 border-emerald-200 dark:border-emerald-800/60",
        text: "text-emerald-700 dark:text-emerald-300",
        dot: "bg-emerald-500",
      };
    case "in_progress":
      return {
        bg: "bg-sky-50 dark:bg-sky-950/50 border-sky-200 dark:border-sky-800/60",
        text: "text-sky-700 dark:text-sky-300",
        dot: "bg-sky-500",
      };
    case "closed":
      return {
        bg: "bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700",
        text: "text-slate-700 dark:text-slate-300",
        dot: "bg-slate-400",
      };
    case "cancelled":
      return {
        bg: "bg-rose-50 dark:bg-rose-950/50 border-rose-200 dark:border-rose-800/60",
        text: "text-rose-700 dark:text-rose-300",
        dot: "bg-rose-500",
      };
    default:
      return {
        bg: "bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700",
        text: "text-slate-600 dark:text-slate-400",
        dot: "bg-slate-400",
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
        bg: "bg-amber-50 dark:bg-amber-950/50 border-amber-200 dark:border-amber-800/60",
        text: "text-amber-700 dark:text-amber-300",
        dot: "bg-amber-500",
        label: "Pending Review",
      };
    case "accepted":
      return {
        bg: "bg-emerald-50 dark:bg-emerald-950/50 border-emerald-200 dark:border-emerald-800/60",
        text: "text-emerald-700 dark:text-emerald-300",
        dot: "bg-emerald-500",
        label: "Accepted",
      };
    case "rejected":
      return {
        bg: "bg-rose-50 dark:bg-rose-950/50 border-rose-200 dark:border-rose-800/60",
        text: "text-rose-700 dark:text-rose-300",
        dot: "bg-rose-500",
        label: "Not Selected",
      };
    default:
      return {
        bg: "bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700",
        text: "text-slate-600 dark:text-slate-400",
        dot: "bg-slate-400",
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
        bg: "bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700",
        text: "text-slate-700 dark:text-slate-300",
        dot: "bg-slate-400",
        label: "Draft",
      };
    case "active":
      return {
        bg: "bg-blue-50 dark:bg-blue-950/50 border-blue-200 dark:border-blue-800/60",
        text: "text-blue-700 dark:text-blue-300",
        dot: "bg-blue-500",
        label: "Active",
      };
    case "completed":
      return {
        bg: "bg-emerald-50 dark:bg-emerald-950/50 border-emerald-200 dark:border-emerald-800/60",
        text: "text-emerald-700 dark:text-emerald-300",
        dot: "bg-emerald-500",
        label: "Completed",
      };
    case "cancelled":
      return {
        bg: "bg-rose-50 dark:bg-rose-950/50 border-rose-200 dark:border-rose-800/60",
        text: "text-rose-700 dark:text-rose-300",
        dot: "bg-rose-500",
        label: "Cancelled",
      };
    default:
      return {
        bg: "bg-slate-100 dark:bg-slate-800 border-slate-200 dark:border-slate-700",
        text: "text-slate-600 dark:text-slate-400",
        dot: "bg-slate-400",
        label: status,
      };
  }
}

