"use client";

import React, { useState, useMemo } from "react";
import Link from "next/link";
import {
  FileText,
  Clock,
  CheckCircle2,
  XCircle,
  ChevronRight,
  Search,
  GraduationCap,
  Briefcase,
  DollarSign,
  ArrowRight,
  ExternalLink,
  ShieldCheck,
  ShieldAlert,
  Inbox,
  Calendar,
  Layers,
  ChevronDown,
  ChevronUp,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { getMyApplications } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import {
  formatBudget,
  formatJobDate,
  getApplicationStatusBadgeClasses,
} from "@/lib/formatters";
import type { ApplicationStatus, ApplicationWithDetails } from "@/types/api";

type FilterStatus = "all" | ApplicationStatus;

export default function ApplicationsPage() {
  const {
    user,
    role,
    isVerified,
    isAuthenticated,
    isLoading: isAuthLoading,
    login,
  } = useAuth();

  const [activeFilter, setActiveFilter] = useState<FilterStatus>("all");
  const [searchQuery, setSearchQuery] = useState("");
  const [expandedProposalIds, setExpandedProposalIds] = useState<Set<string>>(
    new Set()
  );

  // Fetch student's submitted applications
  const {
    data: applications,
    isLoading: isAppsLoading,
    error,
    refetch,
  } = useQuery(getMyApplications, {
    enabled: isAuthenticated && role === "student",
  });

  const toggleExpand = (id: string) => {
    setExpandedProposalIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  };

  // Compute status counts
  const counts = useMemo(() => {
    const list = applications || [];
    return {
      all: list.length,
      pending: list.filter((a) => a.status === "pending").length,
      accepted: list.filter((a) => a.status === "accepted").length,
      rejected: list.filter((a) => a.status === "rejected").length,
    };
  }, [applications]);

  // Filter & search applications
  const filteredApplications = useMemo(() => {
    if (!applications) return [];
    return applications.filter((app) => {
      // Status filter
      if (activeFilter !== "all" && app.status !== activeFilter) {
        return false;
      }

      // Search filter (matches job title, department, or cover letter keywords)
      if (searchQuery.trim()) {
        const query = searchQuery.toLowerCase().trim();
        const titleMatch = app.job?.title?.toLowerCase().includes(query) ?? false;
        const deptMatch =
          app.job?.department?.toLowerCase().includes(query) ?? false;
        const letterMatch = app.cover_letter?.toLowerCase().includes(query) ?? false;
        return titleMatch || deptMatch || letterMatch;
      }

      return true;
    });
  }, [applications, activeFilter, searchQuery]);

  // Auth Loading Skeleton
  if (isAuthLoading) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-12 sm:px-6 lg:px-8 animate-pulse space-y-8">
        <div className="h-10 w-72 rounded-[10px] bg-slate-200 dark:bg-slate-800" />
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div key={i} className="h-24 rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
          ))}
        </div>
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div key={i} className="h-44 rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
          ))}
        </div>
      </div>
    );
  }

  // Unauthenticated Guard
  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 text-center sm:px-6 lg:px-8">
        <div className="rounded-[10px] border border-slate-200 bg-white p-10 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500 mb-6">
            <FileText className="h-8 w-8" />
          </div>
          <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
            Sign In to View Applications
          </h2>
          <p className="mx-auto mt-3 max-w-md text-sm text-slate-600 dark:text-slate-400">
            Please log in with your verified university account to track your submitted proposals, status updates, and active contracts.
          </p>
          <div className="mt-8">
            <button
              onClick={() => login({ redirectPath: "/applications" })}
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-sm transition hover:bg-primary"
            >
              <span>Sign In with University Email</span>
              <ArrowRight className="h-4 w-4" />
            </button>
          </div>
        </div>
      </div>
    );
  }

  // Employer Notice (if user signed in as employer)
  if (role === "employer") {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:px-8 text-center">
        <div className="rounded-[10px] border border-slate-200 bg-white p-10 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500 mb-6">
            <Briefcase className="h-8 w-8" />
          </div>
          <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
            Employer Dashboard Notice
          </h2>
          <p className="mx-auto mt-3 max-w-md text-sm text-slate-600 dark:text-slate-400 leading-relaxed">
            You are currently signed in with an <strong>Employer</strong> account. The Applications Tracking dashboard is tailored for student freelancers tracking their submitted proposals.
          </p>
          <div className="mt-8 flex flex-wrap items-center justify-center gap-4">
            <Link
              href="/jobs"
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-5 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-primary"
            >
              <span>Explore Campus Gigs</span>
              <ArrowRight className="h-4 w-4" />
            </Link>
            <Link
              href="/profile"
              className="inline-flex items-center gap-2 rounded-[10px] border border-slate-200 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-200"
            >
              <span>View Organization Profile</span>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Page Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 mb-8">
        <div>
          <div className="flex items-center gap-3">
            <h1 className="text-2xl sm:text-3xl font-bold tracking-tight text-slate-900 dark:text-white">
              My Job Applications
            </h1>
            {isVerified ? (
              <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2.5 py-0.5 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/60">
                <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                Verified
              </span>
            ) : (
              <span className="inline-flex items-center gap-1 rounded-full bg-amber-50 px-2.5 py-0.5 text-xs font-semibold text-amber-700 dark:bg-amber-950/60 dark:text-amber-300 border border-amber-200 dark:border-amber-800/60">
                <ShieldAlert className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                Unverified Email
              </span>
            )}
          </div>
          <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">
            Track your proposals, review employer responses, and navigate to active contracts.
          </p>
        </div>

        <Link
          href="/jobs"
          className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-4 py-2.5 text-xs font-semibold text-white shadow-sm shadow-sm transition hover:bg-primary self-start sm:self-auto"
        >
          <Search className="h-4 w-4" />
          <span>Browse More Gigs</span>
        </Link>
      </div>

      {/* Metric Counters Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 mb-8">
        <button
          type="button"
          onClick={() => setActiveFilter("all")}
          className={`rounded-[10px] border p-4 text-left transition ${
            activeFilter === "all"
              ? "border-primary bg-emerald-50 shadow-sm dark:border-primary dark:bg-emerald-950/40"
              : "border-border bg-card hover:border-primary/50"
          }`}
        >
          <div className="flex items-center justify-between text-xs font-medium text-slate-500 dark:text-slate-400 mb-1">
            <span>Total Submitted</span>
            <Layers className="h-4 w-4 text-primary dark:text-emerald-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900 dark:text-white">
            {counts.all}
          </div>
        </button>

        <button
          type="button"
          onClick={() => setActiveFilter("pending")}
          className={`rounded-[10px] border p-4 text-left transition ${
            activeFilter === "pending"
              ? "border-amber-500 bg-amber-50/50 shadow-sm dark:border-amber-400 dark:bg-amber-950/40"
              : "border-slate-200/80 bg-white hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900"
          }`}
        >
          <div className="flex items-center justify-between text-xs font-medium text-amber-600 dark:text-amber-400 mb-1">
            <span>Pending Review</span>
            <Clock className="h-4 w-4 text-amber-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900 dark:text-white">
            {counts.pending}
          </div>
        </button>

        <button
          type="button"
          onClick={() => setActiveFilter("accepted")}
          className={`rounded-[10px] border p-4 text-left transition ${
            activeFilter === "accepted"
              ? "border-emerald-500 bg-emerald-50/50 shadow-sm dark:border-emerald-400 dark:bg-emerald-950/40"
              : "border-slate-200/80 bg-white hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900"
          }`}
        >
          <div className="flex items-center justify-between text-xs font-medium text-emerald-600 dark:text-emerald-400 mb-1">
            <span>Accepted Gigs</span>
            <CheckCircle2 className="h-4 w-4 text-emerald-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900 dark:text-white">
            {counts.accepted}
          </div>
        </button>

        <button
          type="button"
          onClick={() => setActiveFilter("rejected")}
          className={`rounded-[10px] border p-4 text-left transition ${
            activeFilter === "rejected"
              ? "border-rose-500 bg-rose-50/50 shadow-sm dark:border-rose-400 dark:bg-rose-950/40"
              : "border-slate-200/80 bg-white hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900"
          }`}
        >
          <div className="flex items-center justify-between text-xs font-medium text-rose-600 dark:text-rose-400 mb-1">
            <span>Not Selected</span>
            <XCircle className="h-4 w-4 text-rose-500" />
          </div>
          <div className="text-2xl font-bold text-slate-900 dark:text-white">
            {counts.rejected}
          </div>
        </button>
      </div>

      {/* Filter and Search Bar */}
      <div className="rounded-[10px] border border-slate-200/80 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900 mb-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
          {/* Status Filter Pills */}
          <div className="flex flex-wrap items-center gap-1.5">
            {(["all", "pending", "accepted", "rejected"] as FilterStatus[]).map(
              (filter) => (
                <button
                  key={filter}
                  type="button"
                  onClick={() => setActiveFilter(filter)}
                  className={`rounded-[10px] px-3.5 py-1.5 text-xs font-semibold capitalize transition ${
                    activeFilter === filter
                      ? "bg-primary text-primary-foreground text-white shadow-sm"
                      : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
                  }`}
                >
                  {filter} ({counts[filter]})
                </button>
              )
            )}
          </div>

          {/* Search Query Input */}
          <div className="relative w-full sm:w-72">
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search by job title or major..."
              className="w-full rounded-[10px] border border-slate-200 bg-slate-50/60 pl-9 pr-3.5 py-2 text-xs text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
            />
            <Search className="pointer-events-none absolute left-3 top-2.5 h-3.5 w-3.5 text-slate-400" />
          </div>
        </div>
      </div>

      {/* Loading Skeletons */}
      {isAppsLoading && (
        <div className="space-y-4">
          {Array.from({ length: 3 }).map((_, i) => (
            <div
              key={i}
              className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 animate-pulse space-y-4"
            >
              <div className="flex items-center justify-between">
                <div className="h-6 w-1/3 rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                <div className="h-6 w-24 rounded-full bg-slate-200 dark:bg-slate-800" />
              </div>
              <div className="h-4 w-1/2 rounded bg-slate-100 dark:bg-slate-800" />
              <div className="h-16 rounded-[10px] bg-slate-50 dark:bg-slate-800/50" />
            </div>
          ))}
        </div>
      )}

      {/* Error Banner */}
      {error && !isAppsLoading && (
        <div className="rounded-[10px] border border-rose-200 bg-rose-50 p-6 text-center text-sm text-rose-800 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
          <p className="font-semibold">Unable to load your applications.</p>
          <p className="mt-1 text-xs">{error.message}</p>
          <button
            onClick={() => refetch()}
            className="mt-4 rounded-[10px] bg-rose-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-rose-500"
          >
            Try Again
          </button>
        </div>
      )}

      {/* Empty State */}
      {!isAppsLoading && !error && filteredApplications.length === 0 && (
        <div className="rounded-[10px] border border-slate-200/80 bg-white p-12 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500 mb-4">
            <Inbox className="h-8 w-8" />
          </div>
          <h3 className="text-lg font-bold text-slate-900 dark:text-white">
            {searchQuery || activeFilter !== "all"
              ? "No matching applications found"
              : "No applications submitted yet"}
          </h3>
          <p className="mx-auto mt-2 max-w-sm text-xs text-slate-500 dark:text-slate-400 leading-relaxed">
            {searchQuery || activeFilter !== "all"
              ? "Try adjusting your search criteria or resetting status filters to find your proposals."
              : "Discover campus projects, research assistantships, and freelance gigs matched to your university major."}
          </p>
          <div className="mt-6 flex justify-center gap-3">
            {searchQuery || activeFilter !== "all" ? (
              <button
                type="button"
                onClick={() => {
                  setActiveFilter("all");
                  setSearchQuery("");
                }}
                className="rounded-[10px] border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-300"
              >
                Clear Filters
              </button>
            ) : (
              <Link
                href="/jobs"
                className="inline-flex items-center gap-1.5 rounded-[10px] bg-primary text-primary-foreground px-5 py-2.5 text-xs font-semibold text-white shadow-sm transition hover:bg-primary"
              >
                <Search className="h-4 w-4" />
                <span>Explore Open Campus Gigs</span>
              </Link>
            )}
          </div>
        </div>
      )}

      {/* Applications Cards List */}
      {!isAppsLoading && !error && filteredApplications.length > 0 && (
        <div className="space-y-4">
          {filteredApplications.map((app) => {
            const statusStyles = getApplicationStatusBadgeClasses(app.status);
            const isAccepted = app.status === "accepted";
            const isExpanded = expandedProposalIds.has(app.id);

            return (
              <div
                key={app.id}
                className="group rounded-[10px] border border-slate-200/80 bg-white p-6 shadow-sm transition hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700"
              >
                {/* Top Row: Job Title, Status, and Actions */}
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div>
                    <div className="flex flex-wrap items-center gap-2">
                      <Link
                        href={`/jobs/${app.job_id}`}
                        className="text-base sm:text-lg font-bold text-slate-900 dark:text-white transition group-hover:text-primary dark:group-hover:text-primary"
                      >
                        {app.job?.title || "Campus Gig"}
                      </Link>
                      <ExternalLink className="h-3.5 w-3.5 text-slate-400 opacity-60" />
                    </div>

                    <div className="mt-1.5 flex flex-wrap items-center gap-3 text-xs text-slate-500 dark:text-slate-400">
                      {app.job?.department && (
                        <span className="flex items-center gap-1 font-medium text-slate-700 dark:text-slate-300">
                          <GraduationCap className="h-3.5 w-3.5 text-primary dark:text-emerald-500" />
                          {app.job.department}
                        </span>
                      )}
                      {app.job && (
                        <>
                          <span>•</span>
                          <span className="font-semibold text-slate-900 dark:text-white">
                            {formatBudget(app.job.budget, app.job.pay_type)}
                          </span>
                        </>
                      )}
                      <span>•</span>
                      <span className="flex items-center gap-1">
                        <Calendar className="h-3 w-3" />
                        Submitted {formatJobDate(app.created_at)}
                      </span>
                    </div>
                  </div>

                  {/* Right Status Badge */}
                  <div className="flex items-center gap-2.5 self-start sm:self-auto">
                    <span
                      className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold ${statusStyles.bg} ${statusStyles.text}`}
                    >
                      <span
                        className={`h-2 w-2 rounded-full ${statusStyles.dot}`}
                      />
                      {statusStyles.label}
                    </span>
                  </div>
                </div>

                {/* Accepted Contract Callout Banner */}
                {isAccepted && (
                  <div className="mt-4 flex flex-col sm:flex-row sm:items-center justify-between gap-3 rounded-[10px] border border-emerald-200 bg-emerald-50/70 p-4 dark:border-emerald-900/50 dark:bg-emerald-950/30">
                    <div className="flex items-center gap-2.5">
                      <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-[10px] bg-emerald-600 text-white shadow-sm">
                        <CheckCircle2 className="h-4 w-4" />
                      </div>
                      <div>
                        <h4 className="text-xs font-bold text-emerald-900 dark:text-emerald-200">
                          Proposal Accepted • Contract Active
                        </h4>
                        <p className="text-[11px] text-emerald-700 dark:text-emerald-300">
                          Milestone terms agreed. Ready for progress tracking and peer review.
                        </p>
                      </div>
                    </div>

                    <Link
                      href={
                        app.contract?.id
                          ? `/contracts/${app.contract.id}`
                          : "/contracts"
                      }
                      className="inline-flex items-center gap-1.5 rounded-[10px] bg-emerald-600 px-3.5 py-2 text-xs font-semibold text-white shadow-sm shadow-emerald-600/20 transition hover:bg-emerald-500 self-start sm:self-auto"
                    >
                      <span>View Active Contract</span>
                      <ChevronRight className="h-3.5 w-3.5" />
                    </Link>
                  </div>
                )}

                {/* Proposal Cover Letter Section */}
                <div className="mt-4 rounded-[10px] border border-slate-100 bg-slate-50/60 p-4 dark:border-slate-800/80 dark:bg-slate-800/40">
                  <div className="flex items-center justify-between text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                    <span>Submitted Cover Letter</span>
                    {app.resume_key && (
                      <span className="inline-flex items-center gap-1 text-[11px] font-medium text-primary dark:text-emerald-500">
                        <FileText className="h-3 w-3" />
                        Attached Resume
                      </span>
                    )}
                  </div>

                  <p
                    className={`text-xs text-slate-600 dark:text-slate-400 leading-relaxed ${
                      !isExpanded ? "line-clamp-2" : ""
                    }`}
                  >
                    {app.cover_letter}
                  </p>

                  {app.cover_letter.length > 140 && (
                    <button
                      type="button"
                      onClick={() => toggleExpand(app.id)}
                      className="mt-2 inline-flex items-center gap-1 text-[11px] font-semibold text-primary hover:text-primary dark:text-emerald-500"
                    >
                      {isExpanded ? (
                        <>
                          <span>Show Less</span>
                          <ChevronUp className="h-3 w-3" />
                        </>
                      ) : (
                        <>
                          <span>Read Full Proposal</span>
                          <ChevronDown className="h-3 w-3" />
                        </>
                      )}
                    </button>
                  )}
                </div>

                {/* Bottom Footer Actions */}
                <div className="mt-4 pt-3 border-t border-slate-100 dark:border-slate-800 flex items-center justify-between text-xs text-slate-500 dark:text-slate-400">
                  <span className="text-[11px]">
                    Application ID: {app.id.slice(0, 8)}...
                  </span>

                  <div className="flex items-center gap-3">
                    <Link
                      href={`/jobs/${app.job_id}`}
                      className="text-xs font-medium text-slate-600 hover:text-primary dark:text-slate-400 dark:hover:text-primary transition"
                    >
                      View Job Post
                    </Link>
                    {isAccepted && (
                      <Link
                        href="/contracts"
                        className="text-xs font-semibold text-emerald-600 hover:text-emerald-700 dark:text-emerald-400 transition"
                      >
                        Go to Contracts →
                      </Link>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
