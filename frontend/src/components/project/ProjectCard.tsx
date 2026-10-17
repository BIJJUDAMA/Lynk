import Link from "next/link";
import { Users, ArrowRight, Sparkles } from "lucide-react";
import type { Project } from "@/types/project";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import {
  formatProjectDate,
  getRecruitmentStatusBadgeClasses,
  getLifecycleBadgeClasses,
} from "@/lib/formatters";

export interface ProjectCardProps {
  project: Project;
  className?: string;
}

export function ProjectCard({ project, className }: ProjectCardProps) {
  const recruitmentStyles = getRecruitmentStatusBadgeClasses(project.recruitment_status);
  const lifecycleStyles = getLifecycleBadgeClasses(project.lifecycle_status);

  const leadName =
    (typeof project.settings?.owner_name === "string" && project.settings.owner_name.trim()) ||
    (typeof project.settings?.lead_name === "string" && project.settings.lead_name.trim()) ||
    "Campus Lead";

  const dateLabel = formatProjectDate(project.updated_at || project.created_at, "Updated");

  const projectHref = `/projects/${project.slug || project.id}`;

  return (
    <article
      data-animate-item
      className={cn(
        "group rounded-xl border border-border bg-card p-6 transition-all duration-200 hover:border-foreground/30 hover:-translate-y-0.5 shadow-none flex flex-col justify-between",
        className
      )}
    >
      <div>
        {/* Top Badges Bar: Category, Recruitment, Lifecycle, Visibility */}
        <div className="flex flex-wrap items-center gap-2">
          {/* Category Badge */}
          {project.category && (
            <Badge variant="outline" className="font-mono text-xs">
              {project.category}
            </Badge>
          )}

          {/* Recruitment Status Badge */}
          <span
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-mono uppercase tracking-wider",
              recruitmentStyles.bg,
              recruitmentStyles.text,
              recruitmentStyles.border
            )}
          >
            <span className={cn("h-1.5 w-1.5 rounded-full", recruitmentStyles.dot)} />
            {recruitmentStyles.label}
          </span>

          {/* Lifecycle Status Badge */}
          <span
            className={cn(
              "inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-mono",
              lifecycleStyles.border,
              lifecycleStyles.text,
              lifecycleStyles.bg
            )}
          >
            {lifecycleStyles.label}
          </span>

          {/* Unlisted Visibility Pill */}
          {project.visibility === "unlisted" && (
            <span className="inline-flex items-center rounded-full border border-dashed border-border px-2 py-0.5 text-[11px] font-mono text-muted-foreground">
              Unlisted
            </span>
          )}

          {/* Semantic Similarity Match Indicator */}
          {typeof project.similarity === "number" && (
            <span className="inline-flex items-center gap-1 rounded-full border border-pastel-blueText/20 bg-pastel-blue px-2 py-0.5 text-[10px] font-mono text-pastel-blueText uppercase">
              <Sparkles className="h-3 w-3" />
              {Math.round(project.similarity * 100)}% match
            </span>
          )}
        </div>

        {/* Project Title */}
        <div className="mt-3">
          <Link href={projectHref}>
            <h3 className="font-serif text-xl font-normal leading-snug text-foreground hover:text-foreground/80 transition-colors line-clamp-2 [text-wrap:balance]">
              {project.title}
            </h3>
          </Link>
        </div>

        {/* Description */}
        <p className="mt-3 text-sm text-muted-foreground leading-relaxed line-clamp-2">
          {project.description}
        </p>
      </div>

      {/* Footer: Lead, Timestamp & Workspace Link */}
      <div className="mt-6 flex items-center justify-between border-t border-border pt-4">
        <div className="flex flex-col min-w-0 pr-2">
          <div className="flex items-center gap-1.5 text-xs font-mono text-foreground truncate">
            <Users className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
            <span className="truncate">{leadName}</span>
          </div>
          <span className="text-[11px] text-muted-foreground font-mono mt-0.5">{dateLabel}</span>
        </div>

        <Link
          href={projectHref}
          className="inline-flex shrink-0 items-center gap-1 text-xs font-mono font-medium text-foreground hover:text-foreground/70 transition-colors"
        >
          <span>View Workspace</span>
          <ArrowRight className="h-3.5 w-3.5 transition-transform group-hover:translate-x-0.5" />
        </Link>
      </div>
    </article>
  );
}
