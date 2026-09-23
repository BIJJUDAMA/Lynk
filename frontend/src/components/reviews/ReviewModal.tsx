"use client";

import React, { useState, useEffect, useCallback } from "react";
import { AlertCircle, Loader2, CheckCircle2, ShieldCheck } from "lucide-react";
import { createReview, ApiClientError, createApiClient } from "@/lib/api";
import { useAuth } from "@/components/auth/AuthProvider";
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog";
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
  1: "Poor",
  2: "Fair",
  3: "Good",
  4: "Very Good",
  5: "Excellent",
};

// SVG star primitive - no emoji, no text star
function StarIcon({
  filled,
  hovered,
  className,
}: {
  filled: boolean;
  hovered: boolean;
  className?: string;
}) {
  return (
    <svg viewBox="0 0 20 20" aria-hidden="true" className={className} style={{ display: "block" }}>
      <path
        d="M10 1.5l2.47 5.01 5.53.8-4 3.9.94 5.5L10 14.27l-4.94 2.59.94-5.5-4-3.9 5.53-.8z"
        fill={filled || hovered ? "currentColor" : "none"}
        stroke="currentColor"
        strokeWidth={filled || hovered ? "0" : "1.5"}
        strokeLinejoin="round"
      />
    </svg>
  );
}

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

  const resetForm = () => {
    setRating(0);
    setHoveredRating(0);
    setComment("");
    setErrorMessage(null);
    setIsSubmitting(false);
  };

  const handleClose = useCallback(() => {
    if (isSubmitting) return;
    resetForm();
    onClose();
  }, [isSubmitting, onClose]);

  const handleKeyDown = useCallback(
    (e: KeyboardEvent) => {
      if (e.key === "Escape") handleClose();
    },
    [handleClose]
  );

  useEffect(() => {
    if (isOpen) {
      window.addEventListener("keydown", handleKeyDown);
      return () => window.removeEventListener("keydown", handleKeyDown);
    }
  }, [isOpen, handleKeyDown]);

  const trimmedComment = comment.trim();
  const isFormValid = rating >= 1 && rating <= 5 && trimmedComment.length >= 5;
  const currentDisplayRating = hoveredRating || rating;

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (isSubmitting) return;

    if (rating < 1 || rating > 5) {
      setErrorMessage("Please select a rating between 1 and 5 stars.");
      return;
    }
    if (trimmedComment.length < 5) {
      setErrorMessage("Please enter at least 5 characters in your review.");
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
      const newReview = await createReview(contractId, { rating, comment: trimmedComment }, client);
      resetForm();
      onSuccess?.(newReview);
      onClose();
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        if (err.code === "DUPLICATE_REVIEW" || err.status === 409) {
          setErrorMessage("You have already submitted a review for this contract.");
        } else if (
          err.code === "CONTRACT_NOT_COMPLETED" ||
          (err.status === 400 && err.message.toLowerCase().includes("completed"))
        ) {
          setErrorMessage("Reviews can only be submitted for completed contracts.");
        } else if (err.code === "FORBIDDEN" || err.status === 403) {
          setErrorMessage("Only participants of this contract may submit reviews.");
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
    <Dialog
      open={isOpen}
      onOpenChange={(open) => {
        if (!open) handleClose();
      }}
    >
      <DialogContent
        onClose={isSubmitting ? undefined : handleClose}
        className="max-w-lg"
        aria-labelledby="review-modal-title"
      >
        <DialogHeader>
          <DialogTitle id="review-modal-title">Leave a Peer Review</DialogTitle>
          <p className="text-sm text-muted-foreground">
            {jobTitle ? (
              <>
                For <span className="font-medium text-foreground">{jobTitle}</span>
              </>
            ) : (
              "Share your feedback with your project collaborator."
            )}
          </p>
        </DialogHeader>

        {/* Counterparty context */}
        {counterpartyName && (
          <div className="flex items-center gap-2 rounded-lg border border-border bg-muted/40 px-3.5 py-2.5 text-xs text-muted-foreground mb-2">
            <ShieldCheck className="h-3.5 w-3.5 shrink-0 text-foreground" />
            <span>
              Reviewing <span className="font-medium text-foreground">{counterpartyName}</span>
              {counterpartyRole ? ` (${counterpartyRole})` : ""}
            </span>
          </div>
        )}

        {/* Error alert */}
        {errorMessage && (
          <div className="flex items-start gap-2.5 rounded-lg border border-border bg-muted/40 p-3.5 text-xs text-foreground mb-2">
            <AlertCircle className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
            <span>{errorMessage}</span>
          </div>
        )}

        <form onSubmit={handleSubmit} className="space-y-5">
          {/* Star rating selector */}
          <div>
            <label className="block text-xs font-medium text-foreground mb-2">
              Overall Rating <span className="text-muted-foreground">*</span>
            </label>
            <div className="flex items-center gap-3">
              <div
                className="flex items-center gap-1"
                onMouseLeave={() => setHoveredRating(0)}
                role="radiogroup"
                aria-label="Star rating"
              >
                {[1, 2, 3, 4, 5].map((starValue) => {
                  const isFilled = starValue <= currentDisplayRating;
                  const isHovered = hoveredRating > 0 && starValue <= hoveredRating;
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
                      className="p-0.5 rounded transition-transform hover:scale-110 focus:outline-none focus-visible:ring-2 focus-visible:ring-foreground disabled:opacity-50"
                      aria-label={`Rate ${starValue} out of 5`}
                      role="radio"
                      aria-checked={rating === starValue}
                    >
                      <StarIcon
                        filled={isFilled && !isHovered}
                        hovered={isHovered}
                        className={`h-7 w-7 transition-colors ${
                          isFilled
                            ? "text-foreground"
                            : "text-muted-foreground hover:text-foreground/60"
                        }`}
                      />
                    </button>
                  );
                })}
              </div>
              {currentDisplayRating > 0 && (
                <span className="text-xs font-mono text-muted-foreground">
                  {currentDisplayRating}/5 &mdash; {RATING_LABELS[currentDisplayRating]}
                </span>
              )}
              {currentDisplayRating === 0 && (
                <span className="text-xs text-muted-foreground">Select 1-5 stars</span>
              )}
            </div>
          </div>

          {/* Comment textarea */}
          <div>
            <div className="flex items-center justify-between mb-1.5">
              <label htmlFor="review-comment" className="text-xs font-medium text-foreground">
                Written Feedback <span className="text-muted-foreground">*</span>
              </label>
              <span
                className={`text-[11px] font-mono ${
                  trimmedComment.length > 5000
                    ? "text-foreground font-semibold"
                    : "text-muted-foreground"
                }`}
              >
                {comment.length}/5000
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
              placeholder="Share specific feedback on deliverable quality, communication, and timeliness..."
              className="w-full min-h-[100px] rounded-md border border-border bg-background px-3 py-2.5 text-sm text-foreground placeholder:text-muted-foreground focus:border-foreground/50 focus:outline-none transition resize-y"
            />
          </div>

          {/* Actions */}
          <div className="flex items-center justify-end gap-3 pt-3 border-t border-border">
            <button
              type="button"
              onClick={handleClose}
              disabled={isSubmitting}
              className="rounded-md border border-border px-4 py-2 text-xs font-medium text-foreground transition hover:bg-muted disabled:opacity-50 active:scale-[0.98]"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={!isFormValid || isSubmitting}
              className="inline-flex items-center gap-2 rounded-md bg-foreground px-5 py-2 text-xs font-medium text-background transition hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40 active:scale-[0.98]"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  Submitting...
                </>
              ) : (
                <>
                  <CheckCircle2 className="h-3.5 w-3.5" />
                  Submit Review
                </>
              )}
            </button>
          </div>
        </form>
      </DialogContent>
    </Dialog>
  );
}
