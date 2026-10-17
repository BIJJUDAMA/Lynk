import { test } from "node:test";
import assert from "node:assert/strict";
import type { ProjectMember } from "../types/project.ts";
import {
  countActiveLeads,
  isSoleLead,
  canDemoteOrRemoveMember,
  canTransferLeadership,
  validateInviteEmail,
  getMemberDisplayName,
} from "./team-management.ts";

const mockMembers: ProjectMember[] = [
  {
    id: "mem-1",
    project_id: "proj-1",
    member_id: "user-1",
    role: "lead",
    status: "active",
    joined_at: "2026-10-01T00:00:00Z",
    email: "lead@mit.edu",
    first_name: "Alice",
    last_name: "Smith",
    department: "Computer Science",
  },
  {
    id: "mem-2",
    project_id: "proj-1",
    member_id: "user-2",
    role: "maintainer",
    status: "active",
    joined_at: "2026-10-02T00:00:00Z",
    email: "maintainer@mit.edu",
    first_name: "Bob",
    last_name: "Jones",
    department: "Electrical Engineering",
  },
  {
    id: "mem-3",
    project_id: "proj-1",
    member_id: "user-3",
    role: "member",
    status: "active",
    joined_at: "2026-10-03T00:00:00Z",
    email: "collab@mit.edu",
    member: {
      email: "collab@mit.edu",
      first_name: "Charlie",
      last_name: "Brown",
      department: "Physics",
    },
  },
];

test("countActiveLeads accurately counts active leads", () => {
  assert.equal(countActiveLeads(mockMembers), 1);

  const twoLeads: ProjectMember[] = [
    ...mockMembers,
    {
      id: "mem-4",
      project_id: "proj-1",
      member_id: "user-4",
      role: "lead",
      status: "active",
      joined_at: "2026-10-04T00:00:00Z",
      email: "colead@mit.edu",
    },
  ];
  assert.equal(countActiveLeads(twoLeads), 2);
});

test("isSoleLead protects single lead project configuration", () => {
  assert.equal(isSoleLead(mockMembers, "user-1"), true);
  assert.equal(isSoleLead(mockMembers, "user-2"), false);

  const twoLeads: ProjectMember[] = [
    ...mockMembers,
    {
      id: "mem-4",
      project_id: "proj-1",
      member_id: "user-4",
      role: "lead",
      status: "active",
      joined_at: "2026-10-04T00:00:00Z",
      email: "colead@mit.edu",
    },
  ];
  assert.equal(isSoleLead(twoLeads, "user-1"), false);
  assert.equal(isSoleLead(twoLeads, "user-4"), false);
});

test("canDemoteOrRemoveMember denies action for sole lead and allows for others", () => {
  const soleLeadCheck = canDemoteOrRemoveMember(mockMembers, "user-1");
  assert.equal(soleLeadCheck.allowed, false);
  assert.match(soleLeadCheck.reason || "", /at least one Lead/);

  const maintainerCheck = canDemoteOrRemoveMember(mockMembers, "user-2");
  assert.equal(maintainerCheck.allowed, true);
});

test("canTransferLeadership validates recipient validity", () => {
  // Valid transfer to member
  const validTransfer = canTransferLeadership(mockMembers, "user-1", "user-3");
  assert.equal(validTransfer.allowed, true);

  // Invalid: cannot transfer to non-existent member
  const nonExistent = canTransferLeadership(mockMembers, "user-1", "unknown-user");
  assert.equal(nonExistent.allowed, false);
  assert.match(nonExistent.reason || "", /not found/);

  // Invalid: cannot transfer to existing lead
  const alreadyLead = canTransferLeadership(mockMembers, "user-2", "user-1");
  assert.equal(alreadyLead.allowed, false);
  assert.match(alreadyLead.reason || "", /already a Lead/);

  // Invalid: cannot transfer to yourself
  const selfTransfer = canTransferLeadership(mockMembers, "user-2", "user-2");
  assert.equal(selfTransfer.allowed, false);
  assert.match(selfTransfer.reason || "", /to yourself/);
});

test("validateInviteEmail checks format and active membership collision", () => {
  assert.equal(validateInviteEmail("").valid, false);
  assert.equal(validateInviteEmail("someone@gmail.com").valid, false);
  assert.equal(validateInviteEmail("lead@mit.edu", mockMembers).valid, false);
  assert.equal(validateInviteEmail("collab@mit.edu", mockMembers).valid, false);

  const validNew = validateInviteEmail("newstudent@harvard.edu", mockMembers);
  assert.equal(validNew.valid, true);
  assert.equal(validNew.error, undefined);
});

test("getMemberDisplayName extracts name or fallback correctly", () => {
  assert.equal(getMemberDisplayName(mockMembers[0]), "Alice Smith");
  assert.equal(getMemberDisplayName(mockMembers[2]), "Charlie Brown");

  const noName: ProjectMember = {
    id: "mem-x",
    project_id: "proj-1",
    member_id: "user-x",
    role: "viewer",
    status: "active",
    joined_at: "2026-10-01T00:00:00Z",
    email: "anon@mit.edu",
  };
  assert.equal(getMemberDisplayName(noName), "anon@mit.edu");
});
