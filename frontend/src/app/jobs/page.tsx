"use client";

import { useState, useEffect, Suspense, useMemo, useCallback, useRef } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { Briefcase, AlertCircle, RotateCcw, ShieldCheck, PlusCircle } from "lucide-react";
import { Job, JobStatus } from "@/types/api";
import { listJobs } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import { JobCard } from "@/components/jobs/JobCard";
import { JobFilterBar, JobFilterValues } from "@/components/jobs/JobFilterBar";
import { formatJobBudget } from "@/lib/formatters";
import { animateStaggerList } from "@/lib/animations";
import { Badge } from "@/components/ui/badge";

function JobSearchContent() {
  const searchParams = useSearchParams();
  const { isAuthenticated } = useAuth();
  const gridRef = useRef<HTMLDivElement | null>(null);

  // Read initial filter values from URL query string if provided
  const initialSearch = searchParams.get("search") || "";
  const initialDept = searchParams.get("department") || "";
  const initialSkill = searchParams.get("skill") || "";

  const [filters, setFilters] = useState<JobFilterValues>({
    search: initialSearch,
    department: initialDept,
    skill: initialSkill,
    status: "open",
    budgetSort: "",
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
            status: (filters.status as JobStatus) || undefined,
          },
          client
        ),
      [filters.search, filters.department, filters.skill, filters.status]
    ),
    [filters.search, filters.department, filters.skill, filters.status]
  );

  useEffect(() => {
    if (gridRef.current && jobs && jobs.length > 0) {
      return animateStaggerList(gridRef.current);
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
      status: "open",
      budgetSort: "",
    });
  };

  // Sort jobs by budget rate if requested
  const jobList = useMemo(() => {
    let list = jobs ?? [];
    if (filters.budgetSort) {
      list = [...list].sort((a, b) => {
        const getRate = (j: Job) => {
          const bStr = formatJobBudget(j);
          const match = bStr.match(/\$(\d+)/);
          return match ? parseInt(match[1], 10) : 0;
        };
        const rateA = getRate(a);
        const rateB = getRate(b);
        return filters.budgetSort === "high" ? rateB - rateA : rateA - rateB;
      });
    }
    return list;
  }, [jobs, filters.budgetSort]);

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Editorial Header */}
      <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
        <div>
          <Badge variant="outline" className="font-mono text-xs mb-3">
            Campus Marketplace
          </Badge>
          <h1 className="font-serif text-4xl sm:text-5xl font-normal tracking-tight text-foreground">
            Campus Opportunities
          </h1>
          <p className="mt-2 max-w-2xl text-sm sm:text-base text-muted-foreground">
            Browse peer freelance projects, campus research gigs, and contract deliverables.
          </p>
        </div>

        {/* Action Button for Campus Members */}
        {isAuthenticated && (
          <Link
            href="/jobs/create"
            className="inline-flex items-center gap-2 self-start rounded-lg bg-foreground px-4 py-2.5 text-xs font-mono font-medium text-background transition hover:bg-foreground/90"
          >
            <PlusCircle className="h-4 w-4" />
            Post a Gig
          </Link>
        )}
      </div>

      {/* Institutional Verification Notice */}
      <div className="mt-6 flex items-center justify-between rounded-xl border border-border bg-card p-4 text-xs text-muted-foreground">
        <div className="flex items-center gap-2.5">
          <ShieldCheck className="h-4 w-4 shrink-0 text-foreground" />
          <span>
            <strong className="text-foreground font-medium">High-Trust Campus Verification:</strong>{" "}
            All gig applications and contracts are reserved for verified university members with
            institutional (.edu) credentials.
          </span>
        </div>
        {!isAuthenticated && (
          <Link
            href="/login"
            className="hidden shrink-0 font-mono text-xs underline hover:text-foreground sm:inline"
          >
            Sign in to apply &rarr;
          </Link>
        )}
      </div>

      {/* Filter Toolbar */}
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
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mt-8">
            {Array.from({ length: 6 }).map((_, i) => (
              <div
                key={i}
                className="flex flex-col justify-between rounded-xl border border-border bg-card p-6 shadow-none"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <div className="h-5 w-20 animate-pulse rounded-full bg-muted" />
                    <div className="h-5 w-24 animate-pulse rounded-full bg-muted" />
                  </div>
                  <div className="mt-4 h-6 w-3/4 animate-pulse rounded-md bg-muted" />
                  <div className="mt-2 h-4 w-1/3 animate-pulse rounded-md bg-muted" />
                  <div className="mt-4 space-y-2">
                    <div className="h-4 w-full animate-pulse rounded-md bg-muted/60" />
                    <div className="h-4 w-4/5 animate-pulse rounded-md bg-muted/60" />
                  </div>
                  <div className="mt-4 flex gap-1.5">
                    <div className="h-5 w-14 animate-pulse rounded-md bg-muted" />
                    <div className="h-5 w-16 animate-pulse rounded-md bg-muted" />
                    <div className="h-5 w-12 animate-pulse rounded-md bg-muted" />
                  </div>
                </div>
                <div className="mt-6 flex items-center justify-between border-t border-border pt-4">
                  <div className="h-4 w-24 animate-pulse rounded-md bg-muted" />
                  <div className="h-4 w-20 animate-pulse rounded-md bg-muted" />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Error State */}
        {!isLoading && error && (
          <div className="rounded-xl border border-destructive/20 bg-card p-8 text-center mt-8">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-pastel-red text-pastel-redText">
              <AlertCircle className="h-6 w-6" />
            </div>
            <h2 className="mt-4 font-serif text-xl font-medium text-foreground">
              Unable to load campus opportunities
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              {error.message || "A network or server error occurred while retrieving job listings."}
            </p>
            <div className="mt-6 flex justify-center gap-3">
              <button
                type="button"
                onClick={() => refetch()}
                className="inline-flex items-center gap-2 rounded-lg bg-foreground px-4 py-2 text-xs font-mono font-medium text-background transition hover:bg-foreground/90"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Retry Search
              </button>
            </div>
          </div>
        )}

        {/* Empty State */}
        {!isLoading && !error && jobList.length === 0 && (
          <div className="rounded-xl border border-dashed border-border bg-card p-12 text-center mt-8">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <Briefcase className="h-6 w-6" />
            </div>
            <h2 className="mt-4 font-serif text-xl font-medium text-foreground">
              No campus gigs found
            </h2>
            <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
              No active campus gigs matching this filter. Clear filters or post a new project.
            </p>
            <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
              <button
                type="button"
                onClick={handleResetFilters}
                className="inline-flex items-center gap-2 rounded-lg border border-border bg-background px-4 py-2 text-xs font-mono font-medium text-foreground transition hover:border-foreground/40"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Clear Filters
              </button>
              <Link
                href="/jobs/create"
                className="inline-flex items-center gap-2 rounded-lg bg-foreground px-4 py-2 text-xs font-mono font-medium text-background transition hover:bg-foreground/90"
              >
                <PlusCircle className="h-4 w-4" />
                Post a Gig
              </Link>
            </div>
          </div>
        )}

        {/* Job Cards Grid */}
        {!isLoading && !error && jobList.length > 0 && (
          <div ref={gridRef} className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mt-8">
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
        <div className="mx-auto max-w-7xl px-4 py-12 text-center font-mono text-xs text-muted-foreground">
          Loading campus opportunities...
        </div>
      }
    >
      <JobSearchContent />
    </Suspense>
  );
}
