import React from "react";
import { Users, Calendar, GraduationCap } from "lucide-react";
import type { Project, ProjectMember, ProjectRole, ProjectSection } from "@/types/project";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { formatJobDate } from "@/lib/formatters";

export interface TeamSectionProps {
  section: ProjectSection;
  project: Project;
  members?: ProjectMember[];
  isLeadOrMaintainer?: boolean;
  className?: string;
  onManageTeam?: () => void;
}

function getInitials(name?: string): string {
  if (!name) return "U";
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

function getRoleBadgeClasses(role: ProjectRole): {
  bg: string;
  text: string;
  border: string;
  label: string;
} {
  switch (role) {
    case "lead":
      return {
        bg: "bg-purple-500/10",
        text: "text-purple-600",
        border: "border-purple-500/20",
        label: "Lead",
      };
    case "maintainer":
      return {
        bg: "bg-indigo-500/10",
        text: "text-indigo-600",
        border: "border-indigo-500/20",
        label: "Maintainer",
      };
    case "member":
      return {
        bg: "bg-emerald-500/10",
        text: "text-emerald-600",
        border: "border-emerald-500/20",
        label: "Member",
      };
    case "viewer":
    default:
      return {
        bg: "bg-muted",
        text: "text-muted-foreground",
        border: "border-border",
        label: "Viewer",
      };
  }
}

export function TeamSection({
  section,
  members = [],
  isLeadOrMaintainer = false,
  className,
  onManageTeam,
}: TeamSectionProps) {
  const activeMembers = members.filter((m) => !m.status || m.status === "active");

  return (
    <section
      id="team-section"
      className={cn("rounded-xl border border-border bg-card p-6 shadow-none", className)}
    >
      {/* Section Header */}
      <div className="flex items-center justify-between border-b border-border pb-4">
        <div className="flex items-center gap-2">
          <Users className="h-4 w-4 text-muted-foreground" />
          <h2 className="text-base font-semibold text-foreground">
            {section.title || "Project Team"}
          </h2>
        </div>
        <div className="flex items-center gap-2">
          <Badge variant="outline" className="font-mono text-xs">
            {activeMembers.length} {activeMembers.length === 1 ? "member" : "members"}
          </Badge>
          {isLeadOrMaintainer && onManageTeam && (
            <button
              type="button"
              onClick={onManageTeam}
              className="inline-flex items-center gap-1 rounded-md border border-border bg-surface px-2.5 py-1 font-mono text-xs font-medium text-foreground hover:bg-muted transition-colors"
            >
              <span>Manage Team</span>
            </button>
          )}
        </div>
      </div>

      {/* Team Roster Grid */}
      {activeMembers.length === 0 ? (
        <div className="py-8 text-center">
          <Users className="mx-auto h-8 w-8 text-muted-foreground/60" />
          <p className="mt-2 text-sm text-muted-foreground">No team members listed yet.</p>
        </div>
      ) : (
        <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {activeMembers.map((member) => {
            const firstName = member.member?.first_name || member.first_name || "";
            const lastName = member.member?.last_name || member.last_name || "";
            const fullName = `${firstName} ${lastName}`.trim() || member.email || "Team Member";
            const initials = getInitials(fullName);
            const department = member.member?.department || member.department;
            const roleStyles = getRoleBadgeClasses(member.role);
            const joinedDate = member.joined_at ? formatJobDate(member.joined_at) : null;

            return (
              <div
                key={member.id || member.member_id}
                className="flex flex-col justify-between rounded-lg border border-border/80 bg-muted/20 p-4 transition-colors hover:border-foreground/30 hover:bg-muted/40"
              >
                <div>
                  {/* Top row: Avatar & Role */}
                  <div className="flex items-center justify-between">
                    <div className="flex h-10 w-10 items-center justify-center rounded-full border border-border bg-surface font-mono text-xs font-semibold text-foreground">
                      {initials}
                    </div>
                    <span
                      className={cn(
                        "inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-mono font-medium",
                        roleStyles.bg,
                        roleStyles.text,
                        roleStyles.border
                      )}
                    >
                      {roleStyles.label}
                    </span>
                  </div>

                  {/* Member Name */}
                  <div className="mt-3">
                    <p className="text-sm font-medium text-foreground truncate">{fullName}</p>
                    {member.email && member.email !== fullName && (
                      <p className="font-mono text-xs text-muted-foreground truncate">
                        {member.email}
                      </p>
                    )}
                  </div>

                  {/* Department */}
                  {department && (
                    <div className="mt-2 flex items-center gap-1.5 text-xs text-muted-foreground">
                      <GraduationCap className="h-3.5 w-3.5 shrink-0" />
                      <span className="truncate">{department}</span>
                    </div>
                  )}

                  {/* Bio */}
                  {member.member?.bio && (
                    <p className="mt-2 text-xs text-muted-foreground line-clamp-2">
                      {member.member.bio}
                    </p>
                  )}
                </div>

                {/* Footer: Joined date */}
                {joinedDate && (
                  <div className="mt-4 flex items-center gap-1.5 border-t border-border/60 pt-2.5 font-mono text-[11px] text-muted-foreground">
                    <Calendar className="h-3 w-3 shrink-0" />
                    <span>Joined {joinedDate}</span>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </section>
  );
}
