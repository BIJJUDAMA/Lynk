import React from "react";
import Link from "next/link";
import {
  Sparkles,
  Briefcase,
  CheckCircle2,
  ShieldAlert,
  Lock,
  PauseCircle,
  XCircle,
} from "lucide-react";
import type { Project, ProjectSection } from "@/types/project";
import { cn } from "@/lib/utils";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";
import { getRecruitmentStatusBadgeClasses } from "@/lib/formatters";

export interface RecruitmentSectionProps {
  section: ProjectSection;
  project: Project;
  canApply?: boolean;
  isMember?: boolean;
  isLeadOrMaintainer?: boolean;
  isAuthenticated?: boolean;
  isVerified?: boolean;
  onApplyClick?: () => void;
  className?: string;
}

export function RecruitmentSection({
  section,
  project,
  canApply = false,
  isMember = false,
  isAuthenticated = false,
  isVerified = false,
  onApplyClick,
  className,
}: RecruitmentSectionProps) {
  const statusStyles = getRecruitmentStatusBadgeClasses(project.recruitment_status);

  const statement =
    section.content ||
    (typeof section.config?.statement === "string" && section.config.statement) ||
    "We are actively seeking passionate student contributors, engineers, and researchers to join this campus workspace.";

  const roles = Array.isArray(section.config?.roles) ? (section.config.roles as string[]) : [];

  const skills = Array.isArray(section.config?.skills) ? (section.config.skills as string[]) : [];

  const isOpen = project.recruitment_status === "open";
  const isPaused = project.recruitment_status === "paused";

  return (
    <section
      id="recruitment-section"
      className={cn("rounded-xl border border-border bg-card p-6 shadow-none", className)}
    >
      {/* Section Header */}
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-2">
          <Briefcase className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold text-foreground">
            {section.title || "Recruitment & Open Roles"}
          </h2>
        </div>

        {/* Recruitment Status Pill */}
        <span
          className={cn(
            "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-mono uppercase tracking-wider",
            statusStyles.bg,
            statusStyles.text,
            statusStyles.border
          )}
        >
          <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
          {statusStyles.label}
        </span>
      </div>

      {/* Main Statement */}
      <div className="mt-4">
        <p className="text-sm text-foreground/90 leading-relaxed whitespace-pre-line">
          {statement}
        </p>
      </div>

      {/* Open Roles Listing if configured */}
      {roles.length > 0 && (
        <div className="mt-5 border-t border-border/60 pt-4">
          <h3 className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
            Target Roles &amp; Opportunities
          </h3>
          <div className="mt-2.5 flex flex-wrap gap-2">
            {roles.map((role, idx) => (
              <Badge
                key={idx}
                variant="outline"
                className="font-mono text-xs border-border bg-muted/40 py-1"
              >
                {role}
              </Badge>
            ))}
          </div>
        </div>
      )}

      {/* Required Skills if configured */}
      {skills.length > 0 && (
        <div className="mt-4">
          <h3 className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
            Desired Skills &amp; Background
          </h3>
          <div className="mt-2.5 flex flex-wrap gap-1.5">
            {skills.map((skill, idx) => (
              <span
                key={idx}
                className="inline-flex items-center rounded-md border border-border/80 bg-surface px-2.5 py-0.5 font-mono text-[11px] text-foreground"
              >
                {skill}
              </span>
            ))}
          </div>
        </div>
      )}

      {/* Interactive Action Card */}
      <div className="mt-6 rounded-lg border border-border/80 bg-muted/30 p-4 sm:p-5">
        {isMember ? (
          <div className="flex items-center gap-3 text-emerald-600">
            <CheckCircle2 className="h-5 w-5 shrink-0" />
            <div>
              <p className="text-sm font-medium text-foreground">You are a team member</p>
              <p className="text-xs text-muted-foreground">
                You have active collaborator permissions on this workspace.
              </p>
            </div>
          </div>
        ) : isOpen ? (
          canApply ? (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div>
                <p className="text-sm font-medium text-foreground">
                  Interested in joining this project?
                </p>
                <p className="text-xs text-muted-foreground">
                  Submit a brief statement of interest and your academic background.
                </p>
              </div>
              <Button
                type="button"
                onClick={onApplyClick}
                className="inline-flex shrink-0 items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98]"
              >
                <Sparkles className="h-3.5 w-3.5" />
                <span>Apply to Join Team</span>
              </Button>
            </div>
          ) : !isAuthenticated ? (
            <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
              <div className="flex items-center gap-3">
                <Lock className="h-5 w-5 text-muted-foreground shrink-0" />
                <div>
                  <p className="text-sm font-medium text-foreground">
                    Campus authentication required
                  </p>
                  <p className="text-xs text-muted-foreground">
                    Sign in with your verified university account to submit an application.
                  </p>
                </div>
              </div>
              <Link
                href="/login"
                className="inline-flex shrink-0 items-center justify-center rounded-md bg-[#111111] px-4 py-2 text-xs font-medium text-white hover:bg-[#222222] transition-colors"
              >
                Sign In to Apply
              </Link>
            </div>
          ) : !isVerified ? (
            <div className="flex items-start gap-3 text-pastel-yellowText">
              <ShieldAlert className="h-5 w-5 shrink-0 mt-0.5" />
              <div>
                <p className="text-sm font-medium text-foreground">
                  University email verification required
                </p>
                <p className="text-xs text-muted-foreground mt-0.5">
                  Only students with verified institutional email addresses can apply for campus
                  project workspaces. Please verify your email in your profile.
                </p>
              </div>
            </div>
          ) : (
            <div className="text-xs text-muted-foreground">
              Applications are currently open for verified campus members.
            </div>
          )
        ) : isPaused ? (
          <div className="flex items-center gap-3 text-amber-600">
            <PauseCircle className="h-5 w-5 shrink-0" />
            <div>
              <p className="text-sm font-medium text-foreground">Recruitment currently paused</p>
              <p className="text-xs text-muted-foreground">
                The project leads have temporarily paused reviewing new applications.
              </p>
            </div>
          </div>
        ) : (
          <div className="flex items-center gap-3 text-muted-foreground">
            <XCircle className="h-5 w-5 shrink-0" />
            <div>
              <p className="text-sm font-medium text-foreground">Recruitment closed</p>
              <p className="text-xs text-muted-foreground">
                This project is not currently accepting new members.
              </p>
            </div>
          </div>
        )}
      </div>
    </section>
  );
}
