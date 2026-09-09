"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  Briefcase,
  DollarSign,
  Calendar,
  GraduationCap,
  Sparkles,
  Plus,
  X,
  AlertCircle,
  ArrowLeft,
  Eye,
  Edit3,
  Loader2,
  CheckCircle2,
  ShieldAlert,
  ShieldCheck,
  Lock,
} from "lucide-react";
import { JobPayType, CreateJobRequest } from "@/types/api";
import { createJob, ApiClientError } from "@/lib/api";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  COMMON_DEPARTMENTS,
  POPULAR_SKILLS,
  formatBudget,
} from "@/lib/formatters";
import { cn } from "@/lib/utils";

const OTHER_DEPARTMENT_VALUE = "__OTHER__";

export default function CreateJobPage() {
  const router = useRouter();
  const { user, isAuthenticated, isVerified, isLoading: authLoading, login } = useAuth();

  // Form fields
  const [title, setTitle] = useState("");
  const [departmentSelection, setDepartmentSelection] = useState("");
  const [customDepartment, setCustomDepartment] = useState("");
  const [description, setDescription] = useState("");
  const [payType, setPayType] = useState<JobPayType>("fixed");
  const [budget, setBudget] = useState("");
  const [deadline, setDeadline] = useState("");
  const [skills, setSkills] = useState<string[]>([]);
  const [skillInput, setSkillInput] = useState("");

  // UI state
  const [isPreviewMode, setIsPreviewMode] = useState(false);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [formErrors, setFormErrors] = useState<Record<string, string>>({});
  const [apiError, setApiError] = useState<string | null>(null);

  // Minimum allowed date for deadline: today
  const todayDateString = new Date().toISOString().split("T")[0];

  // Tag Management for skills
  const handleAddSkill = (skillToAdd?: string) => {
    const target = (skillToAdd ?? skillInput).trim();
    if (!target) return;

    const exists = skills.some(
      (s) => s.toLowerCase() === target.toLowerCase()
    );
    if (!exists) {
      setSkills((prev) => [...prev, target]);
      if (formErrors.skills) {
        setFormErrors((prev) => {
          const next = { ...prev };
          delete next.skills;
          return next;
        });
      }
    }
    setSkillInput("");
  };

  const handleRemoveSkill = (skillToRemove: string) => {
    setSkills((prev) => prev.filter((s) => s !== skillToRemove));
  };

  const handleSkillKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      e.preventDefault();
      handleAddSkill();
    }
  };

  // Form validation
  const validateForm = (): boolean => {
    const errors: Record<string, string> = {};

    const trimmedTitle = title.trim();
    if (!trimmedTitle) {
      errors.title = "Job title is required.";
    } else if (trimmedTitle.length < 5) {
      errors.title = "Job title must be at least 5 characters.";
    } else if (trimmedTitle.length > 200) {
      errors.title = "Job title must not exceed 200 characters.";
    }

    const dept =
      departmentSelection === OTHER_DEPARTMENT_VALUE
        ? customDepartment.trim()
        : departmentSelection.trim();
    if (!dept) {
      errors.department = "Department is required.";
    }

    const trimmedDesc = description.trim();
    if (!trimmedDesc) {
      errors.description = "Job description is required.";
    } else if (trimmedDesc.length < 20) {
      errors.description = "Job description must be at least 20 characters.";
    } else if (trimmedDesc.length > 5000) {
      errors.description = "Job description must not exceed 5000 characters.";
    }

    const budgetNum = Number(budget);
    if (budget === "" || isNaN(budgetNum)) {
      errors.budget = "Please enter a valid budget amount.";
    } else if (budgetNum < 0) {
      errors.budget = "Budget cannot be negative.";
    } else if (budgetNum > 100000) {
      errors.budget = "Budget exceeds maximum allowed ($100,000).";
    }

    if (skills.length === 0) {
      errors.skills = "Add at least one required skill or course tag.";
    }

    if (deadline && deadline < todayDateString) {
      errors.deadline = "Deadline date cannot be in the past.";
    }

    setFormErrors(errors);
    return Object.keys(errors).length === 0;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setApiError(null);

    if (!validateForm()) {
      return;
    }

    setIsSubmitting(true);

    const resolvedDept =
      departmentSelection === OTHER_DEPARTMENT_VALUE
        ? customDepartment.trim()
        : departmentSelection.trim();

    const payload: CreateJobRequest = {
      title: title.trim(),
      department: resolvedDept,
      description: description.trim(),
      pay_type: payType,
      budget_cents: Math.round(Number(budget) * 100),
      required_skills: skills,
      deadline: deadline ? deadline : undefined,
    };

    try {
      const created = await createJob(payload);
      router.push(`/jobs/${created.id}`);
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setApiError(err.message);
      } else if (err instanceof Error) {
        setApiError(err.message);
      } else {
        setApiError("Failed to post job. Please try again.");
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  // Loading state
  if (authLoading) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-12 sm:px-6 lg:px-8">
        <div className="flex min-h-[50vh] items-center justify-center">
          <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  // Unauthenticated Gate
  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-sm">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            <Lock className="h-6 w-6" />
          </div>
          <h1 className="mt-4 text-xl font-semibold tracking-tight text-foreground">
            Campus Sign In Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            To publish opportunities, hire peers, or review applications, please sign in with your campus account.
          </p>
          <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-center">
            <button
              onClick={() => login({ redirectPath: "/jobs/create" })}
              className="inline-flex items-center justify-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
            >
              <Briefcase className="h-4 w-4" />
              <span>Sign In with Campus Account</span>
            </button>
            <Link
              href="/jobs"
              className="inline-flex items-center justify-center gap-2 rounded-lg border border-border bg-background px-5 py-2.5 text-sm font-medium text-foreground hover:bg-muted"
            >
              Browse Jobs
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Unverified Campus Email Gate
  if (!isVerified) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-xl border border-amber-500/30 bg-amber-500/10 p-8 text-center">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-amber-500/20 text-amber-700 dark:text-amber-300">
            <ShieldAlert className="h-6 w-6" />
          </div>
          <h1 className="mt-4 text-xl font-semibold tracking-tight text-foreground">
            Institutional Email Verification Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            You are signed in as <span className="font-semibold text-foreground">{user?.email}</span>, but your university email has not been confirmed yet.
            Please verify your institutional email to publish campus gigs.
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <Link
              href="/jobs"
              className="inline-flex items-center gap-2 rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-muted"
            >
              Browse Existing Jobs
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const resolvedDeptPreview =
    departmentSelection === OTHER_DEPARTMENT_VALUE
      ? customDepartment.trim() || "Unspecified Department"
      : departmentSelection || "Unspecified Department";

  return (
    <div className="mx-auto max-w-4xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Back Navigation */}
      <div className="mb-6 flex items-center justify-between">
        <Link
          href="/jobs"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-muted-foreground transition hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to Opportunities</span>
        </Link>

        {/* Form / Preview Mode Switcher */}
        <div className="flex items-center gap-1 rounded-lg border border-border bg-muted/40 p-1">
          <button
            type="button"
            onClick={() => setIsPreviewMode(false)}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition",
              !isPreviewMode
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            <Edit3 className="h-3.5 w-3.5" />
            <span>Editor</span>
          </button>
          <button
            type="button"
            onClick={() => {
              validateForm();
              setIsPreviewMode(true);
            }}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition",
              isPreviewMode
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            <Eye className="h-3.5 w-3.5" />
            <span>Preview</span>
          </button>
        </div>
      </div>

      {/* Page Heading */}
      <div className="mb-8">
        <h1 className="text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
          Publish a Campus Opportunity
        </h1>
        <p className="mt-1 text-sm text-muted-foreground">
          Create a freelance task, research assistantship, or campus project. Any verified campus member can post and hire.
        </p>
      </div>

      {/* API Level Error Banner */}
      {apiError && (
        <div className="mb-6 flex items-center gap-3 rounded-lg border border-red-500/30 bg-red-500/10 p-4 text-sm text-red-700 dark:text-red-300">
          <AlertCircle className="h-5 w-5 shrink-0" />
          <div>
            <p className="font-semibold">Unable to publish opportunity</p>
            <p className="text-xs">{apiError}</p>
          </div>
        </div>
      )}

      {isPreviewMode ? (
        /* Preview View */
        <div className="space-y-6">
          <div className="rounded-xl border border-border bg-card p-6 shadow-sm sm:p-8">
            <div className="flex items-center gap-2">
              <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-0.5 text-xs font-semibold text-emerald-700 dark:text-emerald-300">
                <span className="h-1.5 w-1.5 rounded-full bg-emerald-500" />
                Preview Mode
              </span>
              <span className="rounded-md bg-muted px-2 py-0.5 text-xs font-medium text-muted-foreground">
                {resolvedDeptPreview}
              </span>
            </div>

            <h2 className="mt-3 text-xl font-bold text-foreground sm:text-2xl">
              {title.trim() || "Untitled Opportunity"}
            </h2>

            <div className="mt-3 flex flex-wrap items-center gap-4 text-xs text-muted-foreground">
              <span className="font-semibold text-emerald-600 dark:text-emerald-400">
                {budget ? formatBudget(Math.round(Number(budget) * 100), payType) : "$0.00"}
              </span>
              <span>•</span>
              <span>Deadline: {deadline || "Flexible"}</span>
              <span>•</span>
              <span>Posted by: {user?.name || user?.email}</span>
            </div>

            <div className="mt-6 border-t border-border pt-6">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Description
              </h3>
              <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-foreground">
                {description.trim() || "No description provided yet."}
              </p>
            </div>

            <div className="mt-6 border-t border-border pt-6">
              <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                Required Skills & Tags
              </h3>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {skills.length > 0 ? (
                  skills.map((skill) => (
                    <span
                      key={skill}
                      className="rounded-md border border-border bg-muted/60 px-2.5 py-1 text-xs font-medium text-foreground"
                    >
                      {skill}
                    </span>
                  ))
                ) : (
                  <span className="text-xs text-muted-foreground italic">
                    No skills specified
                  </span>
                )}
              </div>
            </div>
          </div>

          <div className="flex justify-end gap-3">
            <button
              type="button"
              onClick={() => setIsPreviewMode(false)}
              className="rounded-lg border border-border bg-background px-4 py-2 text-xs font-medium text-foreground hover:bg-muted"
            >
              Back to Edit
            </button>
            <button
              type="button"
              disabled={isSubmitting}
              onClick={handleSubmit}
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Publishing...</span>
                </>
              ) : (
                <span>Publish Opportunity</span>
              )}
            </button>
          </div>
        </div>
      ) : (
        /* Edit Form */
        <form onSubmit={handleSubmit} className="space-y-6">
          <div className="rounded-xl border border-border bg-card p-6 shadow-sm space-y-6">
            {/* Title */}
            <div>
              <label className="block text-xs font-medium text-foreground">
                Opportunity Title <span className="text-red-500">*</span>
              </label>
              <input
                type="text"
                value={title}
                onChange={(e) => setTitle(e.target.value)}
                placeholder="e.g. Next.js Developer for Psychology Lab Experiment"
                className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
              />
              {formErrors.title && (
                <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                  {formErrors.title}
                </p>
              )}
            </div>

            {/* Department */}
            <div>
              <label className="block text-xs font-medium text-foreground">
                Department / Discipline <span className="text-red-500">*</span>
              </label>
              <select
                value={departmentSelection}
                onChange={(e) => setDepartmentSelection(e.target.value)}
                className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
              >
                <option value="">Select a department...</option>
                {COMMON_DEPARTMENTS.map((dept) => (
                  <option key={dept} value={dept}>
                    {dept}
                  </option>
                ))}
                <option value={OTHER_DEPARTMENT_VALUE}>Other (Custom)</option>
              </select>

              {departmentSelection === OTHER_DEPARTMENT_VALUE && (
                <input
                  type="text"
                  value={customDepartment}
                  onChange={(e) => setCustomDepartment(e.target.value)}
                  placeholder="Enter department name"
                  className="mt-2 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                />
              )}
              {formErrors.department && (
                <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                  {formErrors.department}
                </p>
              )}
            </div>

            {/* Budget & Pay Type */}
            <div className="grid grid-cols-1 gap-4 sm:grid-cols-3">
              <div>
                <label className="block text-xs font-medium text-foreground">
                  Compensation Type
                </label>
                <select
                  value={payType}
                  onChange={(e) => setPayType(e.target.value as JobPayType)}
                  className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                >
                  <option value="fixed">Fixed Price</option>
                  <option value="hourly">Hourly Rate</option>
                </select>
              </div>

              <div>
                <label className="block text-xs font-medium text-foreground">
                  Budget (USD) <span className="text-red-500">*</span>
                </label>
                <div className="relative mt-1.5">
                  <span className="absolute inset-y-0 left-0 flex items-center pl-3 text-xs text-muted-foreground">
                    $
                  </span>
                  <input
                    type="number"
                    min="0"
                    step="1"
                    value={budget}
                    onChange={(e) => setBudget(e.target.value)}
                    placeholder={payType === "hourly" ? "25" : "500"}
                    className="w-full rounded-lg border border-border bg-background pl-7 pr-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                  />
                </div>
                {formErrors.budget && (
                  <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                    {formErrors.budget}
                  </p>
                )}
              </div>

              <div>
                <label className="block text-xs font-medium text-foreground">
                  Target Deadline (Optional)
                </label>
                <input
                  type="date"
                  min={todayDateString}
                  value={deadline}
                  onChange={(e) => setDeadline(e.target.value)}
                  className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                />
                {formErrors.deadline && (
                  <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                    {formErrors.deadline}
                  </p>
                )}
              </div>
            </div>

            {/* Description */}
            <div>
              <div className="flex items-center justify-between">
                <label className="block text-xs font-medium text-foreground">
                  Description & Scope <span className="text-red-500">*</span>
                </label>
                <span className="text-[11px] text-muted-foreground">
                  {description.length} / 5000 chars
                </span>
              </div>
              <textarea
                rows={6}
                maxLength={5000}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
                placeholder="Outline project deliverables, requirements, timelines, and collaboration expectations..."
                className="mt-1.5 w-full rounded-lg border border-border bg-background p-3 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
              />
              {formErrors.description && (
                <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                  {formErrors.description}
                </p>
              )}
            </div>

            {/* Required Skills */}
            <div>
              <label className="block text-xs font-medium text-foreground">
                Required Skills & Tags <span className="text-red-500">*</span>
              </label>
              <div className="mt-1.5 flex gap-2">
                <input
                  type="text"
                  value={skillInput}
                  onChange={(e) => setSkillInput(e.target.value)}
                  onKeyDown={handleSkillKeyDown}
                  placeholder="e.g. React, Python, Data Analysis..."
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                />
                <button
                  type="button"
                  onClick={() => handleAddSkill()}
                  className="inline-flex items-center gap-1 rounded-lg bg-secondary px-3 py-2 text-xs font-medium text-secondary-foreground hover:bg-secondary/80"
                >
                  <Plus className="h-3.5 w-3.5" />
                  Add
                </button>
              </div>

              {/* Added Skills */}
              <div className="mt-3 flex flex-wrap gap-1.5">
                {skills.map((skill) => (
                  <span
                    key={skill}
                    className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/60 px-2.5 py-1 text-xs font-medium text-foreground"
                  >
                    <span>{skill}</span>
                    <button
                      type="button"
                      onClick={() => handleRemoveSkill(skill)}
                      className="text-muted-foreground hover:text-foreground"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </span>
                ))}
              </div>

              {formErrors.skills && (
                <p className="mt-1 text-xs text-red-600 dark:text-red-400">
                  {formErrors.skills}
                </p>
              )}

              {/* Suggestions */}
              <div className="mt-3 pt-3 border-t border-border/50">
                <span className="text-[11px] font-medium text-muted-foreground">Suggested Skills:</span>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {POPULAR_SKILLS.slice(0, 8).map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      disabled={skills.includes(preset)}
                      onClick={() => handleAddSkill(preset)}
                      className="rounded-md border border-border/60 bg-background px-2 py-0.5 text-[11px] text-muted-foreground hover:border-border hover:text-foreground disabled:opacity-40"
                    >
                      + {preset}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Submit Actions */}
          <div className="flex justify-end gap-3">
            <Link
              href="/jobs"
              className="rounded-lg border border-border bg-background px-4 py-2.5 text-xs font-medium text-foreground hover:bg-muted"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={isSubmitting}
              className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-xs font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Publishing...</span>
                </>
              ) : (
                <span>Publish Opportunity</span>
              )}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
