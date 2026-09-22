"use client";

import { useState, useCallback } from "react";
import Link from "next/link";
import { useParams } from "next/navigation";
import {
  ArrowLeft,
  Calendar,
  Clock,
  Building2,
  ShieldAlert,
  Briefcase,
  AlertCircle,
  CheckCircle2,
  Send,
  FileText,
  RotateCcw,
  Users,
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
import { formatJobDate, getStatusBadgeClasses, formatJobBudget } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
  DialogFooter,
} from "@/components/ui/dialog";

export default function JobDetailPage() {
  const params = useParams();
  const jobId = typeof params.id === "string" ? params.id : "";

  const { user, backendUser, isAuthenticated, isVerified, isLoading: authLoading } = useAuth();

  // Application modal and form state
  const [applyModalOpen, setApplyModalOpen] = useState(false);
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

  // Fetch member's application for this job only
  const { data: existingApplication } = useQuery<ApplicationWithDetails | null>(
    useCallback((client) => getMyApplicationForJob(jobId, client), [jobId]),
    {
      enabled: Boolean(isAuthenticated && jobId),
    }
  );

  const alreadyApplied = Boolean(existingApplication || hasAppliedSuccess);

  // Determine if current viewer is the creator of this job
  const isCreator = Boolean(
    isAuthenticated && job && (job.created_by === user?.id || job.created_by === backendUser?.id)
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
      setApplyModalOpen(false);
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
      <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
        <div className="h-5 w-32 animate-pulse rounded bg-muted" />
        <div className="mt-6 grid grid-cols-1 lg:grid-cols-[1fr_360px] gap-8">
          <div className="rounded-xl border border-border bg-card p-6 sm:p-8 shadow-none space-y-6">
            <div className="flex gap-2">
              <div className="h-6 w-20 animate-pulse rounded-full bg-muted" />
              <div className="h-6 w-32 animate-pulse rounded-full bg-muted" />
            </div>
            <div className="h-10 w-3/4 animate-pulse rounded-md bg-muted" />
            <div className="h-5 w-1/2 animate-pulse rounded-md bg-muted" />
            <div className="space-y-3 pt-4">
              <div className="h-4 w-full animate-pulse rounded bg-muted" />
              <div className="h-4 w-5/6 animate-pulse rounded bg-muted" />
              <div className="h-4 w-4/6 animate-pulse rounded bg-muted" />
            </div>
            <div className="flex gap-2 pt-4">
              <div className="h-6 w-16 animate-pulse rounded-md bg-muted" />
              <div className="h-6 w-20 animate-pulse rounded-md bg-muted" />
              <div className="h-6 w-16 animate-pulse rounded-md bg-muted" />
            </div>
          </div>
          <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-6 h-fit">
            <div className="space-y-2">
              <div className="h-4 w-28 animate-pulse rounded bg-muted" />
              <div className="h-8 w-24 animate-pulse rounded bg-muted" />
            </div>
            <div className="space-y-2 pt-4 border-t border-border">
              <div className="h-4 w-32 animate-pulse rounded bg-muted" />
              <div className="h-5 w-20 animate-pulse rounded bg-muted" />
            </div>
            <div className="pt-4 border-t border-border">
              <div className="h-10 w-full animate-pulse rounded-md bg-muted" />
            </div>
          </div>
        </div>
      </div>
    );
  }

  // Error state
  if (jobError || !job) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-red text-pastel-redText">
          <AlertCircle className="h-6 w-6" />
        </div>
        <h1 className="mt-4 font-serif text-2xl font-normal text-foreground">
          Opportunity Not Found
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          {jobError?.message || "The requested job posting does not exist or has been removed."}
        </p>
        <div className="mt-6 flex justify-center gap-3">
          <Button onClick={() => refetchJob()} variant="outline" className="font-mono text-xs">
            <RotateCcw className="h-3.5 w-3.5 mr-1.5" />
            Retry
          </Button>
          <Link href="/jobs">
            <Button className="font-mono text-xs">
              <ArrowLeft className="h-3.5 w-3.5 mr-1.5" />
              Browse Gigs
            </Button>
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
  const budgetDisplay = formatJobBudget(job);

  return (
    <div className="mx-auto max-w-5xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Back Navigation Link */}
      <div>
        <Link
          href="/jobs"
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to Opportunities</span>
        </Link>
      </div>

      {/* Success Notification Banner */}
      {hasAppliedSuccess && (
        <div className="mt-6 flex items-center justify-between rounded-xl border border-pastel-greenText/20 bg-pastel-green p-4 text-xs text-pastel-greenText">
          <div className="flex items-center gap-2.5">
            <CheckCircle2 className="h-4 w-4 shrink-0" />
            <span>
              <strong>Proposal Submitted!</strong> Your application has been sent to the poster.
            </span>
          </div>
          <Link href="/activity" className="font-mono text-xs underline hover:text-foreground">
            View in Activity &rarr;
          </Link>
        </div>
      )}

      {/* Creator Alert Banner */}
      {isCreator && (
        <div className="mt-6 flex flex-col gap-3 rounded-xl border border-border bg-card p-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-3">
            <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-muted text-foreground">
              <Briefcase className="h-4 w-4" />
            </div>
            <div>
              <p className="text-sm font-medium text-foreground">Your Opportunity Posting</p>
              <p className="text-xs text-muted-foreground">
                You created this posting. Review proposals and manage student deliverables.
              </p>
            </div>
          </div>
          <Link href={`/jobs/${job.id}/applicants`}>
            <Button size="sm" className="font-mono text-xs">
              <Users className="h-3.5 w-3.5 mr-1.5" />
              <span>Manage Applicants</span>
            </Button>
          </Link>
        </div>
      )}

      {/* Two-Column Document Grid */}
      <div className="mt-6 grid grid-cols-1 lg:grid-cols-[1fr_360px] gap-8 items-start">
        {/* Main Document Column */}
        <div className="rounded-xl border border-border bg-card p-6 sm:p-8 shadow-none space-y-6">
          {/* Department badge and spot pastel status badge */}
          <div className="flex flex-wrap items-center gap-2">
            <span
              className={cn(
                "inline-flex items-center gap-1.5 rounded-full border px-2.5 py-0.5 text-xs font-mono uppercase tracking-wider",
                statusStyles.bg,
                statusStyles.text
              )}
            >
              <span className={cn("h-1.5 w-1.5 rounded-full", statusStyles.dot)} />
              {job.status.replace("_", " ")}
            </span>

            {job.department && (
              <Badge variant="outline" className="text-xs font-mono">
                {job.department}
              </Badge>
            )}
          </div>

          {/* Job Title */}
          <h1 className="font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
            {job.title}
          </h1>

          {/* Metadata Row */}
          <div className="flex flex-wrap items-center gap-x-6 gap-y-2 border-y border-border py-4 text-xs font-mono text-muted-foreground">
            <div className="flex items-center gap-1.5">
              <Building2 className="h-3.5 w-3.5 text-muted-foreground/80" />
              <span className="text-foreground">{creatorDisplayName}</span>
              {job.creator?.organization && <span>({job.creator.organization})</span>}
            </div>
            <div className="flex items-center gap-1.5">
              <Calendar className="h-3.5 w-3.5 text-muted-foreground/80" />
              <span>Posted {formatJobDate(job.created_at)}</span>
            </div>
            {job.deadline && (
              <div className="flex items-center gap-1.5">
                <Clock className="h-3.5 w-3.5 text-muted-foreground/80" />
                <span>Deadline: {formatJobDate(job.deadline)}</span>
              </div>
            )}
          </div>

          {/* Project Description */}
          <div className="space-y-3">
            <h2 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
              Project Description & Requirements
            </h2>
            <div className="text-foreground/90 leading-relaxed text-base whitespace-pre-line">
              {job.description}
            </div>
          </div>

          {/* Required Skills */}
          <div className="space-y-3 border-t border-border pt-6">
            <h2 className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
              Required Skills
            </h2>
            <div className="flex flex-wrap gap-2">
              {job.required_skills && job.required_skills.length > 0 ? (
                job.required_skills.map((skill, index) => (
                  <Badge
                    key={`${skill}-${index}`}
                    variant="secondary"
                    className="font-mono text-xs"
                  >
                    {skill}
                  </Badge>
                ))
              ) : (
                <span className="font-mono text-xs text-muted-foreground italic">
                  No specific skills specified
                </span>
              )}
            </div>
          </div>
        </div>

        {/* Sidebar Metadata Card */}
        <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-6">
          {/* Budget */}
          <div>
            <div className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
              Budget / Compensation
            </div>
            <div className="mt-1 font-mono text-2xl font-bold text-foreground">{budgetDisplay}</div>
          </div>

          {/* Submission Deadline */}
          <div className="border-t border-border pt-4">
            <div className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
              Submission Deadline
            </div>
            <div className="mt-1 font-mono text-sm text-foreground">
              {formatJobDate(job.deadline)}
            </div>
          </div>

          {/* Posted Date */}
          <div className="border-t border-border pt-4">
            <div className="text-xs font-mono uppercase tracking-wider text-muted-foreground">
              Posted Date
            </div>
            <div className="mt-1 font-mono text-xs text-muted-foreground">
              {formatJobDate(job.created_at)}
            </div>
          </div>

          {/* Direct Apply CTA & Verification Gate */}
          <div className="border-t border-border pt-4 space-y-3">
            {isCreator ? (
              <div className="space-y-2">
                <Link href={`/jobs/${job.id}/applicants`} className="block w-full">
                  <Button variant="outline" className="w-full font-mono text-xs">
                    <Users className="h-3.5 w-3.5 mr-1.5" />
                    Review Proposals
                  </Button>
                </Link>
                <p className="text-[11px] text-muted-foreground text-center">
                  You created this posting.
                </p>
              </div>
            ) : alreadyApplied ? (
              <div className="space-y-2">
                <Button disabled variant="secondary" className="w-full font-mono text-xs">
                  <CheckCircle2 className="h-3.5 w-3.5 text-pastel-greenText mr-1.5" />
                  Proposal Submitted
                </Button>
                <p className="text-[11px] text-muted-foreground text-center">
                  Track in{" "}
                  <Link href="/activity" className="underline hover:text-foreground">
                    My Activity
                  </Link>
                </p>
              </div>
            ) : !isAuthenticated ? (
              <div className="space-y-2">
                <Link href="/login" className="block w-full">
                  <Button className="w-full font-mono text-xs">Sign In to Apply</Button>
                </Link>
                <p className="text-[11px] text-muted-foreground text-center">
                  Institutional sign-in required to submit proposals.
                </p>
              </div>
            ) : !isVerified ? (
              <div className="space-y-3">
                <Button disabled variant="secondary" className="w-full font-mono text-xs">
                  Verification Required
                </Button>
                <div className="rounded-lg border border-pastel-yellowText/20 bg-pastel-yellow p-3 text-xs text-pastel-yellowText space-y-1">
                  <div className="flex items-center gap-1.5 font-medium">
                    <ShieldAlert className="h-3.5 w-3.5 shrink-0" />
                    <span>Institutional Verification Required</span>
                  </div>
                  <p className="text-[11px] leading-relaxed">
                    Verify your university email (.edu) to apply for campus opportunities.
                  </p>
                  <Link
                    href="/verify-email"
                    className="font-medium underline hover:text-foreground inline-block pt-1"
                  >
                    Verify Institutional Email &rarr;
                  </Link>
                </div>
              </div>
            ) : job.status !== "open" ? (
              <div className="space-y-2">
                <Button disabled variant="secondary" className="w-full font-mono text-xs">
                  Opportunity Closed
                </Button>
                <p className="text-[11px] text-muted-foreground text-center">
                  This opportunity is not currently open for new proposals.
                </p>
              </div>
            ) : (
              <Button onClick={() => setApplyModalOpen(true)} className="w-full font-mono text-xs">
                Submit Proposal
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Application Proposal Dialog */}
      <Dialog open={applyModalOpen} onOpenChange={setApplyModalOpen}>
        <DialogContent onClose={() => setApplyModalOpen(false)} className="sm:max-w-lg">
          <DialogHeader>
            <DialogTitle>Submit Proposal</DialogTitle>
            <DialogDescription>
              Submit your proposal and approach for {job.title}.
            </DialogDescription>
          </DialogHeader>

          <form onSubmit={handleSubmitApplication} className="space-y-4">
            {submitError && (
              <div className="flex items-center gap-2 rounded-lg border border-pastel-redText/20 bg-pastel-red p-3 text-xs text-pastel-redText">
                <AlertCircle className="h-4 w-4 shrink-0" />
                <span>{submitError}</span>
              </div>
            )}

            <div className="space-y-2">
              <label
                htmlFor="cover-letter"
                className="block text-xs font-mono uppercase tracking-wider text-muted-foreground"
              >
                Proposal Cover Letter
              </label>
              <Textarea
                id="cover-letter"
                rows={5}
                value={coverLetter}
                onChange={(e) => setCoverLetter(e.target.value)}
                placeholder="Describe your relevant campus experience, approach to this deliverable, and timeline..."
                className="min-h-[140px] text-sm leading-relaxed"
              />
              <div className="flex justify-between text-xs font-mono text-muted-foreground">
                <span>Min 20 characters</span>
                <span
                  className={cn(
                    coverLetter.length >= 20
                      ? "text-foreground font-medium"
                      : "text-muted-foreground"
                  )}
                >
                  {coverLetter.length} chars
                </span>
              </div>
            </div>

            {/* Resume on file indicator */}
            <div className="rounded-lg border border-border bg-muted/40 p-3.5 text-xs text-muted-foreground">
              {memberProfile?.resume_key ? (
                <div className="flex items-center justify-between gap-2">
                  <div className="flex items-center gap-2 text-foreground font-mono">
                    <FileText className="h-4 w-4 text-foreground shrink-0" />
                    <span className="truncate">
                      Resume: {memberProfile.resume_filename || "resume.pdf"}
                    </span>
                  </div>
                  <Badge variant="outline" className="text-[10px] font-mono shrink-0">
                    On File
                  </Badge>
                </div>
              ) : (
                <div className="space-y-1">
                  <p className="text-foreground">No resume uploaded to your profile yet.</p>
                  <p className="text-[11px]">
                    You can still submit, or{" "}
                    <Link href="/profile" className="text-foreground underline font-medium">
                      upload a resume in your profile &rarr;
                    </Link>
                  </p>
                </div>
              )}
            </div>

            <DialogFooter className="mt-6">
              <Button
                type="button"
                variant="outline"
                onClick={() => setApplyModalOpen(false)}
                disabled={isApplying}
                className="font-mono text-xs"
              >
                Cancel
              </Button>
              <Button
                type="submit"
                disabled={isApplying || coverLetter.trim().length < 20}
                className="font-mono text-xs"
              >
                {isApplying ? (
                  <span>Submitting...</span>
                ) : (
                  <>
                    <Send className="h-3.5 w-3.5 mr-1.5" />
                    <span>Submit Proposal</span>
                  </>
                )}
              </Button>
            </DialogFooter>
          </form>
        </DialogContent>
      </Dialog>
    </div>
  );
}
