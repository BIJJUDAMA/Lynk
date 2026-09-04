"use client";

import Link from "next/link";
import {
  GraduationCap,
  ShieldCheck,
  ShieldAlert,
  Briefcase,
  LogOut,
  User as UserIcon,
  ChevronRight,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";

export function Navbar() {
  const { user, isAuthenticated, isLoading, isVerified, role, logout } = useAuth();

  return (
    <>
      <header className="sticky top-0 z-50 border-b border-border bg-background/80 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
          {/* Brand Logo */}
          <Link href="/" className="flex items-center gap-2 group">
            <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-primary text-primary-foreground shadow-sm transition group-hover:scale-105">
              <GraduationCap className="h-6 w-6" />
            </div>
            <span className="text-xl font-bold tracking-tight text-foreground">
              Lynk<span className="text-primary">.</span>
            </span>
          </Link>

          {/* Nav Links */}
          <nav className="hidden items-center gap-6 md:flex">
            <Link
              href="/jobs"
              className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
            >
              Explore Jobs
            </Link>

            {isAuthenticated && role === "student" && (
              <>
                <Link
                  href="/applications"
                  className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
                >
                  My Applications
                </Link>
                <Link
                  href="/contracts"
                  className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
                >
                  Contracts
                </Link>
                <Link
                  href="/profile"
                  className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
                >
                  Student Profile
                </Link>
              </>
            )}

            {isAuthenticated && role === "employer" && (
              <>
                <Link
                  href="/employer/jobs"
                  className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
                >
                  My Postings
                </Link>
                <Link
                  href="/jobs/create"
                  className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
                >
                  Post a Job
                </Link>
                <Link
                  href="/contracts"
                  className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
                >
                  Contracts
                </Link>
                <Link
                  href="/profile"
                  className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
                >
                  Company Profile
                </Link>
              </>
            )}

            <Link
              href="/#features"
              className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
            >
              How It Works
            </Link>
          </nav>

          {/* Auth Action Section */}
          <div className="flex items-center gap-3">
            {isLoading ? (
              <div className="flex items-center gap-2">
                <div className="h-8 w-24 animate-pulse rounded-[10px] bg-muted" />
                <div className="h-8 w-8 animate-pulse rounded-[10px] bg-muted" />
              </div>
            ) : isAuthenticated && user ? (
              <div className="flex items-center gap-3">
                {/* Verification Badge */}
                {role === "student" && (
                  <>
                    {isVerified ? (
                      <span className="hidden sm:inline-flex items-center gap-1 rounded-[10px] border border-emerald-200 bg-emerald-50 px-2.5 py-1 text-xs font-semibold text-emerald-700 dark:border-emerald-800 dark:bg-emerald-950/60 dark:text-emerald-300">
                        <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                        Verified .edu
                      </span>
                    ) : (
                      <span className="hidden sm:inline-flex items-center gap-1 rounded-[10px] border border-amber-200 bg-amber-50 px-2.5 py-1 text-xs font-semibold text-amber-700 dark:border-amber-900/60 dark:bg-amber-950/60 dark:text-amber-300" title="Email not yet verified">
                        <ShieldAlert className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                        Unverified .edu
                      </span>
                    )}
                  </>
                )}

                {role === "employer" && (
                  <span className="hidden sm:inline-flex items-center gap-1 rounded-[10px] border border-border bg-secondary px-2.5 py-1 text-xs font-semibold text-secondary-foreground">
                    <Briefcase className="h-3.5 w-3.5 text-primary" />
                    Employer
                  </span>
                )}

                {/* User Identity Display */}
                <div className="flex items-center gap-2">
                  <div className="flex h-8 w-8 items-center justify-center rounded-[10px] bg-accent text-accent-foreground font-semibold text-xs border border-border">
                    {user.name ? user.name.slice(0, 2).toUpperCase() : <UserIcon className="h-4 w-4" />}
                  </div>
                  <div className="hidden lg:flex flex-col text-left">
                    <span className="text-xs font-medium text-foreground truncate max-w-[130px]">
                      {user.name}
                    </span>
                    <span className="text-[10px] text-muted-foreground truncate max-w-[130px]">
                      {user.email}
                    </span>
                  </div>
                </div>

                {/* Logout Button */}
                <button
                  onClick={() => logout()}
                  title="Sign Out"
                  className="inline-flex items-center gap-1.5 rounded-[10px] border border-border bg-card px-3 py-1.5 text-xs font-medium text-foreground shadow-sm transition hover:bg-accent hover:text-accent-foreground"
                >
                  <LogOut className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">Sign Out</span>
                </button>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <Link
                  href="/login"
                  className="rounded-[10px] px-3.5 py-1.5 text-xs sm:text-sm font-medium text-foreground transition hover:bg-secondary"
                >
                  Sign In
                </Link>
                <Link
                  href="/login"
                  className="inline-flex items-center gap-1 rounded-[10px] bg-primary px-3.5 py-1.5 text-xs sm:text-sm font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90"
                >
                  <span>Get Started</span>
                  <ChevronRight className="h-3.5 w-3.5" />
                </Link>
              </div>
            )}
          </div>
        </div>
      </header>

      {/* Institutional Email Warning Banner for unverified students */}
      {isAuthenticated && role === "student" && !isVerified && (
        <div className="border-b border-amber-200 bg-amber-50 px-4 py-2 text-center text-xs text-amber-800 dark:border-amber-900/50 dark:bg-amber-950/40 dark:text-amber-200">
          <div className="mx-auto flex max-w-7xl items-center justify-center gap-2">
            <ShieldAlert className="h-4 w-4 shrink-0 text-amber-600 dark:text-amber-400" />
            <span>
              <strong>Action required:</strong> Please verify your university email address in Keycloak to unlock job applications and resume uploads.
            </span>
          </div>
        </div>
      )}
    </>
  );
}
