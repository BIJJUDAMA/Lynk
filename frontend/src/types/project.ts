/**
 * Lynk Project-First Architecture Type Definitions
 *
 * Implements core domain models, DTOs, and lifecycle status enums
 * for the project-first collaboration platform.
 */

// ============================================================================
// Lifecycle & Domain Enums
// ============================================================================

export type ProjectVisibility = "public" | "unlisted" | "private";

export type RecruitmentStatus = "open" | "paused" | "closed";

export type LifecycleStatus = "idea" | "active" | "completed" | "archived";

export type ProjectRole = "lead" | "maintainer" | "member" | "viewer";

export type ApplicationStatus = "submitted" | "shortlisted" | "accepted" | "rejected" | "withdrawn";

export type SectionType = "overview" | "team" | "resources" | "recruitment" | "updates" | "docs";

// Constant arrays for runtime validation, forms, and filtering
export const PROJECT_VISIBILITIES: readonly ProjectVisibility[] = [
  "public",
  "unlisted",
  "private",
] as const;

export const RECRUITMENT_STATUSES: readonly RecruitmentStatus[] = [
  "open",
  "paused",
  "closed",
] as const;

export const LIFECYCLE_STATUSES: readonly LifecycleStatus[] = [
  "idea",
  "active",
  "completed",
  "archived",
] as const;

export const PROJECT_ROLES: readonly ProjectRole[] = [
  "lead",
  "maintainer",
  "member",
  "viewer",
] as const;

export const APPLICATION_STATUSES: readonly ApplicationStatus[] = [
  "submitted",
  "shortlisted",
  "accepted",
  "rejected",
  "withdrawn",
] as const;

export const SECTION_TYPES: readonly SectionType[] = [
  "overview",
  "team",
  "resources",
  "recruitment",
  "updates",
  "docs",
] as const;

// ============================================================================
// Core Domain Models
// ============================================================================

export interface Project {
  id: string;
  slug: string;
  owner_member_id: string;
  title: string;
  description: string;
  category: string;
  visibility: ProjectVisibility;
  recruitment_status: RecruitmentStatus;
  lifecycle_status: LifecycleStatus;
  settings: Record<string, unknown>;
  created_at: string;
  updated_at: string;
  similarity?: number;
  distance?: number;
}

export interface ProjectMemberProfile {
  id?: string;
  email: string;
  first_name: string;
  last_name: string;
  department: string;
  bio?: string;
}

export interface ProjectMember {
  id: string;
  project_id: string;
  member_id: string;
  role: ProjectRole;
  status: string;
  joined_at: string;
  departed_at?: string;
  email?: string;
  first_name?: string;
  last_name?: string;
  department?: string;
  member?: ProjectMemberProfile;
}

export interface ProjectSection {
  id: string;
  project_id: string;
  section_type: SectionType;
  title: string;
  enabled: boolean;
  position: number;
  config: Record<string, unknown>;
  content: string;
}

export interface ProjectResource {
  id: string;
  project_id: string;
  label: string;
  url: string;
  resource_type: string;
  created_at: string;
}

export interface ApplicantProfile {
  id?: string;
  email: string;
  first_name: string;
  last_name: string;
  department: string;
  bio?: string;
}

export interface ProjectApplication {
  id: string;
  project_id: string;
  applicant_id: string;
  status: ApplicationStatus;
  statement: string;
  resume_asset_key?: string;
  reviewer_notes?: string;
  created_at: string;
  decided_at?: string;
  applicant?: ApplicantProfile;
  applicant_name?: string;
  applicant_email?: string;
  applicant_department?: string;
  project_title?: string;
}

export interface ProjectInvitation {
  id: string;
  project_id: string;
  email: string;
  role: ProjectRole;
  status: string;
  created_at: string;
  expires_at: string;
  inviter_id?: string;
  invitee_id?: string;
  proposed_role?: ProjectRole;
}

// ============================================================================
// Request Payloads & DTOs
// ============================================================================

export interface CreateProjectPayload {
  title: string;
  description: string;
  category: string;
  visibility: ProjectVisibility;
}

export interface UpdateProjectPayload {
  title?: string;
  description?: string;
  category?: string;
  visibility?: ProjectVisibility;
  recruitment_status?: RecruitmentStatus;
  lifecycle_status?: LifecycleStatus;
  settings?: Record<string, unknown>;
}

export interface UpdateVisibilityPayload {
  visibility: ProjectVisibility;
}

export interface ApplicationAnswer {
  question_id: string;
  answer_text: string;
}

export interface SubmitApplicationPayload {
  statement: string;
  resume_asset_key?: string;
  answers?: ApplicationAnswer[];
}

export interface InviteMemberPayload {
  email: string;
  role: ProjectRole;
  invitee_id?: string;
  proposed_role?: ProjectRole;
}

export interface AddResourcePayload {
  label: string;
  url: string;
  resource_type: string;
}

export interface ProjectFilter {
  category?: string;
  recruitment_status?: RecruitmentStatus;
  visibility?: ProjectVisibility;
  lifecycle_status?: LifecycleStatus;
  search?: string;
  limit?: number;
  offset?: number;
  owner_id?: string;
}
