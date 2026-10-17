"use client";

import React, { useState, useEffect, useCallback, useMemo } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Search,
  Users,
  CheckCircle2,
  Clock,
  Star,
  XCircle,
  FileText,
  Lock,
  AlertCircle,
  RotateCcw,
  UserCheck,
  Loader2,
  Mail,
  GraduationCap,
  Calendar,
} from "lucide-react";
import type {
  Project,
  ProjectMember,
  ProjectApplication,
  ApplicationStatus,
} from "@/types/project";
import {
  getProject,
  listProjectMembers,
  listProjectApplications,
  acceptProjectApplication,
  shortlistProjectApplication,
  rejectProjectApplication,
} from "@/lib/project-api";
import { ApiClientError } from "@/lib/api";
import { useAuth } from "@/components/auth/AuthProvider";
import { formatJobDate } from "@/lib/formatters";
import {
  getApplicantDisplayName,
  getApplicantEmail,
  getApplicantDepartment,
  getApplicantInitials,
  getApplicationStatusBadgeClasses,
  computeApplicationMetrics,
  filterApplications,
} from "@/lib/recruitment";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Skeleton } from "@/components/ui/skeleton";
import { ApplicantReviewModal } from "@/components/project/ApplicantReviewModal";

type StatusFilterOption = ApplicationStatus | "all";

