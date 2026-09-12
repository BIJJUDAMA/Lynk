"use client";

import { useState, useCallback, useEffect } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import {
  ArrowLeft,
  Calendar,
  Clock,
  GraduationCap,
  ShieldCheck,
  ShieldAlert,
  Briefcase,
  AlertCircle,
  CheckCircle2,
  Send,
  FileText,
  RotateCcw,
  Lock,
  Users,
  User as UserIcon,
} from "lucide-react";
import { Job, Profile, ApplicationWithDetails } from "@/types/api";
import {
  getJobById,
  applyToJob,
  getMyProfile,
  getMyApplicationForJob,
  isEmailNotVerifiedError,
  ApiClientError,
} from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatJobDate,
  getStatusBadgeClasses,
} from "@/components/jobs/JobCard";
import { cn } from "@/lib/utils";

export default function JobDetailPage() {
  const params = useParams();
  const router = useRouter();
  const jobId = typeof params.id === "string" ? params.id : "";

  const {
    user,
    backendUser,
    isAuthenticated,
    isVerified,
    login,
    isLoading: authLoading,
  } = useAuth();

  // Application form state
  const [coverLetter, setCoverLetter] = useState("");
  const [isApplying, setIsApplying] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [hasAppliedSuccess, setHasAppliedSuccess] = useState(false);

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

  // Fetch member profile for resume attachment
  const { data: memberProfile } = useQuery<Profile>(
    useCallback((client) => getMyProfile(client), []),
    {
      enabled: Boolean(isAuthenticated && isVerified),
    }
  );

  // Fetch member's application for this job only (not full mine list)
  const { data: existingApplication } = useQuery<ApplicationWithDetails | null>(
    useCallback((client) => getMyApplicationForJob(jobId, client), [jobId]),
    {
      enabled: Boolean(isAuthenticated && jobId),
    }
  );

  const alreadyApplied = Boolean(existingApplication || hasAppliedSuccess);

  // Determine if current viewer is the creator of this job
  const isCreator = Boolean(
    isAuthenticated &&
    job &&
    (job.created_by === user?.id || job.created_by === backendUser?.id)
  );

  // Handle application submission
  const handleSubmitApplication = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!coverLetter.trim() || coverLetter.trim().length < 20) {
      setSubmitError("Please write at least 20 characters for your proposal cover letter.");
      return;
    }

    setIsApplying(true);
    setSubmitError(null);

    try {
      await applyToJob(jobId, {
        cover_letter: coverLetter.trim(),
        resume_key: memberProfile?.resume_key ?? undefined,
      });
      setHasAppliedSuccess(true);
    } catch (err: unknown) {
      if (isEmailNotVerifiedError(err)) {
        setSubmitError(
          "Institutional email verification required. Please verify your .edu email before applying."
        );
      } else if (err instanceof ApiClientError) {
        setSubmitError(err.message);
      } else if (err instanceof Error) {
        setSubmitError(err.message);
      } else {
        setSubmitError("Failed to submit proposal. Please try again.");
      }
    } finally {
      setIsApplying(false);
    }
  };

  // Loading state
  if (jobLoading || authLoading) {
    return (
      <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="h-6 w-32 animate-pulse rounded bg-muted" />
        <div className="mt-6 rounded-xl border border-border bg-card p-8 shadow-sm">
          <div className="h-8 w-2/3 animate-pulse rounded bg-muted" />
          <div className="mt-4 flex gap-3">
            <div className="h-6 w-24 animate-pulse rounded-full bg-muted" />
            <div className="h-6 w-32 animate-pulse rounded-full bg-muted" />
          </div>
          <div className="mt-8 space-y-3">
            <div className="h-4 w-full animate-pulse rounded bg-muted" />
            <div className="h-4 w-5/6 animate-pulse rounded bg-muted" />
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (jobError || !job) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-red-500/10 text-red-600 dark:text-red-400">
          <AlertCircle className="h-6 w-6" />
        </div>
        <h1 className="mt-4 text-xl font-bold text-foreground">
          Opportunity Not Found
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {jobError?.message || "The requested job posting does not exist or has been removed."}
        </p>
        <div className="mt-6 flex justify-center gap-4">
          <button
            onClick={() => refetchJob()}
            className="inline-flex items-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            Retry
          </button>
          <Link
            href="/jobs"
            className="inline-flex items-center gap-2 rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-muted"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Browse All Jobs
          </Link>
        </div>
      </div>
    );
  }

  const statusStyles = getStatusBadgeClasses(job.status);
  const creatorDisplayName =
    job.creator?.first_name || job.creator?.last_name
      ? `${job.creator.first_name || ""} ${job.creator.last_name || ""}`.trim()
      : job.creator?.organization ||
        job.employer?.company_or_org ||
        job.employer?.contact_name ||
        "Campus Member";

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Back Link */}
      <div className="mb-6">
        <Link
          href="/jobs"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to All Opportunities</span>
        </Link>
      </div>

      {/* Creator Banner if viewer owns this job */}
      {isCreator && (
        <div className="mb-6 flex flex-col gap-3 rounded-xl border border-primary/20 bg-primary/5 p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Briefcase className="h-5 w-5" />
            </div>
            <div>
              <p className="text-sm font-semibold text-foreground">
                Your Job Posting
              </p>
              <p className="text-xs text-muted-foreground">
                You created this opportunity. You can review applicant proposals and manage contracts.
              </p>
            </div>
          </div>

          <Link
            href={`/jobs/${job.id}/applicants`}
            className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
          >
            <Users className="h-4 w-4" />
            <span>Manage Applicants</span>
          </Link>
        </div>
      )}

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        {/* Main Job Details Column (2/3 width) */}
        <div className="space-y-6 lg:col-span-2">
          {/* Header Card */}
          <div className="rounded-xl border border-border bg-card p-6 shadow-sm sm:p-8">
            <div className="flex flex-wrap items-center gap-2 pb-4">
              <span
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-full border px-3 py-0.5 text-xs font-semibold uppercase tracking-wider",
                  statusStyles.bg,
                  statusStyles.text
                )}
              >
                <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
                {job.status.replace("_", " ")}
              </span>

              {job.department && (
                <span className="inline-flex items-center gap-1 rounded-md bg-muted px-2.5 py-0.5 text-xs font-medium text-foreground">
                  <GraduationCap className="h-3.5 w-3.5 text-muted-foreground" />
                  <span>{job.department}</span>
                </span>
              )}
            </div>

            <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
              {job.title}
            </h1>

            {/* Meta Row */}
            <div className="mt-4 flex flex-wrap items-center gap-y-2 gap-x-6 text-xs text-muted-foreground border-b border-border/60 pb-5">
              <div className="flex items-center gap-1.5">
                <UserIcon className="h-4 w-4 text-muted-foreground" />
                <span className="font-medium text-foreground">{creatorDisplayName}</span>
                {job.creator?.organization && (
                  <span className="text-muted-foreground">({job.creator.organization})</span>
                )}
              </div>

              <div className="flex items-center gap-1.5">
                <Calendar className="h-4 w-4 text-muted-foreground" />
                <span>Posted {formatJobDate(job.created_at)}</span>
              </div>

              <div className="flex items-center gap-1.5">
                <Clock className="h-4 w-4 text-muted-foreground" />
                <span>Deadline: {formatJobDate(job.deadline)}</span>
              </div>
            </div>

            {/* Description */}
            <div className="mt-6">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                Project Description & Requirements
              </h2>
              <div className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-foreground">
                {job.description}
              </div>
            </div>

            {/* Skills */}
            <div className="mt-8 border-t border-border/60 pt-6">
              <h2 className="text-sm font-semibold uppercase tracking-wider text-muted-foreground">
                Required Skills & Academic Background
              </h2>
              <div className="mt-3 flex flex-wrap gap-2">
                {job.required_skills && job.required_skills.length > 0 ? (
                  job.required_skills.map((skill, index) => (
                    <span
                      key={`${skill}-${index}`}
                      className="rounded-md border border-border bg-muted/60 px-3 py-1 text-xs font-medium text-foreground"
                    >
                      {skill}
                    </span>
                  ))
                ) : (
                  <span className="text-xs text-muted-foreground italic">
                    No specific skills specified
                  </span>
                )}
              </div>
            </div>
          </div>
        </div>

        {/* Right Sidebar: Proposal Action */}
        <div className="space-y-6">
          {/* Action Card: Apply or Manage */}
          <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
            {isCreator ? (
              /* Creator State */
              <div className="space-y-3">
                <h3 className="text-sm font-semibold text-foreground">
                  Job Management
                </h3>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  As the creator of this posting, you can review proposals, inspect student resumes, and generate contracts.
                </p>
                <Link
                  href={`/jobs/${job.id}/applicants`}
                  className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
                >
                  <Users className="h-4 w-4" />
                  <span>Review Proposals</span>
                </Link>
              </div>
            ) : alreadyApplied ? (
              /* Already Applied State */
              <div className="space-y-3 text-center">
                <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-full bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
                  <CheckCircle2 className="h-6 w-6" />
                </div>
                <h3 className="text-sm font-semibold text-foreground">
                  Proposal Submitted
                </h3>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  Your application for this opportunity has been received. You will be notified once the creator reviews your proposal.
                </p>
                <Link
                  href="/activity"
                  className="inline-flex items-center justify-center gap-2 rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-muted w-full"
                >
                  View in My Activity
                </Link>
              </div>
            ) : !isAuthenticated ? (
              /* Unauthenticated State */
              <div className="space-y-3">
                <h3 className="text-sm font-semibold text-foreground">
                  Ready to Apply?
                </h3>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  Sign in with your campus account to submit a proposal and attach your resume.
                </p>
                <button
                  onClick={() => login({ redirectPath: `/jobs/${jobId}` })}
                  className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
                >
                  <Lock className="h-4 w-4" />
                  <span>Sign In to Apply</span>
                </button>
              </div>
            ) : !isVerified ? (
              /* Unverified Email State */
              <div className="space-y-3">
                <div className="flex items-center gap-2 text-amber-600 dark:text-amber-400">
                  <ShieldAlert className="h-4 w-4" />
                  <h3 className="text-sm font-semibold text-foreground">
                    Verification Required
                  </h3>
                </div>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  You must verify your institutional .edu email address before applying to campus jobs.
                </p>
                <div className="rounded-lg border border-amber-500/20 bg-amber-500/10 p-3 text-xs text-amber-800 dark:text-amber-200">
                  Check your inbox for a verification link or re-authenticate.
                </div>
              </div>
            ) : (
              /* Proposal Form for Verified Campus Member */
              <form onSubmit={handleSubmitApplication} className="space-y-4">
                <h3 className="text-sm font-semibold text-foreground">
                  Submit Proposal
                </h3>
                <p className="text-xs text-muted-foreground">
                  Introduce yourself, describe your relevant coursework, and summarize how you would deliver the project.
                </p>

                {submitError && (
                  <div className="flex items-center gap-2 rounded-lg border border-red-500/30 bg-red-500/10 p-2.5 text-xs text-red-700 dark:text-red-300">
                    <AlertCircle className="h-4 w-4 shrink-0" />
                    <span>{submitError}</span>
                  </div>
                )}

                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Proposal Cover Letter
                  </label>
                  <textarea
                    rows={4}
                    value={coverLetter}
                    onChange={(e) => setCoverLetter(e.target.value)}
                    placeholder="Describe your qualifications, approach, and availability..."
                    className="mt-1.5 w-full rounded-lg border border-border bg-background p-2.5 text-xs text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                  <div className="mt-1 flex justify-between text-[11px] text-muted-foreground">
                    <span>Min 20 characters</span>
                    <span>{coverLetter.length} chars</span>
                  </div>
                </div>

                {/* Resume Status */}
                <div className="rounded-lg border border-border bg-muted/40 p-3 text-xs text-muted-foreground">
                  {memberProfile?.resume_key ? (
                    <div className="flex items-center gap-2 text-foreground">
                      <FileText className="h-4 w-4 text-primary shrink-0" />
                      <span className="truncate">
                        Attached Resume: {memberProfile.resume_filename || "resume.pdf"}
                      </span>
                    </div>
                  ) : (
                    <div className="space-y-1">
                      <p>No resume uploaded to your profile yet.</p>
                      <Link href="/profile" className="text-primary hover:underline font-medium">
                        Upload resume in Profile &rarr;
                      </Link>
                    </div>
                  )}
                </div>

                <button
                  type="submit"
                  disabled={isApplying}
                  className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 disabled:opacity-50"
                >
                  {isApplying ? (
                    <span>Submitting...</span>
                  ) : (
                    <>
                      <Send className="h-3.5 w-3.5" />
                      <span>Submit Proposal</span>
                    </>
                  )}
                </button>
              </form>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
