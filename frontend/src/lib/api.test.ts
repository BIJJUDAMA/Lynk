import test from "node:test";
import assert from "node:assert/strict";
import {
  ApiClientError,
  DEFAULT_API_BASE_URL,
  buildQueryString,
  createApiClient,
  isEmailNotVerifiedError,
  isUnauthorizedError,
  getMe,
  uploadResume,
  listJobs,
  createJob,
  applyToJob,
  updateApplicationStatus,
  updateContractStatus,
  createReview,
} from "./api.ts";

/**
 * Helper to mock globalThis.fetch for the duration of a test.
 */
function mockFetch(
  handler: (url: string, init?: RequestInit) => Promise<Response> | Response
): () => void {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = (input: RequestInfo | URL, init?: RequestInit) => {
    const url =
      typeof input === "string"
        ? input
        : input instanceof URL
        ? input.toString()
        : input.url;
    return Promise.resolve(handler(url, init));
  };
  return () => {
    globalThis.fetch = originalFetch;
  };
}

test("buildQueryString encodes primitives, arrays, and skips null/undefined", () => {
  const params = {
    search: "full stack",
    department: "Computer Science",
    skill: "Go",
    skills: ["React", "PostgreSQL"],
    min_budget: 500,
    active: true,
    empty: "",
    unassigned: undefined,
    nullish: null,
  };

  const qs = buildQueryString(params);
  assert.ok(qs.startsWith("?"));
  assert.match(qs, /search=full\+stack/);
  assert.match(qs, /department=Computer\+Science/);
  assert.match(qs, /skill=Go/);
  assert.match(qs, /skills=React/);
  assert.match(qs, /skills=PostgreSQL/);
  assert.match(qs, /min_budget=500/);
  assert.match(qs, /active=true/);
  assert.ok(!qs.includes("empty="));
  assert.ok(!qs.includes("unassigned="));
  assert.ok(!qs.includes("nullish="));

  // Empty or undefined params
  assert.equal(buildQueryString(undefined), "");
  assert.equal(buildQueryString({}), "");
});

test("ApiClient injects Bearer token and JSON headers properly", async () => {
  let capturedUrl = "";
  let capturedInit: RequestInit | undefined;

  const restore = mockFetch((url, init) => {
    capturedUrl = url;
    capturedInit = init;
    return new Response(
      JSON.stringify({
        success: true,
        data: { message: "pong" },
        error: null,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } }
    );
  });

  try {
    const client = createApiClient(
      async () => "test-mock-jwt-token",
      "http://localhost:8080/api/v1"
    );

    const data = await client.post<{ message: string }>("/test", { foo: "bar" });
    assert.deepEqual(data, { message: "pong" });

    assert.equal(capturedUrl, "http://localhost:8080/api/v1/test");
    assert.equal(capturedInit?.method, "POST");

    const headers = capturedInit?.headers as Record<string, string>;
    assert.equal(headers["Authorization"], "Bearer test-mock-jwt-token");
    assert.equal(headers["Content-Type"], "application/json");
    assert.equal(headers["Accept"], "application/json");
    assert.equal(capturedInit?.body, JSON.stringify({ foo: "bar" }));
    assert.equal(capturedInit?.credentials, "include");
  } finally {
    restore();
  }
});

test("ApiClient includes credentials: include by default", async () => {
  let capturedInit: RequestInit | undefined;

  const restore = mockFetch((_url, init) => {
    capturedInit = init;
    return new Response(
      JSON.stringify({
        success: true,
        data: { ok: true },
        error: null,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } }
    );
  });

  try {
    const client = createApiClient();
    await client.get("/test-credentials");
    assert.equal(capturedInit?.credentials, "include");
  } finally {
    restore();
  }
});

test("ApiClient unwraps success envelope correctly", async () => {
  const restore = mockFetch(() => {
    return new Response(
      JSON.stringify({
        success: true,
        data: { id: "job-101", title: "Go Developer" },
        error: null,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } }
    );
  });

  try {
    const client = createApiClient();
    const result = await client.get<{ id: string; title: string }>("/jobs/job-101");
    assert.equal(result.id, "job-101");
    assert.equal(result.title, "Go Developer");
  } finally {
    restore();
  }
});

