import { test } from "node:test";
import assert from "node:assert/strict";
import type { ProjectApplication } from "../types/project.ts";
import {
  getApplicantDisplayName,
  getApplicantEmail,
  getApplicantDepartment,
  getApplicantInitials,
  getApplicationStatusBadgeClasses,
  computeApplicationMetrics,
  filterApplications,
} from "./recruitment.ts";

const mockApplications: ProjectApplication[] = [
  {
    id: "app-1",
    project_id: "proj-1",
    applicant_id: "user-1",
    status: "submitted",
    statement: "I have 2 years of experience with Python and machine learning.",
    resume_asset_key: "resumes/user-1.pdf",
    created_at: "2026-10-10T12:00:00Z",
    applicant_name: "Alice Walker",
    applicant_email: "alice@mit.edu",
    applicant_department: "Computer Science",
  },
  {
    id: "app-2",
    project_id: "proj-1",
    applicant_id: "user-2",
    status: "shortlisted",
    statement: "Hardware designer proficient in Altium and KiCad for PCB layout.",
    reviewer_notes: "Strong candidate for robotics team",
    created_at: "2026-10-11T14:30:00Z",
    applicant: {
      email: "bob@stanford.edu",
      first_name: "Bob",
      last_name: "Taylor",
      department: "Electrical Engineering",
      bio: "Hardware enthusiast",
    },
  },
  {
    id: "app-3",
    project_id: "proj-1",
    applicant_id: "user-3",
    status: "accepted",
    statement: "Passionate about autonomous flight and drone telemetry.",
    resume_asset_key: "resumes/user-3.pdf",
    reviewer_notes: "Accepted directly into avionics pod",
    created_at: "2026-10-12T09:15:00Z",
    decided_at: "2026-10-13T10:00:00Z",
    applicant_name: "Charlie Davis",
    applicant_email: "charlie@berkeley.edu",
    applicant_department: "AeroAstro",
  },
  {
    id: "app-4",
    project_id: "proj-1",
    applicant_id: "user-4",
    status: "rejected",
    statement: "Interested in front-end design but lacking required systems knowledge.",
    reviewer_notes: "Position filled for this cohort",
    created_at: "2026-10-08T11:00:00Z",
    decided_at: "2026-10-09T16:00:00Z",
    applicant: {
      email: "dan@cmu.edu",
      first_name: "Dan",
      last_name: "Evans",
      department: "Human-Computer Interaction",
    },
  },
  {
    id: "app-5",
    project_id: "proj-1",
    applicant_id: "user-5",
    status: "withdrawn",
    statement: "Withdrawing due to course conflict.",
    created_at: "2026-10-05T08:00:00Z",
    applicant_email: "eva@harvard.edu",
  },
];

test("getApplicantDisplayName handles flat, nested, and fallback names", () => {
  assert.equal(getApplicantDisplayName(mockApplications[0]), "Alice Walker");
  assert.equal(getApplicantDisplayName(mockApplications[1]), "Bob Taylor");
  assert.equal(getApplicantDisplayName(mockApplications[4]), "eva@harvard.edu");

  const emptyApp: ProjectApplication = {
    id: "app-empty",
    project_id: "proj-1",
    applicant_id: "user-6",
    status: "submitted",
    statement: "Test",
    created_at: "2026-10-01T00:00:00Z",
  };
  assert.equal(getApplicantDisplayName(emptyApp), "Applicant");
});

test("getApplicantEmail resolves email correctly", () => {
  assert.equal(getApplicantEmail(mockApplications[0]), "alice@mit.edu");
  assert.equal(getApplicantEmail(mockApplications[1]), "bob@stanford.edu");
  assert.equal(getApplicantEmail(mockApplications[4]), "eva@harvard.edu");
});

