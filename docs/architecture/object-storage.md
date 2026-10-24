# Object Storage Architecture & MinIO Policies - Lynk

> **Authoritative Object Storage Specification**
> **Storage Engine:** MinIO S3 (`lynk-minio` on ports `9000` API / `9001` Web Console)
> **Scope:** Dedicated exclusively to student resumes (`resumes` bucket)
> **Upload Boundary:** Streaming S3 decoupled from PostgreSQL transactions & advisory locks (F-02)
> **Access Policy:** Private bucket with short-lived pre-signed download URLs (15-minute TTL)

---

## 1. Storage Invariants & Scope Boundary

Lynk enforces strict architectural boundaries separating relational state from binary file storage:

```
+--------------------------------------------------------+
|                   PostgreSQL 16                        |
|  Stores ONLY lightweight metadata:                     |
|  - resume_key: "resumes/st_usr_9a4f21e0/4c1b9f8d.pdf" |
|  - file_name, file_size, mime_type, uploaded_at        |
+--------------------------------------------------------+
                           ^
                           | Reference key only
                           v
+--------------------------------------------------------+
|                     MinIO S3                           |
|  Stores binary document payloads:                      |
|  - Bucket: "resumes" (private, no anonymous access)    |
|  - Object Key Pattern: "resumes/{user_id}/{uuid}.ext"  |
+--------------------------------------------------------+
```

### 1.1 Non-Negotiable Storage Rules
1. **Resumes Only:** MinIO is dedicated strictly to student resumes. It must not be used for generic avatar hosting, static asset serving, or system logs.
2. **Zero Container Disk Persistence:** Resumes are never written to the local container or host filesystem. File uploads stream directly from the incoming HTTP multipart request body into MinIO via the AWS S3 Go SDK v2.
3. **No File Blobs in PostgreSQL:** PostgreSQL columns must never store `BYTEA` or binary blobs. Only S3 object keys are persisted in `profiles.resume_key`, `applications.resume_key`, and `project_applications.resume_asset_key`.
4. **Private by Default:** The `resumes` bucket is strictly private. Anonymous read/write access is blocked at the MinIO bucket policy level.

---

## 2. Bucket Provisioning & Initialization

The `resumes` bucket is provisioned automatically and idempotently during Go backend startup via `EnsureBucket(ctx)` (`cmd/api/main.go` and `internal/storage/s3.go`):

```go
// EnsureBucket probes bucket existence and creates it if missing
func (c *S3Client) EnsureBucket(ctx context.Context) error {
    _, err := c.internalClient.HeadBucket(ctx, &s3.HeadBucketInput{
        Bucket: aws.String(c.bucket),
    })
    if err != nil {
        _, err = c.internalClient.CreateBucket(ctx, &s3.CreateBucketInput{
            Bucket: aws.String(c.bucket),
        })
    }
    return err
}
```

### 2.1 Bucket Configuration Parameters
| Parameter | Value | Description |
| :--- | :--- | :--- |
| **Bucket Name** | `resumes` | Root storage namespace for all campus resumes |
| **Storage Class** | `STANDARD` | Default MinIO erasure-coded storage class |
| **Versioning** | Disabled | New uploads replace existing keys or generate unique UUIDs |
| **Access Policy** | Private (`private`) | Zero anonymous access; authenticated S3 credentials required |

---

## 3. Decoupled Upload Pipeline & Concurrency Safety (F-02, F-13)

### 3.1 The Problem: Advisory Lock Contention (F-02)
Prior to the Matrix Audit remediation, streaming file uploads into MinIO occurred *inside* a PostgreSQL advisory lock transaction (`WithProfileLock`). Under slow student network connections (e.g. uploading a 4MB PDF over campus Wi-Fi for 45 seconds), the database connection and advisory lock were held for the entire upload duration, exhausting the database connection pool and blocking other profile updates.

### 3.2 The Solution: External Streaming with Compensating Deletion
The upload pipeline in `internal/user/service.go` now completely decouples S3 network transfer from database transactions:

