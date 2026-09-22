"use client";

import React, { useState, useCallback, useMemo } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  Calendar,
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
  Briefcase,
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
import { formatJobDate, formatJobBudget, getContractStatusBadgeClasses } from "@/lib/formatters";
import { ReviewModal } from "@/components/reviews/ReviewModal";
import type { ContractWithDetails, Review } from "@/types/api";

// Visual state machine stepper
const STATES: { key: string; label: string }[] = [
  { key: "draft", label: "Draft" },
  { key: "active", label: "Active" },
  { key: "completed", label: "Completed" },
];

function getStepIndex(status: string): number {
  if (status === "draft") return 0;
  if (status === "active") return 1;
  if (status === "completed") return 2;
  return -1; // cancelled
}

function StateStepper({ status }: { status: string }) {
  const currentIdx = getStepIndex(status);
  const isCancelled = status === "cancelled";

  if (isCancelled) {
    return (
      <div className="flex items-center gap-2">
        <span className="inline-flex items-center gap-1.5 rounded-full border border-pastel-redText/20 bg-pastel-red px-2.5 py-1 text-xs font-medium text-pastel-redText">
          <XCircle className="h-3.5 w-3.5" />
          Cancelled
        </span>
        <span className="font-mono text-xs text-muted-foreground">Contract terminated early.</span>
      </div>
    );
  }

  return (
    <div className="flex items-center gap-0">
      {STATES.map((s, idx) => {
        const isDone = currentIdx > idx;
        const isCurrent = currentIdx === idx;
        return (
          <React.Fragment key={s.key}>
            <div className="flex flex-col items-center">
              <div
                className={`flex h-7 w-7 items-center justify-center rounded-full border text-xs font-mono font-medium transition-colors ${
                  isDone
                    ? "border-pastel-greenText/30 bg-pastel-green text-pastel-greenText"
                    : isCurrent
                      ? "border-foreground bg-foreground text-background"
                      : "border-border bg-card text-muted-foreground"
                }`}
              >
                {isDone ? <CheckCircle2 className="h-3.5 w-3.5" /> : idx + 1}
              </div>
              <span
                className={`mt-1 font-mono text-[10px] uppercase tracking-wider ${
                  isCurrent ? "text-foreground font-semibold" : "text-muted-foreground"
                }`}
              >
                {s.label}
              </span>
            </div>
            {idx < STATES.length - 1 && (
              <div
                className={`mb-4 mx-1.5 h-px w-10 transition-colors ${
                  currentIdx > idx ? "bg-foreground/30" : "bg-border"
                }`}
              />
            )}
          </React.Fragment>
        );
      })}
    </div>
  );
}

