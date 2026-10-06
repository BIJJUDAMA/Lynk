import test from "node:test";
import assert from "node:assert/strict";
import type { Job, Application, Contract, ContractWithDetails } from "./api.ts";

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

test("ContractWithDetails resolves counterparty with nullish coalescing for freelancer/student and client/employer", () => {
  const c1: ContractWithDetails = {
    id: "contract-1",
    job_id: "job-1",
    application_id: "app-1",
    client_id: "client-1",
    freelancer_id: "freelancer-1",
    status: "active",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    freelancer: {
      id: "freelancer-1",
      email: "freelancer@uni.edu",
      first_name: "Jane",
      last_name: "Doe",
    },
    client: {
      id: "client-1",
      email: "client@uni.edu",
      first_name: "John",
      last_name: "Smith",
      company_or_org: "Acme Corp",
    },
  };
  const freelancer1 = c1.freelancer ?? c1.student;
  const client1 = c1.client ?? c1.employer;
  assert.equal(freelancer1?.first_name, "Jane");
  assert.equal(client1?.company_or_org, "Acme Corp");

  const c2: ContractWithDetails = {
    id: "contract-2",
    job_id: "job-2",
    application_id: "app-2",
    client_id: "client-2",
    freelancer_id: "freelancer-2",
    status: "active",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
    student: {
      id: "freelancer-2",
      email: "student@uni.edu",
      first_name: "Alice",
      last_name: "Student",
    },
    employer: {
      id: "client-2",
      email: "employer@uni.edu",
      first_name: "Bob",
      last_name: "Employer",
      company_or_org: "Beta LLC",
    },
  };
  const freelancer2 = c2.freelancer ?? c2.student;
  const client2 = c2.client ?? c2.employer;
  assert.equal(freelancer2?.first_name, "Alice");
  assert.equal(client2?.company_or_org, "Beta LLC");
});
