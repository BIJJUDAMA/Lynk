"use client";

import { useState, useEffect, Suspense, useMemo, useCallback } from "react";
import { useSearchParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  Briefcase,
  Sparkles,
  AlertCircle,
  RotateCcw,
  ShieldCheck,
  PlusCircle,
} from "lucide-react";
import { Job, JobStatus, JobPayType } from "@/types/api";
import { listJobs } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import { JobCard } from "@/components/jobs/JobCard";
import { animateStaggerList } from "@/lib/animations";
import { useRef } from "react";
import {
  JobFilterBar,
  JobFilterValues,
} from "@/components/jobs/JobFilterBar";

function JobSearchContent() {
  const searchParams = useSearchParams();
  const router = useRouter();
  const { isAuthenticated, role, isVerified } = useAuth();
  const gridRef = useRef<HTMLDivElement | null>(null);

  // Read initial filter values from URL query string if provided
  const initialSearch = searchParams.get("search") || "";
  const initialDept = searchParams.get("department") || "";
  const initialSkill = searchParams.get("skill") || "";
  const initialPayType = (searchParams.get("pay_type") as JobPayType) || "";
  const initialMinBudget = searchParams.get("min_budget_cents") || searchParams.get("min_budget") || "";
  const initialMaxBudget = searchParams.get("max_budget_cents") || searchParams.get("max_budget") || "";

  const [filters, setFilters] = useState<JobFilterValues>({
    search: initialSearch,
    department: initialDept,
    skill: initialSkill,
    pay_type: initialPayType,
    min_budget: initialMinBudget,
    max_budget: initialMaxBudget,
    status: "open",
  });

  // Query jobs from API with the current filters
  const {
    data: jobs,
    isLoading,
    error,
    refetch,
  } = useQuery<Job[]>(
    useCallback(
      (client) =>
        listJobs(
          {
            search: filters.search.trim() || undefined,
            department: filters.department.trim() || undefined,
            skill: filters.skill.trim() || undefined,
            pay_type: (filters.pay_type as JobPayType) || undefined,
            min_budget_cents: filters.min_budget
              ? Math.round(Number(filters.min_budget) * 100)
              : undefined,
            max_budget_cents: filters.max_budget
              ? Math.round(Number(filters.max_budget) * 100)
              : undefined,
            status: (filters.status as JobStatus) || undefined,
          },
          client
        ),
      [filters]
    ),
    [filters]
  );

  useEffect(() => {
    if (gridRef.current && jobs && jobs.length > 0) {
      animateStaggerList(gridRef.current);
    }
  }, [jobs]);

  const handleFilterChange = (newFilters: JobFilterValues) => {
    setFilters(newFilters);
  };

  const handleResetFilters = () => {
    setFilters({
      search: "",
      department: "",
      skill: "",
      pay_type: "",
      min_budget: "",
      max_budget: "",
      status: "open",
    });
  };

  const jobList = jobs ?? [];

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Header Section */}
      <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
        <div>
          <div className="inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50/70 px-3 py-1 text-xs font-semibold text-emerald-700 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300">
            <Sparkles className="h-3.5 w-3.5" />
            <span>Verified Campus Opportunities</span>
          </div>

          <h1 className="mt-2 text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
            Explore Campus Jobs & Gigs
          </h1>
          <p className="mt-2 max-w-2xl text-sm text-slate-600 dark:text-slate-400">
            Find freelance projects, campus jobs, and research opportunities posted by verified university employers and departments.
          </p>
        </div>

        {/* Action Button for Campus Members */}
        {isAuthenticated && (
          <Link
            href="/jobs/create"
            className="inline-flex items-center gap-2 self-start rounded-[10px] bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-emerald-500"
          >
            <PlusCircle className="h-4 w-4" />
            Post a New Job
          </Link>
        )}
      </div>

      {/* Institutional Verification Notice */}
      <div className="mt-6 flex items-center justify-between rounded-[10px] border border-emerald-100 bg-emerald-50/50 p-3.5 text-xs text-emerald-900 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-200">
        <div className="flex items-center gap-2">
          <ShieldCheck className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
          <span>
            <strong>High-Trust Guarantee:</strong> All job applications are strictly reserved for verified university students with institutional (.edu) credentials.
          </span>
        </div>
        {!isAuthenticated && (
          <Link
            href="/login"
            className="hidden shrink-0 font-semibold underline hover:text-emerald-600 sm:inline"
          >
            Sign in to apply &rarr;
          </Link>
        )}
      </div>

      {/* Filter Bar */}
      <div className="mt-6">
        <JobFilterBar
          filters={filters}
          onFilterChange={handleFilterChange}
          onReset={handleResetFilters}
          totalCount={jobList.length}
          isLoading={isLoading}
        />
      </div>

      {/* Main Content Area */}
      <div className="mt-8">
        {/* Loading Skeletons */}
        {isLoading && (
          <div className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {Array.from({ length: 6 }).map((_, i) => (
              <div
                key={i}
                className="flex flex-col justify-between rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900/80"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <div className="h-5 w-20 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
                    <div className="h-5 w-24 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
                  </div>
                  <div className="mt-4 h-6 w-3/4 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                  <div className="mt-2 h-4 w-1/2 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                  <div className="mt-4 space-y-2">
                    <div className="h-4 w-full animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
                    <div className="h-4 w-4/5 animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
                  </div>
                  <div className="mt-4 flex gap-1.5">
                    <div className="h-5 w-14 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                    <div className="h-5 w-16 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                    <div className="h-5 w-12 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                  </div>
                </div>
                <div className="mt-6 flex items-center justify-between border-t border-slate-100 pt-4 dark:border-slate-800/80">
                  <div className="h-4 w-28 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                  <div className="h-4 w-20 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Error State */}
        {!isLoading && error && (
          <div className="rounded-[10px] border border-rose-200 bg-rose-50/60 p-8 text-center dark:border-rose-900/50 dark:bg-rose-950/30">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-[10px] bg-rose-100 text-rose-600 dark:bg-rose-900/50 dark:text-rose-400">
              <AlertCircle className="h-6 w-6" />
            </div>
            <h2 className="mt-4 text-lg font-bold text-slate-900 dark:text-white">
              Unable to load campus jobs
            </h2>
            <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
              {error.message || "A network or server error occurred while retrieving job listings."}
            </p>
            <div className="mt-6 flex justify-center gap-3">
              <button
                type="button"
                onClick={() => refetch()}
                className="inline-flex items-center gap-2 rounded-[10px] bg-emerald-600 px-4 py-2.5 text-xs font-semibold text-white shadow-sm transition hover:bg-emerald-500"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Retry Search
              </button>
            </div>
          </div>
        )}

        {/* Empty State */}
        {!isLoading && !error && jobList.length === 0 && (
          <div className="rounded-[10px] border border-dashed border-slate-300 bg-white p-12 text-center dark:border-slate-800 dark:bg-slate-900/60">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-[10px] bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400">
              <Briefcase className="h-6 w-6" />
            </div>
            <h2 className="mt-4 text-base font-bold text-slate-900 dark:text-white">
              No campus gigs match your filters
            </h2>
            <p className="mx-auto mt-2 max-w-sm text-xs text-slate-500 dark:text-slate-400">
              Try adjusting your search criteria, clearing specific skill tags, or resetting all filters.
            </p>
            <div className="mt-6">
              <button
                type="button"
                onClick={handleResetFilters}
                className="inline-flex items-center gap-2 rounded-[10px] border border-slate-300 bg-white px-4 py-2 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Reset All Filters
              </button>
            </div>
          </div>
        )}

        {/* Job Cards Grid */}
        {!isLoading && !error && jobList.length > 0 && (
          <div ref={gridRef} className="grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
            {jobList.map((job) => (
              <JobCard key={job.id} job={job} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export default function JobsPage() {
  return (
    <Suspense
      fallback={
        <div className="mx-auto max-w-7xl px-4 py-12 text-center text-sm text-slate-500">
          Loading jobs...
        </div>
      }
    >
      <JobSearchContent />
    </Suspense>
  );
}
