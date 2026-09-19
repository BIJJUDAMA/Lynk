"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import {
  User as UserIcon,
  ShieldCheck,
  ShieldAlert,
  GraduationCap,
  Building,
  Globe,
  Plus,
  X,
  Save,
  CheckCircle2,
  AlertCircle,
  Loader2,
  ExternalLink,
  Mail,
  FileText,
  Briefcase,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { getMyProfile, updateMyProfile, ApiClientError } from "@/lib/api";
import { useQuery, useMutation } from "@/lib/useApi";
import { COMMON_DEPARTMENTS, POPULAR_SKILLS, isValidUrl } from "@/lib/formatters";
import { ResumeUploader } from "@/components/profile/ResumeUploader";
import type { Profile, UpdateProfileRequest } from "@/types/api";

const CURRENT_YEAR = new Date().getFullYear();
const GRADUATION_YEARS = Array.from({ length: 9 }, (_, i) => CURRENT_YEAR - 2 + i);

export default function ProfilePage() {
  const { user, isVerified, isAuthenticated, isLoading: isAuthLoading, login } = useAuth();

  // Query unified campus profile
  const {
    data: profile,
    isLoading: isProfileLoading,
    refetch: refetchProfile,
  } = useQuery(getMyProfile, {
    enabled: isAuthenticated,
  });

  // Form State
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [department, setDepartment] = useState(COMMON_DEPARTMENTS[0]);
  const [customDepartment, setCustomDepartment] = useState("");
  const [graduationYear, setGraduationYear] = useState<number | undefined>(CURRENT_YEAR + 2);
  const [bio, setBio] = useState("");
  const [organization, setOrganization] = useState("");
  const [orgWebsite, setOrgWebsite] = useState("");
  const [skills, setSkills] = useState<string[]>([]);
  const [newSkillInput, setNewSkillInput] = useState("");
  const [portfolioLinks, setPortfolioLinks] = useState<string[]>([]);
  const [newLinkInput, setNewLinkInput] = useState("");
  const [linkError, setLinkError] = useState<string | null>(null);

  // Status feedback
  const [successMessage, setSuccessMessage] = useState<string | null>(null);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Sync loaded profile into form state
  useEffect(() => {
    if (profile) {
      setFirstName(profile.first_name || "");
      setLastName(profile.last_name || "");
      if (COMMON_DEPARTMENTS.includes(profile.department)) {
        setDepartment(profile.department);
        setCustomDepartment("");
      } else if (profile.department) {
        setDepartment("Other");
        setCustomDepartment(profile.department);
      }
      setGraduationYear(profile.graduation_year || undefined);
      setBio(profile.bio || profile.description || "");
      setOrganization(profile.organization || profile.company_or_org || "");
      setOrgWebsite(profile.organization_website || profile.website || "");
      setSkills(Array.isArray(profile.skills) ? profile.skills : []);
      setPortfolioLinks(Array.isArray(profile.portfolio_links) ? profile.portfolio_links : []);
    } else if (user) {
      if (user.name) {
        const parts = user.name.split(" ");
        setFirstName(parts[0] || "");
        setLastName(parts.slice(1).join(" ") || "");
      }
    }
  }, [profile, user]);

  // Mutation for updating profile
  const { mutate: mutateProfile, isLoading: isSaving } = useMutation(
    async (payload: UpdateProfileRequest) => {
      return await updateMyProfile(payload);
    }
  );

  const handleAddSkill = (skillToAdd?: string) => {
    const s = (skillToAdd || newSkillInput).trim();
    if (!s) return;
    if (skills.some((existing) => existing.toLowerCase() === s.toLowerCase())) {
      setNewSkillInput("");
      return;
    }
    if (skills.length >= 15) {
      setErrorMessage("Maximum 15 skills allowed.");
      return;
    }
    setSkills([...skills, s]);
    setNewSkillInput("");
    setErrorMessage(null);
  };

  const handleRemoveSkill = (skillToRemove: string) => {
    setSkills(skills.filter((s) => s !== skillToRemove));
  };

  const handleAddLink = () => {
    const raw = newLinkInput.trim();
    if (!raw) return;

    let targetUrl = raw;
    // Reject non-http/https URL schemes like javascript:, data:, ftp:
    if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(raw) && !/^https?:\/\//i.test(raw)) {
      setLinkError("Only http:// and https:// URLs are allowed.");
      return;
    }

    if (!targetUrl.startsWith("http://") && !targetUrl.startsWith("https://")) {
      targetUrl = "https://" + targetUrl;
    }

    if (!isValidUrl(targetUrl)) {
      setLinkError(
        "Please enter a valid http:// or https:// URL (e.g. https://github.com/username)"
      );
      return;
    }

    if (portfolioLinks.includes(targetUrl)) {
      setNewLinkInput("");
      setLinkError(null);
      return;
    }

    if (portfolioLinks.length >= 5) {
      setLinkError("Maximum 5 links allowed.");
      return;
    }

    setPortfolioLinks([...portfolioLinks, targetUrl]);
    setNewLinkInput("");
    setLinkError(null);
  };

  const handleRemoveLink = (linkToRemove: string) => {
    setPortfolioLinks(portfolioLinks.filter((l) => l !== linkToRemove));
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSuccessMessage(null);
    setErrorMessage(null);

    const effectiveDepartment =
      department === "Other" ? customDepartment.trim() : department.trim();

    const trimmedOrgWebsite = orgWebsite.trim();
    let validatedOrgWebsite = "";
    if (trimmedOrgWebsite) {
      let targetWebsite = trimmedOrgWebsite;
      if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(targetWebsite) && !/^https?:\/\//i.test(targetWebsite)) {
        setErrorMessage("Organization website must use http:// or https://");
        return;
      }
      if (!targetWebsite.startsWith("http://") && !targetWebsite.startsWith("https://")) {
        targetWebsite = "https://" + targetWebsite;
      }
      if (!isValidUrl(targetWebsite)) {
        setErrorMessage(
          "Please enter a valid organization website URL (must be http:// or https://)"
        );
        return;
      }
      validatedOrgWebsite = targetWebsite;
    }

    for (const link of portfolioLinks) {
      if (!isValidUrl(link)) {
        setErrorMessage(
          "One or more portfolio links are invalid. Only http:// and https:// URLs are allowed."
        );
        return;
      }
    }

    const payload: UpdateProfileRequest = {
      first_name: firstName.trim(),
      last_name: lastName.trim(),
      department: effectiveDepartment,
      graduation_year: graduationYear ? Number(graduationYear) : 0,
      bio: bio.trim(),
      skills,
      portfolio_links: portfolioLinks.filter((l) => l.trim() !== ""),
      organization: organization.trim(),
      organization_website: validatedOrgWebsite,
    };

    try {
      await mutateProfile(payload);
      setSuccessMessage("Campus profile updated successfully.");
      refetchProfile();
      setTimeout(() => setSuccessMessage(null), 4000);
    } catch (err) {
      if (err instanceof ApiClientError) {
        setErrorMessage(err.message || "Failed to update profile.");
      } else {
        setErrorMessage("An unexpected error occurred while saving profile.");
      }
    }
  };

  if (isAuthLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <Loader2 className="h-8 w-8 animate-spin text-muted-foreground" />
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 text-center">
        <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
          <UserIcon className="h-6 w-6" />
        </div>
        <h2 className="mt-4 text-xl font-semibold tracking-tight text-foreground">
          Authentication Required
        </h2>
        <p className="mt-2 text-sm text-muted-foreground">
          Please sign in with your campus credentials to view and manage your profile.
        </p>
        <div className="mt-6">
          <button
            onClick={() => login()}
            className="rounded-lg bg-primary px-5 py-2.5 text-sm font-semibold text-primary-foreground shadow-sm hover:bg-primary/90"
          >
            Sign In with Campus Account
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Header Profile Identity Banner */}
      <div className="mb-8 rounded-xl border border-border bg-card p-6 shadow-sm sm:p-8">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-16 w-16 items-center justify-center rounded-xl bg-primary/10 text-xl font-bold text-primary">
              {firstName ? firstName[0].toUpperCase() : user.email[0].toUpperCase()}
              {lastName ? lastName[0].toUpperCase() : ""}
            </div>
            <div>
              <div className="flex items-center gap-2.5">
                <h1 className="text-xl font-semibold tracking-tight text-foreground sm:text-2xl">
                  {firstName || lastName ? `${firstName} ${lastName}`.trim() : "Campus Member"}
                </h1>
                {isVerified ? (
                  <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-0.5 text-xs font-medium text-emerald-700 dark:text-emerald-300">
                    <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                    <span>Verified .edu</span>
                  </span>
                ) : (
                  <span className="inline-flex items-center gap-1 rounded-full border border-amber-500/20 bg-amber-500/10 px-2.5 py-0.5 text-xs font-medium text-amber-700 dark:text-amber-300">
                    <ShieldAlert className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                    <span>Verification Pending</span>
                  </span>
                )}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 text-xs text-muted-foreground">
                <span className="inline-flex items-center gap-1">
                  <Mail className="h-3 w-3" />
                  {user.email}
                </span>
                {profile?.department && <span>• Department: {profile.department}</span>}
                {profile?.organization && <span>• Org / Lab: {profile.organization}</span>}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Link
              href="/activity"
              className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background px-3.5 py-2 text-xs font-medium text-foreground transition hover:bg-muted"
            >
              <Briefcase className="h-3.5 w-3.5" />
              <span>My Activity</span>
            </Link>
          </div>
        </div>
      </div>

      {/* Notifications */}
      {successMessage && (
        <div className="mb-6 flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 px-4 py-3 text-sm text-emerald-700 dark:text-emerald-300">
          <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
          <span>{successMessage}</span>
        </div>
      )}

      {errorMessage && (
        <div className="mb-6 flex items-center gap-2 rounded-lg border border-red-500/30 bg-red-500/10 px-4 py-3 text-sm text-red-700 dark:text-red-300">
          <AlertCircle className="h-4 w-4 shrink-0 text-red-600 dark:text-red-400" />
          <span>{errorMessage}</span>
        </div>
      )}

      <div className="grid grid-cols-1 gap-8 lg:grid-cols-3">
        {/* Left 2 Cols: Unified Profile Form */}
        <div className="lg:col-span-2 space-y-8">
          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Identity & Department */}
            <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
              <h2 className="text-base font-semibold text-foreground">Campus Identity & Details</h2>
              <p className="mt-1 text-xs text-muted-foreground">
                Your personal details, academic department, and affiliation.
              </p>

              <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-foreground">First Name</label>
                  <input
                    type="text"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="e.g. Sarah"
                    className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-foreground">Last Name</label>
                  <input
                    type="text"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="e.g. Chen"
                    className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Department / Discipline
                  </label>
                  <select
                    value={department}
                    onChange={(e) => setDepartment(e.target.value)}
                    className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                  >
                    {COMMON_DEPARTMENTS.map((dept) => (
                      <option key={dept} value={dept}>
                        {dept}
                      </option>
                    ))}
                    <option value="Other">Other (Custom)</option>
                  </select>
                  {department === "Other" && (
                    <input
                      type="text"
                      value={customDepartment}
                      onChange={(e) => setCustomDepartment(e.target.value)}
                      placeholder="Specify your department"
                      className="mt-2 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                    />
                  )}
                </div>

                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Graduation Year (Optional)
                  </label>
                  <select
                    value={graduationYear || ""}
                    onChange={(e) =>
                      setGraduationYear(e.target.value ? Number(e.target.value) : undefined)
                    }
                    className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                  >
                    <option value="">Not Applicable / Faculty / Staff</option>
                    {GRADUATION_YEARS.map((yr) => (
                      <option key={yr} value={yr}>
                        Class of {yr}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Affiliation / Lab / Org (Optional)
                  </label>
                  <input
                    type="text"
                    value={organization}
                    onChange={(e) => setOrganization(e.target.value)}
                    placeholder="e.g. AI Robotics Lab, CS Club, or Company"
                    className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Affiliation Website (Optional)
                  </label>
                  <input
                    type="text"
                    value={orgWebsite}
                    onChange={(e) => setOrgWebsite(e.target.value)}
                    placeholder="https://lab.university.edu"
                    className="mt-1.5 w-full rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                  />
                </div>
              </div>
            </div>

            {/* Bio */}
            <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
              <div className="flex items-center justify-between">
                <h2 className="text-base font-semibold text-foreground">About & Background</h2>
                <span className="text-xs text-muted-foreground">
                  {bio.length} / 1000 characters
                </span>
              </div>
              <p className="mt-1 text-xs text-muted-foreground">
                Introduce yourself, your research interests, freelance offerings, or what projects
                you are hiring for.
              </p>

              <textarea
                rows={4}
                maxLength={1000}
                value={bio}
                onChange={(e) => setBio(e.target.value)}
                placeholder="Share your focus, project experience, research background, or campus initiatives..."
                className="mt-3 w-full rounded-lg border border-border bg-background p-3 text-sm text-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
              />
            </div>

            {/* Skills & Coursework */}
            <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
              <h2 className="text-base font-semibold text-foreground">
                Skills & Technical Competencies
              </h2>
              <p className="mt-1 text-xs text-muted-foreground">
                Highlight skills relevant for taking on campus gigs or evaluating proposals.
              </p>

              <div className="mt-3 flex gap-2">
                <input
                  type="text"
                  value={newSkillInput}
                  onChange={(e) => setNewSkillInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      handleAddSkill();
                    }
                  }}
                  placeholder="e.g. Python, Next.js, Figma, PyTorch..."
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

              {/* Current tags */}
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

              {/* Popular presets */}
              <div className="mt-4 pt-3 border-t border-border/50">
                <span className="text-[11px] font-medium text-muted-foreground">
                  Suggested Skills:
                </span>
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

            {/* Links & Portfolios */}
            <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
              <h2 className="text-base font-semibold text-foreground">
                External Portfolio & Links
              </h2>
              <p className="mt-1 text-xs text-muted-foreground">
                GitHub, LinkedIn, personal portfolio, or research publications.
              </p>

              <div className="mt-3 flex gap-2">
                <input
                  type="text"
                  value={newLinkInput}
                  onChange={(e) => {
                    setNewLinkInput(e.target.value);
                    setLinkError(null);
                  }}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      handleAddLink();
                    }
                  }}
                  placeholder="https://github.com/username"
                  className="flex-1 rounded-lg border border-border bg-background px-3 py-2 text-sm text-foreground focus:border-primary focus:outline-none"
                />
                <button
                  type="button"
                  onClick={handleAddLink}
                  className="inline-flex items-center gap-1 rounded-lg bg-secondary px-3 py-2 text-xs font-medium text-secondary-foreground hover:bg-secondary/80"
                >
                  <Plus className="h-3.5 w-3.5" />
                  Add Link
                </button>
              </div>

              {linkError && (
                <p className="mt-1.5 text-xs text-red-600 dark:text-red-400">{linkError}</p>
              )}

              <div className="mt-3 space-y-1.5">
                {portfolioLinks.map((link) => (
                  <div
                    key={link}
                    className="flex items-center justify-between rounded-lg border border-border bg-background px-3 py-2 text-xs"
                  >
                    <a
                      href={isValidUrl(link) ? link : "#"}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1.5 text-primary hover:underline truncate max-w-[85%]"
                    >
                      <Globe className="h-3.5 w-3.5 shrink-0" />
                      <span className="truncate">{link}</span>
                      <ExternalLink className="h-3 w-3 shrink-0" />
                    </a>
                    <button
                      type="button"
                      onClick={() => handleRemoveLink(link)}
                      className="text-muted-foreground hover:text-foreground"
                    >
                      <X className="h-3.5 w-3.5" />
                    </button>
                  </div>
                ))}
              </div>
            </div>

            {/* Save Action */}
            <div className="flex justify-end">
              <button
                type="submit"
                disabled={isSaving}
                className="inline-flex items-center gap-2 rounded-lg bg-primary px-5 py-2.5 text-sm font-semibold text-primary-foreground shadow-sm hover:bg-primary/90 disabled:opacity-50"
              >
                {isSaving ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Saving Changes...</span>
                  </>
                ) : (
                  <>
                    <Save className="h-4 w-4" />
                    <span>Save Profile Changes</span>
                  </>
                )}
              </button>
            </div>
          </form>
        </div>

        {/* Right 1 Col: Resume & Account Verification */}
        <div className="space-y-6">
          {/* Resume Management */}
          <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
            <div className="flex items-center gap-2">
              <FileText className="h-5 w-5 text-primary" />
              <h2 className="text-base font-semibold text-foreground">Resume Document</h2>
            </div>
            <p className="mt-1 text-xs text-muted-foreground">
              Uploaded resumes are stored securely in dedicated object storage and streamed to
              prospective collaborators.
            </p>

            <div className="mt-4">
              <ResumeUploader
                resumeKey={profile?.resume_key}
                resumeFilename={profile?.resume_filename}
                resumeByteSize={profile?.resume_byte_size}
                updatedAt={profile?.updated_at}
                isVerified={isVerified}
                onUploadSuccess={() => {
                  refetchProfile();
                }}
              />
            </div>
          </div>

          {/* Verification Status Card */}
          <div className="rounded-xl border border-border bg-card p-6 shadow-sm">
            <h3 className="text-sm font-semibold text-foreground">Campus Account Trust</h3>
            <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
              Lynk enforces institutional authenticity. One account allows you to post jobs, hire
              applicants, submit proposals, and sign deliverable contracts.
            </p>

            <div className="mt-4 space-y-2 text-xs">
              <div className="flex items-center justify-between border-b border-border/60 pb-2">
                <span className="text-muted-foreground">Institutional Email</span>
                <span className="font-mono text-foreground truncate max-w-[150px]">
                  {user.email}
                </span>
              </div>
              <div className="flex items-center justify-between border-b border-border/60 pb-2">
                <span className="text-muted-foreground">Verification Status</span>
                <span
                  className={
                    isVerified ? "text-emerald-600 font-medium" : "text-amber-600 font-medium"
                  }
                >
                  {isVerified ? "Verified" : "Pending"}
                </span>
              </div>
              <div className="flex items-center justify-between pt-1">
                <span className="text-muted-foreground">Member Role</span>
                <span className="capitalize font-medium text-foreground">Campus Member</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
