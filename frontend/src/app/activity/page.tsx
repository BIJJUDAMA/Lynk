"use client";

import React, { useState, useMemo, useCallback } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  Briefcase,
  FileText,
  FileCheck,
  PlusCircle,
  Users,
  Search,
  RotateCcw,
  AlertCircle,
  Lock,
  GraduationCap,
  Calendar,
  Clock,
  DollarSign,
  ArrowRight,
  ExternalLink,
  CheckCircle2,
  XCircle,
  Loader2,
  ChevronRight,
  ShieldCheck,
} from "lucide-react";
import {
  Job,
  ApplicationWithDetails,
  ContractWithDetails,
  ApplicationStatus,
  JobStatus,
  ContractStatus,
} from "@/types/api";
import {
  getMyApplications,
  getMyJobs,
  listContracts,
  ApiClientError,
} from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatBudget,
  formatJobDate,
  getStatusBadgeClasses,
  getApplicationStatusBadgeClasses,
  getContractStatusBadgeClasses,
} from "@/lib/formatters";
import { cn } from "@/lib/utils";

type ActivityTab = "applications" | "postings" | "contracts";

function ActivityContent() {
  const searchParams = useSearchParams();
  const initialTab = (searchParams.get("tab") as ActivityTab) || "applications";

  const {
    user,
    backendUser,
    isAuthenticated,
    isVerified,
    isLoading: authLoading,
    login,
  } = useAuth();

  const [activeTab, setActiveTab] = useState<ActivityTab>(
    ["applications", "postings", "contracts"].includes(initialTab) ? initialTab : "applications"
  );
  const [searchQuery, setSearchQuery] = useState("");

  // 1. Fetch Applications (applied to by current user)
  const {
    data: applications,
    isLoading: isAppsLoading,
    error: appsError,
    refetch: refetchApps,
  } = useQuery<ApplicationWithDetails[]>(
    useCallback((client) => getMyApplications(client), []),
    {
      enabled: Boolean(isAuthenticated),
    }
  );

  // 2. Fetch My Posted Jobs (created by current user)
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

  // 3. Fetch Contracts (involving current user as client or freelancer)
  const {
    data: contracts,
    isLoading: isContractsLoading,
    error: contractsError,
    refetch: refetchContracts,
  } = useQuery<ContractWithDetails[]>(
    useCallback((client) => listContracts(client), []),
    {
      enabled: Boolean(isAuthenticated),
    }
  );

  const appsList = useMemo(() => applications ?? [], [applications]);
  const jobsList = useMemo(() => myJobs ?? [], [myJobs]);
  const contractsList = useMemo(() => contracts ?? [], [contracts]);

  // Counts
  const activeContractsCount = useMemo(
    () => contractsList.filter((c) => c.status === "active").length,
    [contractsList]
  );

  // Filtered Applications
  const filteredApplications = useMemo(() => {
    if (!searchQuery.trim()) return appsList;
    const q = searchQuery.toLowerCase().trim();
    return appsList.filter((app) => {
      const title = app.job?.title?.toLowerCase() || "";
      const dept = app.job?.department?.toLowerCase() || "";
      return title.includes(q) || dept.includes(q);
    });
  }, [appsList, searchQuery]);

  // Filtered Postings
  const filteredJobs = useMemo(() => {
    if (!searchQuery.trim()) return jobsList;
    const q = searchQuery.toLowerCase().trim();
    return jobsList.filter((job) => {
      const title = job.title.toLowerCase();
      const dept = job.department.toLowerCase();
      return title.includes(q) || dept.includes(q);
    });
  }, [jobsList, searchQuery]);

  // Filtered Contracts
  const filteredContracts = useMemo(() => {
    if (!searchQuery.trim()) return contractsList;
    const q = searchQuery.toLowerCase().trim();
    return contractsList.filter((c) => {
      const title = c.job?.title?.toLowerCase() || "";
      return title.includes(q);
    });
  }, [contractsList, searchQuery]);

  // Loading State
  if (authLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  // Unauthenticated Gate
  if (!isAuthenticated || !user) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 text-center">
        <div className="rounded-xl border border-border bg-card p-8 shadow-sm">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            <Lock className="h-6 w-6" />
          </div>
          <h1 className="mt-4 text-xl font-semibold tracking-tight text-foreground">
            Campus Sign In Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Sign in to access your unified workspace: applications submitted, opportunities posted, and active contracts.
          </p>
          <div className="mt-6 flex justify-center">
            <button
              onClick={() => login({ redirectPath: "/activity" })}
              className="rounded-lg bg-primary px-5 py-2.5 text-sm font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
            >
              Sign In with Campus Account
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Top Header */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between border-b border-border pb-6">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
            Campus Activity Workspace
          </h1>
          <p className="mt-1 text-sm text-muted-foreground">
            Track submitted proposals, manage your job postings, and review active contracts.
          </p>
        </div>

        <Link
          href="/jobs/create"
          className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 self-start sm:self-auto"
        >
          <PlusCircle className="h-4 w-4" />
          <span>Post an Opportunity</span>
        </Link>
      </div>

      {/* Metrics Overview Bento */}
      <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-3">
        <div
          onClick={() => setActiveTab("applications")}
          className={cn(
            "cursor-pointer rounded-xl border p-5 transition",
            activeTab === "applications"
              ? "border-primary bg-primary/5 shadow-sm"
              : "border-border bg-card hover:border-border/80"
          )}
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-muted-foreground">Applications Submitted</span>
            <FileText className="h-4 w-4 text-primary" />
          </div>
          <div className="mt-2 text-2xl font-bold text-foreground">
            {appsList.length}
          </div>
          <p className="mt-1 text-[11px] text-muted-foreground">
            Gigs you applied to with your resume
          </p>
        </div>

        <div
          onClick={() => setActiveTab("postings")}
          className={cn(
            "cursor-pointer rounded-xl border p-5 transition",
            activeTab === "postings"
              ? "border-primary bg-primary/5 shadow-sm"
              : "border-border bg-card hover:border-border/80"
          )}
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-muted-foreground">My Postings</span>
            <Briefcase className="h-4 w-4 text-primary" />
          </div>
          <div className="mt-2 text-2xl font-bold text-foreground">
            {jobsList.length}
          </div>
          <p className="mt-1 text-[11px] text-muted-foreground">
            Opportunities you posted on campus
          </p>
        </div>

        <div
          onClick={() => setActiveTab("contracts")}
          className={cn(
            "cursor-pointer rounded-xl border p-5 transition",
            activeTab === "contracts"
              ? "border-primary bg-primary/5 shadow-sm"
              : "border-border bg-card hover:border-border/80"
          )}
        >
          <div className="flex items-center justify-between">
            <span className="text-xs font-medium text-muted-foreground">Active Contracts</span>
            <FileCheck className="h-4 w-4 text-primary" />
          </div>
          <div className="mt-2 text-2xl font-bold text-foreground">
            {contractsList.length}
          </div>
          <p className="mt-1 text-[11px] text-muted-foreground">
            {activeContractsCount} active contracts
          </p>
        </div>
      </div>

      {/* Tabs & Search Bar */}
      <div className="mt-8 flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex rounded-lg border border-border bg-muted/40 p-1 self-start">
          <button
            type="button"
            onClick={() => setActiveTab("applications")}
            className={cn(
              "rounded-md px-3.5 py-1.5 text-xs font-medium transition",
              activeTab === "applications"
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            Applications ({appsList.length})
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("postings")}
            className={cn(
              "rounded-md px-3.5 py-1.5 text-xs font-medium transition",
              activeTab === "postings"
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            My Postings ({jobsList.length})
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("contracts")}
            className={cn(
              "rounded-md px-3.5 py-1.5 text-xs font-medium transition",
              activeTab === "contracts"
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            Contracts ({contractsList.length})
          </button>
        </div>

        {/* Search */}
        <div className="relative w-full sm:w-64">
          <Search className="absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Filter list..."
            className="w-full rounded-lg border border-border bg-background pl-9 pr-3 py-1.5 text-xs text-foreground focus:border-primary focus:outline-none"
          />
        </div>
      </div>

      {/* Tab 1: Applications */}
      {activeTab === "applications" && (
        <div className="mt-6 space-y-4">
          {isAppsLoading ? (
            <div className="flex min-h-[30vh] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : appsError ? (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center text-xs text-red-700 dark:text-red-300">
              <p>Failed to load applications: {appsError.message}</p>
              <button
                onClick={() => refetchApps()}
                className="mt-2 font-semibold underline"
              >
                Retry
              </button>
            </div>
          ) : filteredApplications.length === 0 ? (
            <div className="rounded-xl border border-border bg-card p-12 text-center shadow-sm">
              <FileText className="mx-auto h-8 w-8 text-muted-foreground" />
              <h3 className="mt-3 text-sm font-semibold text-foreground">
                No applications submitted
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                Browse open campus jobs, research gigs, and student projects to apply.
              </p>
              <div className="mt-4">
                <Link
                  href="/jobs"
                  className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground hover:bg-primary/90"
                >
                  Browse Campus Jobs
                </Link>
              </div>
            </div>
          ) : (
            filteredApplications.map((app) => {
              const statusStyles = getApplicationStatusBadgeClasses(app.status);
              return (
                <div
                  key={app.id}
                  className="rounded-xl border border-border bg-card p-5 shadow-sm transition hover:border-border/80"
                >
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div className="flex items-center gap-2">
                        <span
                          className={cn(
                            "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-[11px] font-medium",
                            statusStyles.bg,
                            statusStyles.text
                          )}
                        >
                          <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
                          {statusStyles.label}
                        </span>
                        {app.job?.department && (
                          <span className="text-xs text-muted-foreground">
                            • {app.job.department}
                          </span>
                        )}
                      </div>

                      <Link
                        href={`/jobs/${app.job_id}`}
                        className="mt-2 block text-base font-semibold text-foreground hover:text-primary transition"
                      >
                        {app.job?.title || "Untitled Opportunity"}
                      </Link>

                      <p className="mt-1 text-xs text-muted-foreground">
                        Applied on {formatJobDate(app.created_at)}
                      </p>
                    </div>

                    <div className="flex items-center gap-2 self-start sm:self-auto">
                      {app.status === "accepted" && (
                        <Link
                          href="/contracts"
                          className="inline-flex items-center gap-1 rounded-lg bg-emerald-600 px-3 py-1.5 text-xs font-medium text-white shadow-sm hover:bg-emerald-500"
                        >
                          <FileCheck className="h-3.5 w-3.5" />
                          <span>View Contract</span>
                        </Link>
                      )}
                      <Link
                        href={`/jobs/${app.job_id}`}
                        className="inline-flex items-center gap-1 rounded-lg border border-border bg-background px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted"
                      >
                        <span>View Opportunity</span>
                        <ChevronRight className="h-3.5 w-3.5" />
                      </Link>
                    </div>
                  </div>

                  {/* Proposal Snippet */}
                  <div className="mt-3 rounded-lg border border-border/50 bg-muted/30 p-3 text-xs text-muted-foreground">
                    <p className="font-medium text-foreground text-[11px]">Proposal Cover Letter:</p>
                    <p className="mt-1 line-clamp-2">{app.cover_letter}</p>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}

      {/* Tab 2: My Postings */}
      {activeTab === "postings" && (
        <div className="mt-6 space-y-4">
          {isJobsLoading ? (
            <div className="flex min-h-[30vh] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : jobsError ? (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center text-xs text-red-700 dark:text-red-300">
              <p>Failed to load postings: {jobsError.message}</p>
              <button
                onClick={() => refetchJobs()}
                className="mt-2 font-semibold underline"
              >
                Retry
              </button>
            </div>
          ) : filteredJobs.length === 0 ? (
            <div className="rounded-xl border border-border bg-card p-12 text-center shadow-sm">
              <Briefcase className="mx-auto h-8 w-8 text-muted-foreground" />
              <h3 className="mt-3 text-sm font-semibold text-foreground">
                No opportunities posted yet
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                As a verified campus member, you can publish gigs, research tasks, and projects.
              </p>
              <div className="mt-4">
                <Link
                  href="/jobs/create"
                  className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground hover:bg-primary/90"
                >
                  <PlusCircle className="h-3.5 w-3.5" />
                  <span>Post Your First Job</span>
                </Link>
              </div>
            </div>
          ) : (
            filteredJobs.map((job) => {
              const statusStyles = getStatusBadgeClasses(job.status);
              return (
                <div
                  key={job.id}
                  className="rounded-xl border border-border bg-card p-5 shadow-sm transition hover:border-border/80"
                >
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div className="flex items-center gap-2">
                        <span
                          className={cn(
                            "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-[11px] font-medium",
                            statusStyles.bg,
                            statusStyles.text
                          )}
                        >
                          <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
                          {job.status.replace("_", " ")}
                        </span>
                        <span className="text-xs text-muted-foreground">
                          • {job.department}
                        </span>
                        <span className="text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                          • {formatBudget(job.budget_cents, job.pay_type)}
                        </span>
                      </div>

                      <Link
                        href={`/jobs/${job.id}`}
                        className="mt-2 block text-base font-semibold text-foreground hover:text-primary transition"
                      >
                        {job.title}
                      </Link>

                      <p className="mt-1 text-xs text-muted-foreground">
                        Posted {formatJobDate(job.created_at)} • Deadline: {formatJobDate(job.deadline)}
                      </p>
                    </div>

                    <div className="flex items-center gap-2 self-start sm:self-auto">
                      <Link
                        href={`/jobs/${job.id}/applicants`}
                        className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-3.5 py-1.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
                      >
                        <Users className="h-3.5 w-3.5" />
                        <span>Review Applicants</span>
                      </Link>
                      <Link
                        href={`/jobs/${job.id}`}
                        className="inline-flex items-center gap-1 rounded-lg border border-border bg-background px-3 py-1.5 text-xs font-medium text-foreground hover:bg-muted"
                      >
                        <span>View</span>
                        <ChevronRight className="h-3.5 w-3.5" />
                      </Link>
                    </div>
                  </div>
                </div>
              );
            })
          )}
        </div>
      )}

      {/* Tab 3: Contracts */}
      {activeTab === "contracts" && (
        <div className="mt-6 space-y-4">
          {isContractsLoading ? (
            <div className="flex min-h-[30vh] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : contractsError ? (
            <div className="rounded-xl border border-red-500/30 bg-red-500/10 p-6 text-center text-xs text-red-700 dark:text-red-300">
              <p>Failed to load contracts: {contractsError.message}</p>
              <button
                onClick={() => refetchContracts()}
                className="mt-2 font-semibold underline"
              >
                Retry
              </button>
            </div>
          ) : filteredContracts.length === 0 ? (
            <div className="rounded-xl border border-border bg-card p-12 text-center shadow-sm">
              <FileCheck className="mx-auto h-8 w-8 text-muted-foreground" />
              <h3 className="mt-3 text-sm font-semibold text-foreground">
                No contracts initiated yet
              </h3>
              <p className="mt-1 text-xs text-muted-foreground">
                Contracts are automatically generated when a job application proposal is accepted.
              </p>
            </div>
          ) : (
            filteredContracts.map((contract) => {
              const statusStyles = getContractStatusBadgeClasses(contract.status);
              const isClient =
                contract.client_id === user?.id || contract.client_id === backendUser?.id;

              return (
                <div
                  key={contract.id}
                  className="rounded-xl border border-border bg-card p-5 shadow-sm transition hover:border-border/80"
                >
                  <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
                    <div>
                      <div className="flex items-center gap-2">
                        <span
                          className={cn(
                            "inline-flex items-center gap-1 rounded-full border px-2.5 py-0.5 text-[11px] font-medium",
                            statusStyles.bg,
                            statusStyles.text
                          )}
                        >
                          <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
                          {statusStyles.label}
                        </span>

                        <span className="rounded bg-muted px-2 py-0.5 text-[10px] font-medium text-muted-foreground">
                          {isClient ? "You are Client" : "You are Freelancer"}
                        </span>

                        <span className="text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                          • {formatBudget(contract.agreed_budget_cents, "fixed")}
                        </span>
                      </div>

                      <h3 className="mt-2 text-base font-semibold text-foreground">
                        {contract.job?.title || "Contract Agreement"}
                      </h3>

                      <p className="mt-1 text-xs text-muted-foreground">
                        Initiated {formatJobDate(contract.created_at)}
                      </p>
                    </div>

                    <div className="flex items-center gap-2 self-start sm:self-auto">
                      <Link
                        href={`/contracts/${contract.id}`}
                        className="inline-flex items-center gap-1.5 rounded-lg bg-primary px-3.5 py-1.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
                      >
                        <FileCheck className="h-3.5 w-3.5" />
                        <span>Manage Contract</span>
                      </Link>
                    </div>
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
    <React.Suspense
      fallback={
        <div className="flex min-h-[60vh] items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <ActivityContent />
    </React.Suspense>
  );
}