test("getApplicantDepartment resolves department and provides campus fallback", () => {
  assert.equal(getApplicantDepartment(mockApplications[0]), "Computer Science");
  assert.equal(getApplicantDepartment(mockApplications[1]), "Electrical Engineering");
  assert.equal(getApplicantDepartment(mockApplications[4]), "General Campus");
});

test("getApplicantInitials generates 1-2 character ASCII initials", () => {
  assert.equal(getApplicantInitials("Alice Walker"), "AW");
  assert.equal(getApplicantInitials("Bob"), "BO");
  assert.equal(getApplicantInitials(""), "AP");
  assert.equal(getApplicantInitials(undefined), "AP");
});

test("getApplicationStatusBadgeClasses returns valid styles for all statuses", () => {
  const submittedStyle = getApplicationStatusBadgeClasses("submitted");
  assert.equal(submittedStyle.label, "Submitted");
  assert.ok(submittedStyle.text.includes("blue"));

  const shortlistedStyle = getApplicationStatusBadgeClasses("shortlisted");
  assert.equal(shortlistedStyle.label, "Shortlisted");
  assert.ok(shortlistedStyle.text.includes("amber"));

  const acceptedStyle = getApplicationStatusBadgeClasses("accepted");
  assert.equal(acceptedStyle.label, "Accepted");
  assert.ok(acceptedStyle.text.includes("emerald"));

  const rejectedStyle = getApplicationStatusBadgeClasses("rejected");
  assert.equal(rejectedStyle.label, "Rejected");
  assert.ok(rejectedStyle.text.includes("rose"));

  const withdrawnStyle = getApplicationStatusBadgeClasses("withdrawn");
  assert.equal(withdrawnStyle.label, "Withdrawn");
});

test("computeApplicationMetrics aggregates application counts accurately", () => {
  const metrics = computeApplicationMetrics(mockApplications);
  assert.equal(metrics.total, 5);
  assert.equal(metrics.submitted, 1);
  assert.equal(metrics.shortlisted, 1);
  assert.equal(metrics.accepted, 1);
  assert.equal(metrics.rejected, 1);
  assert.equal(metrics.withdrawn, 1);

  const emptyMetrics = computeApplicationMetrics([]);
  assert.equal(emptyMetrics.total, 0);
  assert.equal(emptyMetrics.submitted, 0);
});

test("filterApplications filters by search query across multiple fields", () => {
  // Search by name
  const nameResults = filterApplications(mockApplications, "Alice", "all");
  assert.equal(nameResults.length, 1);
  assert.equal(nameResults[0].id, "app-1");

  // Search by email domain
  const domainResults = filterApplications(mockApplications, "stanford.edu", "all");
  assert.equal(domainResults.length, 1);
  assert.equal(domainResults[0].id, "app-2");

  // Search by department
  const deptResults = filterApplications(mockApplications, "AeroAstro", "all");
  assert.equal(deptResults.length, 1);
  assert.equal(deptResults[0].id, "app-3");

  // Search by statement text
  const statementResults = filterApplications(mockApplications, "telemetry", "all");
  assert.equal(statementResults.length, 1);
  assert.equal(statementResults[0].id, "app-3");

  // Search non-existent term
  const noResults = filterApplications(mockApplications, "QuantumBiology", "all");
  assert.equal(noResults.length, 0);
});

test("filterApplications filters by status correctly", () => {
  const shortlistedOnly = filterApplications(mockApplications, "", "shortlisted");
  assert.equal(shortlistedOnly.length, 1);
  assert.equal(shortlistedOnly[0].id, "app-2");

  const acceptedOnly = filterApplications(mockApplications, "", "accepted");
  assert.equal(acceptedOnly.length, 1);
  assert.equal(acceptedOnly[0].id, "app-3");

  // Combined status and search query
  const combined = filterApplications(mockApplications, "Davis", "accepted");
  assert.equal(combined.length, 1);

  const combinedMismatch = filterApplications(mockApplications, "Davis", "shortlisted");
  assert.equal(combinedMismatch.length, 0);
});