```mermaid
sequenceDiagram
    autonumber
    actor User as Campus Student
    participant API as Go REST API
    participant S3 as MinIO S3 Engine
    participant DB as PostgreSQL Database

    User->>API: POST /api/v1/profile/resume (multipart/form-data)
    Note over API: Step 1: Validate MIME & Size (<= 5MB)

    Note over API,S3: Step 2: Stream to S3 OUTSIDE DB Lock
    API->>S3: PutObject(resumes, "resumes/{id}/{uuid}.pdf", stream)
    S3-->>API: PutObject Success

    Note over API,DB: Step 3: Fast DB Advisory Lock
    API->>DB: Begin WithProfileLock(user_id)
    alt Database Commit Success
        API->>DB: UPDATE profiles SET resume_key = $1
        API->>DB: Commit Lock
        API-->>User: HTTP 200 OK (Resume Metadata)
    else Database Commit Failure
        API->>DB: Rollback Lock
        Note over API,S3: Step 4: Compensating S3 Deletion
        API->>S3: DeleteObject(resumes, "resumes/{id}/{uuid}.pdf")
        API-->>User: HTTP 500 Internal Error ("Profile update failed")
    end
```

### 3.3 Compensating Deletion Invariant
If PostgreSQL fails to commit the new `resume_key` (e.g. database timeout, foreign key failure), the Go service immediately issues a compensating `DeleteObject` call to MinIO, guaranteeing that orphaned files never accumulate in the storage bucket.

### 3.4 Immutable Storage Client Injection (F-13)
The `storage.Client` is injected immutably into `user.NewService(userRepo, s3Client)` at server startup (`cmd/api/main.go`). Route handlers (`internal/user/resume_handler.go`) are strictly forbidden from mutating `h.service.storage` at runtime, completely eliminating data races under concurrent uploads.

---

## 4. Download & Retrieval Architecture

Resume documents are retrieved through short-lived presigned URLs generated by the Go API:

### 4.1 Pre-Signed GET URL Generation
For viewing in the browser or downloading in the Next.js frontend, the Go API generates a short-lived pre-signed URL:
```go
func (c *S3Client) GetPresignedDownloadURL(ctx context.Context, key string, expiry time.Duration) (string, error) {
    req, err := c.presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
        Bucket: aws.String(c.bucket),
        Key:    aws.String(key),
    }, s3.WithPresignExpires(expiry))
    return req.URL, err
}
```
- **TTL (Time to Live):** 15 minutes (`15 * time.Minute`).
- **Signature:** AWS Signature Version 4 (HMAC-SHA256).
- **Dual-Client Resolver:** Generated using `publicClient` (`MINIO_PUBLIC_ENDPOINT`) so that SigV4 host headers match browser requests (`localhost:9000`), preventing signature mismatch errors.
- **Payload:** Endpoints (`GET /api/v1/profile/resume`, `GET /api/v1/profile/{id}/resume`) return a JSON envelope containing `{"download_url": "...", "url": "...", "filename": "..."}`.
- **Authorization:** Gated by `RequireVerifiedEmail()` and `PGResumeAccessChecker`. Only the profile owner, an administrator, or a hiring manager with an active application from the candidate can access the download link.

---

## 5. Security & Validation Rules

| Dimension | Rule | Enforcement Location | Failure Response |
| :--- | :--- | :--- | :--- |
| **Max File Size** | 5,242,880 bytes (`5MB`) | Handler (`http.MaxBytesReader`) & Service | `400 Bad Request` (`FILE_TOO_LARGE`) |
| **Allowed File Types** | `application/pdf`, `.docx` | Service (magic bytes `%PDF-` & OOXML zip check) | `400 Bad Request` (`INVALID_FILE_TYPE`) |
| **Key Namespace** | `resumes/{user_id}/{uuid}-{filename}.{ext}` | Service (Path construction) | Internal invariant |
| **Authorization Gate** | Verified `.edu` email required | `RequireVerifiedEmail` Middleware | `403 Forbidden` |
| **Orphan Prevention** | Compensating S3 delete on DB error | `internal/user/service.go` | Automated cleanup |
