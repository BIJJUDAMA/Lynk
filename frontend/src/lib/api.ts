/**
 * Lynk Typed API Client & Domain Function Library
 *
 * Provides a robust HTTP client wrapper for the Go REST API:
 * - Handles standard JSON envelope unwrapping ({ success, data, error }).
 * - Injects Bearer JWT tokens via dynamic token providers or localStorage fallback.
 * - Extracts ApiClientError with standard codes (including EMAIL_NOT_VERIFIED and UNAUTHORIZED).
 * - Implements typed domain API functions for Auth, Profiles, Jobs, Applications, Contracts, and Reviews.
 */

import type {
  ApiResponse,
  User,
  StudentProfile,
  EmployerProfile,
  UserProfileSummary,
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

// ============================================================================
// Configuration & Constants
// ============================================================================

export const DEFAULT_API_BASE_URL =
  process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080/api/v1";

export type TokenProvider = () => Promise<string | null> | string | null;

export interface ApiClientConfig {
  baseUrl?: string;
  getToken?: TokenProvider;
}

// ============================================================================
// ApiClientError
// ============================================================================

/**
 * ApiClientError represents an error response from the Lynk REST API or underlying transport.
 * Normalizes error codes, status codes, and provides helper getters for common flow gating.
 */
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

    // Restore prototype chain for instanceof checks
    Object.setPrototypeOf(this, ApiClientError.prototype);
  }

  /**
   * True if error is due to missing or expired authentication credentials.
   */
  get isUnauthorized(): boolean {
    return this.code === "UNAUTHORIZED" || this.status === 401;
  }

  /**
   * True if student attempt was gated because university email is unverified.
   */
  get isEmailNotVerified(): boolean {
    return (
      this.code === "EMAIL_NOT_VERIFIED" ||
      (this.status === 403 && this.code.includes("EMAIL"))
    );
  }

  /**
   * True if authenticated caller lacks required role or ownership permission.
   */
  get isForbidden(): boolean {
    return this.code === "FORBIDDEN" || this.status === 403;
  }

  /**
   * True if requested resource was not found.
   */
  get isNotFound(): boolean {
    return (
      this.code === "NOT_FOUND" ||
      this.code.endsWith("_NOT_FOUND") ||
      this.status === 404
    );
  }
}

/**
 * Helper to determine if an error was caused by unverified institutional email.
 */
export function isEmailNotVerifiedError(error: unknown): boolean {
  if (error instanceof ApiClientError) {
    return error.isEmailNotVerified;
  }
  if (error && typeof error === "object" && "code" in error) {
    return (error as { code: string }).code === "EMAIL_NOT_VERIFIED";
  }
  return false;
}

/**
 * Helper to determine if an error was caused by missing or invalid authentication.
 */
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

/**
 * Builds a query string safely from an arbitrary parameters object.
 */
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

/**
 * Creates an ApiClient instance with configurable baseUrl and token provider.
 */
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

  // Normalize base URL without trailing slash
  const normalizedBaseUrl = baseUrl.replace(/\/+$/, "");

  /**
   * Resolves authentication headers including Bearer token if present.
   */
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

    // Fallback to client-side localStorage stored token if not supplied
    if (!token && typeof window !== "undefined") {
      token = getStoredTokens()?.accessToken ?? null;
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

  /**
   * Executes HTTP request and handles JSON envelope decoding and error mapping.
   */
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

    // Apply JSON content-type if body is JSON string and Content-Type not explicitly set
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
        headers,
      });
    } catch (err: unknown) {
      const msg =
        err instanceof Error
          ? err.message
          : "Network request failed or server unreachable";
      throw new ApiClientError(msg, "NETWORK_ERROR", 0, err);
    }

    // 204 No Content
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

    // Check envelope failure or HTTP status failure
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

    // Unpack success envelope
    if (isSuccessEnvelope && "data" in envelope) {
      return envelope.data as T;
    }

    // Fallback if returned JSON was not in envelope format
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

