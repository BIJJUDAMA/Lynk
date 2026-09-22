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
  Mail,
  Eye,
  EyeOff,
  AlertCircle,
  Loader2,
  RefreshCw,
  LogOut,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { isEduEmail } from "@/lib/email-validation";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Badge } from "@/components/ui/badge";

function LoginContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const redirectPath = searchParams.get("redirect") || "/jobs";

  const { login, register, isAuthenticated, user, isVerified, logout, resendVerificationEmail } =
    useAuth();

  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [firstName, setFirstName] = useState("");
  const [lastName, setLastName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [emailTouched, setEmailTouched] = useState(false);
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
  const showEduWarning = isRegistering && emailTouched && hasEmail && !isEdu;

  const isFormValid =
    trimmedEmail.length > 0 &&
    password.length >= 8 &&
    (!isRegistering || (isEdu && firstName.trim().length > 0));

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErrorMessage(null);

    if (isRegistering && !isEdu) {
      setErrorMessage(
        "Institutional .edu address required. Only university email addresses are eligible for campus membership."
      );
      return;
    }

    if (!password) {
      setErrorMessage("Please enter your password.");
      return;
    }

    if (password.length < 8) {
      setErrorMessage("Password must be at least 8 characters long.");
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
        router.push("/verify-email");
      }
    } catch (err: unknown) {
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
      <div className="min-h-screen flex items-center justify-center bg-background px-4 py-12">
        <div className="w-full max-w-md rounded-xl border border-border bg-card p-8 text-center shadow-none space-y-6">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-yellow text-pastel-yellowText">
            <ShieldAlert className="h-6 w-6" />
          </div>

          <div>
            <Badge variant="warning">Verification Pending</Badge>
            <h2 className="mt-3 font-serif text-2xl font-normal tracking-tight text-foreground">
              Verify University Email
            </h2>
            <p className="mt-2 text-xs text-muted-foreground leading-relaxed">
              A verification link was sent to your university inbox. Confirm your email address to
              unlock campus opportunities and contract participation.
            </p>
          </div>

          {user?.email && (
            <div className="inline-block rounded-md border border-border bg-muted/40 px-3.5 py-1.5 font-mono text-xs text-foreground">
              {user.email}
            </div>
          )}

          {resendSuccess && (
            <div className="flex items-center gap-2 rounded-lg border border-pastel-greenText/20 bg-pastel-green p-3 text-left font-mono text-xs text-pastel-greenText">
              <CheckCircle2 className="h-4 w-4 shrink-0" />
              <span>A fresh verification link has been dispatched to your inbox.</span>
            </div>
          )}

          {resendError && (
            <div className="flex items-center gap-2 rounded-lg border border-pastel-redText/20 bg-pastel-red p-3 text-left font-mono text-xs text-pastel-redText">
              <AlertCircle className="h-4 w-4 shrink-0" />
              <span>{resendError}</span>
            </div>
          )}

          <div className="space-y-3 pt-2">
            <button
              type="button"
              onClick={handleResendVerification}
              disabled={isResending}
              className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#111111] px-4 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:opacity-50"
            >
              {isResending ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>Dispatching link...</span>
                </>
              ) : (
                <>
                  <Mail className="h-3.5 w-3.5" />
                  <span>Resend Verification Email</span>
                </>
              )}
            </button>

            <Button
              type="button"
              variant="outline"
              onClick={() => window.location.reload()}
              className="w-full text-xs"
            >
              <RefreshCw className="h-3.5 w-3.5 mr-1.5" />
              <span>Check Verification Status</span>
            </Button>

            <button
              type="button"
              onClick={() => logout()}
              className="inline-flex items-center justify-center gap-1.5 text-xs text-muted-foreground hover:text-foreground transition-colors pt-2"
            >
              <LogOut className="h-3 w-3" />
              <span>Sign out</span>
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
      <div className="min-h-screen flex items-center justify-center bg-background px-4 py-12">
        <div className="w-full max-w-md rounded-xl border border-border bg-card p-8 text-center shadow-none space-y-6">
          <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-xl bg-pastel-green text-pastel-greenText">
            <CheckCircle2 className="h-6 w-6" />
          </div>

          <div>
            <Badge variant="default">VERIFIED .EDU</Badge>
            <h2 className="mt-3 font-serif text-2xl font-normal tracking-tight text-foreground">
              Signed In
            </h2>
            <p className="mt-1 font-mono text-xs text-muted-foreground">{user.email}</p>
          </div>

          <div className="space-y-2.5 pt-2">
            <Link
              href="/jobs"
              className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#111111] px-4 py-2.5 text-xs font-medium text-white hover:bg-[#222222] transition-colors active:scale-[0.98]"
            >
              <span>Explore Opportunities</span>
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
            <Button asChild variant="outline" className="w-full text-xs">
              <Link href="/contracts">Contracts &amp; Deliverables</Link>
            </Button>
            <Button asChild variant="outline" className="w-full text-xs">
              <Link href="/profile">Campus Profile</Link>
            </Button>
            <button
              onClick={() => logout()}
              className="text-xs text-muted-foreground hover:text-foreground transition-colors pt-2"
            >
              Sign out
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
    <div className="min-h-screen flex items-center justify-center bg-background px-4 py-12">
      <div className="w-full max-w-md space-y-6">
        {/* Back Link */}
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 font-mono text-xs text-muted-foreground transition-colors hover:text-foreground"
        >
          <ArrowLeft className="h-3 w-3" />
          <span>Back to Lynk</span>
        </Link>

        {/* Card Container */}
        <div className="rounded-xl border border-border bg-card p-8 shadow-none space-y-6">
          {/* Header */}
          <div className="text-center space-y-1">
            <span className="font-mono text-xs uppercase tracking-widest text-muted-foreground">
              LYNK
            </span>
            <h1 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground">
              {authMode === "login" ? "Sign in to your account" : "Join your campus exchange"}
            </h1>
            <p className="text-xs text-muted-foreground">Institutional .edu address required.</p>
          </div>

          {/* Clean Underline Tab Switcher */}
          <div className="flex border-b border-border">
            <button
              type="button"
              onClick={() => {
                setAuthMode("login");
                setErrorMessage(null);
              }}
              className={`flex-1 pb-2.5 text-xs font-medium border-b-2 transition-colors ${
                authMode === "login"
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
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
              className={`flex-1 pb-2.5 text-xs font-medium border-b-2 transition-colors ${
                authMode === "register"
                  ? "border-foreground text-foreground"
                  : "border-transparent text-muted-foreground hover:text-foreground"
              }`}
            >
              Create Account
            </button>
          </div>

          {/* Error Banner */}
          {errorMessage && (
            <div className="flex items-start gap-2.5 rounded-md border border-pastel-redText/20 bg-pastel-red p-3 text-xs text-pastel-redText">
              <AlertCircle className="h-4 w-4 shrink-0 mt-0.5" />
              <div className="flex-1">{errorMessage}</div>
            </div>
          )}

          {/* Form */}
          <form onSubmit={handleSubmit} className="space-y-4">
            {/* For Create Account: First & Last Name */}
            {isRegistering && (
              <div className="grid grid-cols-2 gap-3">
                <div className="space-y-1.5 text-left">
                  <label className="block text-xs font-medium text-foreground">First Name</label>
                  <Input
                    type="text"
                    required
                    value={firstName}
                    onChange={(e) => setFirstName(e.target.value)}
                    placeholder="e.g. Sarah"
                    className="text-sm"
                  />
                </div>
                <div className="space-y-1.5 text-left">
                  <label className="block text-xs font-medium text-foreground">Last Name</label>
                  <Input
                    type="text"
                    required
                    value={lastName}
                    onChange={(e) => setLastName(e.target.value)}
                    placeholder="e.g. Chen"
                    className="text-sm"
                  />
                </div>
              </div>
            )}

            {/* Email Field */}
            <div className="space-y-1.5 text-left">
              <label htmlFor="email" className="block text-xs font-medium text-foreground">
                Institutional Email Address
              </label>
              <div className="relative">
                <Input
                  id="email"
                  name="email"
                  type="email"
                  autoComplete="email"
                  required
                  value={email}
                  onBlur={() => setEmailTouched(true)}
                  onChange={(e) => {
                    setEmail(e.target.value);
                    if (errorMessage) setErrorMessage(null);
                  }}
                  placeholder={
                    isRegistering ? "student@university.edu" : "your.name@university.edu"
                  }
                  className="font-mono text-sm"
                />
              </div>

              {/* Real-time .edu warning */}
              {showEduWarning && (
                <p className="font-mono text-xs text-pastel-yellowText pt-0.5">
                  Institutional .edu address required.
                </p>
              )}
            </div>

            {/* Password Field */}
            <div className="space-y-1.5 text-left">
              <div className="flex items-center justify-between">
                <label htmlFor="password" className="block text-xs font-medium text-foreground">
                  Password
                </label>
                <button
                  type="button"
                  onClick={() => setShowPassword(!showPassword)}
                  className="text-xs text-muted-foreground hover:text-foreground transition-colors inline-flex items-center gap-1"
                >
                  {showPassword ? <EyeOff className="h-3 w-3" /> : <Eye className="h-3 w-3" />}
                  <span>{showPassword ? "Hide" : "Show"}</span>
                </button>
              </div>
              <Input
                id="password"
                name="password"
                type={showPassword ? "text" : "password"}
                autoComplete={isRegistering ? "new-password" : "current-password"}
                required
                minLength={8}
                value={password}
                onChange={(e) => {
                  setPassword(e.target.value);
                  if (errorMessage) setErrorMessage(null);
                }}
                placeholder="Minimum 8 characters"
                className="font-mono text-sm"
              />
            </div>

            {/* Submit Button */}
            <button
              type="submit"
              disabled={isSubmitting || !isFormValid}
              className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-[#111111] px-5 py-2.5 text-xs font-medium text-white shadow-none hover:bg-[#222222] transition-colors active:scale-[0.98] disabled:pointer-events-none disabled:opacity-50"
            >
              {isSubmitting ? (
                <>
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  <span>{authMode === "login" ? "Authenticating..." : "Creating Account..."}</span>
                </>
              ) : (
                <span>{authMode === "login" ? "Sign In" : "Create Campus Account"}</span>
              )}
            </button>
          </form>

          {/* Institutional Trust Note */}
          <div className="pt-2 border-t border-border flex items-start gap-2 text-left">
            <ShieldCheck className="h-4 w-4 shrink-0 text-muted-foreground mt-0.5" />
            <p className="text-[11px] text-muted-foreground leading-relaxed">
              Every verified campus account can both publish project opportunities and submit
              deliverable proposals.
            </p>
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
        <div className="min-h-screen flex items-center justify-center bg-background">
          <Loader2 className="h-6 w-6 animate-spin text-muted-foreground" />
        </div>
      }
    >
      <LoginContent />
    </Suspense>
  );
}
