/**
 * Validates that an email address belongs strictly to an institutional .edu domain.
 */
export function isEduEmail(email: string): boolean {
  if (!email) return false;
  const trimmed = email.trim().toLowerCase();
  const emailRegex = /^[a-zA-Z0-9._%+-]+@([a-zA-Z0-9-]+\.)+edu$/;
  return emailRegex.test(trimmed);
}
