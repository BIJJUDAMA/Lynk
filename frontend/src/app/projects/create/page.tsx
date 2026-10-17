"use client";

import { useState, useMemo } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  ArrowLeft,
  FolderGit2,
  Globe,
  Link2,
  Lock,
  ShieldAlert,
  Loader2,
  AlertCircle,
  Check,
  Sparkles,
} from "lucide-react";
import type { ProjectVisibility, CreateProjectPayload } from "@/types/project";
import { createProject } from "@/lib/project-api";
import { ApiClientError } from "@/lib/api";
import { useAuth } from "@/components/auth/AuthProvider";
import { PROJECT_CATEGORIES, slugifyProjectTitle } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

const OTHER_CATEGORY_VALUE = "__OTHER__";

const AVAILABLE_CATEGORIES = PROJECT_CATEGORIES.filter((c) => c.value !== "");

interface VisibilityOptionConfig {
  value: ProjectVisibility;
  title: string;
  badge: string;
  icon: typeof Globe;
  description: string;
  recommendation: string;
}

const VISIBILITY_CONFIGS: VisibilityOptionConfig[] = [
  {
    value: "public",
    title: "Public",
    badge: "Discovery",
    icon: Globe,
    description: "Visible in campus search, discovery, and recommendations.",
    recommendation:
      "Recommended for open research labs, student clubs, and campus-wide collaborations.",
  },
  {
    value: "unlisted",
    title: "Unlisted",
    badge: "Direct Link",
    icon: Link2,
    description: "Accessible only via direct link. Not listed in public discovery.",
    recommendation:
      "Best for stealth ventures, classroom assignments, or selective campus recruitment.",
  },
  {
    value: "private",
    title: "Private",
    badge: "Restricted",
    icon: Lock,
    description: "Restricted exclusively to accepted team members and invitees.",
    recommendation:
      "Strict access control for confidential university IP, patent-pending research, or closed teams.",
  },
];

