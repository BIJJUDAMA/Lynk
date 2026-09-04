"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import {
  User as UserIcon,
  ShieldCheck,
  ShieldAlert,
  GraduationCap,
  Briefcase,
  Globe,
  Plus,
  X,
  Save,
  CheckCircle2,
  AlertCircle,
  Loader2,
  Sparkles,
  ExternalLink,
  Mail,
  Calendar,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import {
  getStudentProfile,
  updateStudentProfile,
  getEmployerProfile,
  updateEmployerProfile,
  ApiClientError,
} from "@/lib/api";
import { useQuery, useMutation } from "@/lib/useApi";
import {
  COMMON_DEPARTMENTS,
  POPULAR_SKILLS,
  isValidUrl,
} from "@/lib/formatters";
import { ResumeUploader } from "@/components/profile/ResumeUploader";
import type {
  UpdateStudentProfileRequest,
  UpdateEmployerProfileRequest,
} from "@/types/api";

const CURRENT_YEAR = new Date().getFullYear();
const GRADUATION_YEARS = Array.from({ length: 9 }, (_, i) => CURRENT_YEAR - 2 + i);

export default function ProfilePage() {
  const {
    user,
    role: authRole,
    isVerified,
    isAuthenticated,
    isLoading: isAuthLoading,
    login,
  } = useAuth();

  // Active tab role: defaults to user's assigned role, or "student"
  const [activeRole, setActiveRole] = useState<"student" | "employer">("student");

  useEffect(() => {
    if (authRole === "employer") {
      setActiveRole("employer");
    } else if (authRole === "student") {
      setActiveRole("student");
    }
  }, [authRole]);

  // Queries for existing profile data
  const {
    data: studentProfile,
    isLoading: isStudentLoading,
    refetch: refetchStudentProfile,
  } = useQuery(getStudentProfile, {
    enabled: isAuthenticated && activeRole === "student",
  });

  const {
    data: employerProfile,
    isLoading: isEmployerLoading,
    refetch: refetchEmployerProfile,
  } = useQuery(getEmployerProfile, {
    enabled: isAuthenticated && activeRole === "employer",
  });

  // Student Form State
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [department, setDepartment] = useState(COMMON_DEPARTMENTS[0]);
  const [graduationYear, setGraduationYear] = useState(CURRENT_YEAR + 2);
  const [bio, setBio] = useState("");
  const [skills, setSkills] = useState<string[]>([]);
  const [newSkillInput, setNewSkillInput] = useState("");
  const [portfolioLinks, setPortfolioLinks] = useState<string[]>([]);
  const [newLinkInput, setNewLinkInput] = useState("");
  const [linkError, setLinkError] = useState<string | null>(null);

  // Employer Form State
  const [companyOrOrg, setCompanyOrOrg] = useState("");
  const [contactName, setContactName] = useState("");
  const [employerDesc, setEmployerDesc] = useState("");
  const [website, setWebsite] = useState("");

  // Feedback notifications
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Populate Student Form
  useEffect(() => {
    if (studentProfile) {
      setFirstName(studentProfile.first_name || "");
      setLastName(studentProfile.last_name || "");
      setDepartment(studentProfile.department || COMMON_DEPARTMENTS[0]);
      setGraduationYear(studentProfile.graduation_year || CURRENT_YEAR + 2);
      setBio(studentProfile.bio || "");
      setSkills(studentProfile.skills || []);
      setPortfolioLinks(studentProfile.portfolio_links || []);
    }
  }, [studentProfile]);

  // Populate Employer Form
  useEffect(() => {
    if (employerProfile) {
      setCompanyOrOrg(employerProfile.company_or_org || "");
      setContactName(employerProfile.contact_name || "");
      setEmployerDesc(employerProfile.description || "");
      setWebsite(employerProfile.website || "");
    }
  }, [employerProfile]);

  // Mutations
  const { mutate: mutateStudentProfile, isLoading: isSavingStudent } =
    useMutation<UpdateStudentProfileRequest, unknown>(
      (data, client) => updateStudentProfile(data, client),
      {
        onSuccess: () => {
          setSuccessMessage("Student profile updated successfully!");
          setErrorMessage(null);
          refetchStudentProfile();
          setTimeout(() => setSuccessMessage(null), 4000);
        },
        onError: (err) => {
          setErrorMessage(err.message || "Failed to update student profile.");
          setSuccessMessage(null);
        },
      }
    );

  const { mutate: mutateEmployerProfile, isLoading: isSavingEmployer } =
    useMutation<UpdateEmployerProfileRequest, unknown>(
      (data, client) => updateEmployerProfile(data, client),
      {
        onSuccess: () => {
          setSuccessMessage("Employer profile updated successfully!");
          setErrorMessage(null);
          refetchEmployerProfile();
          setTimeout(() => setSuccessMessage(null), 4000);
        },
        onError: (err) => {
          setErrorMessage(err.message || "Failed to update employer profile.");
          setSuccessMessage(null);
        },
      }
    );

  // Skill tag management
  const handleAddSkill = (skillToAdd: string) => {
    const trimmed = skillToAdd.trim();
    if (!trimmed) return;
    if (skills.some((s) => s.toLowerCase() === trimmed.toLowerCase())) {
      setNewSkillInput("");
      return;
    }
    setSkills([...skills, trimmed]);
    setNewSkillInput("");
  };

  const handleRemoveSkill = (skillToRemove: string) => {
    setSkills(skills.filter((s) => s !== skillToRemove));
  };

  // Portfolio link management
  const handleAddLink = () => {
    const trimmed = newLinkInput.trim();
    if (!trimmed) return;

    if (!isValidUrl(trimmed)) {
      setLinkError("URL must start with http:// or https:// (e.g. https://github.com/my-work)");
      return;
    }

    if (portfolioLinks.includes(trimmed)) {
      setLinkError("This link is already added.");
      return;
    }

    setPortfolioLinks([...portfolioLinks, trimmed]);
    setNewLinkInput("");
    setLinkError(null);
  };

  const handleRemoveLink = (linkToRemove: string) => {
    setPortfolioLinks(portfolioLinks.filter((link) => link !== linkToRemove));
  };

  // Form Submissions
  const handleSaveStudent = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    setSuccessMessage(null);

    if (!firstName.trim() || !lastName.trim()) {
      setErrorMessage("First name and last name are required.");
      return;
    }

    await mutateStudentProfile({
      first_name: firstName.trim(),
      last_name: lastName.trim(),
      department,
      graduation_year: Number(graduationYear),
      bio: bio.trim(),
      skills,
      portfolio_links: portfolioLinks,
    });
  };

  const handleSaveEmployer = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);
    setSuccessMessage(null);

    if (!companyOrOrg.trim() || !contactName.trim()) {
      setErrorMessage("Company/Organization and contact name are required.");
      return;
    }

    await mutateEmployerProfile({
      company_or_org: companyOrOrg.trim(),
      contact_name: contactName.trim(),
      description: employerDesc.trim(),
      website: website.trim(),
    });
  };

  // Auth & Loading Guard
  if (isAuthLoading) {
    return (
      <div className="mx-auto max-w-5xl px-4 py-12 sm:px-6 lg:px-8 animate-pulse space-y-8">
        <div className="h-10 w-64 rounded-[10px] bg-slate-200 dark:bg-slate-800" />
        <div className="h-48 rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
        <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
          <div className="md:col-span-2 h-96 rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
          <div className="h-96 rounded-[10px] bg-slate-100 dark:bg-slate-800/60" />
        </div>
      </div>
    );
  }

  if (!isAuthenticated) {
    return (
      <div className="mx-auto max-w-3xl px-4 py-16 text-center sm:px-6 lg:px-8">
        <div className="rounded-[10px] border border-slate-200 bg-white p-10 shadow-sm dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-[10px] bg-emerald-50 dark:bg-emerald-950/50 text-primary dark:bg-emerald-950/60 dark:text-emerald-500 mb-6">
            <UserIcon className="h-8 w-8" />
          </div>
          <h2 className="text-2xl font-bold text-slate-900 dark:text-white">
            Sign In Required
          </h2>
          <p className="mx-auto mt-3 max-w-md text-sm text-slate-600 dark:text-slate-400">
            Please sign in with your verified university credentials to manage your student profile, upload your resume, or hire campus talent.
          </p>
          <div className="mt-8">
            <button
              onClick={() => login({ redirectPath: "/profile" })}
              className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-sm transition hover:bg-primary"
            >
              <span>Sign In with University Account</span>
            </button>
          </div>
        </div>
      </div>
    );
  }

  const isProfileLoading =
    activeRole === "student" ? isStudentLoading : isEmployerLoading;

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Header Profile Identity Section */}
      <div className="rounded-[10px] border border-slate-200/80 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8 mb-8">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-6">
          <div className="flex items-start gap-4">
            <div className="flex h-16 w-16 shrink-0 items-center justify-center rounded-[10px] bg-primary text-primary-foreground text-white text-xl font-bold shadow-md shadow-sm">
              {user?.name ? user.name.slice(0, 2).toUpperCase() : <UserIcon className="h-8 w-8" />}
            </div>
            <div>
              <div className="flex flex-wrap items-center gap-2">
                <h1 className="text-2xl font-bold text-slate-900 dark:text-white">
                  {user?.name || "Campus Member"}
                </h1>
                {/* Institutional Email Verification Status Badge */}
                {isVerified ? (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-emerald-50 px-3 py-1 text-xs font-semibold text-emerald-700 dark:bg-emerald-950/60 dark:text-emerald-300 border border-emerald-200 dark:border-emerald-800/60">
                    <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                    Verified .edu Student
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-amber-50 px-3 py-1 text-xs font-semibold text-amber-700 dark:bg-amber-950/60 dark:text-amber-300 border border-amber-200 dark:border-amber-800/60">
                    <ShieldAlert className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                    Unverified .edu Email
                  </span>
                )}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-4 text-xs text-slate-500 dark:text-slate-400">
                <span className="flex items-center gap-1">
                  <Mail className="h-3.5 w-3.5" />
                  {user?.email}
                </span>
                <span className="capitalize font-medium text-slate-700 dark:text-slate-300">
                  Role: {authRole || "Member"}
                </span>
              </div>
            </div>
          </div>

          {/* Role Navigation Tabs */}
          <div className="flex items-center gap-2 rounded-[10px] bg-slate-100 p-1.5 dark:bg-slate-800 self-start sm:self-auto">
            <button
              type="button"
              onClick={() => {
                setActiveRole("student");
                setSuccessMessage(null);
                setErrorMessage(null);
              }}
              className={`flex items-center gap-1.5 rounded-[10px] px-3.5 py-2 text-xs font-semibold transition ${
                activeRole === "student"
                  ? "bg-white text-primary shadow-sm dark:bg-slate-700 dark:text-white"
                  : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
              }`}
            >
              <GraduationCap className="h-4 w-4" />
              <span>Student Profile</span>
            </button>
            <button
              type="button"
              onClick={() => {
                setActiveRole("employer");
                setSuccessMessage(null);
                setErrorMessage(null);
              }}
              className={`flex items-center gap-1.5 rounded-[10px] px-3.5 py-2 text-xs font-semibold transition ${
                activeRole === "employer"
                  ? "bg-white text-primary shadow-sm dark:bg-slate-700 dark:text-white"
                  : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
              }`}
            >
              <Briefcase className="h-4 w-4" />
              <span>Employer Profile</span>
            </button>
          </div>
        </div>

        {/* Unverified Institutional Email Banner */}
        {!isVerified && (
          <div className="mt-6 flex items-start gap-3 rounded-[10px] border border-amber-200 bg-amber-50/80 p-4 text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-200">
            <ShieldAlert className="h-5 w-5 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" />
            <div className="leading-relaxed">
              <strong className="font-semibold">Action Required for University Verification:</strong> Your institutional email address is currently unverified in Keycloak. You may complete your profile details now, but verified university email status is mandatory before you can upload resumes or apply for campus jobs.
            </div>
          </div>
        )}
      </div>

      {/* Feedback Alerts */}
      {successMessage && (
        <div className="mb-6 flex items-center gap-2.5 rounded-[10px] border border-emerald-200 bg-emerald-50 p-4 text-sm text-emerald-800 dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300">
          <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
          <span>{successMessage}</span>
        </div>
      )}

      {errorMessage && (
        <div className="mb-6 flex items-center gap-2.5 rounded-[10px] border border-rose-200 bg-rose-50 p-4 text-sm text-rose-800 dark:border-rose-900/50 dark:bg-rose-950/40 dark:text-rose-300">
          <AlertCircle className="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400" />
          <span>{errorMessage}</span>
        </div>
      )}

      {/* Main Profile Editor Panels */}
      {activeRole === "student" ? (
        <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
          {/* Left / Middle: Student Profile Form */}
          <div className="lg:col-span-2 space-y-8">
            <form
              onSubmit={handleSaveStudent}
              className="rounded-[10px] border border-slate-200/80 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8 space-y-6"
            >
              <div className="border-b border-slate-100 pb-4 dark:border-slate-800 flex items-center justify-between">
                <div>
                  <h2 className="text-lg font-bold text-slate-900 dark:text-white">
                    Academic & Personal Details
                  </h2>
                  <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                    Showcase your major, graduation timeline, and skills to campus employers.
                  </p>
                </div>
                {isProfileLoading && (
                  <Loader2 className="h-4 w-4 animate-spin text-primary" />
                )}
              </div>

              {/* Name fields */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                    First Name <span className="text-rose-500">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="e.g. Alex"
                    className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                  />
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                    Last Name <span className="text-rose-500">*</span>
                  </label>
                  <input
                    type="text"
                    required
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="e.g. Rivera"
                    className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                  />
                </div>
              </div>

              {/* Department & Graduation Year */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div>
                  <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                    Academic Department / Major <span className="text-rose-500">*</span>
                  </label>
                  <div className="relative">
                    <select
                      value={department}
                      onChange={(e) => setDepartment(e.target.value)}
                      className="w-full appearance-none rounded-[10px] border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                    >
                      {COMMON_DEPARTMENTS.map((dept) => (
                        <option key={dept} value={dept}>
                          {dept}
                        </option>
                      ))}
                    </select>
                    <GraduationCap className="pointer-events-none absolute right-3.5 top-3 h-4 w-4 text-slate-400" />
                  </div>
                </div>

                <div>
                  <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                    Expected Graduation Year <span className="text-rose-500">*</span>
                  </label>
                  <div className="relative">
                    <select
                      value={graduationYear}
                      onChange={(e) => setGraduationYear(Number(e.target.value))}
                      className="w-full appearance-none rounded-[10px] border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                    >
                      {GRADUATION_YEARS.map((yr) => (
                        <option key={yr} value={yr}>
                          {yr}
                        </option>
                      ))}
                    </select>
                    <Calendar className="pointer-events-none absolute right-3.5 top-3 h-4 w-4 text-slate-400" />
                  </div>
                </div>
              </div>

              {/* Bio with Character Counter */}
              <div>
                <div className="flex items-center justify-between mb-1.5">
                  <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                    Student Bio
                  </label>
                  <span
                    className={`text-[11px] font-mono ${
                      bio.length > 900
                        ? "text-rose-500 font-bold"
                        : "text-slate-400 dark:text-slate-500"
                    }`}
                  >
                    {bio.length} / 1000 characters
                  </span>
                </div>
                <textarea
                  rows={4}
                  maxLength={1000}
                  value={bio}
                  onChange={(e) => setBio(e.target.value)}
                  placeholder="Introduce yourself, your academic focus, relevant coursework, lab experiences, and what projects you're excited to work on..."
                  className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 p-3.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                />
              </div>

              {/* Skills Tags Management */}
              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                  Skills & Technical Competencies
                </label>
                <div className="flex items-center gap-2">
                  <input
                    type="text"
                    value={newSkillInput}
                    onChange={(e) => setNewSkillInput(e.target.value)}
                    onKeyDown={(e) => {
                      if (e.key === "Enter") {
                        e.preventDefault();
                        handleAddSkill(newSkillInput);
                      }
                    }}
                    placeholder="Add a skill (e.g. Next.js, Python, Figma) and press Enter"
                    className="flex-1 rounded-[10px] border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                  />
                  <button
                    type="button"
                    onClick={() => handleAddSkill(newSkillInput)}
                    className="inline-flex items-center gap-1 rounded-[10px] bg-slate-100 px-3.5 py-2.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
                  >
                    <Plus className="h-4 w-4" />
                    <span>Add</span>
                  </button>
                </div>

                {/* Popular Skill Presets */}
                <div className="mt-3 flex flex-wrap items-center gap-1.5">
                  <span className="text-[11px] font-medium text-slate-400 dark:text-slate-500 mr-1">
                    Popular:
                  </span>
                  {POPULAR_SKILLS.map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      onClick={() => handleAddSkill(preset)}
                      className={`rounded-[10px] px-2 py-0.5 text-[11px] font-medium transition ${
                        skills.includes(preset)
                          ? "bg-emerald-50 dark:bg-emerald-950/50 text-emerald-800 dark:text-emerald-300 dark:bg-emerald-950/60 dark:text-emerald-300"
                          : "bg-slate-100 text-slate-600 hover:bg-emerald-50 dark:bg-emerald-950/50 hover:text-primary dark:bg-slate-800 dark:text-slate-400 dark:hover:bg-slate-700"
                      }`}
                    >
                      + {preset}
                    </button>
                  ))}
                </div>

                {/* Active Skill Badges */}
                {skills.length > 0 && (
                  <div className="mt-4 flex flex-wrap gap-2">
                    {skills.map((skill) => (
                      <span
                        key={skill}
                        className="inline-flex items-center gap-1.5 rounded-[10px] bg-emerald-50 px-3 py-1 text-xs font-medium text-emerald-800 dark:border-emerald-900/50 dark:bg-emerald-950/60 dark:text-emerald-300 border border-emerald-200"
                      >
                        <span>{skill}</span>
                        <button
                          type="button"
                          onClick={() => handleRemoveSkill(skill)}
                          className="rounded-full p-0.5 hover:bg-emerald-200/60 dark:hover:bg-emerald-800/60 transition"
                          title="Remove skill"
                        >
                          <X className="h-3 w-3" />
                        </button>
                      </span>
                    ))}
                  </div>
                )}
              </div>

              {/* Portfolio & External Links */}
              <div>
                <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                  External Portfolio, GitHub, or Research Links
                </label>
                <div className="flex items-center gap-2">
                  <div className="relative flex-1">
                    <input
                      type="url"
                      value={newLinkInput}
                      onChange={(e) => {
                        setNewLinkInput(e.target.value);
                        if (linkError) setLinkError(null);
                      }}
                      onKeyDown={(e) => {
                        if (e.key === "Enter") {
                          e.preventDefault();
                          handleAddLink();
                        }
                      }}
                      placeholder="https://github.com/your-handle or https://portfolio.me"
                      className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 pl-9 pr-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                    />
                    <Globe className="pointer-events-none absolute left-3 top-3 h-4 w-4 text-slate-400" />
                  </div>
                  <button
                    type="button"
                    onClick={handleAddLink}
                    className="inline-flex items-center gap-1 rounded-[10px] bg-slate-100 px-3.5 py-2.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-200 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
                  >
                    <Plus className="h-4 w-4" />
                    <span>Add Link</span>
                  </button>
                </div>
                {linkError && (
                  <p className="mt-1.5 text-xs text-rose-600 dark:text-rose-400 font-medium">
                    {linkError}
                  </p>
                )}

                {/* Added Portfolio Links List */}
                {portfolioLinks.length > 0 && (
                  <div className="mt-3 space-y-2">
                    {portfolioLinks.map((link) => (
                      <div
                        key={link}
                        className="flex items-center justify-between rounded-[10px] border border-slate-100 bg-slate-50/60 px-3.5 py-2 text-xs text-slate-700 dark:border-slate-800 dark:bg-slate-800/50 dark:text-slate-300"
                      >
                        <a
                          href={link}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="flex items-center gap-1.5 hover:text-primary truncate max-w-md transition"
                        >
                          <ExternalLink className="h-3.5 w-3.5 shrink-0 text-slate-400" />
                          <span className="truncate">{link}</span>
                        </a>
                        <button
                          type="button"
                          onClick={() => handleRemoveLink(link)}
                          className="rounded-[10px] p-1 text-slate-400 hover:text-rose-600 hover:bg-slate-100 dark:hover:bg-slate-700 transition"
                          title="Remove link"
                        >
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              {/* Save Button */}
              <div className="pt-4 border-t border-slate-100 dark:border-slate-800 flex justify-end">
                <button
                  type="submit"
                  disabled={isSavingStudent}
                  className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-2.5 text-sm font-semibold text-white shadow-sm shadow-sm transition hover:bg-primary disabled:opacity-50"
                >
                  {isSavingStudent ? (
                    <>
                      <Loader2 className="h-4 w-4 animate-spin" />
                      <span>Saving Profile...</span>
                    </>
                  ) : (
                    <>
                      <Save className="h-4 w-4" />
                      <span>Save Profile</span>
                    </>
                  )}
                </button>
              </div>
            </form>
          </div>

          {/* Right Column: Resume Uploader Card */}
          <div className="space-y-6">
            <div className="rounded-[10px] border border-slate-200/80 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-7">
              <div className="mb-4">
                <h3 className="text-base font-bold text-slate-900 dark:text-white">
                  Student Resume
                </h3>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                  Your resume will automatically attach to job proposals submitted on campus.
                </p>
              </div>

              <ResumeUploader
                resumeKey={studentProfile?.resume_key}
                resumeFilename={studentProfile?.resume_filename}
                resumeByteSize={studentProfile?.resume_byte_size}
                updatedAt={studentProfile?.updated_at}
                isVerified={isVerified}
                onUploadSuccess={() => {
                  refetchStudentProfile();
                }}
              />
            </div>

            {/* Application Quick Link Card */}
            <div className="rounded-[10px] border border-border bg-gradient-to-br from-emerald-50/50 to-card p-6 shadow-sm dark:from-card dark:to-card">
              <div className="flex items-center gap-2.5 text-primary dark:text-emerald-500 mb-2">
                <Sparkles className="h-5 w-5" />
                <h4 className="text-sm font-bold">Applications & Contracts</h4>
              </div>
              <p className="text-xs text-muted-foreground leading-relaxed mb-4">
                Track all your submitted gig proposals, review acceptances, and view active contracts.
              </p>
              <Link
                href="/applications"
                className="inline-flex items-center justify-center w-full rounded-[10px] border border-emerald-200 bg-card px-4 py-2.5 text-xs font-semibold text-emerald-800 dark:border-emerald-800/50 dark:bg-card dark:text-emerald-300 shadow-sm transition hover:bg-emerald-50 dark:hover:bg-accent"
              >
                Go to Applications Dashboard
              </Link>
            </div>
          </div>
        </div>
      ) : (
        /* Employer Profile Form */
        <div className="max-w-2xl">
          <form
            onSubmit={handleSaveEmployer}
            className="rounded-[10px] border border-slate-200/80 bg-white p-6 shadow-sm dark:border-slate-800 dark:bg-slate-900 sm:p-8 space-y-6"
          >
            <div className="border-b border-slate-100 pb-4 dark:border-slate-800 flex items-center justify-between">
              <div>
                <h2 className="text-lg font-bold text-slate-900 dark:text-white">
                  Employer / Campus Organization Profile
                </h2>
                <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                  Information displayed on your campus gig postings and applicant contracts.
                </p>
              </div>
              {isEmployerLoading && (
                <Loader2 className="h-4 w-4 animate-spin text-primary" />
              )}
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                Organization / Company / Lab Name <span className="text-rose-500">*</span>
              </label>
              <input
                type="text"
                required
                value={companyOrOrg}
                onChange={(e) => setCompanyOrOrg(e.target.value)}
                placeholder="e.g. Distributed Systems Research Lab or Campus Activities Board"
                className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                Primary Contact Name <span className="text-rose-500">*</span>
              </label>
              <input
                type="text"
                required
                value={contactName}
                onChange={(e) => setContactName(e.target.value)}
                placeholder="e.g. Dr. Sarah Jenkins"
                className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 px-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                About Your Organization
              </label>
              <textarea
                rows={4}
                value={employerDesc}
                onChange={(e) => setEmployerDesc(e.target.value)}
                placeholder="Describe your research team, university club, startup, or department mission..."
                className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 p-3.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
              />
            </div>

            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300 mb-1.5">
                Official Website or Lab Link
              </label>
              <div className="relative">
                <input
                  type="url"
                  value={website}
                  onChange={(e) => setWebsite(e.target.value)}
                  placeholder="https://lab.university.edu"
                  className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 pl-9 pr-3.5 py-2.5 text-sm text-slate-900 outline-none transition focus:border-primary focus:bg-white focus:ring-2 focus:ring-ring/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
                />
                <Globe className="pointer-events-none absolute left-3 top-3 h-4 w-4 text-slate-400" />
              </div>
            </div>

            <div className="pt-4 border-t border-slate-100 dark:border-slate-800 flex justify-end">
              <button
                type="submit"
                disabled={isSavingEmployer}
                className="inline-flex items-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-6 py-2.5 text-sm font-semibold text-white shadow-sm shadow-sm transition hover:bg-primary disabled:opacity-50"
              >
                {isSavingEmployer ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Saving Profile...</span>
                  </>
                ) : (
                  <>
                    <Save className="h-4 w-4" />
                    <span>Save Employer Profile</span>
                  </>
                )}
              </button>
            </div>
          </form>
        </div>
      )}
    </div>
  );
}