test("ApiClient handles 204 No Content response gracefully", async () => {
  const restore = mockFetch(() => {
    return new Response(null, { status: 204 });
  });

  try {
    const client = createApiClient();
    const result = await client.delete<void>("/jobs/job-101");
    assert.equal(result, undefined);
  } finally {
    restore();
  }
});

test("ApiClient extracts ApiClientError for EMAIL_NOT_VERIFIED error envelope", async () => {
  const restore = mockFetch(() => {
    return new Response(
      JSON.stringify({
        success: false,
        data: null,
        error: {
          code: "EMAIL_NOT_VERIFIED",
          message: "University email must be verified before performing this action",
        },
      }),
      { status: 403, headers: { "Content-Type": "application/json" } }
    );
  });

  try {
    const client = createApiClient();
    await client.post("/jobs/job-101/applications", { cover_letter: "Hire me" });
    assert.fail("Expected ApiClientError to be thrown");
  } catch (err: unknown) {
    assert.ok(err instanceof ApiClientError);
    assert.equal(err.code, "EMAIL_NOT_VERIFIED");
    assert.equal(err.status, 403);
    assert.equal(
      err.message,
      "University email must be verified before performing this action"
    );
    assert.equal(err.isEmailNotVerified, true);
    assert.equal(err.isUnauthorized, false);
    assert.equal(err.isForbidden, true);
    assert.equal(isEmailNotVerifiedError(err), true);
  } finally {
    restore();
  }
});

test("ApiClient extracts ApiClientError for UNAUTHORIZED error response", async () => {
  const restore = mockFetch(() => {
    return new Response(
      JSON.stringify({
        success: false,
        data: null,
        error: {
          code: "UNAUTHORIZED",
          message: "Missing credentials",
        },
      }),
      { status: 401, headers: { "Content-Type": "application/json" } }
    );
  });

  try {
    const client = createApiClient();
    await client.get("/auth/me");
    assert.fail("Expected ApiClientError to be thrown");
  } catch (err: unknown) {
    assert.ok(err instanceof ApiClientError);
    assert.equal(err.code, "UNAUTHORIZED");
    assert.equal(err.status, 401);
    assert.equal(err.message, "Missing credentials");
    assert.equal(err.isUnauthorized, true);
    assert.equal(err.isEmailNotVerified, false);
    assert.equal(isUnauthorizedError(err), true);
  } finally {
    restore();
  }
});

test("ApiClient converts network failures to ApiClientError with NETWORK_ERROR code", async () => {
  const restore = mockFetch(() => {
    throw new TypeError("Failed to fetch");
  });

  try {
    const client = createApiClient();
    await client.get("/health");
    assert.fail("Expected ApiClientError to be thrown");
  } catch (err: unknown) {
    assert.ok(err instanceof ApiClientError);
    assert.equal(err.code, "NETWORK_ERROR");
    assert.equal(err.status, 0);
    assert.match(err.message, /Failed to fetch/);
  } finally {
    restore();
  }
});

test("ApiClient uploadFile constructs FormData with specified field name", async () => {
  let capturedInit: RequestInit | undefined;

  const restore = mockFetch((_, init) => {
    capturedInit = init;
    return new Response(
      JSON.stringify({
        success: true,
        data: {
          message: "Resume uploaded successfully",
          filename: "resume.pdf",
          byte_size: 1024,
          resume_key: "students/123/resume.pdf",
        },
        error: null,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } }
    );
  });

  try {
    const client = createApiClient();
    const fakeFile = new File(["dummy pdf content"], "my_resume.pdf", {
      type: "application/pdf",
    });

    const res = await uploadResume(fakeFile, client);
    assert.equal(res.filename, "resume.pdf");
    assert.equal(res.resume_key, "students/123/resume.pdf");

    assert.equal(capturedInit?.method, "POST");
    assert.ok(capturedInit?.body instanceof FormData);

    const formData = capturedInit.body as FormData;
    const fileEntry = formData.get("resume") as File;
    assert.ok(fileEntry);
    assert.equal(fileEntry.name, "my_resume.pdf");

    // Ensure Content-Type is NOT forced to application/json on multipart
    const headers = capturedInit.headers as Record<string, string>;
    assert.equal(headers["Content-Type"], undefined);
  } finally {
    restore();
  }
});

