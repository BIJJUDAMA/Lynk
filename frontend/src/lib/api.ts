/**
 * Lynk Typed API Client & Domain Function Library
 *
 * Provides a robust HTTP client wrapper for the Go REST API:
 * - Handles standard JSON envelope unwrapping ({ success, data, error }).
 * - Injects Bearer JWT tokens via dynamic token providers or localStorage fallback.
 * - Extracts ApiClientError with standard codes (including EMAIL_NOT_VERIFIED and UNAUTHORIZED).
 * - Implements typed domain API functions for Auth, Unified Profiles, Jobs, Applications, Contracts, and Reviews.
 */

import type {
  ApiResponse,
  User,
  Profile,
  StudentProfile,
  EmployerProfile,
  UserProfileSummary,
  UpdateProfileRequest,
  UpdateStudentProfileRequest,
  UpdateEmployerProfileRequest,
  SyncUserRequest,
  ResumeUploadResponse,
  ResumeDownloadResponse,
  Job,
  CreateJobRequest,
  UpdateJobRequest,
  JobFilter,
  Application,
  ApplicationWithDetails,
  ApplyRequest,
  UpdateApplicationStatusRequest,
  Contract,
  ContractWithDetails,
  UpdateContractStatusRequest,
  ContractStatus,
  Review,
  CreateReviewRequest,
  UserReviewSummary,
} from "@/types/api";
import { getStoredTokens } from "./auth.ts";
import { Session } from "./supertokens.ts";

// ============================================================================
// Configuration & Constants
// ============================================================================

const rawApiUrl = (process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080").replace(/\/+$/, "");
export const DEFAULT_API_BASE_URL = rawApiUrl.endsWith("/api/v1")
  ? rawApiUrl
  : `${rawApiUrl}/api/v1`;

export type TokenProvider = () => Promise<string | null> | string | null;

export interface ApiClientConfig {
  baseUrl?: string;
  getToken?: TokenProvider;
}

// ============================================================================
// ApiClientError
// ============================================================================

export class ApiClientError extends Error {
  readonly code: string;
  readonly status: number;
  readonly details?: unknown;

  constructor(
    message: string,
    code = "API_ERROR",
    status = 500,
    details?: unknown
  ) {
    super(message);
    this.name = "ApiClientError";
    this.code = code;
    this.status = status;
    this.details = details;

    Object.setPrototypeOf(this, ApiClientError.prototype);
  }

  get isUnauthorized(): boolean {
    return this.code === "UNAUTHORIZED" || this.status === 401;
  }

  get isEmailNotVerified(): boolean {
    return (
      this.code === "EMAIL_NOT_VERIFIED" ||
      (this.status === 403 && this.code.includes("EMAIL"))
    );
  }

  get isForbidden(): boolean {
    return this.code === "FORBIDDEN" || this.status === 403;
  }

  get isNotFound(): boolean {
    return (
      this.code === "NOT_FOUND" ||
      this.code.endsWith("_NOT_FOUND") ||
      this.status === 404
    );
  }
}

export function isEmailNotVerifiedError(error: unknown): boolean {
  if (error instanceof ApiClientError) {
    return error.isEmailNotVerified;
  }
  if (error && typeof error === "object" && "code" in error) {
    return (error as { code: string }).code === "EMAIL_NOT_VERIFIED";
  }
  return false;
}

export function isUnauthorizedError(error: unknown): boolean {
  if (error instanceof ApiClientError) {
    return error.isUnauthorized;
  }
  if (error && typeof error === "object" && "code" in error) {
    return (error as { code: string }).code === "UNAUTHORIZED";
  }
  return false;
}

// ============================================================================
// ApiClient Interface & Factory
// ============================================================================

export interface ApiClient {
  get<T>(path: string, params?: Record<string, unknown>): Promise<T>;
  post<T>(path: string, body?: unknown): Promise<T>;
  put<T>(path: string, body?: unknown): Promise<T>;
  patch<T>(path: string, body?: unknown): Promise<T>;
  delete<T>(path: string): Promise<T>;
  uploadFile<T>(path: string, file: File, fieldName?: string): Promise<T>;
}

export function buildQueryString(params?: Record<string, unknown>): string {
  if (!params) return "";
  const searchParams = new URLSearchParams();

  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    if (Array.isArray(value)) {
      for (const item of value) {
        if (item !== undefined && item !== null) {
          searchParams.append(key, String(item));
        }
      }
    } else {
      searchParams.set(key, String(value));
    }
  }

  const query = searchParams.toString();
  return query ? `?${query}` : "";
}

