# Complete REST API Specifications - Lynk

> **Authoritative HTTP API Specification**  
> **Base URL:** `http://localhost:8080/api/v1`  
> **Content-Type:** `application/json` (or `multipart/form-data` for resume upload)  
> **Authentication:** SuperTokens Session Cookies (`sAccessToken`, `sRefreshToken`) or `Authorization: Bearer <sAccessToken>`  
> **Standard Error Response:** `{"error": "human-readable error description"}`

---

## 1. Global API Conventions & Protocol Standards

### 1.1 Base URL & Path Structure
All endpoints are versioned under the `/api/v1` path prefix:
```text
http://localhost:8080/api/v1/<domain>/<resource>
```

### 1.2 Authentication & Headers
- **Session Cookies:** Frontend web clients automatically include HTTP-only cookies (`sAccessToken`, `sRefreshToken`) via `credentials: "include"`.
- **Bearer Tokens:** API or mobile clients may supply the access token via the standard header:
  ```http
  Authorization: Bearer <sAccessToken>
  ```
- **Anti-CSRF:** When anti-CSRF is enabled by SuperTokens, write operations pass the `anti-csrf` token in the request header.
- **Request ID Tracking:** Every response returns a unique `X-Request-Id` header for end-to-end distributed log tracing.

### 1.3 Standard Error Responses
All error responses adhere to a uniform JSON envelope:
```json
{
  "error": "Detailed error message describing validation failure or rule violation"
}
```

| HTTP Status Code | Meaning | Example Trigger |
| :--- | :--- | :--- |
| `400 Bad Request` | Malformed JSON, validation failure, bounds exceeded | Description > 5,000 chars, non-HTTP link scheme |
| `401 Unauthorized` | Missing or expired session token | Accessing protected endpoint without active session |
| `403 Forbidden` | Campus verification pending or unauthorized resource | Unverified .edu email, non-owner updating job |
| `404 Not Found` | Resource does not exist | Invalid Job ID or Profile ID |
| `409 Conflict` | Unique constraint or state conflict | Applying twice to same job, re-accepting accepted gig |
| `413 Payload Too Large` | Request entity exceeds size limit | JSON payload > 1MB or resume file > 5MB |
| `500 Internal Server Error` | Unhandled system exception | Database failure, S3 connectivity loss |

---

## 2. Authentication Domain (`/auth`)

SuperTokens Core endpoints are mounted under `/api/v1/auth`.

### 2.1 Sign Up
`POST /auth/signup`
Creates a new campus member account with mandatory institutional `.edu` email validation.

- **Auth Required:** No
- **Request Body:**
  ```json
  {
    "email": "student@stanford.edu",
    "password": "SecurePassword123!"
  }
  ```
- **Validation Rules:**
  - `email`: Must be a valid institutional domain ending in `.edu` (`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.edu$`).
  - `password`: Minimum 8 characters with lowercase, uppercase, and digit.
- **Response `200 OK`:**
  ```json
  {
    "status": "OK",
    "user": {
      "id": "st_usr_9a4f21e0-3e21-4f40-b8cf-b0d36746f332",
      "email": "student@stanford.edu",
      "timeJoined": 1726051200000
    }
  }
  ```
- **Response `400 Bad Request`:**
  ```json
  {
    "status": "FIELD_ERROR",
    "formFields": [
      {"id": "email", "error": "Only institutional .edu email addresses are permitted"}
    ]
  }
  ```

### 2.2 Sign In
`POST /auth/signin`
Authenticates existing member credentials and issues dual session cookies (`sAccessToken`, `sRefreshToken`).

- **Auth Required:** No
- **Request Body:**
  ```json
  {
    "email": "student@stanford.edu",
    "password": "SecurePassword123!"
  }
  ```
- **Response `200 OK`:**
  ```json
  {
    "status": "OK",
    "user": {
      "id": "st_usr_9a4f21e0-3e21-4f40-b8cf-b0d36746f332",
      "email": "student@stanford.edu"
    }
  }
  ```

### 2.3 Sign Out
`POST /auth/signout`
Invalidates active session token in SuperTokens Core and clears HTTP-only session cookies.

- **Auth Required:** Yes (`sAccessToken`)
- **Response `200 OK`:**
  ```json
  {
    "status": "OK"
  }
  ```

### 2.4 Current Authenticated User & Claims
`GET /auth/me`
Returns current session claims, user ID, email, role, and dynamic email verification status.

