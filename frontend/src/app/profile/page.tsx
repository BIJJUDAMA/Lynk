"use client";

import React, { useState, useEffect } from "react";
import Link from "next/link";
import {
  User as UserIcon,
  Globe,
  Plus,
  X,
  CheckCircle2,
  AlertCircle,
  Loader2,
  ExternalLink,
  Mail,
  FileText,
  Briefcase,
  Lock,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { getMyProfile, updateMyProfile, ApiClientError } from "@/lib/api";
import { useQuery, useMutation } from "@/lib/useApi";
import { COMMON_DEPARTMENTS, POPULAR_SKILLS, isValidUrl } from "@/lib/formatters";
import { ResumeUploader } from "@/components/profile/ResumeUploader";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import type { UpdateProfileRequest } from "@/types/api";

const CURRENT_YEAR = new Date().getFullYear();
const GRADUATION_YEARS = Array.from({ length: 9 }, (_, i) => CURRENT_YEAR - 2 + i);

export default function ProfilePage() {
  const { user, isVerified, isAuthenticated, isLoading: isAuthLoading, login } = useAuth();

  // Query unified campus profile strictly from the backend database
  const {
    data: profile,
    isLoading: isProfileLoading,
    error: profileError,
    refetch: refetchProfile,
  } = useQuery(getMyProfile, {
    enabled: isAuthenticated,
  });

  // Form State - strictly blank by default until populated by database
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [department, setDepartment] = useState("");
  const [customDepartment, setCustomDepartment] = useState("");
  const [graduationYear, setGraduationYear] = useState<number | undefined>(undefined);
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

  // Sync loaded database profile into form state. If no profile in DB, leave blank.
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
      } else {
        setDepartment("");
        setCustomDepartment("");
      }
      setGraduationYear(
        profile.graduation_year && profile.graduation_year > 0 ? profile.graduation_year : undefined
      );
      setBio(profile.bio || profile.description || "");
      setOrganization(profile.organization || profile.company_or_org || "");
      setOrgWebsite(profile.organization_website || profile.website || "");
      setSkills(Array.isArray(profile.skills) ? profile.skills : []);
      setPortfolioLinks(Array.isArray(profile.portfolio_links) ? profile.portfolio_links : []);
    } else {
      // No DB data or DB unreachable -> all fields remain blank
      setFirstName("");
      setLastName("");
      setDepartment("");
      setCustomDepartment("");
      setGraduationYear(undefined);
      setBio("");
      setOrganization("");
      setOrgWebsite("");
      setSkills([]);
      setPortfolioLinks([]);
    }
  }, [profile]);

  // Mutation for updating profile in database
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
    if (/^[a-zA-Z][a-zA-Z0-9+.-]*:/.test(raw) && !/^https?:\/\//i.test(raw)) {
      setLinkError("Only http:// and https:// URLs are allowed.");
      return;
    }

    if (!targetUrl.startsWith("http://") && !targetUrl.startsWith("https://")) {
      targetUrl = "https://" + targetUrl;
    }

    if (!isValidUrl(targetUrl)) {
      setLinkError("Please enter a valid URL (e.g. https://github.com/username)");
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

  if (isAuthLoading || (isAuthenticated && isProfileLoading)) {
    return (
      <div className="mx-auto max-w-5xl px-4 py-16 sm:px-6 lg:px-8">
        <div className="flex min-h-[40vh] items-center justify-center">
          <Loader2 className="h-6 w-6 animate-spin text-ink-secondary" />
        </div>
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 text-center">
        <div className="rounded-xl border border-line bg-surface-elevated p-8 shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-surface text-ink-secondary">
            <Lock className="h-6 w-6" />
          </div>
          <h2 className="mt-4 font-display text-2xl font-normal tracking-tight text-ink">
            Campus Sign In Required
          </h2>
          <p className="mt-2 text-sm text-ink-secondary">
            Please sign in with your campus credentials to view and manage your profile.
          </p>
          <div className="mt-6">
            <Button
              onClick={() => login({ redirectPath: "/profile" })}
              className="rounded-md bg-ink text-canvas hover:bg-ink/90"
            >
              <UserIcon className="h-4 w-4 mr-2" />
              Sign In with Campus Account
            </Button>
          </div>
        </div>
      </div>
    );
  }

  // Display name purely from database fields or user account
  const displayName =
    firstName || lastName ? `${firstName} ${lastName}`.trim() : user.name || user.email || "";

  // Avatar initials purely from database name or email
  const initialChar = firstName
    ? firstName[0].toUpperCase()
    : user.email
      ? user.email[0].toUpperCase()
      : "";
  const secondInitial = lastName ? lastName[0].toUpperCase() : "";

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Header Profile Identity Banner */}
      <div className="mb-8 rounded-xl border border-line bg-surface-elevated p-6 shadow-none sm:p-8">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-surface text-lg font-mono font-semibold text-ink border border-line">
              {initialChar}
              {secondInitial}
            </div>
            <div>
              <div className="flex flex-wrap items-center gap-2.5">
                <h1 className="font-display text-2xl sm:text-3xl font-normal tracking-tight text-ink">
                  {displayName}
                </h1>
                {isVerified ? (
                  <Badge variant="default">VERIFIED .EDU</Badge>
                ) : (
                  <Badge variant="warning">VERIFICATION PENDING</Badge>
                )}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-xs text-ink-secondary">
                {user.email && (
                  <span className="inline-flex items-center gap-1">
                    <Mail className="h-3 w-3" />
                    {user.email}
                  </span>
                )}
                {profile?.department && <span>// {profile.department}</span>}
                {profile?.organization && <span>// {profile.organization}</span>}
              </div>
            </div>
          </div>

          <div className="flex items-center gap-3">
            <Button asChild variant="outline" size="sm" className="text-xs">
              <Link href="/activity">
                <Briefcase className="h-3.5 w-3.5 mr-1.5" />
                <span>My Activity</span>
              </Link>
            </Button>
          </div>
        </div>
      </div>

      {/* Notifications */}
      {successMessage && (
        <div className="mb-6 flex items-center gap-2.5 rounded-lg border border-pastel-greenText/20 bg-pastel-green px-4 py-3 text-xs text-pastel-greenText">
          <CheckCircle2 className="h-4 w-4 shrink-0" />
          <span>{successMessage}</span>
        </div>
      )}

      {errorMessage && (
        <div className="mb-6 flex items-center gap-2.5 rounded-lg border border-pastel-redText/20 bg-pastel-red px-4 py-3 text-xs text-pastel-redText">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>{errorMessage}</span>
        </div>
      )}

      {/* Notice if database profile could not be loaded */}
      {profileError && (
        <div className="mb-6 flex items-center gap-2.5 rounded-lg border border-line bg-surface px-4 py-3 text-xs text-ink-secondary">
          <AlertCircle className="h-4 w-4 shrink-0" />
          <span>
            No stored profile data found in database. Fill out the fields below to create your
            campus profile.
          </span>
        </div>
      )}

      {/* Two-Column Workspace Layout */}
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-[1fr_360px]">
        {/* Left Column: Unified Profile Form */}
        <div className="space-y-6">
          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Section 1: Identity & Bio */}
            <div className="rounded-xl border border-line bg-surface-elevated p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-ink font-display text-base">
                  1. Campus Identity &amp; Bio
                </h2>
                <p className="text-xs text-ink-secondary">
                  Your public display name and professional background.
                </p>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-ink">First Name</label>
                  <Input
                    type="text"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder=""
                    className="mt-1.5 text-sm"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-ink">Last Name</label>
                  <Input
                    type="text"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder=""
                    className="mt-1.5 text-sm"
                  />
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between">
                  <label className="block text-xs font-medium text-ink">About / Bio</label>
                  <span className="font-mono text-[11px] text-ink-secondary">
                    {bio.length} / 1000 chars
                  </span>
                </div>
                <Textarea
                  rows={4}
                  maxLength={1000}
                  value={bio}
                  onChange={(e) => setBio(e.target.value)}
                  placeholder=""
                  className="mt-1.5 min-h-[100px] text-sm"
                />
              </div>
            </div>

            {/* Section 2: Department & Academic Year */}
            <div className="rounded-xl border border-line bg-surface-elevated p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-ink font-display text-base">
                  2. Academic Department &amp; Year
                </h2>
                <p className="text-xs text-ink-secondary">
                  Your academic focus area and expected graduation date.
                </p>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-ink">
                    Department / Discipline
                  </label>
                  <select
                    value={department}
                    onChange={(e) => setDepartment(e.target.value)}
                    className="mt-1.5 flex h-10 w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring transition-colors"
                  >
                    <option value="">Select Department / Discipline</option>
                    {COMMON_DEPARTMENTS.map((dept) => (
                      <option key={dept} value={dept}>
                        {dept}
                      </option>
                    ))}
                    <option value="Other">Other (Custom)</option>
                  </select>
                  {department === "Other" && (
                    <Input
                      type="text"
                      value={customDepartment}
                      onChange={(e) => setCustomDepartment(e.target.value)}
                      placeholder="Specify custom department name"
                      className="mt-2 text-sm"
                    />
                  )}
                </div>

                <div>
                  <label className="block text-xs font-medium text-ink">
                    Graduation Year (Optional)
                  </label>
                  <select
                    value={graduationYear || ""}
                    onChange={(e) =>
                      setGraduationYear(e.target.value ? Number(e.target.value) : undefined)
                    }
                    className="mt-1.5 flex h-10 w-full rounded-md border border-line bg-card px-3 py-2 text-sm text-ink focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring transition-colors font-mono"
                  >
                    <option value="">Not Specified</option>
                    {GRADUATION_YEARS.map((yr) => (
                      <option key={yr} value={yr}>
                        Class of {yr}
                      </option>
                    ))}
                  </select>
                </div>
              </div>
            </div>

            {/* Section 3: Skills Tag Selector */}
            <div className="rounded-xl border border-line bg-surface-elevated p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-ink font-display text-base">
                  3. Skills &amp; Technical Competencies
                </h2>
                <p className="text-xs text-ink-secondary">
                  Highlight skills for gig proposals or project collaborations.
                </p>
              </div>

              <div className="flex gap-2">
                <Input
                  type="text"
                  value={newSkillInput}
                  onChange={(e) => setNewSkillInput(e.target.value)}
                  onKeyDown={(e) => {
                    if (e.key === "Enter") {
                      e.preventDefault();
                      handleAddSkill();
                    }
                  }}
                  placeholder="Enter a skill..."
                  className="flex-1 text-sm"
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

              {/* Current skill tags from database */}
              {skills.length > 0 && (
                <div className="flex flex-wrap gap-1.5">
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
                        className="text-ink-secondary hover:text-ink transition-colors"
                        aria-label={`Remove ${skill}`}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
              )}

              {/* Skill quick selectors */}
              <div className="pt-3 border-t border-line">
                <span className="font-mono text-[11px] text-ink-secondary">Presets:</span>
                <div className="mt-1.5 flex flex-wrap gap-1.5">
                  {POPULAR_SKILLS.map((preset) => (
                    <button
                      key={preset}
                      type="button"
                      disabled={skills.includes(preset)}
                      onClick={() => handleAddSkill(preset)}
                      className="rounded-md border border-line bg-card px-2 py-0.5 font-mono text-xs text-ink-secondary hover:border-ink hover:text-ink transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
                    >
                      + {preset}
                    </button>
                  ))}
                </div>
              </div>
            </div>

            {/* Section 4: External Links & Affiliation */}
            <div className="rounded-xl border border-line bg-surface-elevated p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-ink font-display text-base">
                  4. Affiliation &amp; External Links
                </h2>
                <p className="text-xs text-ink-secondary">
                  Lab affiliation, GitHub, portfolio, or publications.
                </p>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-ink">
                    Affiliation / Lab / Club (Optional)
                  </label>
                  <Input
                    type="text"
                    value={organization}
                    onChange={(e) => setOrganization(e.target.value)}
                    placeholder=""
                    className="mt-1.5 text-sm"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-ink">
                    Affiliation Website (Optional)
                  </label>
                  <Input
                    type="text"
                    value={orgWebsite}
                    onChange={(e) => setOrgWebsite(e.target.value)}
                    placeholder=""
                    className="mt-1.5 text-sm"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-ink mb-1.5">
                  Portfolio &amp; External URLs
                </label>
                <div className="flex gap-2">
                  <Input
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
                    placeholder="https://"
                    className="flex-1 text-sm"
                  />
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    onClick={handleAddLink}
                    className="h-10 px-3 text-xs"
                  >
                    <Plus className="h-3.5 w-3.5 mr-1" />
                    Add Link
                  </Button>
                </div>

                {linkError && (
                  <p className="mt-1.5 font-mono text-xs text-pastel-redText">{linkError}</p>
                )}

                {portfolioLinks.length > 0 && (
                  <div className="mt-3 space-y-1.5">
                    {portfolioLinks.map((link) => (
                      <div
                        key={link}
                        className="flex items-center justify-between rounded-md border border-line bg-card px-3 py-2 text-xs"
                      >
                        <a
                          href={isValidUrl(link) ? link : "#"}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1.5 font-mono text-ink hover:underline truncate max-w-[85%]"
                        >
                          <Globe className="h-3.5 w-3.5 shrink-0 text-ink-secondary" />
                          <span className="truncate">{link}</span>
                          <ExternalLink className="h-3 w-3 shrink-0 text-ink-secondary" />
                        </a>
                        <button
                          type="button"
                          onClick={() => handleRemoveLink(link)}
                          className="text-ink-secondary hover:text-ink transition-colors"
                          aria-label={`Remove link ${link}`}
                        >
                          <X className="h-3.5 w-3.5" />
                        </button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {/* Save Action */}
            <div className="flex justify-end pt-2">
              <button
                type="submit"
                disabled={isSaving}
                className="inline-flex items-center justify-center gap-2 rounded-md bg-ink px-5 py-2.5 text-xs font-medium text-canvas shadow-none hover:bg-ink/90 transition-colors active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50"
              >
                {isSaving ? (
                  <>
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                    <span>Saving Changes...</span>
                  </>
                ) : (
                  <span>Save Profile Changes</span>
                )}
              </button>
            </div>
          </form>
        </div>

        {/* Right Column: Resume Vault & Account Trust */}
        <div className="space-y-6">
          {/* Resume Vault Bento Card */}
          <div className="rounded-xl border border-line bg-surface-elevated p-6 shadow-none space-y-3">
            <div className="flex items-center gap-2">
              <FileText className="h-4 w-4 text-ink" />
              <h2 className="font-display text-xl font-normal text-ink">Resume Vault</h2>
            </div>
            <p className="text-xs text-ink-secondary leading-relaxed">
              Your resume is stored securely in your private MinIO vault. Share it with gig
              applications.
            </p>

            <div className="pt-2">
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

          {/* Account Trust Bento Card */}
          <div className="rounded-xl border border-line bg-surface-elevated p-6 shadow-none space-y-4">
            <div>
              <h3 className="font-display text-lg font-normal text-ink">Campus Trust</h3>
              <p className="mt-1 text-xs text-ink-secondary leading-relaxed">
                Authenticity guaranteed through institutional .edu credentials.
              </p>
            </div>

            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between border-b border-line pb-2.5">
                <span className="text-ink-secondary">Institutional Email</span>
                <span className="font-mono text-ink truncate max-w-[170px]">
                  {user.email || ""}
                </span>
              </div>
              <div className="flex items-center justify-between border-b border-line pb-2.5">
                <span className="text-ink-secondary">Verification</span>
                <span>
                  {isVerified ? (
                    <Badge variant="default" className="text-[10px]">
                      VERIFIED .EDU
                    </Badge>
                  ) : (
                    <Badge variant="warning" className="text-[10px]">
                      PENDING
                    </Badge>
                  )}
                </span>
              </div>
              <div className="flex items-center justify-between pt-0.5">
                <span className="text-ink-secondary">Status</span>
                <span className="font-mono text-ink">
                  {user.role === "admin"
                    ? "Administrator"
                    : isVerified
                      ? "Verified Campus Member"
                      : "Unverified Member"}
                </span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
