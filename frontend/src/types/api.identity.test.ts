import test from "node:test";
import assert from "node:assert/strict";
import type { Job, Application, Contract } from "./api.ts";

export function assertCanonicalJob(job: Job): string {
  return job.created_by;
}

export function assertCanonicalApplication(app: Application): string {
  return app.applicant_id;
}

export function assertCanonicalContract(c: Contract): string {
  return `${c.client_id}:${c.freelancer_id}`;
}

test("Job requires created_by", () => {
  const job: Job = {
    id: "job-1",
    created_by: "user-1",
    title: "Test",
    description: "Desc",
    required_skills: [],
    department: "CS",
    status: "open",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
  assert.equal(assertCanonicalJob(job), "user-1");
});

test("Application requires applicant_id", () => {
  const app: Application = {
    id: "app-1",
    job_id: "job-1",
    applicant_id: "user-2",
    cover_letter: "Hello",
    status: "pending",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
  assert.equal(assertCanonicalApplication(app), "user-2");
});

test("Contract requires client_id and freelancer_id", () => {
  const contract: Contract = {
    id: "contract-1",
    job_id: "job-1",
    application_id: "app-1",
    client_id: "user-1",
    freelancer_id: "user-2",
    status: "active",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  };
  assert.equal(assertCanonicalContract(contract), "user-1:user-2");
});
