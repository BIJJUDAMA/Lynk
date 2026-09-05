"use client";

import { useState, useCallback, useMemo } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  Briefcase,
  Users,
  FileText,
  CheckCircle2,
  XCircle,
  AlertCircle,
  ArrowLeft,
  GraduationCap,
  Calendar,
  Clock,
  DollarSign,
  Download,
  ExternalLink,
  Loader2,
  ShieldCheck,
  AlertTriangle,
  RotateCcw,
  Sparkles,
  Lock,
  Mail,
} from "lucide-react";
import {
  Job,
  ApplicationWithDetails,
  ApplicationStatus,
} from "@/types/api";
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
  formatBudget,
  formatJobDate,
  getStatusBadgeClasses,
  getApplicationStatusBadgeClasses,
} from "@/lib/formatters";
import { cn } from "@/lib/utils";

type FilterTab = "all" | "pending" | "accepted" | "rejected";

export default function JobApplicantsPage() {
  const params = useParams();
  const router = useRouter();
  const jobId = typeof params.id === "string" ? params.id : "";

  const {
    user,
    backendUser,
    isAuthenticated,
    role,
    isLoading: authLoading,
    login,
  } = useAuth();

  // State for active filter tab
  const [activeTab, setActiveTab] = useState<FilterTab>("all");

  // State for resume download loading per application ID
  const [downloadingResumeId, setDownloadingResumeId] = useState<string | null>(null);
  const [resumeError, setResumeError] = useState<{ id: string; message: string } | null>(null);

  // State for Accept Confirmation Modal
  const [acceptingApp, setAcceptingApp] = useState<ApplicationWithDetails | null>(null);
  const [isAcceptingSubmitting, setIsAcceptingSubmitting] = useState(false);
  const [acceptError, setAcceptError] = useState<string | null>(null);

  // State for Reject Confirmation Modal
  const [rejectingApp, setRejectingApp] = useState<ApplicationWithDetails | null>(null);
  const [isRejectingSubmitting, setIsRejectingSubmitting] = useState(false);
  const [rejectError, setRejectError] = useState<string | null>(null);

  // State for acceptance success banner
  const [acceptanceSuccess, setAcceptanceSuccess] = useState<{
    studentName: string;
    contractId?: string;
  } | null>(null);

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

  // Applicant counts
  const totalCount = applicationsList.length;
  const pendingCount = applicationsList.filter((a) => a.status === "pending").length;
  const acceptedCount = applicationsList.filter((a) => a.status === "accepted").length;
  const rejectedCount = applicationsList.filter((a) => a.status === "rejected").length;

  // Filtered applications
  const filteredApplications = useMemo(() => {
    if (activeTab === "all") return applicationsList;
    return applicationsList.filter((a) => a.status === activeTab);
  }, [applicationsList, activeTab]);

  // Employer ownership check
  const isJobOwner =
    job &&
    (job.created_by === backendUser?.id ||
      job.created_by === user?.id ||
      job.employer_id === backendUser?.id ||
      job.employer_id === user?.id ||
      (job.creator && (job.creator.id === backendUser?.id || job.creator.id === user?.id)) ||
      (job.employer && (job.employer.id === backendUser?.id || job.employer.id === user?.id)));

  // Handle Resume Presigned Download
  const handleDownloadResume = async (app: ApplicationWithDetails) => {
    const studentId = app.applicant?.id || app.applicant_id || app.student?.id || app.student_id;
    if (!studentId) return;

    setDownloadingResumeId(app.id);
    setResumeError(null);

    try {
      const res = await getStudentResumeUrl(studentId);
      const urlToOpen = res.download_url || res.url;
      if (urlToOpen) {
        window.open(urlToOpen, "_blank", "noopener,noreferrer");
      } else {
        setResumeError({
          id: app.id,
          message: "No download URL was returned by storage service.",
        });
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

  // Handle Accept Application Flow
  const handleConfirmAccept = async () => {
    if (!acceptingApp) return;

    setIsAcceptingSubmitting(true);
    setAcceptError(null);

    try {
      const updated = await updateApplicationStatus(acceptingApp.id, "accepted");
      const studentName = acceptingApp.student
        ? `${acceptingApp.student.first_name} ${acceptingApp.student.last_name}`
        : "Student";

      // Check if updated returned a contract or details
      const contractId =
        (updated as ApplicationWithDetails).contract?.id || undefined;

      setAcceptanceSuccess({
        studentName,
        contractId,
      });

      setAcceptingApp(null);
      // Refetch both job and applications to sync UI state
      await Promise.all([refetchJob(), refetchApplicants()]);
    } catch (err: unknown) {
      const msg =
        err instanceof ApiClientError || err instanceof Error
          ? err.message
          : "Failed to accept application.";
      setAcceptError(msg);
    } finally {
      setIsAcceptingSubmitting(false);
    }
  };

  // Handle Reject Application Flow
  const handleConfirmReject = async () => {
    if (!rejectingApp) return;

    setIsRejectingSubmitting(true);
    setRejectError(null);

    try {
      await updateApplicationStatus(rejectingApp.id, "rejected");
      setRejectingApp(null);
      await Promise.all([refetchJob(), refetchApplicants()]);
    } catch (err: unknown) {
      const msg =
        err instanceof ApiClientError || err instanceof Error
          ? err.message
          : "Failed to decline application.";
      setRejectError(msg);
    } finally {
      setIsRejectingSubmitting(false);
    }
  };

  // Loading state
  if (authLoading || jobLoading) {
    return (
      <div className="mx-auto max-w-6xl px-4 py-10 sm:px-6 lg:px-8">
        <div className="h-6 w-32 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
        <div className="mt-6 rounded-[10px] border border-slate-200 bg-white p-8 dark:border-slate-800 dark:bg-slate-900">
          <div className="h-8 w-1/3 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
          <div className="mt-4 flex gap-4">
            <div className="h-5 w-24 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
            <div className="h-5 w-32 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
          </div>
        </div>
        <div className="mt-8 space-y-4">
          <div className="h-40 w-full animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
          <div className="h-40 w-full animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
        </div>
      </div>
    );
  }

  // Not authenticated gate
  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8 text-center">
        <div className="rounded-[10px] border border-slate-200 bg-white p-8 dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 text-primary dark:bg-emerald-950/60 dark:text-emerald-400">
            <Lock className="h-7 w-7" />
          </div>
          <h1 className="mt-4 text-2xl font-bold text-slate-900 dark:text-white">
            Campus Sign In Required
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            Please sign in with your campus account to review proposals for this opportunity.
          </p>
          <div className="mt-6">
            <button
              onClick={() =>
                login({
                  redirectPath: `/jobs/${jobId}/applicants`,
                  
                })
              }
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary px-6 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-primary/90"
            >
              <Briefcase className="h-4 w-4" />
              <span>Sign In with Campus Account</span>
            </button>
          </div>
        </div>
      </div>
    );
  }



  // Job error
  if (jobError || !job) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 text-center">
        <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-rose-100 text-rose-600 dark:bg-rose-950 dark:text-rose-400">
          <AlertCircle className="h-8 w-8" />
        </div>
        <h1 className="mt-4 text-2xl font-bold text-slate-900 dark:text-white">
          Job Posting Not Found
        </h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
          {jobError?.message || "Could not load the specified job posting."}
        </p>
        <div className="mt-6 flex justify-center gap-4">
          <button
            onClick={() => refetchJob()}
            className="inline-flex items-center gap-2 rounded-[10px] bg-primary px-4 py-2 text-xs font-semibold text-white hover:bg-primary/90"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            Retry
          </button>
          <Link
            href="/activity"
            className="inline-flex items-center gap-2 rounded-[10px] border border-slate-300 bg-white px-4 py-2 text-xs font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Back to My Activity
          </Link>
        </div>
      </div>
    );
  }

  // Ownership warning gate (if employer doesn't own this job)
  if (isJobOwner === false) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8 text-center">
        <div className="rounded-[10px] border border-amber-200 bg-amber-50/60 p-8 dark:border-amber-900/50 dark:bg-amber-950/30">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300">
            <ShieldCheck className="h-7 w-7" />
          </div>
          <h1 className="mt-4 text-2xl font-bold text-slate-900 dark:text-white">
            Unauthorized Access
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
            You can only review applicants for opportunities created by your campus account.
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <Link
              href="/employer/jobs"
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary px-5 py-2.5 text-sm font-semibold text-white hover:bg-primary/90"
            >
              <span>View My Job Postings</span>
            </Link>
            <Link
              href={`/jobs/${job.id}`}
              className="inline-flex items-center gap-2 rounded-[10px] border border-slate-300 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
            >
              <span>View Public Job Details</span>
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const jobStatusStyles = getStatusBadgeClasses(job.status);

  return (
    <div className="mx-auto max-w-6xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Back Navigation Bar */}
      <div className="mb-6 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-4">
          <Link
            href="/employer/jobs"
            className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 transition hover:text-primary dark:text-slate-400 dark:hover:text-emerald-400"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            <span>My Postings</span>
          </Link>
          <span className="text-slate-300 dark:text-slate-700">/</span>
          <Link
            href={`/jobs/${job.id}`}
            className="text-xs font-semibold text-slate-600 transition hover:text-primary dark:text-slate-300 dark:hover:text-emerald-400"
          >
            View Public Listing
          </Link>
        </div>

        <span className="text-xs text-slate-400 dark:text-slate-500">
          Applicant Management Hub
        </span>
      </div>

      {/* Acceptance Success Banner */}
      {acceptanceSuccess && (
        <div className="mb-6 rounded-[10px] border border-emerald-300 bg-emerald-50/90 p-5 shadow-sm dark:border-emerald-800/80 dark:bg-emerald-950/60">
          <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
            <div className="flex items-start gap-3">
              <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-[10px] bg-emerald-100 text-emerald-600 dark:bg-emerald-900/60 dark:text-emerald-300">
                <CheckCircle2 className="h-6 w-6" />
              </div>
              <div>
                <h3 className="text-sm font-bold text-emerald-900 dark:text-emerald-200">
                  🎉 Contract Initiated with {acceptanceSuccess.studentName}!
                </h3>
                <p className="mt-0.5 text-xs text-emerald-800 dark:text-emerald-300">
                  The proposal has been accepted, this job is now marked as <strong>In Progress</strong>, and an active contract has been created.
                </p>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <Link
                href="/contracts"
                className="inline-flex items-center gap-1.5 rounded-[10px] bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-sm transition hover:bg-emerald-500"
              >
                <span>Go to Contracts</span>
                <ExternalLink className="h-3.5 w-3.5" />
              </Link>
              <button
                type="button"
                onClick={() => setAcceptanceSuccess(null)}
                className="rounded-[10px] p-1.5 text-emerald-700 hover:bg-emerald-100 dark:text-emerald-300 dark:hover:bg-emerald-900/40"
                title="Dismiss banner"
              >
                <XCircle className="h-4 w-4" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Job Summary Banner Card */}
      <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
        <div className="flex flex-col justify-between gap-6 md:flex-row md:items-center">
          <div>
            <div className="flex flex-wrap items-center gap-2">
              <span
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-full border px-3 py-0.5 text-xs font-semibold uppercase tracking-wider",
                  jobStatusStyles.bg,
                  jobStatusStyles.text
                )}
              >
                <span className={cn("h-2 w-2 rounded-full", jobStatusStyles.dot)} />
                {job.status.replace("_", " ")}
              </span>

              {job.department && (
                <span className="inline-flex items-center gap-1 rounded-md bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                  <GraduationCap className="h-3.5 w-3.5 text-slate-500" />
                  <span>{job.department}</span>
                </span>
              )}
            </div>

            <h1 className="mt-2 text-2xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
              {job.title}
            </h1>

            <div className="mt-3 flex flex-wrap items-center gap-y-2 gap-x-6 text-xs text-slate-600 dark:text-slate-400">
              <div className="flex items-center gap-1.5 font-semibold text-slate-900 dark:text-white">
                <DollarSign className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />
                <span>{formatBudget(job.budget, job.pay_type)}</span>
              </div>

              {job.deadline && (
                <div className="flex items-center gap-1.5">
                  <Calendar className="h-4 w-4 text-slate-400" />
                  <span>Deadline: {formatJobDate(job.deadline)}</span>
                </div>
              )}

              <div className="flex items-center gap-1.5">
                <Clock className="h-4 w-4 text-slate-400" />
                <span>Posted {formatJobDate(job.created_at)}</span>
              </div>
            </div>
          </div>

          {/* Quick Metrics Pill */}
          <div className="flex shrink-0 items-center gap-3 rounded-[10px] border border-slate-100 bg-slate-50/80 p-4 dark:border-slate-800 dark:bg-slate-800/40">
            <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-accent text-primary dark:bg-emerald-950/80 dark:text-emerald-400">
              <Users className="h-6 w-6" />
            </div>
            <div>
              <span className="text-2xl font-extrabold text-slate-900 dark:text-white">
                {totalCount}
              </span>
              <p className="text-xs font-medium text-slate-500 dark:text-slate-400">
                Total {totalCount === 1 ? "Applicant" : "Applicants"}
              </p>
            </div>
          </div>
        </div>
      </div>

      {/* Filter Tabs Header */}
      <div className="mt-8 flex flex-col justify-between gap-4 border-b border-slate-200 pb-4 sm:flex-row sm:items-center dark:border-slate-800">
        <div className="flex flex-wrap items-center gap-2">
          <button
            type="button"
            onClick={() => setActiveTab("all")}
            className={cn(
              "rounded-[10px] px-4 py-2 text-xs font-semibold transition",
              activeTab === "all"
                ? "bg-slate-900 text-white dark:bg-white dark:text-slate-900"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            All Applicants ({totalCount})
          </button>

          <button
            type="button"
            onClick={() => setActiveTab("pending")}
            className={cn(
              "rounded-[10px] px-4 py-2 text-xs font-semibold transition",
              activeTab === "pending"
                ? "bg-amber-600 text-white shadow-sm"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            Pending Review ({pendingCount})
          </button>

          <button
            type="button"
            onClick={() => setActiveTab("accepted")}
            className={cn(
              "rounded-[10px] px-4 py-2 text-xs font-semibold transition",
              activeTab === "accepted"
                ? "bg-emerald-600 text-white shadow-sm"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            Accepted ({acceptedCount})
          </button>

          <button
            type="button"
            onClick={() => setActiveTab("rejected")}
            className={cn(
              "rounded-[10px] px-4 py-2 text-xs font-semibold transition",
              activeTab === "rejected"
                ? "bg-rose-600 text-white shadow-sm"
                : "bg-slate-100 text-slate-600 hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            Not Selected ({rejectedCount})
          </button>
        </div>

        <button
          type="button"
          onClick={() => refetchApplicants()}
          disabled={applicantsLoading}
          className="inline-flex items-center gap-1.5 self-start text-xs font-semibold text-slate-500 hover:text-primary sm:self-auto dark:text-slate-400 dark:hover:text-emerald-400"
        >
          <RotateCcw
            className={cn("h-3.5 w-3.5", applicantsLoading && "animate-spin")}
          />
          <span>Refresh List</span>
        </button>
      </div>

      {/* Global Resume Error Banner if present */}
      {resumeError && (
        <div className="mt-4 flex items-center justify-between rounded-[10px] border border-rose-200 bg-rose-50/70 p-3 text-xs text-rose-800 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-200">
          <div className="flex items-center gap-2">
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-600" />
            <span>{resumeError.message}</span>
          </div>
          <button
            type="button"
            onClick={() => setResumeError(null)}
            className="text-rose-600 hover:text-rose-900"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>
      )}

      {/* Applicants List Area */}
      <div className="mt-6 space-y-6">
        {/* Loading Skeletons */}
        {applicantsLoading && (
          <div className="space-y-4">
            {Array.from({ length: 3 }).map((_, i) => (
              <div
                key={i}
                className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900"
              >
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-3">
                    <div className="h-12 w-12 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
                    <div>
                      <div className="h-5 w-36 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
                      <div className="mt-2 h-4 w-28 animate-pulse rounded bg-slate-100 dark:bg-slate-800" />
                    </div>
                  </div>
                  <div className="h-6 w-24 animate-pulse rounded-full bg-slate-200 dark:bg-slate-800" />
                </div>
                <div className="mt-6 space-y-2">
                  <div className="h-4 w-full animate-pulse rounded bg-slate-100 dark:bg-slate-800" />
                  <div className="h-4 w-4/5 animate-pulse rounded bg-slate-100 dark:bg-slate-800" />
                </div>
              </div>
            ))}
          </div>
        )}

        {/* Error State */}
        {!applicantsLoading && applicantsError && (
          <div className="rounded-[10px] border border-rose-200 bg-rose-50/60 p-8 text-center dark:border-rose-900/50 dark:bg-rose-950/30">
            <AlertCircle className="mx-auto h-8 w-8 text-rose-600 dark:text-rose-400" />
            <h3 className="mt-2 text-base font-bold text-slate-900 dark:text-white">
              Failed to load applications
            </h3>
            <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
              {applicantsError.message || "An unexpected error occurred while fetching applicants."}
            </p>
            <button
              onClick={() => refetchApplicants()}
              className="mt-4 inline-flex items-center gap-1.5 rounded-[10px] bg-primary px-4 py-2 text-xs font-semibold text-white hover:bg-primary/90"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Try Again
            </button>
          </div>
        )}

        {/* Empty State */}
        {!applicantsLoading && !applicantsError && filteredApplications.length === 0 && (
          <div className="rounded-[10px] border border-slate-200 bg-white p-12 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 text-primary dark:bg-emerald-950/60 dark:text-emerald-400">
              <Users className="h-7 w-7" />
            </div>
            <h3 className="mt-4 text-base font-bold text-slate-900 dark:text-white">
              {activeTab === "all"
                ? "No applications submitted yet"
                : `No ${activeTab} applications`}
            </h3>
            <p className="mx-auto mt-2 max-w-sm text-xs text-slate-500 dark:text-slate-400">
              {activeTab === "all"
                ? "Verified university students will appear here as they submit proposals for your job posting."
                : `There are currently no proposals in "${activeTab}" status.`}
            </p>
            {activeTab !== "all" && (
              <button
                type="button"
                onClick={() => setActiveTab("all")}
                className="mt-4 text-xs font-semibold text-primary hover:underline dark:text-emerald-400"
              >
                View all applicants ({totalCount})
              </button>
            )}
          </div>
        )}

        {/* Applicant Cards List */}
        {!applicantsLoading &&
          !applicantsError &&
          filteredApplications.map((app) => {
            const student = app.applicant || app.student;
            const applicantIdentifier = app.applicant_id || app.student_id || app.id || "";
            const studentFullName = student
              ? `${student.first_name} ${student.last_name}`.trim() || student.email
              : applicantIdentifier
              ? `Applicant ID: ${applicantIdentifier.slice(0, 8)}...`
              : "Applicant";

            const initials = student?.first_name
              ? `${student.first_name[0]}${student.last_name ? student.last_name[0] : ""}`.toUpperCase()
              : "MB";

            const statusStyles = getApplicationStatusBadgeClasses(app.status);
            const hasResume = Boolean(
              app.resume_key ||
                student?.resume_key ||
                student?.resume_filename
            );
            const isDownloadingThisResume = downloadingResumeId === app.id;

            return (
              <div
                key={app.id}
                className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm transition hover:border-slate-300 dark:border-slate-800 dark:bg-slate-900 dark:hover:border-slate-700 sm:p-8"
              >
                {/* Header Row: Student Profile & Status Badge */}
                <div className="flex flex-col justify-between gap-4 sm:flex-row sm:items-center">
                  <div className="flex items-center gap-4">
                    {/* Student Initials Avatar */}
                    <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[10px] bg-accent font-bold text-emerald-800 text-sm shadow-inner dark:bg-emerald-950/60 dark:text-emerald-300">
                      {initials}
                    </div>

                    <div>
                      <div className="flex flex-wrap items-center gap-2">
                        <h3 className="text-base font-bold text-slate-900 dark:text-white">
                          {studentFullName}
                        </h3>
                        <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 px-2 py-0.5 text-[10px] font-semibold text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300">
                          <ShieldCheck className="h-3 w-3" />
                          Campus Verified
                        </span>
                      </div>

                      <div className="mt-1 flex flex-wrap items-center gap-y-1 gap-x-3 text-xs text-slate-500 dark:text-slate-400">
                        {student?.department && (
                          <span className="flex items-center gap-1 font-medium text-slate-700 dark:text-slate-300">
                            <GraduationCap className="h-3.5 w-3.5 text-slate-400" />
                            {student.department}
                          </span>
                        )}

                        {student?.graduation_year && (
                          <span>Class of {student.graduation_year}</span>
                        )}

                        {student?.email && (
                          <span className="flex items-center gap-1 text-slate-400">
                            <Mail className="h-3.5 w-3.5" />
                            {student.email}
                          </span>
                        )}
                      </div>
                    </div>
                  </div>

                  {/* Status Badge */}
                  <div className="flex items-center gap-3 self-start sm:self-auto">
                    <span
                      className={cn(
                        "inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold",
                        statusStyles.bg,
                        statusStyles.text
                      )}
                    >
                      <span className={cn("h-2 w-2 rounded-full", statusStyles.dot)} />
                      {statusStyles.label}
                    </span>
                  </div>
                </div>

                {/* Student Bio (if available) */}
                {student?.bio && (
                  <div className="mt-4 text-xs text-slate-600 dark:text-slate-400 italic">
                    &ldquo;{student.bio}&rdquo;
                  </div>
                )}

                {/* Student Skills Badges */}
                {student?.skills && student.skills.length > 0 && (
                  <div className="mt-3 flex flex-wrap items-center gap-1.5">
                    {student.skills.map((skill) => (
                      <span
                        key={skill}
                        className="rounded-md border border-slate-200 bg-slate-50 px-2 py-0.5 text-[11px] font-medium text-slate-600 dark:border-slate-800 dark:bg-slate-800/60 dark:text-slate-300"
                      >
                        {skill}
                      </span>
                    ))}
                  </div>
                )}

                {/* Cover Letter Proposal Box */}
                <div className="mt-5 rounded-[10px] border border-slate-100 bg-slate-50/60 p-4 dark:border-slate-800 dark:bg-slate-950/40">
                  <div className="flex items-center justify-between pb-2 text-xs font-bold text-slate-700 dark:text-slate-300">
                    <div className="flex items-center gap-1.5">
                      <FileText className="h-4 w-4 text-primary dark:text-emerald-400" />
                      <span>Applicant Proposal & Cover Letter</span>
                    </div>
                    <span className="text-[11px] font-normal text-slate-400">
                      Applied {formatJobDate(app.created_at)}
                    </span>
                  </div>
                  <div className="whitespace-pre-line text-xs leading-relaxed text-slate-700 dark:text-slate-300">
                    {app.cover_letter}
                  </div>
                </div>

                {/* Action Toolbar: Resume & Decisions */}
                <div className="mt-6 flex flex-col justify-between gap-4 border-t border-slate-100 pt-4 sm:flex-row sm:items-center dark:border-slate-800">
                  {/* Left: Resume Inspector */}
                  <div>
                    {hasResume ? (
                      <button
                        type="button"
                        disabled={isDownloadingThisResume}
                        onClick={() => handleDownloadResume(app)}
                        className="inline-flex items-center gap-2 rounded-[10px] border border-slate-200 bg-white px-3.5 py-2 text-xs font-semibold text-slate-700 shadow-sm transition hover:border-emerald-300 hover:bg-slate-50 hover:text-primary disabled:cursor-not-allowed disabled:opacity-60 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
                      >
                        {isDownloadingThisResume ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin text-primary" />
                        ) : (
                          <Download className="h-3.5 w-3.5 text-primary" />
                        )}
                        <span>
                          {isDownloadingThisResume
                            ? "Generating Secure Link..."
                            : student?.resume_filename
                            ? `View Resume (${student.resume_filename})`
                            : "View / Download Resume (PDF)"}
                        </span>
                        <ExternalLink className="h-3 w-3 text-slate-400" />
                      </button>
                    ) : (
                      <span className="inline-flex items-center gap-1.5 rounded-[10px] bg-slate-100 px-3 py-1.5 text-xs text-slate-400 dark:bg-slate-800 dark:text-slate-500">
                        <FileText className="h-3.5 w-3.5" />
                        No resume uploaded on profile
                      </span>
                    )}
                  </div>

                  {/* Right: Decision Buttons for Pending Applications */}
                  <div>
                    {app.status === "pending" && (
                      <div className="flex items-center gap-2">
                        {/* Decline button */}
                        <button
                          type="button"
                          onClick={() => {
                            setRejectError(null);
                            setRejectingApp(app);
                          }}
                          className="inline-flex items-center gap-1.5 rounded-[10px] border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-rose-600 shadow-sm transition hover:border-rose-200 hover:bg-rose-50 dark:border-slate-700 dark:bg-slate-800 dark:text-rose-400 dark:hover:bg-rose-950/40"
                        >
                          <XCircle className="h-3.5 w-3.5" />
                          <span>Decline</span>
                        </button>

                        {/* Accept button */}
                        <button
                          type="button"
                          onClick={() => {
                            setAcceptError(null);
                            setAcceptingApp(app);
                          }}
                          className="inline-flex items-center gap-1.5 rounded-[10px] bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-sm transition hover:bg-emerald-500"
                        >
                          <CheckCircle2 className="h-3.5 w-3.5" />
                          <span>Accept Proposal</span>
                        </button>
                      </div>
                    )}

                    {app.status === "accepted" && (
                      <div className="flex items-center gap-2">
                        <span className="text-xs font-semibold text-emerald-600 dark:text-emerald-400">
                          ✓ Contract Initiated
                        </span>
                        <Link
                          href="/contracts"
                          className="inline-flex items-center gap-1 rounded-[10px] bg-emerald-50 px-3 py-1.5 text-xs font-semibold text-emerald-700 hover:bg-emerald-100 dark:bg-emerald-950/60 dark:text-emerald-300"
                        >
                          <span>Manage Contract</span>
                          <ExternalLink className="h-3 w-3" />
                        </Link>
                      </div>
                    )}

                    {app.status === "rejected" && (
                      <span className="text-xs text-slate-400 dark:text-slate-500">
                        Proposal declined
                      </span>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
      </div>

      {/* Confirmation Modal: Accept Application & Initiate Contract */}
      {acceptingApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4 backdrop-blur-sm">
          <div className="w-full max-w-lg rounded-[10px] border border-slate-200 bg-white p-6 shadow-2xl dark:border-slate-800 dark:bg-slate-900 sm:p-8">
            <div className="flex items-start gap-4">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[10px] bg-emerald-100 text-emerald-600 dark:bg-emerald-950 dark:text-emerald-400">
                <CheckCircle2 className="h-6 w-6" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-slate-900 dark:text-white">
                  Accept Proposal & Initiate Contract
                </h3>
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  You are about to hire{" "}
                  <strong>
                    {acceptingApp.student
                      ? `${acceptingApp.student.first_name} ${acceptingApp.student.last_name}`
                      : "this student applicant"}
                  </strong>{" "}
                  for <strong>{job.title}</strong>.
                </p>
              </div>
            </div>

            {/* Atomic Action Breakdown Callout */}
            <div className="mt-5 rounded-[10px] border border-amber-200 bg-amber-50/70 p-4 text-xs text-amber-900 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-200">
              <div className="flex items-center gap-2 font-bold">
                <AlertTriangle className="h-4 w-4 text-amber-600 dark:text-amber-400" />
                <span>Important Atomic System Actions:</span>
              </div>
              <ul className="mt-2 space-y-1.5 pl-5 list-disc text-[11px] leading-relaxed">
                <li>
                  An <strong>Active Contract</strong> will be generated immediately for{" "}
                  <strong>{formatBudget(job.budget, job.pay_type)}</strong>.
                </li>
                <li>
                  This job will transition to <strong>In Progress</strong> and close to new applicants.
                </li>
                <li>
                  All other pending proposals for this job will be <strong>automatically declined</strong>.
                </li>
              </ul>
            </div>

            {/* Error in modal */}
            {acceptError && (
              <div className="mt-4 flex items-center gap-2 rounded-[10px] border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{acceptError}</span>
              </div>
            )}

            {/* Modal Buttons */}
            <div className="mt-6 flex flex-col-reverse justify-end gap-3 sm:flex-row">
              <button
                type="button"
                disabled={isAcceptingSubmitting}
                onClick={() => setAcceptingApp(null)}
                className="rounded-[10px] border border-slate-300 bg-white px-5 py-2.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={isAcceptingSubmitting}
                onClick={handleConfirmAccept}
                className="inline-flex items-center justify-center gap-2 rounded-[10px] bg-emerald-600 px-6 py-2.5 text-xs font-semibold text-white shadow-sm hover:bg-emerald-500 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isAcceptingSubmitting ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    <span>Initiating Contract...</span>
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    <span>Confirm & Hire Student</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Confirmation Modal: Reject Application */}
      {rejectingApp && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-900/60 p-4 backdrop-blur-sm">
          <div className="w-full max-w-md rounded-[10px] border border-slate-200 bg-white p-6 shadow-2xl dark:border-slate-800 dark:bg-slate-900 sm:p-8">
            <div className="flex items-start gap-4">
              <div className="flex h-12 w-12 shrink-0 items-center justify-center rounded-[10px] bg-rose-100 text-rose-600 dark:bg-rose-950 dark:text-rose-400">
                <XCircle className="h-6 w-6" />
              </div>
              <div>
                <h3 className="text-lg font-bold text-slate-900 dark:text-white">
                  Decline Proposal
                </h3>
                <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                  Are you sure you want to decline the proposal from{" "}
                  <strong>
                    {rejectingApp.student
                      ? `${rejectingApp.student.first_name} ${rejectingApp.student.last_name}`
                      : "this applicant"}
                  </strong>
                  ?
                </p>
              </div>
            </div>

            {rejectError && (
              <div className="mt-4 flex items-center gap-2 rounded-[10px] border border-rose-200 bg-rose-50 p-3 text-xs text-rose-700 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{rejectError}</span>
              </div>
            )}

            <div className="mt-6 flex flex-col-reverse justify-end gap-3 sm:flex-row">
              <button
                type="button"
                disabled={isRejectingSubmitting}
                onClick={() => setRejectingApp(null)}
                className="rounded-[10px] border border-slate-300 bg-white px-5 py-2.5 text-xs font-semibold text-slate-700 hover:bg-slate-50 disabled:opacity-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
              >
                Cancel
              </button>
              <button
                type="button"
                disabled={isRejectingSubmitting}
                onClick={handleConfirmReject}
                className="inline-flex items-center justify-center gap-2 rounded-[10px] bg-rose-600 px-6 py-2.5 text-xs font-semibold text-white shadow-sm hover:bg-rose-500 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isRejectingSubmitting ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    <span>Declining...</span>
                  </>
                ) : (
                  <>
                    <XCircle className="h-3.5 w-3.5" />
                    <span>Decline Proposal</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