export function createApiClient(
  tokenOrConfig?: TokenProvider | ApiClientConfig,
  customBaseUrl?: string
): ApiClient {
  let getToken: TokenProvider | undefined;
  let baseUrl: string = DEFAULT_API_BASE_URL;

  if (typeof tokenOrConfig === "function") {
    getToken = tokenOrConfig;
    if (customBaseUrl) {
      baseUrl = customBaseUrl;
    }
  } else if (typeof tokenOrConfig === "object" && tokenOrConfig !== null) {
    getToken = tokenOrConfig.getToken;
    baseUrl = tokenOrConfig.baseUrl || customBaseUrl || DEFAULT_API_BASE_URL;
  } else if (customBaseUrl) {
    baseUrl = customBaseUrl;
  }

  const normalizedBaseUrl = baseUrl.replace(/\/+$/, "");

  async function resolveHeaders(
    customHeaders?: HeadersInit
  ): Promise<Record<string, string>> {
    const headers: Record<string, string> = {
      Accept: "application/json",
    };

    let token: string | null = null;
    if (getToken) {
      try {
        token = await getToken();
      } catch {
        token = null;
      }
    }

    if (!token && typeof window !== "undefined") {
      try {
        if (await Session.doesSessionExist()) {
          token = (await Session.getAccessToken()) ?? null;
        }
      } catch {
        // Fallback or ignore if SuperTokens is uninitialized
      }
      if (!token) {
        token = getStoredTokens()?.accessToken ?? null;
      }
    }

    if (token) {
      headers["Authorization"] = `Bearer ${token}`;
    }

    if (customHeaders) {
      if (customHeaders instanceof Headers) {
        customHeaders.forEach((val, key) => {
          headers[key] = val;
        });
      } else if (Array.isArray(customHeaders)) {
        for (const [key, val] of customHeaders) {
          headers[key] = val;
        }
      } else {
        Object.assign(headers, customHeaders);
      }
    }

    return headers;
  }

  async function executeRequest<T>(
    path: string,
    options: RequestInit & { params?: Record<string, unknown> } = {}
  ): Promise<T> {
    const normalizedPath = path.startsWith("/") ? path : `/${path}`;
    const queryString = buildQueryString(options.params);
    let fullUrl: string;
    if (queryString) {
      if (normalizedPath.includes("?")) {
        fullUrl = `${normalizedBaseUrl}${normalizedPath}&${queryString.replace(/^\?/, "")}`;
      } else {
        fullUrl = `${normalizedBaseUrl}${normalizedPath}${queryString}`;
      }
    } else {
      fullUrl = `${normalizedBaseUrl}${normalizedPath}`;
    }

    const headers = await resolveHeaders(options.headers);

    if (
      options.body &&
      !(options.body instanceof FormData) &&
      !headers["Content-Type"]
    ) {
      headers["Content-Type"] = "application/json";
    }

    let response: Response;
    try {
      response = await fetch(fullUrl, {
        ...options,
        credentials: options.credentials !== undefined ? options.credentials : "include",
        headers,
      });
    } catch (err: unknown) {
      const msg =
        err instanceof Error
          ? err.message
          : "Network request failed or server unreachable";
      throw new ApiClientError(msg, "NETWORK_ERROR", 0, err);
    }

    if (response.status === 204) {
      return undefined as unknown as T;
    }

    const contentType = response.headers.get("content-type") || "";
    const isJson = contentType.includes("application/json");

    if (!isJson) {
      const text = await response.text().catch(() => "");
      if (!response.ok) {
        const defaultCode =
          response.status === 401
            ? "UNAUTHORIZED"
            : response.status === 403
            ? "FORBIDDEN"
            : response.status === 404
            ? "NOT_FOUND"
            : "API_ERROR";
        throw new ApiClientError(
          text || response.statusText || "Request failed",
          defaultCode,
          response.status,
          { rawText: text }
        );
      }
      return text as unknown as T;
    }

    let parsed: ApiResponse<T> | unknown;
    try {
      parsed = await response.json();
    } catch (parseErr) {
      throw new ApiClientError(
        "Failed to parse server JSON response",
        "MALFORMED_RESPONSE",
        response.status,
        parseErr
      );
    }

    const envelope = parsed as ApiResponse<T>;
    const isSuccessEnvelope =
      envelope && typeof envelope === "object" && envelope.success === true;
    const isErrorEnvelope =
      envelope && typeof envelope === "object" && envelope.success === false;

    if (!response.ok || isErrorEnvelope) {
      const errorObj = isErrorEnvelope ? envelope.error : null;
      let code = errorObj?.code;

      if (!code) {
        if (response.status === 401) {
          code = "UNAUTHORIZED";
        } else if (response.status === 403) {
          code = "FORBIDDEN";
        } else if (response.status === 404) {
          code = "NOT_FOUND";
        } else if (response.status >= 500) {
          code = "INTERNAL_ERROR";
        } else {
          code = "API_ERROR";
        }
      }

      const message =
        errorObj?.message || response.statusText || "Request failed";

      throw new ApiClientError(message, code, response.status, errorObj ?? parsed);
    }

    if (isSuccessEnvelope && "data" in envelope) {
      return envelope.data as T;
    }

    return parsed as T;
  }

  return {
    get<T>(path: string, params?: Record<string, unknown>): Promise<T> {
      return executeRequest<T>(path, { method: "GET", params });
    },

    post<T>(path: string, body?: unknown): Promise<T> {
      return executeRequest<T>(path, {
        method: "POST",
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });
    },

    put<T>(path: string, body?: unknown): Promise<T> {
      return executeRequest<T>(path, {
        method: "PUT",
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });
    },

    patch<T>(path: string, body?: unknown): Promise<T> {
      return executeRequest<T>(path, {
        method: "PATCH",
        body: body !== undefined ? JSON.stringify(body) : undefined,
      });
    },

    delete<T>(path: string): Promise<T> {
      return executeRequest<T>(path, { method: "DELETE" });
    },

    uploadFile<T>(
      path: string,
      file: File,
      fieldName = "resume"
    ): Promise<T> {
      const formData = new FormData();
      formData.append(fieldName, file, file.name);
      return executeRequest<T>(path, {
        method: "POST",
        body: formData,
      });
    },
  };
}

