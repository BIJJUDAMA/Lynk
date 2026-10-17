import test from "node:test";
import assert from "node:assert/strict";
import type { ApiClient } from "./api.ts";
import {
  listProjects,
  getProject,
  createProject,
  updateProject,
  updateProjectVisibility,
  listProjectMembers,
  updateMemberRole,
  removeMember,
  inviteMember,
  acceptInvitation,
  transferLeadership,
  getProjectSections,
  updateProjectSections,
  listProjectResources,
  addProjectResource,
  deleteProjectResource,
  submitProjectApplication,
  listProjectApplications,
  listMyApplications,
  getApplication,
  acceptProjectApplication,
  rejectProjectApplication,
  shortlistProjectApplication,
  withdrawProjectApplication,
  searchProjectsSemantic,
} from "./project-api.ts";
import type {
  Project,
  ProjectMember,
  ProjectSection,
  ProjectResource,
  ProjectApplication,
  ProjectInvitation,
  CreateProjectPayload,
  UpdateProjectPayload,
  SubmitApplicationPayload,
  InviteMemberPayload,
  AddResourcePayload,
  ProjectFilter,
} from "../types/project.ts";

interface RecordedCall {
  method: string;
  path: string;
  body?: unknown;
  params?: Record<string, unknown>;
}

function createMockClient() {
  const calls: RecordedCall[] = [];
  const responses: Record<string, unknown> = {};

  const client: ApiClient = {
    async get<T>(path: string, params?: Record<string, unknown>): Promise<T> {
      calls.push({ method: "GET", path, params });
      const key = `GET:${path}`;
      if (key in responses) {
        return responses[key] as T;
      }
      return (responses[path] ?? []) as T;
    },
    async post<T>(path: string, body?: unknown): Promise<T> {
      calls.push({ method: "POST", path, body });
      const key = `POST:${path}`;
      if (key in responses) {
        return responses[key] as T;
      }
      return (responses[path] ?? {}) as T;
    },
    async put<T>(path: string, body?: unknown): Promise<T> {
      calls.push({ method: "PUT", path, body });
      const key = `PUT:${path}`;
      if (key in responses) {
        return responses[key] as T;
      }
      return (responses[path] ?? {}) as T;
    },
    async patch<T>(path: string, body?: unknown): Promise<T> {
      calls.push({ method: "PATCH", path, body });
      const key = `PATCH:${path}`;
      if (key in responses) {
        return responses[key] as T;
      }
      return (responses[path] ?? {}) as T;
    },
    async delete<T>(path: string): Promise<T> {
      calls.push({ method: "DELETE", path });
      const key = `DELETE:${path}`;
      if (key in responses) {
        return responses[key] as T;
      }
      return (responses[path] ?? undefined) as T;
    },
    async uploadFile<T>(path: string, file: File): Promise<T> {
      calls.push({ method: "POST_FILE", path, body: file });
      return (responses[`POST_FILE:${path}`] ?? {}) as T;
    },
  };

  return { client, calls, responses };
}

const sampleProject: Project = {
  id: "p-1",
  slug: "project-alpha",
  owner_member_id: "m-1",
  title: "Project Alpha",
  description: "Innovative student project",
  category: "Computer Science",
  visibility: "public",
  recruitment_status: "open",
  lifecycle_status: "active",
  settings: {},
  created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z",
};

test("listProjects sends GET /projects with query parameters", async () => {
  const { client, calls, responses } = createMockClient();
  responses["GET:/projects"] = [sampleProject];

  const filter: ProjectFilter = {
    category: "Robotics",
    recruitment_status: "open",
    visibility: "public",
    lifecycle_status: "active",
    search: "rover",
    limit: 10,
    offset: 20,
  };

  const result = await listProjects(filter, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].path, "/projects");
  assert.deepEqual(calls[0].params, {
    category: "Robotics",
    recruitment_status: "open",
    visibility: "public",
    lifecycle_status: "active",
    search: "rover",
    limit: 10,
    offset: 20,
  });
  assert.equal(result.length, 1);
  assert.equal(result[0].id, "p-1");
});

