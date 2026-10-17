"use client";

import React, { useState, useMemo, useCallback, Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  FolderGit2,
  FileText,
  Briefcase,
  Users,
  Search,
  Lock,
  Loader2,
  ArrowRight,
  Plus,
  RotateCcw,
} from "lucide-react";
import type { Project, ProjectApplication, ProjectRole } from "@/types/project";
import type { Job } from "@/types/api";
import { listProjects, listMyApplications, getProject } from "@/lib/project-api";
import { getMyJobs } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatJobDate,
  formatProjectDate,
  getRecruitmentStatusBadgeClasses,
  getLifecycleBadgeClasses,
  getStatusBadgeClasses,
} from "@/lib/formatters";
import { getApplicationStatusBadgeClasses } from "@/lib/recruitment";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

type ActivityTab = "projects" | "applications" | "postings";

export interface WorkspaceItem {
  project: Project;
  role: ProjectRole;
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
        bg: "bg-ink text-canvas",
        text: "text-canvas",
        border: "border-ink",
        label: "Lead",
      };
    case "maintainer":
      return {
        bg: "bg-pastel-blue border-pastel-blueText/20",
        text: "text-pastel-blueText",
        border: "border-pastel-blueText/20",
        label: "Maintainer",
      };
    case "member":
      return {
        bg: "bg-pastel-green border-pastel-greenText/20",
        text: "text-pastel-greenText",
        border: "border-pastel-greenText/20",
        label: "Member",
      };
    case "viewer":
    default:
      return {
        bg: "bg-muted border-border",
        text: "text-muted-foreground",
        border: "border-border",
        label: "Viewer",
      };
  }
}

