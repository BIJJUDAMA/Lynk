"use client";

import { Suspense, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import {
  ArrowLeft,
  ArrowRight,
  CheckCircle2,
  ShieldCheck,
  ShieldAlert,
  GraduationCap,
  Mail,
  Lock,
  AlertCircle,
  Loader2,
  RefreshCw,
  LogOut,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { isEduEmail } from "@/lib/email-validation";

function LoginContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const redirectPath = searchParams.get("redirect") || "/jobs";

  const {
    login,
    register,
    isAuthenticated,
    user,
    isVerified,
    logout,
    resendVerificationEmail,
  } = useAuth();

  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [errorMessage, setErrorMessage] = useState<string | null>(null);

  // Resend verification email state
  const [isResending, setIsResending] = useState(false);
  const [resendSuccess, setResendSuccess] = useState(false);
  const [resendError, setResendError] = useState<string | null>(null);

  const isRegistering = authMode === "register";
  const trimmedEmail = email.trim();
  const hasEmail = trimmedEmail.length > 0;
  const isEdu = isEduEmail(trimmedEmail);
  const showEduWarning = isRegistering && hasEmail && !isEdu;

  const isFormValid =
    trimmedEmail.length > 0 &&
    password.length > 0 &&
    (!isRegistering || isEdu);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);

    if (isRegistering && !isEdu) {
      setErrorMessage(
        "Only institutional .edu email addresses are eligible for campus membership."
      );
      return;
    }

    if (!password) {
      setErrorMessage("Please enter your password.");
      return;
    }

    setIsSubmitting(true);
    try {
      if (authMode === "login") {
        await login(trimmedEmail, password);
        if (redirectPath) {
          router.push(redirectPath);
        }
      } else {
        await register(trimmedEmail, password);
      }
    } catch (err: unknown) {
      console.error("Authentication error:", err);
      setErrorMessage(
        err instanceof Error
          ? err.message
          : "Authentication failed. Please check your credentials and try again."
      );
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleResendVerification = async () => {
    setIsResending(true);
    setResendError(null);
    setResendSuccess(false);
    try {
      await resendVerificationEmail();
      setResendSuccess(true);
    } catch (err: unknown) {
      console.error("Resend verification error:", err);
      setResendError(
        err instanceof Error
          ? err.message
          : "Failed to send verification email. Please try again in a few moments."
      );
    } finally {
      setIsResending(false);
    }
  };

  // -------------------------------------------------------------
  // Authenticated State 1: Verification Pending
  // -------------------------------------------------------------
  if (isAuthenticated && !isVerified) {
    return (
      <div className="flex min-h-[80vh] items-center justify-center px-4 py-12">
        <div className="w-full max-w-md rounded-xl border border-border bg-card p-8 text-center shadow-sm">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl border border-amber-500/20 bg-amber-500/10 text-amber-600 dark:text-amber-400">
            <ShieldAlert className="h-7 w-7" />
          </div>

          <div className="mt-4 flex justify-center">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-amber-500/30 bg-amber-500/10 px-3 py-1 text-xs font-medium text-amber-700 dark:text-amber-300">
              <ShieldAlert className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
              Campus verification pending
            </span>
          </div>

          <h2 className="mt-4 text-2xl font-bold tracking-tight text-foreground">
            Campus Verification Pending
          </h2>

          <p className="mt-2 text-sm text-muted-foreground">
            A verification link has been sent to your university inbox. Please verify your email to unlock campus opportunities.
          </p>

          {user?.email && (
            <div className="mt-4 inline-block rounded-lg border border-border bg-muted/40 px-3.5 py-1.5 text-xs font-medium text-foreground">
              {user.email}
            </div>
          )}

          {resendSuccess && (
            <div className="mt-4 flex items-center gap-2 rounded-lg border border-emerald-500/30 bg-emerald-500/10 p-3 text-left text-xs text-emerald-700 dark:text-emerald-300">
              <CheckCircle2 className="h-4 w-4 shrink-0 text-emerald-600 dark:text-emerald-400" />
              <span>A fresh verification link has been sent to your university email!</span>
            </div>
          )}

          {resendError && (
            <div className="mt-4 flex items-center gap-2 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3 text-left text-xs text-rose-700 dark:text-rose-300">
              <AlertCircle className="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400" />
              <span>{resendError}</span>
            </div>
          )}

          <div className="mt-6 space-y-3">
            <button
              type="button"
              onClick={handleResendVerification}
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
                  <span>Resend Verification Email</span>
                </>
              )}
            </button>

            <button
              type="button"
              onClick={() => window.location.reload()}
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-background px-4 py-2.5 text-sm font-medium text-foreground transition hover:bg-accent hover:text-accent-foreground"
            >
              <RefreshCw className="h-4 w-4" />
              <span>Refresh Status</span>
            </button>

            <button
              type="button"
              onClick={() => logout()}
              className="flex w-full items-center justify-center gap-1.5 rounded-lg px-4 py-2 text-xs font-medium text-muted-foreground transition hover:text-foreground"
            >
              <LogOut className="h-3.5 w-3.5" />
              <span>Sign Out</span>
            </button>
          </div>
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------
  // Authenticated State 2: Verified User
  // -------------------------------------------------------------
  if (isAuthenticated && user && isVerified) {
    return (
      <div className="flex min-h-[80vh] items-center justify-center px-4 py-12">
        <div className="w-full max-w-md rounded-xl border border-border bg-card p-8 text-center shadow-sm">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl border border-emerald-500/20 bg-emerald-500/10 text-emerald-600 dark:text-emerald-400">
            <CheckCircle2 className="h-7 w-7" />
          </div>

          <h2 className="mt-4 text-2xl font-bold tracking-tight text-foreground">
            Signed In
          </h2>
          <p className="mt-1 text-sm text-muted-foreground">
            {user.name ? (
              <span className="font-medium text-foreground">{user.name}</span>
            ) : null}
            {user.name ? " • " : ""}
            <span>{user.email}</span>
          </p>

          <div className="mt-3 flex justify-center">
            <span className="inline-flex items-center gap-1.5 rounded-full border border-emerald-500/30 bg-emerald-500/10 px-3 py-1 text-xs font-medium text-emerald-700 dark:text-emerald-300">
              <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
              Campus Verified ✓
            </span>
          </div>

          <div className="mt-6 space-y-2.5">
            <Link
              href="/jobs"
              className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90"
            >
              Browse Opportunities
              <ArrowRight className="h-4 w-4" />
            </Link>
            <Link
              href="/activity"
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-background px-4 py-2.5 text-sm font-medium text-foreground transition hover:bg-accent hover:text-accent-foreground"
            >
              View Activity & Contracts
            </Link>
            <Link
              href="/profile"
              className="flex w-full items-center justify-center gap-2 rounded-lg border border-border bg-background px-4 py-2.5 text-sm font-medium text-foreground transition hover:bg-accent hover:text-accent-foreground"
            >
              Campus Profile
            </Link>
            <button
              onClick={() => logout()}
              className="w-full rounded-lg px-4 py-2 text-xs font-medium text-muted-foreground transition hover:text-foreground"
            >
              Sign Out
            </button>
          </div>
        </div>
      </div>
    );
  }

  // -------------------------------------------------------------
  // Unauthenticated State: Sign In / Create Account Form
  // -------------------------------------------------------------
  return (
    <div className="flex min-h-[80vh] flex-col justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs text-muted-foreground transition hover:text-foreground"
        >
          <ArrowLeft className="h-3.5 w-3.5" />
          Back to Lynk
        </Link>

        <div className="mt-6 text-center">
          <div className="mx-auto flex h-11 w-11 items-center justify-center rounded-xl bg-primary text-primary-foreground shadow-sm">
            <GraduationCap className="h-6 w-6" />
          </div>

          <h1 className="mt-4 text-2xl font-bold tracking-tight text-foreground sm:text-3xl">
            {authMode === "login" ? "Sign In to Lynk" : "Create Your Campus Account"}
          </h1>
          <p className="mt-2 text-sm text-muted-foreground">
            High-trust freelance and collaboration platform for university campus members.
          </p>
        </div>

        {/* Mode Switcher */}
        <div className="mt-6 grid grid-cols-2 rounded-lg border border-border bg-muted/50 p-1">
          <button
            type="button"
            onClick={() => {
              setAuthMode("login");
              setErrorMessage(null);
            }}
            className={`rounded-md py-2 text-xs font-medium transition ${
              authMode === "login"
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            Sign In
          </button>
          <button
            type="button"
            onClick={() => {
              setAuthMode("register");
              setErrorMessage(null);
            }}
            className={`rounded-md py-2 text-xs font-medium transition ${
              authMode === "register"
                ? "bg-background text-foreground shadow-sm"
                : "text-muted-foreground hover:text-foreground"
            }`}
          >
            Create Account
          </button>
        </div>

        {/* Action Card */}
        <div className="mt-6 rounded-xl border border-border bg-card p-6 shadow-sm">
          {errorMessage && (
            <div className="mb-4 flex items-start gap-2.5 rounded-lg border border-rose-500/30 bg-rose-500/10 p-3 text-xs text-rose-700 dark:text-rose-300">
              <AlertCircle className="h-4 w-4 shrink-0 text-rose-600 dark:text-rose-400 mt-0.5" />
              <div className="flex-1">{errorMessage}</div>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            {/* Email Field */}
            <div className="space-y-1.5 text-left">
              <label
                htmlFor="email"
                className="block text-xs font-medium text-foreground"
              >
                University Email Address
              </label>
              <div className="relative">
                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">
                  <Mail className="h-4 w-4" />
                </div>
                <input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  value={email}
                  onChange={(e) => {
                    setEmail(e.target.value);
                    if (errorMessage) setErrorMessage(null);
                  }}
                  placeholder={
                    isRegistering
                      ? "student@university.edu"
                      : "your.name@university.edu"
                  }
                  className="block w-full rounded-lg border border-border bg-background py-2.5 pl-10 pr-3 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </div>

              {/* Real-time .edu warning in registration mode */}
              {showEduWarning && (
                <div className="mt-1.5 flex items-start gap-2 rounded-md border border-amber-500/30 bg-amber-500/10 p-2.5 text-xs text-amber-700 dark:text-amber-300">
                  <AlertCircle className="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400 mt-0.5" />
                  <span>
                    Only institutional <strong>.edu</strong> email addresses are eligible for campus membership.
                  </span>
                </div>
              )}
            </div>

            {/* Password Field */}
            <div className="space-y-1.5 text-left">
              <label
                htmlFor="password"
                className="block text-xs font-medium text-foreground"
              >
                Password
              </label>
              <div className="relative">
                <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-muted-foreground">
                  <Lock className="h-4 w-4" />
                </div>
                <input
                  id="password"
                  name="password"
                  type="password"
                  autoComplete={
                    isRegistering ? "new-password" : "current-password"
                  }
                  required
                  minLength={8}
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value);
                    if (errorMessage) setErrorMessage(null);
                  }}
                  placeholder="••••••••"
                  className="block w-full rounded-lg border border-border bg-background py-2.5 pl-10 pr-3 text-sm text-foreground placeholder:text-muted-foreground focus:border-primary focus:outline-none focus:ring-1 focus:ring-primary"
                />
              </div>
              {isRegistering && (
                <p className="text-[11px] text-muted-foreground">
                  Must be at least 8 characters long.
                </p>
              )}
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              disabled={isSubmitting || !isFormValid}
              className="flex w-full items-center justify-center gap-2 rounded-lg bg-primary px-4 py-3 text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90 disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin" />
                  <span>
                    {authMode === "login" ? "Signing In..." : "Creating Account..."}
                  </span>
                </>
              ) : (
                <>
                  <span>
                    {authMode === "login"
                      ? "Sign In"
                      : "Create Campus Account"}
                  </span>
                  <ArrowRight className="h-4 w-4" />
                </>
              )}
            </button>
          </form>

          {/* Value Prop Banner */}
          <div className="mt-6 rounded-lg border border-border/60 bg-muted/30 p-4 text-left">
            <div className="flex items-start gap-3">
              <ShieldCheck className="h-4 w-4 shrink-0 text-primary mt-0.5" />
              <div className="space-y-1 text-xs text-muted-foreground leading-relaxed">
                <p className="font-medium text-foreground">
                  One Campus Identity
                </p>
                <p>
                  Every verified member can both post jobs and apply to campus opportunities. No separate student or employer profiles needed.
                </p>
              </div>
            </div>
          </div>

          <div className="mt-4 text-center text-xs text-muted-foreground">
            Institutional <span className="font-medium text-foreground">.edu</span> email verification unlocks full publishing, applying, and resume features.
          </div>
        </div>
      </div>
    </div>
  );
}

export default function LoginPage() {
  return (
    <Suspense
      fallback={
        <div className="flex min-h-[80vh] items-center justify-center">
          <div className="flex flex-col items-center gap-3 text-muted-foreground">
            <Loader2 className="h-8 w-8 animate-spin text-primary" />
            <p className="text-xs font-medium">Loading sign in...</p>
          </div>
        </div>
      }
    >
      <LoginContent />
    </Suspense>
  );
}
