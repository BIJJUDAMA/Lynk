"use client";

import { Suspense, useEffect, useRef, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { CheckCircle2, ShieldAlert, ArrowRight, Loader2, ArrowLeft, LogOut } from "lucide-react";
import { initSuperTokens, EmailVerification, Session } from "@/lib/supertokens";
import { useAuth } from "@/components/auth/AuthProvider";
import { refreshSessionAfterEmailVerification } from "@/lib/verify-session";
import { Button } from "@/components/ui/button";
import { Badge } from "@/components/ui/badge";

function VerifyEmailContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const token = searchParams.get("token");

  const { isAuthenticated, user, isVerified, resendVerificationEmail, setSession, logout } =
    useAuth();

  const [status, setStatus] = useState<"instructions" | "verifying" | "success" | "error">(
    token ? "verifying" : "instructions"
  );
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Resend countdown state
  const [isResending, setIsResending] = useState(false);
  const [resendSuccess, setResendSuccess] = useState(false);
  const [resendCountdown, setResendCountdown] = useState(0);

  const processedRef = useRef(false);

  // Countdown timer effect
  useEffect(() => {
    if (resendCountdown <= 0) return;
    const timer = setInterval(() => {
      setResendCountdown((prev) => (prev > 0 ? prev - 1 : 0));
    }, 1000);
    return () => clearInterval(timer);
  }, [resendCountdown]);

  // Token-based verification
  useEffect(() => {
    if (!token) return;
    if (processedRef.current) return;
    processedRef.current = true;

    const performVerification = async () => {
      initSuperTokens();
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
            "This email verification link is invalid or has expired. Verification links are single-use."
          );
        } else {
          setStatus("error");
          setErrorMessage(
            "An error occurred while verifying your email. Please request a new verification link."
          );
        }
      } catch (err: unknown) {
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

  // Check verification status button handler
  const handleCheckStatus = async () => {
    try {
      await setSession();
      if (isVerified) {
        router.push("/jobs");
      } else {
        setErrorMessage("Email not yet verified. Please click the link in your email first.");
      }
    } catch {
      window.location.reload();
    }
  };

  const handleResend = async () => {
    if (resendCountdown > 0 || isResending) return;

    if (!isAuthenticated) {
      router.push("/login");
      return;
    }

    setIsResending(true);
    setErrorMessage(null);
    setResendSuccess(false);

    try {
      await resendVerificationEmail();
      setResendSuccess(true);
      setResendCountdown(60);
    } catch (err: unknown) {
      setErrorMessage(
        err instanceof Error
          ? err.message
          : "Failed to resend verification link. Please try again shortly."
      );
    } finally {
      setIsResending(false);
    }
  };

  return (
    <div className="min-h-screen flex items-center justify-center bg-background px-4 py-12">
      <div className="w-full max-w-md space-y-6">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3 w-3" />
          <span>Back to Lynk</span>
        </Link>

        <div className="rounded-xl border border-border bg-card p-8 text-center shadow-none space-y-6">
          {/* Header */}
          <div className="space-y-1">
            <span className="font-mono text-xs uppercase tracking-widest text-muted-foreground">
              LYNK VERIFICATION
            </span>
            <h1 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground">
              {status === "success" ? "University Email Confirmed" : "Verify your university email"}
            </h1>
          </div>

          {/* State 1: Verifying token */}
          {status === "verifying" && (
            <div className="py-6 space-y-4">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-muted text-foreground">
                <Loader2 className="h-6 w-6 animate-spin" />
              </div>
              <p className="font-mono text-xs text-muted-foreground">
                Validating institutional email credentials...
              </p>
            </div>
          )}

          {/* State 2: Success */}
          {status === "success" && (
            <div className="space-y-5">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-green text-pastel-greenText">
                <CheckCircle2 className="h-6 w-6" />
              </div>

              <div>
                <Badge variant="default">VERIFIED .EDU</Badge>
                <p className="mt-2 text-xs text-muted-foreground">
                  Your university email is confirmed. You now have full access to publish gigs,
                  apply, and sign contracts.
                </p>
              </div>

              <div className="pt-2">
                <Button asChild className="w-full text-xs">
                  <Link href="/jobs">
                    <span>Continue to Opportunities</span>
                    <ArrowRight className="h-3.5 w-3.5 ml-1.5" />
                  </Link>
                </Button>
              </div>
            </div>
          )}

          {/* State 3: Instructions (Default when waiting for user to click link) */}
          {status === "instructions" && (
            <div className="space-y-6 text-left">
              {user?.email && (
                <div className="rounded-md border border-border bg-muted/40 p-3 text-center">
                  <p className="text-[11px] text-muted-foreground">Verification sent to:</p>
                  <p className="font-mono text-xs font-semibold text-foreground mt-0.5">
                    {user.email}
                  </p>
                </div>
              )}

              {/* Step list */}
              <div className="space-y-3 border-l-2 border-border pl-4">
                <div className="space-y-0.5">
                  <span className="font-mono text-[10px] text-muted-foreground uppercase tracking-wider">
                    Step 1
                  </span>
                  <p className="text-xs text-foreground font-medium">
                    Check your institutional inbox for a verification link.
                  </p>
                </div>
                <div className="space-y-0.5">
                  <span className="font-mono text-[10px] text-muted-foreground uppercase tracking-wider">
                    Step 2
                  </span>
                  <p className="text-xs text-foreground font-medium">
                    Click the link in the email to confirm your university address.
                  </p>
                </div>
                <div className="space-y-0.5">
                  <span className="font-mono text-[10px] text-muted-foreground uppercase tracking-wider">
                    Step 3
                  </span>
                  <p className="text-xs text-foreground font-medium">
                    Return here and click Continue below.
                  </p>
                </div>
              </div>

              {resendSuccess && (
                <div className="flex items-center gap-2 rounded-md border border-pastel-greenText/20 bg-pastel-green p-3 text-xs text-pastel-greenText font-mono">
                  <CheckCircle2 className="h-4 w-4 shrink-0" />
                  <span>Fresh link sent to your university inbox.</span>
                </div>
              )}

              {errorMessage && (
                <div className="flex items-start gap-2 rounded-md border border-pastel-redText/20 bg-pastel-red p-3 text-xs text-pastel-redText">
                  <ShieldAlert className="h-4 w-4 shrink-0 mt-0.5" />
                  <span>{errorMessage}</span>
                </div>
              )}

              <div className="space-y-3 pt-2">
                <button
                  type="button"
                  onClick={handleCheckStatus}
                  className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white shadow-none hover:bg-[#222222] transition-colors active:scale-[0.98]"
                >
                  <span>Continue</span>
                  <ArrowRight className="h-3.5 w-3.5" />
                </button>

                <div className="text-center">
                  <button
                    type="button"
                    onClick={handleResend}
                    disabled={isResending || resendCountdown > 0}
                    className="font-mono text-xs text-muted-foreground underline-offset-4 hover:text-foreground hover:underline transition-colors disabled:opacity-50 disabled:no-underline"
                  >
                    {isResending
                      ? "Dispatching link..."
                      : resendCountdown > 0
                        ? `Resend in ${resendCountdown}s`
                        : "Resend verification email"}
                  </button>
                </div>
              </div>
            </div>
          )}

          {/* State 4: Error */}
          {status === "error" && (
            <div className="space-y-5 text-center">
              <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-red text-pastel-redText">
                <ShieldAlert className="h-6 w-6" />
              </div>

              <div className="space-y-1">
                <h2 className="font-serif text-xl font-normal text-foreground">
                  Verification Link Invalid or Expired
                </h2>
                <p className="text-xs text-muted-foreground leading-relaxed">
                  {errorMessage ||
                    "This verification link has expired or was already used. Please request a new link."}
                </p>
              </div>

              {resendSuccess && (
                <div className="flex items-center gap-2 rounded-md border border-pastel-greenText/20 bg-pastel-green p-3 text-xs text-pastel-greenText font-mono">
                  <CheckCircle2 className="h-4 w-4 shrink-0" />
                  <span>New verification email sent.</span>
                </div>
              )}

              <div className="space-y-2.5 pt-2">
                <button
                  type="button"
                  onClick={handleResend}
                  disabled={isResending || resendCountdown > 0}
                  className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white shadow-none hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:opacity-50"
                >
                  {isResending ? (
                    <>
                      <Loader2 className="h-3.5 w-3.5 animate-spin" />
                      <span>Sending...</span>
                    </>
                  ) : resendCountdown > 0 ? (
                    <span>Resend in {resendCountdown}s</span>
                  ) : (
                    <span>Request New Verification Link</span>
                  )}
                </button>

                <Button asChild variant="outline" className="w-full text-xs">
                  <Link href="/login">Return to Sign In</Link>
                </Button>
              </div>
            </div>
          )}

          {/* Sign Out link */}
          {isAuthenticated && (
            <div className="pt-2 border-t border-border">
              <button
                type="button"
                onClick={() => logout()}
                className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground hover:text-foreground transition-colors"
              >
                <LogOut className="h-3 w-3" />
                <span>Sign out</span>
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

export default function VerifyEmailPage() {
  return (
    <Suspense
      fallback={
        <div className="min-h-screen flex items-center justify-center bg-background">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <VerifyEmailContent />
    </Suspense>
  );
}
