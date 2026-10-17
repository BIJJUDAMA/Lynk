/**
 * Lynk Project-First API Client Library
 *
 * Provides type-safe domain API functions interfacing with Go REST API endpoints
 * for project discovery, workspace sections, team management, and recruitment.
 */

import type { ApiClient } from "./api.ts";
import { apiClient } from "./api.ts";
import type {
  Project,
  ProjectMember,
  ProjectSection,
  ProjectResource,
  ProjectApplication,
  ProjectInvitation,
  CreateProjectPayload,
  UpdateProjectPayload,
  ProjectVisibility,
  ProjectRole,
  SubmitApplicationPayload,
  InviteMemberPayload,
  AddResourcePayload,
  ProjectFilter,
} from "../types/project.ts";

// ============================================================================
// Project Discovery & Management
// ============================================================================

export async function listProjects(
  filter?: ProjectFilter,
  client: ApiClient = apiClient
): Promise<Project[]> {
  const queryParams: Record<string, unknown> = {};

  if (filter) {
    if (filter.category) queryParams.category = filter.category;
    if (filter.recruitment_status) {
      queryParams.recruitment_status = filter.recruitment_status;
    }
    if (filter.visibility) queryParams.visibility = filter.visibility;
    if (filter.lifecycle_status) {
      queryParams.lifecycle_status = filter.lifecycle_status;
    }
    if (filter.search) queryParams.search = filter.search;
    if (filter.owner_id) queryParams.owner_id = filter.owner_id;
    if (filter.limit !== undefined) queryParams.limit = filter.limit;
    if (filter.offset !== undefined) queryParams.offset = filter.offset;
  }

  const result = await client.get<Project[]>("/projects", queryParams);
  return result ?? [];
}

export async function getProject(
  slugOrId: string,
  client: ApiClient = apiClient
): Promise<Project> {
  return client.get<Project>(`/projects/${encodeURIComponent(slugOrId)}`);
}

export async function createProject(
  payload: CreateProjectPayload,
  client: ApiClient = apiClient
): Promise<Project> {
  return client.post<Project>("/projects", payload);
}

export async function updateProject(
  id: string,
  payload: UpdateProjectPayload,
  client: ApiClient = apiClient
): Promise<Project> {
  return client.patch<Project>(`/projects/${encodeURIComponent(id)}`, payload);
}

export async function updateProjectVisibility(
  id: string,
  visibility: ProjectVisibility,
  client: ApiClient = apiClient
): Promise<Project> {
  return client.patch<Project>(`/projects/${encodeURIComponent(id)}/visibility`, { visibility });
}

// ============================================================================
// Team & Collaborator Management
// ============================================================================

export async function listProjectMembers(
  projectId: string,
  client: ApiClient = apiClient
): Promise<ProjectMember[]> {
  const result = await client.get<ProjectMember[]>(
    `/projects/${encodeURIComponent(projectId)}/members`
  );
  return result ?? [];
}

export async function updateMemberRole(
  projectId: string,
  memberId: string,
  role: ProjectRole,
  client: ApiClient = apiClient
): Promise<void> {
  await client.patch<void>(
    `/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(memberId)}`,
    { role }
  );
}

export async function removeMember(
  projectId: string,
  memberId: string,
  client: ApiClient = apiClient
): Promise<void> {
  await client.delete<void>(
    `/projects/${encodeURIComponent(projectId)}/members/${encodeURIComponent(memberId)}`
  );
}

export async function inviteMember(
  projectId: string,
  payload: InviteMemberPayload,
  client: ApiClient = apiClient
): Promise<ProjectInvitation> {
  const body: Record<string, unknown> = {
    email: payload.email,
    role: payload.role,
    invitee_id: payload.invitee_id || payload.email,
    proposed_role: payload.proposed_role || payload.role,
  };

  return client.post<ProjectInvitation>(
    `/projects/${encodeURIComponent(projectId)}/invitations`,
    body
  );
}

export async function acceptInvitation(
  invitationId: string,
  client: ApiClient = apiClient
): Promise<void> {
  await client.post<void>(`/invitations/${encodeURIComponent(invitationId)}/accept`);
}

export async function transferLeadership(
  projectId: string,
  newLeadId: string,
  client: ApiClient = apiClient
): Promise<void> {
  await client.post<void>(`/projects/${encodeURIComponent(projectId)}/transfer-leadership`, {
    new_lead_id: newLeadId,
  });
}

// ============================================================================
// Workspace Sections & Resources
// ============================================================================

export async function getProjectSections(
  projectId: string,
  client: ApiClient = apiClient
): Promise<ProjectSection[]> {
  const result = await client.get<ProjectSection[]>(
    `/projects/${encodeURIComponent(projectId)}/sections`
  );
  return result ?? [];
}

export async function updateProjectSections(
  projectId: string,
  sections: ProjectSection[],
  client: ApiClient = apiClient
): Promise<ProjectSection[]> {
  const result = await client.put<ProjectSection[] | { message?: string }>(
    `/projects/${encodeURIComponent(projectId)}/sections`,
    sections
  );

  if (Array.isArray(result)) {
    return result;
  }
  return getProjectSections(projectId, client);
}