export const apiClient: ApiClient = createApiClient();
export default apiClient;

// ============================================================================
// Typed Domain Functions: Auth & Unified Profiles
// ============================================================================

export async function syncUser(
  data?: SyncUserRequest,
  client: ApiClient = apiClient
): Promise<User> {
  return client.post<User>("/auth/sync", data);
}

export async function getMe(
  client: ApiClient = apiClient
): Promise<UserProfileSummary> {
  return client.get<UserProfileSummary>("/auth/me");
}

/**
 * Retrieves the current authenticated campus member's profile.
 */
export async function getMyProfile(
  client: ApiClient = apiClient
): Promise<Profile> {
  return client.get<Profile>("/profile/me");
}

export interface BuildProfilePayloadInput {
  firstName: string;
  lastName: string;
  department: string;
  customDepartment?: string;
  graduationYear?: number | string | null;
  bio: string;
  skills: string[];
  portfolioLinks: string[];
  organization: string;
  validatedOrgWebsite: string;
}

/**
 * Builds an UpdateProfileRequest payload ensuring optional fields retain empty strings
 * or 0 rather than undefined, allowing users to clear existing values in PostgreSQL.
 */
export function buildProfileUpdatePayload(
  input: BuildProfilePayloadInput
): UpdateProfileRequest {
  const effectiveDepartment =
    input.department === "Other"
      ? (input.customDepartment || "").trim()
      : input.department.trim();

  return {
    first_name: input.firstName.trim(),
    last_name: input.lastName.trim(),
    department: effectiveDepartment,
    graduation_year: input.graduationYear ? Number(input.graduationYear) : 0,
    bio: input.bio.trim(),
    skills: input.skills,
    portfolio_links: input.portfolioLinks.filter((l) => l.trim() !== ""),
    organization: input.organization.trim(),
    organization_website: input.validatedOrgWebsite,
  };
}

