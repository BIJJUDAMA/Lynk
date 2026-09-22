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
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      </div>
    );
  }

  if (!isAuthenticated || !user) {
    return (
      <div className="mx-auto max-w-2xl px-4 py-16 text-center">
        <div className="rounded-xl border border-border bg-card p-8 shadow-none">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-muted-foreground">
            <Lock className="h-6 w-6" />
          </div>
          <h2 className="mt-4 font-serif text-2xl font-normal tracking-tight text-foreground">
            Campus Sign In Required
          </h2>
          <p className="mt-2 text-sm text-muted-foreground">
            Please sign in with your campus credentials to view and manage your profile.
          </p>
          <div className="mt-6">
            <Button onClick={() => login({ redirectPath: "/profile" })} className="rounded-md">
              <UserIcon className="h-4 w-4 mr-2" />
              Sign In with Campus Account
            </Button>
          </div>
        </div>
      </div>
    );
  }

  const displayName = firstName || lastName ? `${firstName} ${lastName}`.trim() : "Campus Member";

  return (
    <div className="mx-auto max-w-5xl px-4 py-10 sm:px-6 lg:px-8">
      {/* Header Profile Identity Banner */}
      <div className="mb-8 rounded-xl border border-border bg-card p-6 shadow-none sm:p-8">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
          <div className="flex items-center gap-4">
            <div className="flex h-14 w-14 shrink-0 items-center justify-center rounded-xl bg-muted text-lg font-mono font-semibold text-foreground">
              {firstName ? firstName[0].toUpperCase() : user.email[0].toUpperCase()}
              {lastName ? lastName[0].toUpperCase() : ""}
            </div>
            <div>
              <div className="flex flex-wrap items-center gap-2.5">
                <h1 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground">
                  {displayName}
                </h1>
                {isVerified ? (
                  <Badge variant="default">VERIFIED .EDU</Badge>
                ) : (
                  <Badge variant="warning">VERIFICATION PENDING</Badge>
                )}
              </div>
              <div className="mt-1 flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-xs text-muted-foreground">
                <span className="inline-flex items-center gap-1">
                  <Mail className="h-3 w-3" />
                  {user.email}
                </span>
                {profile?.department && <span>- {profile.department}</span>}
                {profile?.organization && <span>- {profile.organization}</span>}
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

      {/* Two-Column Workspace Layout */}
      <div className="grid grid-cols-1 gap-8 lg:grid-cols-[1fr_360px]">
        {/* Left Column: Unified Profile Form */}
        <div className="space-y-6">
          <form onSubmit={handleSubmit} className="space-y-6">
            {/* Section 1: Identity & Bio */}
            <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-foreground">
                  1. Campus Identity &amp; Bio
                </h2>
                <p className="text-xs text-muted-foreground">
                  Your public display name and professional background.
                </p>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-foreground">First Name</label>
                  <Input
                    type="text"
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="e.g. Sarah"
                    className="mt-1.5 text-sm"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-foreground">Last Name</label>
                  <Input
                    type="text"
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="e.g. Chen"
                    className="mt-1.5 text-sm"
                  />
                </div>
              </div>

              <div>
                <div className="flex items-center justify-between">
                  <label className="block text-xs font-medium text-foreground">About / Bio</label>
                  <span className="font-mono text-[11px] text-muted-foreground">
                    {bio.length} / 1000 chars
                  </span>
                </div>
                <Textarea
                  rows={4}
                  maxLength={1000}
                  value={bio}
                  onChange={(e) => setBio(e.target.value)}
                  placeholder="Share your focus, project experience, research background, or campus initiatives..."
                  className="mt-1.5 min-h-[100px] text-sm"
                />
              </div>
            </div>

            {/* Section 2: Department & Academic Year */}
            <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-foreground">
                  2. Academic Department &amp; Year
                </h2>
                <p className="text-xs text-muted-foreground">
                  Your academic focus area and expected graduation date.
                </p>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Department / Discipline
                  </label>
                  <select
                    value={department}
                    onChange={(e) => setDepartment(e.target.value)}
                    className="mt-1.5 flex h-10 w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-foreground transition-colors"
                  >
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
                  <label className="block text-xs font-medium text-foreground">
                    Graduation Year (Optional)
                  </label>
                  <select
                    value={graduationYear || ""}
                    onChange={(e) =>
                      setGraduationYear(e.target.value ? Number(e.target.value) : undefined)
                    }
                    className="mt-1.5 flex h-10 w-full rounded-md border border-border bg-card px-3 py-2 text-sm text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-foreground transition-colors font-mono"
                  >
                    <option value="">Not Applicable / Staff</option>
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
            <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-foreground">
                  3. Skills &amp; Technical Competencies
                </h2>
                <p className="text-xs text-muted-foreground">
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
                  placeholder="e.g. Python, Next.js, Figma, PyTorch..."
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

              {/* Current skill tags */}
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
                        className="text-muted-foreground hover:text-foreground transition-colors"
                        aria-label={`Remove ${skill}`}
                      >
                        <X className="h-3 w-3" />
                      </button>
                    </Badge>
                  ))}
                </div>
              )}

              {/* Popular skill presets */}
              <div className="pt-3 border-t border-border">
                <span className="font-mono text-[11px] text-muted-foreground">
                  Suggested Skills:
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

            {/* Section 4: External Links & Affiliation */}
            <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-4">
              <div>
                <h2 className="text-sm font-semibold tracking-tight text-foreground">
                  4. Affiliation &amp; External Links
                </h2>
                <p className="text-xs text-muted-foreground">
                  Lab affiliation, GitHub, portfolio, or publications.
                </p>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Affiliation / Lab / Club (Optional)
                  </label>
                  <Input
                    type="text"
                    value={organization}
                    onChange={(e) => setOrganization(e.target.value)}
                    placeholder="e.g. AI Robotics Lab, CS Club"
                    className="mt-1.5 text-sm"
                  />
                </div>

                <div>
                  <label className="block text-xs font-medium text-foreground">
                    Affiliation Website (Optional)
                  </label>
                  <Input
                    type="text"
                    value={orgWebsite}
                    onChange={(e) => setOrgWebsite(e.target.value)}
                    placeholder="https://lab.university.edu"
                    className="mt-1.5 text-sm"
                  />
                </div>
              </div>

              <div>
                <label className="block text-xs font-medium text-foreground mb-1.5">
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
                    placeholder="https://github.com/username"
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
                        className="flex items-center justify-between rounded-md border border-border bg-card px-3 py-2 text-xs"
                      >
                        <a
                          href={isValidUrl(link) ? link : "#"}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="inline-flex items-center gap-1.5 font-mono text-foreground hover:underline truncate max-w-[85%]"
                        >
                          <Globe className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
                          <span className="truncate">{link}</span>
                          <ExternalLink className="h-3 w-3 shrink-0 text-muted-foreground" />
                        </a>
                        <button
                          type="button"
                          onClick={() => handleRemoveLink(link)}
                          className="text-muted-foreground hover:text-foreground transition-colors"
                          aria-label="Remove link"
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
                className="inline-flex items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white shadow-none hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50"
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
          <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-3">
            <div className="flex items-center gap-2">
              <FileText className="h-4 w-4 text-foreground" />
              <h2 className="font-serif text-xl font-medium text-foreground">Resume Vault</h2>
            </div>
            <p className="text-xs text-muted-foreground leading-relaxed">
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
          <div className="rounded-xl border border-border bg-card p-6 shadow-none space-y-4">
            <div>
              <h3 className="font-serif text-lg font-medium text-foreground">Campus Trust</h3>
              <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
                Authenticity guaranteed through institutional .edu credentials.
              </p>
            </div>

            <div className="space-y-3 text-xs">
              <div className="flex items-center justify-between border-b border-border pb-2.5">
                <span className="text-muted-foreground">Institutional Email</span>
                <span className="font-mono text-foreground truncate max-w-[170px]">
                  {user.email}
                </span>
              </div>
              <div className="flex items-center justify-between border-b border-border pb-2.5">
                <span className="text-muted-foreground">Verification</span>
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
                <span className="text-muted-foreground">Status</span>
                <span className="font-mono text-foreground">Active Campus Member</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
