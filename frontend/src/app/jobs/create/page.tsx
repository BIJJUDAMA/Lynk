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
  const { user, isAuthenticated, isLoading: authLoading, role, login } = useAuth();

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

    // Check duplicate (case-insensitive)
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
      errors.title = "Job title cannot exceed 200 characters.";
    }

    const resolvedDept =
      departmentSelection === OTHER_DEPARTMENT_VALUE
        ? customDepartment.trim()
        : departmentSelection.trim();

    if (!resolvedDept) {
      errors.department = "Please select or specify an academic department.";
    } else if (resolvedDept.length > 100) {
      errors.department = "Department name cannot exceed 100 characters.";
    }

    const trimmedDesc = description.trim();
    if (!trimmedDesc) {
      errors.description = "Project description is required.";
    } else if (trimmedDesc.length < 20) {
      errors.description =
        "Please provide more detail in the description (at least 20 characters).";
    }

    if (skills.length === 0) {
      errors.skills = "Please specify at least one required skill or technology.";
    }

    if (!budget.trim()) {
      errors.budget = "Please enter a budget amount.";
    } else {
      const numBudget = Number(budget);
      if (isNaN(numBudget) || numBudget < 0) {
        errors.budget = "Budget must be a non-negative number.";
      }
    }

    if (deadline) {
      const selected = new Date(deadline);
      const today = new Date();
      today.setHours(0, 0, 0, 0);
      if (selected < today) {
        errors.deadline = "Application deadline cannot be in the past.";
      }
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
      budget: Number(budget),
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
        <div className="h-6 w-36 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
        <div className="mt-6 space-y-4 rounded-[10px] border border-slate-200 bg-white p-8 dark:border-slate-800 dark:bg-slate-900">
          <div className="h-8 w-1/2 animate-pulse rounded bg-slate-200 dark:bg-slate-800" />
          <div className="h-4 w-3/4 animate-pulse rounded bg-slate-100 dark:bg-slate-800" />
          <div className="mt-8 space-y-6">
            <div className="h-12 w-full animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800" />
            <div className="h-12 w-full animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800" />
            <div className="h-32 w-full animate-pulse rounded-[10px] bg-slate-100 dark:bg-slate-800" />
          </div>
        </div>
      </div>
    );
  }

  // Not authenticated gate
  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-[10px] border border-slate-200 bg-white p-8 text-center shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500">
            <Lock className="h-7 w-7" />
          </div>
          <h1 className="mt-4 text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
            Employer Sign In Required
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            To post campus jobs and review student proposals, please sign in or create an employer account.
          </p>
          <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-center">
            <button
              onClick={() =>
                login({
                  redirectPath: "/jobs/create",
                  roleHint: "employer",
                })
              }
              className="inline-flex items-center justify-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-primary"
            >
              <Briefcase className="h-4 w-4" />
              <span>Sign In as Employer</span>
            </button>
            <Link
              href="/jobs"
              className="inline-flex items-center justify-center gap-2 rounded-[10px] border border-slate-200 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
            >
              Browse Jobs
            </Link>
          </div>
        </div>
      </div>
    );
  }

  // Authenticated but non-employer role gate
  if (role !== "employer") {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="rounded-[10px] border border-amber-200 bg-amber-50/60 p-8 text-center dark:border-amber-900/50 dark:bg-amber-950/30">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-amber-100 text-amber-700 dark:bg-amber-900/50 dark:text-amber-300">
            <AlertCircle className="h-7 w-7" />
          </div>
          <h1 className="mt-4 text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
            Employer Account Required
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-300">
            You are currently signed in as a student (<span className="font-semibold">{user?.email}</span>).
            Posting jobs is exclusively reserved for campus departments and verified employers.
          </p>
          <div className="mt-6 flex flex-col gap-3 sm:flex-row sm:justify-center">
            <Link
              href="/jobs"
              className="inline-flex items-center justify-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-2.5 text-sm font-semibold text-white shadow-sm transition hover:bg-primary"
            >
              <GraduationCap className="h-4 w-4" />
              <span>Explore Student Gigs</span>
            </Link>
            <button
              onClick={() =>
                login({
                  redirectPath: "/jobs/create",
                  roleHint: "employer",
                })
              }
              className="inline-flex items-center justify-center gap-2 rounded-[10px] border border-slate-300 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300"
            >
              Switch Account
            </button>
          </div>
        </div>
      </div>
    );
  }

  const resolvedDepartmentName =
    departmentSelection === OTHER_DEPARTMENT_VALUE
      ? customDepartment || "Custom Department"
      : departmentSelection || "Department";

  return (
    <div className="mx-auto max-w-4xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Navigation breadcrumb */}
      <div className="mb-6 flex items-center justify-between">
        <Link
          href="/employer/jobs"
          className="inline-flex items-center gap-1.5 text-xs font-semibold text-slate-500 transition hover:text-primary dark:text-slate-400 dark:hover:text-primary"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          <span>Back to My Postings</span>
        </Link>
        <span className="text-xs text-slate-400 dark:text-slate-500">
          Employer Workspace
        </span>
      </div>

      {/* Page Header */}
      <div className="rounded-[10px] border border-border bg-card p-6 shadow-sm sm:p-8">
        <div className="inline-flex items-center gap-2 rounded-[10px] border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-800 dark:border-emerald-800 dark:bg-emerald-950/40 dark:text-emerald-300">
          <Sparkles className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
          <span>Verified University Marketplace</span>
        </div>

        <h1 className="mt-3 text-2xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
          Post a Campus Job or Gig
        </h1>
        <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
          Hire verified university student freelancers and talent across academic departments.
          Applications are strictly reserved for students with verified institutional (.edu) credentials.
        </p>
      </div>

      {/* Global API Error Alert */}
      {apiError && (
        <div className="mt-6 flex items-start gap-3 rounded-[10px] border border-rose-200 bg-rose-50/80 p-4 text-xs text-rose-900 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-200">
          <AlertCircle className="h-5 w-5 shrink-0 text-rose-600 dark:text-rose-400" />
          <div>
            <h4 className="font-bold">Posting Failed</h4>
            <p className="mt-0.5">{apiError}</p>
          </div>
        </div>
      )}

      {/* Job Creation Form */}
      <form onSubmit={handleSubmit} className="mt-8 space-y-8">
        {/* Section 1: Basic Job Information */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
          <h2 className="text-base font-bold text-slate-900 dark:text-white">
            1. Role & Academic Department
          </h2>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            Define the job title and the university discipline this role aligns with.
          </p>

          <div className="mt-6 space-y-6">
            {/* Job Title */}
            <div>
              <div className="flex items-center justify-between">
                <label className="block text-xs font-bold text-slate-800 dark:text-slate-200">
                  Job Title <span className="text-rose-500">*</span>
                </label>
                <span
                  className={cn(
                    "text-[11px]",
                    title.length > 200
                      ? "font-bold text-rose-600"
                      : "text-slate-400"
                  )}
                >
                  {title.length}/200
                </span>
              </div>
              <input
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
                placeholder="e.g. Next.js Frontend Developer for Campus Portal"
                className={cn(
                  "mt-2 w-full rounded-[10px] border bg-slate-50/50 px-4 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:bg-white focus:outline-none focus:ring-2 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900",
                  formErrors.title
                    ? "border-rose-400 focus:border-rose-500 focus:ring-rose-500/20"
                    : "border-slate-200 focus:border-primary focus:ring-ring/20 dark:border-slate-700"
                )}
              />
              {formErrors.title && (
                <p className="mt-1.5 text-xs text-rose-600 dark:text-rose-400">
                  {formErrors.title}
                </p>
              )}
            </div>

            {/* Department Dropdown + Custom Field */}
            <div>
              <label className="block text-xs font-bold text-slate-800 dark:text-slate-200">
                Academic Department <span className="text-rose-500">*</span>
              </label>
              <div className="mt-2 grid grid-cols-1 gap-3 sm:grid-cols-2">
                <div>
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
                    className={cn(
                      "w-full rounded-[10px] border bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 focus:bg-white focus:outline-none focus:ring-2 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900",
                      formErrors.department && !departmentSelection
                        ? "border-rose-400 focus:border-rose-500 focus:ring-rose-500/20"
                        : "border-slate-200 focus:border-primary focus:ring-ring/20 dark:border-slate-700"
                    )}
                  >
                    <option value="">Select Academic Department...</option>
                    {COMMON_DEPARTMENTS.map((dept) => (
                      <option key={dept} value={dept}>
                        {dept}
                      </option>
                    ))}
                    <option value={OTHER_DEPARTMENT_VALUE}>
                      + Other / Custom Department
                    </option>
                  </select>
                </div>

                {departmentSelection === OTHER_DEPARTMENT_VALUE && (
                  <div>
                    <input
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
                      placeholder="Enter custom department name..."
                      className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 px-4 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:border-primary focus:bg-white focus:outline-none focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900"
                    />
                  </div>
                )}
              </div>
              {formErrors.department && (
                <p className="mt-1.5 text-xs text-rose-600 dark:text-rose-400">
                  {formErrors.department}
                </p>
              )}
            </div>
          </div>
        </div>

        {/* Section 2: Compensation & Timeline */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
          <h2 className="text-base font-bold text-slate-900 dark:text-white">
            2. Compensation & Timeline
          </h2>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            Set clear expectations for budget, payment model, and project deadline.
          </p>

          <div className="mt-6 grid grid-cols-1 gap-6 sm:grid-cols-3">
            {/* Pay Type Toggle */}
            <div>
              <label className="block text-xs font-bold text-slate-800 dark:text-slate-200">
                Payment Model <span className="text-rose-500">*</span>
              </label>
              <div className="mt-2 flex rounded-[10px] border border-slate-200 bg-slate-100/70 p-1 dark:border-slate-700 dark:bg-slate-800">
                <button
                  type="button"
                  onClick={() => setPayType("fixed")}
                  className={cn(
                    "flex-1 rounded-[10px] py-2 text-center text-xs font-semibold transition",
                    payType === "fixed"
                      ? "bg-white text-slate-900 shadow-sm dark:bg-slate-900 dark:text-white"
                      : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                  )}
                >
                  Fixed Budget
                </button>
                <button
                  type="button"
                  onClick={() => setPayType("hourly")}
                  className={cn(
                    "flex-1 rounded-[10px] py-2 text-center text-xs font-semibold transition",
                    payType === "hourly"
                      ? "bg-white text-slate-900 shadow-sm dark:bg-slate-900 dark:text-white"
                      : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                  )}
                >
                  Hourly Rate
                </button>
              </div>
            </div>

            {/* Budget Input */}
            <div>
              <label className="block text-xs font-bold text-slate-800 dark:text-slate-200">
                {payType === "fixed" ? "Project Budget ($)" : "Hourly Rate ($/hr)"}{" "}
                <span className="text-rose-500">*</span>
              </label>
              <div className="relative mt-2">
                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5 text-slate-400">
                  <DollarSign className="h-4 w-4" />
                </div>
                <input
                  type="number"
                  min="0"
                  step="1"
                  value={budget}
                  onChange={(e) => {
                    setBudget(e.target.value);
                    if (formErrors.budget) {
                      setFormErrors((prev) => {
                        const next = { ...prev };
                        delete next.budget;
                        return next;
                      });
                    }
                  }}
                  placeholder={payType === "fixed" ? "500" : "25"}
                  className={cn(
                    "w-full rounded-[10px] border bg-slate-50/50 py-2.5 pl-9 pr-4 text-sm text-slate-900 placeholder:text-slate-400 focus:bg-white focus:outline-none focus:ring-2 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900",
                    formErrors.budget
                      ? "border-rose-400 focus:border-rose-500 focus:ring-rose-500/20"
                      : "border-slate-200 focus:border-primary focus:ring-ring/20 dark:border-slate-700"
                  )}
                />
              </div>
              {formErrors.budget ? (
                <p className="mt-1.5 text-xs text-rose-600 dark:text-rose-400">
                  {formErrors.budget}
                </p>
              ) : (
                <p className="mt-1.5 text-[11px] text-slate-400">
                  {payType === "fixed"
                    ? "Total compensation for contract completion."
                    : "Hourly compensation rate in USD."}
                </p>
              )}
            </div>

            {/* Deadline Input */}
            <div>
              <label className="block text-xs font-bold text-slate-800 dark:text-slate-200">
                Application Deadline (Optional)
              </label>
              <div className="relative mt-2">
                <input
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
                  className={cn(
                    "w-full rounded-[10px] border bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 focus:bg-white focus:outline-none focus:ring-2 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900",
                    formErrors.deadline
                      ? "border-rose-400 focus:border-rose-500 focus:ring-rose-500/20"
                      : "border-slate-200 focus:border-primary focus:ring-ring/20 dark:border-slate-700"
                  )}
                />
              </div>
              {formErrors.deadline ? (
                <p className="mt-1.5 text-xs text-rose-600 dark:text-rose-400">
                  {formErrors.deadline}
                </p>
              ) : (
                <p className="mt-1.5 text-[11px] text-slate-400">
                  Leave empty for rolling applications.
                </p>
              )}
            </div>
          </div>
        </div>

        {/* Section 3: Required Skills */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
          <h2 className="text-base font-bold text-slate-900 dark:text-white">
            3. Required Skills & Competencies
          </h2>
          <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
            Specify technologies or abilities student freelancers should possess.
          </p>

          <div className="mt-6 space-y-4">
            {/* Input field + Add button */}
            <div className="flex gap-2">
              <input
                type="text"
                value={skillInput}
                onChange={(e) => setSkillInput(e.target.value)}
                onKeyDown={handleSkillKeyDown}
                placeholder="Type a skill and press Enter (e.g. React, Python, Figma)..."
                className="flex-1 rounded-[10px] border border-slate-200 bg-slate-50/50 px-4 py-2.5 text-sm text-slate-900 placeholder:text-slate-400 focus:border-primary focus:bg-white focus:outline-none focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900"
              />
              <button
                type="button"
                onClick={() => handleAddSkill()}
                className="inline-flex items-center gap-1.5 rounded-[10px] bg-slate-100 px-4 py-2.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
              >
                <Plus className="h-4 w-4" />
                <span>Add</span>
              </button>
            </div>

            {/* Selected Skills Chips */}
            {skills.length > 0 ? (
              <div className="flex flex-wrap gap-2 pt-1">
                {skills.map((skill) => (
                  <span
                    key={skill}
                    className="inline-flex items-center gap-1.5 rounded-[10px] border border-emerald-200 bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-800 dark:border-emerald-800/60 dark:bg-emerald-950/60 dark:text-emerald-300"
                  >
                    <span>{skill}</span>
                    <button
                      type="button"
                      onClick={() => handleRemoveSkill(skill)}
                      className="text-emerald-600 hover:text-emerald-800 dark:text-emerald-400 dark:hover:text-emerald-200"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </span>
                ))}
              </div>
            ) : (
              <p className="text-xs text-slate-400">
                No skills added yet. Add at least one required skill.
              </p>
            )}
            {formErrors.skills && (
              <p className="text-xs text-rose-600 dark:text-rose-400">
                {formErrors.skills}
              </p>
            )}

            {/* Popular Skills Presets */}
            <div className="border-t border-slate-100 pt-4 dark:border-slate-800">
              <span className="text-xs font-medium text-slate-500 dark:text-slate-400">
                Quick add popular skills:
              </span>
              <div className="mt-2 flex flex-wrap gap-1.5">
                {POPULAR_SKILLS.map((preset) => {
                  const isAdded = skills.some(
                    (s) => s.toLowerCase() === preset.toLowerCase()
                  );
                  return (
                    <button
                      key={preset}
                      type="button"
                      disabled={isAdded}
                      onClick={() => handleAddSkill(preset)}
                      className={cn(
                        "rounded-[10px] border px-2.5 py-1 text-[11px] font-medium transition",
                        isAdded
                          ? "cursor-default border-slate-200 bg-slate-100 text-slate-400 dark:border-slate-800 dark:bg-slate-800 dark:text-slate-600"
                          : "border-border bg-card text-muted-foreground hover:border-primary/50 hover:text-foreground"
                      )}
                    >
                      {isAdded ? `✓ ${preset}` : `+ ${preset}`}
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        </div>

        {/* Section 4: Project Description & Scope */}
        <div className="rounded-[10px] border border-slate-200 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8">
          <div className="flex flex-col justify-between gap-2 sm:flex-row sm:items-center">
            <div>
              <h2 className="text-base font-bold text-slate-900 dark:text-white">
                4. Project Description & Requirements
              </h2>
              <p className="mt-1 text-xs text-slate-500 dark:text-slate-400">
                Provide detailed scope, deliverables, and student eligibility.
              </p>
            </div>

            {/* Edit / Preview Toggle */}
            <div className="flex rounded-[10px] border border-slate-200 bg-slate-100/70 p-0.5 dark:border-slate-700 dark:bg-slate-800">
              <button
                type="button"
                onClick={() => setIsPreviewMode(false)}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-[10px] px-3 py-1.5 text-xs font-semibold transition",
                  !isPreviewMode
                    ? "bg-white text-slate-900 shadow-sm dark:bg-slate-900 dark:text-white"
                    : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                )}
              >
                <Edit3 className="h-3 w-3" />
                <span>Write</span>
              </button>
              <button
                type="button"
                onClick={() => setIsPreviewMode(true)}
                className={cn(
                  "inline-flex items-center gap-1.5 rounded-[10px] px-3 py-1.5 text-xs font-semibold transition",
                  isPreviewMode
                    ? "bg-white text-slate-900 shadow-sm dark:bg-slate-900 dark:text-white"
                    : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
                )}
              >
                <Eye className="h-3 w-3" />
                <span>Preview</span>
              </button>
            </div>
          </div>

          <div className="mt-6">
            {!isPreviewMode ? (
              <div>
                <div className="flex items-center justify-between pb-1.5">
                  <label className="block text-xs font-bold text-slate-800 dark:text-slate-200">
                    Detailed Scope & Deliverables <span className="text-rose-500">*</span>
                  </label>
                  <span
                    className={cn(
                      "text-[11px]",
                      description.trim().length >= 20
                        ? "text-slate-400"
                        : "font-semibold text-amber-600 dark:text-amber-400"
                    )}
                  >
                    {description.length} chars (min 20)
                  </span>
                </div>
                <textarea
                  rows={8}
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
                  placeholder="Describe project responsibilities, estimated hours, milestones, required experience, and desired deliverables..."
                  className={cn(
                    "w-full rounded-[10px] border bg-slate-50/50 p-4 text-sm leading-relaxed text-slate-900 placeholder:text-slate-400 focus:bg-white focus:outline-none focus:ring-2 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900",
                    formErrors.description
                      ? "border-rose-400 focus:border-rose-500 focus:ring-rose-500/20"
                      : "border-slate-200 focus:border-primary focus:ring-ring/20 dark:border-slate-700"
                  )}
                />
                {formErrors.description && (
                  <p className="mt-1.5 text-xs text-rose-600 dark:text-rose-400">
                    {formErrors.description}
                  </p>
                )}
              </div>
            ) : (
              <div className="rounded-[10px] border border-slate-200 bg-slate-50/50 p-6 dark:border-slate-800 dark:bg-slate-950/40">
                <div className="border-b border-slate-200 pb-4 dark:border-slate-800">
                  <span className="text-xs font-semibold text-primary dark:text-emerald-500">
                    {resolvedDepartmentName}
                  </span>
                  <h3 className="mt-1 text-xl font-bold text-slate-900 dark:text-white">
                    {title || "Untitled Job Posting"}
                  </h3>
                  <div className="mt-2 flex items-center gap-3 text-xs text-slate-500">
                    <span>
                      {budget ? formatBudget(Number(budget), payType) : "$0"}
                    </span>
                    {deadline && <span>• Deadline: {deadline}</span>}
                  </div>
                </div>
                <div className="mt-4 whitespace-pre-line text-sm leading-relaxed text-slate-700 dark:text-slate-300">
                  {description || "No description provided yet."}
                </div>
              </div>
            )}
          </div>
        </div>

        {/* Action Controls */}
        <div className="flex flex-col-reverse items-center justify-between gap-4 border-t border-slate-200 pt-6 sm:flex-row dark:border-slate-800">
          <Link
            href="/employer/jobs"
            className="w-full text-center text-xs font-semibold text-slate-600 hover:text-slate-900 sm:w-auto dark:text-slate-400 dark:hover:text-white"
          >
            Cancel and Discard
          </Link>

          <button
            type="submit"
            disabled={isSubmitting}
            className="inline-flex w-full items-center justify-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-8 py-3 text-sm font-semibold text-white shadow-md shadow-sm transition hover:bg-primary disabled:cursor-not-allowed disabled:opacity-60 sm:w-auto"
          >
            {isSubmitting ? (
              <>
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>Publishing Job...</span>
              </>
            ) : (
              <>
                <CheckCircle2 className="h-4 w-4" />
                <span>Publish Campus Job</span>
              </>
            )}
          </button>
        </div>
      </form>
    </div>
  );
}
