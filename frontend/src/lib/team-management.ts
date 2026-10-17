import type { ProjectMember } from "../types/project.ts";
import { isEduEmail } from "./email-validation.ts";

/**
 * Counts how many active leads exist in the project roster.
 */
export function countActiveLeads(members: ProjectMember[]): number {
  return members.filter((m) => m.role === "lead" && (!m.status || m.status === "active")).length;
}

/**
 * Checks whether a specific member is the sole active lead in the project.
 */
export function isSoleLead(members: ProjectMember[], memberId: string): boolean {
  const member = members.find((m) => m.member_id === memberId || m.id === memberId);
  if (!member || member.role !== "lead") {
    return false;
  }
  return countActiveLeads(members) <= 1;
}

/**
 * Evaluates whether a member can be demoted or removed.
 * Sole leads cannot be demoted or removed.
 */
export function canDemoteOrRemoveMember(
  members: ProjectMember[],
  memberId: string
): { allowed: boolean; reason?: string } {
  if (isSoleLead(members, memberId)) {
    return {
      allowed: false,
      reason: "A project must retain at least one Lead.",
    };
  }
  return { allowed: true };
}

/**
 * Evaluates whether leadership can be transferred to a target member.
 */
export function canTransferLeadership(
  members: ProjectMember[],
  currentUserId: string | undefined,
  targetMemberId: string
): { allowed: boolean; reason?: string } {
  const target = members.find((m) => m.member_id === targetMemberId || m.id === targetMemberId);

  if (!target) {
    return {
      allowed: false,
      reason: "Target member not found in project roster.",
    };
  }

  if (target.status && target.status !== "active") {
    return {
      allowed: false,
      reason: "Target member is not active.",
    };
  }

  if (target.role === "lead") {
    return {
      allowed: false,
      reason: "Target member is already a Lead.",
    };
  }

  if (currentUserId && (target.member_id === currentUserId || target.id === currentUserId)) {
    return {
      allowed: false,
      reason: "Cannot transfer leadership to yourself.",
    };
  }

  return { allowed: true };
}

/**
 * Validates an email address for project invitation:
 * - Must be non-empty
 * - Must be valid .edu domain
 * - Must not already belong to an active member
 */
export function validateInviteEmail(
  email: string,
  existingMembers: ProjectMember[] = []
): { valid: boolean; error?: string } {
  const trimmed = email.trim().toLowerCase();

  if (!trimmed) {
    return {
      valid: false,
      error: "Please enter an email address.",
    };
  }

  if (!isEduEmail(trimmed)) {
    return {
      valid: false,
      error: "Invitation requires an institutional .edu email address.",
    };
  }

  const isAlreadyMember = existingMembers.some((m) => {
    const memEmail = (m.email || m.member?.email || "").trim().toLowerCase();
    const isActive = !m.status || m.status === "active";
    return memEmail === trimmed && isActive;
  });

  if (isAlreadyMember) {
    return {
      valid: false,
      error: "This university member is already an active collaborator on this project.",
    };
  }

  return { valid: true };
}

/**
 * Resolves a human-friendly display name for a project member.
 */
export function getMemberDisplayName(member: ProjectMember): string {
  const firstName = member.member?.first_name || member.first_name || "";
  const lastName = member.member?.last_name || member.last_name || "";
  const fullName = `${firstName} ${lastName}`.trim();
  if (fullName) return fullName;
  if (member.email) return member.email;
  if (member.member?.email) return member.member.email;
  return "Team Member";
}

/**
 * Resolves email address for a project member.
 */
export function getMemberEmail(member: ProjectMember): string {
  return member.email || member.member?.email || "";
}

/**
 * Resolves department for a project member.
 */
export function getMemberDepartment(member: ProjectMember): string {
  return member.department || member.member?.department || "";
}