function ActivityContent() {
  const searchParams = useSearchParams();
  const rawTab = searchParams.get("tab");
  const initialTab: ActivityTab =
    rawTab === "applications"
      ? "applications"
      : rawTab === "postings" || rawTab === "jobs"
        ? "postings"
        : "projects";

  const { user, isAuthenticated, isLoading: authLoading, login } = useAuth();

  const [activeTab, setActiveTab] = useState<ActivityTab>(initialTab);
  const [searchQuery, setSearchQuery] = useState("");

  // 1. Fetch My Projects (workspaces where user is Lead, Maintainer, or Member)
  const {
    data: myProjects,
    isLoading: isProjectsLoading,
    error: projectsError,
    refetch: refetchProjects,
  } = useQuery<WorkspaceItem[]>(
    useCallback(
      async (client) => {
        if (!user?.id) return [];

        const workspaces: WorkspaceItem[] = [];
        const seenProjectIds = new Set<string>();

        // Query projects owned by current user
        try {
          const owned = await listProjects({ owner_id: user.id }, client);
          for (const proj of owned || []) {
            if (!seenProjectIds.has(proj.id)) {
              seenProjectIds.add(proj.id);
              workspaces.push({ project: proj, role: "lead" });
            }
          }
        } catch {
          // If owner query fails or returns empty, proceed with application-linked workspaces
        }

        // Discover projects where user was accepted as team member
        try {
          const apps = await listMyApplications(client);
          const acceptedApps = (apps || []).filter(
            (app) => app.status === "accepted" && !seenProjectIds.has(app.project_id)
          );

          if (acceptedApps.length > 0) {
            const acceptedWorkspaces = await Promise.all(
              acceptedApps.map(async (app) => {
                try {
                  const proj = await getProject(app.project_id, client);
                  return { project: proj, role: "member" as const };
                } catch {
                  return null;
                }
              })
            );

            for (const item of acceptedWorkspaces) {
              if (item && !seenProjectIds.has(item.project.id)) {
                seenProjectIds.add(item.project.id);
                workspaces.push(item);
              }
            }
          }
        } catch {
          // Continue if application query fails
        }

        return workspaces;
      },
      [user?.id]
    ),
    {
      enabled: Boolean(isAuthenticated && user?.id),
    }
  );

  // 2. Fetch Project Applications submitted by the user
  const {
    data: applications,
    isLoading: isAppsLoading,
    error: appsError,
    refetch: refetchApps,
  } = useQuery<ProjectApplication[]>(
    useCallback((client) => listMyApplications(client), []),
    {
      enabled: Boolean(isAuthenticated),
    }
  );

  // 3. Fetch Legacy Job Postings (for graceful backward compatibility)
  const {
    data: myJobs,
    isLoading: isJobsLoading,
    error: jobsError,
    refetch: refetchJobs,
  } = useQuery<Job[]>(
    useCallback((client) => getMyJobs(client), []),
    {
      enabled: Boolean(isAuthenticated),
    }
  );

  const projectsList = useMemo(() => myProjects ?? [], [myProjects]);
  const appsList = useMemo(() => applications ?? [], [applications]);
  const jobsList = useMemo(() => myJobs ?? [], [myJobs]);

  // Filtered Projects
  const filteredProjects = useMemo(() => {
    if (!searchQuery.trim()) return projectsList;
    const q = searchQuery.toLowerCase().trim();
    return projectsList.filter(({ project }) => {
      const title = project.title?.toLowerCase() || "";
      const desc = project.description?.toLowerCase() || "";
      const cat = project.category?.toLowerCase() || "";
      return title.includes(q) || desc.includes(q) || cat.includes(q);
    });
  }, [projectsList, searchQuery]);

  // Filtered Applications
  const filteredApplications = useMemo(() => {
    if (!searchQuery.trim()) return appsList;
    const q = searchQuery.toLowerCase().trim();
    return appsList.filter((app) => {
      const title = app.project_title?.toLowerCase() || "";
      const stmt = app.statement?.toLowerCase() || "";
      const status = app.status?.toLowerCase() || "";
      return title.includes(q) || stmt.includes(q) || status.includes(q);
    });
  }, [appsList, searchQuery]);

  // Filtered Legacy Jobs
  const filteredJobs = useMemo(() => {
    if (!searchQuery.trim()) return jobsList;
    const q = searchQuery.toLowerCase().trim();
    return jobsList.filter((job) => {
      const title = job.title?.toLowerCase() || "";
      const dept = job.department?.toLowerCase() || "";
      return title.includes(q) || dept.includes(q);
    });
  }, [jobsList, searchQuery]);

  // Loading State
  if (authLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // Unauthenticated Gate
  if (!isAuthenticated || !user) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 text-center">
        <div className="rounded-xl border border-border bg-card p-8 shadow-none space-y-4">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            <Lock className="h-6 w-6" />
          </div>
          <h1 className="font-serif text-2xl font-normal leading-snug text-foreground">
            Campus Sign In Required
          </h1>
          <p className="text-xs text-muted-foreground max-w-md mx-auto">
            Sign in with your university account to access your activity hub: project workspaces and
            team applications.
          </p>
          <div className="pt-2">
            <Button onClick={() => login({ redirectPath: "/activity" })} className="text-xs">
              Sign In with Campus Account
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8 space-y-8">
      {/* Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between border-b border-border pb-6">
        <div>
          <h1 className="font-serif text-3xl sm:text-4xl font-normal leading-[1.2] text-foreground">
            Activity Hub
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            Manage your campus project workspaces, team roles, and recruitment applications.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-2.5">
          <Button asChild variant="outline" size="sm" className="text-xs">
            <Link href="/projects">
              <span>Browse Projects</span>
            </Link>
          </Button>
          <Button asChild size="sm" className="text-xs">
            <Link href="/projects/create">
              <Plus className="h-3.5 w-3.5 mr-1.5" />
              <span>Start a Project</span>
            </Link>
          </Button>
        </div>
      </div>

      {/* Tab Strip & Search Bar */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between border-b border-border pb-1">
        {/* Horizontal Underline Tabs */}
        <div className="flex items-end gap-1 -mb-1 overflow-x-auto scrollbar-none w-full sm:w-auto">
          <button
            type="button"
            onClick={() => setActiveTab("projects")}
            className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${
              activeTab === "projects"
                ? "border-foreground text-foreground font-semibold"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            My Projects
            <span className="ml-1.5 font-mono text-[11px] text-muted-foreground">
              ({projectsList.length})
            </span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("applications")}
            className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${
              activeTab === "applications"
                ? "border-foreground text-foreground font-semibold"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            My Applications
            <span className="ml-1.5 font-mono text-[11px] text-muted-foreground">
              ({appsList.length})
            </span>
          </button>
          {jobsList.length > 0 && (
            <button
              type="button"
              onClick={() => setActiveTab("postings")}
              className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${
                activeTab === "postings"
                  ? "border-foreground text-foreground font-semibold"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              Legacy Jobs
              <span className="ml-1.5 font-mono text-[11px] text-muted-foreground">
                ({jobsList.length})
              </span>
            </button>
          )}
        </div>

        {/* Real-time Search */}
        <div className="relative w-full sm:w-64 pb-1">
          <Search className="pointer-events-none absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Filter list..."
            className="w-full rounded-md border border-border bg-card py-1.5 pl-8 pr-3 text-xs text-foreground placeholder:text-muted-foreground focus:border-foreground/50 focus:outline-none transition"
          />
        </div>
      </div>

      {/* Tab 1: My Projects */}
      {activeTab === "projects" && (
        <div className="space-y-4">
          {isProjectsLoading ? (
            <div className="flex min-h-[30vh] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : projectsError ? (
            <div className="rounded-xl border border-pastel-redText/20 bg-pastel-red p-6 text-center text-xs text-pastel-redText space-y-2">
              <p>Failed to load projects: {projectsError.message}</p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => refetchProjects()}
                className="text-xs"
              >
                <RotateCcw className="h-3 w-3 mr-1.5" />
                Retry
              </Button>
            </div>
          ) : filteredProjects.length === 0 ? (
            <div className="rounded-xl border border-border bg-card p-12 text-center shadow-none space-y-3">
              <FolderGit2 className="mx-auto h-8 w-8 text-muted-foreground" />
              <div>
                <h3 className="font-serif text-xl font-medium text-foreground">
                  No active project workspaces
                </h3>
                <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
                  You are not currently leading or collaborating on any projects. Start your own
                  project or browse campus initiatives.
                </p>
              </div>
              <div className="pt-2 flex flex-wrap justify-center gap-3">
                <Button asChild className="text-xs">
                  <Link href="/projects/create">
                    <Plus className="h-3.5 w-3.5 mr-1.5" />
                    <span>Start a Project</span>
                  </Link>
                </Button>
                <Button asChild variant="outline" className="text-xs">
                  <Link href="/projects">
                    <span>Browse Campus Projects</span>
                    <ArrowRight className="h-3.5 w-3.5 ml-1.5" />
                  </Link>
                </Button>
              </div>
            </div>
          ) : (
            filteredProjects.map(({ project, role }) => {
              const roleStyles = getRoleBadgeClasses(role);
              const recruitmentStyles = getRecruitmentStatusBadgeClasses(
                project.recruitment_status
              );
              const lifecycleStyles = getLifecycleBadgeClasses(project.lifecycle_status);
              const projectHref = `/projects/${project.slug || project.id}`;

              return (
                <div
                  key={project.id}
                  className="rounded-xl border border-border bg-card p-6 shadow-none transition hover:border-foreground/30 flex flex-col md:flex-row justify-between gap-5"
                >
                  <div className="space-y-2.5 flex-1">
                    {/* Badge Row: Role, Category, Visibility, Recruitment, Lifecycle */}
                    <div className="flex flex-wrap items-center gap-2">
                      {/* Role Pill */}
                      <span
                        className={cn(
                          "inline-flex items-center rounded-full border px-2.5 py-0.5 text-xs font-mono font-semibold uppercase tracking-wider",
                          roleStyles.bg,
                          roleStyles.border,
                          roleStyles.text
                        )}
                      >
                        {roleStyles.label}
                      </span>

                      {/* Category Badge */}
                      {project.category && (
                        <Badge variant="outline" className="font-mono text-xs">
                          {project.category}
                        </Badge>
                      )}

                      {/* Visibility Pill */}
                      <span className="inline-flex items-center rounded-full border border-border px-2 py-0.5 text-[11px] font-mono text-muted-foreground capitalize">
                        {project.visibility}
                      </span>

                      {/* Recruitment Status */}
                      <span
                        className={cn(
                          "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[11px] font-mono uppercase tracking-wider",
                          recruitmentStyles.bg,
                          recruitmentStyles.text,
                          recruitmentStyles.border
                        )}
                      >
                        <span className={cn("h-1.5 w-1.5 rounded-full", recruitmentStyles.dot)} />
                        {recruitmentStyles.label}
                      </span>

                      {/* Lifecycle Status */}
                      <span
                        className={cn(
                          "inline-flex items-center rounded-full border px-2 py-0.5 text-[11px] font-mono",
                          lifecycleStyles.bg,
                          lifecycleStyles.text,
                          lifecycleStyles.border
                        )}
                      >
                        {lifecycleStyles.label}
                      </span>
                    </div>

                    {/* Title */}
                    <Link href={projectHref}>
                      <h2 className="font-serif text-xl font-medium text-foreground hover:underline underline-offset-2 transition-colors">
                        {project.title}
                      </h2>
                    </Link>

                    {/* Description */}
                    {project.description && (
                      <p className="text-xs text-muted-foreground line-clamp-2 leading-relaxed">
                        {project.description}
                      </p>
                    )}

                    {/* Timestamp */}
                    <p className="font-mono text-[11px] text-muted-foreground pt-1">
                      {formatProjectDate(project.updated_at || project.created_at, "Last active")}
                    </p>
                  </div>

                  {/* Actions */}
                  <div className="flex flex-wrap items-center gap-2.5 shrink-0 pt-2 md:pt-0">
                    <Button asChild size="sm" className="text-xs">
                      <Link href={projectHref}>
                        <span>Open Workspace</span>
                        <ArrowRight className="h-3 w-3 ml-1.5" />
                      </Link>
                    </Button>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}

      {/* Tab 2: My Applications */}
      {activeTab === "applications" && (
        <div className="space-y-4">
          {isAppsLoading ? (
            <div className="flex min-h-[30vh] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : appsError ? (
            <div className="rounded-xl border border-pastel-redText/20 bg-pastel-red p-6 text-center text-xs text-pastel-redText space-y-2">
              <p>Failed to load applications: {appsError.message}</p>
              <Button variant="outline" size="sm" onClick={() => refetchApps()} className="text-xs">
                <RotateCcw className="h-3 w-3 mr-1.5" />
                Retry
              </Button>
            </div>
          ) : filteredApplications.length === 0 ? (
            <div className="rounded-xl border border-border bg-card p-12 text-center shadow-none space-y-3">
              <FileText className="mx-auto h-8 w-8 text-muted-foreground" />
              <div>
                <h3 className="font-serif text-xl font-medium text-foreground">
                  No applications submitted
                </h3>
                <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
                  You have not applied to any campus project teams yet. Browse open projects to find
                  an initiative that matches your skills.
                </p>
              </div>
              <div className="pt-2">
                <Button asChild className="text-xs">
                  <Link href="/projects">
                    <span>Browse Campus Projects</span>
                    <ArrowRight className="h-3.5 w-3.5 ml-1.5" />
                  </Link>
                </Button>
              </div>
            </div>
          ) : (
            filteredApplications.map((app) => {
              const statusStyles = getApplicationStatusBadgeClasses(app.status);
              const projectLink = `/projects/${app.project_id}`;

              return (
                <div
                  key={app.id}
                  className="rounded-xl border border-border bg-card p-6 shadow-none transition hover:border-foreground/30 flex flex-col md:flex-row justify-between gap-5"
                >
                  <div className="space-y-2.5 flex-1">
                    {/* Status and Project Link */}
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={cn(
                          "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-mono font-semibold uppercase tracking-wider",
                          statusStyles.bg,
                          statusStyles.text,
                          statusStyles.border
                        )}
                      >
                        <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
                        {statusStyles.label}
                      </span>
                    </div>

                    <Link href={projectLink}>
                      <h2 className="font-serif text-xl font-medium text-foreground hover:underline underline-offset-2 transition-colors">
                        {app.project_title || "Campus Project"}
                      </h2>
                    </Link>

                    <p className="font-mono text-xs text-muted-foreground">
                      Applied on {formatJobDate(app.created_at)}
                    </p>

                    {/* Statement Snippet */}
                    {app.statement && (
                      <p className="border-l-2 border-border pl-3 text-xs text-foreground/80 line-clamp-2 leading-relaxed pt-1">
                        {app.statement}
                      </p>
                    )}

                    {/* Reviewer Feedback Notes */}
                    {app.reviewer_notes && (
                      <div className="mt-2 rounded-md bg-muted/40 p-2.5 text-xs text-foreground/90 border border-border/50">
                        <span className="font-semibold text-muted-foreground font-mono uppercase text-[10px] block mb-0.5">
                          Team Feedback
                        </span>
                        {app.reviewer_notes}
                      </div>
                    )}
                  </div>

                  {/* Actions */}
                  <div className="flex flex-wrap items-center gap-2.5 shrink-0 pt-2 md:pt-0">
                    <Button asChild variant="outline" size="sm" className="text-xs">
                      <Link href={projectLink}>
                        <span>View Project</span>
                        <ArrowRight className="h-3 w-3 ml-1.5" />
                      </Link>
                    </Button>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}

      {/* Tab 3: Legacy Jobs (Graceful backward compatibility) */}
      {activeTab === "postings" && jobsList.length > 0 && (
        <div className="space-y-4">
          {isJobsLoading ? (
            <div className="flex min-h-[30vh] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : jobsError ? (
            <div className="rounded-xl border border-pastel-redText/20 bg-pastel-red p-6 text-center text-xs text-pastel-redText space-y-2">
              <p>Failed to load postings: {jobsError.message}</p>
              <Button variant="outline" size="sm" onClick={() => refetchJobs()} className="text-xs">
                <RotateCcw className="h-3 w-3 mr-1.5" />
                Retry
              </Button>
            </div>
          ) : filteredJobs.length === 0 ? (
            <div className="rounded-xl border border-border bg-card p-12 text-center shadow-none space-y-3">
              <Briefcase className="mx-auto h-8 w-8 text-muted-foreground" />
              <div>
                <h3 className="font-serif text-xl font-medium text-foreground">
                  No legacy postings match filter
                </h3>
              </div>
            </div>
          ) : (
            filteredJobs.map((job) => {
              const statusStyles = getStatusBadgeClasses(job.status);
              return (
                <div
                  key={job.id}
                  className="rounded-xl border border-border bg-card p-6 shadow-none transition hover:border-foreground/30 flex flex-col md:flex-row justify-between gap-5"
                >
                  <div className="space-y-2 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
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
                      {job.department && (
                        <Badge variant="secondary" className="font-mono text-xs">
                          {job.department}
                        </Badge>
                      )}
                    </div>

                    <Link href={`/jobs/${job.id}`}>
                      <h2 className="font-serif text-xl font-medium text-foreground hover:underline underline-offset-2 transition-colors">
                        {job.title}
                      </h2>
                    </Link>

                    <p className="font-mono text-xs text-muted-foreground">
                      Posted {formatJobDate(job.created_at)} - Deadline:{" "}
                      {formatJobDate(job.deadline)}
                    </p>
                  </div>

                  <div className="flex flex-wrap items-center gap-2.5 shrink-0 pt-2 md:pt-0">
                    <Button asChild size="sm" className="text-xs">
                      <Link href={`/jobs/${job.id}/applicants`}>
                        <Users className="h-3.5 w-3.5 mr-1.5" />
                        <span>Review Applicants</span>
                      </Link>
                    </Button>
                    <Button asChild variant="outline" size="sm" className="text-xs">
                      <Link href={`/jobs/${job.id}`}>
                        <span>View Listing</span>
                        <ArrowRight className="h-3 w-3 ml-1.5" />
                      </Link>
                    </Button>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}
    </div>
  );
}

export default function ActivityPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[60vh] items-center justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <ActivityContent />
    </Suspense>
  );
}