/**
 * Default singleton API client utilizing stored localStorage tokens.
 */
export const apiClient: ApiClient = createApiClient();
export default apiClient;

// ============================================================================
// Typed Domain Functions: Auth & User Profiles
// ============================================================================

/**
 * Synchronizes user with Go backend PostgreSQL database and creates initial profile record.
 */
export async function syncUser(
  data?: SyncUserRequest,
  client: ApiClient = apiClient
): Promise<User> {
  return client.post<User>("/auth/sync", data);
}

/**
 * Retrieves the current authenticated user's profile summary (user info, verification, student/employer profiles).
 */
export async function getMe(
  client: ApiClient = apiClient
): Promise<UserProfileSummary> {
  return client.get<UserProfileSummary>("/auth/me");
}

/**
 * Retrieves authenticated student's profile.
 */
export async function getStudentProfile(
  client: ApiClient = apiClient
): Promise<StudentProfile> {
  return client.get<StudentProfile>("/profile/student");
}

/**
 * Updates authenticated student's profile information (bio, skills, department, graduation year, portfolio links).
 */
export async function updateStudentProfile(
  data: UpdateStudentProfileRequest,
  client: ApiClient = apiClient
): Promise<StudentProfile> {
  return client.put<StudentProfile>("/profile/student", data);
}

/**
 * Retrieves authenticated employer's profile.
 */
export async function getEmployerProfile(
  client: ApiClient = apiClient
): Promise<EmployerProfile> {
  return client.get<EmployerProfile>("/profile/employer");
}

/**
 * Updates authenticated employer's profile information (company name, contact name, description, website).
 */
export async function updateEmployerProfile(
  data: UpdateEmployerProfileRequest,
  client: ApiClient = apiClient
): Promise<EmployerProfile> {
  return client.put<EmployerProfile>("/profile/employer", data);
}

/**
 * Uploads student resume PDF/DOCX to MinIO object storage.
 * Enforces verified .edu email and max 5MB size limit.
 */
export async function uploadResume(
  file: File,
  client: ApiClient = apiClient
): Promise<ResumeUploadResponse> {
  return client.uploadFile<ResumeUploadResponse>(
    "/profile/student/resume",
    file,
    "resume"
  );
}

/**
 * Generates a 15-minute presigned download URL for the authenticated student's resume.
 */
export async function getMyResumeUrl(
  client: ApiClient = apiClient
): Promise<ResumeDownloadResponse> {
  return client.get<ResumeDownloadResponse>("/profile/student/resume");
}

/**
 * Generates a 15-minute presigned download URL for a specific student's resume (employers/admins/owner).
 */
export async function getStudentResumeUrl(
  studentId: string,
  client: ApiClient = apiClient
): Promise<ResumeDownloadResponse> {
  return client.get<ResumeDownloadResponse>(
    `/profile/student/${studentId}/resume`
  );
}

/**
 * Retrieves public/employer view of a student profile by student UUID.
 */
export async function getStudentProfileById(
  studentId: string,
  client: ApiClient = apiClient
): Promise<StudentProfile> {
  return client.get<StudentProfile>(`/profile/student/${studentId}`);
}

// ============================================================================
// Typed Domain Functions: Jobs
// ============================================================================

/**
 * Lists jobs with optional filtering (search, department, skills, budget, status, pagination).
 */
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
    if (filters.pay_type) queryParams.pay_type = filters.pay_type;
    if (filters.status) queryParams.status = filters.status;
    if (filters.min_budget !== undefined)
      queryParams.min_budget = filters.min_budget;
    if (filters.max_budget !== undefined)
      queryParams.max_budget = filters.max_budget;
    if (filters.limit !== undefined) queryParams.limit = filters.limit;
    if (filters.offset !== undefined) queryParams.offset = filters.offset;
  }

  return client.get<Job[]>("/jobs", queryParams);
}

/**
 * Retrieves all jobs posted by the authenticated employer.
 */
