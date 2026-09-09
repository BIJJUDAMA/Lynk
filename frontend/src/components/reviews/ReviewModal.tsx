"use client";

import React, { useState, useEffect, useCallback } from "react";
import {
  Star,
  X,
  AlertCircle,
  Loader2,
  CheckCircle2,
  ShieldCheck,
} from "lucide-react";
import { createReview, ApiClientError, createApiClient } from "@/lib/api";
import { useAuth } from "@/components/auth/AuthProvider";
import { animateModal } from "@/lib/animations";
import type { Review } from "@/types/api";

export interface ReviewModalProps {
  isOpen: boolean;
  onClose: () => void;
  contractId: string;
  jobTitle?: string;
  counterpartyName?: string;
  counterpartyRole?: string;
  onSuccess?: (newReview: Review) => void;
}

const RATING_LABELS: Record<number, string> = {
  1: "1 - Poor",
  2: "2 - Fair",
  3: "3 - Good",
  4: "4 - Very Good",
  5: "5 - Excellent",
};

export function ReviewModal({
  isOpen,
  onClose,
  contractId,
  jobTitle,
  counterpartyName,
  counterpartyRole,
  onSuccess,
}: ReviewModalProps) {
  const { getToken } = useAuth();

  const [rating, setRating] = useState<number>(0);
  const [hoveredRating, setHoveredRating] = useState<number>(0);
  const [comment, setComment] = useState<string>("");
  const [isSubmitting, setIsSubmitting] = useState<boolean>(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Reset form when modal opens
  useEffect(() => {
    if (isOpen) {
      setRating(0);
      setHoveredRating(0);
      setComment("");
      setErrorMessage(null);
      setIsSubmitting(false);
    }
  }, [isOpen]);

  const modalCardRef = React.useRef<HTMLDivElement | null>(null);

  // Handle ESC key to dismiss modal
  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "Escape" && !isSubmitting) {
        onClose();
      }
    },
    [isSubmitting, onClose]
  );

  useEffect(() => {
    if (isOpen) {
      window.addEventListener("keydown", handleKeyDown);
      return () => window.removeEventListener("keydown", handleKeyDown);
    }
  }, [isOpen, handleKeyDown]);

  // GSAP entrance animation
  useEffect(() => {
    if (isOpen && modalCardRef.current) {
      const revert = animateModal(modalCardRef.current);
      return () => revert();
    }
  }, [isOpen]);

  if (!isOpen) return null;

  const currentDisplayRating = hoveredRating || rating;
  const ratingText = currentDisplayRating > 0 ? RATING_LABELS[currentDisplayRating] : "Select a rating (1 to 5 stars)";
  const trimmedComment = comment.trim();
  const isFormValid = rating >= 1 && rating <= 5 && trimmedComment.length >= 5;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isSubmitting) return;

    if (rating < 1 || rating > 5) {
      setErrorMessage("Please select a rating between 1 and 5 stars.");
      return;
    }

    if (trimmedComment.length < 5) {
      setErrorMessage("Please enter at least 5 characters in your written review.");
      return;
    }

    if (trimmedComment.length > 5000) {
      setErrorMessage("Review comment cannot exceed 5,000 characters.");
      return;
    }

    setIsSubmitting(true);
    setErrorMessage(null);

    try {
      const client = createApiClient(getToken);
      const newReview = await createReview(
        contractId,
        {
          rating,
          comment: trimmedComment,
        },
        client
      );

      onSuccess?.(newReview);
      onClose();
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        if (err.code === "DUPLICATE_REVIEW" || err.status === 409) {
          setErrorMessage("You have already submitted a review for this contract.");
        } else if (err.code === "CONTRACT_NOT_COMPLETED" || (err.status === 400 && err.message.toLowerCase().includes("completed"))) {
          setErrorMessage("Reviews can only be submitted for completed contracts.");
        } else if (err.code === "FORBIDDEN" || err.status === 403) {
          setErrorMessage("Only participants of this contract are permitted to submit reviews.");
        } else {
          setErrorMessage(err.message || "Failed to submit review. Please try again.");
        }
      } else if (err instanceof Error) {
        setErrorMessage(err.message || "An unexpected error occurred. Please try again.");
      } else {
        setErrorMessage("Failed to submit review. Please try again.");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="review-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-slate-900/60 p-4 backdrop-blur-sm animate-in fade-in duration-200"
      onClick={() => {
        if (!isSubmitting) onClose();
      }}
    >
      <div
        ref={modalCardRef}
        onClick={(e) => e.stopPropagation()}
        className="w-full max-w-lg rounded-[10px] border border-border bg-card p-6 shadow-2xl transition-all text-card-foreground sm:p-7"
      >
        {/* Header */}
        <div className="flex items-start justify-between pb-4 border-b border-slate-100 dark:border-slate-800">
          <div className="flex items-center gap-3">
            <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400">
              <Star className="h-5 w-5 fill-amber-400 text-amber-400" />
            </div>
            <div>
              <h2
                id="review-modal-title"
                className="text-lg font-bold text-slate-900 dark:text-white"
              >
                Leave a Peer Review
              </h2>
              <p className="text-xs text-slate-500 dark:text-slate-400">
                {jobTitle ? (
                  <span>For <strong className="font-semibold text-slate-700 dark:text-slate-300">{jobTitle}</strong></span>
                ) : (
                  "Share your feedback with your project collaborator"
                )}
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            disabled={isSubmitting}
            className="rounded-[10px] p-1.5 text-slate-400 transition hover:bg-slate-100 hover:text-slate-600 dark:hover:bg-slate-800 dark:hover:text-slate-200 disabled:opacity-50"
            aria-label="Close dialog"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Counterparty context */}
        {counterpartyName && (
          <div className="mt-4 flex items-center gap-2 rounded-[10px] bg-slate-50 px-3.5 py-2.5 text-xs text-slate-600 dark:bg-slate-800/60 dark:text-slate-300">
            <ShieldCheck className="h-4 w-4 text-primary dark:text-emerald-500 shrink-0" />
            <span>
              Reviewing{" "}
              <strong className="font-semibold text-slate-900 dark:text-white">
                {counterpartyName}
              </strong>
              {counterpartyRole ? ` (${counterpartyRole})` : ""}
            </span>
          </div>
        )}

        {/* Error Alert */}
        {errorMessage && (
          <div className="mt-4 flex items-start gap-2.5 rounded-[10px] border border-rose-200 bg-rose-50 p-3.5 text-xs text-rose-800 dark:border-rose-900/60 dark:bg-rose-950/40 dark:text-rose-300">
            <AlertCircle className="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400 mt-0.5" />
            <span>{errorMessage}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="mt-5 space-y-5">
          {/* Star Rating Selector */}
          <div>
            <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-2">
              Overall Rating <span className="text-rose-500">*</span>
            </label>
            <div className="flex items-center gap-2">
              <div
                className="flex items-center gap-1.5"
                onMouseLeave={() => setHoveredRating(0)}
              >
                {[1, 2, 3, 4, 5].map((starValue) => {
                  const isFilled = starValue <= currentDisplayRating;
                  return (
                    <button
                      key={starValue}
                      type="button"
                      disabled={isSubmitting}
                      onClick={() => {
                        setRating(starValue);
                        setErrorMessage(null);
                      }}
                      onMouseEnter={() => setHoveredRating(starValue)}
                      className="group rounded-[10px] p-1 transition transform hover:scale-110 focus:outline-none focus:ring-2 focus:ring-amber-400"
                      aria-label={`Rate ${starValue} of 5 stars`}
                    >
                      <Star
                        className={`h-7 w-7 transition-colors ${
                          isFilled
                            ? "fill-amber-400 text-amber-400"
                            : "text-slate-300 dark:text-slate-600 group-hover:text-amber-300"
                        }`}
                      />
                    </button>
                  );
                })}
              </div>
              <span className="ml-2 text-xs font-medium text-slate-600 dark:text-slate-400">
                {ratingText}
              </span>
            </div>
          </div>

          {/* Written Feedback Textarea */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label
                htmlFor="review-comment"
                className="block text-xs font-semibold text-slate-700 dark:text-slate-300"
              >
                Written Feedback <span className="text-rose-500">*</span>
              </label>
              <span
                className={`text-[11px] ${
                  trimmedComment.length > 5000
                    ? "text-rose-600 font-semibold"
                    : trimmedComment.length >= 5
                    ? "text-slate-500 dark:text-slate-400"
                    : "text-amber-600 dark:text-amber-400"
                }`}
              >
                {comment.length} / 5,000 characters (min 5)
              </span>
            </div>
            <textarea
              id="review-comment"
              rows={4}
              value={comment}
              disabled={isSubmitting}
              onChange={(e) => {
                setComment(e.target.value);
                if (errorMessage) setErrorMessage(null);
              }}
              placeholder="Highlight project deliverables and communication, reliability, work quality, and what it was like working together..."
              className="w-full rounded-[10px] border border-input bg-background p-3 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-2 focus:ring-ring/20 transition-colors"
            />
          </div>

          {/* Modal Actions */}
          <div className="flex items-center justify-end gap-3 pt-3 border-t border-border">
            <button
              type="button"
              onClick={onClose}
              disabled={isSubmitting}
              className="rounded-[10px] border border-border bg-card px-4 py-2.5 text-xs font-medium text-foreground shadow-sm transition hover:bg-secondary disabled:opacity-50"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!isFormValid || isSubmitting}
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary px-5 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  <span>Submitting Review...</span>
                </>
              ) : (
                <>
                  <CheckCircle2 className="h-4 w-4" />
                  <span>Submit Review</span>
                </>
              )}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
