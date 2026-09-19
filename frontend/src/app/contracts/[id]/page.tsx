"use client";

import React, { useState, useCallback, useMemo } from "react";
import { useParams, useRouter } from "next/navigation";
import Link from "next/link";
import {
  FileCheck,
  ArrowLeft,
  Calendar,
  Briefcase,
  GraduationCap,
  Building2,
  CheckCircle2,
  XCircle,
  Clock,
  Star,
  ShieldCheck,
  AlertCircle,
  AlertTriangle,
  RotateCcw,
  Loader2,
  ExternalLink,
  Info,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  getContractById,
  getContractReviews,
  updateContractStatus,
  createApiClient,
  ApiClientError,
} from "@/lib/api";
import { useQuery } from "@/lib/useApi";
import { formatJobDate, getContractStatusBadgeClasses } from "@/lib/formatters";
import { ReviewModal } from "@/components/reviews/ReviewModal";
import type { ContractWithDetails, Review, ContractStatus } from "@/types/api";

export default function ContractDetailPage() {
  const params = useParams();
  const router = useRouter();
  const contractId = typeof params.id === "string" ? params.id : "";

  const {
    user,
    backendUser,
    role,
    isAuthenticated,
    isLoading: isAuthLoading,
    getToken,
    login,
  } = useAuth();

  // Modals & Action States
  const [isReviewModalOpen, setIsReviewModalOpen] = useState(false);
  const [showCompleteModal, setShowCompleteModal] = useState(false);
  const [showCancelModal, setShowCancelModal] = useState(false);
  const [isUpdatingStatus, setIsUpdatingStatus] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  // Fetch contract details
  const {
    data: contract,
    isLoading: isContractLoading,
    error: contractError,
    refetch: refetchContract,
  } = useQuery<ContractWithDetails>(
    useCallback((client) => getContractById(contractId, client), [contractId]),
    {
      deps: [contractId],
      enabled: Boolean(isAuthenticated && contractId),
    }
  );

  // Fetch contract reviews (always safe to call; returns empty list if no reviews or not completed)
  const {
    data: reviewsData,
    isLoading: isReviewsLoading,
    error: reviewsError,
    refetch: refetchReviews,
  } = useQuery<Review[]>(
    useCallback((client) => getContractReviews(contractId, client), [contractId]),
    {
      deps: [contractId],
      enabled: Boolean(isAuthenticated && contractId),
    }
  );

  const reviews = useMemo(() => reviewsData ?? [], [reviewsData]);

  // Determine user participant status (unified: client_id/freelancer_id with fallback to legacy aliases)
  const currentUserId = backendUser?.id || user?.id;
  const freelancerId = contract?.freelancer_id;
  const clientId = contract?.client_id;

  const isFreelancer =
    Boolean(currentUserId && freelancerId === currentUserId) ||
    Boolean(
      user?.email &&
      (contract?.freelancer?.email === user.email || contract?.student?.email === user.email)
    );

  const isClient =
    Boolean(currentUserId && clientId === currentUserId) ||
    Boolean(
      user?.email &&
      (contract?.client?.email === user.email || contract?.employer?.email === user.email)
    );

  const isParticipant = isFreelancer || isClient;

  // Check if current user has already submitted a review
  const myReview = useMemo(() => {
    if (!currentUserId && !user?.email) return null;
    return reviews.find(
      (r) =>
        (currentUserId && r.reviewer_id === currentUserId) ||
        (r.reviewer &&
          user?.name &&
          `${r.reviewer.first_name} ${r.reviewer.last_name}`.trim() === user.name)
    );
  }, [reviews, currentUserId, user]);

  const hasReviewed = Boolean(myReview);

  // Counterparty display
  const clientName =
    contract?.client?.company_or_org ||
    `${contract?.client?.first_name ?? ""} ${contract?.client?.last_name ?? ""}`.trim() ||
    contract?.employer?.company_or_org ||
    contract?.employer?.contact_name ||
    "Client";

  const freelancerName =
    `${contract?.freelancer?.first_name ?? ""} ${contract?.freelancer?.last_name ?? ""}`.trim() ||
    `${contract?.student?.first_name ?? ""} ${contract?.student?.last_name ?? ""}`.trim() ||
    "Freelancer";

  const counterpartyName = isFreelancer ? clientName : freelancerName;
  const counterpartyRole = isFreelancer ? "Client" : "Freelancer";

  // State Transition Handlers
  const handleTransitionStatus = async (targetStatus: "completed" | "cancelled") => {
    if (!contractId || isUpdatingStatus) return;
    setIsUpdatingStatus(true);
    setActionError(null);

    try {
      const client = createApiClient(getToken);
      await updateContractStatus(contractId, { status: targetStatus }, client);
      setShowCompleteModal(false);
      setShowCancelModal(false);
      await refetchContract();
      if (targetStatus === "completed") {
        await refetchReviews();
      }
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setActionError(err.message || `Failed to transition contract to ${targetStatus}.`);
      } else if (err instanceof Error) {
        setActionError(err.message);
      } else {
        setActionError(`Failed to transition contract to ${targetStatus}.`);
      }
    } finally {
      setIsUpdatingStatus(false);
    }
  };

  // Review created callback
  const handleReviewSuccess = async () => {
    await refetchReviews();
  };

  // Unauthenticated screen
  if (!isAuthLoading && !isAuthenticated) {
    return (
      <div className="mx-auto max-w-4xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-[10px] border border-slate-200 bg-white p-8 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <FileCheck className="mx-auto h-12 w-12 text-primary dark:text-emerald-500" />
          <h2 className="mt-4 text-xl font-bold text-slate-900 dark:text-white">
            Authentication Required
          </h2>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            Please sign in to view this contract agreement and its peer reviews.
          </p>
          <div className="mt-6">
            <button
              onClick={() => login({ redirectPath: `/contracts/${contractId}` })}
              className="inline-flex items-center justify-center rounded-[10px] bg-primary text-primary-foreground px-5 py-2.5 text-sm font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
            >
              Sign In to Continue
            </button>
          </div>
        </div>
      </div>
    );
  }

  // Loading screen
  if (isContractLoading) {
    return (
      <div className="mx-auto max-w-5xl px-4 py-12 sm:px-6 lg:px-8">
        <div className="animate-pulse space-y-6">
          <div className="h-5 w-32 rounded bg-slate-200 dark:bg-slate-800" />
          <div className="rounded-[10px] border border-slate-200 bg-white p-8 dark:border-slate-800 dark:bg-slate-900">
            <div className="h-8 w-2/3 rounded bg-slate-200 dark:bg-slate-800 mb-4" />
            <div className="h-4 w-1/3 rounded bg-slate-200 dark:bg-slate-800 mb-8" />
            <div className="grid grid-cols-1 gap-6 sm:grid-cols-3">
              <div className="h-20 rounded-[10px] bg-slate-200 dark:bg-slate-800" />
              <div className="h-20 rounded-[10px] bg-slate-200 dark:bg-slate-800" />
              <div className="h-20 rounded-[10px] bg-slate-200 dark:bg-slate-800" />
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Error screen (e.g. 404 Not Found or Forbidden)
  if (contractError || !contract) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-[10px] border border-slate-200 bg-white p-8 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <AlertCircle className="mx-auto h-12 w-12 text-rose-500" />
          <h2 className="mt-4 text-xl font-bold text-slate-900 dark:text-white">
            Contract Not Found
          </h2>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            {contractError?.message ||
              "The requested contract does not exist or you do not have permission to view it."}
          </p>
          <div className="mt-6 flex justify-center gap-4">
            <Link
              href="/contracts"
              className="inline-flex items-center gap-1.5 rounded-[10px] border border-slate-200 bg-white px-4 py-2 text-xs font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              <span>Back to Contracts</span>
            </Link>
            <button
              onClick={() => refetchContract()}
              className="inline-flex items-center gap-1.5 rounded-[10px] bg-primary text-primary-foreground px-4 py-2 text-xs font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              <span>Retry</span>
            </button>
          </div>
        </div>
      </div>
    );
  }

  const badge = getContractStatusBadgeClasses(contract.status);

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Navigation Breadcrumb */}
      <div className="mb-6 flex items-center justify-between">
        <Link
          href="/contracts"
          className="inline-flex items-center gap-2 text-xs font-medium text-slate-500 transition hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
        >
          <ArrowLeft className="h-4 w-4" />
          <span>Back to Contracts</span>
        </Link>
        <span className="text-xs font-mono text-slate-400">
          Contract ID: #{contract.id.slice(0, 8)}
        </span>
      </div>

      {/* Global Action Error Alert */}
      {actionError && (
        <div className="mb-6 flex items-start gap-3 rounded-[10px] border border-rose-200 bg-rose-50 p-4 text-xs text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300">
          <AlertCircle className="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400 mt-0.5" />
          <div>
            <strong className="font-semibold">Action Failed:</strong> {actionError}
          </div>
        </div>
      )}

      {/* Contract Hero Overview Card */}
      <div className="overflow-hidden rounded-[10px] border border-slate-200 bg-white shadow-sm dark:border-slate-800 dark:bg-slate-900">
        <div className="p-6 sm:p-8">
          <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
            <div>
              <div className="flex flex-wrap items-center gap-2.5">
                <span
                  className={`inline-flex items-center gap-1.5 rounded-full border px-3 py-1 text-xs font-semibold ${badge.bg} ${badge.text}`}
                >
                  <span className={`h-1.5 w-1.5 rounded-full ${badge.dot}`} />
                  {badge.label}
                </span>

                {contract.job?.department && (
                  <span className="rounded-full bg-slate-100 px-2.5 py-0.5 text-xs font-medium text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                    {contract.job.department}
                  </span>
                )}
              </div>

              <h1 className="mt-3 text-2xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
                {contract.job?.title || "Marketplace Contract Agreement"}
              </h1>

              {contract.job_id && (
                <div className="mt-2 flex items-center gap-2">
                  <Link
                    href={`/jobs/${contract.job_id}`}
                    className="inline-flex items-center gap-1 text-xs font-medium text-primary hover:text-emerald-800 dark:text-emerald-300 dark:text-emerald-500 dark:hover:text-emerald-300"
                  >
                    <span>View original job listing</span>
                    <ExternalLink className="h-3 w-3" />
                  </Link>
                </div>
              )}
            </div>
          </div>

          {/* Job Description Snippet */}
          {contract.job?.description && (
            <div className="mt-6 rounded-[10px] bg-slate-50/70 p-4 dark:bg-slate-800/40">
              <span className="text-xs font-semibold uppercase tracking-wider text-slate-500 dark:text-slate-400">
                Scope & Requirements Summary
              </span>
              <p className="mt-1.5 text-xs sm:text-sm text-slate-700 dark:text-slate-300 line-clamp-3 leading-relaxed">
                {contract.job.description}
              </p>
            </div>
          )}

          {/* Key Dates Timeline Grid */}
          <div className="mt-6 grid grid-cols-1 gap-4 sm:grid-cols-3 border-t border-slate-100 pt-6 dark:border-slate-800">
            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                <Calendar className="h-4 w-4" />
              </div>
              <div>
                <span className="text-[11px] font-medium uppercase text-slate-400">Created</span>
                <p className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                  {formatJobDate(contract.created_at)}
                </p>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <div className="flex h-9 w-9 items-center justify-center rounded-[10px] bg-blue-50 text-blue-600 dark:bg-blue-950/60 dark:text-blue-400">
                <Clock className="h-4 w-4" />
              </div>
              <div>
                <span className="text-[11px] font-medium uppercase text-slate-400">Started</span>
                <p className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                  {contract.started_at ? formatJobDate(contract.started_at) : "Pending start"}
                </p>
              </div>
            </div>

            <div className="flex items-center gap-3">
              <div
                className={`flex h-9 w-9 items-center justify-center rounded-[10px] ${
                  contract.status === "completed"
                    ? "bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400"
                    : contract.status === "cancelled"
                      ? "bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400"
                      : "bg-slate-100 text-slate-400 dark:bg-slate-800 dark:text-slate-500"
                }`}
              >
                {contract.status === "completed" ? (
                  <CheckCircle2 className="h-4 w-4" />
                ) : contract.status === "cancelled" ? (
                  <XCircle className="h-4 w-4" />
                ) : (
                  <Clock className="h-4 w-4" />
                )}
              </div>
              <div>
                <span className="text-[11px] font-medium uppercase text-slate-400">
                  {contract.status === "completed"
                    ? "Completed"
                    : contract.status === "cancelled"
                      ? "Cancelled"
                      : "Completion Target"}
                </span>
                <p className="text-xs font-semibold text-slate-800 dark:text-slate-200">
                  {contract.completed_at
                    ? formatJobDate(contract.completed_at)
                    : contract.status === "active"
                      ? "In Progress"
                      : "-"}
                </p>
              </div>
            </div>
          </div>
        </div>

        {/* State Machine Action Bar (when status is active) */}
        {contract.status === "active" && isParticipant && (
          <div className="flex flex-col sm:flex-row items-center justify-between gap-4 border-t border-slate-100 bg-slate-50/60 px-6 py-4 dark:border-slate-800 dark:bg-slate-800/40 sm:px-8">
            <div className="flex items-center gap-2 text-xs text-slate-600 dark:text-slate-400">
              <Info className="h-4 w-4 text-blue-500 shrink-0" />
              <span>
                Work is active. Mark as completed once all deliverables are satisfied to unlock peer
                reviews.
              </span>
            </div>

            <div className="flex items-center gap-3 w-full sm:w-auto">
              <button
                type="button"
                onClick={() => setShowCancelModal(true)}
                disabled={isUpdatingStatus}
                className="flex-1 sm:flex-initial inline-flex items-center justify-center gap-1.5 rounded-[10px] border border-rose-200 bg-white px-4 py-2 text-xs font-semibold text-rose-700 shadow-sm transition hover:bg-rose-50 dark:border-rose-900/60 dark:bg-slate-900 dark:text-rose-400 dark:hover:bg-rose-950/40 disabled:opacity-50"
              >
                <XCircle className="h-3.5 w-3.5" />
                <span>Cancel Contract</span>
              </button>

              {isClient && (
                <button
                  type="button"
                  onClick={() => setShowCompleteModal(true)}
                  disabled={isUpdatingStatus}
                  className="flex-1 sm:flex-initial inline-flex items-center justify-center gap-1.5 rounded-[10px] bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-md shadow-emerald-600/20 transition hover:bg-emerald-500 disabled:opacity-50"
                >
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  <span>Mark as Completed</span>
                </button>
              )}
            </div>
          </div>
        )}

        {/* Completed Celebratory Banner */}
        {contract.status === "completed" && (
          <div className="border-t border-emerald-100 bg-emerald-50/80 px-6 py-4 dark:border-emerald-900/40 dark:bg-emerald-950/30 sm:px-8">
            <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
              <div className="flex items-center gap-2.5 text-xs text-emerald-800 dark:text-emerald-300">
                <CheckCircle2 className="h-5 w-5 text-emerald-600 dark:text-emerald-400 shrink-0" />
                <span>
                  <strong>Contract Completed!</strong> Deliverables fulfilled on{" "}
                  {formatJobDate(contract.completed_at)}. Both parties are now eligible to exchange
                  peer reviews below.
                </span>
              </div>

              {isParticipant && !hasReviewed && (
                <button
                  type="button"
                  onClick={() => setIsReviewModalOpen(true)}
                  className="inline-flex items-center justify-center gap-1.5 rounded-[10px] bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-md shadow-emerald-600/20 transition hover:bg-emerald-500 shrink-0"
                >
                  <Star className="h-3.5 w-3.5 fill-white" />
                  <span>Leave a Review</span>
                </button>
              )}
            </div>
          </div>
        )}

        {/* Cancelled Warning Banner */}
        {contract.status === "cancelled" && (
          <div className="border-t border-rose-100 bg-rose-50/80 px-6 py-4 dark:border-rose-900/40 dark:bg-rose-950/30 sm:px-8">
            <div className="flex items-center gap-2.5 text-xs text-rose-800 dark:text-rose-300">
              <XCircle className="h-5 w-5 text-rose-600 dark:text-rose-400 shrink-0" />
              <span>
                <strong>Contract Cancelled.</strong> This agreement was terminated early. Peer
                reviews are disabled for cancelled contracts.
              </span>
            </div>
          </div>
        )}
      </div>

      {/* Participants Information Grid */}
      <div className="mt-8 grid grid-cols-1 gap-6 md:grid-cols-2">
        {/* Student Participant Card */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between pb-4 border-b border-slate-100 dark:border-slate-800">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
                <GraduationCap className="h-5 w-5" />
              </div>
              <div>
                <h2 className="text-sm font-bold text-slate-900 dark:text-white">
                  Student Freelancer
                </h2>
                <span className="text-[11px] text-slate-500 dark:text-slate-400">
                  Assigned Contributor
                </span>
              </div>
            </div>

            {isFreelancer && (
              <span className="rounded-full bg-emerald-50 dark:bg-emerald-950/50 px-2.5 py-0.5 text-xs font-semibold text-emerald-800 dark:text-emerald-300 dark:bg-emerald-950/60 dark:text-emerald-300">
                You
              </span>
            )}
          </div>

          <div className="mt-4 space-y-3 text-xs">
            <div className="flex items-center justify-between">
              <span className="text-slate-500 dark:text-slate-400">Name:</span>
              <span className="font-semibold text-slate-900 dark:text-white">
                {contract.student
                  ? `${contract.student.first_name} ${contract.student.last_name}`
                  : "Student"}
              </span>
            </div>

            <div className="flex items-center justify-between">
              <span className="text-slate-500 dark:text-slate-400">Email:</span>
              <span className="font-mono text-slate-700 dark:text-slate-300">
                {contract.student?.email || "Registered Student"}
              </span>
            </div>

            {contract.student?.department && (
              <div className="flex items-center justify-between">
                <span className="text-slate-500 dark:text-slate-400">Department:</span>
                <span className="text-slate-800 dark:text-slate-200">
                  {contract.student.department}
                </span>
              </div>
            )}

            {contract.student?.graduation_year && (
              <div className="flex items-center justify-between">
                <span className="text-slate-500 dark:text-slate-400">Class of:</span>
                <span className="text-slate-800 dark:text-slate-200">
                  {contract.student.graduation_year}
                </span>
              </div>
            )}

            <div className="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-slate-800">
              <span className="text-slate-500 dark:text-slate-400">Status:</span>
              <span className="inline-flex items-center gap-1 font-semibold text-emerald-600 dark:text-emerald-400">
                <ShieldCheck className="h-3.5 w-3.5" />
                Verified Student
              </span>
            </div>
          </div>
        </div>

        {/* Employer Participant Card */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="flex items-center justify-between pb-4 border-b border-slate-100 dark:border-slate-800">
            <div className="flex items-center gap-3">
              <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
                <Building2 className="h-5 w-5" />
              </div>
              <div>
                <h2 className="text-sm font-bold text-slate-900 dark:text-white">
                  Employer / Organization
                </h2>
                <span className="text-[11px] text-slate-500 dark:text-slate-400">
                  Project Sponsor
                </span>
              </div>
            </div>

            {isClient && (
              <span className="rounded-full bg-emerald-50 dark:bg-emerald-950/50 px-2.5 py-0.5 text-xs font-semibold text-emerald-800 dark:text-emerald-300 dark:bg-emerald-950/60 dark:text-emerald-300">
                You
              </span>
            )}
          </div>

          <div className="mt-4 space-y-3 text-xs">
            <div className="flex items-center justify-between">
              <span className="text-slate-500 dark:text-slate-400">Company / Org:</span>
              <span className="font-semibold text-slate-900 dark:text-white">
                {contract.employer?.company_or_org || "Employer"}
              </span>
            </div>

            {contract.employer?.contact_name && (
              <div className="flex items-center justify-between">
                <span className="text-slate-500 dark:text-slate-400">Contact:</span>
                <span className="text-slate-800 dark:text-slate-200">
                  {contract.employer.contact_name}
                </span>
              </div>
            )}

            <div className="flex items-center justify-between">
              <span className="text-slate-500 dark:text-slate-400">Email:</span>
              <span className="font-mono text-slate-700 dark:text-slate-300">
                {contract.employer?.email || "Registered Employer"}
              </span>
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-slate-800">
              <span className="text-slate-500 dark:text-slate-400">Role:</span>
              <span className="inline-flex items-center gap-1 font-semibold text-primary dark:text-emerald-500">
                <Briefcase className="h-3.5 w-3.5" />
                Verified Employer
              </span>
            </div>
          </div>
        </div>
      </div>

      {/* Reviews & Ratings Section */}
      <div className="mt-8 rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between pb-6 border-b border-slate-100 dark:border-slate-800">
          <div>
            <div className="flex items-center gap-2">
              <Star className="h-5 w-5 fill-amber-400 text-amber-400" />
              <h2 className="text-lg font-bold text-slate-900 dark:text-white">
                Peer Reviews & Ratings
              </h2>
            </div>
            <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
              Mutual peer feedback exchanged upon contract completion
            </p>
          </div>

          {/* Review submission action or status pill */}
          {contract.status === "completed" && isParticipant && (
            <div>
              {hasReviewed ? (
                <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300">
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  You reviewed this contract
                </span>
              ) : (
                <button
                  type="button"
                  onClick={() => setIsReviewModalOpen(true)}
                  className="inline-flex items-center gap-1.5 rounded-[10px] bg-primary text-primary-foreground px-4 py-2 text-xs font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
                >
                  <Star className="h-3.5 w-3.5 fill-white" />
                  <span>Leave a Review & Rating</span>
                </button>
              )}
            </div>
          )}
        </div>

        {/* Notice when contract is not completed */}
        {contract.status !== "completed" && (
          <div className="mt-6 rounded-[10px] bg-slate-50 p-6 text-center dark:bg-slate-800/40">
            <Star className="mx-auto h-8 w-8 text-slate-300 dark:text-slate-600" />
            <h3 className="mt-2 text-sm font-semibold text-slate-800 dark:text-slate-200">
              Reviews unlock upon contract completion
            </h3>
            <p className="mx-auto mt-1 max-w-sm text-xs text-slate-500 dark:text-slate-400">
              {contract.status === "cancelled"
                ? "Reviews are not permitted for cancelled contracts."
                : "Both parties will be able to submit a 1-5 star rating and written review once the contract is marked as completed."}
            </p>
          </div>
        )}

        {/* Reviews list when contract is completed */}
        {contract.status === "completed" && (
          <div className="mt-6">
            {isReviewsLoading && (
              <div className="space-y-4">
                <div className="h-20 rounded-[10px] bg-slate-100 animate-pulse dark:bg-slate-800" />
                <div className="h-20 rounded-[10px] bg-slate-100 animate-pulse dark:bg-slate-800" />
              </div>
            )}

            {!isReviewsLoading && reviews.length === 0 && (
              <div className="rounded-[10px] border border-dashed border-slate-200 p-8 text-center dark:border-slate-800">
                <Star className="mx-auto h-8 w-8 text-amber-400/60" />
                <h3 className="mt-2 text-sm font-semibold text-slate-800 dark:text-slate-200">
                  No reviews submitted yet
                </h3>
                <p className="mx-auto mt-1 max-w-md text-xs text-slate-500 dark:text-slate-400">
                  Be the first to share feedback on this collaboration!
                </p>
                {isParticipant && !hasReviewed && (
                  <button
                    type="button"
                    onClick={() => setIsReviewModalOpen(true)}
                    className="mt-4 inline-flex items-center gap-1.5 rounded-[10px] bg-primary text-primary-foreground px-4 py-2 text-xs font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
                  >
                    <Star className="h-3.5 w-3.5 fill-white" />
                    <span>Leave a Review</span>
                  </button>
                )}
              </div>
            )}

            {!isReviewsLoading && reviews.length > 0 && (
              <div className="space-y-4">
                {reviews.map((rev) => {
                  const isMyReview =
                    (currentUserId && rev.reviewer_id === currentUserId) ||
                    (rev.reviewer &&
                      user?.name &&
                      `${rev.reviewer.first_name} ${rev.reviewer.last_name}`.trim() === user.name);

                  const reviewerDisplayName = rev.reviewer
                    ? `${rev.reviewer.first_name} ${rev.reviewer.last_name}`
                    : "Collaborator";

                  const reviewerRoleLabel =
                    rev.reviewer?.role === "student" ? "Student" : "Employer";

                  return (
                    <div
                      key={rev.id}
                      className={`rounded-[10px] border p-5 transition ${
                        isMyReview
                          ? "border-emerald-200 bg-emerald-50/40 dark:border-emerald-800/50 dark:bg-emerald-950/20"
                          : "border-border bg-secondary/30"
                      }`}
                    >
                      <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
                        <div className="flex items-center gap-2.5">
                          <div className="flex h-8 w-8 items-center justify-center rounded-full bg-accent text-accent-foreground text-emerald-800 dark:text-emerald-300 font-semibold text-xs dark:bg-emerald-950 dark:text-emerald-300">
                            {reviewerDisplayName.slice(0, 2).toUpperCase()}
                          </div>
                          <div>
                            <div className="flex items-center gap-2">
                              <span className="text-xs font-bold text-slate-900 dark:text-white">
                                {reviewerDisplayName}
                              </span>
                              <span className="rounded-full bg-slate-200/80 px-2 py-0.5 text-[10px] font-medium text-slate-700 dark:bg-slate-700 dark:text-slate-300">
                                {reviewerRoleLabel}
                              </span>
                              {isMyReview && (
                                <span className="rounded-full bg-accent text-accent-foreground px-2 py-0.5 text-[10px] font-semibold text-emerald-800 dark:text-emerald-300 dark:bg-emerald-950/60 dark:text-emerald-300">
                                  Your Review
                                </span>
                              )}
                            </div>
                            <span className="text-[11px] text-slate-400">
                              {formatJobDate(rev.created_at)}
                            </span>
                          </div>
                        </div>

                        {/* Star Rating Display */}
                        <div className="flex items-center gap-1.5">
                          <div className="flex items-center gap-0.5">
                            {[1, 2, 3, 4, 5].map((s) => (
                              <Star
                                key={s}
                                className={`h-4 w-4 ${
                                  s <= rev.rating
                                    ? "fill-amber-400 text-amber-400"
                                    : "text-slate-300 dark:text-slate-600"
                                }`}
                              />
                            ))}
                          </div>
                          <span className="text-xs font-bold text-slate-900 dark:text-white">
                            {rev.rating}.0
                          </span>
                        </div>
                      </div>

                      {/* Comment body */}
                      {rev.comment && (
                        <p className="mt-3 text-xs sm:text-sm text-slate-700 dark:text-slate-300 leading-relaxed">
                          &ldquo;{rev.comment}&rdquo;
                        </p>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Interactive Peer Review Modal */}
      <ReviewModal
        isOpen={isReviewModalOpen}
        onClose={() => setIsReviewModalOpen(false)}
        contractId={contract.id}
        jobTitle={contract.job?.title}
        counterpartyName={counterpartyName}
        counterpartyRole={counterpartyRole}
        onSuccess={handleReviewSuccess}
      />

      {/* Mark As Completed Confirmation Modal */}
      {showCompleteModal && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-slate-900/60 p-4 backdrop-blur-sm"
          onClick={() => {
            if (!isUpdatingStatus) setShowCompleteModal(false);
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-md rounded-[10px] border border-slate-200 bg-white p-6 shadow-2xl dark:border-slate-800 dark:bg-slate-900"
          >
            <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
              <CheckCircle2 className="h-6 w-6" />
            </div>

            <h3 className="mt-4 text-lg font-bold text-slate-900 dark:text-white">
              Mark Contract as Completed?
            </h3>
            <p className="mt-2 text-xs text-slate-600 dark:text-slate-400 leading-relaxed">
              This confirms that all agreed project deliverables have been satisfactorily fulfilled.
              Once completed, the contract moves to a <strong>permanent terminal status</strong> and
              unlocks mutual peer review submissions.
            </p>

            <div className="mt-6 flex items-center justify-end gap-3">
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => setShowCompleteModal(false)}
                className="rounded-[10px] border border-slate-200 bg-white px-4 py-2 text-xs font-medium text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
              >
                Go Back
              </button>
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => handleTransitionStatus("completed")}
                className="inline-flex items-center gap-2 rounded-[10px] bg-emerald-600 px-4 py-2 text-xs font-semibold text-white shadow-md shadow-emerald-600/20 transition hover:bg-emerald-500 disabled:opacity-50"
              >
                {isUpdatingStatus ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Completing...</span>
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="h-4 w-4" />
                    <span>Confirm Completion</span>
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Cancel Contract Confirmation Modal */}
      {showCancelModal && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-slate-900/60 p-4 backdrop-blur-sm"
          onClick={() => {
            if (!isUpdatingStatus) setShowCancelModal(false);
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-md rounded-[10px] border border-slate-200 bg-white p-6 shadow-2xl dark:border-slate-800 dark:bg-slate-900"
          >
            <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-rose-50 text-rose-600 dark:bg-rose-950/60 dark:text-rose-400">
              <AlertTriangle className="h-6 w-6" />
            </div>

            <h3 className="mt-4 text-lg font-bold text-slate-900 dark:text-white">
              Cancel This Contract?
            </h3>
            <p className="mt-2 text-xs text-slate-600 dark:text-slate-400 leading-relaxed">
              Are you sure you want to cancel this contract? Cancellation terminates the agreement
              immediately. This is a <strong>permanent terminal state</strong>; peer reviews will be
              disabled.
            </p>

            <div className="mt-6 flex items-center justify-end gap-3">
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => setShowCancelModal(false)}
                className="rounded-[10px] border border-slate-200 bg-white px-4 py-2 text-xs font-medium text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
              >
                Keep Contract Active
              </button>
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => handleTransitionStatus("cancelled")}
                className="inline-flex items-center gap-2 rounded-[10px] bg-rose-600 px-4 py-2 text-xs font-semibold text-white shadow-md shadow-rose-600/20 transition hover:bg-rose-500 disabled:opacity-50"
              >
                {isUpdatingStatus ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Cancelling...</span>
                  </>
                ) : (
                  <>
                    <XCircle className="h-4 w-4" />
                    <span>Yes, Cancel Contract</span>
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