/**
 * Updates the current authenticated campus member's profile.
 */
export async function updateMyProfile(
  data: UpdateProfileRequest,
  client: ApiClient = apiClient
): Promise<Profile> {
  return client.put<Profile>("/profile/me", data);
}

/**
 * Retrieves public view of a campus member's profile by ID.
 */
export async function getProfileById(
  id: string,
  client: ApiClient = apiClient
): Promise<Profile> {
  return client.get<Profile>(`/profile/${id}`);
}

// Backward-compatible aliases
export async function getStudentProfile(
  client: ApiClient = apiClient
): Promise<StudentProfile> {
  return getMyProfile(client);
}

export async function updateStudentProfile(
  data: UpdateStudentProfileRequest,
  client: ApiClient = apiClient
): Promise<StudentProfile> {
  return updateMyProfile(data, client);
}

export async function getEmployerProfile(
  client: ApiClient = apiClient
): Promise<EmployerProfile> {
  return getMyProfile(client);
}

export async function updateEmployerProfile(
  data: UpdateEmployerProfileRequest,
  client: ApiClient = apiClient
): Promise<EmployerProfile> {
  return updateMyProfile(data, client);
}

export async function getStudentProfileById(
  studentId: string,
  client: ApiClient = apiClient
): Promise<StudentProfile> {
  return getProfileById(studentId, client);
}

/**
 * Uploads resume PDF/DOCX to MinIO object storage.
 * Enforces verified .edu email and max 5MB size limit.
 */
export async function uploadResume(
  file: File,
  client: ApiClient = apiClient
): Promise<ResumeUploadResponse> {
  return client.uploadFile<ResumeUploadResponse>(
    "/profile/resume",
    file,
    "resume"
  );
}

/**
 * Generates a 15-minute presigned download URL for the authenticated member's resume.
 */
export async function getMyResumeUrl(
  client: ApiClient = apiClient
): Promise<ResumeDownloadResponse> {
  return client.get<ResumeDownloadResponse>("/profile/resume");
}

/**
 * Generates a 15-minute presigned download URL for a specific member's resume.
 */
export async function getResumeUrl(
  memberId: string,
  client: ApiClient = apiClient
): Promise<ResumeDownloadResponse> {
  return client.get<ResumeDownloadResponse>(`/profile/${memberId}/resume`);
}

export async function getStudentResumeUrl(
  studentId: string,
  client: ApiClient = apiClient
): Promise<ResumeDownloadResponse> {
  return getResumeUrl(studentId, client);
}

// ============================================================================
// Typed Domain Functions: Jobs
// ============================================================================

export async function listJobs(
  filters?: JobFilter,
  client: ApiClient = apiClient
): Promise<Job[]> {
  const queryParams: Record<string, unknown> = {};

  if (filters) {
    if (filters.search) queryParams.search = filters.search;
    if (filters.department) queryParams.department = filters.department;
    if (filters.skill) {
      queryParams.skill = filters.skill;
    } else if (filters.skills && filters.skills.length > 0) {
      queryParams.skill = filters.skills.join(",");
    }
    if (filters.status) queryParams.status = filters.status;
    if (filters.limit !== undefined) queryParams.limit = filters.limit;
    if (filters.offset !== undefined) queryParams.offset = filters.offset;
  }

  return client.get<Job[]>("/jobs", queryParams);
}