export default function ProjectApplicationsPage() {
  const params = useParams();
  const slug = typeof params?.slug === "string" ? params.slug : "";

  const { user, backendUser, isLoading: authLoading } = useAuth();

  // Project & Applications State
  const [project, setProject] = useState<Project | null>(null);
  const [members, setMembers] = useState<ProjectMember[]>([]);
  const [applications, setApplications] = useState<ProjectApplication[]>([]);

  // Loading & Error State
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isRefreshing, setIsRefreshing] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);
  const [errorCode, setErrorCode] = useState<number | null>(null);

  // Filter & Search State
  const [searchQuery, setSearchQuery] = useState<string>("");
  const [statusFilter, setStatusFilter] = useState<StatusFilterOption>("all");

  // Modal Review State
  const [selectedApplication, setSelectedApplication] = useState<ProjectApplication | null>(null);
  const [isReviewModalOpen, setIsReviewModalOpen] = useState<boolean>(false);

  // Quick action in-flight tracking
  const [quickActionId, setQuickActionId] = useState<string | null>(null);
  const [quickActionError, setQuickActionError] = useState<string | null>(null);

  // Fetch project, members, and applications
  const loadDashboardData = useCallback(async () => {
    if (!slug) return;
    setIsLoading(true);
    setErrorMessage(null);
    setErrorCode(null);

    try {
      const proj = await getProject(slug);
      setProject(proj);

      const [membersData, applicationsData] = await Promise.all([
        listProjectMembers(proj.id),
        listProjectApplications(proj.id),
      ]);

      setMembers(membersData || []);
      setApplications(applicationsData || []);
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setErrorCode(err.status);
        setErrorMessage(err.message);
      } else if (err instanceof Error) {
        setErrorMessage(err.message);
      } else {
        setErrorMessage("Failed to load recruitment dashboard.");
      }
    } finally {
      setIsLoading(false);
    }
  }, [slug]);

  useEffect(() => {
    void loadDashboardData();
  }, [loadDashboardData]);

  // Refresh applications list
  const refreshApplications = async () => {
    if (!project) return;
    setIsRefreshing(true);
    try {
      const apps = await listProjectApplications(project.id);
      setApplications(apps || []);
    } catch {
      // Ignore background refresh failure
    } finally {
      setIsRefreshing(false);
    }
  };

  // Permission Verification (Lead or Maintainer)
  const isAuthorized = useMemo(() => {
    if (!project) return false;

    const currentUserId = user?.id || backendUser?.id;
    const currentUserEmail = user?.email?.toLowerCase();

    // Check if user is project owner
    const isOwner = Boolean(
      currentUserId && project.owner_member_id && project.owner_member_id === currentUserId
    );
    if (isOwner) return true;

    // Check if user is in members list as lead or maintainer
    const memberMatch = members.find((m) => {
      if (currentUserId && m.member_id === currentUserId) return true;
      if (currentUserEmail && m.email && m.email.toLowerCase() === currentUserEmail) {
        return true;
      }
      return false;
    });

    if (memberMatch && (!memberMatch.status || memberMatch.status === "active")) {
      return memberMatch.role === "lead" || memberMatch.role === "maintainer";
    }

    return false;
  }, [project, members, user, backendUser]);

  // Aggregate Metrics
  const metrics = useMemo(() => {
    return computeApplicationMetrics(applications);
  }, [applications]);

  // Filtered Applications
  const filteredApplications = useMemo(() => {
    return filterApplications(applications, searchQuery, statusFilter);
  }, [applications, searchQuery, statusFilter]);

  // Handle Review Modal Open
  const handleOpenReview = (app: ProjectApplication) => {
    setSelectedApplication(app);
    setIsReviewModalOpen(true);
  };

  // Handle Application Decision from Modal or Inline
  const handleApplicationUpdated = (updatedApp: ProjectApplication) => {
    setApplications((prev) => prev.map((app) => (app.id === updatedApp.id ? updatedApp : app)));
    if (selectedApplication && selectedApplication.id === updatedApp.id) {
      setSelectedApplication(updatedApp);
    }
  };

  // Inline Quick Actions (Accept / Shortlist / Reject)
  const handleQuickDecision = async (
    app: ProjectApplication,
    decision: "accept" | "shortlist" | "reject"
  ) => {
    setQuickActionId(app.id);
    setQuickActionError(null);
    try {
      let updated: ProjectApplication;
      if (decision === "accept") {
        updated = await acceptProjectApplication(
          app.id,
          "Quick accepted from recruitment dashboard"
        );
      } else if (decision === "shortlist") {
        updated = await shortlistProjectApplication(app.id);
      } else {
        updated = await rejectProjectApplication(app.id);
      }
      handleApplicationUpdated(updated);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setQuickActionError(err.message);
      } else {
        setQuickActionError("Quick action failed.");
      }
    } finally {
      setQuickActionId(null);
    }
  };

  // Loading skeleton state
  if (isLoading || authLoading) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8 space-y-8">
        <div>
          <Skeleton className="h-4 w-40" />
        </div>
        <div className="space-y-3">
          <Skeleton className="h-9 w-64" />
          <Skeleton className="h-4 w-96" />
        </div>
        <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
          <Skeleton className="h-24 w-full rounded-xl" />
          <Skeleton className="h-24 w-full rounded-xl" />
          <Skeleton className="h-24 w-full rounded-xl" />
          <Skeleton className="h-24 w-full rounded-xl" />
        </div>
        <div className="space-y-4">
          <Skeleton className="h-10 w-full rounded-lg" />
          <Skeleton className="h-64 w-full rounded-xl" />
        </div>
      </div>
    );
  }

  // Error screen
  if (errorMessage || !project) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-pastel-redText">
            <AlertCircle className="h-6 w-6" />
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal text-foreground">
            {errorCode === 404 ? "Project Not Found" : "Unable to Load Recruitment Dashboard"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            {errorMessage || "An unexpected error occurred while loading recruitment applications."}
          </p>
          <div className="mt-6 flex items-center justify-center gap-3">
            <Button
              type="button"
              variant="outline"
              onClick={() => void loadDashboardData()}
              className="inline-flex items-center gap-2"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              <span>Try Again</span>
            </Button>
            <Link
              href="/projects"
              className="inline-flex items-center justify-center rounded-md bg-[#111111] px-4 py-2 text-xs font-medium text-white hover:bg-[#222222] transition-colors"
            >
              Back to Projects
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Authorization Check: Only Lead and Maintainer permitted
  if (!isAuthorized) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            <Lock className="h-6 w-6" />
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal text-foreground">
            Access Restricted
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Unauthorized: Lead or Maintainer permissions required to view the recruitment dashboard.
          </p>
          <div className="mt-6">
            <Link
              href={`/projects/${encodeURIComponent(project.slug || project.id)}`}
              className="inline-flex items-center justify-center rounded-md bg-[#111111] px-4 py-2 text-xs font-medium text-white hover:bg-[#222222] transition-colors gap-1.5"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>Back to Project Workspace</span>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const projectUrl = `/projects/${encodeURIComponent(project.slug || project.id)}`;

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8 space-y-8">
      {/* Breadcrumb Navigation */}
      <div>
        <Link
          href={projectUrl}
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to Project Workspace</span>
        </Link>
      </div>

      {/* Dashboard Header */}
      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-4 border-b border-border pb-6">
        <div>
          <div className="flex items-center gap-2">
            {project.category && (
              <Badge variant="outline" className="font-mono text-xs">
                {project.category}
              </Badge>
            )}
            <span className="font-mono text-xs text-muted-foreground">Workspace Recruitment</span>
          </div>
          <h1 className="mt-2 font-serif text-3xl sm:text-4xl font-normal text-foreground">
            Recruitment Dashboard
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Reviewing candidate proposals for{" "}
            <span className="font-medium text-foreground">{project.title}</span>
          </p>
        </div>

        <div className="flex items-center gap-2">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => void refreshApplications()}
            disabled={isRefreshing}
            className="inline-flex items-center gap-1.5 text-xs h-9"
          >
            <RotateCcw className={cn("h-3.5 w-3.5", isRefreshing && "animate-spin")} />
            <span>Refresh</span>
          </Button>

          <Link
            href={projectUrl}
            className="inline-flex items-center justify-center rounded-md border border-border bg-card px-3 py-2 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors"
          >
            Project Workspace
          </Link>
        </div>
      </div>

      {/* Quick Action Error Alert */}
      {quickActionError && (
        <div className="flex items-center justify-between gap-3 rounded-lg border border-rose-500/20 bg-rose-500/10 p-3.5 text-xs text-rose-600">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0" />
            <span>{quickActionError}</span>
          </div>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            onClick={() => setQuickActionError(null)}
            className="h-6 px-2 text-xs text-rose-600 hover:text-rose-700"
          >
            Dismiss
          </Button>
        </div>
      )}

      {/* Metric Stats Cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        {/* Total Applications */}
        <div className="rounded-xl border border-border bg-card p-4 shadow-none">
          <div className="flex items-center justify-between text-muted-foreground">
            <span className="font-mono text-xs uppercase tracking-wider">Total</span>
            <Users className="h-4 w-4" />
          </div>
          <p className="mt-3 font-serif text-2xl sm:text-3xl font-medium text-foreground">
            {metrics.total}
          </p>
          <p className="mt-1 text-[11px] text-muted-foreground">All submitted applications</p>
        </div>

        {/* Pending Review */}
        <div className="rounded-xl border border-border bg-card p-4 shadow-none">
          <div className="flex items-center justify-between text-blue-600">
            <span className="font-mono text-xs uppercase tracking-wider">Pending</span>
            <Clock className="h-4 w-4" />
          </div>
          <p className="mt-3 font-serif text-2xl sm:text-3xl font-medium text-foreground">
            {metrics.submitted}
          </p>
          <p className="mt-1 text-[11px] text-muted-foreground">Awaiting lead review</p>
        </div>

        {/* Shortlisted */}
        <div className="rounded-xl border border-border bg-card p-4 shadow-none">
          <div className="flex items-center justify-between text-amber-600">
            <span className="font-mono text-xs uppercase tracking-wider">Shortlisted</span>
            <Star className="h-4 w-4" />
          </div>
          <p className="mt-3 font-serif text-2xl sm:text-3xl font-medium text-foreground">
            {metrics.shortlisted}
          </p>
          <p className="mt-1 text-[11px] text-muted-foreground">Marked for next steps</p>
        </div>

        {/* Accepted Members */}
        <div className="rounded-xl border border-border bg-card p-4 shadow-none">
          <div className="flex items-center justify-between text-emerald-600">
            <span className="font-mono text-xs uppercase tracking-wider">Accepted</span>
            <CheckCircle2 className="h-4 w-4" />
          </div>
          <p className="mt-3 font-serif text-2xl sm:text-3xl font-medium text-foreground">
            {metrics.accepted}
          </p>
          <p className="mt-1 text-[11px] text-muted-foreground">Active team collaborators</p>
        </div>
      </div>

      {/* Filter & Search Toolbar */}
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        {/* Status Toggle Pills */}
        <div className="flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            onClick={() => setStatusFilter("all")}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono transition-colors",
              statusFilter === "all"
                ? "bg-[#111111] text-white"
                : "bg-muted text-muted-foreground hover:text-foreground"
            )}
          >
            <span>All</span>
            <span className="text-[10px] opacity-70">({metrics.total})</span>
          </button>

          <button
            type="button"
            onClick={() => setStatusFilter("submitted")}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono transition-colors",
              statusFilter === "submitted"
                ? "bg-blue-600 text-white"
                : "bg-muted text-muted-foreground hover:text-foreground"
            )}
          >
            <span>Submitted</span>
            <span className="text-[10px] opacity-70">({metrics.submitted})</span>
          </button>

          <button
            type="button"
            onClick={() => setStatusFilter("shortlisted")}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono transition-colors",
              statusFilter === "shortlisted"
                ? "bg-amber-600 text-white"
                : "bg-muted text-muted-foreground hover:text-foreground"
            )}
          >
            <span>Shortlisted</span>
            <span className="text-[10px] opacity-70">({metrics.shortlisted})</span>
          </button>

          <button
            type="button"
            onClick={() => setStatusFilter("accepted")}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono transition-colors",
              statusFilter === "accepted"
                ? "bg-emerald-600 text-white"
                : "bg-muted text-muted-foreground hover:text-foreground"
            )}
          >
            <span>Accepted</span>
            <span className="text-[10px] opacity-70">({metrics.accepted})</span>
          </button>

          <button
            type="button"
            onClick={() => setStatusFilter("rejected")}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-full px-3 py-1 text-xs font-mono transition-colors",
              statusFilter === "rejected"
                ? "bg-rose-600 text-white"
                : "bg-muted text-muted-foreground hover:text-foreground"
            )}
          >
            <span>Rejected</span>
            <span className="text-[10px] opacity-70">({metrics.rejected})</span>
          </button>
        </div>

        {/* Search Input */}
        <div className="relative w-full sm:w-72">
          <Search className="absolute left-3 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            type="text"
            placeholder="Search candidate, email, major..."
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="pl-9 text-xs h-9"
          />
        </div>
      </div>

      {/* Applications List / Table */}
      {filteredApplications.length === 0 ? (
        <div className="rounded-xl border border-dashed border-border bg-card p-12 text-center">
          <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-lg bg-muted text-muted-foreground">
            <FileText className="h-5 w-5" />
          </div>
          <h3 className="mt-3 font-serif text-lg font-medium text-foreground">
            {applications.length === 0
              ? "No Applications Received"
              : "No Applications Match Filter"}
          </h3>
          <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto">
            {applications.length === 0
              ? "When campus students apply to join this workspace team, their proposals and resumes will appear here."
              : "Try adjusting your search keywords or switching status filters above."}
          </p>
          {applications.length > 0 && (
            <div className="mt-4">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  setSearchQuery("");
                  setStatusFilter("all");
                }}
                className="text-xs"
              >
                Clear Filters
              </Button>
            </div>
          )}
        </div>
      ) : (
        <div className="space-y-3">
          {/* Desktop Table View */}
          <div className="hidden md:block overflow-hidden rounded-xl border border-border bg-card">
            <table className="w-full border-collapse text-left text-xs">
              <thead>
                <tr className="border-b border-border bg-muted/40 font-mono uppercase tracking-wider text-muted-foreground">
                  <th className="py-3 pl-4 pr-3 font-medium">Candidate</th>
                  <th className="px-3 py-3 font-medium">Department</th>
                  <th className="px-3 py-3 font-medium">Applied</th>
                  <th className="px-3 py-3 font-medium">Status</th>
                  <th className="py-3 pl-3 pr-4 text-right font-medium">Actions</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {filteredApplications.map((app) => {
                  const name = getApplicantDisplayName(app);
                  const email = getApplicantEmail(app);
                  const dept = getApplicantDepartment(app);
                  const initials = getApplicantInitials(name);
                  const badge = getApplicationStatusBadgeClasses(app.status);
                  const isPendingOrShortlisted =
                    app.status === "submitted" || app.status === "shortlisted";
                  const isActionBusy = quickActionId === app.id;

                  return (
                    <tr key={app.id} className="hover:bg-muted/20 transition-colors">
                      {/* Candidate Column */}
                      <td className="py-3.5 pl-4 pr-3">
                        <div className="flex items-center gap-3">
                          <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-foreground text-background font-mono text-xs font-semibold">
                            {initials}
                          </div>
                          <div className="min-w-0">
                            <p className="font-medium text-foreground truncate max-w-[200px]">
                              {name}
                            </p>
                            {email && (
                              <p className="text-[11px] text-muted-foreground truncate max-w-[200px]">
                                {email}
                              </p>
                            )}
                          </div>
                        </div>
                      </td>

                      {/* Department Column */}
                      <td className="px-3 py-3.5 text-muted-foreground truncate max-w-[150px]">
                        {dept}
                      </td>

                      {/* Applied Date Column */}
                      <td className="px-3 py-3.5 font-mono text-muted-foreground whitespace-nowrap">
                        {formatJobDate(app.created_at)}
                      </td>

                      {/* Status Column */}
                      <td className="px-3 py-3.5 whitespace-nowrap">
                        <span
                          className={cn(
                            "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-mono uppercase tracking-wider",
                            badge.bg,
                            badge.text,
                            badge.border
                          )}
                        >
                          <span className={cn("h-1.5 w-1.5 rounded-full", badge.dot)} />
                          {badge.label}
                        </span>
                      </td>

                      {/* Actions Column */}
                      <td className="py-3.5 pl-3 pr-4 text-right whitespace-nowrap">
                        <div className="flex items-center justify-end gap-2">
                          {/* Quick Decision Buttons for un-decided candidates */}
                          {isPendingOrShortlisted && (
                            <>
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                title="Reject candidate"
                                onClick={() => void handleQuickDecision(app, "reject")}
                                disabled={isActionBusy}
                                className="h-7 w-7 p-0 border-rose-500/20 text-rose-600 hover:bg-rose-500/10 hover:text-rose-700"
                              >
                                {isActionBusy ? (
                                  <Loader2 className="h-3 w-3 animate-spin" />
                                ) : (
                                  <XCircle className="h-3.5 w-3.5" />
                                )}
                              </Button>

                              {app.status !== "shortlisted" && (
                                <Button
                                  type="button"
                                  variant="outline"
                                  size="sm"
                                  title="Shortlist candidate"
                                  onClick={() => void handleQuickDecision(app, "shortlist")}
                                  disabled={isActionBusy}
                                  className="h-7 w-7 p-0 border-amber-500/20 text-amber-600 hover:bg-amber-500/10 hover:text-amber-700"
                                >
                                  <Star className="h-3.5 w-3.5" />
                                </Button>
                              )}

                              <Button
                                type="button"
                                size="sm"
                                title="Accept and add to team"
                                onClick={() => void handleQuickDecision(app, "accept")}
                                disabled={isActionBusy}
                                className="h-7 px-2 bg-emerald-600 hover:bg-emerald-700 text-white text-[11px] inline-flex items-center gap-1"
                              >
                                <UserCheck className="h-3 w-3" />
                                <span>Accept</span>
                              </Button>
                            </>
                          )}

                          {/* Full Review Modal Trigger */}
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => handleOpenReview(app)}
                            className="h-7 px-2.5 text-[11px]"
                          >
                            Review
                          </Button>
                        </div>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          {/* Mobile Card View */}
          <div className="grid grid-cols-1 gap-3 md:hidden">
            {filteredApplications.map((app) => {
              const name = getApplicantDisplayName(app);
              const email = getApplicantEmail(app);
              const dept = getApplicantDepartment(app);
              const initials = getApplicantInitials(name);
              const badge = getApplicationStatusBadgeClasses(app.status);
              const isPendingOrShortlisted =
                app.status === "submitted" || app.status === "shortlisted";
              const isActionBusy = quickActionId === app.id;

              return (
                <div key={app.id} className="rounded-xl border border-border bg-card p-4 space-y-3">
                  <div className="flex items-start justify-between gap-3">
                    <div className="flex items-center gap-2.5">
                      <div className="flex h-9 w-9 shrink-0 items-center justify-center rounded-full bg-foreground text-background font-mono text-xs font-semibold">
                        {initials}
                      </div>
                      <div>
                        <h4 className="text-sm font-medium text-foreground">{name}</h4>
                        {email && (
                          <p className="text-xs text-muted-foreground flex items-center gap-1 mt-0.5">
                            <Mail className="h-3 w-3" />
                            <span>{email}</span>
                          </p>
                        )}
                      </div>
                    </div>

                    <span
                      className={cn(
                        "inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 text-[10px] font-mono uppercase tracking-wider",
                        badge.bg,
                        badge.text,
                        badge.border
                      )}
                    >
                      <span className={cn("h-1.5 w-1.5 rounded-full", badge.dot)} />
                      {badge.label}
                    </span>
                  </div>

                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground pt-1 border-t border-border/60">
                    <div className="flex items-center gap-1">
                      <GraduationCap className="h-3 w-3" />
                      <span>{dept}</span>
                    </div>
                    <div className="flex items-center gap-1 font-mono text-[11px]">
                      <Calendar className="h-3 w-3" />
                      <span>{formatJobDate(app.created_at)}</span>
                    </div>
                  </div>

                  {app.statement && (
                    <p className="text-xs text-foreground/80 line-clamp-2 leading-relaxed bg-muted/30 p-2 rounded-md">
                      {app.statement}
                    </p>
                  )}

                  <div className="flex items-center justify-end gap-2 pt-1 border-t border-border/60">
                    {isPendingOrShortlisted && (
                      <>
                        <Button
                          type="button"
                          variant="outline"
                          size="sm"
                          onClick={() => void handleQuickDecision(app, "reject")}
                          disabled={isActionBusy}
                          className="h-8 border-rose-500/20 text-rose-600 hover:bg-rose-500/10 text-xs"
                        >
                          Reject
                        </Button>

                        {app.status !== "shortlisted" && (
                          <Button
                            type="button"
                            variant="outline"
                            size="sm"
                            onClick={() => void handleQuickDecision(app, "shortlist")}
                            disabled={isActionBusy}
                            className="h-8 border-amber-500/20 text-amber-600 hover:bg-amber-500/10 text-xs"
                          >
                            Shortlist
                          </Button>
                        )}

                        <Button
                          type="button"
                          size="sm"
                          onClick={() => void handleQuickDecision(app, "accept")}
                          disabled={isActionBusy}
                          className="h-8 bg-emerald-600 hover:bg-emerald-700 text-white text-xs"
                        >
                          Accept
                        </Button>
                      </>
                    )}

                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      onClick={() => handleOpenReview(app)}
                      className="h-8 text-xs"
                    >
                      Review
                    </Button>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Detailed Applicant Review Modal */}
      <ApplicantReviewModal
        open={isReviewModalOpen}
        onClose={() => setIsReviewModalOpen(false)}
        application={selectedApplication}
        projectTitle={project.title}
        onDecision={handleApplicationUpdated}
      />
    </div>
  );
}
