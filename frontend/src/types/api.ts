/**
 * Lynk Shared API Types
 * Strictly mirrors Go backend JSON models, envelopes, and request payloads.
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

export type UserRole = "student" | "employer" | "admin";

export type JobPayType = "fixed" | "hourly";

export type JobStatus = "open" | "in_progress" | "closed" | "cancelled";

export type ApplicationStatus = "pending" | "accepted" | "rejected";

export type ContractStatus = "draft" | "active" | "completed" | "cancelled";

// ==========================================
// User & Profile Models
// ==========================================

export interface User {
  id: string; // UUID
  email: string;
  role: UserRole;
  created_at: string; // ISO 8601 string
  updated_at: string; // ISO 8601 string
}

export interface StudentProfile {
  id: string;
  user_id: string;
  first_name: string;
  last_name: string;
  bio: string;
  department: string;
  graduation_year: number;
  skills: string[];
  portfolio_links: string[];
  resume_key?: string | null;
  resume_filename?: string | null;
  resume_byte_size: number;
  updated_at: string;
}

export interface EmployerProfile {
  id: string;
  user_id: string;
  company_or_org: string;
  contact_name: string;
  description: string;
  website: string;
  updated_at: string;
}

export interface UserProfileSummary {
  user: User | null;
  email_verified: boolean;
  student_profile?: StudentProfile | null;
  employer_profile?: EmployerProfile | null;
}

export interface UpdateStudentProfileRequest {
  first_name: string;
  last_name: string;
  bio: string;
  department: string;
  graduation_year: number;
  skills: string[];
  portfolio_links: string[];
}

export interface UpdateEmployerProfileRequest {
  company_or_org: string;
  contact_name: string;
  description: string;
  website: string;
}

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

export interface EmployerInfo {
  id: string;
  email: string;
  company_or_org: string;
  contact_name: string;
  website: string;
}

export interface Job {
  id: string;
  employer_id: string;
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
  employer?: EmployerInfo | null;
}

export interface JobSummary {
  id: string;
  employer_id: string;
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
  employer_id?: string;
  limit?: number;
  offset?: number;
}

// ==========================================
// Application Models
// ==========================================

export interface StudentSummary {
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

export interface Application {
  id: string;
  job_id: string;
  student_id: string;
  cover_letter: string;
  resume_key?: string | null;
  status: ApplicationStatus;
  created_at: string;
  updated_at: string;
}

export interface ApplicationWithDetails extends Application {
  job?: JobSummary | null;
  student?: StudentSummary | null;
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

export interface EmployerSummary {
  id: string;
  email: string;
  company_or_org: string;
  contact_name: string;
}

export interface Contract {
  id: string;
  job_id: string;
  application_id: string;
  employer_id: string;
  student_id: string;
  agreed_budget: number;
  status: ContractStatus;
  started_at?: string | null;
  completed_at?: string | null;
  created_at: string;
  updated_at: string;
}

export interface ContractWithDetails extends Contract {
  job?: JobSummary | null;
  employer?: EmployerSummary | null;
  student?: StudentSummary | null;
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
