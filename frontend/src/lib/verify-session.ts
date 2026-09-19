export async function refreshSessionAfterEmailVerification(session: {
  attemptRefreshingSession?: () => Promise<boolean>;
}): Promise<void> {
  if (typeof session.attemptRefreshingSession === "function") {
    await session.attemptRefreshingSession();
  }
}

export async function syncStaleEmailVerification(
  claimed: boolean | undefined,
  checkVerification: () => Promise<{ isVerified?: boolean } | null | undefined>,
  attemptRefresh: () => Promise<boolean | void>
): Promise<boolean> {
  let emailVerified = claimed ?? false;
  if (!claimed) {
    try {
      const isVerifiedRes = await checkVerification();
      if (isVerifiedRes?.isVerified) {
        emailVerified = true;
        await attemptRefresh().catch(() => false);
      }
    } catch {
      // ignore
    }
  }
  return emailVerified;
}
