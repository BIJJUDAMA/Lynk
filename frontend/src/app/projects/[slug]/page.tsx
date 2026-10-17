"use client";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Users,
  Calendar,
  Sparkles,
  Lock,
  Globe,
  Link2,
  ShieldAlert,
  ShieldCheck,
  AlertCircle,
  RotateCcw,
  Loader2,
  CheckCircle2,
  Settings,
  ClipboardList,
} from "lucide-react";
import type {
  Project,
  ProjectMember,
  ProjectResource,
  ProjectSection,
  ProjectRole,
  SubmitApplicationPayload,
} from "@/types/project";
import {
  getProject,
  getProjectSections,
  listProjectMembers,
  listProjectResources,
  submitProjectApplication,
} from "@/lib/project-api";
import { ApiClientError } from "@/lib/api";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatProjectDate,
  getRecruitmentStatusBadgeClasses,
  getLifecycleBadgeClasses,
} from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { Skeleton } from "@/components/ui/skeleton";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";
import { SectionRenderer } from "@/components/project/SectionRenderer";
import { TeamManagementModal } from "@/components/project/TeamManagementModal";

export default function ProjectWorkspacePage() {
  const params = useParams();
  const slug = typeof params?.slug === "string" ? params.slug : "";

  const {
    user,
    backendUser,
    isAuthenticated,
    isVerified,
    isLoading: authLoading,
    login,
  } = useAuth();

  // Workspace Data State
  const [project, setProject] = useState<Project | null>(null);
  const [sections, setSections] = useState<ProjectSection[]>([]);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [resources, setResources] = useState<ProjectResource[]>([]);

  // Loading & Error State
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [errorCode, setErrorCode] = useState<number | null>(null);

  // Application Modal State
  const [isApplyModalOpen, setIsApplyModalOpen] = useState<boolean>(false);
  const [applyStatement, setApplyStatement] = useState<string>("");
  const [isSubmittingApp, setIsSubmittingApp] = useState<boolean>(false);
  const [applyError, setApplyError] = useState<string | null>(null);
  const [applySuccess, setApplySuccess] = useState<boolean>(false);

  // Team Management Modal State
  const [isTeamModalOpen, setIsTeamModalOpen] = useState<boolean>(false);

  // Fetch all workspace details
  const fetchWorkspace = useCallback(async () => {
    if (!slug) return;
    setIsLoading(true);
    setErrorMessage(null);
    setErrorCode(null);

    try {
      const proj = await getProject(slug);
      setProject(proj);

      // Concurrently fetch sections, members, and resources
      const [secRes, memRes, resRes] = await Promise.allSettled([
        getProjectSections(proj.id),
        listProjectMembers(proj.id),
        listProjectResources(proj.id),
      ]);

      if (secRes.status === "fulfilled") {
        setSections(secRes.value || []);
      } else {
        setSections([]);
      }

      if (memRes.status === "fulfilled") {
        setMembers(memRes.value || []);
      } else {
        setMembers([]);
      }

      if (resRes.status === "fulfilled") {
        setResources(resRes.value || []);
      } else {
        setResources([]);
      }
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setErrorCode(err.status);
        setErrorMessage(err.message);
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage("Failed to load project workspace.");
      }
    } finally {
      setIsLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    void fetchWorkspace();
  }, [fetchWorkspace]);

  // Viewer Permission Detection
  const { isMember, isLead, isLeadOrMaintainer, canApply } = useMemo(() => {
    if (!project) {
      return {
        isOwner: false,
        isMember: false,
        isLead: false,
        viewerRole: null as ProjectRole | null,
        isLeadOrMaintainer: false,
        canApply: false,
      };
    }

    const currentUserId = user?.id || backendUser?.id;
    const currentUserEmail = user?.email?.toLowerCase();

    const ownerMatch = Boolean(
      currentUserId && project.owner_member_id && project.owner_member_id === currentUserId
    );

    const memberMatch = members.find((m) => {
      if (currentUserId && m.member_id === currentUserId) return true;
      if (currentUserEmail && m.email && m.email.toLowerCase() === currentUserEmail) {
        return true;
      }
      return false;
    });

    const memberActive = Boolean(
      memberMatch && (!memberMatch.status || memberMatch.status === "active")
    );

    const memberRole: ProjectRole | null = ownerMatch ? "lead" : memberMatch?.role || null;

    const isLeadUser = Boolean(ownerMatch || memberRole === "lead");

    const leadOrMaintainer = ownerMatch || memberRole === "lead" || memberRole === "maintainer";

    const applyAllowed =
      isAuthenticated &&
      isVerified &&
      !ownerMatch &&
      !memberActive &&
      project.recruitment_status === "open" &&
      project.lifecycle_status !== "archived";

    return {
      isOwner: ownerMatch,
      isMember: ownerMatch || memberActive,
      isLead: isLeadUser,
      viewerRole: memberRole,
      isLeadOrMaintainer: leadOrMaintainer,
      canApply: applyAllowed,
    };
  }, [project, members, user, backendUser, isAuthenticated, isVerified]);

  // Application submission handler
  const handleApplySubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!project) return;

    const trimmedStatement = applyStatement.trim();
    if (trimmedStatement.length < 10) {
      setApplyError("Please provide a statement of at least 10 characters.");
      return;
    }

    setIsSubmittingApp(true);
    setApplyError(null);

    const payload: SubmitApplicationPayload = {
      statement: trimmedStatement,
    };

    try {
      await submitProjectApplication(project.id, payload);
      setApplySuccess(true);
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setApplyError(err.message);
      } else if (err instanceof Error) {
        setApplyError(err.message);
      } else {
        setApplyError("Failed to submit application. Please try again.");
      }
    } finally {
      setIsSubmittingApp(false);
    }
  };

  const handleCloseApplyModal = () => {
    setIsApplyModalOpen(false);
    setApplyStatement("");
    setApplyError(null);
    setApplySuccess(false);
  };

  // Loading skeleton screen
  if (isLoading || authLoading) {
    return (
      <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8 space-y-8">
        <div>
          <Skeleton className="h-4 w-36" />
        </div>
        <div className="rounded-xl border border-border bg-card p-6 sm:p-8 space-y-4">
          <div className="flex gap-2">
            <Skeleton className="h-5 w-24 rounded-full" />
            <Skeleton className="h-5 w-20 rounded-full" />
            <Skeleton className="h-5 w-20 rounded-full" />
          </div>
          <Skeleton className="h-9 w-3/4" />
          <Skeleton className="h-4 w-full" />
          <Skeleton className="h-4 w-2/3" />
          <div className="pt-4 border-t border-border flex justify-between items-center">
            <Skeleton className="h-4 w-32" />
            <Skeleton className="h-9 w-28 rounded-md" />
          </div>
        </div>
        <div className="space-y-6">
          <Skeleton className="h-48 w-full rounded-xl" />
          <Skeleton className="h-48 w-full rounded-xl" />
        </div>
      </div>
    );
  }

  // Error screen (404, 403, or Network Error)
  if (errorMessage || !project) {
    const isNotFound = errorCode === 404;
    const isForbidden = errorCode === 403 || errorCode === 401;

    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            {isForbidden ? (
              <Lock className="h-6 w-6" />
            ) : (
              <AlertCircle className="h-6 w-6 text-pastel-redText" />
            )}
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal leading-snug text-foreground [text-wrap:balance]">
            {isNotFound
              ? "Project Workspace Not Found"
              : isForbidden
                ? "Access Restricted to Workspace"
                : "Unable to Load Project"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {isNotFound
              ? "The campus project workspace you requested does not exist or may have been unlisted."
              : isForbidden
                ? "This project workspace is set to private and is restricted to accepted team members and leads."
                : errorMessage || "An unexpected error occurred while loading this workspace."}
          </p>
          <div className="mt-6 flex flex-col sm:flex-row items-center justify-center gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => void fetchWorkspace()}
              className="inline-flex items-center gap-2"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              <span>Try Again</span>
            </Button>
            <Link
              href="/projects"
              className="inline-flex items-center justify-center rounded-md bg-[#111111] px-4 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors"
            >
              Browse Campus Projects
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Styles and Badges
  const recruitmentStyles = getRecruitmentStatusBadgeClasses(project.recruitment_status);
  const lifecycleStyles = getLifecycleBadgeClasses(project.lifecycle_status);

  const leadName =
    (typeof project.settings?.lead_name === "string" && project.settings.lead_name.trim()) ||
    (typeof project.settings?.owner_name === "string" && project.settings.owner_name.trim()) ||
    "Campus Lead";

  const updatedLabel = formatProjectDate(project.updated_at || project.created_at, "Updated");

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8 space-y-8">
      {/* Top Navigation & Breadcrumb */}
      <div>
        <Link
          href="/projects"
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to Campus Projects</span>
        </Link>
      </div>

      {/* Workspace Header Card */}
      <header className="rounded-xl border border-border bg-card p-6 sm:p-8 shadow-none">
        {/* Badges Row */}
        <div className="flex flex-wrap items-center gap-2">
          {project.category && (
            <Badge variant="outline" className="font-mono text-xs">
              {project.category}
            </Badge>
          )}

          {/* Recruitment Status Pill */}
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

          {/* Lifecycle Status Pill */}
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

          {/* Visibility Pill */}
          {project.visibility === "unlisted" && (
            <span className="inline-flex items-center gap-1 rounded-full border border-dashed border-border px-2 py-0.5 text-[11px] font-mono text-muted-foreground">
              <Link2 className="h-3 w-3" />
              Unlisted Workspace
            </span>
          )}
          {project.visibility === "private" && (
            <span className="inline-flex items-center gap-1 rounded-full border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-muted-foreground">
              <Lock className="h-3 w-3" />
              Private Workspace
            </span>
          )}
          {project.visibility === "public" && (
            <span className="inline-flex items-center gap-1 rounded-full border border-border bg-surface px-2 py-0.5 text-[11px] font-mono text-muted-foreground">
              <Globe className="h-3 w-3" />
              Public
            </span>
          )}
        </div>

        {/* Project Title */}
        <h1 className="mt-4 font-serif text-3xl sm:text-4xl font-normal leading-tight text-foreground [text-wrap:balance]">
          {project.title}
        </h1>

        {/* Summary Description */}
        <p className="mt-3 text-sm text-muted-foreground leading-relaxed max-w-3xl">
          {project.description}
        </p>

        {/* Header Footer: Metadata & Actions */}
        <div className="mt-6 flex flex-col gap-4 border-t border-border pt-4 sm:flex-row sm:items-center sm:justify-between">
          {/* Left Metadata: Owner & Date */}
          <div className="flex flex-wrap items-center gap-4 text-xs font-mono text-muted-foreground">
            <div className="flex items-center gap-1.5 text-foreground">
              <Users className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              <span className="font-medium">{leadName}</span>
            </div>
            <div className="flex items-center gap-1.5">
              <Calendar className="h-3.5 w-3.5 shrink-0" />
              <span>{updatedLabel}</span>
            </div>
          </div>

          {/* Right Action Buttons */}
          <div className="flex flex-wrap items-center gap-2">
            {isLeadOrMaintainer && (
              <>
                <Button
                  type="button"
                  variant="outline"
                  onClick={() => setIsTeamModalOpen(true)}
                  className="inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-3 py-2 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors shadow-none h-auto"
                >
                  <Users className="h-3.5 w-3.5" />
                  <span>Manage Team</span>
                </Button>
                <Link
                  href={`/projects/${encodeURIComponent(project.slug || project.id)}/applications`}
                  className="inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-3 py-2 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors shadow-none"
                >
                  <ClipboardList className="h-3.5 w-3.5" />
                  <span>Applications</span>
                </Link>
                <Link
                  href={`/projects/${encodeURIComponent(project.slug || project.id)}/settings`}
                  className="inline-flex items-center gap-1.5 rounded-md border border-border bg-card px-3 py-2 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors shadow-none"
                >
                  <Settings className="h-3.5 w-3.5" />
                  <span>Manage Workspace</span>
                </Link>
              </>
            )}

            {isMember && !isLeadOrMaintainer && (
              <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-3 py-1 font-mono text-xs text-emerald-600">
                <CheckCircle2 className="h-3.5 w-3.5" />
                Team Member
              </span>
            )}

            {canApply && (
              <Button
                type="button"
                onClick={() => setIsApplyModalOpen(true)}
                className="inline-flex items-center gap-1.5 rounded-md bg-[#111111] px-4 py-2 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98]"
              >
                <Sparkles className="h-3.5 w-3.5" />
                <span>Apply to Join</span>
              </Button>
            )}

            {!isAuthenticated && project.recruitment_status === "open" && (
              <button
                type="button"
                onClick={() =>
                  login({
                    redirectPath: `/projects/${project.slug || project.id}`,
                  })
                }
                className="inline-flex items-center gap-1.5 rounded-md bg-[#111111] px-4 py-2 text-xs font-medium text-white hover:bg-[#222222] transition-colors"
              >
                <span>Sign In to Apply</span>
              </button>
            )}

            {isAuthenticated &&
              !isVerified &&
              !isMember &&
              project.recruitment_status === "open" && (
                <span className="inline-flex items-center gap-1.5 rounded-md border border-pastel-yellowText/30 bg-pastel-yellow px-2.5 py-1 text-xs font-mono text-pastel-yellowText">
                  <ShieldAlert className="h-3.5 w-3.5 shrink-0" />
                  <span>Verify Email to Apply</span>
                </span>
              )}
          </div>
        </div>
      </header>

      {/* Main Ordered Workspace Sections */}
      <main>
        <SectionRenderer
          sections={sections}
          project={project}
          members={members}
          resources={resources}
          isLeadOrMaintainer={isLeadOrMaintainer}
          isMember={isMember}
          canApply={canApply}
          isAuthenticated={isAuthenticated}
          isVerified={isVerified}
          onApplyClick={() => setIsApplyModalOpen(true)}
          onManageTeam={() => setIsTeamModalOpen(true)}
        />
      </main>

      {/* Application Submission Dialog Modal */}
      <Dialog open={isApplyModalOpen} onOpenChange={handleCloseApplyModal}>
        <DialogContent onClose={handleCloseApplyModal} className="max-w-lg">
          <DialogHeader>
            <DialogTitle>Apply to Join Project</DialogTitle>
            <DialogDescription>
              Submit your statement of interest for{" "}
              <span className="font-semibold text-foreground">{project.title}</span>. The project
              leads will review your application.
            </DialogDescription>
          </DialogHeader>

          {applySuccess ? (
            <div className="py-6 text-center space-y-4">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-600">
                <CheckCircle2 className="h-6 w-6" />
              </div>
              <div>
                <h3 className="font-serif text-lg font-medium text-foreground">
                  Application Submitted
                </h3>
                <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto">
                  Your application has been delivered to the project leads. You will be notified
                  when your status is updated.
                </p>
              </div>
              <div className="pt-2">
                <Button
                  type="button"
                  onClick={handleCloseApplyModal}
                  className="rounded-md bg-[#111111] px-5 py-2 text-xs font-medium text-white hover:bg-[#222222]"
                >
                  Close
                </Button>
              </div>
            </div>
          ) : (
            <form onSubmit={handleApplySubmit} className="space-y-4">
              {applyError && (
                <div className="flex items-start gap-2 rounded-lg border border-pastel-redText/20 bg-pastel-red p-3 text-xs text-pastel-redText">
                  <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
                  <span>{applyError}</span>
                </div>
              )}

              <div>
                <div className="flex items-center justify-between">
                  <label
                    htmlFor="application-statement"
                    className="block text-xs font-medium text-foreground"
                  >
                    Statement of Interest <span className="text-pastel-redText">*</span>
                  </label>
                  <span className="font-mono text-[11px] text-muted-foreground">
                    {applyStatement.length} chars (min 10)
                  </span>
                </div>
                <Textarea
                  id="application-statement"
                  rows={5}
                  value={applyStatement}
                  onChange={(e) => setApplyStatement(e.target.value)}
                  placeholder="Share your relevant background, projects, or why you want to collaborate on this initiative..."
                  className="mt-1.5"
                  required
                />
              </div>

              <div className="rounded-lg border border-border/80 bg-muted/20 p-3 flex items-start gap-2.5 text-xs text-muted-foreground">
                <ShieldCheck className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
                <span>
                  Signed in as <span className="font-mono text-foreground">{user?.email}</span>.
                  Your institutional verification status will be attached to this submission.
                </span>
              </div>

              <DialogFooter>
                <Button
                  type="button"
                  variant="outline"
                  onClick={handleCloseApplyModal}
                  disabled={isSubmittingApp}
                >
                  Cancel
                </Button>
                <Button
                  type="submit"
                  disabled={isSubmittingApp}
                  className="inline-flex items-center gap-2 rounded-md bg-[#111111] px-4 py-2 text-xs font-medium text-white hover:bg-[#222222]"
                >
                  {isSubmittingApp ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      <span>Submitting...</span>
                    </>
                  ) : (
                    <>
                      <Sparkles className="h-3.5 w-3.5" />
                      <span>Submit Application</span>
                    </>
                  )}
                </Button>
              </DialogFooter>
            </form>
          )}
        </DialogContent>
      </Dialog>

      {/* Team Management Modal */}
      {project && (
        <TeamManagementModal
          open={isTeamModalOpen}
          onClose={() => setIsTeamModalOpen(false)}
          project={project}
          members={members}
          currentUserId={user?.id || backendUser?.id}
          currentUserEmail={user?.email}
          isLead={Boolean(isLead)}
          onMembersChange={() => void fetchWorkspace()}
        />
      )}
    </div>
  );
}
