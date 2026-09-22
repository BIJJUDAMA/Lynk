"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import Link from "next/link";
import {
  Briefcase,
  Plus,
  X,
  AlertCircle,
  ArrowLeft,
  Eye,
  Edit3,
  Loader2,
  ShieldAlert,
  Lock,
} from "lucide-react";
import { CreateJobRequest } from "@/types/api";
import { createJob, ApiClientError } from "@/lib/api";
import { useAuth } from "@/components/auth/AuthProvider";
import { COMMON_DEPARTMENTS, POPULAR_SKILLS, formatJobDate } from "@/lib/formatters";
import { cn } from "@/lib/utils";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";

const OTHER_DEPARTMENT_VALUE = "__OTHER__";

export default function CreateJobPage() {
  const router = useRouter();
  const { user, isAuthenticated, isVerified, isLoading: authLoading, login } = useAuth();

  // Form fields
  const [title, setTitle] = useState("");
  const [departmentSelection, setDepartmentSelection] = useState("");
  const [customDepartment, setCustomDepartment] = useState("");
  const [description, setDescription] = useState("");
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

    const exists = skills.some((s) => s.toLowerCase() === target.toLowerCase());
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
      errors.department = "Academic department is required.";
    }

    const trimmedDesc = description.trim();
    if (!trimmedDesc) {
      errors.description = "Deliverable description is required.";
    } else if (trimmedDesc.length < 20) {
      errors.description = "Deliverable description must be at least 20 characters.";
    } else if (trimmedDesc.length > 5000) {
      errors.description = "Deliverable description must not exceed 5000 characters.";
    }

    if (skills.length === 0) {
      errors.skills = "Add at least one required skill tag.";
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

    const trimmedBudget = budget.trim();
    let finalDescription = description.trim();
    if (trimmedBudget) {
      const formattedBudget = trimmedBudget.startsWith("$") ? trimmedBudget : `$${trimmedBudget}`;
      if (!finalDescription.toLowerCase().includes("compensation:")) {
        finalDescription = `${finalDescription}\n\nCompensation: ${formattedBudget}`;
      }
    }

    const payload: CreateJobRequest = {
      title: title.trim(),
      department: resolvedDept,
      description: finalDescription,
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
          <h1 className="mt-4 font-serif text-2xl font-normal tracking-tight text-foreground">
            Campus Sign In Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            To publish opportunities, hire peers, or review deliverables, please sign in with your
            campus account.
          </p>
          <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-center">
            <button
              onClick={() => login({ redirectPath: "/jobs/create" })}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98]"
            >
              <Briefcase className="h-4 w-4" />
              <span>Sign In with Campus Account</span>
            </button>
            <Link
              href="/jobs"
              className="inline-flex items-center justify-center gap-2 rounded-md border border-border bg-card px-4 py-2.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors"
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
        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-yellow text-pastel-yellowText">
            <ShieldAlert className="h-6 w-6" />
          </div>
          <h1 className="mt-4 font-serif text-2xl font-normal tracking-tight text-foreground">
            Institutional Email Verification Required
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            You are signed in as <span className="font-mono text-foreground">{user?.email}</span>,
            but your university email has not been confirmed yet. Please verify your institutional
            email to publish campus gigs.
          </p>
          <div className="mt-6 flex justify-center gap-3">
            <Link
              href="/jobs"
              className="inline-flex items-center gap-2 rounded-md border border-border bg-card px-4 py-2 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors"
            >
              Browse Existing Jobs
            </Link>
          </div>
        </div>
      </div>
    );
  }

  const resolvedDept =
    departmentSelection === OTHER_DEPARTMENT_VALUE
      ? customDepartment.trim() || "Unspecified Department"
      : departmentSelection || "Unspecified Department";

  const displayBudget = budget.trim()
    ? budget.trim().startsWith("$")
      ? budget.trim()
      : `$${budget.trim()}`
    : "Negotiable / Departmental";

  return (
    <div className="mx-auto max-w-2xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Top Navigation & Mode Switcher */}
      <div className="flex items-center justify-between">
        <Link
          href="/jobs"
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to Opportunities</span>
        </Link>

        {/* Form / Preview Mode Switcher */}
        <div className="flex items-center gap-1 rounded-lg border border-border bg-card p-1">
          <button
            type="button"
            onClick={() => setIsPreviewMode(false)}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition-colors",
              !isPreviewMode
                ? "bg-muted text-foreground"
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
              "inline-flex items-center gap-1.5 rounded-md px-3 py-1 text-xs font-medium transition-colors",
              isPreviewMode
                ? "bg-muted text-foreground"
                : "text-muted-foreground hover:text-foreground"
            )}
          >
            <Eye className="h-3.5 w-3.5" />
            <span>Preview</span>
          </button>
        </div>
      </div>

      {/* Editorial Heading */}
      <div className="mt-6">
        <h1 className="font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
          Post a Campus Gig
        </h1>
        <p className="mt-2 text-sm text-muted-foreground">
          Define a clear project deliverable for verified university peers.
        </p>
      </div>

      {/* API Level Error Banner */}
      {apiError && (
        <div className="mt-6 flex items-start gap-3 rounded-xl border border-pastel-redText/20 bg-pastel-red p-4 text-xs text-pastel-redText">
          <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
          <div>
            <p className="font-semibold">Unable to publish project</p>
            <p className="mt-0.5">{apiError}</p>
          </div>
        </div>
      )}

      {isPreviewMode ? (
        /* Preview Card */
        <div className="mt-8 rounded-xl border border-border bg-card p-8 shadow-none max-w-2xl mx-auto space-y-6">
          <div className="flex items-center justify-between gap-2 border-b border-border pb-4">
            <div className="flex items-center gap-2">
              <Badge variant="default" className="font-mono text-xs">
                Open
              </Badge>
              <span className="font-mono text-xs text-muted-foreground">{resolvedDept}</span>
            </div>
            <span className="font-mono text-xs font-medium text-foreground">{displayBudget}</span>
          </div>

          <div>
            <h2 className="font-serif text-2xl font-normal tracking-tight text-foreground">
              {title.trim() || "Untitled Opportunity"}
            </h2>
            <div className="mt-2 flex flex-wrap items-center gap-4 text-xs text-muted-foreground font-mono">
              <span>Deadline: {formatJobDate(deadline)}</span>
              <span>-</span>
              <span>Posted by: {user?.email || "Campus Member"}</span>
            </div>
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
              Deliverable Scope &amp; Requirements
            </h3>
            <p className="mt-2 whitespace-pre-wrap text-sm leading-relaxed text-foreground">
              {description.trim() || "No description provided yet."}
            </p>
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="font-mono text-xs uppercase tracking-wider text-muted-foreground">
              Required Skills
            </h3>
            <div className="mt-2 flex flex-wrap gap-1.5">
              {skills.length > 0 ? (
                skills.map((skill) => (
                  <Badge key={skill} variant="secondary" className="font-mono text-xs">
                    {skill}
                  </Badge>
                ))
              ) : (
                <span className="font-mono text-xs text-muted-foreground italic">
                  No skills specified
                </span>
              )}
            </div>
          </div>

          <div className="flex items-center justify-end gap-3 border-t border-border pt-6">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setIsPreviewMode(false)}
              className="text-xs"
            >
              Back to Editor
            </Button>
            <button
              type="button"
              disabled={isSubmitting}
              onClick={handleSubmit}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2 text-xs font-medium text-white shadow-none hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Publishing...</span>
                </>
              ) : (
                <span>Publish Project</span>
              )}
            </button>
          </div>
        </div>
      ) : (
        /* Edit Form */
        <form
          onSubmit={handleSubmit}
          className="mt-8 rounded-xl border border-border bg-card p-8 shadow-none max-w-2xl mx-auto space-y-8"
        >
          {/* Section 1: Title input & Academic department selector */}
          <div className="space-y-4">
            <div>
              <h2 className="text-sm font-semibold tracking-tight text-foreground">
                1. Deliverable Title &amp; Department
              </h2>
              <p className="text-xs text-muted-foreground">
                Specify a concise project title and academic category.
              </p>
            </div>

            {/* Title */}
            <div>
              <div className="flex items-center justify-between">
                <label className="block text-xs font-medium text-foreground">
                  Project Title <span className="text-pastel-redText">*</span>
                </label>
                <span className="font-mono text-[11px] text-muted-foreground">
                  {title.length}/200
                </span>
              </div>
              <Input
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
                placeholder="e.g. Next.js Developer for Psychology Lab Experiment"
                className="mt-1.5"
              />
              {formErrors.title && (
                <p className="mt-1 font-mono text-xs text-pastel-redText">{formErrors.title}</p>
              )}
            </div>

            {/* Academic Department */}
            <div>
              <label className="block text-xs font-medium text-foreground">
                Academic Department <span className="text-pastel-redText">*</span>
              </label>
              <select
                value={departmentSelection}
                onChange={(e) => {
                  setDepartmentSelection(e.target.value);
                  if (formErrors.department) {
                    setFormErrors((prev) => {
                      const next = { ...prev };
                      delete next.department;
                      return next;
                    });
                  }
                }}
                className="mt-1.5 flex h-10 w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-foreground transition-colors"
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
                <Input
                  type="text"
                  value={customDepartment}
                  onChange={(e) => {
                    setCustomDepartment(e.target.value);
                    if (formErrors.department) {
                      setFormErrors((prev) => {
                        const next = { ...prev };
                        delete next.department;
                        return next;
                      });
                    }
                  }}
                  placeholder="Enter custom department name"
                  className="mt-2"
                />
              )}
              {formErrors.department && (
                <p className="mt-1 font-mono text-xs text-pastel-redText">
                  {formErrors.department}
                </p>
              )}
            </div>
          </div>

          {/* Section 2: Deliverable description textarea */}
          <div className="space-y-4 border-t border-border pt-6">
            <div>
              <h2 className="text-sm font-semibold tracking-tight text-foreground">
                2. Project Scope &amp; Requirements
              </h2>
              <p className="text-xs text-muted-foreground">
                Detail the deliverables, technical context, and timeline expectations.
              </p>
            </div>

            <div>
              <div className="flex items-center justify-between">
                <label className="block text-xs font-medium text-foreground">
                  Deliverable Description <span className="text-pastel-redText">*</span>
                </label>
                <span className="font-mono text-[11px] text-muted-foreground">
                  {description.length} / 5000 chars (min. 20)
                </span>
              </div>
              <Textarea
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
                placeholder="Outline specific milestones, deliverable expectations, estimated effort, and collaboration details..."
                className="mt-1.5 min-h-[160px]"
              />
              {formErrors.description && (
                <p className="mt-1 font-mono text-xs text-pastel-redText">
                  {formErrors.description}
                </p>
              )}
            </div>
          </div>

          {/* Section 3: Budget / Compensation & Deadline */}
          <div className="space-y-4 border-t border-border pt-6">
            <div>
              <h2 className="text-sm font-semibold tracking-tight text-foreground">
                3. Compensation &amp; Timeline
              </h2>
              <p className="text-xs text-muted-foreground">
                Set estimated compensation guidance and project deadline.
              </p>
            </div>

            <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
              {/* Budget */}
              <div>
                <label className="block text-xs font-medium text-foreground">
                  Compensation / Budget (Optional)
                </label>
                <Input
                  type="text"
                  value={budget}
                  onChange={(e) => setBudget(e.target.value)}
                  placeholder="e.g. $45/hr or $650 Fixed"
                  className="mt-1.5 font-mono text-sm"
                />
                <p className="mt-1 font-mono text-[11px] text-muted-foreground">
                  Fixed deliverable amount or estimated hourly rate.
                </p>
              </div>

              {/* Deadline */}
              <div>
                <label className="block text-xs font-medium text-foreground">
                  Target Deadline (Optional)
                </label>
                <Input
                  type="date"
                  min={todayDateString}
                  value={deadline}
                  onChange={(e) => {
                    setDeadline(e.target.value);
                    if (formErrors.deadline) {
                      setFormErrors((prev) => {
                        const next = { ...prev };
                        delete next.deadline;
                        return next;
                      });
                    }
                  }}
                  className="mt-1.5 font-mono text-sm"
                />
                {formErrors.deadline ? (
                  <p className="mt-1 font-mono text-xs text-pastel-redText">
                    {formErrors.deadline}
                  </p>
                ) : (
                  <p className="mt-1 font-mono text-[11px] text-muted-foreground">
                    Estimated completion date for applicants.
                  </p>
                )}
              </div>
            </div>
          </div>

          {/* Section 4: Required skills tag selector */}
          <div className="space-y-4 border-t border-border pt-6">
            <div>
              <h2 className="text-sm font-semibold tracking-tight text-foreground">
                4. Required Skills
              </h2>
              <p className="text-xs text-muted-foreground">
                Tag prerequisites, frameworks, or tools needed for this gig.
              </p>
            </div>

            <div>
              <label className="block text-xs font-medium text-foreground">
                Skills &amp; Prerequisites <span className="text-pastel-redText">*</span>
              </label>
              <div className="mt-1.5 flex gap-2">
                <Input
                  type="text"
                  value={skillInput}
                  onChange={(e) => setSkillInput(e.target.value)}
                  onKeyDown={handleSkillKeyDown}
                  placeholder="e.g. React, Python, Data Analysis..."
                  className="flex-1"
                />
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  onClick={() => handleAddSkill()}
                  className="h-10 px-3 text-xs"
                >
                  <Plus className="h-3.5 w-3.5 mr-1" />
                  Add
                </Button>
              </div>

              {/* Selected Skills */}
              {skills.length > 0 && (
                <div className="mt-3 flex flex-wrap gap-1.5">
                  {skills.map((skill) => (
                    <Badge
                      key={skill}
                      variant="secondary"
                      className="font-mono text-xs inline-flex items-center gap-1.5 py-1 px-2.5"
                    >
                      <span>{skill}</span>
                      <button
                        type="button"
                        onClick={() => handleRemoveSkill(skill)}
                        className="text-muted-foreground hover:text-foreground transition-colors"
                        aria-label={`Remove ${skill}`}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
              )}

              {formErrors.skills && (
                <p className="mt-1 font-mono text-xs text-pastel-redText">{formErrors.skills}</p>
              )}

              {/* Popular Skill Suggestions */}
              <div className="mt-3 pt-3 border-t border-border">
                <span className="font-mono text-[11px] text-muted-foreground">
                  Popular Suggestions:
                </span>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {POPULAR_SKILLS.map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      disabled={skills.includes(preset)}
                      onClick={() => handleAddSkill(preset)}
                      className="rounded-md border border-border bg-card px-2 py-0.5 font-mono text-xs text-muted-foreground hover:border-foreground/30 hover:text-foreground transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      + {preset}
                    </button>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Submission Actions */}
          <div className="flex items-center justify-end gap-3 border-t border-border pt-6">
            <Link
              href="/jobs"
              className="inline-flex items-center justify-center rounded-md border border-border bg-card px-4 py-2.5 text-xs font-medium text-foreground hover:bg-muted/60 transition-colors shadow-none"
            >
              Cancel
            </Link>
            <button
              type="submit"
              disabled={isSubmitting}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white shadow-none hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Publishing...</span>
                </>
              ) : (
                <span>Publish Project</span>
              )}
            </button>
          </div>
        </form>
      )}
    </div>
  );
}