test("listProjects supports owner_id filter for user workspaces", async () => {
  const { client, calls, responses } = createMockClient();
  responses["GET:/projects"] = [sampleProject];

  const result = await listProjects({ owner_id: "usr-42" }, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].path, "/projects");
  assert.deepEqual(calls[0].params, {
    owner_id: "usr-42",
  });
  assert.equal(result.length, 1);
});

test("getProject sends GET /projects/:slugOrId", async () => {
  const { client, calls, responses } = createMockClient();
  responses["GET:/projects/project-alpha"] = sampleProject;

  const result = await getProject("project-alpha", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].path, "/projects/project-alpha");
  assert.equal(result.title, "Project Alpha");
});

test("createProject sends POST /projects with CreateProjectPayload", async () => {
  const { client, calls, responses } = createMockClient();
  responses["POST:/projects"] = sampleProject;

  const payload: CreateProjectPayload = {
    title: "New Venture",
    description: "Startup workspace",
    category: "Design",
    visibility: "public",
  };

  const result = await createProject(payload, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].path, "/projects");
  assert.deepEqual(calls[0].body, payload);
  assert.equal(result.id, "p-1");
});

test("updateProject sends PATCH /projects/:id with UpdateProjectPayload", async () => {
  const { client, calls, responses } = createMockClient();
  responses["PATCH:/projects/p-1"] = {
    ...sampleProject,
    recruitment_status: "paused",
  };

  const payload: UpdateProjectPayload = {
    recruitment_status: "paused",
  };

  const result = await updateProject("p-1", payload, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "PATCH");
  assert.equal(calls[0].path, "/projects/p-1");
  assert.deepEqual(calls[0].body, payload);
  assert.equal(result.recruitment_status, "paused");
});

test("updateProjectVisibility sends PATCH /projects/:id/visibility", async () => {
  const { client, calls, responses } = createMockClient();
  responses["PATCH:/projects/p-1/visibility"] = {
    ...sampleProject,
    visibility: "unlisted",
  };

  const result = await updateProjectVisibility("p-1", "unlisted", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "PATCH");
  assert.equal(calls[0].path, "/projects/p-1/visibility");
  assert.deepEqual(calls[0].body, { visibility: "unlisted" });
  assert.equal(result.visibility, "unlisted");
});

test("listProjectMembers sends GET /projects/:projectId/members", async () => {
  const { client, calls, responses } = createMockClient();
  const members: ProjectMember[] = [
    {
      id: "pm-1",
      project_id: "p-1",
      member_id: "u-1",
      role: "lead",
      status: "active",
      joined_at: "2026-01-01T00:00:00Z",
    },
  ];
  responses["GET:/projects/p-1/members"] = members;

  const result = await listProjectMembers("p-1", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].path, "/projects/p-1/members");
  assert.equal(result.length, 1);
  assert.equal(result[0].role, "lead");
});

test("updateMemberRole sends PATCH /projects/:projectId/members/:memberId", async () => {
  const { client, calls } = createMockClient();

  await updateMemberRole("p-1", "u-2", "maintainer", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "PATCH");
  assert.equal(calls[0].path, "/projects/p-1/members/u-2");
  assert.deepEqual(calls[0].body, { role: "maintainer" });
});

test("removeMember sends DELETE /projects/:projectId/members/:memberId", async () => {
  const { client, calls } = createMockClient();

  await removeMember("p-1", "u-3", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "DELETE");
  assert.equal(calls[0].path, "/projects/p-1/members/u-3");
});

test("inviteMember sends POST /projects/:projectId/invitations with mapped body", async () => {
  const { client, calls, responses } = createMockClient();
  const invitation: ProjectInvitation = {
    id: "inv-1",
    project_id: "p-1",
    email: "partner@university.edu",
    role: "member",
    status: "pending",
    created_at: "2026-01-01T00:00:00Z",
    expires_at: "2026-01-08T00:00:00Z",
  };
  responses["POST:/projects/p-1/invitations"] = invitation;

  const payload: InviteMemberPayload = {
    email: "partner@university.edu",
    role: "member",
  };

  const result = await inviteMember("p-1", payload, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].path, "/projects/p-1/invitations");
  assert.equal(result.id, "inv-1");
});