export default function ContractDetailPage() {
  const params = useParams();
  const contractId = typeof params.id === "string" ? params.id : "";

  const {
    user,
    backendUser,
    isAuthenticated,
    isLoading: isAuthLoading,
    getToken,
    login,
  } = useAuth();

  const [isReviewModalOpen, setIsReviewModalOpen] = useState(false);
  const [showCompleteModal, setShowCompleteModal] = useState(false);
  const [showCancelModal, setShowCancelModal] = useState(false);
  const [isUpdatingStatus, setIsUpdatingStatus] = useState(false);
  const [actionError, setActionError] = useState<string | null>(null);

  const {
    data: contract,
    isLoading: isContractLoading,
    error: contractError,
    refetch: refetchContract,
  } = useQuery<ContractWithDetails>(
    useCallback((client) => getContractById(contractId, client), [contractId]),
    { deps: [contractId], enabled: Boolean(isAuthenticated && contractId) }
  );

  const {
    data: reviewsData,
    isLoading: isReviewsLoading,
    refetch: refetchReviews,
  } = useQuery<Review[]>(
    useCallback((client) => getContractReviews(contractId, client), [contractId]),
    { deps: [contractId], enabled: Boolean(isAuthenticated && contractId) }
  );

  const reviews = useMemo(() => reviewsData ?? [], [reviewsData]);

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

  const handleTransitionStatus = async (targetStatus: "active" | "completed" | "cancelled") => {
    if (!contractId || isUpdatingStatus) return;
    setIsUpdatingStatus(true);
    setActionError(null);
    try {
      const client = createApiClient(getToken);
      await updateContractStatus(contractId, { status: targetStatus }, client);
      setShowCompleteModal(false);
      setShowCancelModal(false);
      await refetchContract();
      if (targetStatus === "completed") await refetchReviews();
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

  const handleReviewSuccess = async () => {
    await refetchReviews();
  };

  // Unauthenticated
  if (!isAuthLoading && !isAuthenticated) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center">
          <h2 className="font-serif text-xl font-medium text-foreground">Sign In Required</h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please sign in to view this contract and its peer reviews.
          </p>
          <button
            onClick={() => login({ redirectPath: `/contracts/${contractId}` })}
            className="mt-6 inline-flex items-center justify-center rounded-md bg-foreground px-5 py-2.5 text-sm font-medium text-background transition hover:opacity-90"
          >
            Sign In to Continue
          </button>
        </div>
      </div>
    );
  }

  // Loading
  if (isContractLoading) {
    return (
      <div className="mx-auto max-w-5xl px-4 py-12 sm:px-6 lg:px-8">
        <div className="animate-pulse space-y-6">
          <div className="h-4 w-28 rounded bg-muted" />
          <div className="rounded-xl border border-border bg-card p-8">
            <div className="h-7 w-2/3 rounded bg-muted mb-3" />
            <div className="h-4 w-1/3 rounded bg-muted mb-6" />
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div className="h-16 rounded bg-muted" />
              <div className="h-16 rounded bg-muted" />
              <div className="h-16 rounded bg-muted" />
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Error / Not found
  if (contractError || !contract) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center">
          <AlertCircle className="mx-auto h-8 w-8 text-muted-foreground" />
          <h2 className="mt-4 font-serif text-xl font-medium text-foreground">
            Contract Not Found
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            {contractError?.message ||
              "The requested contract does not exist or you do not have permission to view it."}
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <Link
              href="/contracts"
              className="inline-flex items-center gap-1.5 rounded-md border border-border px-4 py-2 text-xs font-medium text-foreground transition hover:bg-muted"
            >
              <ArrowLeft className="h-3.5 w-3.5" />
              Back to Contracts
            </Link>
            <button
              onClick={() => refetchContract()}
              className="inline-flex items-center gap-1.5 rounded-md bg-foreground px-4 py-2 text-xs font-medium text-background transition hover:opacity-90"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              Retry
            </button>
          </div>
        </div>
      </div>
    );
  }

  const badge = getContractStatusBadgeClasses(contract.status);

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Back nav */}
      <div className="mb-6 flex items-center justify-between">
        <Link
          href="/contracts"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Back to Contracts
        </Link>
        <span className="font-mono text-xs text-muted-foreground">#{contract.id.slice(0, 8)}</span>
      </div>

      {/* Action error */}
      {actionError && (
        <div className="mb-6 flex items-start gap-3 rounded-xl border border-border bg-card p-4 text-xs text-foreground">
          <AlertCircle className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
          <div>
            <span className="font-medium">Action failed:</span> {actionError}
          </div>
        </div>
      )}

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-[1fr_360px] gap-8">
        {/* -- Left Column ------------------------------------------------- */}
        <div className="space-y-6">
          {/* Contract overview card */}
          <div className="rounded-xl border border-border bg-card p-6 shadow-none">
            {/* Status + Stepper */}
            <div className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-4 mb-5">
              <div className="flex flex-wrap items-center gap-2">
                <span
                  className={`inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-semibold uppercase tracking-wider ${badge.bg} ${badge.text}`}
                >
                  <span className={`h-1.5 w-1.5 rounded-full ${badge.dot}`} />
                  {badge.label}
                </span>
                {contract.job?.department && (
                  <span className="inline-flex items-center rounded-full border border-border bg-muted px-2.5 py-0.5 text-xs font-mono text-muted-foreground">
                    {contract.job.department}
                  </span>
                )}
                {contract.job && (
                  <span className="font-mono text-xs font-semibold text-foreground ml-auto sm:ml-2">
                    {formatJobBudget(contract.job)}
                  </span>
                )}
              </div>
              <StateStepper status={contract.status} />
            </div>

            {/* Title */}
            <h1 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground">
              {contract.job?.title || "Marketplace Contract"}
            </h1>

            {contract.job_id && (
              <Link
                href={`/jobs/${contract.job_id}`}
                className="mt-2 inline-flex items-center gap-1 text-xs text-muted-foreground hover:text-foreground transition"
              >
                View original job listing
                <ExternalLink className="h-3 w-3" />
              </Link>
            )}

            {/* Deliverable scope */}
            {contract.job?.description && (
              <div className="mt-5 border-l-2 border-border pl-4">
                <p className="text-[11px] font-medium uppercase tracking-wider text-muted-foreground mb-1.5">
                  Scope &amp; Requirements
                </p>
                <p className="text-sm text-foreground/80 leading-relaxed line-clamp-4">
                  {contract.job.description}
                </p>
              </div>
            )}

            {/* Key dates */}
            <div className="mt-6 pt-5 border-t border-border grid grid-cols-1 sm:grid-cols-3 gap-4">
              <div className="flex items-start gap-2.5">
                <Calendar className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
                <div>
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                    Created
                  </p>
                  <p className="font-mono text-xs text-foreground">
                    {formatJobDate(contract.created_at)}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-2.5">
                <Clock className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
                <div>
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                    Started
                  </p>
                  <p className="font-mono text-xs text-foreground">
                    {contract.started_at ? formatJobDate(contract.started_at) : "Pending"}
                  </p>
                </div>
              </div>
              <div className="flex items-start gap-2.5">
                {contract.status === "completed" ? (
                  <CheckCircle2 className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
                ) : (
                  <Clock className="h-4 w-4 text-muted-foreground mt-0.5 shrink-0" />
                )}
                <div>
                  <p className="text-[10px] font-medium uppercase tracking-wider text-muted-foreground">
                    {contract.status === "completed" ? "Completed" : "Status"}
                  </p>
                  <p className="font-mono text-xs text-foreground">
                    {contract.completed_at
                      ? formatJobDate(contract.completed_at)
                      : contract.status === "active"
                        ? "In Progress"
                        : contract.status === "cancelled"
                          ? "Cancelled"
                          : "Draft"}
                  </p>
                </div>
              </div>
            </div>
          </div>

          {/* Status action bar */}
          {isParticipant && (
            <div>
              {/* Draft: Activate */}
              {contract.status === "draft" && (
                <div className="rounded-xl border border-border bg-card p-5 shadow-none flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                  <p className="text-xs text-muted-foreground">
                    This contract is in draft. Activate it to begin work on the deliverable.
                  </p>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setShowCancelModal(true)}
                      disabled={isUpdatingStatus}
                      className="rounded-md border border-border px-3.5 py-2 text-xs font-medium text-muted-foreground transition hover:bg-muted disabled:opacity-50"
                    >
                      Cancel Contract
                    </button>
                    <button
                      type="button"
                      onClick={() => handleTransitionStatus("active")}
                      disabled={isUpdatingStatus}
                      className="inline-flex items-center gap-2 rounded-md bg-foreground px-4 py-2 text-xs font-medium text-background transition hover:opacity-90 disabled:opacity-50"
                    >
                      {isUpdatingStatus ? (
                        <>
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                          Activating...
                        </>
                      ) : (
                        <>
                          <CheckCircle2 className="h-3.5 w-3.5" />
                          Activate Contract
                        </>
                      )}
                    </button>
                  </div>
                </div>
              )}

              {/* Active: Complete Deliverable */}
              {contract.status === "active" && (
                <div className="rounded-xl border border-border bg-card p-5 shadow-none flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3">
                  <p className="text-xs text-muted-foreground">
                    Work is in progress. Mark as completed once all deliverables are satisfied.
                  </p>
                  <div className="flex items-center gap-2">
                    <button
                      type="button"
                      onClick={() => setShowCancelModal(true)}
                      disabled={isUpdatingStatus}
                      className="rounded-md border border-border px-3.5 py-2 text-xs font-medium text-muted-foreground transition hover:bg-muted disabled:opacity-50"
                    >
                      Cancel
                    </button>
                    {isClient && (
                      <button
                        type="button"
                        onClick={() => setShowCompleteModal(true)}
                        disabled={isUpdatingStatus}
                        className="inline-flex items-center gap-2 rounded-md bg-foreground px-4 py-2 text-xs font-medium text-background transition hover:opacity-90 disabled:opacity-50"
                      >
                        <CheckCircle2 className="h-3.5 w-3.5" />
                        Complete Deliverable
                      </button>
                    )}
                  </div>
                </div>
              )}
            </div>
          )}

          {/* Reviews & Ratings */}
          <div className="rounded-xl border border-border bg-card p-6 shadow-none">
            <div className="flex items-center justify-between pb-4 border-b border-border">
              <div>
                <h2 className="font-serif text-xl font-medium text-foreground">
                  Peer Reviews &amp; Ratings
                </h2>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Mutual feedback exchanged upon completion
                </p>
              </div>

              {contract.status === "completed" && isParticipant && (
                <div>
                  {hasReviewed ? (
                    <span className="inline-flex items-center gap-1.5 rounded border border-border bg-muted px-2.5 py-1 text-xs font-medium text-muted-foreground">
                      <CheckCircle2 className="h-3.5 w-3.5" />
                      Reviewed
                    </span>
                  ) : (
                    <button
                      type="button"
                      onClick={() => setIsReviewModalOpen(true)}
                      className="inline-flex items-center gap-1.5 rounded-md bg-foreground px-3.5 py-2 text-xs font-medium text-background transition hover:opacity-90"
                    >
                      <Star className="h-3.5 w-3.5" />
                      Leave a Review
                    </button>
                  )}
                </div>
              )}
            </div>

            {/* Not completed: locked notice */}
            {contract.status !== "completed" && (
              <div className="mt-5 rounded-lg border border-border bg-muted/30 p-6 text-center">
                <Star className="mx-auto h-6 w-6 text-muted-foreground" />
                <p className="mt-2 text-sm font-medium text-foreground">
                  Reviews unlock upon completion
                </p>
                <p className="mx-auto mt-1 max-w-sm text-xs text-muted-foreground">
                  {contract.status === "cancelled"
                    ? "Reviews are not available for cancelled contracts."
                    : "Both parties may submit a 1-5 star rating and written review once the contract is completed."}
                </p>
              </div>
            )}

            {/* Reviews list */}
            {contract.status === "completed" && (
              <div className="mt-5">
                {isReviewsLoading && (
                  <div className="space-y-3">
                    <div className="h-16 animate-pulse rounded-lg bg-muted" />
                    <div className="h-16 animate-pulse rounded-lg bg-muted" />
                  </div>
                )}

                {!isReviewsLoading && reviews.length === 0 && (
                  <div className="rounded-lg border border-dashed border-border p-8 text-center">
                    <Star className="mx-auto h-6 w-6 text-muted-foreground" />
                    <p className="mt-2 text-sm font-medium text-foreground">No reviews yet</p>
                    <p className="mt-1 text-xs text-muted-foreground">
                      Be the first to share feedback on this collaboration.
                    </p>
                    {isParticipant && !hasReviewed && (
                      <button
                        type="button"
                        onClick={() => setIsReviewModalOpen(true)}
                        className="mt-4 inline-flex items-center gap-1.5 rounded-md bg-foreground px-3.5 py-2 text-xs font-medium text-background transition hover:opacity-90"
                      >
                        Leave a Review
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
                          `${rev.reviewer.first_name} ${rev.reviewer.last_name}`.trim() ===
                            user.name);

                      const reviewerDisplayName = rev.reviewer
                        ? `${rev.reviewer.first_name} ${rev.reviewer.last_name}`
                        : "Collaborator";

                      const reviewerRoleLabel =
                        rev.reviewer?.role === "student" ? "Student" : "Employer";

                      return (
                        <div
                          key={rev.id}
                          className={`rounded-lg border p-4 ${
                            isMyReview
                              ? "border-foreground/20 bg-muted/40"
                              : "border-border bg-card"
                          }`}
                        >
                          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
                            <div className="flex items-center gap-2.5">
                              <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-xs font-mono font-semibold text-foreground">
                                {reviewerDisplayName.slice(0, 2).toUpperCase()}
                              </div>
                              <div>
                                <div className="flex items-center gap-2">
                                  <span className="text-xs font-medium text-foreground">
                                    {reviewerDisplayName}
                                  </span>
                                  <span className="rounded border border-border bg-muted px-1.5 py-0.5 text-[10px] text-muted-foreground">
                                    {reviewerRoleLabel}
                                  </span>
                                  {isMyReview && (
                                    <span className="rounded border border-border bg-muted px-1.5 py-0.5 text-[10px] font-medium text-foreground">
                                      You
                                    </span>
                                  )}
                                </div>
                                <span className="font-mono text-[11px] text-muted-foreground">
                                  {formatJobDate(rev.created_at)}
                                </span>
                              </div>
                            </div>

                            {/* Stars */}
                            <div className="flex items-center gap-1">
                              {[1, 2, 3, 4, 5].map((s) => (
                                <svg
                                  key={s}
                                  viewBox="0 0 16 16"
                                  className={`h-4 w-4 ${s <= rev.rating ? "fill-foreground" : "fill-muted stroke-border"}`}
                                  aria-hidden="true"
                                >
                                  <path d="M8 1l1.854 3.756 4.146.603-3 2.923.708 4.129L8 10.25l-3.708 1.95.708-4.13-3-2.922 4.146-.603z" />
                                </svg>
                              ))}
                              <span className="ml-1 font-mono text-xs text-foreground">
                                {rev.rating}.0
                              </span>
                            </div>
                          </div>

                          {rev.comment && (
                            <p className="mt-3 border-l-2 border-border pl-3 text-xs text-foreground/80 leading-relaxed">
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
        </div>

        {/* -- Right Column (sidebar) --------------------------------------- */}
        <div className="space-y-5">
          {/* Student participant */}
          <div className="rounded-xl border border-border bg-card p-5 shadow-none">
            <div className="flex items-center justify-between pb-3 border-b border-border mb-3">
              <div className="flex items-center gap-2">
                <GraduationCap className="h-4 w-4 text-muted-foreground" />
                <span className="text-xs font-medium text-foreground">Student Freelancer</span>
              </div>
              {isFreelancer && (
                <span className="rounded border border-border bg-muted px-1.5 py-0.5 text-[10px] font-medium text-foreground">
                  You
                </span>
              )}
            </div>
            <div className="space-y-2.5 text-xs">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Name</span>
                <span className="font-medium text-foreground">
                  {contract.student
                    ? `${contract.student.first_name} ${contract.student.last_name}`
                    : "Student"}
                </span>
              </div>
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Email</span>
                <span className="font-mono text-foreground">
                  {contract.student?.email || "N/A"}
                </span>
              </div>
              {contract.student?.department && (
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Department</span>
                  <span className="text-foreground">{contract.student.department}</span>
                </div>
              )}
              {contract.student?.graduation_year && (
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Class of</span>
                  <span className="font-mono text-foreground">
                    {contract.student.graduation_year}
                  </span>
                </div>
              )}
              <div className="flex items-center justify-between pt-2 border-t border-border">
                <span className="text-muted-foreground">Status</span>
                <span className="inline-flex items-center gap-1 text-foreground">
                  <ShieldCheck className="h-3.5 w-3.5" />
                  Verified .edu
                </span>
              </div>
            </div>
          </div>

          {/* Employer participant */}
          <div className="rounded-xl border border-border bg-card p-5 shadow-none">
            <div className="flex items-center justify-between pb-3 border-b border-border mb-3">
              <div className="flex items-center gap-2">
                <Building2 className="h-4 w-4 text-muted-foreground" />
                <span className="text-xs font-medium text-foreground">Employer</span>
              </div>
              {isClient && (
                <span className="rounded border border-border bg-muted px-1.5 py-0.5 text-[10px] font-medium text-foreground">
                  You
                </span>
              )}
            </div>
            <div className="space-y-2.5 text-xs">
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Company</span>
                <span className="font-medium text-foreground">
                  {contract.employer?.company_or_org || "Employer"}
                </span>
              </div>
              {contract.employer?.contact_name && (
                <div className="flex items-center justify-between">
                  <span className="text-muted-foreground">Contact</span>
                  <span className="text-foreground">{contract.employer.contact_name}</span>
                </div>
              )}
              <div className="flex items-center justify-between">
                <span className="text-muted-foreground">Email</span>
                <span className="font-mono text-foreground">
                  {contract.employer?.email || "N/A"}
                </span>
              </div>
              <div className="flex items-center justify-between pt-2 border-t border-border">
                <span className="text-muted-foreground">Role</span>
                <span className="inline-flex items-center gap-1 text-foreground">
                  <Briefcase className="h-3.5 w-3.5" />
                  Verified Employer
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>

      {/* Review Modal */}
      <ReviewModal
        isOpen={isReviewModalOpen}
        onClose={() => setIsReviewModalOpen(false)}
        contractId={contract.id}
        jobTitle={contract.job?.title}
        counterpartyName={counterpartyName}
        counterpartyRole={counterpartyRole}
        onSuccess={handleReviewSuccess}
      />

      {/* Complete confirmation modal */}
      {showCompleteModal && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4 backdrop-blur-sm"
          onClick={() => {
            if (!isUpdatingStatus) setShowCompleteModal(false);
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-none"
          >
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted">
              <CheckCircle2 className="h-5 w-5 text-foreground" />
            </div>
            <h3 className="mt-4 font-serif text-xl font-medium text-foreground">
              Mark as Completed?
            </h3>
            <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
              This confirms all agreed deliverables have been fulfilled. Once completed, the
              contract moves to a permanent terminal state and unlocks peer review submissions.
            </p>
            <div className="mt-6 flex items-center justify-end gap-3">
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => setShowCompleteModal(false)}
                className="rounded-md border border-border px-4 py-2 text-xs font-medium text-foreground transition hover:bg-muted disabled:opacity-50"
              >
                Go Back
              </button>
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => handleTransitionStatus("completed")}
                className="inline-flex items-center gap-2 rounded-md bg-foreground px-4 py-2 text-xs font-medium text-background transition hover:opacity-90 disabled:opacity-50"
              >
                {isUpdatingStatus ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    Completing...
                  </>
                ) : (
                  <>
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    Confirm Completion
                  </>
                )}
              </button>
            </div>
          </div>
        </div>
      )}

      {/* Cancel confirmation modal */}
      {showCancelModal && (
        <div
          role="dialog"
          aria-modal="true"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/30 p-4 backdrop-blur-sm"
          onClick={() => {
            if (!isUpdatingStatus) setShowCancelModal(false);
          }}
        >
          <div
            onClick={(e) => e.stopPropagation()}
            className="w-full max-w-md rounded-xl border border-border bg-card p-6 shadow-none"
          >
            <div className="flex h-10 w-10 items-center justify-center rounded-md bg-muted">
              <AlertTriangle className="h-5 w-5 text-muted-foreground" />
            </div>
            <h3 className="mt-4 font-serif text-xl font-medium text-foreground">
              Cancel This Contract?
            </h3>
            <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
              Cancellation terminates the agreement immediately. This is a permanent terminal state;
              peer reviews will be disabled.
            </p>
            <div className="mt-6 flex items-center justify-end gap-3">
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => setShowCancelModal(false)}
                className="rounded-md border border-border px-4 py-2 text-xs font-medium text-foreground transition hover:bg-muted disabled:opacity-50"
              >
                Keep Active
              </button>
              <button
                type="button"
                disabled={isUpdatingStatus}
                onClick={() => handleTransitionStatus("cancelled")}
                className="inline-flex items-center gap-2 rounded-md border border-border bg-muted px-4 py-2 text-xs font-medium text-foreground transition hover:bg-foreground hover:text-background disabled:opacity-50"
              >
                {isUpdatingStatus ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    Cancelling...
                  </>
                ) : (
                  <>
                    <XCircle className="h-3.5 w-3.5" />
                    Yes, Cancel
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