- **Auth Required:** Yes (`sAccessToken`)
- **Response `200 OK`:**
  ```json
  {
    "id": "st_usr_9a4f21e0-3e21-4f40-b8cf-b0d36746f332",
    "email": "student@stanford.edu",
    "role": "member",
    "email_verified": true
  }
  ```

---

## 3. Unified Campus Member Profile Domain (`/profile`)

### 3.1 Get Current User Profile
`GET /profile/me`
Retrieves the unified campus profile for the calling authenticated member.

- **Auth Required:** Yes
- **Response `200 OK`:**
  ```json
  {
    "user_id": "st_usr_9a4f21e0-3e21-4f40-b8cf-b0d36746f332",
    "full_name": "Jane Stanford",
    "department": "Computer Science",
    "major": "Artificial Intelligence",
    "graduation_year": 2026,
    "bio": "Graduate researcher building agentic AI and distributed systems.",
    "skills": ["Go", "TypeScript", "PostgreSQL", "Next.js"],
    "portfolio_links": ["https://github.com/janestanford", "https://janestanford.dev"],
    "organization_name": "Stanford ACM Chapter",
    "organization_website": "https://acm.stanford.edu",
    "resume_key": "resumes/st_usr_9a4f21e0-3e21-4f40-b8cf-b0d36746f332/9b23fa1c.pdf",
    "created_at": "2026-09-01T12:00:00Z",
    "updated_at": "2026-09-11T16:30:00Z"
  }
  ```

### 3.2 Update Current User Profile
`PUT /profile/me`
Updates profile metadata with strict URL scheme sanitization. Preserves empty strings `""` and `0` when explicitly clearing optional fields (F-14).

- **Auth Required:** Yes
- **Request Body:**
  ```json
  {
    "full_name": "Jane Stanford",
    "department": "Computer Science",
    "major": "Artificial Intelligence",
    "graduation_year": 2026,
    "bio": "Updated researcher biography.",
    "skills": ["Go", "TypeScript", "PostgreSQL", "Next.js", "Docker"],
    "portfolio_links": ["https://github.com/janestanford"],
    "organization_name": "",
    "organization_website": ""
  }
  ```
- **Validation Invariants:**
  - `portfolio_links`: Each link must start with `http://` or `https://`. Rejects `javascript:`, `data:`, or relative URIs (F-06).
  - `organization_website`: Must start with `http://` or `https://` if non-empty (F-06).
  - `graduation_year`: Must be between 2020 and 2040 (or 0 to clear).
- **Response `200 OK`:** Returns updated Profile JSON object.

### 3.3 Get Public Member Profile
`GET /profile/{id}`
Retrieves sanitized public profile of any verified campus member.

- **Auth Required:** No
- **Response `200 OK`:** Returns Profile JSON object (excluding sensitive contact metadata).
- **Response `404 Not Found`:** `{"error": "Profile not found"}`

### 3.4 Upload Student Resume
`POST /profile/resume`
Streams a PDF or DOCX resume to MinIO S3. S3 upload streams *before* database advisory lock acquisition to prevent thread pool exhaustion (F-02, F-13).

- **Auth Required:** Yes (Must have `email_verified: true`)
- **Content-Type:** `multipart/form-data`
- **Form Field:** `resume` (Binary file)
- **Constraints:**
  - Max Size: 5MB (`5 * 1024 * 1024` bytes).
  - Allowed MIME Types: `application/pdf`, `application/msword`, `application/vnd.openxmlformats-officedocument.wordprocessingml.document`.
- **Response `200 OK`:**
  ```json
  {
    "resume_key": "resumes/st_usr_9a4f21e0/4c1b9f8d-7a6e.pdf",
    "file_name": "Jane_Stanford_Resume_2026.pdf",
    "file_size": 184320,
    "mime_type": "application/pdf",
    "uploaded_at": "2026-09-11T17:00:00Z"
  }
  ```

### 3.5 Download / Stream Resume
`GET /profile/resume`
Streams the caller's resume directly or generates a short-lived pre-signed download URL (15-minute TTL).

- **Auth Required:** Yes
- **Response `200 OK`:** Binary stream with `Content-Type: application/pdf` and `Content-Disposition: inline; filename="..."`.

---

## 4. Jobs & Opportunities Domain (`/jobs`)

### 4.1 Search & Filter Jobs
`GET /jobs`
Lists open campus opportunities with optional multi-criteria query parameters.

