"use client";

import React, { useState, useEffect } from "react";
import {
  UserCheck,
  Star,
  XCircle,
  FileText,
  Mail,
  GraduationCap,
  Calendar,
  AlertCircle,
  Loader2,
  CheckCircle2,
  ExternalLink,
} from "lucide-react";
import type { ProjectApplication } from "@/types/project";
import {
  acceptProjectApplication,
  shortlistProjectApplication,
  rejectProjectApplication,
} from "@/lib/project-api";
import { getResumeUrl } from "@/lib/api";
import { formatJobDate } from "@/lib/formatters";
import {
  getApplicantDisplayName,
  getApplicantEmail,
  getApplicantDepartment,
  getApplicantInitials,
  getApplicationStatusBadgeClasses,
} from "@/lib/recruitment";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";

export interface ApplicantReviewModalProps {
  open: boolean;
  onClose: () => void;
  application: ProjectApplication | null;
  projectTitle?: string;
  onDecision?: (updatedApp: ProjectApplication) => void;
}

export function ApplicantReviewModal({
  open,
  onClose,
  application,
  projectTitle,
  onDecision,
}: ApplicantReviewModalProps) {
  const [notes, setNotes] = useState<string>("");
  const [isSubmitting, setIsSubmitting] = useState<"accept" | "shortlist" | "reject" | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);

  const [isLoadingResume, setIsLoadingResume] = useState<boolean>(false);
  const [resumeError, setResumeError] = useState<string | null>(null);

  useEffect(() => {
    if (application) {
      setNotes(application.reviewer_notes || "");
      setActionError(null);
      setResumeError(null);
      setIsSubmitting(null);
    }
  }, [application]);

  if (!application) {
    return null;
  }

  const applicantName = getApplicantDisplayName(application);
  const applicantEmail = getApplicantEmail(application);
  const applicantDepartment = getApplicantDepartment(application);
  const applicantInitials = getApplicantInitials(applicantName);
  const statusStyle = getApplicationStatusBadgeClasses(application.status);
  const isDecided =
    application.status === "accepted" ||
    application.status === "rejected" ||
    application.status === "withdrawn";

  const applicantBio = application.applicant?.bio;
  const hasResume = Boolean(application.resume_asset_key || application.applicant_id);

  const handleViewResume = async () => {
    if (!application) return;

    if (
      application.resume_asset_key &&
      (application.resume_asset_key.startsWith("http://") ||
        application.resume_asset_key.startsWith("https://"))
    ) {
      window.open(application.resume_asset_key, "_blank", "noopener,noreferrer");
      return;
    }

    setIsLoadingResume(true);
    setResumeError(null);
    try {
      const res = await getResumeUrl(application.applicant_id);
      const downloadUrl = res.download_url || res.url;
      if (downloadUrl) {
        window.open(downloadUrl, "_blank", "noopener,noreferrer");
      } else {
        setResumeError("Resume download URL could not be generated.");
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setResumeError(err.message);
      } else {
        setResumeError("Failed to retrieve resume link.");
      }
    } finally {
      setIsLoadingResume(false);
    }
  };

  const handleDecision = async (decision: "accept" | "shortlist" | "reject") => {
    if (!application || isSubmitting) return;

    setIsSubmitting(decision);
    setActionError(null);

    try {
      let updated: ProjectApplication;
      if (decision === "accept") {
        updated = await acceptProjectApplication(application.id, notes);
      } else if (decision === "shortlist") {
        updated = await shortlistProjectApplication(application.id, notes);
      } else {
        updated = await rejectProjectApplication(application.id, notes);
      }

      onDecision?.(updated);
      onClose();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setActionError(err.message);
      } else {
        setActionError("Failed to update application decision.");
      }
    } finally {
      setIsSubmitting(null);
    }
  };

  return (
    <Dialog open={open} onOpenChange={(isOpen) => !isOpen && onClose()}>
      <DialogContent className="max-w-2xl max-h-[90vh] overflow-y-auto sm:rounded-xl">
        <DialogHeader className="border-b border-border pb-4">
          <div className="flex items-start justify-between gap-4">
            <div>
              <DialogTitle className="font-serif text-2xl font-normal text-foreground">
                Applicant Review
              </DialogTitle>
              <DialogDescription className="text-xs text-muted-foreground mt-1">
                {projectTitle
                  ? `Application for ${projectTitle}`
                  : "Review student statement, background, and candidate decision."}
              </DialogDescription>
            </div>

            {/* Current Status Badge */}
            <span
              className={cn(
                "inline-flex shrink-0 items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-mono uppercase tracking-wider",
                statusStyle.bg,
                statusStyle.text,
                statusStyle.border
              )}
            >
              <span className={cn("h-1.5 w-1.5 rounded-full", statusStyle.dot)} />
              {statusStyle.label}
            </span>
          </div>
        </DialogHeader>

        <div className="space-y-6 pt-4">
          {/* Action error banner */}
          {actionError && (
            <div className="flex items-start gap-3 rounded-lg border border-rose-500/20 bg-rose-500/10 p-3.5 text-xs text-rose-600">
              <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
              <div className="flex-1">
                <p className="font-medium">Action Failed</p>
                <p className="mt-0.5">{actionError}</p>
              </div>
            </div>
          )}

          {/* Profile Summary Card */}
          <div className="rounded-lg border border-border bg-muted/30 p-4">
            <div className="flex items-start gap-3.5">
              <div className="flex h-11 w-11 shrink-0 items-center justify-center rounded-full bg-foreground text-background font-mono text-sm font-semibold">
                {applicantInitials}
              </div>
              <div className="flex-1 min-w-0">
                <h3 className="text-base font-semibold text-foreground truncate">
                  {applicantName}
                </h3>
                <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                  {applicantEmail && (
                    <div className="flex items-center gap-1.5 truncate">
                      <Mail className="h-3.5 w-3.5 shrink-0" />
                      <span className="truncate">{applicantEmail}</span>
                    </div>
                  )}
                  <div className="flex items-center gap-1.5">
                    <GraduationCap className="h-3.5 w-3.5 shrink-0" />
                    <span>{applicantDepartment}</span>
                  </div>
                  <div className="flex items-center gap-1.5 font-mono">
                    <Calendar className="h-3.5 w-3.5 shrink-0" />
                    <span>Applied {formatJobDate(application.created_at)}</span>
                  </div>
                </div>

                {applicantBio && (
                  <p className="mt-2.5 text-xs text-foreground/80 leading-relaxed border-t border-border/50 pt-2">
                    {applicantBio}
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Application Statement / Proposal */}
          <div>
            <h4 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
              Statement of Interest / Proposal
            </h4>
            <div className="mt-2 rounded-lg border border-border bg-card p-4">
              <p className="text-sm text-foreground leading-relaxed whitespace-pre-line">
                {application.statement || "No statement text provided."}
              </p>
            </div>
          </div>

          {/* Resume Attachment Section */}
          {hasResume && (
            <div>
              <h4 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
                Candidate Resume
              </h4>
              <div className="mt-2 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-3 rounded-lg border border-border bg-card p-3.5">
                <div className="flex items-center gap-2.5">
                  <div className="flex h-8 w-8 items-center justify-center rounded-md bg-muted text-muted-foreground">
                    <FileText className="h-4 w-4" />
                  </div>
                  <div>
                    <p className="text-xs font-medium text-foreground">Student Resume Document</p>
                    <p className="text-[11px] text-muted-foreground font-mono">
                      Institutional S3 object storage
                    </p>
                  </div>
                </div>

                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={handleViewResume}
                  disabled={isLoadingResume}
                  className="inline-flex items-center gap-1.5 text-xs h-8"
                >
                  {isLoadingResume ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <ExternalLink className="h-3.5 w-3.5" />
                  )}
                  <span>View Resume</span>
                </Button>
              </div>

              {resumeError && (
                <p className="mt-1.5 text-xs text-rose-600 flex items-center gap-1">
                  <AlertCircle className="h-3.5 w-3.5" />
                  <span>{resumeError}</span>
                </p>
              )}
            </div>
          )}

          {/* Reviewer Internal Notes */}
          <div>
            <label
              htmlFor="reviewer-notes"
              className="text-xs font-mono uppercase tracking-wider text-muted-foreground block"
            >
              Reviewer Notes (Internal)
            </label>
            <p className="text-[11px] text-muted-foreground mt-0.5">
              Private notes visible only to project leads and maintainers.
            </p>
            <div className="mt-2">
              <Textarea
                id="reviewer-notes"
                value={notes}
                onChange={(e) => setNotes(e.target.value)}
                placeholder="Add evaluation feedback, interview notes, or rationale..."
                disabled={Boolean(isSubmitting) || isDecided}
                className="min-h-[80px] resize-y text-xs leading-relaxed"
              />
            </div>
          </div>

          {/* Decision Actions Area */}
          <div className="border-t border-border pt-4">
            {application.status === "accepted" ? (
              <div className="flex items-center gap-2.5 rounded-lg border border-emerald-500/20 bg-emerald-500/10 p-3.5 text-xs text-emerald-700">
                <CheckCircle2 className="h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">Accepted -- Member of Team</p>
                  <p className="mt-0.5 text-emerald-600/90 text-[11px]">
                    This applicant was accepted and added directly to the project team roster.
                  </p>
                </div>
              </div>
            ) : application.status === "rejected" ? (
              <div className="flex items-center gap-2.5 rounded-lg border border-rose-500/20 bg-rose-500/10 p-3.5 text-xs text-rose-600">
                <XCircle className="h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">Application Rejected</p>
                  <p className="mt-0.5 text-rose-600/80 text-[11px]">
                    This application was not selected for this project.
                  </p>
                </div>
              </div>
            ) : application.status === "withdrawn" ? (
              <div className="flex items-center gap-2.5 rounded-lg border border-border bg-muted/40 p-3.5 text-xs text-muted-foreground">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <div>
                  <p className="font-medium">Application Withdrawn</p>
                  <p className="mt-0.5 text-[11px]">The student has withdrawn this application.</p>
                </div>
              </div>
            ) : (
              <div className="space-y-3">
                <div className="flex flex-col sm:flex-row items-stretch sm:items-center justify-between gap-3">
                  <div className="text-[11px] text-muted-foreground">
                    Acceptance atomically adds applicant to project team.
                  </div>

                  <div className="flex flex-wrap items-center gap-2">
                    {/* Reject Button */}
                    <Button
                      type="button"
                      variant="outline"
                      onClick={() => void handleDecision("reject")}
                      disabled={Boolean(isSubmitting)}
                      className="border-rose-500/30 text-rose-600 hover:bg-rose-500/10 hover:text-rose-700 text-xs h-9 inline-flex items-center gap-1.5"
                    >
                      {isSubmitting === "reject" ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <XCircle className="h-3.5 w-3.5" />
                      )}
                      <span>Reject</span>
                    </Button>

                    {/* Shortlist Button */}
                    {application.status !== "shortlisted" && (
                      <Button
                        type="button"
                        onClick={() => void handleDecision("shortlist")}
                        disabled={Boolean(isSubmitting)}
                        className="bg-amber-600 hover:bg-amber-700 text-white text-xs h-9 inline-flex items-center gap-1.5"
                      >
                        {isSubmitting === "shortlist" ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <Star className="h-3.5 w-3.5" />
                        )}
                        <span>Shortlist</span>
                      </Button>
                    )}

                    {/* Accept & Add to Team Button */}
                    <Button
                      type="button"
                      onClick={() => void handleDecision("accept")}
                      disabled={Boolean(isSubmitting)}
                      className="bg-emerald-600 hover:bg-emerald-700 text-white text-xs h-9 inline-flex items-center gap-1.5"
                    >
                      {isSubmitting === "accept" ? (
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      ) : (
                        <UserCheck className="h-3.5 w-3.5" />
                      )}
                      <span>Accept &amp; Add to Team</span>
                    </Button>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>
      </DialogContent>
    </Dialog>
  );
}
