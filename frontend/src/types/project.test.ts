import test from "node:test";
import assert from "node:assert/strict";
import {
  PROJECT_VISIBILITIES,
  RECRUITMENT_STATUSES,
  LIFECYCLE_STATUSES,
  PROJECT_ROLES,
  APPLICATION_STATUSES,
  SECTION_TYPES,
} from "./project.ts";
import type {
  Project,
  ProjectMember,
  ProjectSection,
  ProjectResource,
  ProjectApplication,
  ProjectInvitation,
  CreateProjectPayload,
  UpdateProjectPayload,
  UpdateVisibilityPayload,
  SubmitApplicationPayload,
  InviteMemberPayload,
  AddResourcePayload,
  ProjectFilter,
} from "./project.ts";

test("Project constants contain all canonical status and visibility values", () => {
  assert.deepEqual(PROJECT_VISIBILITIES, ["public", "unlisted", "private"]);
  assert.deepEqual(RECRUITMENT_STATUSES, ["open", "paused", "closed"]);
  assert.deepEqual(LIFECYCLE_STATUSES, ["idea", "active", "completed", "archived"]);
  assert.deepEqual(PROJECT_ROLES, ["lead", "maintainer", "member", "viewer"]);
  assert.deepEqual(APPLICATION_STATUSES, [
    "submitted",
    "shortlisted",
    "accepted",
    "rejected",
    "withdrawn",
  ]);
  assert.deepEqual(SECTION_TYPES, [
    "overview",
    "team",
    "resources",
    "recruitment",
    "updates",
    "docs",
  ]);
});

test("Project interface validates required fields and optional semantic metrics", () => {
  const project: Project = {
    id: "p-100",
    slug: "autonomous-rover",
    owner_member_id: "m-1",
    title: "Autonomous Rover",
    description: "Robotics club rover project",
    category: "Robotics",
    visibility: "public",
    recruitment_status: "open",
    lifecycle_status: "active",
    settings: { allow_external_viewers: true },
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-02T00:00:00Z",
    similarity: 0.95,
    distance: 0.05,
  };

  assert.equal(project.id, "p-100");
  assert.equal(project.slug, "autonomous-rover");
  assert.equal(project.owner_member_id, "m-1");
  assert.equal(project.visibility, "public");
  assert.equal(project.recruitment_status, "open");
  assert.equal(project.lifecycle_status, "active");
  assert.equal(project.similarity, 0.95);
});

test("ProjectMember supports nested and top-level profile information", () => {
  const memberWithNested: ProjectMember = {
    id: "pm-1",
    project_id: "p-100",
    member_id: "u-101",
    role: "lead",
    status: "active",
    joined_at: "2026-01-01T00:00:00Z",
    member: {
      email: "lead@university.edu",
      first_name: "Alice",
      last_name: "Smith",
      department: "Computer Science",
      bio: "Robotics researcher",
    },
  };

  const memberWithFlat: ProjectMember = {
    id: "pm-2",
    project_id: "p-100",
    member_id: "u-102",
    role: "member",
    status: "active",
    joined_at: "2026-01-02T00:00:00Z",
    email: "member@university.edu",
    first_name: "Bob",
    last_name: "Jones",
    department: "Electrical Engineering",
  };

  assert.equal(memberWithNested.role, "lead");
  assert.equal(memberWithNested.member?.first_name, "Alice");
  assert.equal(memberWithFlat.role, "member");
  assert.equal(memberWithFlat.email, "member@university.edu");
});

test("ProjectSection models section types and configurations", () => {
  const section: ProjectSection = {
    id: "sec-1",
    project_id: "p-100",
    section_type: "recruitment",
    title: "Open Roles",
    enabled: true,
    position: 3,
    config: { max_applicants: 10 },
    content: "We are seeking ROS developers and hardware designers.",
  };

  assert.equal(section.section_type, "recruitment");
  assert.equal(section.enabled, true);
  assert.equal(section.position, 3);
  assert.equal(section.config.max_applicants, 10);
});

test("ProjectResource models external and internal links", () => {
  const resource: ProjectResource = {
    id: "res-1",
    project_id: "p-100",
    label: "Hardware Schematics",
    url: "https://github.com/org/rover-hardware",
    resource_type: "github",
    created_at: "2026-01-03T00:00:00Z",
  };

  assert.equal(resource.resource_type, "github");
  assert.ok(resource.url.startsWith("https://"));
});

test("ProjectApplication models student proposals and answers", () => {
  const application: ProjectApplication = {
    id: "app-1",
    project_id: "p-100",
    applicant_id: "u-200",
    status: "submitted",
    statement: "Excited to contribute to firmware control systems.",
    resume_asset_key: "resumes/u-200.pdf",
    reviewer_notes: "Strong C++ background",
    created_at: "2026-01-04T00:00:00Z",
    applicant: {
      email: "applicant@university.edu",
      first_name: "Charlie",
      last_name: "Brown",
      department: "Mechanical Engineering",
    },
  };

  assert.equal(application.status, "submitted");
  assert.equal(application.applicant?.first_name, "Charlie");
});

test("ProjectInvitation models direct team invitations", () => {
  const invitation: ProjectInvitation = {
    id: "inv-1",
    project_id: "p-100",
    email: "collaborator@university.edu",
    role: "maintainer",
    status: "pending",
    created_at: "2026-01-05T00:00:00Z",
    expires_at: "2026-01-12T00:00:00Z",
  };

  assert.equal(invitation.role, "maintainer");
  assert.equal(invitation.status, "pending");
});

test("Payload DTOs validate create, update, and search shapes", () => {
  const createPayload: CreateProjectPayload = {
    title: "Campus AI Assistant",
    description: "LLM assistant for campus scheduling",
    category: "AI / ML",
    visibility: "public",
  };

  const updatePayload: UpdateProjectPayload = {
    recruitment_status: "paused",
    lifecycle_status: "completed",
    settings: { featured: true },
  };

  const visibilityPayload: UpdateVisibilityPayload = {
    visibility: "private",
  };

  const applyPayload: SubmitApplicationPayload = {
    statement: "I have experience with LangChain and Next.js.",
    resume_asset_key: "resumes/resume-key.pdf",
    answers: [{ question_id: "q-1", answer_text: "Available 10 hrs/week" }],
  };

  const invitePayload: InviteMemberPayload = {
    email: "newmember@university.edu",
    role: "member",
  };

  const resourcePayload: AddResourcePayload = {
    label: "Demo Video",
    url: "https://youtube.com/watch?v=123",
    resource_type: "video",
  };

  const filter: ProjectFilter = {
    category: "AI / ML",
    recruitment_status: "open",
    visibility: "public",
    lifecycle_status: "active",
    search: "campus",
    limit: 20,
    offset: 0,
  };

  assert.equal(createPayload.title, "Campus AI Assistant");
  assert.equal(updatePayload.recruitment_status, "paused");
  assert.equal(visibilityPayload.visibility, "private");
  assert.equal(applyPayload.answers?.[0].answer_text, "Available 10 hrs/week");
  assert.equal(invitePayload.role, "member");
  assert.equal(resourcePayload.resource_type, "video");
  assert.equal(filter.limit, 20);
});
