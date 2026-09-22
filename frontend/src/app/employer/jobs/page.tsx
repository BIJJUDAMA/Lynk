"use client";

import React, { useCallback, useMemo } from "react";
import Link from "next/link";
import {
  Briefcase,
  Users,
  Lock,
  Loader2,
  ArrowRight,
  Plus,
  ArrowLeft,
  RotateCcw,
} from "lucide-react";
import { getMyJobs } from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import { formatJobDate, formatJobBudget, getStatusBadgeClasses } from "@/lib/formatters";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { Job } from "@/types/api";

export default function EmployerJobsPage() {
  const { user, isAuthenticated, isLoading: authLoading, login } = useAuth();

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

  const jobsList = useMemo(() => myJobs ?? [], [myJobs]);

  if (authLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
      </div>
    );
  }

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
            Please sign in to view and manage your posted gigs, review student applicants, and track
            contracts.
          </p>
          <div className="pt-2">
            <Button onClick={() => login({ redirectPath: "/employer/jobs" })} className="text-xs">
              Sign In with Campus Account
            </Button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8 space-y-8">
      {/* Back nav & Header */}
      <div>
        <Link
          href="/activity"
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground transition-colors mb-4"
        >
          <ArrowLeft className="h-3 w-3" />
          <span>Back to Activity</span>
        </Link>

        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between border-b border-border pb-6">
          <div>
            <h1 className="font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
              My Posted Gigs
            </h1>
            <p className="mt-1 text-xs text-muted-foreground">
              Manage your posted campus opportunities, review proposals, and initiate contracts.
            </p>
          </div>

          <Button asChild className="text-xs self-start sm:self-auto">
            <Link href="/jobs/create">
              <Plus className="h-3.5 w-3.5 mr-1.5" />
              <span>Post a New Gig</span>
            </Link>
          </Button>
        </div>
      </div>

      {/* Loading State */}
      {isJobsLoading && (
        <div className="space-y-4">
          {[1, 2, 3].map((n) => (
            <div
              key={n}
              className="rounded-xl border border-border bg-card p-6 shadow-none animate-pulse space-y-3"
            >
              <div className="h-4 w-28 bg-muted rounded" />
              <div className="h-6 w-2/3 bg-muted rounded" />
              <div className="h-4 w-1/3 bg-muted rounded" />
            </div>
          ))}
        </div>
      )}

      {/* Error State */}
      {!isJobsLoading && jobsError && (
        <div className="rounded-xl border border-pastel-redText/20 bg-pastel-red p-6 text-center text-xs text-pastel-redText space-y-3">
          <p>Failed to load your posted gigs: {jobsError.message}</p>
          <Button
            variant="outline"
            size="sm"
            onClick={() => refetchJobs()}
            className="text-xs rounded-md"
          >
            <RotateCcw className="h-3 w-3 mr-1.5" />
            Retry
          </Button>
        </div>
      )}

      {/* Empty State */}
      {!isJobsLoading && !jobsError && jobsList.length === 0 && (
        <div className="rounded-xl border border-border bg-card p-12 text-center shadow-none space-y-4">
          <Briefcase className="mx-auto h-8 w-8 text-muted-foreground" />
          <div>
            <h3 className="font-serif text-xl font-medium text-foreground">No gigs posted yet</h3>
            <p className="mt-1.5 text-xs text-muted-foreground max-w-sm mx-auto leading-relaxed">
              You have not posted any campus gigs yet. Publish an opportunity to begin receiving
              proposals from verified university students.
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
      )}

      {/* Jobs List */}
      {!isJobsLoading && !jobsError && jobsList.length > 0 && (
        <div className="space-y-4">
          {jobsList.map((job) => {
            const statusStyles = getStatusBadgeClasses(job.status);
            return (
              <div
                key={job.id}
                className="rounded-xl border border-border bg-card p-6 shadow-none transition hover:border-foreground/30 flex flex-col md:flex-row md:items-center justify-between gap-5"
              >
                <div className="space-y-2">
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

                  <div className="flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-xs text-muted-foreground">
                    <span>Posted {formatJobDate(job.created_at)}</span>
                    <span>-</span>
                    <span>Deadline: {formatJobDate(job.deadline)}</span>
                  </div>
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
          })}
        </div>
      )}
    </div>
  );
}