export async function getMyJobs(
  client: ApiClient = apiClient
): Promise<Job[]> {
  return client.get<Job[]>("/jobs/mine");
}

/**
 * Retrieves public details of a specific job posting by ID.
 */
export async function getJobById(
  id: string,
  client: ApiClient = apiClient
): Promise<Job> {
  return client.get<Job>(`/jobs/${id}`);
}

/**
 * Creates a new job posting (Employer role required).
 */
export async function createJob(
  data: CreateJobRequest,
  client: ApiClient = apiClient
): Promise<Job> {
  return client.post<Job>("/jobs", data);
}

/**
 * Updates an existing job posting (Employer owner required).
 */
export async function updateJob(
  id: string,
  data: UpdateJobRequest,
  client: ApiClient = apiClient
): Promise<Job> {
  return client.put<Job>(`/jobs/${id}`, data);
}

/**
 * Cancels or deletes a job posting (Employer owner required).
 */
export async function deleteJob(
  id: string,
  client: ApiClient = apiClient
): Promise<{ message?: string } | void> {
  return client.delete<{ message?: string }>(`/jobs/${id}`);
}

// ============================================================================
// Typed Domain Functions: Applications
// ============================================================================

/**
 * Submits an application for an open job (Student role and verified .edu email required).
 */
export async function applyToJob(
  jobId: string,
  data: ApplyRequest,
  client: ApiClient = apiClient
): Promise<Application> {
  return client.post<Application>(`/jobs/${jobId}/applications`, data);
}

/**
 * Lists all applications submitted to a specific job (Job owner employer only).
 */
export async function listJobApplications(
  jobId: string,
  client: ApiClient = apiClient
): Promise<ApplicationWithDetails[]> {
  return client.get<ApplicationWithDetails[]>(`/jobs/${jobId}/applications`);
}

/**
 * Lists all applications submitted by the authenticated student.
 */
export async function getMyApplications(
  client: ApiClient = apiClient
): Promise<ApplicationWithDetails[]> {
  return client.get<ApplicationWithDetails[]>("/applications/mine");
}

/**
 * Retrieves application details by application ID (Applicant student or Job owner employer).
 */
export async function getApplicationById(
  id: string,
  client: ApiClient = apiClient
): Promise<ApplicationWithDetails> {
  return client.get<ApplicationWithDetails>(`/applications/${id}`);
}

/**
 * Updates an application's status to accepted or rejected (Job owner employer only).
 * Accepting an application automatically generates an Active Contract.
 */
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

/**
 * Lists all contracts involving the authenticated student or employer.
 */
export async function listContracts(
  client: ApiClient = apiClient
): Promise<ContractWithDetails[]> {
  return client.get<ContractWithDetails[]>("/contracts");
}

/**
 * Retrieves contract details by ID (Participant student or employer).
 */
export async function getContractById(
  id: string,
  client: ApiClient = apiClient
): Promise<ContractWithDetails> {
  return client.get<ContractWithDetails>(`/contracts/${id}`);
}

/**
 * Transitions contract state machine (draft -> active -> completed | cancelled).
 */
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

/**
 * Retrieves public reviews submitted on a completed contract.
 */
export async function getContractReviews(
  contractId: string,
  client: ApiClient = apiClient
): Promise<Review[]> {
  return client.get<Review[]>(`/contracts/${contractId}/reviews`);
}

/**
 * Retrieves public review summary and list of reviews received by a user.
 */
export async function getUserReviews(
  userId: string,
  client: ApiClient = apiClient
): Promise<UserReviewSummary> {
  return client.get<UserReviewSummary>(`/users/${userId}/reviews`);
}

/**
 * Submits a 1-5 star review and comment for a completed contract (Contract participant required).
 */
export async function createReview(
  contractId: string,
  data: CreateReviewRequest,
  client: ApiClient = apiClient
): Promise<Review> {
  return client.post<Review>(`/contracts/${contractId}/reviews`, data);
}
