"use client";

import { useState, useCallback, useMemo } from "react";
import Link from "next/link";
import {
  Briefcase,
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
  Sparkles,
  CheckCircle2,
  ExternalLink,
} from "lucide-react";
import { Job, JobStatus } from "@/types/api";
import { getMyJobs } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatBudget,
  formatJobDate,
  getStatusBadgeClasses,
} from "@/lib/formatters";
import { cn } from "@/lib/utils";

type JobFilterStatus = "all" | JobStatus;

export default function EmployerJobsPage() {
  const { user, isAuthenticated, role, isLoading: authLoading, login } = useAuth();

  const [searchQuery, setSearchQuery] = useState("");
  const [statusFilter, setStatusFilter] = useState<JobFilterStatus>("all");

  // Fetch jobs posted by the employer
  const {
    data: jobs,
    isLoading: jobsLoading,
    error: jobsError,
    refetch: refetchJobs,
  } = useQuery<Job[]>(
    useCallback((client) => getMyJobs(client), []),
    {
      enabled: Boolean(isAuthenticated && role === "employer"),
    }
  );

  const jobList = useMemo(() => jobs ?? [], [jobs]);

  // Metrics
  const totalCount = jobList.length;
  const openCount = jobList.filter((j) => j.status === "open").length;
  const inProgressCount = jobList.filter((j) => j.status === "in_progress").length;
  const closedCount = jobList.filter((j) => j.status === "closed" || j.status === "cancelled").length;

  // Filtered list
  const filteredJobs = useMemo(() => {
    return jobList.filter((j) => {
      if (statusFilter !== "all" && j.status !== statusFilter) {
        return false;
      }
      if (searchQuery.trim()) {
        const query = searchQuery.toLowerCase();
        const matchesTitle = j.title.toLowerCase().includes(query);
        const matchesDept = j.department.toLowerCase().includes(query);
        const matchesSkill = j.required_skills?.some((s) =>
          s.toLowerCase().includes(query)
        );
        if (!matchesTitle && !matchesDept && !matchesSkill) {
          return false;
        }
      }
      return true;
    });
  }, [jobList, statusFilter, searchQuery]);

  // Loading state
  if (authLoading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="h-8 w-48 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
        <div className="mt-6 grid grid-cols-1 gap-6 sm:grid-cols-2 lg:grid-cols-4">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="h-24 animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800/60"
            />
          ))}
        </div>
      </div>
    );
  }

  // Not authenticated gate
  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8 text-center">
        <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
            <Lock className="h-7 w-7" />
          </div>
          <h1 className="mt-4 text-2xl font-bold text-slate-900 dark:text-white">
            Employer Sign In Required
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            Sign in with your employer account to manage job postings and applicant proposals.
          </p>
          <div className="mt-6">
            <button
              onClick={() =>
                login({
                  redirectPath: "/employer/jobs",
                  roleHint: "employer",
                })
              }
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-primary"
            >
              <Briefcase className="h-4 w-4" />
              <span>Sign In as Employer</span>
            </button>
          </div>
        </div>
      </div>
    );
  }

  // Non-employer role gate
  if (role !== "employer") {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8 text-center">
        <div className="rounded-[10px] border border-amber-200 bg-amber-50/60 p-8 shadow-sm dark:border-amber-900/50 dark:bg-amber-950/30">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300">
            <AlertCircle className="h-7 w-7" />
          </div>
          <h1 className="mt-4 text-2xl font-bold text-slate-900 dark:text-white">
            Employer Dashboard Access
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
            You are logged in as a student (<span className="font-semibold">{user?.email}</span>).
            Managing postings is reserved for campus employers.
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <Link
              href="/jobs"
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-5 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-primary"
            >
              <GraduationCap className="h-4 w-4" />
              <span>Explore Student Gigs</span>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Header Row */}
      <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
        <div>
          <div className="inline-flex items-center gap-2 rounded-[10px] border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300">
            <Briefcase className="h-3.5 w-3.5" />
            <span>Employer Management Center</span>
          </div>

          <h1 className="mt-2 text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
            My Job Postings
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            Review applicant proposals, initiate contracts, and manage campus gigs posted by your department.
          </p>
        </div>

        {/* Quick Post Action */}
        <Link
          href="/jobs/create"
          className="inline-flex items-center gap-2 self-start rounded-[10px] bg-primary text-primary-foreground px-5 py-2.5 text-sm font-semibold text-white shadow-md shadow-sm transition hover:bg-primary md:self-auto"
        >
          <PlusCircle className="h-4 w-4" />
          <span>Post a New Job</span>
        </Link>
      </div>

      {/* KPI Metrics Summary Row */}
      <div className="mt-8 grid grid-cols-2 gap-4 sm:grid-cols-4">
        {/* Metric 1: Total */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            Total Postings
          </span>
          <p className="mt-2 text-3xl font-extrabold text-slate-900 dark:text-white">
            {totalCount}
          </p>
        </div>

        {/* Metric 2: Open */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-emerald-600 dark:text-emerald-400">
            Active / Open
          </span>
          <p className="mt-2 text-3xl font-extrabold text-slate-900 dark:text-white">
            {openCount}
          </p>
        </div>

        {/* Metric 3: In Progress */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-primary dark:text-emerald-500">
            In Progress
          </span>
          <p className="mt-2 text-3xl font-extrabold text-slate-900 dark:text-white">
            {inProgressCount}
          </p>
        </div>

        {/* Metric 4: Closed */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-5 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
            Closed / Completed
          </span>
          <p className="mt-2 text-3xl font-extrabold text-slate-900 dark:text-white">
            {closedCount}
          </p>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <div className="mt-8 flex flex-col justify-between gap-4 rounded-[10px] border border-slate-200 bg-white p-4 shadow-sm md:flex-row md:items-center dark:border-slate-800 dark:bg-slate-900">
        {/* Status Filter Tabs */}
        <div className="flex flex-wrap items-center gap-1.5">
          <button
            type="button"
            onClick={() => setStatusFilter("all")}
            className={cn(
              "rounded-[10px] px-3.5 py-1.5 text-xs font-semibold transition",
              statusFilter === "all"
                ? "bg-slate-900 text-white dark:bg-white dark:text-slate-900"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            All ({totalCount})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter("open")}
            className={cn(
              "rounded-[10px] px-3.5 py-1.5 text-xs font-semibold transition",
              statusFilter === "open"
                ? "bg-emerald-600 text-white"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            Open ({openCount})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter("in_progress")}
            className={cn(
              "rounded-[10px] px-3.5 py-1.5 text-xs font-semibold transition",
              statusFilter === "in_progress"
                ? "bg-primary text-primary-foreground text-white"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            In Progress ({inProgressCount})
          </button>
          <button
            type="button"
            onClick={() => setStatusFilter("closed")}
            className={cn(
              "rounded-[10px] px-3.5 py-1.5 text-xs font-semibold transition",
              statusFilter === "closed"
                ? "bg-slate-600 text-white"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            Closed ({closedCount})
          </button>
        </div>

        {/* Search Field */}
        <div className="relative w-full md:w-72">
          <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-slate-400">
            <Search className="h-4 w-4" />
          </div>
          <input
            type="text"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            placeholder="Filter postings by title or skill..."
            className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 py-2 pl-9 pr-4 text-xs text-slate-900 placeholder:text-slate-400 focus:border-primary focus:bg-white focus:outline-none focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900"
          />
        </div>
      </div>

      {/* Main Postings List */}
      <div className="mt-6">
        {/* Loading Skeletons */}
        {jobsLoading && (
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <div
                key={i}
                className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
              >
                <div className="flex items-center justify-between">
                  <div className="h-6 w-1/3 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
                  <div className="h-6 w-20 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
                </div>
                <div className="mt-4 h-4 w-2/3 animate-pulse rounded bg-slate-100 dark:bg-slate-800" />
                <div className="mt-6 flex justify-between border-t border-slate-100 pt-4 dark:border-slate-800">
                  <div className="h-5 w-28 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
                  <div className="h-8 w-32 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Error State */}
        {!jobsLoading && jobsError && (
          <div className="rounded-[10px] border border-rose-200 bg-rose-50/60 p-8 text-center dark:border-rose-900/50 dark:bg-rose-950/30">
            <AlertCircle className="mx-auto h-8 w-8 text-rose-600 dark:text-rose-400" />
            <h3 className="mt-2 text-base font-bold text-slate-900 dark:text-white">
              Failed to load postings
            </h3>
            <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
              {jobsError.message || "An unexpected error occurred while fetching your jobs."}
            </p>
            <button
              onClick={() => refetchJobs()}
              className="mt-4 inline-flex items-center gap-1.5 rounded-[10px] bg-primary text-primary-foreground px-4 py-2 text-xs font-semibold text-white hover:bg-primary"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Try Again
            </button>
          </div>
        )}

        {/* Empty State: 0 Jobs Posted */}
        {!jobsLoading && !jobsError && jobList.length === 0 && (
          <div className="rounded-[10px] border border-slate-200 bg-white p-12 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
              <Briefcase className="h-8 w-8" />
            </div>
            <h3 className="mt-4 text-lg font-bold text-slate-900 dark:text-white">
              You haven&apos;t posted any campus jobs yet
            </h3>
            <p className="mx-auto mt-2 max-w-md text-xs leading-relaxed text-slate-500 dark:text-slate-400">
              Publish your first campus opportunity to connect with verified university student freelancers for research projects, software development, design, and department gigs.
            </p>
            <div className="mt-6">
              <Link
                href="/jobs/create"
                className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-3 text-sm font-semibold text-white shadow-md shadow-sm hover:bg-primary"
              >
                <PlusCircle className="h-4 w-4" />
                <span>Post Your First Job</span>
              </Link>
            </div>
          </div>
        )}

        {/* Empty State: 0 Matches for Filter */}
        {!jobsLoading && !jobsError && jobList.length > 0 && filteredJobs.length === 0 && (
          <div className="rounded-[10px] border border-slate-200 bg-white p-10 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <Search className="mx-auto h-8 w-8 text-slate-400" />
            <h3 className="mt-3 text-sm font-bold text-slate-900 dark:text-white">
              No postings match your filter
            </h3>
            <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
              Try adjusting your search query or reset status filters.
            </p>
            <button
              onClick={() => {
                setStatusFilter("all");
                setSearchQuery("");
              }}
              className="mt-4 text-xs font-semibold text-primary hover:underline dark:text-emerald-500"
            >
              Reset Filters
            </button>
          </div>
        )}

        {/* Job Cards */}
        {!jobsLoading && !jobsError && filteredJobs.length > 0 && (
          <div className="space-y-4">
            {filteredJobs.map((job) => {
              const statusStyles = getStatusBadgeClasses(job.status);

              return (
                <div
                  key={job.id}
                  className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm transition hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700 sm:p-7"
                >
                  <div className="flex flex-col justify-between gap-4 md:flex-row md:items-start">
                    <div className="flex-1">
                      {/* Department & Status Badges */}
                      <div className="flex flex-wrap items-center gap-2">
                        <span
                          className={cn(
                            "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-[11px] font-semibold uppercase tracking-wider",
                            statusStyles.bg,
                            statusStyles.text
                          )}
                        >
                          <span
                            className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)}
                          />
                          {job.status.replace("_", " ")}
                        </span>

                        {job.department && (
                          <span className="inline-flex items-center gap-1 rounded-md bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                            <GraduationCap className="h-3 w-3 text-slate-400" />
                            <span>{job.department}</span>
                          </span>
                        )}
                      </div>

                      {/* Job Title */}
                      <h2 className="mt-2.5 text-lg font-bold text-slate-900 dark:text-white">
                        <Link
                          href={`/jobs/${job.id}`}
                          className="hover:text-primary dark:hover:text-primary"
                        >
                          {job.title}
                        </Link>
                      </h2>

                      {/* Description Excerpt */}
                      <p className="mt-1 line-clamp-2 text-xs leading-relaxed text-slate-600 dark:text-slate-400">
                        {job.description}
                      </p>

                      {/* Skills Tags */}
                      {job.required_skills && job.required_skills.length > 0 && (
                        <div className="mt-3 flex flex-wrap gap-1.5">
                          {job.required_skills.slice(0, 5).map((skill) => (
                            <span
                              key={skill}
                              className="rounded-md border border-slate-200 bg-slate-50 px-2 py-0.5 text-[10px] font-medium text-slate-600 dark:border-slate-800 dark:bg-slate-800/60 dark:text-slate-400"
                            >
                              {skill}
                            </span>
                          ))}
                          {job.required_skills.length > 5 && (
                            <span className="text-[10px] text-slate-400 self-center">
                              +{job.required_skills.length - 5} more
                            </span>
                          )}
                        </div>
                      )}
                    </div>

                    {/* Budget & Timeline Column */}
                    <div className="flex flex-col md:items-end text-left md:text-right">
                      <div className="text-base font-extrabold text-slate-900 dark:text-white">
                        {formatBudget(job.budget, job.pay_type)}
                      </div>

                      <div className="mt-1.5 flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-slate-500 dark:text-slate-400">
                        {job.deadline && (
                          <span className="flex items-center gap-1">
                            <Calendar className="h-3.5 w-3.5 text-slate-400" />
                            Due {formatJobDate(job.deadline)}
                          </span>
                        )}
                        <span className="flex items-center gap-1">
                          <Clock className="h-3.5 w-3.5 text-slate-400" />
                          Posted {formatJobDate(job.created_at)}
                        </span>
                      </div>
                    </div>
                  </div>

                  {/* Actions Row */}
                  <div className="mt-6 flex flex-wrap items-center justify-between gap-3 border-t border-slate-100 pt-4 dark:border-slate-800">
                    <Link
                      href={`/jobs/${job.id}`}
                      className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 transition hover:text-primary dark:text-slate-400 dark:hover:text-primary"
                    >
                      <span>Public Listing</span>
                      <ExternalLink className="h-3.5 w-3.5" />
                    </Link>

                    <div className="flex items-center gap-3">
                      <Link
                        href={`/jobs/${job.id}/applicants`}
                        className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-4 py-2 text-xs font-semibold text-white shadow-sm transition hover:bg-primary"
                      >
                        <Users className="h-3.5 w-3.5" />
                        <span>Review Applicants</span>
                        <ArrowRight className="h-3.5 w-3.5" />
                      </Link>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
}
