"use client";

import { useState } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import {
  GraduationCap,
  Briefcase,
  ShieldCheck,
  ArrowRight,
  Sparkles,
  ArrowLeft,
  CheckCircle2,
  Lock,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";

export default function LoginPage() {
  const { login, register, isAuthenticated, user, logout } = useAuth();
  const [authMode, setAuthMode] = useState<"login" | "register">("login");
  const [selectedRole, setSelectedRole] = useState<"student" | "employer">("student");
  const [isRedirecting, setIsRedirecting] = useState<boolean>(false);
  const router = useRouter();

  const handleAction = async (role: "student" | "employer") => {
    setIsRedirecting(true);
    try {
      if (authMode === "login") {
        await login({ roleHint: role });
      } else {
        await register({ roleHint: role });
      }
    } catch (err) {
      console.error("Auth action error:", err);
      setIsRedirecting(false);
    }
  };

  if (isAuthenticated && user) {
    return (
      <div className="flex min-h-[85vh] items-center justify-center px-4 py-12">
        <div className="w-full max-w-md rounded-[10px] border border-slate-200 bg-white p-8 text-center shadow-lg dark:border-slate-800 dark:bg-slate-900">
          <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-[10px] bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
            <CheckCircle2 className="h-8 w-8" />
          </div>

          <h2 className="mt-5 text-2xl font-bold tracking-tight text-slate-900 dark:text-white">
            Already Signed In
          </h2>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            You are logged in as{" "}
            <span className="font-semibold text-slate-900 dark:text-white">
              {user.email}
            </span>
          </p>

          <div className="mt-8 flex flex-col gap-3">
            <Link
              href="/jobs"
              className="inline-flex items-center justify-center gap-2 rounded-[10px] bg-primary text-primary-foreground px-5 py-3 text-sm font-semibold text-white shadow-md shadow-sm transition hover:bg-primary"
            >
              Explore Active Jobs
              <ArrowRight className="h-4 w-4" />
            </Link>
            <button
              onClick={() => logout()}
              className="rounded-[10px] border border-slate-300 bg-white px-5 py-2.5 text-sm font-medium text-slate-700 transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
            >
              Sign Out & Switch Account
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex min-h-[85vh] flex-col justify-center px-4 py-12 sm:px-6 lg:px-8">
      <div className="sm:mx-auto sm:w-full sm:max-w-md">
        <div className="text-center">
          <Link
            href="/"
            className="inline-flex items-center gap-2 text-xs font-medium text-slate-500 transition hover:text-primary dark:text-slate-400 dark:hover:text-primary"
          >
            <ArrowLeft className="h-3.5 w-3.5" />
            Back to Home
          </Link>

          <div className="mx-auto mt-4 flex h-12 w-12 items-center justify-center rounded-[10px] bg-primary text-primary-foreground text-white shadow-md shadow-sm">
            <GraduationCap className="h-7 w-7" />
          </div>

          <h1 className="mt-4 text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white">
            {authMode === "login" ? "Welcome back to Lynk" : "Join Lynk Marketplace"}
          </h1>
          <p className="mt-2 text-sm text-slate-600 dark:text-slate-400">
            {authMode === "login"
              ? "Sign in with your verified campus identity"
              : "Register your account and start collaborating"}
          </p>
        </div>

        {/* Tab Switcher: Sign In vs Register */}
        <div className="mt-8 flex rounded-[10px] bg-slate-100 p-1 dark:bg-slate-800">
          <button
            type="button"
            onClick={() => setAuthMode("login")}
            className={`flex-1 rounded-[10px] py-2.5 text-xs font-semibold transition ${
              authMode === "login"
                ? "bg-white text-slate-900 shadow-sm dark:bg-slate-900 dark:text-white"
                : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            }`}
          >
            Sign In
          </button>
          <button
            type="button"
            onClick={() => setAuthMode("register")}
            className={`flex-1 rounded-[10px] py-2.5 text-xs font-semibold transition ${
              authMode === "register"
                ? "bg-white text-slate-900 shadow-sm dark:bg-slate-900 dark:text-white"
                : "text-slate-600 hover:text-slate-900 dark:text-slate-400 dark:hover:text-white"
            }`}
          >
            Create Account
          </button>
        </div>
      </div>

      <div className="mt-6 sm:mx-auto sm:w-full sm:max-w-md">
        <div className="space-y-4">
          {/* Option 1: Student */}
          <div
            onClick={() => setSelectedRole("student")}
            className={`relative cursor-pointer rounded-[10px] border-2 p-5 transition ${
              selectedRole === "student"
                ? "border-primary bg-emerald-50 dark:bg-emerald-950/30 shadow-md"
                : "border-border bg-card hover:border-primary/50"
            }`}
          >
            <div className="flex items-start gap-4">
              <div
                className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-[10px] transition ${
                  selectedRole === "student"
                    ? "bg-primary text-primary-foreground"
                    : "bg-secondary text-secondary-foreground"
                }`}
              >
                <GraduationCap className="h-6 w-6" />
              </div>

              <div className="flex-1">
                <div className="flex items-center justify-between">
                  <h2 className="text-base font-bold text-foreground">
                    University Student
                  </h2>
                  <span className="inline-flex items-center gap-1 rounded-[10px] border border-emerald-200 bg-emerald-50 px-2 py-0.5 text-[11px] font-semibold text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300">
                    <ShieldCheck className="h-3 w-3" />
                    .edu Required
                  </span>
                </div>
                <p className="mt-1 text-xs text-muted-foreground">
                  Access paid campus gigs, submit proposals, upload resumes, and build your verified reputation.
                </p>
              </div>
            </div>
          </div>

          {/* Option 2: Employer */}
          <div
            onClick={() => setSelectedRole("employer")}
            className={`relative cursor-pointer rounded-[10px] border-2 p-5 transition ${
              selectedRole === "employer"
                ? "border-primary bg-emerald-50 dark:bg-emerald-950/30 shadow-md"
                : "border-border bg-card hover:border-primary/50"
            }`}
          >
            <div className="flex items-start gap-4">
              <div
                className={`flex h-11 w-11 shrink-0 items-center justify-center rounded-[10px] transition ${
                  selectedRole === "employer"
                    ? "bg-primary text-primary-foreground text-white"
                    : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300"
                }`}
              >
                <Briefcase className="h-6 w-6" />
              </div>

              <div className="flex-1">
                <div className="flex items-center justify-between">
                  <h2 className="text-base font-bold text-slate-900 dark:text-white">
                    Campus Employer
                  </h2>
                  <span className="inline-flex items-center gap-1 rounded-full bg-emerald-50 dark:bg-emerald-950/50 px-2 py-0.5 text-[11px] font-semibold text-emerald-800 dark:text-emerald-300 dark:bg-emerald-950/60 dark:text-emerald-300">
                    <Sparkles className="h-3 w-3" />
                    Hire Talent
                  </span>
                </div>
                <p className="mt-1 text-xs text-slate-600 dark:text-slate-400">
                  Post freelance jobs, review student proposals, approve milestones, and release ratings.
                </p>
              </div>
            </div>
          </div>

          {/* Submit Action Button */}
          <button
            type="button"
            disabled={isRedirecting}
            onClick={() => handleAction(selectedRole)}
            className="mt-6 flex w-full items-center justify-center gap-2 rounded-[10px] bg-primary text-primary-foreground py-3.5 text-sm font-semibold text-white shadow-lg shadow-sm transition hover:bg-primary disabled:opacity-60"
          >
            {isRedirecting ? (
              <span className="inline-flex items-center gap-2">
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-white border-t-transparent" />
                Redirecting to Keycloak IAM...
              </span>
            ) : (
              <>
                <span>
                  {authMode === "login"
                    ? `Sign In as ${selectedRole === "student" ? "Student" : "Employer"}`
                    : `Register as ${selectedRole === "student" ? "Student" : "Employer"}`}
                </span>
                <ArrowRight className="h-4 w-4" />
              </>
            )}
          </button>
        </div>

        {/* Security & Verification Disclaimer */}
        <div className="mt-6 rounded-[10px] border border-slate-200 bg-slate-50/70 p-3.5 text-center text-xs text-slate-500 dark:border-slate-800 dark:bg-slate-900/60 dark:text-slate-400">
          <div className="flex items-center justify-center gap-1.5 font-medium text-slate-700 dark:text-slate-300">
            <Lock className="h-3.5 w-3.5 text-primary" />
            <span>Secure OIDC PKCE Authentication</span>
          </div>
          <p className="mt-1 text-[11px]">
            Authentication is powered by Keycloak with S256 PKCE security. Student accounts enforce university email verification.
          </p>
        </div>
      </div>
    </div>
  );
}