test("acceptInvitation sends POST /invitations/:invitationId/accept", async () => {
  const { client, calls } = createMockClient();

  await acceptInvitation("inv-100", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].path, "/invitations/inv-100/accept");
});

test("transferLeadership sends POST /projects/:projectId/transfer-leadership", async () => {
  const { client, calls } = createMockClient();

  await transferLeadership("p-1", "u-next-lead", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].path, "/projects/p-1/transfer-leadership");
  assert.deepEqual(calls[0].body, { new_lead_id: "u-next-lead" });
});

test("getProjectSections sends GET /projects/:projectId/sections", async () => {
  const { client, calls, responses } = createMockClient();
  const sections: ProjectSection[] = [
    {
      id: "sec-1",
      project_id: "p-1",
      section_type: "overview",
      title: "Overview",
      enabled: true,
      position: 0,
      config: {},
      content: "Intro content",
    },
  ];
  responses["GET:/projects/p-1/sections"] = sections;

  const result = await getProjectSections("p-1", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].path, "/projects/p-1/sections");
  assert.equal(result.length, 1);
  assert.equal(result[0].section_type, "overview");
});

test("updateProjectSections returns array directly when returned by server", async () => {
  const { client, calls, responses } = createMockClient();
  const sections: ProjectSection[] = [
    {
      id: "sec-1",
      project_id: "p-1",
      section_type: "team",
      title: "Team",
      enabled: true,
      position: 1,
      config: {},
      content: "Team list",
    },
  ];
  responses["PUT:/projects/p-1/sections"] = sections;

  const result = await updateProjectSections("p-1", sections, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "PUT");
  assert.equal(calls[0].path, "/projects/p-1/sections");
  assert.deepEqual(result, sections);
});

test("updateProjectSections re-fetches sections if server returns message", async () => {
  const { client, calls, responses } = createMockClient();
  const sections: ProjectSection[] = [
    {
      id: "sec-2",
      project_id: "p-1",
      section_type: "docs",
      title: "Documentation",
      enabled: true,
      position: 2,
      config: {},
      content: "Docs content",
    },
  ];
  responses["PUT:/projects/p-1/sections"] = {
    message: "sections updated successfully",
  };
  responses["GET:/projects/p-1/sections"] = sections;

  const result = await updateProjectSections("p-1", sections, client);
  assert.equal(calls.length, 2);
  assert.equal(calls[0].method, "PUT");
  assert.equal(calls[1].method, "GET");
  assert.deepEqual(result, sections);
});

test("listProjectResources and add/delete resource operate on /projects/:id/resources", async () => {
  const { client, calls, responses } = createMockClient();
  const resource: ProjectResource = {
    id: "res-1",
    project_id: "p-1",
    label: "Figma Link",
    url: "https://figma.com/file/123",
    resource_type: "design",
    created_at: "2026-01-01T00:00:00Z",
  };
  responses["GET:/projects/p-1/resources"] = [resource];
  responses["POST:/projects/p-1/resources"] = resource;

  const list = await listProjectResources("p-1", client);
  assert.equal(list.length, 1);

  const payload: AddResourcePayload = {
    label: "Figma Link",
    url: "https://figma.com/file/123",
    resource_type: "design",
  };
  const created = await addProjectResource("p-1", payload, client);
  assert.equal(created.id, "res-1");

  await deleteProjectResource("p-1", "res-1", client);
  assert.equal(calls[2].method, "DELETE");
  assert.equal(calls[2].path, "/projects/p-1/resources/res-1");
});

test("submitProjectApplication sends POST /projects/:projectId/applications", async () => {
  const { client, calls, responses } = createMockClient();
  const application: ProjectApplication = {
    id: "app-1",
    project_id: "p-1",
    applicant_id: "u-9",
    status: "submitted",
    statement: "Ready to work",
    created_at: "2026-01-01T00:00:00Z",
  };
  responses["POST:/projects/p-1/applications"] = application;

  const payload: SubmitApplicationPayload = {
    statement: "Ready to work",
  };

  const result = await submitProjectApplication("p-1", payload, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "POST");
  assert.equal(calls[0].path, "/projects/p-1/applications");
  assert.equal(result.id, "app-1");
});

