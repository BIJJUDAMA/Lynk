export function shouldPostAuthSyncOnSessionRestore(): boolean {
  return false;
}

export function emailVerifiedFromAccessPayload(
  payload: Record<string, unknown>
): boolean | undefined {
  if (typeof payload.emailVerified === "boolean") return payload.emailVerified;
  if (typeof payload.email_verified === "boolean") return payload.email_verified;
  return undefined;
}