test("Domain functions call expected endpoints with expected payloads and parameters", async () => {
  const calls: { url: string; method: string; body?: unknown }[] = [];

  const restore = mockFetch((url, init) => {
    calls.push({
      url,
      method: init?.method || "GET",
      body: init?.body ? JSON.parse(init.body as string) : undefined,
    });

    return new Response(
      JSON.stringify({
        success: true,
        data: { id: "domain-test-id", status: "ok" },
        error: null,
      }),
      { status: 200, headers: { "Content-Type": "application/json" } }
    );
  });

  try {
    const client = createApiClient(undefined, "http://localhost:8080/api/v1");

    // 1. getMe
    await getMe(client);
    assert.equal(calls[0].url, "http://localhost:8080/api/v1/auth/me");
    assert.equal(calls[0].method, "GET");

    // 2. listJobs with filters
    await listJobs(
      { search: "frontend", min_budget: 300, limit: 10 },
      client
    );
    assert.match(calls[1].url, /jobs\?search=frontend/);
    assert.match(calls[1].url, /min_budget=300/);
    assert.match(calls[1].url, /limit=10/);
    assert.equal(calls[1].method, "GET");

    // 3. createJob
    await createJob(
      {
        title: "Web App Design",
        description: "Need modern UI",
        budget: 1500,
        pay_type: "fixed",
        required_skills: ["Figma", "Tailwind"],
        department: "Design",
      },
      client
    );
    assert.equal(calls[2].url, "http://localhost:8080/api/v1/jobs");
    assert.equal(calls[2].method, "POST");
    assert.equal((calls[2].body as { title: string }).title, "Web App Design");

    // 4. applyToJob
    await applyToJob("job-abc", { cover_letter: "I am experienced" }, client);
    assert.equal(
      calls[3].url,
      "http://localhost:8080/api/v1/jobs/job-abc/applications"
    );
    assert.equal(calls[3].method, "POST");
    assert.equal(
      (calls[3].body as { cover_letter: string }).cover_letter,
      "I am experienced"
    );

    // 5. updateApplicationStatus
    await updateApplicationStatus("app-xyz", "accepted", client);
    assert.equal(
      calls[4].url,
      "http://localhost:8080/api/v1/applications/app-xyz/status"
    );
    assert.equal(calls[4].method, "PATCH");
    assert.deepEqual(calls[4].body, { status: "accepted" });

    // 6. updateContractStatus
    await updateContractStatus("contract-123", "active", client);
    assert.equal(
      calls[5].url,
      "http://localhost:8080/api/v1/contracts/contract-123/status"
    );
    assert.equal(calls[5].method, "PATCH");
    assert.deepEqual(calls[5].body, { status: "active" });

    // 7. createReview
    await createReview(
      "contract-123",
      { rating: 5, comment: "Exceptional quality work!" },
      client
    );
    assert.equal(
      calls[6].url,
      "http://localhost:8080/api/v1/contracts/contract-123/reviews"
    );
    assert.equal(calls[6].method, "POST");
    assert.deepEqual(calls[6].body, {
      rating: 5,
      comment: "Exceptional quality work!",
    });
  } finally {
    restore();
  }
});

test("DEFAULT_API_BASE_URL correctly resolves with /api/v1 suffix", () => {
  assert.ok(DEFAULT_API_BASE_URL.endsWith("/api/v1"));
  assert.ok(!DEFAULT_API_BASE_URL.endsWith("/api/v1/api/v1"));
});

