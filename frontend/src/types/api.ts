/**
 * Lynk Shared API Types
 * Strictly mirrors Go backend JSON models, envelopes, and request payloads.
 * Reflects Unified Campus Member architecture (Single Identity: Member).
 */

// ==========================================
// Generic API Response & Error Envelopes
// ==========================================

export interface ApiError {
  code: string;
  message: string;
}

export interface ApiResponse<T> {
  success: boolean;
  data: T | null;
  error: ApiError | null;
}

// ==========================================
// String Literal Enums & Domain Unions
// ==========================================

export type UserRole = "member" | "admin";

export type JobPayType = "fixed" | "hourly";

export type JobStatus = "open" | "in_progress" | "closed" | "cancelled";

export type ApplicationStatus = "pending" | "accepted" | "rejected";

export type ContractStatus = "draft" | "active" | "completed" | "cancelled";

// ==========================================
// User & Unified Profile Models
// ==========================================

export interface User {
  id: string; // UUID
  email: string;
  role: UserRole;
  created_at: string; // ISO 8601 string
  updated_at: string; // ISO 8601 string
}

export interface Profile {
  id: string;
  user_id: string;
  first_name: string;
  last_name: string;
  bio: string;
  department: string;
  graduation_year: number;
  skills: string[];
  portfolio_links: string[];
  organization?: string;
  company_or_org?: string;
  contact_name?: string;
  description?: string;
  website?: string;
  organization_website?: string;
  resume_key?: string | null;
  resume_filename?: string | null;
  resume_byte_size: number;
  updated_at: string;
}

// Backward-compatibility aliases
export type StudentProfile = Profile;
export type EmployerProfile = Profile;

export interface UserProfileSummary {
  user: User | null;
  email_verified: boolean;
  profile?: Profile | null;
  student_profile?: Profile | null;
  employer_profile?: Profile | null;
}

export interface UpdateProfileRequest {
  first_name?: string;
  last_name?: string;
  bio?: string;
  department?: string;
  graduation_year?: number;
  skills?: string[];
  portfolio_links?: string[];
  organization?: string;
  organization_website?: string;
  company_or_org?: string;
  contact_name?: string;
  description?: string;
  website?: string;
}

export type UpdateStudentProfileRequest = UpdateProfileRequest;
export type UpdateEmployerProfileRequest = UpdateProfileRequest;

export interface SyncUserRequest {
  role?: UserRole;
}

export interface ResumeUploadResponse {
  message: string;
  filename: string;
  byte_size: number;
  resume_key: string;
}

export interface ResumeDownloadResponse {
  download_url: string;
  url: string;
  filename: string;
}

// ==========================================
// Job Models
// ==========================================

export interface CreatorInfo {
  id: string;
  email: string;
  first_name?: string;
  last_name?: string;
  organization?: string;
  company_or_org?: string;
  contact_name?: string;
  department?: string;
  website?: string;
}
export type EmployerInfo = CreatorInfo;

export interface Job {
  id: string;
  created_by: string; // UUID of posting member
  employer_id?: string; // backward-compatibility alias
  title: string;
  description: string;
  budget: number;
  pay_type: JobPayType;
  required_skills: string[];
  department: string;
  deadline?: string | null;
  status: JobStatus;
  created_at: string;
  updated_at: string;
  creator?: CreatorInfo | null;
  employer?: CreatorInfo | null; // backward-compatibility alias
}

export interface JobSummary {
  id: string;
  created_by: string;
  employer_id?: string; // alias
  title: string;
  description: string;
  budget: number;
  pay_type: JobPayType;
  department: string;
  status: JobStatus;
}

export interface CreateJobRequest {
  title: string;
  description: string;
  budget: number;
  pay_type: JobPayType;
  required_skills: string[];
  department: string;
  deadline?: string | null;
}

export interface UpdateJobRequest {
  title?: string;
  description?: string;
  budget?: number;
  pay_type?: JobPayType;
  required_skills?: string[];
  department?: string;
  deadline?: string | null;
  status?: JobStatus;
}

export interface JobFilter {
  search?: string;
  department?: string;
  skill?: string;
  skills?: string[];
  min_budget?: number;
  max_budget?: number;
  pay_type?: JobPayType;
  status?: JobStatus;
  created_by?: string;
  employer_id?: string;
  limit?: number;
  offset?: number;
}

// ==========================================
// Application Models
// ==========================================

export interface ApplicantSummary {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  bio?: string;
  department: string;
  graduation_year: number;
  skills: string[];
  resume_key?: string | null;
  resume_filename?: string | null;
}
export type StudentSummary = ApplicantSummary;

export interface Application {
  id: string;
  job_id: string;
  applicant_id: string;
  student_id: string; // alias for backward-compatibility
  cover_letter: string;
  resume_key?: string | null;
  status: ApplicationStatus;
  created_at: string;
  updated_at: string;
}

export interface ApplicationWithDetails extends Application {
  job?: JobSummary | null;
  applicant?: ApplicantSummary | null;
  student?: ApplicantSummary | null; // alias
  contract?: Contract | null;
}

export interface ApplyRequest {
  cover_letter: string;
  resume_key?: string | null;
}

export interface UpdateApplicationStatusRequest {
  status: "accepted" | "rejected";
}

// ==========================================
// Contract Models
// ==========================================

export interface MemberSummary {
  id: string;
  email: string;
  first_name: string;
  last_name: string;
  department?: string;
  graduation_year?: number;
  company_or_org?: string;
  contact_name?: string;
}
export type EmployerSummary = MemberSummary;

export interface Contract {
  id: string;
  job_id: string;
  application_id: string;
  client_id: string;
  freelancer_id: string;
  employer_id?: string; // alias
  student_id?: string; // alias
  agreed_budget: number;
  status: ContractStatus;
  started_at?: string | null;
  completed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ContractWithDetails extends Contract {
  job?: JobSummary | null;
  client?: MemberSummary | null;
  freelancer?: MemberSummary | null;
  employer?: MemberSummary | null; // alias
  student?: MemberSummary | null; // alias
}

export interface UpdateContractStatusRequest {
  status: ContractStatus;
}

// ==========================================
// Review Models
// ==========================================

export interface ReviewUserSummary {
  id: string;
  first_name: string;
  last_name: string;
  role: string;
}

export interface Review {
  id: string;
  contract_id: string;
  reviewer_id: string;
  reviewee_id: string;
  rating: number; // 1 to 5
  comment: string;
  created_at: string;
  reviewer?: ReviewUserSummary | null;
  reviewee?: ReviewUserSummary | null;
}

export interface CreateReviewRequest {
  rating: number; // 1 to 5
  comment: string;
}

export interface UserReviewSummary {
  user_id: string;
  average_rating: number;
  review_count: number;
  reviews: Review[];
}