export async function getMyJobs(
  client: ApiClient = apiClient
): Promise<Job[]> {
  return client.get<Job[]>("/jobs/mine");
}

export async function getJobById(
  id: string,
  client: ApiClient = apiClient
): Promise<Job> {
  return client.get<Job>(`/jobs/${id}`);
}

export async function createJob(
  data: CreateJobRequest,
  client: ApiClient = apiClient
): Promise<Job> {
  return client.post<Job>("/jobs", data);
}

export async function updateJob(
  id: string,
  data: UpdateJobRequest,
  client: ApiClient = apiClient
): Promise<Job> {
  return client.put<Job>(`/jobs/${id}`, data);
}

export async function deleteJob(
  id: string,
  client: ApiClient = apiClient
): Promise<{ message?: string } | void> {
  return client.delete<{ message?: string }>(`/jobs/${id}`);
}

// ============================================================================
// Typed Domain Functions: Applications
// ============================================================================

export async function applyToJob(
  jobId: string,
  data: ApplyRequest,
  client: ApiClient = apiClient
): Promise<Application> {
  return client.post<Application>(`/jobs/${jobId}/applications`, data);
}

export async function listJobApplications(
  jobId: string,
  client: ApiClient = apiClient
): Promise<ApplicationWithDetails[]> {
  return client.get<ApplicationWithDetails[]>(`/jobs/${jobId}/applications`);
}

export async function getMyApplications(
  client: ApiClient = apiClient
): Promise<ApplicationWithDetails[]> {
  return client.get<ApplicationWithDetails[]>("/applications/mine");
}

export async function getMyApplicationForJob(
  jobId: string,
  client: ApiClient = apiClient
): Promise<ApplicationWithDetails | null> {
  const data = await client.get<ApplicationWithDetails | null>(
    `/applications/applied?job_id=${encodeURIComponent(jobId)}`
  );
  return data ?? null;
}

export async function getApplicationById(
  id: string,
  client: ApiClient = apiClient
): Promise<ApplicationWithDetails> {
  return client.get<ApplicationWithDetails>(`/applications/${id}`);
}

export async function updateApplicationStatus(
  id: string,
  status: "accepted" | "rejected" | UpdateApplicationStatusRequest,
  client: ApiClient = apiClient
): Promise<Application> {
  const payload = typeof status === "string" ? { status } : status;
  return client.patch<Application>(`/applications/${id}/status`, payload);
}

// ============================================================================
// Typed Domain Functions: Contracts
// ============================================================================

export async function listContracts(
  client: ApiClient = apiClient
): Promise<ContractWithDetails[]> {
  return client.get<ContractWithDetails[]>("/contracts");
}

export async function getContractById(
  id: string,
  client: ApiClient = apiClient
): Promise<ContractWithDetails> {
  return client.get<ContractWithDetails>(`/contracts/${id}`);
}

export async function updateContractStatus(
  id: string,
  status: ContractStatus | UpdateContractStatusRequest,
  client: ApiClient = apiClient
): Promise<Contract> {
  const payload = typeof status === "string" ? { status } : status;
  return client.patch<Contract>(`/contracts/${id}/status`, payload);
}

// ============================================================================
// Typed Domain Functions: Reviews & Ratings
// ============================================================================

export async function getContractReviews(
  contractId: string,
  client: ApiClient = apiClient
): Promise<Review[]> {
  return client.get<Review[]>(`/contracts/${contractId}/reviews`);
}

export async function getUserReviews(
  userId: string,
  client: ApiClient = apiClient
): Promise<UserReviewSummary> {
  return client.get<UserReviewSummary>(`/users/${userId}/reviews`);
}

export async function createReview(
  contractId: string,
  data: CreateReviewRequest,
  client: ApiClient = apiClient
): Promise<Review> {
  return client.post<Review>(`/contracts/${contractId}/reviews`, data);
}