- **Query Parameters:**
  - `search` (string): Keyword matching across title and description (trigram-accelerated).
  - `department` (string): Academic department filter.
  - `skills` (string): Comma-separated required skill tags.
  - `status` (string): Filter by status (`open`, `closed`, `completed`, `cancelled`). Default: `open`.
  - `page` / `limit` (integer): Pagination controls (default limit: 20, max: 100).
- **Response `200 OK`:**
  ```json
  {
    "jobs": [
      {
        "id": "7b6f284e-3c29-4d6b-8fa2-1a4c8e7f9012",
        "created_by": "st_usr_9a4f21e0",
        "title": "Full-Stack Engineer for Autonomous Lab Drone System",
        "description": "Building telemetry dashboard in Next.js with Go backend.",
        "required_skills": ["Go", "Next.js", "Tailwind CSS"],
        "department": "Aeronautics & Astronautics",
        "status": "open",
        "deadline": "2026-10-15T00:00:00Z",
        "created_at": "2026-09-10T08:00:00Z"
      }
    ],
    "total": 1,
    "page": 1,
    "limit": 20
  }
  ```

### 4.2 Create Job Opportunity
`POST /jobs`
Publishes a new campus opportunity. Gated behind institutional email verification. Emphasizes opportunity discovery, academic department context, required skills, and task deadline.

- **Auth Required:** Yes (Must have `email_verified: true`)
- **Request Body:**
  ```json
  {
    "title": "Computer Vision Pipeline for Cell Morphology",
    "description": "Develop Python/OpenCV analysis pipeline for biology research lab.",
    "required_skills": ["Python", "OpenCV", "Machine Learning"],
    "department": "Bioengineering",
    "deadline": "2026-09-30T23:59:59Z"
  }
  ```
- **Validation Bounds (F-10, F-25):**
  - `title`: 5 to 255 characters.
  - `description`: Up to 5,000 characters (strictly capped).
  - `required_skills`: Capped at 25 skills, each skill truncated to 50 characters, deduplicated case-insensitively.
  - `deadline`: Normalized to UTC calendar day (`CalendarDayUTC`). Same-day deadlines are accepted; strictly past calendar days rejected.
  - `department`: Optional academic department string (up to 128 characters).
- **Response `201 Created`:** Returns created Job JSON object.

### 4.3 Get Job Details
`GET /jobs/{id}`
Retrieves comprehensive details of a specific job posting.

- **Auth Required:** No
- **Response `200 OK`:** Returns Job JSON object.
- **Response `404 Not Found`:** `{"error": "Job not found"}`

### 4.4 Update Job Posting
`PUT /jobs/{id}`
Updates posting parameters. Closed and cancelled jobs are completely immutable (F-20).

- **Auth Required:** Yes (Must be the job author)
- **Validation Invariants:**
  - Status invariant (F-20): If the job status is `closed` or `cancelled`, rejects update with `400 Bad Request` (`{"error": "Cannot update a closed or cancelled job"}`).
  - Description capped at 5,000 characters.
  - Skills capped at 25 items <= 50 characters.
- **Response `200 OK`:** Returns updated Job JSON object.

### 4.5 Delete / Close Job Posting
`DELETE /jobs/{id}`
Transitions an open job posting to `cancelled`. Cannot be executed if an active contract exists.

- **Auth Required:** Yes (Must be the job author)
- **Response `200 OK`:** `{"message": "Job cancelled successfully"}`

---

## 5. Applications Domain (`/applications`)

### 5.1 Submit Application Proposal
`POST /applications`
Submits a proposal pitch against an open opportunity with attached verified resume key.

- **Auth Required:** Yes (Must have `email_verified: true`)
- **Request Body:**
  ```json
  {
    "job_id": "7b6f284e-3c29-4d6b-8fa2-1a4c8e7f9012",
    "cover_letter": "I have 2 years of experience with OpenCV cell segmentation models."
  }
  ```
- **Invariants:**
  - Cannot apply to one's own posting (`created_by == applicant_id` rejected with 400).
  - Cannot submit duplicate applications (enforced by `uq_applications_job_applicant`).
  - Automatically snapshots applicant's active `profiles.resume_key`.
- **Response `201 Created`:** Returns Application JSON object.

### 5.2 List Member Applications
`GET /applications`
Lists applications submitted by the caller or incoming applicants for an employer's postings.

- **Query Parameters:**
  - `job_id` (optional UUID): Filter by job (only allowed if caller created the job).
  - `status` (string): `pending`, `accepted`, `rejected`.