test("listProjectApplications sends GET /projects/:projectId/applications", async () => {
  const { client, calls, responses } = createMockClient();
  responses["GET:/projects/p-1/applications"] = [];

  const result = await listProjectApplications("p-1", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].path, "/projects/p-1/applications");
  assert.deepEqual(result, []);
});

test("listMyApplications sends GET /applications/me", async () => {
  const { client, calls, responses } = createMockClient();
  responses["GET:/applications/me"] = [];

  const result = await listMyApplications(client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].path, "/applications/me");
  assert.deepEqual(result, []);
});

test("getApplication sends GET /applications/:id", async () => {
  const { client, calls, responses } = createMockClient();
  const application: ProjectApplication = {
    id: "app-10",
    project_id: "p-1",
    applicant_id: "u-9",
    status: "accepted",
    statement: "Accepted statement",
    created_at: "2026-01-01T00:00:00Z",
  };
  responses["GET:/applications/app-10"] = application;

  const result = await getApplication("app-10", client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].method, "GET");
  assert.equal(calls[0].path, "/applications/app-10");
  assert.equal(result.status, "accepted");
});

test("application review actions (accept, reject, shortlist, withdraw) execute correctly", async () => {
  const { client, calls, responses } = createMockClient();
  const updatedApp: ProjectApplication = {
    id: "app-20",
    project_id: "p-1",
    applicant_id: "u-12",
    status: "accepted",
    statement: "I am ready",
    reviewer_notes: "Accepted to team",
    created_at: "2026-01-01T00:00:00Z",
  };

  responses["POST:/applications/app-20/accept"] = updatedApp;
  responses["POST:/applications/app-20/reject"] = {
    ...updatedApp,
    status: "rejected",
  };
  responses["POST:/applications/app-20/shortlist"] = {
    ...updatedApp,
    status: "shortlisted",
  };
  responses["POST:/applications/app-20/withdraw"] = {
    ...updatedApp,
    status: "withdrawn",
  };

  const accepted = await acceptProjectApplication("app-20", "Accepted to team", client);
  assert.equal(accepted.status, "accepted");
  assert.equal(calls[0].path, "/applications/app-20/accept");
  assert.deepEqual(calls[0].body, { reviewer_notes: "Accepted to team" });

  const rejected = await rejectProjectApplication("app-20", "Over capacity", client);
  assert.equal(rejected.status, "rejected");
  assert.equal(calls[1].path, "/applications/app-20/reject");

  const shortlisted = await shortlistProjectApplication("app-20", "Strong candidate", client);
  assert.equal(shortlisted.status, "shortlisted");
  assert.equal(calls[2].path, "/applications/app-20/shortlist");

  const withdrawn = await withdrawProjectApplication("app-20", client);
  assert.equal(withdrawn.status, "withdrawn");
  assert.equal(calls[3].path, "/applications/app-20/withdraw");
});

test("searchProjectsSemantic calls /search/projects and normalizes ProjectMatch items", async () => {
  const { client, calls, responses } = createMockClient();
  responses["GET:/search/projects"] = [
    {
      project_id: "p-50",
      slug: "ai-search-bot",
      title: "AI Search Bot",
      description: "Semantic search engine",
      category: "AI",
      visibility: "public",
      similarity: 0.92,
      distance: 0.08,
    },
  ];

  const results = await searchProjectsSemantic("semantic AI", 5, client);
  assert.equal(calls.length, 1);
  assert.equal(calls[0].path, "/search/projects");
  assert.deepEqual(calls[0].params, { q: "semantic AI", limit: 5 });
  assert.equal(results.length, 1);
  assert.equal(results[0].id, "p-50");
  assert.equal(results[0].slug, "ai-search-bot");
  assert.equal(results[0].similarity, 0.92);
});