export async function listProjectResources(
  projectId: string,
  client: ApiClient = apiClient
): Promise<ProjectResource[]> {
  const result = await client.get<ProjectResource[]>(
    `/projects/${encodeURIComponent(projectId)}/resources`
  );
  return result ?? [];
}

export async function addProjectResource(
  projectId: string,
  payload: AddResourcePayload,
  client: ApiClient = apiClient
): Promise<ProjectResource> {
  return client.post<ProjectResource>(
    `/projects/${encodeURIComponent(projectId)}/resources`,
    payload
  );
}

export async function deleteProjectResource(
  projectId: string,
  resourceId: string,
  client: ApiClient = apiClient
): Promise<void> {
  await client.delete<void>(
    `/projects/${encodeURIComponent(projectId)}/resources/${encodeURIComponent(resourceId)}`
  );
}

// ============================================================================
// Recruitment & Applications
// ============================================================================

export async function submitProjectApplication(
  projectId: string,
  payload: SubmitApplicationPayload,
  client: ApiClient = apiClient
): Promise<ProjectApplication> {
  return client.post<ProjectApplication>(
    `/projects/${encodeURIComponent(projectId)}/applications`,
    payload
  );
}

export async function listProjectApplications(
  projectId: string,
  client: ApiClient = apiClient
): Promise<ProjectApplication[]> {
  const result = await client.get<ProjectApplication[]>(
    `/projects/${encodeURIComponent(projectId)}/applications`
  );
  return result ?? [];
}

export async function listMyApplications(
  client: ApiClient = apiClient
): Promise<ProjectApplication[]> {
  const result = await client.get<ProjectApplication[]>("/applications/me");
  return result ?? [];
}

export async function getApplication(
  id: string,
  client: ApiClient = apiClient
): Promise<ProjectApplication> {
  return client.get<ProjectApplication>(`/applications/${encodeURIComponent(id)}`);
}

export async function acceptProjectApplication(
  id: string,
  reviewerNotes?: string,
  client: ApiClient = apiClient
): Promise<ProjectApplication> {
  const body = reviewerNotes ? { reviewer_notes: reviewerNotes } : {};
  const result = await client.post<ProjectApplication | { message?: string }>(
    `/applications/${encodeURIComponent(id)}/accept`,
    body
  );

  if (result && typeof result === "object" && "id" in result) {
    return result as ProjectApplication;
  }
  return getApplication(id, client);
}

export async function rejectProjectApplication(
  id: string,
  reviewerNotes?: string,
  client: ApiClient = apiClient
): Promise<ProjectApplication> {
  const body = reviewerNotes ? { reviewer_notes: reviewerNotes } : {};
  const result = await client.post<ProjectApplication | { message?: string }>(
    `/applications/${encodeURIComponent(id)}/reject`,
    body
  );

  if (result && typeof result === "object" && "id" in result) {
    return result as ProjectApplication;
  }
  return getApplication(id, client);
}

export async function shortlistProjectApplication(
  id: string,
  reviewerNotes?: string,
  client: ApiClient = apiClient
): Promise<ProjectApplication> {
  const body = reviewerNotes ? { reviewer_notes: reviewerNotes } : {};
  const result = await client.post<ProjectApplication | { message?: string }>(
    `/applications/${encodeURIComponent(id)}/shortlist`,
    body
  );

  if (result && typeof result === "object" && "id" in result) {
    return result as ProjectApplication;
  }
  return getApplication(id, client);
}

export async function withdrawProjectApplication(
  id: string,
  client: ApiClient = apiClient
): Promise<ProjectApplication> {
  const result = await client.post<ProjectApplication | { message?: string }>(
    `/applications/${encodeURIComponent(id)}/withdraw`,
    {}
  );

  if (result && typeof result === "object" && "id" in result) {
    return result as ProjectApplication;
  }
  return getApplication(id, client);
}

export async function searchProjectsSemantic(
  query: string,
  limit?: number,
  client: ApiClient = apiClient
): Promise<Project[]> {
  const params: Record<string, unknown> = { q: query };
  if (limit !== undefined) {
    params.limit = limit;
  }

  const raw = await client.get<
    Array<
      Project & {
        project_id?: string;
        similarity?: number;
        distance?: number;
      }
    >
  >("/search/projects", params);

  if (!Array.isArray(raw)) {
    return [];
  }

  return raw.map((item) => {
    if (item && item.project_id && !item.id) {
      return {
        id: item.project_id,
        slug: item.slug ?? "",
        owner_member_id: item.owner_member_id ?? "",
        title: item.title ?? "",
        description: item.description ?? "",
        category: item.category ?? "",
        visibility: item.visibility ?? "public",
        recruitment_status: item.recruitment_status ?? "open",
        lifecycle_status: item.lifecycle_status ?? "active",
        settings: item.settings ?? {},
        created_at: item.created_at ?? new Date().toISOString(),
        updated_at: item.updated_at ?? new Date().toISOString(),
        similarity: item.similarity,
        distance: item.distance,
      };
    }
    return item;
  });
}