- **Response `200 OK`:**
  ```json
  [
    {
      "id": "1c8f42e0-2b19-4f7a-8fa3-2c1b9f8d7a6e",
      "job_id": "7b6f284e-3c29-4d6b-8fa2-1a4c8e7f9012",
      "applicant_id": "st_usr_9a4f21e0",
      "cover_letter": "I have 2 years of experience...",
      "resume_key": "resumes/st_usr_9a4f21e0/resume.pdf",
      "status": "pending",
      "created_at": "2026-09-11T12:00:00Z"
    }
  ]
  ```

### 5.3 Accept Application & Generate Contract
`POST /applications/{id}/accept`
Accepts a candidate application, transitions job to `closed`, automatically marks competing proposals as `rejected`, and generates an `active` contract in a single atomic database transaction (`AcceptApplicationTx`). Allowed even if submission deadline has passed (F-04).

- **Auth Required:** Yes (Must be the job author)
- **Response `200 OK`:**
  ```json
  {
    "contract_id": "3a9c7b2e-4f18-4e9b-b0cf-1a2b3c4d5e6f",
    "status": "active"
  }
  ```

### 5.4 Reject Application
`POST /applications/{id}/reject`
Explicitly rejects a pending application proposal.

- **Auth Required:** Yes (Must be the job author)
- **Response `200 OK`:** `{"status": "rejected"}`

---

## 6. Contracts Domain (`/contracts`)

### 6.1 List User Contracts
`GET /contracts`
Returns all contracts where the authenticated caller is either the client or the freelancer.

- **Query Parameters:**
  - `status` (string): `active`, `completed`, `cancelled`.
- **Response `200 OK`:** Returns array of Contract JSON objects.

### 6.2 Get Contract Details
`GET /contracts/{id}`
Retrieves deliverables, timeline status, and participants for a specific contract.

- **Auth Required:** Yes (Caller must be client or freelancer on the contract)
- **Response `200 OK`:** Returns Contract JSON object.

### 6.3 Complete Contract
`POST /contracts/{id}/complete`
Marks deliverables accepted and completes the contract. Unlocks review submission.

- **Auth Required:** Yes (Must be the contract `client_id`)
- **State Machine Invariant:** Contract must currently be in `active` status.
- **Response `200 OK`:** `{"status": "completed"}`

### 6.4 Cancel Contract & Restore Candidates
`POST /contracts/{id}/cancel`
Cancels an active contract, restores competing candidate applications to `pending`, and re-opens parent job for hiring (F-01, F-03).

- **Auth Required:** Yes (Must be the contract `client_id`)
- **Lock Ordering Discipline (F-01):** Locks `jobs` row before mutating `contracts` table to eliminate deadlocks.
- **Response `200 OK`:**
  ```json
  {
    "status": "cancelled",
    "job_status": "open",
    "restored_applications_count": 4
  }
  ```

---

## 7. Reviews & Ratings Domain (`/reviews`)

### 7.1 Submit Contract Review
`POST /reviews`
Submits a 1-5 star rating and qualitative evaluation for a completed contract.

- **Auth Required:** Yes (Must be a participant on the completed contract)
- **Request Body:**
  ```json
  {
    "contract_id": "3a9c7b2e-4f18-4e9b-b0cf-1a2b3c4d5e6f",
    "rating": 5,
    "comment": "Exceptional delivery speed, clean code, and great technical communication."
  }
  ```
- **Validation Rules:**
  - `contract_id`: Contract status must be `completed`.
  - `rating`: Integer between 1 and 5.
  - Uniqueness: One review per participant per contract (`uq_reviews_contract_reviewer`).
- **Response `201 Created`:** Returns Review JSON object.

### 7.2 Get Member Reviews
`GET /reviews/user/{id}`
Returns all public peer reviews received by a campus member.

- **Auth Required:** No
- **Response `200 OK`:**
  ```json
  [
    {
      "id": "8b2c4f1a-9e3d-4c8a-9a1b-3f2e1d0c9b8a",
      "contract_id": "3a9c7b2e-4f18-4e9b-b0cf-1a2b3c4d5e6f",
      "reviewer_id": "st_usr_client123",
      "reviewee_id": "st_usr_9a4f21e0",
      "rating": 5,
      "comment": "Exceptional delivery speed...",
      "created_at": "2026-09-11T18:00:00Z"
    }
  ]
  ```
