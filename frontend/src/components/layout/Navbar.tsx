"use client";

import Link from "next/link";
import { useState } from "react";
import {
  GraduationCap,
  ShieldCheck,
  ShieldAlert,
  LogOut,
  User as UserIcon,
  PlusCircle,
  Briefcase,
  Menu,
  X,
  FileText,
} from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";

export function Navbar() {
  const { user, isAuthenticated, isLoading, isVerified, logout } = useAuth();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);

  return (
    <>
      <header className="sticky top-0 z-50 border-b border-border bg-background/90 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-4 sm:px-6 lg:px-8">
          {/* Brand Logo */}
          <div className="flex items-center gap-8">
            <Link href="/" className="flex items-center gap-2.5">
              <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-primary text-primary-foreground shadow-sm">
                <GraduationCap className="h-5 w-5" />
              </div>
              <span className="text-lg font-bold tracking-tight text-foreground">
                Lynk<span className="text-primary">.</span>
              </span>
            </Link>

            {/* Desktop Navigation Links */}
            <nav className="hidden md:flex items-center gap-6">
              <Link
                href="/jobs"
                className="text-sm font-medium text-muted-foreground transition hover:text-foreground"
              >
                Browse Jobs
              </Link>

              <Link
                href="/jobs/create"
                className="text-sm font-medium text-muted-foreground transition hover:text-foreground inline-flex items-center gap-1.5"
              >
                <PlusCircle className="h-3.5 w-3.5" />
                Post a Job
              </Link>

              {isAuthenticated && (
                <>
                  <Link
                    href="/activity"
                    className="text-sm font-medium text-muted-foreground transition hover:text-foreground inline-flex items-center gap-1.5"
                  >
                    <Briefcase className="h-3.5 w-3.5" />
                    Activity
                  </Link>

                  <Link
                    href="/profile"
                    className="text-sm font-medium text-muted-foreground transition hover:text-foreground inline-flex items-center gap-1.5"
                  >
                    <UserIcon className="h-3.5 w-3.5" />
                    Profile
                  </Link>
                </>
              )}
            </nav>
          </div>

          {/* Desktop Right Actions */}
          <div className="hidden md:flex items-center gap-4">
            {isLoading ? (
              <div className="flex items-center gap-2">
                <div className="h-8 w-24 animate-pulse rounded-lg bg-muted" />
                <div className="h-8 w-8 animate-pulse rounded-full bg-muted" />
              </div>
            ) : isAuthenticated && user ? (
              <div className="flex items-center gap-3">
                {/* Verification Status Badge */}
                {isVerified ? (
                  <span className="inline-flex items-center gap-1 rounded-full border border-emerald-500/20 bg-emerald-500/10 px-2.5 py-1 text-xs font-medium text-emerald-700 dark:text-emerald-300">
                    <ShieldCheck className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />
                    <span>Campus Verified ✓</span>
                  </span>
                ) : (
                  <span
                    className="inline-flex items-center gap-1 rounded-full border border-amber-500/20 bg-amber-500/10 px-2.5 py-1 text-xs font-medium text-amber-700 dark:text-amber-300"
                    title="Please verify your .edu university email to apply, post, or upload resumes"
                  >
                    <ShieldAlert className="h-3.5 w-3.5 text-amber-600 dark:text-amber-400" />
                    <span>Campus verification pending</span>
                  </span>
                )}

                {/* User Identity Display */}
                <Link
                  href="/profile"
                  className="flex items-center gap-2 rounded-lg p-1.5 transition hover:bg-muted/50"
                >
                  <div className="flex h-8 w-8 items-center justify-center rounded-full bg-muted text-foreground font-semibold text-xs border border-border">
                    {user.name ? (
                      user.name.slice(0, 2).toUpperCase()
                    ) : (
                      <UserIcon className="h-4 w-4" />
                    )}
                  </div>
                  <div className="flex flex-col text-left">
                    <span className="text-xs font-medium text-foreground truncate max-w-[120px]">
                      {user.name || "Member"}
                    </span>
                    <span className="text-[10px] text-muted-foreground truncate max-w-[120px]">
                      {user.email}
                    </span>
                  </div>
                </Link>

                {/* Sign Out */}
                <button
                  onClick={() => logout()}
                  title="Sign Out"
                  className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background px-3 py-1.5 text-xs font-medium text-muted-foreground transition hover:bg-muted hover:text-foreground"
                >
                  <LogOut className="h-3.5 w-3.5" />
                  <span>Sign Out</span>
                </button>
              </div>
            ) : (
              <div className="flex items-center gap-3">
                <Link
                  href="/login"
                  className="rounded-lg px-3 py-1.5 text-xs font-medium text-muted-foreground transition hover:text-foreground"
                >
                  Sign In
                </Link>
                <Link
                  href="/login"
                  className="rounded-lg bg-primary px-3.5 py-1.5 text-xs font-semibold text-primary-foreground shadow-sm transition hover:bg-primary/90"
                >
                  Get Started
                </Link>
              </div>
            )}
          </div>

          {/* Mobile Menu Toggle */}
          <div className="flex md:hidden items-center gap-2">
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="rounded-lg p-2 text-muted-foreground hover:bg-muted hover:text-foreground"
              aria-label="Toggle menu"
            >
              {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
            </button>
          </div>
        </div>

        {/* Mobile Dropdown Menu */}
        {mobileMenuOpen && (
          <div className="border-b border-border bg-background px-4 py-4 md:hidden space-y-3">
            <Link
              href="/jobs"
              onClick={() => setMobileMenuOpen(false)}
              className="block rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted"
            >
              Browse Jobs
            </Link>
            <Link
              href="/jobs/create"
              onClick={() => setMobileMenuOpen(false)}
              className="block rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted"
            >
              Post a Job
            </Link>

            {isAuthenticated ? (
              <>
                <Link
                  href="/activity"
                  onClick={() => setMobileMenuOpen(false)}
                  className="block rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted"
                >
                  Activity & Contracts
                </Link>
                <Link
                  href="/profile"
                  onClick={() => setMobileMenuOpen(false)}
                  className="block rounded-lg px-3 py-2 text-sm font-medium text-foreground hover:bg-muted"
                >
                  Campus Profile
                </Link>
                <div className="pt-2 border-t border-border flex items-center justify-between">
                  <div className="text-xs text-muted-foreground truncate max-w-[200px]">
                    {user?.email}
                  </div>
                  <button
                    onClick={() => {
                      setMobileMenuOpen(false);
                      logout();
                    }}
                    className="inline-flex items-center gap-1 text-xs text-red-600 dark:text-red-400"
                  >
                    <LogOut className="h-3.5 w-3.5" />
                    Sign Out
                  </button>
                </div>
              </>
            ) : (
              <div className="pt-2 border-t border-border flex flex-col gap-2">
                <Link
                  href="/login"
                  onClick={() => setMobileMenuOpen(false)}
                  className="w-full text-center rounded-lg border border-border py-2 text-xs font-medium text-foreground"
                >
                  Sign In
                </Link>
              </div>
            )}
          </div>
        )}
      </header>

      {/* Global verification banner if user is logged in but unverified */}
      {isAuthenticated && !isVerified && (
        <div className="border-b border-amber-500/20 bg-amber-500/10 px-4 py-2 text-center text-xs text-amber-800 dark:text-amber-200">
          <div className="mx-auto flex max-w-7xl items-center justify-center gap-2">
            <ShieldAlert className="h-3.5 w-3.5 shrink-0 text-amber-600 dark:text-amber-400" />
            <span>
              <strong>Campus verification pending.</strong> Please verify your institutional .edu
              email address to apply for jobs, post gigs, and upload resumes.
            </span>
          </div>
        </div>
      )}
    </>
  );
}
