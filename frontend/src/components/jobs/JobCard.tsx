import Link from "next/link";
import { Building2, ArrowRight } from "lucide-react";
import { Job } from "@/types/api";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { formatJobDate, getStatusBadgeClasses, formatJobBudget } from "@/lib/formatters";

export { formatJobDate, getStatusBadgeClasses, formatJobBudget };

export interface JobCardProps {
  job: Job;
  className?: string;
}

export function JobCard({ job, className }: JobCardProps) {
  const statusStyles = getStatusBadgeClasses(job.status);
  const posterName =
    job.creator?.first_name || job.creator?.last_name
      ? `${job.creator.first_name || ""} ${job.creator.last_name || ""}`.trim()
      : job.creator?.organization ||
        job.employer?.company_or_org ||
        job.employer?.contact_name ||
        "Campus Member";

  const skills = job.required_skills ?? [];
  const visibleSkills = skills.slice(0, 4);
  const overflowSkillsCount = skills.length - visibleSkills.length;
  const budgetDisplay = formatJobBudget(job);

  return (
    <article
      data-animate-item
      className={cn(
        "rounded-xl border border-border bg-card p-6 transition-all duration-200 hover:border-foreground/30 hover:-translate-y-0.5 shadow-none flex flex-col justify-between",
        className
      )}
    >
      <div>
        {/* Top Badges Bar: Status, Department */}
        <div className="flex flex-wrap items-center gap-2">
          {/* Status Badge */}
          <span
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-mono uppercase tracking-wider",
              statusStyles.bg,
              statusStyles.text
            )}
          >
            <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
            {job.status.replace("_", " ")}
          </span>

          {/* Department Badge */}
          <Badge variant="outline" className="text-xs font-mono">
            {job.department || "General"}
          </Badge>
        </div>

        {/* Job Title & Creator */}
        <div className="mt-3">
          <Link href={`/jobs/${job.id}`}>
            <h3 className="font-serif text-xl font-medium tracking-tight text-foreground hover:text-foreground/80 transition-colors line-clamp-1">
              {job.title}
            </h3>
          </Link>

          <div className="mt-1 flex items-center gap-1.5 text-xs font-mono text-muted-foreground">
            <Building2 className="h-3.5 w-3.5 text-muted-foreground/70" />
            <span className="truncate">{posterName}</span>
          </div>
        </div>

        {/* Description Snippet */}
        <p className="mt-3 text-sm text-muted-foreground leading-relaxed line-clamp-2">
          {job.description}
        </p>

        {/* Required Skills Tags */}
        {skills.length > 0 && (
          <div className="mt-4 flex flex-wrap gap-1.5">
            {visibleSkills.map((skill, index) => (
              <Badge
                key={`${skill}-${index}`}
                variant="secondary"
                className="text-[11px] font-mono py-0.5 px-2"
              >
                {skill}
              </Badge>
            ))}
            {overflowSkillsCount > 0 && (
              <Badge
                variant="secondary"
                className="text-[11px] font-mono py-0.5 px-2 text-muted-foreground"
              >
                +{overflowSkillsCount} more
              </Badge>
            )}
          </div>
        )}
      </div>

      {/* Card Footer: Budget, Posted Date & Direct Link */}
      <div className="mt-6 flex items-center justify-between border-t border-border pt-4">
        <div className="flex flex-col">
          <span className="font-mono text-base font-semibold text-foreground">{budgetDisplay}</span>
          <span className="text-xs text-muted-foreground font-mono">
            {job.created_at
              ? `Posted ${formatJobDate(job.created_at)}`
              : job.deadline
                ? `Due ${formatJobDate(job.deadline)}`
                : "Active"}
          </span>
        </div>

        <Link
          href={`/jobs/${job.id}`}
          className="inline-flex items-center gap-1 text-xs font-mono font-medium text-foreground hover:text-foreground/70 transition-colors"
        >
          <span>View Gig</span>
          <ArrowRight className="h-3.5 w-3.5 transition group-hover:translate-x-0.5" />
        </Link>
      </div>
    </article>
  );
}
