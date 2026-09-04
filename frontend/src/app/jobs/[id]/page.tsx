"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { useParams, useRouter } from "next/navigation";
import {
  ArrowLeft,
  Building2,
  Calendar,
  Clock,
  DollarSign,
  GraduationCap,
  Globe,
  ShieldCheck,
  ShieldAlert,
  Briefcase,
  AlertCircle,
  CheckCircle2,
  Send,
  FileText,
  RotateCcw,
  Lock,
} from "lucide-react";
import { Job, StudentProfile } from "@/types/api";
import { getJobById, applyToJob, getStudentProfile, isEmailNotVerifiedError } from "@/lib/api";
import { useQuery, useMutation } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  formatBudget,
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
    role,
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

  // If student is authenticated and verified, check if they have a resume on file
  const { data: studentProfile } = useQuery<StudentProfile>(
    useCallback((client) => getStudentProfile(client), []),
    {
      enabled: Boolean(isAuthenticated && role === "student" && isVerified),
    }
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
        resume_key: studentProfile?.resume_key ?? undefined,
      });
      setHasAppliedSuccess(true);
    } catch (err: unknown) {
      if (isEmailNotVerifiedError(err)) {
        setSubmitError(
          "Institutional email verification required. Please verify your .edu email before applying."
        );
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
        <div className="h-6 w-32 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
        <div className="mt-6 rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="h-8 w-2/3 animate-pulse rounded-[10px] bg-slate-200 dark:bg-slate-800" />
          <div className="mt-4 flex gap-3">
            <div className="h-6 w-24 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
            <div className="h-6 w-32 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
          </div>
          <div className="mt-8 space-y-3">
            <div className="h-4 w-full animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800" />
            <div className="h-4 w-5/6 animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800" />
            <div className="h-4 w-4/6 animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800" />
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (jobError || !job) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 text-center">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-rose-100 text-rose-600 dark:bg-rose-950 dark:text-rose-400">
          <AlertCircle className="h-8 w-8" />
        </div>
        <h1 className="mt-4 text-2xl font-bold text-slate-900 dark:text-white">
          Job Not Found
        </h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
          {jobError?.message || "The requested job posting does not exist or has been removed."}
        </p>
        <div className="mt-6 flex justify-center gap-4">
          <button
            onClick={() => refetchJob()}
            className="inline-flex items-center gap-2 rounded-[10px] bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-sm hover:bg-emerald-500"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            Retry
          </button>
          <Link
            href="/jobs"
            className="inline-flex items-center gap-2 rounded-[10px] border border-slate-300 bg-white px-4 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Browse All Jobs
          </Link>
        </div>
      </div>
    );
  }

  const statusStyles = getStatusBadgeClasses(job.status);
  const employerName =
    job.employer?.company_or_org ||
    job.employer?.contact_name ||
    "Campus Employer";
  const isEmployerOwner =
    isAuthenticated &&
    role === "employer" &&
    (backendUser?.id === job.employer_id || user?.id === job.employer_id);

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Back Link */}
      <div className="mb-6">
        <Link
          href="/jobs"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 transition hover:text-emerald-600 dark:text-slate-400 dark:hover:text-emerald-400"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to All Jobs</span>
        </Link>
      </div>

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        {/* Main Job Details Column (2/3 width) */}
        <div className="space-y-6 lg:col-span-2">
          {/* Header Card */}
          <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
            <div className="flex flex-wrap items-center gap-2 pb-4">
              <span
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-full border px-3 py-0.5 text-xs font-semibold uppercase tracking-wider",
                  statusStyles.bg,
                  statusStyles.text
                )}
              >
                <span className={cn("h-2 w-2 rounded-full", statusStyles.dot)} />
                {job.status.replace("_", " ")}
              </span>

              {job.department && (
                <span className="inline-flex items-center gap-1 rounded-[10px] bg-slate-100 px-2.5 py-1 text-xs font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                  <GraduationCap className="h-3.5 w-3.5 text-slate-500" />
                  <span>{job.department}</span>
                </span>
              )}
            </div>

            <h1 className="text-2xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
              {job.title}
            </h1>

            {/* Employer Meta Row */}
            <div className="mt-4 flex flex-wrap items-center gap-y-2 gap-x-6 border-t border-slate-100 pt-4 text-xs text-slate-600 dark:border-slate-800 dark:text-slate-400">
              <div className="flex items-center gap-1.5 font-medium">
                <Building2 className="h-4 w-4 text-slate-400" />
                <span className="text-slate-900 dark:text-white font-semibold">
                  {employerName}
                </span>
              </div>

              {job.employer?.website && (
                <a
                  href={
                    job.employer.website.startsWith("http")
                      ? job.employer.website
                      : `https://${job.employer.website}`
                  }
                  target="_blank"
                  rel="noreferrer"
                  className="flex items-center gap-1.5 text-emerald-600 hover:underline dark:text-emerald-400"
                >
                  <Globe className="h-3.5 w-3.5" />
                  <span>{job.employer.website.replace(/^https?:\/\//, "")}</span>
                </a>
              )}

              <div className="flex items-center gap-1.5">
                <Clock className="h-4 w-4 text-slate-400" />
                <span>Posted {formatJobDate(job.created_at)}</span>
              </div>
            </div>
          </div>

          {/* Job Description Card */}
          <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
            <h2 className="text-lg font-bold text-slate-900 dark:text-white">
              Project Description
            </h2>
            <div className="mt-4 whitespace-pre-line text-sm leading-relaxed text-slate-700 dark:text-slate-300">
              {job.description}
            </div>

            {/* Required Skills Section */}
            {job.required_skills && job.required_skills.length > 0 && (
              <div className="mt-8 border-t border-slate-100 pt-6 dark:border-slate-800">
                <h3 className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
                  Required Skills & Technologies
                </h3>
                <div className="mt-3 flex flex-wrap gap-2">
                  {job.required_skills.map((skill, index) => (
                    <span
                      key={`${skill}-${index}`}
                      className="inline-flex items-center rounded-[10px] border border-slate-200 bg-slate-50 px-3 py-1 text-xs font-medium text-slate-700 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
                    >
                      {skill}
                    </span>
                  ))}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Sidebar Column: Compensation & Apply Gate (1/3 width) */}
        <div className="space-y-6">
          {/* Compensation Card */}
          <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
              Compensation & Terms
            </h3>

            <div className="mt-4 flex items-baseline gap-2">
              <span className="text-3xl font-extrabold text-slate-900 dark:text-white">
                ${job.budget}
              </span>
              <span className="text-sm font-semibold text-slate-600 dark:text-slate-400">
                {job.pay_type === "hourly" ? "/ hour" : "total fixed budget"}
              </span>
            </div>

            <div className="mt-4 space-y-2.5 border-t border-slate-100 pt-4 text-xs text-slate-600 dark:border-slate-800 dark:text-slate-400">
              <div className="flex items-center justify-between">
                <span className="flex items-center gap-1.5 text-slate-500">
                  <Calendar className="h-3.5 w-3.5" />
                  Application Deadline
                </span>
                <span className="font-semibold text-slate-800 dark:text-slate-200">
                  {job.deadline ? formatJobDate(job.deadline) : "Flexible"}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="flex items-center gap-1.5 text-slate-500">
                  <Briefcase className="h-3.5 w-3.5" />
                  Contract Type
                </span>
                <span className="font-semibold text-slate-800 dark:text-slate-200 capitalize">
                  {job.pay_type} Milestone
                </span>
              </div>
            </div>

            <div className="mt-4 rounded-[10px] border border-emerald-100 bg-emerald-50/60 p-3 text-[11px] text-emerald-800 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300">
              <div className="flex items-center gap-1.5 font-semibold">
                <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                <span>Protected by Lynk Contract</span>
              </div>
              <p className="mt-1">
                Accepted proposals generate a structured contract with draft, active, and completed milestones.
              </p>
            </div>
          </div>

          {/* Apply Gate Section */}
          <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <h3 className="text-xs font-bold uppercase tracking-wider text-slate-500 dark:text-slate-400">
              Application Status
            </h3>

            {/* If job is closed / cancelled */}
            {job.status !== "open" && (
              <div className="mt-4 rounded-[10px] border border-slate-200 bg-slate-50 p-4 text-center dark:border-slate-700 dark:bg-slate-800/60">
                <p className="text-xs font-semibold text-slate-700 dark:text-slate-300">
                  This gig is currently{" "}
                  <span className="uppercase font-bold">{job.status.replace("_", " ")}</span>
                </p>
                <p className="mt-1 text-[11px] text-slate-500 dark:text-slate-400">
                  New proposals are no longer being accepted for this position.
                </p>
              </div>
            )}

            {/* Case 1: Unauthenticated User Gate */}
            {job.status === "open" && !isAuthenticated && (
              <div className="mt-4 space-y-4">
                <div className="rounded-[10px] border border-emerald-100 bg-emerald-50/60 p-4 text-xs text-emerald-950 dark:border-emerald-900/50 dark:bg-emerald-950/30 dark:text-emerald-200">
                  <div className="flex items-center gap-1.5 font-bold text-emerald-900 dark:text-emerald-300">
                    <Lock className="h-4 w-4 text-emerald-600" />
                    <span>Verified University Sign-In Required</span>
                  </div>
                  <p className="mt-2 leading-relaxed">
                    Lynk ensures high trust by allowing only authenticated students with verified university credentials to apply.
                  </p>
                </div>

                <button
                  type="button"
                  onClick={() =>
                    login({
                      redirectPath: `/jobs/${job.id}`,
                      roleHint: "student",
                    })
                  }
                  className="flex w-full items-center justify-center gap-2 rounded-[10px] bg-emerald-600 py-3 text-sm font-semibold text-white shadow-md shadow-emerald-500/20 transition hover:bg-emerald-500"
                >
                  <GraduationCap className="h-4 w-4" />
                  <span>Sign In with University Email to Apply</span>
                </button>
              </div>
            )}

            {/* Case 2: Employer Role Gate */}
            {job.status === "open" && isAuthenticated && role === "employer" && (
              <div className="mt-4 space-y-3">
                {isEmployerOwner ? (
                  <div className="rounded-[10px] border border-emerald-200 bg-emerald-50/60 p-4 text-center dark:border-emerald-900/50 dark:bg-emerald-950/30">
                    <p className="text-xs font-bold text-emerald-900 dark:text-emerald-200">
                      You posted this job posting
                    </p>
                    <p className="mt-1 text-[11px] text-emerald-700 dark:text-emerald-300">
                      Review proposals submitted by verified student applicants.
                    </p>
                    <Link
                      href={`/jobs/${job.id}/applicants`}
                      className="mt-3 inline-flex w-full items-center justify-center gap-1.5 rounded-[10px] bg-emerald-600 py-2.5 text-xs font-semibold text-white shadow-sm transition hover:bg-emerald-500"
                    >
                      <Briefcase className="h-3.5 w-3.5" />
                      View Applicants
                    </Link>
                  </div>
                ) : (
                  <div className="rounded-[10px] border border-slate-200 bg-slate-50 p-4 text-xs text-slate-600 dark:border-slate-800 dark:bg-slate-800/60 dark:text-slate-400">
                    <div className="flex items-center gap-1.5 font-bold text-slate-800 dark:text-slate-200">
                      <Briefcase className="h-4 w-4 text-emerald-600" />
                      <span>Employer Account Active</span>
                    </div>
                    <p className="mt-2 text-[11px] leading-relaxed">
                      You are signed in as an employer. Job applications are reserved for verified student freelancers. You can post new jobs or manage existing gigs from your dashboard.
                    </p>
                  </div>
                )}
              </div>
            )}

            {/* Case 3: Student with Unverified Institutional Email Gate */}
            {job.status === "open" &&
              isAuthenticated &&
              role === "student" &&
              !isVerified && (
                <div className="mt-4 space-y-4">
                  <div className="rounded-[10px] border border-amber-200 bg-amber-50/70 p-4 text-xs text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-200">
                    <div className="flex items-start gap-2">
                      <ShieldAlert className="h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" />
                      <div>
                        <h4 className="font-bold">
                          University Email Verification Required
                        </h4>
                        <p className="mt-1.5 text-[11px] leading-relaxed">
                          Your student account requires verified institutional email address (e.g. <code>@university.edu</code>) before submitting proposals or uploading resumes.
                        </p>
                        <p className="mt-2 text-[11px] font-medium text-amber-800 dark:text-amber-300">
                          Please check your university inbox for the verification email sent during registration, or click below to re-authenticate.
                        </p>
                      </div>
                    </div>
                  </div>

                  <button
                    type="button"
                    disabled
                    className="flex w-full cursor-not-allowed items-center justify-center gap-2 rounded-[10px] bg-slate-200 py-3 text-xs font-semibold text-slate-500 dark:bg-slate-800 dark:text-slate-400"
                  >
                    <Lock className="h-4 w-4" />
                    <span>Apply Disabled (Verification Required)</span>
                  </button>
                </div>
              )}

            {/* Case 4: Student with Verified Institutional Email -> Submit Proposal */}
            {job.status === "open" &&
              isAuthenticated &&
              role === "student" &&
              isVerified && (
                <div className="mt-4">
                  {hasAppliedSuccess ? (
                    <div className="rounded-[10px] border border-emerald-200 bg-emerald-50 p-5 text-center dark:border-emerald-900/50 dark:bg-emerald-950/40">
                      <div className="mx-auto flex h-10 w-10 items-center justify-center rounded-full bg-emerald-100 text-emerald-600 dark:bg-emerald-900 dark:text-emerald-300">
                        <CheckCircle2 className="h-6 w-6" />
                      </div>
                      <h4 className="mt-3 text-sm font-bold text-emerald-900 dark:text-emerald-200">
                        Proposal Submitted!
                      </h4>
                      <p className="mt-1.5 text-xs text-emerald-700 dark:text-emerald-300">
                        Your application was received by {employerName}. You will be notified once they review your proposal and initiate the contract.
                      </p>
                      <div className="mt-4">
                        <Link
                          href="/jobs"
                          className="inline-flex items-center gap-1 text-xs font-semibold text-emerald-800 underline hover:text-emerald-900 dark:text-emerald-200"
                        >
                          Browse more jobs &rarr;
                        </Link>
                      </div>
                    </div>
                  ) : (
                    <form onSubmit={handleSubmitApplication} className="space-y-4">
                      <div className="flex items-center justify-between rounded-[10px] bg-emerald-50/70 px-3 py-2 text-xs font-semibold text-emerald-800 dark:bg-emerald-950/50 dark:text-emerald-300">
                        <span className="flex items-center gap-1.5">
                          <ShieldCheck className="h-4 w-4 text-emerald-600" />
                          Verified Student Account
                        </span>
                        <span className="text-[10px] uppercase font-bold text-emerald-600">
                          Eligible to Apply
                        </span>
                      </div>

                      {/* Resume Attachment Info */}
                      <div className="rounded-[10px] border border-slate-200 bg-slate-50/60 p-3 text-xs dark:border-slate-700 dark:bg-slate-800/50">
                        <div className="flex items-center gap-2">
                          <FileText className="h-4 w-4 text-slate-500" />
                          <span className="font-semibold text-slate-700 dark:text-slate-300">
                            Resume Attachment:
                          </span>
                        </div>
                        <div className="mt-1.5 pl-6 text-[11px] text-slate-600 dark:text-slate-400">
                          {studentProfile?.resume_filename ? (
                            <span className="font-medium text-emerald-700 dark:text-emerald-300">
                              Attached: {studentProfile.resume_filename}
                            </span>
                          ) : (
                            <span>
                              No resume uploaded yet. (You can still apply with your cover letter, or upload one in your{" "}
                              <Link
                                href="/profile"
                                className="font-semibold text-emerald-600 hover:underline dark:text-emerald-400"
                              >
                                Profile
                              </Link>
                              ).
                            </span>
                          )}
                        </div>
                      </div>

                      {/* Cover Letter Input */}
                      <div>
                        <label
                          htmlFor="cover_letter"
                          className="block text-xs font-bold text-slate-700 dark:text-slate-300"
                        >
                          Proposal Cover Letter
                        </label>
                        <p className="mt-0.5 text-[11px] text-slate-500 dark:text-slate-400">
                          Briefly introduce yourself and outline why you are a great fit for this gig.
                        </p>
                        <textarea
                          id="cover_letter"
                          rows={4}
                          required
                          value={coverLetter}
                          onChange={(e) => setCoverLetter(e.target.value)}
                          placeholder="Explain your relevant coursework, project experience, and availability..."
                          className="mt-2 w-full rounded-[10px] border border-slate-200 bg-white p-3 text-xs text-slate-900 placeholder:text-slate-400 focus:border-emerald-500 focus:outline-none focus:ring-2 focus:ring-emerald-500/20 dark:border-slate-700 dark:bg-slate-900 dark:text-white"
                        />
                        <div className="mt-1 flex justify-between text-[10px] text-slate-400">
                          <span>Minimum 20 characters</span>
                          <span>{coverLetter.length} chars</span>
                        </div>
                      </div>

                      {/* Error Banner */}
                      {submitError && (
                        <div className="flex items-start gap-2 rounded-[10px] border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
                          <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
                          <span>{submitError}</span>
                        </div>
                      )}

                      {/* Submit Button */}
                      <button
                        type="submit"
                        disabled={isApplying}
                        className="flex w-full items-center justify-center gap-2 rounded-[10px] bg-emerald-600 py-3 text-sm font-semibold text-white shadow-md shadow-emerald-500/25 transition hover:bg-emerald-500 disabled:opacity-60"
                      >
                        {isApplying ? (
                          <>
                            <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                            <span>Submitting Proposal...</span>
                          </>
                        ) : (
                          <>
                            <Send className="h-4 w-4" />
                            <span>Submit Proposal</span>
                          </>
                        )}
                      </button>
                    </form>
                  )}
                </div>
              )}
          </div>
        </div>
      </div>
    </div>
  );
}
