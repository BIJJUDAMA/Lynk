import type { ApplicationStatus, ContractStatus, JobStatus } from "@/types/api";

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

/**
 * Formats a job's budget or hourly rate into a clean monospace display string.
 */
export function formatJobBudget(job: {
  budget?: number | string | null;
  department?: string;
  required_skills?: string[];
}): string {
  if (job.budget !== undefined && job.budget !== null && job.budget !== "") {
    if (typeof job.budget === "number" && job.budget > 0) {
      return `$${job.budget} Fixed`;
    }
    if (typeof job.budget === "string" && job.budget.trim() !== "") {
      const b = job.budget.trim();
      return b.startsWith("$") ? b : `$${b}`;
    }
  }

  const dept = job.department || "";
  if (dept.includes("Computer Science") || dept.includes("Engineering")) return "$45/hr";
  if (dept.includes("Data Science") || dept.includes("AI")) return "$50/hr";
  if (dept.includes("Design") || dept.includes("Creative")) return "$650 Fixed";
  if (dept.includes("Business") || dept.includes("Marketing")) return "$35/hr";
  if (dept.includes("Biology") || dept.includes("Life Sciences")) return "$40/hr";
  if (dept.includes("Mathematics") || dept.includes("Statistics")) return "$42/hr";
  if (dept.includes("Writing") || dept.includes("Communications")) return "$30/hr";
  if (dept.includes("Economics") || dept.includes("Finance")) return "$45/hr";
  if (dept.includes("Psychology")) return "$30/hr";
  return "$35/hr";
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
