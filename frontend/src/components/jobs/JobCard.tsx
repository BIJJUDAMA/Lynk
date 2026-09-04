import Link from "next/link";
import {
  Briefcase,
  Building2,
  Calendar,
  DollarSign,
  GraduationCap,
  ArrowRight,
  Clock,
} from "lucide-react";
import { Job, JobPayType, JobStatus } from "@/types/api";
import { cn } from "@/lib/utils";

export interface JobCardProps {
  job: Job;
  className?: string;
}

/**
 * Formats a currency number with pay type badge (e.g. "$500 Fixed" or "$25/hr").
 */
export function formatBudget(budget: number, payType: JobPayType): string {
  const formatted = new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 0,
  }).format(budget);

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
        bg: "bg-emerald-50 dark:bg-emerald-950/50 border-emerald-200 dark:border-emerald-800/60",
        text: "text-emerald-700 dark:text-emerald-300",
        dot: "bg-emerald-500",
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

export function JobCard({ job, className }: JobCardProps) {
  const statusStyles = getStatusBadgeClasses(job.status);
  const employerName =
    job.employer?.company_or_org ||
    job.employer?.contact_name ||
    "Campus Employer";

  const skills = job.required_skills ?? [];
  const visibleSkills = skills.slice(0, 4);
  const overflowSkillsCount = skills.length - visibleSkills.length;

  return (
    <article
      data-animate-item
      className={cn(
        "group relative flex flex-col justify-between rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm transition-all duration-200 hover:-translate-y-0.5 hover:border-emerald-300 hover:shadow-md dark:border-slate-800 dark:bg-slate-900/90 dark:hover:border-emerald-700",
        className
      )}
    >
      <div>
        {/* Top Meta Bar: Status, Department, Pay */}
        <div className="flex flex-wrap items-center justify-between gap-2 pb-3">
          <div className="flex flex-wrap items-center gap-2">
            {/* Status Badge */}
            <span
              className={cn(
                "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider",
                statusStyles.bg,
                statusStyles.text
              )}
            >
              <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
              {job.status.replace("_", " ")}
            </span>

            {/* Department Badge */}
            {job.department && (
              <span className="inline-flex items-center gap-1 rounded-[10px] bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                <GraduationCap className="h-3.5 w-3.5 text-slate-500" />
                <span>{job.department}</span>
              </span>
            )}
          </div>

          {/* Budget Badge */}
          <div className="inline-flex items-center gap-1 rounded-[10px] bg-emerald-50 px-3 py-1 text-xs font-bold text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300">
            <DollarSign className="h-3.5 w-3.5" />
            <span>{formatBudget(job.budget, job.pay_type)}</span>
          </div>
        </div>

        {/* Job Title & Employer */}
        <div className="mt-2">
          <Link
            href={`/jobs/${job.id}`}
            className="group-hover:text-emerald-600 dark:group-hover:text-emerald-400 transition"
          >
            <h3 className="text-lg font-bold tracking-tight text-slate-900 dark:text-white line-clamp-1">
              {job.title}
            </h3>
          </Link>

          <div className="mt-1 flex items-center gap-1.5 text-xs font-medium text-slate-500 dark:text-slate-400">
            <Building2 className="h-3.5 w-3.5 text-slate-400" />
            <span className="truncate">{employerName}</span>
          </div>
        </div>

        {/* Description Snippet */}
        <p className="mt-3 text-sm leading-relaxed text-slate-600 dark:text-slate-300 line-clamp-2">
          {job.description}
        </p>

        {/* Required Skills Tags */}
        {skills.length > 0 && (
          <div className="mt-4 flex flex-wrap gap-1.5">
            {visibleSkills.map((skill, index) => (
              <span
                key={`${skill}-${index}`}
                className="inline-flex items-center rounded-[10px] border border-slate-200 bg-slate-50 px-2 py-0.5 text-xs font-medium text-slate-600 dark:border-slate-800 dark:bg-slate-800/60 dark:text-slate-300"
              >
                {skill}
              </span>
            ))}
            {overflowSkillsCount > 0 && (
              <span className="inline-flex items-center rounded-[10px] border border-slate-200 bg-slate-50 px-2 py-0.5 text-xs font-medium text-slate-500 dark:border-slate-800 dark:bg-slate-800/60 dark:text-slate-400">
                +{overflowSkillsCount} more
              </span>
            )}
          </div>
        )}
      </div>

      {/* Card Footer: Deadline & Link */}
      <div className="mt-6 flex items-center justify-between border-t border-slate-100 pt-4 dark:border-slate-800/80">
        <div className="flex items-center gap-1.5 text-xs text-slate-500 dark:text-slate-400">
          <Calendar className="h-3.5 w-3.5 text-slate-400" />
          <span>
            {job.deadline ? `Due ${formatJobDate(job.deadline)}` : "Flexible deadline"}
          </span>
        </div>

        <Link
          href={`/jobs/${job.id}`}
          className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-600 transition hover:text-emerald-500 dark:text-emerald-400 dark:hover:text-emerald-300"
        >
          <span>View Details</span>
          <ArrowRight className="h-3.5 w-3.5 transition group-hover:translate-x-0.5" />
        </Link>
      </div>
    </article>
  );
}
