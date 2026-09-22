"use client";

import React, { useState, useMemo, useCallback, Suspense } from "react";
import Link from "next/link";
import { useSearchParams } from "next/navigation";
import {
  Briefcase,
  FileText,
  FileCheck,
  Users,
  Search,
  Lock,
  Loader2,
  ArrowRight,
  Plus,
  RotateCcw,
} from "lucide-react";
import { Job, ApplicationWithDetails, ContractWithDetails } from "@/types/api";
import { getMyApplications, getMyJobs, listContracts } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatJobDate,
  formatJobBudget,
  getStatusBadgeClasses,
  getApplicationStatusBadgeClasses,
  getContractStatusBadgeClasses,
} from "@/lib/formatters";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

type ActivityTab = "applications" | "postings" | "contracts";

function ActivityContent() {
  const searchParams = useSearchParams();
  const initialTab = (searchParams.get("tab") as ActivityTab) || "applications";

  const { user, backendUser, isAuthenticated, isLoading: authLoading, login } = useAuth();

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
          <h1 className="font-serif text-2xl font-normal tracking-tight text-foreground">
            Campus Sign In Required
          </h1>
          <p className="text-xs text-muted-foreground max-w-md mx-auto">
            Sign in to access your activity hub: submitted proposals, active contracts, and posted
            gigs.
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
          <h1 className="font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
            Activity
          </h1>
          <p className="mt-1 text-xs text-muted-foreground">
            Your pending proposals, active contracts, and completed deliverables.
          </p>
        </div>

        <Button asChild className="text-xs self-start sm:self-auto">
          <Link href="/jobs/create">
            <Plus className="h-3.5 w-3.5 mr-1.5" />
            <span>Post a Project</span>
          </Link>
        </Button>
      </div>

      {/* Tab Strip & Search Bar */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-end sm:justify-between border-b border-border pb-1">
        {/* Horizontal Underline Tabs */}
        <div className="flex items-end gap-1 -mb-1">
          <button
            type="button"
            onClick={() => setActiveTab("applications")}
            className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${
              activeTab === "applications"
                ? "border-foreground text-foreground font-semibold"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            Applications
            <span className="ml-1.5 font-mono text-[11px] text-muted-foreground">
              ({appsList.length})
            </span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("contracts")}
            className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${
              activeTab === "contracts"
                ? "border-foreground text-foreground font-semibold"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            Contracts
            <span className="ml-1.5 font-mono text-[11px] text-muted-foreground">
              ({contractsList.length})
            </span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab("postings")}
            className={`px-4 py-2 text-xs font-medium border-b-2 transition-colors whitespace-nowrap ${
              activeTab === "postings"
                ? "border-foreground text-foreground font-semibold"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            My Postings
            <span className="ml-1.5 font-mono text-[11px] text-muted-foreground">
              ({jobsList.length})
            </span>
          </button>
        </div>

        {/* Search */}
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

      {/* Tab 1: Applications */}
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
                  No proposals submitted
                </h3>
                <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
                  You have not submitted any proposals yet. Browse open gigs to get started.
                </p>
              </div>
              <div className="pt-2">
                <Button asChild className="text-xs">
                  <Link href="/jobs">
                    <span>Browse Campus Jobs</span>
                    <ArrowRight className="h-3.5 w-3.5 ml-1.5" />
                  </Link>
                </Button>
              </div>
            </div>
          ) : (
            filteredApplications.map((app) => {
              const statusStyles = getApplicationStatusBadgeClasses(app.status);
              return (
                <div
                  key={app.id}
                  className="rounded-xl border border-border bg-card p-6 shadow-none transition hover:border-foreground/30 flex flex-col md:flex-row justify-between gap-5"
                >
                  <div className="space-y-2 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold ${statusStyles.bg} ${statusStyles.text}`}
                      >
                        <span className={`h-1.5 w-1.5 rounded-full ${statusStyles.dot}`} />
                        {statusStyles.label}
                      </span>
                      {app.job?.department && (
                        <Badge variant="secondary" className="font-mono text-xs">
                          {app.job.department}
                        </Badge>
                      )}
                      {app.job && (
                        <span className="font-mono text-xs font-semibold text-foreground ml-auto sm:ml-2">
                          {formatJobBudget(app.job)}
                        </span>
                      )}
                    </div>

                    <Link href={`/jobs/${app.job_id}`}>
                      <h2 className="font-serif text-xl font-medium text-foreground hover:underline underline-offset-2 transition-colors">
                        {app.job?.title || "Untitled Opportunity"}
                      </h2>
                    </Link>

                    <p className="font-mono text-xs text-muted-foreground">
                      Applied on {formatJobDate(app.created_at)}
                    </p>

                    {/* Proposal Snippet */}
                    {app.cover_letter && (
                      <p className="border-l-2 border-border pl-3 text-xs text-foreground/80 line-clamp-2 leading-relaxed pt-1">
                        {app.cover_letter}
                      </p>
                    )}
                  </div>

                  <div className="flex flex-wrap items-center gap-2.5 shrink-0 pt-2 md:pt-0">
                    {app.status === "accepted" && (
                      <Button asChild size="sm" className="text-xs">
                        <Link href="/contracts">
                          <FileCheck className="h-3.5 w-3.5 mr-1.5" />
                          <span>View Contract</span>
                        </Link>
                      </Button>
                    )}

                    <Button asChild variant="outline" size="sm" className="text-xs">
                      <Link href={`/jobs/${app.job_id}`}>
                        <span>View Gig</span>
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

      {/* Tab 2: Contracts */}
      {activeTab === "contracts" && (
        <div className="space-y-4">
          {isContractsLoading ? (
            <div className="flex min-h-[30vh] items-center justify-center">
              <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
            </div>
          ) : contractsError ? (
            <div className="rounded-xl border border-pastel-redText/20 bg-pastel-red p-6 text-center text-xs text-pastel-redText space-y-2">
              <p>Failed to load contracts: {contractsError.message}</p>
              <Button
                variant="outline"
                size="sm"
                onClick={() => refetchContracts()}
                className="text-xs"
              >
                <RotateCcw className="h-3 w-3 mr-1.5" />
                Retry
              </Button>
            </div>
          ) : filteredContracts.length === 0 ? (
            <div className="rounded-xl border border-border bg-card p-12 text-center shadow-none space-y-3">
              <FileCheck className="mx-auto h-8 w-8 text-muted-foreground" />
              <div>
                <h3 className="font-serif text-xl font-medium text-foreground">
                  No active contracts
                </h3>
                <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
                  No active contracts. Accepted applications generate contracts automatically.
                </p>
              </div>
            </div>
          ) : (
            filteredContracts.map((contract) => {
              const statusStyles = getContractStatusBadgeClasses(contract.status);
              const isClient =
                contract.client_id === user?.id || contract.client_id === backendUser?.id;

              return (
                <div
                  key={contract.id}
                  className="rounded-xl border border-border bg-card p-6 shadow-none transition hover:border-foreground/30 flex flex-col md:flex-row justify-between gap-5"
                >
                  <div className="space-y-2 flex-1">
                    <div className="flex flex-wrap items-center gap-2">
                      <span className="font-mono text-xs text-muted-foreground">
                        #{contract.id.slice(0, 8)}
                      </span>
                      <span
                        className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider ${statusStyles.bg} ${statusStyles.text}`}
                      >
                        <span className={`h-1.5 w-1.5 rounded-full ${statusStyles.dot}`} />
                        {statusStyles.label}
                      </span>
                      <span className="font-mono text-xs text-muted-foreground">
                        {isClient ? "Client Role" : "Freelancer Role"}
                      </span>
                      {contract.job && (
                        <span className="font-mono text-xs font-semibold text-foreground ml-auto sm:ml-2">
                          {formatJobBudget(contract.job)}
                        </span>
                      )}
                    </div>

                    <Link href={`/contracts/${contract.id}`}>
                      <h2 className="font-serif text-xl font-medium text-foreground hover:underline underline-offset-2 transition-colors">
                        {contract.job?.title || "Contract Agreement"}
                      </h2>
                    </Link>

                    <p className="font-mono text-xs text-muted-foreground">
                      Initiated {formatJobDate(contract.created_at)}
                    </p>
                  </div>

                  <div className="flex items-center gap-2.5 shrink-0 pt-2 md:pt-0">
                    <Button asChild size="sm" className="text-xs">
                      <Link href={`/contracts/${contract.id}`}>
                        <span>Manage Contract</span>
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

      {/* Tab 3: My Postings */}
      {activeTab === "postings" && (
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
                  No opportunities posted
                </h3>
                <p className="mt-1 text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
                  As a verified campus member, you can publish gigs, research tasks, and projects.
                </p>
              </div>
              <div className="pt-2">
                <Button asChild className="text-xs">
                  <Link href="/jobs/create">
                    <Plus className="h-3.5 w-3.5 mr-1.5" />
                    <span>Post Your First Gig</span>
                  </Link>
                </Button>
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
                        className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider ${statusStyles.bg} ${statusStyles.text}`}
                      >
                        <span className={`h-1.5 w-1.5 rounded-full ${statusStyles.dot}`} />
                        {job.status.replace("_", " ")}
                      </span>
                      {job.department && (
                        <Badge variant="secondary" className="font-mono text-xs">
                          {job.department}
                        </Badge>
                      )}
                      <span className="font-mono text-xs font-semibold text-foreground ml-auto sm:ml-2">
                        {formatJobBudget(job)}
                      </span>
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
