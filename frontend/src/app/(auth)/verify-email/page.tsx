"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { CheckCircle2, ShieldAlert, ArrowRight, Loader2, Mail } from "lucide-react";
import { initSuperTokens, EmailVerification, Session } from "@/lib/supertokens";
import { useAuth } from "@/components/auth/AuthProvider";
import { refreshSessionAfterEmailVerification } from "@/lib/verify-session";

function VerifyEmailContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token");

  const { isAuthenticated, resendVerificationEmail, setSession } = useAuth();

  const [status, setStatus] = useState<"verifying" | "success" | "error">("verifying");
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Resend state for unverified users
  const [isResending, setIsResending] = useState(false);
  const [resendSuccess, setResendSuccess] = useState(false);
  const [resendError, setResendError] = useState<string | null>(null);

  const processedRef = useRef(false);

  useEffect(() => {
    if (processedRef.current) return;
    processedRef.current = true;

    const performVerification = async () => {
      initSuperTokens();

      if (!token) {
        setStatus("error");
        setErrorMessage(
          "No verification token was provided in the URL. Please verify that you followed the complete link sent to your university email."
        );
        return;
      }

      try {
        let res: { status: string };
        if (
          typeof (
            EmailVerification as unknown as {
              verifyEmailWithToken?: (args: { token: string }) => Promise<{ status: string }>;
            }
          ).verifyEmailWithToken === "function"
        ) {
          res = await (
            EmailVerification as unknown as {
              verifyEmailWithToken: (args: { token: string }) => Promise<{ status: string }>;
            }
          ).verifyEmailWithToken({ token });
        } else {
          res = await EmailVerification.verifyEmail();
        }

        if (res.status === "OK") {
          try {
            await refreshSessionAfterEmailVerification(Session);
            await setSession();
          } catch (refreshErr) {
            console.warn("Session refresh after email verification encountered error:", refreshErr);
          }
          setStatus("success");
        } else if (res.status === "EMAIL_VERIFICATION_INVALID_TOKEN_ERROR") {
          setStatus("error");
          setErrorMessage(
            "This email verification link is invalid or has expired. Verification links are single-use and time-limited."
          );
        } else {
          setStatus("error");
          setErrorMessage(
            "An error occurred while verifying your email. Please request a new verification link."
          );
        }
      } catch (err: unknown) {
        console.error("Email verification exception:", err);
        setStatus("error");
        setErrorMessage(
          err instanceof Error
            ? err.message
            : "An unexpected error occurred during email verification."
        );
      }
    };

    performVerification();
  }, [token, setSession]);

  const handleResend = async () => {
    if (!isAuthenticated) {
      router.push("/login");
      return;
    }

    setIsResending(true);
    setResendError(null);
    setResendSuccess(false);
    try {
      await resendVerificationEmail();
      setResendSuccess(true);
    } catch (err: unknown) {
      console.error("Resend error:", err);
      setResendError(
        err instanceof Error
          ? err.message
          : "Failed to resend verification link. Please sign in and try again."
      );
    } finally {
      setIsResending(false);
    }
  };

  return (
    <div className="flex min-h-[85vh] items-center justify-center px-4 py-12">
      <div className="w-full max-w-md rounded-xl border border-border bg-card p-8 text-center shadow-sm">
        {/* State 1: Verifying */}
        {status === "verifying" && (
          <div className="space-y-4">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl border border-primary/20 bg-primary/10 text-primary">
              <Loader2 className="h-7 w-7 animate-spin" />
            </div>
            <h2 className="text-2xl font-bold tracking-tight text-foreground">Verifying Email</h2>
            <p className="text-sm text-muted-foreground">
              Verifying your campus institutional email...
            </p>
          </div>
        )}

        {/* State 2: Success */}
        {status === "success" && (
          <div className="space-y-4">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
              <CheckCircle2 className="h-7 w-7" />
            </div>

            <div className="flex justify-center">
              <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-3 py-1 text-xs font-medium text-emerald-700 dark:text-emerald-300">
                <CheckCircle2 className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                Campus Verified ✓
              </span>
            </div>

            <h2 className="text-2xl font-bold tracking-tight text-foreground">Campus Verified ✓</h2>

            <p className="text-sm text-muted-foreground">
              Your university email has been verified. Full campus access unlocked!
            </p>

            <div className="pt-4 space-y-2.5">
              <Link
                href="/jobs"
                className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90"
              >
                Browse Opportunities
                <ArrowRight className="h-4 w-4" />
              </Link>
              <Link
                href="/profile"
                className="flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-background px-4 py-2.5 text-sm font-medium text-foreground transition hover:bg-accent hover:text-accent-foreground"
              >
                Campus Profile
              </Link>
            </div>
          </div>
        )}

        {/* State 3: Error / Expired */}
        {status === "error" && (
          <div className="space-y-5">
            <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl border border-rose-500/20 bg-rose-500/10 text-rose-600 dark:text-rose-400">
              <ShieldAlert className="h-7 w-7" />
            </div>

            <div>
              <h2 className="text-2xl font-bold tracking-tight text-foreground">
                Verification Link Expired or Invalid
              </h2>
              <p className="mt-2 text-sm text-muted-foreground">
                {errorMessage ||
                  "This email verification link is invalid or has expired. Please request a new verification link to activate your campus account."}
              </p>
            </div>

            {resendSuccess && (
              <div className="flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-left text-xs text-emerald-700 dark:text-emerald-300">
                <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
                <span>A new verification link has been sent to your inbox!</span>
              </div>
            )}

            {resendError && (
              <div className="flex items-center gap-2 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3 text-left text-xs text-rose-700 dark:text-rose-300">
                <ShieldAlert className="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400" />
                <span>{resendError}</span>
              </div>
            )}

            <div className="pt-2 space-y-2.5">
              <button
                type="button"
                onClick={handleResend}
                disabled={isResending}
                className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90 disabled:opacity-50"
              >
                {isResending ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>Sending link...</span>
                  </>
                ) : (
                  <>
                    <Mail className="h-4 w-4" />
                    <span>Request New Verification Link</span>
                  </>
                )}
              </button>

              <Link
                href="/login"
                className="flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-background px-4 py-2.5 text-sm font-medium text-foreground transition hover:bg-accent hover:text-accent-foreground"
              >
                Return to Sign In
              </Link>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[85vh] items-center justify-center">
          <div className="flex flex-col items-center gap-3 text-muted-foreground">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
            <p className="text-xs font-medium">Loading email verification...</p>
          </div>
        </div>
      }
    >
      <VerifyEmailContent />
    </Suspense>
  );
}
