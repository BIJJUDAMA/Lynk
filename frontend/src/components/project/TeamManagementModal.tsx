"use client";

import React, { useState } from "react";
import {
  Users,
  UserPlus,
  Crown,
  Trash2,
  AlertCircle,
  CheckCircle2,
  Loader2,
  Calendar,
  GraduationCap,
  ShieldAlert,
  Send,
  HelpCircle,
} from "lucide-react";
import type { Project, ProjectMember, ProjectRole } from "@/types/project";
import {
  updateMemberRole,
  removeMember,
  transferLeadership,
  inviteMember,
} from "@/lib/project-api";
import { ApiClientError } from "@/lib/api";
import { formatJobDate } from "@/lib/formatters";
import {
  isSoleLead,
  canDemoteOrRemoveMember,
  canTransferLeadership,
  validateInviteEmail,
  getMemberDisplayName,
  getMemberEmail,
  getMemberDepartment,
} from "@/lib/team-management";
import { cn } from "@/lib/utils";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";

export interface TeamManagementModalProps {
  open: boolean;
  onClose: () => void;
  project: Project;
  members: ProjectMember[];
  currentUserId?: string;
  currentUserEmail?: string;
  isLead: boolean;
  onMembersChange?: () => void;
}

type ModalTab = "roster" | "invite";

function getInitials(name?: string): string {
  if (!name) return "U";
  const parts = name.trim().split(/\s+/);
  if (parts.length === 1) {
    return parts[0].slice(0, 2).toUpperCase();
  }
  return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase();
}

function getRoleBadgeClasses(role: ProjectRole): {
  bg: string;
  text: string;
  border: string;
  label: string;
} {
  switch (role) {
    case "lead":
      return {
        bg: "bg-purple-500/10",
        text: "text-purple-600",
        border: "border-purple-500/20",
        label: "Lead",
      };
    case "maintainer":
      return {
        bg: "bg-indigo-500/10",
        text: "text-indigo-600",
        border: "border-indigo-500/20",
        label: "Maintainer",
      };
    case "member":
      return {
        bg: "bg-emerald-500/10",
        text: "text-emerald-600",
        border: "border-emerald-500/20",
        label: "Member",
      };
    case "viewer":
    default:
      return {
        bg: "bg-muted",
        text: "text-muted-foreground",
        border: "border-border",
        label: "Viewer",
      };
  }
}