export default function CreateProjectPage() {
  const router = useRouter();
  const {
    user,
    isAuthenticated,
    isVerified,
    isLoading: authLoading,
    login,
    resendVerificationEmail,
  } = useAuth();

  // Form state
  const [title, setTitle] = useState("");
  const [categorySelection, setCategorySelection] = useState<string>("AI / Robotics");
  const [customCategory, setCustomCategory] = useState("");
  const [description, setDescription] = useState("");
  const [visibility, setVisibility] = useState<ProjectVisibility>("public");

  // UI & submission state
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [apiError, setApiError] = useState<string | null>(null);
  const [isResending, setIsResending] = useState(false);
  const [resendSuccess, setResendSuccess] = useState(false);

  // Real-time URL slug preview
  const slugPreview = useMemo(() => slugifyProjectTitle(title), [title]);

  // Handle email verification resend
  const handleResendVerification = async () => {
    setIsResending(true);
    setResendSuccess(false);
    try {
      await resendVerificationEmail();
      setResendSuccess(true);
    } catch (err: unknown) {
      console.warn("Failed to resend verification email:", err);
    } finally {
      setIsResending(false);
    }
  };

  // Form validation
  const validateForm = (): boolean => {
    const errors: Record<string, string> = {};

    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      errors.title = "Project title is required.";
    } else if (trimmedTitle.length < 5) {
      errors.title = "Project title must be at least 5 characters.";
    } else if (trimmedTitle.length > 200) {
      errors.title = "Project title must not exceed 200 characters.";
    }

    const resolvedCategory =
      categorySelection === OTHER_CATEGORY_VALUE ? customCategory.trim() : categorySelection.trim();
    if (!resolvedCategory) {
      errors.category = "Please select or specify an academic track.";
    }

    const trimmedDesc = description.trim();
    if (!trimmedDesc) {
      errors.description = "Project description is required.";
    } else if (trimmedDesc.length < 20) {
      errors.description = "Project description must be at least 20 characters.";
    } else if (trimmedDesc.length > 5000) {
      errors.description = "Project description must not exceed 5000 characters.";
    }

    if (!["public", "unlisted", "private"].includes(visibility)) {
      errors.visibility = "Please select a valid visibility tier.";
    }

    setFormErrors(errors);
    return Object.keys(errors).length === 0;
  };

  // Submission handler
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setApiError(null);

    if (!validateForm()) {
      return;
    }

    setIsSubmitting(true);

    const resolvedCategory =
      categorySelection === OTHER_CATEGORY_VALUE ? customCategory.trim() : categorySelection.trim();

    const payload: CreateProjectPayload = {
      title: title.trim(),
      category: resolvedCategory,
      description: description.trim(),
      visibility,
    };

    try {
      const created = await createProject(payload);
      const destinationSlugOrId = created.slug || created.id;
      router.push(`/projects/${encodeURIComponent(destinationSlugOrId)}`);
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setApiError(err.message);
      } else if (err instanceof Error) {
        setApiError(err.message);
      } else {
        setApiError("Failed to launch campus workspace. Please try again.");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  // Auth Loading Gate
  if (authLoading) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="flex min-h-[40vh] items-center justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  // Unauthenticated Gate
  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            <Lock className="h-6 w-6" />
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal leading-snug text-foreground [text-wrap:balance]">
            Campus Sign In Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            To launch a collaborative campus workspace, organize research labs, or recruit student
            peers, please sign in with your verified university credentials.
          </p>
          <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-center">
            <button
              type="button"
              onClick={() => login({ redirectPath: "/projects/create" })}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98]"
            >
              <FolderGit2 className="h-4 w-4" />
              <span>Sign In with Campus Account</span>
            </button>
            <Link
              href="/projects"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-border bg-card px-4 py-2.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors"
            >
              Browse Campus Projects
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Unverified Institutional Email Gate
  if (!isVerified) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-yellow text-pastel-yellowText">
            <ShieldAlert className="h-6 w-6" />
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal leading-snug text-foreground [text-wrap:balance]">
            Institutional Email Verification Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            You are signed in as <span className="font-mono text-foreground">{user?.email}</span>,
            but your university email has not been verified yet. Only verified university students
            and researchers can create campus workspaces.
          </p>
          <div className="mt-6 flex flex-col sm:flex-row items-center justify-center gap-3">
            <button
              type="button"
              disabled={isResending}
              onClick={handleResendVerification}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:opacity-50"
            >
              {isResending ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Sending Link...</span>
                </>
              ) : (
                <span>Resend Verification Link</span>
              )}
            </button>
            <Link
              href="/projects"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-border bg-card px-4 py-2.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors"
            >
              Browse Existing Projects
            </Link>
          </div>
          {resendSuccess && (
            <p className="mt-3 font-mono text-xs text-emerald-600">
              Verification email sent! Please check your university inbox.
            </p>
          )}
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-3xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Top Navigation & Breadcrumb */}
      <div>
        <Link
          href="/projects"
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to Campus Workspaces</span>
        </Link>
      </div>

      {/* Editorial Header */}
      <div className="mt-4">
        <Badge variant="outline" className="font-mono text-xs mb-2">
          New Workspace
        </Badge>
        <h1 className="font-serif text-3xl sm:text-4xl font-normal leading-[1.2] text-foreground [text-wrap:balance]">
          Launch a Campus Project
        </h1>
        <p className="mt-2 text-sm text-muted-foreground max-w-2xl">
          Set up a collaborative workspace, research lab, or student venture with explicit
          visibility controls and open recruitment.
        </p>
      </div>

      {/* API Level Error Banner */}
      {apiError && (
        <div className="mt-6 flex items-start gap-3 rounded-xl border border-pastel-redText/20 bg-pastel-red p-4 text-xs text-pastel-redText">
          <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
          <div>
            <p className="font-semibold">Unable to launch project</p>
            <p className="mt-0.5">{apiError}</p>
          </div>
        </div>
      )}

      {/* Workspace Creation Form */}
      <form
        onSubmit={handleSubmit}
        className="mt-8 rounded-xl border border-border bg-card p-6 sm:p-8 shadow-none space-y-8"
      >
        {/* Section 1: Project Title & Slug Preview */}
        <div className="space-y-4">
          <div>
            <h2 className="text-sm font-semibold tracking-tight text-foreground">
              Project Title &amp; Workspace URL
            </h2>
            <p className="text-xs text-muted-foreground">
              Choose a distinctive name for your lab, startup, or collaborative initiative.
            </p>
          </div>

          <div>
            <div className="flex items-center justify-between">
              <label htmlFor="project-title" className="block text-xs font-medium text-foreground">
                Project Title <span className="text-pastel-redText">*</span>
              </label>
              <span className="font-mono text-[11px] text-muted-foreground">
                {title.length}/200
              </span>
            </div>
            <Input
              id="project-title"
              type="text"
              value={title}
              onChange={(e) => {
                setTitle(e.target.value);
                if (formErrors.title) {
                  setFormErrors((prev) => {
                    const next = { ...prev };
                    delete next.title;
                    return next;
                  });
                }
              }}
              maxLength={200}
              placeholder="e.g. Autonomous Campus Delivery Rover"
              className="mt-1.5"
            />
            {formErrors.title && (
              <p className="mt-1 font-mono text-xs text-pastel-redText">{formErrors.title}</p>
            )}
          </div>

          {/* Real-time URL Slug Preview */}
          <div className="rounded-lg border border-border/80 bg-muted/30 p-3.5 space-y-1.5">
            <div className="flex items-center justify-between text-xs text-muted-foreground">
              <span className="font-mono text-[11px] uppercase tracking-wider">
                Workspace URL Preview
              </span>
              <span className="font-mono text-[11px] text-muted-foreground">Auto-generated</span>
            </div>
            <div className="flex items-center gap-1 font-mono text-xs break-all">
              <span className="text-muted-foreground select-none">/projects/</span>
              <span
                className={cn(
                  "font-medium",
                  slugPreview ? "text-foreground font-semibold" : "text-muted-foreground/60 italic"
                )}
              >
                {slugPreview || "your-project-slug"}
              </span>
            </div>
          </div>
        </div>

        {/* Section 2: Academic Track / Category */}
        <div className="space-y-4 border-t border-border pt-6">
          <div>
            <h2 className="text-sm font-semibold tracking-tight text-foreground">
              Academic Track &amp; Category
            </h2>
            <p className="text-xs text-muted-foreground">
              Classify your project under a primary academic or technical discipline.
            </p>
          </div>

          <div>
            <label htmlFor="category-select" className="block text-xs font-medium text-foreground">
              Discipline Track <span className="text-pastel-redText">*</span>
            </label>
            <select
              id="category-select"
              value={categorySelection}
              onChange={(e) => {
                setCategorySelection(e.target.value);
                if (formErrors.category) {
                  setFormErrors((prev) => {
                    const next = { ...prev };
                    delete next.category;
                    return next;
                  });
                }
              }}
              className="mt-1.5 flex h-10 w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-foreground transition-colors"
            >
              {AVAILABLE_CATEGORIES.map((cat) => (
                <option key={cat.id} value={cat.value}>
                  {cat.label}
                </option>
              ))}
              <option value={OTHER_CATEGORY_VALUE}>Other (Custom Track)</option>
            </select>

            {/* Custom Category Input */}
            {categorySelection === OTHER_CATEGORY_VALUE && (
              <Input
                type="text"
                value={customCategory}
                onChange={(e) => {
                  setCustomCategory(e.target.value);
                  if (formErrors.category) {
                    setFormErrors((prev) => {
                      const next = { ...prev };
                      delete next.category;
                      return next;
                    });
                  }
                }}
                placeholder="Enter custom discipline or field of study"
                className="mt-2"
              />
            )}

            {formErrors.category && (
              <p className="mt-1 font-mono text-xs text-pastel-redText">{formErrors.category}</p>
            )}

            {/* Quick-select pills */}
            <div className="mt-3 flex flex-wrap gap-1.5">
              {AVAILABLE_CATEGORIES.map((cat) => (
                <button
                  key={cat.id}
                  type="button"
                  onClick={() => {
                    setCategorySelection(cat.value);
                    if (formErrors.category) {
                      setFormErrors((prev) => {
                        const next = { ...prev };
                        delete next.category;
                        return next;
                      });
                    }
                  }}
                  className={cn(
                    "rounded-md border px-2.5 py-1 text-xs font-mono transition-colors",
                    categorySelection === cat.value
                      ? "border-foreground bg-foreground text-background font-medium"
                      : "border-border bg-card text-muted-foreground hover:border-foreground/40 hover:text-foreground"
                  )}
                >
                  {cat.label}
                </button>
              ))}
            </div>
          </div>
        </div>

        {/* Section 3: Project Overview & Scope Description */}
        <div className="space-y-4 border-t border-border pt-6">
          <div>
            <h2 className="text-sm font-semibold tracking-tight text-foreground">
              Project Overview &amp; Mission
            </h2>
            <p className="text-xs text-muted-foreground">
              Detail the research objectives, technical architecture, and collaboration scope.
            </p>
          </div>

          <div>
            <div className="flex items-center justify-between">
              <label
                htmlFor="project-description"
                className="block text-xs font-medium text-foreground"
              >
                Description <span className="text-pastel-redText">*</span>
              </label>
              <span className="font-mono text-[11px] text-muted-foreground">
                {description.length} / 5000 chars (min. 20)
              </span>
            </div>
            <Textarea
              id="project-description"
              rows={6}
              maxLength={5000}
              value={description}
              onChange={(e) => {
                setDescription(e.target.value);
                if (formErrors.description) {
                  setFormErrors((prev) => {
                    const next = { ...prev };
                    delete next.description;
                    return next;
                  });
                }
              }}
              placeholder="Outline your project hypothesis, system architecture, milestones, or the problem your team is setting out to solve..."
              className="mt-1.5 min-h-[160px]"
            />
            {formErrors.description && (
              <p className="mt-1 font-mono text-xs text-pastel-redText">{formErrors.description}</p>
            )}
          </div>
        </div>

        {/* Section 4: Explicit Visibility Selector Cards */}
        <div className="space-y-4 border-t border-border pt-6">
          <div>
            <h2 className="text-sm font-semibold tracking-tight text-foreground">
              Workspace Visibility &amp; Access Control
            </h2>
            <p className="text-xs text-muted-foreground">
              Define how your workspace is discovered and accessed across campus.
            </p>
          </div>

          <div
            role="radiogroup"
            aria-label="Workspace Visibility"
            className="grid grid-cols-1 md:grid-cols-3 gap-4"
          >
            {VISIBILITY_CONFIGS.map((cfg) => {
              const isSelected = visibility === cfg.value;
              const IconComp = cfg.icon;

              return (
                <div
                  key={cfg.value}
                  role="radio"
                  aria-checked={isSelected}
                  tabIndex={0}
                  onClick={() => setVisibility(cfg.value)}
                  onKeyDown={(e) => {
                    if (e.key === " " || e.key === "Enter") {
                      e.preventDefault();
                      setVisibility(cfg.value);
                    }
                  }}
                  className={cn(
                    "relative flex flex-col justify-between rounded-xl border p-4 text-left transition cursor-pointer select-none",
                    isSelected
                      ? "border-foreground bg-muted/40 ring-1 ring-foreground shadow-sm"
                      : "border-border bg-card hover:border-foreground/30 hover:bg-muted/10"
                  )}
                >
                  <div>
                    {/* Header with Icon, Title, and Checkmark */}
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <div
                          className={cn(
                            "flex h-8 w-8 items-center justify-center rounded-lg border",
                            isSelected
                              ? "border-foreground bg-foreground text-background"
                              : "border-border bg-muted/60 text-muted-foreground"
                          )}
                        >
                          <IconComp className="h-4 w-4" />
                        </div>
                        <div>
                          <span className="font-medium text-sm text-foreground">{cfg.title}</span>
                        </div>
                      </div>

                      {/* Selected Indicator */}
                      <div
                        className={cn(
                          "flex h-5 w-5 items-center justify-center rounded-full border transition-colors",
                          isSelected
                            ? "border-foreground bg-foreground text-background"
                            : "border-border bg-background"
                        )}
                      >
                        {isSelected && <Check className="h-3 w-3 stroke-[3]" />}
                      </div>
                    </div>

                    {/* Badge */}
                    <div className="mt-3">
                      <span className="inline-flex items-center rounded-md border border-border bg-background/80 px-2 py-0.5 font-mono text-[10px] text-muted-foreground">
                        {cfg.badge}
                      </span>
                    </div>

                    {/* Exact Brief Description */}
                    <p className="mt-2 text-xs font-medium text-foreground leading-relaxed">
                      {cfg.description}
                    </p>
                  </div>

                  {/* Recommendation footer note */}
                  <div className="mt-3 border-t border-border/60 pt-2.5">
                    <p className="text-[11px] text-muted-foreground leading-tight">
                      {cfg.recommendation}
                    </p>
                  </div>
                </div>
              );
            })}
          </div>

          {formErrors.visibility && (
            <p className="mt-1 font-mono text-xs text-pastel-redText">{formErrors.visibility}</p>
          )}
        </div>

        {/* Action Buttons */}
        <div className="flex items-center justify-end gap-3 border-t border-border pt-6">
          <Link
            href="/projects"
            className="inline-flex items-center justify-center rounded-md border border-border bg-card px-4 py-2.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors shadow-none"
          >
            Cancel
          </Link>

          <Button
            type="submit"
            disabled={isSubmitting}
            className="inline-flex items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="h-3.5 w-3.5 animate-spin" />
                <span>Launching Workspace...</span>
              </>
            ) : (
              <>
                <Sparkles className="h-3.5 w-3.5" />
                <span>Launch Workspace</span>
              </>
            )}
          </Button>
        </div>
      </form>
    </div>
  );
}
