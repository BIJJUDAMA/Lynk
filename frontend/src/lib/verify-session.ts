export async function refreshSessionAfterEmailVerification(session: {
  attemptRefreshingSession?: () => Promise<boolean>;
}): Promise<void> {
  if (typeof session.attemptRefreshingSession === "function") {
    await session.attemptRefreshingSession();
  }
}
