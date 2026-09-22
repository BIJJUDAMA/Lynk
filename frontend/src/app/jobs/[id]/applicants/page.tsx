"use client";

import { useState, useCallback, useMemo } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  FileText,
  CheckCircle2,
  XCircle,
  AlertCircle,
  ArrowLeft,
  GraduationCap,
  Users,
  Download,
  ExternalLink,
  Loader2,
  ShieldCheck,
  RotateCcw,
  Lock,
  Briefcase,
  Mail,
} from "lucide-react";
import { Job, ApplicationWithDetails } from "@/types/api";
import {
  getJobById,
  listJobApplications,
  updateApplicationStatus,
  getStudentResumeUrl,
  ApiClientError,
} from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatJobDate,
  formatJobBudget,
  getStatusBadgeClasses,
  getApplicationStatusBadgeClasses,
} from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";

export default function JobApplicantsPage() {
  const params = useParams();
  const jobId = typeof params.id === "string" ? params.id : "";

  const { user, backendUser, isAuthenticated, isLoading: authLoading, login } = useAuth();

  // Resume download state
  const [downloadingResumeId, setDownloadingResumeId] = useState<string | null>(null);
  const [resumeError, setResumeError] = useState<{ id: string; message: string } | null>(null);

  // Inline accept / reject state
  const [acceptingId, setAcceptingId] = useState<string | null>(null);
  const [rejectingId, setRejectingId] = useState<string | null>(null);
  const [actionError, setActionError] = useState<{ id: string; message: string } | null>(null);
  const [acceptedSuccessId, setAcceptedSuccessId] = useState<string | null>(null);

  // Fetch job details
  const {
    data: job,
    isLoading: jobLoading,
    error: jobError,
    refetch: refetchJob,
  } = useQuery<Job>(
    useCallback((client) => getJobById(jobId, client), [jobId]),
    [jobId]
  );

  // Fetch applicants
  const {
    data: applicants,
    isLoading: applicantsLoading,
    error: applicantsError,
    refetch: refetchApplicants,
  } = useQuery<ApplicationWithDetails[]>(
    useCallback((client) => listJobApplications(jobId, client), [jobId]),
    {
      deps: [jobId],
      enabled: Boolean(isAuthenticated && jobId),
    }
  );

  const applicationsList = useMemo(() => applicants ?? [], [applicants]);
  const totalCount = applicationsList.length;

  // Employer ownership check
  const isJobOwner =
    job &&
    (job.created_by === backendUser?.id ||
      job.created_by === user?.id ||
      (job.creator && (job.creator.id === backendUser?.id || job.creator.id === user?.id)) ||
      (job.employer && (job.employer.id === backendUser?.id || job.employer.id === user?.id)));

  // Resume download handler
  const handleDownloadResume = async (app: ApplicationWithDetails) => {
    const applicantId = app.applicant?.id || app.applicant_id;
    if (!applicantId) return;

    setDownloadingResumeId(app.id);
    setResumeError(null);

    try {
      const res = await getStudentResumeUrl(applicantId);
      const urlToOpen = res.download_url || res.url;
      if (urlToOpen) {
        window.open(urlToOpen, "_blank", "noopener,noreferrer");
      } else {
        setResumeError({ id: app.id, message: "No download URL returned by storage service." });
      }
    } catch (err: unknown) {
      const msg =
        err instanceof ApiClientError || err instanceof Error
          ? err.message
          : "Failed to generate resume download link.";
      setResumeError({ id: app.id, message: msg });
    } finally {
      setDownloadingResumeId(null);
    }
  };

  // Direct accept handler
  const handleAccept = async (appId: string) => {
    setAcceptingId(appId);
    setActionError(null);
    try {
      await updateApplicationStatus(appId, "accepted");
      setAcceptedSuccessId(appId);
      await Promise.all([refetchJob(), refetchApplicants()]);
    } catch (err: unknown) {
      const msg =
        err instanceof ApiClientError || err instanceof Error
          ? err.message
          : "Failed to accept application.";
      setActionError({ id: appId, message: msg });
    } finally {
      setAcceptingId(null);
    }
  };

  // Direct reject handler
  const handleReject = async (appId: string) => {
    setRejectingId(appId);
    setActionError(null);
    try {
      await updateApplicationStatus(appId, "rejected");
      await Promise.all([refetchJob(), refetchApplicants()]);
    } catch (err: unknown) {
      const msg =
        err instanceof ApiClientError || err instanceof Error
          ? err.message
          : "Failed to decline application.";
      setActionError({ id: appId, message: msg });
    } finally {
      setRejectingId(null);
    }
  };

  // --- Loading skeleton ---
  if (authLoading || jobLoading) {
    return (
      <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="h-4 w-24 animate-pulse rounded bg-muted" />
        <div className="mt-6 h-10 w-56 animate-pulse rounded bg-muted" />
        <div className="mt-4 h-4 w-64 animate-pulse rounded bg-muted" />
        <div className="mt-8 space-y-4">
          <div className="h-48 w-full animate-pulse rounded-xl bg-muted/60" />
          <div className="h-48 w-full animate-pulse rounded-xl bg-muted/60" />
        </div>
      </div>
    );
  }

  // --- Not authenticated ---
  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8 text-center">
        <div className="rounded-xl border border-border bg-card p-8 shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            <Lock className="h-6 w-6" />
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal tracking-tight text-foreground">
            Sign In Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            Please sign in with your campus account to review proposals for this opportunity.
          </p>
          <div className="mt-6">
            <Button
              onClick={() => login({ redirectPath: `/jobs/${jobId}/applicants` })}
              className="rounded-md"
            >
              <Briefcase className="h-4 w-4" />
              Sign In with Campus Account
            </Button>
          </div>
        </div>
      </div>
    );
  }

  // --- Job error ---
  if (jobError || !job) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-red text-pastel-redText">
          <AlertCircle className="h-6 w-6" />
        </div>
        <h1 className="mt-4 font-serif text-2xl font-normal tracking-tight text-foreground">
          Job Not Found
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {jobError?.message || "Could not load the specified job posting."}
        </p>
        <div className="mt-6 flex justify-center gap-3">
          <Button onClick={() => refetchJob()} variant="outline" className="rounded-md">
            <RotateCcw className="h-3.5 w-3.5" />
            Retry
          </Button>
          <Button asChild variant="outline" className="rounded-md">
            <Link href="/activity">
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to Activity
            </Link>
          </Button>
        </div>
      </div>
    );
  }

  // --- Ownership guard ---
  if (isJobOwner === false) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8 text-center">
        <div className="rounded-xl border border-border bg-card p-8 shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-yellow text-pastel-yellowText">
            <ShieldCheck className="h-6 w-6" />
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal tracking-tight text-foreground">
            Unauthorized Access
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            You can only review applicants for opportunities created by your campus account.
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <Button asChild className="rounded-md">
              <Link href="/employer/jobs">View My Job Postings</Link>
            </Button>
            <Button asChild variant="outline" className="rounded-md">
              <Link href={`/jobs/${job.id}`}>View Public Listing</Link>
            </Button>
          </div>
        </div>
      </div>
    );
  }

  const jobStatusStyles = getStatusBadgeClasses(job.status);

  return (
    <div className="mx-auto max-w-4xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Back navigation */}
      <Link
        href={`/jobs/${job.id}`}
        className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition hover:text-foreground"
      >
        <ArrowLeft className="h-3.5 w-3.5" />
        Back to job
      </Link>

      {/* Page title */}
      <h1 className="mt-4 font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
        Applicant Proposals
      </h1>

      {/* Job metadata strip */}
      <div className="mt-3 flex flex-wrap items-center gap-3 border-b border-border pb-5">
        <span className="text-sm font-medium text-foreground">{job.title}</span>
        {job.department && (
          <Badge variant="secondary">
            <GraduationCap className="h-3 w-3 mr-1" />
            {job.department}
          </Badge>
        )}
        <span
          className={cn(
            "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider",
            jobStatusStyles.bg,
            jobStatusStyles.text
          )}
        >
          <span className={cn("h-1.5 w-1.5 rounded-full", jobStatusStyles.dot)} />
          {job.status.replace("_", " ")}
        </span>
        <span className="font-mono text-xs text-muted-foreground">{formatJobBudget(job)}</span>
        <span className="ml-auto flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
          <Users className="h-3.5 w-3.5" />
          {totalCount} {totalCount === 1 ? "applicant" : "applicants"}
        </span>
      </div>

      {/* Acceptance success banner */}
      {acceptedSuccessId && (
        <div className="mt-6 flex items-center justify-between rounded-xl border border-border bg-card p-4 shadow-none">
          <div className="flex items-center gap-2 text-sm">
            <CheckCircle2 className="h-4 w-4 text-pastel-greenText" />
            <span className="text-foreground">
              Contract initiated.{" "}
              <Link
                href="/contracts"
                className="underline underline-offset-4 hover:text-foreground/80"
              >
                View contracts
              </Link>
            </span>
          </div>
          <button
            type="button"
            onClick={() => setAcceptedSuccessId(null)}
            className="text-muted-foreground hover:text-foreground"
            aria-label="Dismiss"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Resume error banner */}
      {resumeError && (
        <div className="mt-4 flex items-center justify-between rounded-xl border border-border bg-card p-3 shadow-none">
          <div className="flex items-center gap-2 text-xs text-muted-foreground">
            <AlertCircle className="h-3.5 w-3.5 shrink-0" />
            <span>{resumeError.message}</span>
          </div>
          <button
            type="button"
            onClick={() => setResumeError(null)}
            className="text-muted-foreground hover:text-foreground"
            aria-label="Dismiss"
          >
            <XCircle className="h-3.5 w-3.5" />
          </button>
        </div>
      )}

      {/* Applicants list */}
      <div className="space-y-6 mt-8">
        {/* Loading skeletons */}
        {applicantsLoading && (
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <div key={i} className="rounded-xl border border-border bg-card p-6 shadow-none">
                <div className="h-5 w-40 animate-pulse rounded bg-muted" />
                <div className="mt-3 h-4 w-56 animate-pulse rounded bg-muted" />
                <div className="mt-5 h-16 w-full animate-pulse rounded bg-muted/60" />
              </div>
            ))}
          </div>
        )}

        {/* Error state */}
        {!applicantsLoading && applicantsError && (
          <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none">
            <AlertCircle className="mx-auto h-7 w-7 text-muted-foreground" />
            <p className="mt-3 text-sm text-foreground">Failed to load applications.</p>
            <p className="mt-1 text-xs text-muted-foreground">
              {applicantsError.message || "An unexpected error occurred."}
            </p>
            <Button
              onClick={() => refetchApplicants()}
              variant="outline"
              className="mt-4 rounded-md"
              size="sm"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Try Again
            </Button>
          </div>
        )}

        {/* Empty state */}
        {!applicantsLoading && !applicantsError && applicationsList.length === 0 && (
          <div className="rounded-xl border border-border bg-card p-12 text-center shadow-none">
            <Users className="mx-auto h-8 w-8 text-muted-foreground" />
            <p className="mt-4 text-sm font-medium text-foreground">No proposals yet</p>
            <p className="mx-auto mt-2 max-w-sm text-xs text-muted-foreground">
              No proposals submitted yet for this gig. Students will appear here once they apply.
            </p>
          </div>
        )}

        {/* Applicant cards */}
        {!applicantsLoading &&
          !applicantsError &&
          applicationsList.map((app) => {
            const student = app.applicant || app.student;
            const applicantIdentifier = app.applicant_id || app.id || "";
            const studentFullName = student
              ? `${student.first_name} ${student.last_name}`.trim() || student.email
              : applicantIdentifier
                ? `Applicant ${applicantIdentifier.slice(0, 8)}`
                : "Applicant";

            const statusStyles = getApplicationStatusBadgeClasses(app.status);
            const hasResume = Boolean(
              app.resume_key || student?.resume_key || student?.resume_filename
            );
            const isDownloadingThisResume = downloadingResumeId === app.id;
            const isAccepting = acceptingId === app.id;
            const isRejecting = rejectingId === app.id;
            const thisActionError = actionError?.id === app.id ? actionError.message : null;

            return (
              <div
                key={app.id}
                className="rounded-xl border border-border bg-card p-6 shadow-none space-y-5"
              >
                {/* Card header */}
                <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
                  <div className="space-y-1.5">
                    <div className="flex flex-wrap items-center gap-2">
                      <h3 className="font-serif text-xl font-medium text-foreground">
                        {studentFullName}
                      </h3>
                      <Badge variant="default">VERIFIED .EDU</Badge>
                      <span
                        className={cn(
                          "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold",
                          statusStyles.bg,
                          statusStyles.text
                        )}
                      >
                        <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
                        {statusStyles.label}
                      </span>
                    </div>
                    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-xs text-muted-foreground">
                      {student?.department && (
                        <span className="flex items-center gap-1">
                          <GraduationCap className="h-3 w-3" />
                          {student.department}
                        </span>
                      )}
                      {student?.graduation_year && <span>Class of {student.graduation_year}</span>}
                      {student?.email && (
                        <span className="flex items-center gap-1">
                          <Mail className="h-3 w-3" />
                          {student.email}
                        </span>
                      )}
                    </div>
                  </div>
                  <span className="font-mono text-xs text-muted-foreground shrink-0">
                    Applied {formatJobDate(app.created_at)}
                  </span>
                </div>

                {/* Cover letter */}
                {app.cover_letter && (
                  <div className="border-l-2 border-border pl-4 text-sm text-foreground/90 leading-relaxed whitespace-pre-line">
                    {app.cover_letter}
                  </div>
                )}

                {/* Skills */}
                {student?.skills && student.skills.length > 0 && (
                  <div className="flex flex-wrap gap-1.5">
                    {student.skills.map((skill) => (
                      <Badge key={skill} variant="secondary">
                        {skill}
                      </Badge>
                    ))}
                  </div>
                )}

                {/* Actions bar */}
                <div className="flex flex-col justify-between gap-3 border-t border-border pt-4 sm:flex-row sm:items-center">
                  {/* Resume link */}
                  <div>
                    {hasResume ? (
                      <button
                        type="button"
                        disabled={isDownloadingThisResume}
                        onClick={() => handleDownloadResume(app)}
                        className="inline-flex items-center gap-1.5 text-xs font-medium text-foreground underline-offset-4 hover:underline disabled:opacity-60"
                      >
                        {isDownloadingThisResume ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <Download className="h-3.5 w-3.5" />
                        )}
                        {isDownloadingThisResume ? "Generating link..." : "View Attached Resume"}
                        <ExternalLink className="h-3 w-3 text-muted-foreground" />
                      </button>
                    ) : (
                      <span className="flex items-center gap-1.5 text-xs text-muted-foreground">
                        <FileText className="h-3.5 w-3.5" />
                        No resume uploaded
                      </span>
                    )}
                  </div>

                  {/* Decision area */}
                  <div className="flex items-center gap-2">
                    {app.status === "pending" && (
                      <>
                        <Button
                          variant="destructive"
                          size="sm"
                          className="rounded-md"
                          disabled={isRejecting || isAccepting}
                          onClick={() => handleReject(app.id)}
                        >
                          {isRejecting ? (
                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <XCircle className="h-3.5 w-3.5" />
                          )}
                          Decline
                        </Button>
                        <Button
                          size="sm"
                          className="rounded-md"
                          disabled={isAccepting || isRejecting}
                          onClick={() => handleAccept(app.id)}
                        >
                          {isAccepting ? (
                            <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <CheckCircle2 className="h-3.5 w-3.5" />
                          )}
                          Accept &amp; Create Contract
                        </Button>
                      </>
                    )}

                    {app.status === "accepted" && (
                      <div className="flex items-center gap-2 text-xs">
                        <CheckCircle2 className="h-3.5 w-3.5 text-pastel-greenText" />
                        <span className="text-foreground font-medium">Contract initiated</span>
                        <Link
                          href="/contracts"
                          className="inline-flex items-center gap-1 text-muted-foreground underline-offset-4 hover:underline"
                        >
                          View contracts
                          <ExternalLink className="h-3 w-3" />
                        </Link>
                      </div>
                    )}

                    {app.status === "rejected" && (
                      <span className="text-xs text-muted-foreground">Proposal declined</span>
                    )}
                  </div>
                </div>

                {/* Inline action error */}
                {thisActionError && (
                  <div className="flex items-center gap-2 rounded-xl border border-border bg-card p-3 text-xs text-muted-foreground shadow-none">
                    <AlertCircle className="h-3.5 w-3.5 shrink-0" />
                    <span>{thisActionError}</span>
                    <button
                      type="button"
                      onClick={() => setActionError(null)}
                      className="ml-auto text-muted-foreground hover:text-foreground"
                    >
                      <XCircle className="h-3.5 w-3.5" />
                    </button>
                  </div>
                )}
              </div>
            );
          })}
      </div>

      {/* Refresh control */}
      {!applicantsLoading && !applicantsError && applicationsList.length > 0 && (
        <div className="mt-8 flex justify-end">
          <button
            type="button"
            onClick={() => refetchApplicants()}
            disabled={applicantsLoading}
            className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground hover:text-foreground"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            Refresh list
          </button>
        </div>
      )}
    </div>
  );
}