export function TeamManagementModal({
  open,
  onClose,
  project,
  members,
  currentUserId,
  currentUserEmail,
  isLead,
  onMembersChange,
}: TeamManagementModalProps) {
  const [activeTab, setActiveTab] = useState<ModalTab>("roster");

  // In-flight operation states
  const [updatingMemberId, setUpdatingMemberId] = useState<string | null>(null);
  const [removingMemberId, setRemovingMemberId] = useState<string | null>(null);
  const [isTransferring, setIsTransferring] = useState<boolean>(false);
  const [isSendingInvite, setIsSendingInvite] = useState<boolean>(false);

  // Confirmation targets
  const [transferTarget, setTransferTarget] = useState<ProjectMember | null>(null);
  const [removeTarget, setRemoveTarget] = useState<ProjectMember | null>(null);

  // Invitation form state
  const [inviteEmail, setInviteEmail] = useState<string>("");
  const [inviteRole, setInviteRole] = useState<ProjectRole>("member");
  const [inviteEmailError, setInviteEmailError] = useState<string | null>(null);

  // Global modal alerts
  const [feedback, setFeedback] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const activeMembers = members.filter((m) => !m.status || m.status === "active");

  const handleRoleChange = async (member: ProjectMember, newRole: ProjectRole) => {
    if (member.role === newRole) return;
    const memberId = member.member_id || member.id;

    const protection = canDemoteOrRemoveMember(members, memberId);
    if (!protection.allowed && newRole !== "lead") {
      setFeedback({
        type: "error",
        message: protection.reason || "A project must retain at least one Lead.",
      });
      return;
    }

    setUpdatingMemberId(memberId);
    setFeedback(null);

    try {
      await updateMemberRole(project.id, memberId, newRole);
      setFeedback({
        type: "success",
        message: `Updated role for ${getMemberDisplayName(member)} to ${newRole}.`,
      });
      onMembersChange?.();
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setFeedback({ type: "error", message: err.message });
      } else if (err instanceof Error) {
        setFeedback({ type: "error", message: err.message });
      } else {
        setFeedback({ type: "error", message: "Failed to update member role." });
      }
    } finally {
      setUpdatingMemberId(null);
    }
  };

  const handleConfirmRemove = async () => {
    if (!removeTarget) return;
    const memberId = removeTarget.member_id || removeTarget.id;

    const protection = canDemoteOrRemoveMember(members, memberId);
    if (!protection.allowed) {
      setFeedback({
        type: "error",
        message: protection.reason || "A project must retain at least one Lead.",
      });
      setRemoveTarget(null);
      return;
    }

    setRemovingMemberId(memberId);
    setFeedback(null);

    try {
      await removeMember(project.id, memberId);
      setFeedback({
        type: "success",
        message: `Removed ${getMemberDisplayName(removeTarget)} from the project team.`,
      });
      setRemoveTarget(null);
      onMembersChange?.();
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setFeedback({ type: "error", message: err.message });
      } else if (err instanceof Error) {
        setFeedback({ type: "error", message: err.message });
      } else {
        setFeedback({ type: "error", message: "Failed to remove member." });
      }
    } finally {
      setRemovingMemberId(null);
    }
  };

  const handleConfirmTransfer = async () => {
    if (!transferTarget) return;
    const targetMemberId = transferTarget.member_id || transferTarget.id;

    const check = canTransferLeadership(members, currentUserId, targetMemberId);
    if (!check.allowed) {
      setFeedback({
        type: "error",
        message: check.reason || "Cannot transfer leadership to this member.",
      });
      setTransferTarget(null);
      return;
    }

    setIsTransferring(true);
    setFeedback(null);

    try {
      await transferLeadership(project.id, targetMemberId);
      setFeedback({
        type: "success",
        message: `Leadership successfully transferred to ${getMemberDisplayName(transferTarget)}.`,
      });
      setTransferTarget(null);
      onMembersChange?.();
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setFeedback({ type: "error", message: err.message });
      } else if (err instanceof Error) {
        setFeedback({ type: "error", message: err.message });
      } else {
        setFeedback({ type: "error", message: "Failed to transfer leadership." });
      }
    } finally {
      setIsTransferring(false);
    }
  };

  const handleSendInvite = async (e: React.FormEvent) => {
    e.preventDefault();
    setInviteEmailError(null);
    setFeedback(null);

    const validation = validateInviteEmail(inviteEmail, members);
    if (!validation.valid) {
      setInviteEmailError(validation.error || "Invalid email address.");
      return;
    }

    setIsSendingInvite(true);

    try {
      await inviteMember(project.id, {
        email: inviteEmail.trim().toLowerCase(),
        role: inviteRole,
      });

      setFeedback({
        type: "success",
        message: `Invitation successfully sent to ${inviteEmail.trim().toLowerCase()} as ${inviteRole}.`,
      });
      setInviteEmail("");
      setInviteRole("member");
      onMembersChange?.();
    } catch (err: unknown) {
      if (err instanceof ApiClientError) {
        setFeedback({ type: "error", message: err.message });
      } else if (err instanceof Error) {
        setFeedback({ type: "error", message: err.message });
      } else {
        setFeedback({ type: "error", message: "Failed to send invitation." });
      }
    } finally {
      setIsSendingInvite(false);
    }
  };

  return (
    <Dialog open={open} onOpenChange={onClose} className="max-w-3xl">
      <DialogContent
        onClose={onClose}
        className="max-w-3xl p-0 overflow-hidden flex flex-col max-h-[85vh]"
      >
        {/* Header */}
        <div className="border-b border-border p-6 pb-4">
          <DialogHeader className="mb-0">
            <div className="flex items-center gap-2">
              <Users className="h-5 w-5 text-foreground" />
              <DialogTitle>Team & Collaborators</DialogTitle>
            </div>
            <DialogDescription className="font-mono text-xs text-muted-foreground mt-1">
              Workspace: <span className="font-semibold text-foreground">{project.title}</span>
            </DialogDescription>
          </DialogHeader>

          {/* Tab Navigation */}
          <div className="mt-4 flex items-center gap-2 border-b border-border/60">
            <button
              type="button"
              onClick={() => {
                setActiveTab("roster");
                setFeedback(null);
              }}
              className={cn(
                "inline-flex items-center gap-1.5 border-b-2 px-3 py-2 font-mono text-xs font-medium transition-colors",
                activeTab === "roster"
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              <Users className="h-3.5 w-3.5" />
              <span>Roster</span>
              <span className="ml-1 rounded-full bg-muted px-1.5 py-0.2 text-[10px] text-muted-foreground">
                {activeMembers.length}
              </span>
            </button>

            <button
              type="button"
              onClick={() => {
                setActiveTab("invite");
                setFeedback(null);
              }}
              className={cn(
                "inline-flex items-center gap-1.5 border-b-2 px-3 py-2 font-mono text-xs font-medium transition-colors",
                activeTab === "invite"
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              )}
            >
              <UserPlus className="h-3.5 w-3.5" />
              <span>Invite Collaborators</span>
            </button>
          </div>
        </div>

        {/* Global Modal Alert Banner */}
        {feedback && (
          <div
            className={cn(
              "mx-6 mt-4 flex items-start gap-2.5 rounded-lg border p-3 text-xs",
              feedback.type === "success"
                ? "border-emerald-500/20 bg-emerald-500/10 text-emerald-700"
                : "border-pastel-redText/20 bg-pastel-red text-pastel-redText"
            )}
          >
            {feedback.type === "success" ? (
              <CheckCircle2 className="h-4 w-4 shrink-0 mt-0.5" />
            ) : (
              <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
            )}
            <span className="flex-1">{feedback.message}</span>
            <button
              type="button"
              onClick={() => setFeedback(null)}
              className="text-xs opacity-60 hover:opacity-100"
            >
              Dismiss
            </button>
          </div>
        )}

        {/* Main Body with Clean Scroll */}
        <div className="flex-1 overflow-y-auto p-6 space-y-6">
          {activeTab === "roster" && (
            <div className="space-y-4">
              {!isLead && (
                <div className="flex items-center gap-2 rounded-lg border border-border/80 bg-muted/30 p-3 text-xs text-muted-foreground">
                  <HelpCircle className="h-4 w-4 shrink-0" />
                  <span>
                    Viewing as Maintainer. Only Workspace Leads can adjust roles, remove
                    collaborators, or transfer ownership.
                  </span>
                </div>
              )}

              {/* Roster Listing */}
              {activeMembers.length === 0 ? (
                <div className="py-12 text-center">
                  <Users className="mx-auto h-8 w-8 text-muted-foreground/60" />
                  <p className="mt-2 text-sm text-muted-foreground">
                    No active collaborators found on this project.
                  </p>
                </div>
              ) : (
                <div className="divide-y divide-border rounded-lg border border-border bg-card">
                  {activeMembers.map((member) => {
                    const memberId = member.member_id || member.id;
                    const displayName = getMemberDisplayName(member);
                    const email = getMemberEmail(member);
                    const department = getMemberDepartment(member);
                    const initials = getInitials(displayName);
                    const isSole = isSoleLead(members, memberId);
                    const roleBadge = getRoleBadgeClasses(member.role);
                    const isCurrentUser =
                      Boolean(currentUserId && member.member_id === currentUserId) ||
                      Boolean(
                        currentUserEmail && email.toLowerCase() === currentUserEmail.toLowerCase()
                      );
                    const joinedLabel = member.joined_at ? formatJobDate(member.joined_at) : null;
                    const isUpdating = updatingMemberId === memberId;
                    const isRemoving = removingMemberId === memberId;

                    return (
                      <div
                        key={memberId}
                        className="flex flex-col gap-3 p-4 sm:flex-row sm:items-center sm:justify-between transition-colors hover:bg-muted/20"
                      >
                        {/* Member Identity Details */}
                        <div className="flex items-start gap-3 min-w-0">
                          <div className="flex h-10 w-10 shrink-0 items-center justify-center rounded-full border border-border bg-surface font-mono text-xs font-semibold text-foreground">
                            {initials}
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="flex flex-wrap items-center gap-2">
                              <span className="text-sm font-medium text-foreground truncate">
                                {displayName}
                              </span>
                              {isCurrentUser && (
                                <span className="rounded-full bg-muted px-2 py-0.5 font-mono text-[10px] text-muted-foreground">
                                  You
                                </span>
                              )}
                              {!isLead && (
                                <span
                                  className={cn(
                                    "inline-flex items-center rounded-full border px-2 py-0.5 text-[10px] font-mono",
                                    roleBadge.bg,
                                    roleBadge.text,
                                    roleBadge.border
                                  )}
                                >
                                  {roleBadge.label}
                                </span>
                              )}
                            </div>

                            {email && (
                              <p className="font-mono text-xs text-muted-foreground truncate mt-0.5">
                                {email}
                              </p>
                            )}

                            <div className="mt-1 flex flex-wrap items-center gap-3 text-[11px] text-muted-foreground font-mono">
                              {department && (
                                <span className="flex items-center gap-1">
                                  <GraduationCap className="h-3 w-3 shrink-0" />
                                  <span className="truncate">{department}</span>
                                </span>
                              )}
                              {joinedLabel && (
                                <span className="flex items-center gap-1">
                                  <Calendar className="h-3 w-3 shrink-0" />
                                  <span>Joined {joinedLabel}</span>
                                </span>
                              )}
                            </div>

                            {/* Sole Lead Note */}
                            {isSole && isLead && (
                              <p className="mt-1.5 flex items-center gap-1 font-mono text-[11px] text-pastel-yellowText">
                                <ShieldAlert className="h-3 w-3 shrink-0" />
                                <span>A project must retain at least one Lead.</span>
                              </p>
                            )}
                          </div>
                        </div>

                        {/* Actions & Role Selector */}
                        {isLead && (
                          <div className="flex flex-wrap items-center gap-2 pt-2 sm:pt-0 border-t border-border/50 sm:border-t-0">
                            {/* Role Selector Dropdown */}
                            <div className="relative">
                              <select
                                aria-label={`Role for ${displayName}`}
                                value={member.role}
                                disabled={isSole || isUpdating || isRemoving}
                                onChange={(e) =>
                                  void handleRoleChange(member, e.target.value as ProjectRole)
                                }
                                className={cn(
                                  "rounded-md border border-border bg-surface px-2.5 py-1.5 font-mono text-xs text-foreground transition-colors focus:border-foreground focus:outline-none focus:ring-1 focus:ring-foreground",
                                  isSole && "opacity-60 cursor-not-allowed bg-muted/40",
                                  isUpdating && "opacity-50"
                                )}
                              >
                                <option value="lead">Lead</option>
                                <option value="maintainer">Maintainer</option>
                                <option value="member">Member</option>
                                <option value="viewer">Viewer</option>
                              </select>
                            </div>

                            {/* Transfer Leadership Action */}
                            {member.role !== "lead" && (
                              <Button
                                type="button"
                                variant="outline"
                                size="sm"
                                disabled={isUpdating || isRemoving}
                                onClick={() => setTransferTarget(member)}
                                className="h-8 inline-flex items-center gap-1 px-2.5 text-xs font-mono text-muted-foreground hover:text-foreground"
                                title="Transfer Lead ownership to this member"
                              >
                                <Crown className="h-3.5 w-3.5 text-purple-600" />
                                <span className="hidden md:inline">Transfer Lead</span>
                              </Button>
                            )}

                            {/* Remove Member Action */}
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              disabled={isSole || isUpdating || isRemoving}
                              onClick={() => setRemoveTarget(member)}
                              className={cn(
                                "h-8 px-2 text-xs text-muted-foreground hover:text-pastel-redText hover:bg-pastel-red/20",
                                isSole &&
                                  "opacity-40 cursor-not-allowed hover:bg-transparent hover:text-muted-foreground"
                              )}
                              title={isSole ? "Cannot remove sole lead" : "Remove member"}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                              <span className="sr-only">Remove member</span>
                            </Button>
                          </div>
                        )}
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          )}

          {activeTab === "invite" && (
            <div className="max-w-xl mx-auto space-y-6 py-2">
              <div>
                <h3 className="font-serif text-lg font-normal text-foreground">
                  Direct Institutional Invitation
                </h3>
                <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
                  Invite university students and faculty directly to this workspace by their
                  institutional .edu email address.
                </p>
              </div>

              <form onSubmit={handleSendInvite} className="space-y-4">
                {/* Email Input */}
                <div>
                  <label
                    htmlFor="invite-email"
                    className="block text-xs font-medium text-foreground mb-1.5"
                  >
                    Institutional Email Address <span className="text-pastel-redText">*</span>
                  </label>
                  <Input
                    id="invite-email"
                    type="email"
                    value={inviteEmail}
                    onChange={(e) => {
                      setInviteEmail(e.target.value);
                      if (inviteEmailError) setInviteEmailError(null);
                    }}
                    placeholder="student@university.edu"
                    className={cn(
                      "font-mono text-xs",
                      inviteEmailError && "border-pastel-redText focus-visible:ring-pastel-redText"
                    )}
                    required
                  />
                  {inviteEmailError ? (
                    <p className="mt-1 text-xs font-mono text-pastel-redText">{inviteEmailError}</p>
                  ) : (
                    <p className="mt-1 text-[11px] font-mono text-muted-foreground">
                      Must be a valid .edu institutional address.
                    </p>
                  )}
                </div>

                {/* Role Selector */}
                <div>
                  <label
                    htmlFor="invite-role"
                    className="block text-xs font-medium text-foreground mb-1.5"
                  >
                    Assigned Workspace Role <span className="text-pastel-redText">*</span>
                  </label>
                  <select
                    id="invite-role"
                    value={inviteRole}
                    onChange={(e) => setInviteRole(e.target.value as ProjectRole)}
                    className="w-full rounded-md border border-border bg-surface px-3 py-2 font-mono text-xs text-foreground focus:border-foreground focus:outline-none focus:ring-1 focus:ring-foreground"
                  >
                    <option value="maintainer">
                      Maintainer (Manage sections, invite members, review applications)
                    </option>
                    <option value="member">
                      Member (Active collaborator on tasks and deliverables)
                    </option>
                    <option value="viewer">Viewer (Read-only access to workspace resources)</option>
                  </select>
                </div>

                {/* Role Explanation Card */}
                <div className="rounded-lg border border-border/80 bg-muted/20 p-3 text-xs text-muted-foreground space-y-1.5 font-mono">
                  <div className="font-semibold text-foreground">Role Permissions:</div>
                  <ul className="list-disc list-inside space-y-1 text-[11px]">
                    <li>
                      <span className="font-medium text-foreground">Maintainer:</span> Co-manages
                      workspace content, recruitment, and invites.
                    </li>
                    <li>
                      <span className="font-medium text-foreground">Member:</span> Contributes to
                      project milestones and team discussions.
                    </li>
                    <li>
                      <span className="font-medium text-foreground">Viewer:</span> Observer access
                      for audits and reviews.
                    </li>
                  </ul>
                </div>

                {/* Submit Action */}
                <div className="pt-2 flex justify-end">
                  <Button
                    type="submit"
                    disabled={isSendingInvite}
                    className="inline-flex items-center gap-2 rounded-md bg-[#111111] px-5 py-2 text-xs font-medium text-white hover:bg-[#222222] transition-colors"
                  >
                    {isSendingInvite ? (
                      <>
                        <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        <span>Sending Invitation...</span>
                      </>
                    ) : (
                      <>
                        <Send className="h-3.5 w-3.5" />
                        <span>Send Invitation</span>
                      </>
                    )}
                  </Button>
                </div>
              </form>
            </div>
          )}
        </div>

        {/* Transfer Leadership Confirmation Dialog */}
        {transferTarget && (
          <div className="border-t border-border bg-muted/40 p-4">
            <div className="rounded-lg border border-purple-500/30 bg-purple-500/5 p-4 space-y-3">
              <div className="flex items-start gap-3">
                <Crown className="h-5 w-5 text-purple-600 shrink-0 mt-0.5" />
                <div>
                  <h4 className="font-serif text-sm font-medium text-foreground">
                    Transfer Leadership of Workspace
                  </h4>
                  <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
                    Are you sure you want to transfer Lead ownership of{" "}
                    <span className="font-semibold text-foreground">{project.title}</span> to{" "}
                    <span className="font-semibold text-foreground">
                      {getMemberDisplayName(transferTarget)}
                    </span>
                    ? They will become the primary Lead, and you will continue as a collaborator.
                  </p>
                </div>
              </div>
              <div className="flex justify-end gap-2 pt-1">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={isTransferring}
                  onClick={() => setTransferTarget(null)}
                >
                  Cancel
                </Button>
                <Button
                  type="button"
                  size="sm"
                  disabled={isTransferring}
                  onClick={() => void handleConfirmTransfer()}
                  className="rounded-md bg-[#111111] px-4 text-xs font-medium text-white hover:bg-[#222222]"
                >
                  {isTransferring ? (
                    <span className="flex items-center gap-1.5">
                      <Loader2 className="h-3 w-3 animate-spin" />
                      <span>Transferring...</span>
                    </span>
                  ) : (
                    <span>Confirm Transfer</span>
                  )}
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* Remove Member Confirmation Dialog */}
        {removeTarget && (
          <div className="border-t border-border bg-muted/40 p-4">
            <div className="rounded-lg border border-pastel-redText/30 bg-pastel-red/10 p-4 space-y-3">
              <div className="flex items-start gap-3">
                <AlertCircle className="h-5 w-5 text-pastel-redText shrink-0 mt-0.5" />
                <div>
                  <h4 className="font-serif text-sm font-medium text-foreground">
                    Remove Collaborator from Workspace
                  </h4>
                  <p className="mt-1 text-xs text-muted-foreground leading-relaxed">
                    Are you sure you want to remove{" "}
                    <span className="font-semibold text-foreground">
                      {getMemberDisplayName(removeTarget)}
                    </span>{" "}
                    from <span className="font-semibold text-foreground">{project.title}</span>?
                    They will immediately lose access to team resources.
                  </p>
                </div>
              </div>
              <div className="flex justify-end gap-2 pt-1">
                <Button
                  type="button"
                  variant="outline"
                  size="sm"
                  disabled={removingMemberId !== null}
                  onClick={() => setRemoveTarget(null)}
                >
                  Cancel
                </Button>
                <Button
                  type="button"
                  size="sm"
                  disabled={removingMemberId !== null}
                  onClick={() => void handleConfirmRemove()}
                  className="rounded-md bg-pastel-redText px-4 text-xs font-medium text-white hover:bg-pastel-redText/90"
                >
                  {removingMemberId !== null ? (
                    <span className="flex items-center gap-1.5">
                      <Loader2 className="h-3 w-3 animate-spin" />
                      <span>Removing...</span>
                    </span>
                  ) : (
                    <span>Confirm Remove</span>
                  )}
                </Button>
              </div>
            </div>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
