"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useState } from "react";
import { LogOut, User as UserIcon, Menu, X } from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { cn } from "@/lib/utils";

export function Navbar() {
  const { user, isAuthenticated, isLoading, isVerified, logout } = useAuth();
  const [mobileMenuOpen, setMobileMenuOpen] = useState(false);
  const pathname = usePathname();

  const emailVerified =
    isVerified ||
    Boolean((user as { emailVerified?: boolean } | null)?.emailVerified || user?.isVerified);

  const isRouteActive = (href: string) => {
    if (!pathname) return false;
    if (href === "/jobs") {
      return pathname === "/jobs" || (pathname.startsWith("/jobs/") && pathname !== "/jobs/create");
    }
    if (href === "/jobs/create") {
      return pathname === "/jobs/create";
    }
    return pathname === href || pathname.startsWith(`${href}/`);
  };

  return (
    <>
      <header className="sticky top-0 z-50 w-full border-b border-border/80 bg-background/85 backdrop-blur-md">
        <div className="mx-auto flex h-16 max-w-6xl items-center justify-between px-4 sm:px-6 lg:px-8">
          {/* Brand Logo & Desktop Nav Links */}
          <div className="flex items-center gap-8">
            <Link
              href="/"
              className="font-sans text-lg font-bold tracking-tight text-foreground flex items-center gap-2"
            >
              <span>LYNK</span>
              <Badge
                variant="outline"
                className="hidden sm:inline-flex text-[10px] py-0 px-2 font-mono uppercase tracking-wider"
              >
                CAMPUS
              </Badge>
            </Link>

            {/* Desktop Navigation Links */}
            <nav className="hidden md:flex items-center gap-6">
              <Link
                href="/jobs"
                className={cn(
                  "text-sm transition-colors relative py-1",
                  isRouteActive("/jobs")
                    ? "text-foreground font-medium"
                    : "text-muted-foreground hover:text-foreground"
                )}
              >
                Jobs
                {isRouteActive("/jobs") && (
                  <span className="absolute inset-x-0 -bottom-1 h-[1.5px] bg-foreground rounded-full" />
                )}
              </Link>

              {isAuthenticated && (
                <>
                  <Link
                    href="/contracts"
                    className={cn(
                      "text-sm transition-colors relative py-1",
                      isRouteActive("/contracts")
                        ? "text-foreground font-medium"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    Contracts
                    {isRouteActive("/contracts") && (
                      <span className="absolute inset-x-0 -bottom-1 h-[1.5px] bg-foreground rounded-full" />
                    )}
                  </Link>

                  <Link
                    href="/activity"
                    className={cn(
                      "text-sm transition-colors relative py-1",
                      isRouteActive("/activity")
                        ? "text-foreground font-medium"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    Activity
                    {isRouteActive("/activity") && (
                      <span className="absolute inset-x-0 -bottom-1 h-[1.5px] bg-foreground rounded-full" />
                    )}
                  </Link>

                  <Link
                    href="/jobs/create"
                    className={cn(
                      "text-sm transition-colors relative py-1",
                      isRouteActive("/jobs/create")
                        ? "text-foreground font-medium"
                        : "text-muted-foreground hover:text-foreground"
                    )}
                  >
                    Post a Job
                    {isRouteActive("/jobs/create") && (
                      <span className="absolute inset-x-0 -bottom-1 h-[1.5px] bg-foreground rounded-full" />
                    )}
                  </Link>
                </>
              )}
            </nav>
          </div>

          {/* Desktop Right Actions */}
          <div className="hidden md:flex items-center gap-3">
            {isLoading ? (
              <div className="flex items-center gap-2">
                <div className="h-8 w-20 animate-pulse rounded-md bg-muted" />
              </div>
            ) : isAuthenticated && user ? (
              <div className="flex items-center gap-3">
                {/* Verification Status Badge */}
                {emailVerified ? (
                  <Badge variant="default" className="text-[10px] py-0 px-2">
                    VERIFIED .EDU
                  </Badge>
                ) : (
                  <Link href="/verify-email" title="Click to verify your institutional .edu email">
                    <Badge variant="warning" className="text-[10px] py-0 px-2">
                      VERIFY EMAIL
                    </Badge>
                  </Link>
                )}

                {/* User Identity Display */}
                <Link
                  href="/profile"
                  className={cn(
                    "flex items-center gap-2 rounded-md px-2 py-1.5 transition-colors hover:bg-muted/60",
                    isRouteActive("/profile") && "bg-muted/60"
                  )}
                >
                  <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted text-foreground font-mono text-xs border border-border">
                    {user.name ? (
                      user.name.slice(0, 2).toUpperCase()
                    ) : (
                      <UserIcon className="h-3.5 w-3.5" />
                    )}
                  </div>
                  <span className="text-xs font-medium text-foreground max-w-[120px] truncate hidden sm:inline-block">
                    {user.name || "Profile"}
                  </span>
                </Link>

                {/* Sign Out */}
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={() => logout()}
                  title="Sign Out"
                  className="h-8 px-2.5 text-xs text-muted-foreground hover:text-foreground"
                >
                  <LogOut className="h-3.5 w-3.5 mr-1" />
                  <span>Sign Out</span>
                </Button>
              </div>
            ) : (
              <div className="flex items-center gap-2">
                <Link href="/login">
                  <Button
                    variant="default"
                    size="sm"
                    className="rounded-[6px] px-3.5 text-xs font-medium"
                  >
                    Sign In
                  </Button>
                </Link>
              </div>
            )}
          </div>

          {/* Mobile Menu Toggle */}
          <div className="flex md:hidden items-center gap-2">
            <button
              onClick={() => setMobileMenuOpen(!mobileMenuOpen)}
              className="rounded-md p-2 text-muted-foreground hover:bg-muted/60 hover:text-foreground focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-ring"
              aria-label="Toggle navigation menu"
              aria-expanded={mobileMenuOpen}
            >
              {mobileMenuOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
            </button>
          </div>
        </div>

        {/* Mobile Dropdown Menu */}
        {mobileMenuOpen && (
          <div className="border-b border-border/80 bg-background/95 backdrop-blur-md px-4 py-4 md:hidden space-y-3">
            <nav className="flex flex-col space-y-1">
              <Link
                href="/jobs"
                onClick={() => setMobileMenuOpen(false)}
                className={cn(
                  "rounded-md px-3 py-2 text-sm transition-colors",
                  isRouteActive("/jobs")
                    ? "bg-muted font-medium text-foreground"
                    : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                )}
              >
                Jobs
              </Link>

              {isAuthenticated && (
                <>
                  <Link
                    href="/contracts"
                    onClick={() => setMobileMenuOpen(false)}
                    className={cn(
                      "rounded-md px-3 py-2 text-sm transition-colors",
                      isRouteActive("/contracts")
                        ? "bg-muted font-medium text-foreground"
                        : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                    )}
                  >
                    Contracts
                  </Link>
                  <Link
                    href="/activity"
                    onClick={() => setMobileMenuOpen(false)}
                    className={cn(
                      "rounded-md px-3 py-2 text-sm transition-colors",
                      isRouteActive("/activity")
                        ? "bg-muted font-medium text-foreground"
                        : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                    )}
                  >
                    Activity
                  </Link>
                  <Link
                    href="/jobs/create"
                    onClick={() => setMobileMenuOpen(false)}
                    className={cn(
                      "rounded-md px-3 py-2 text-sm transition-colors",
                      isRouteActive("/jobs/create")
                        ? "bg-muted font-medium text-foreground"
                        : "text-muted-foreground hover:bg-muted/60 hover:text-foreground"
                    )}
                  >
                    Post a Job
                  </Link>
                </>
              )}
            </nav>

            {isAuthenticated && user ? (
              <div className="pt-3 border-t border-border/80 flex flex-col gap-3">
                <div className="flex items-center justify-between">
                  <Link
                    href="/profile"
                    onClick={() => setMobileMenuOpen(false)}
                    className="flex items-center gap-2"
                  >
                    <div className="flex h-7 w-7 items-center justify-center rounded-full bg-muted text-foreground font-mono text-xs border border-border">
                      {user.name ? (
                        user.name.slice(0, 2).toUpperCase()
                      ) : (
                        <UserIcon className="h-3.5 w-3.5" />
                      )}
                    </div>
                    <div className="flex flex-col text-left">
                      <span className="text-xs font-medium text-foreground truncate max-w-[160px]">
                        {user.name || "Member"}
                      </span>
                      <span className="text-[10px] text-muted-foreground truncate max-w-[160px]">
                        {user.email}
                      </span>
                    </div>
                  </Link>

                  {emailVerified ? (
                    <Badge variant="default" className="text-[10px] py-0 px-2">
                      VERIFIED .EDU
                    </Badge>
                  ) : (
                    <Link href="/verify-email" onClick={() => setMobileMenuOpen(false)}>
                      <Badge variant="warning" className="text-[10px] py-0 px-2">
                        VERIFY EMAIL
                      </Badge>
                    </Link>
                  )}
                </div>

                <Button
                  variant="outline"
                  size="sm"
                  onClick={() => {
                    setMobileMenuOpen(false);
                    logout();
                  }}
                  className="w-full justify-center text-xs"
                >
                  <LogOut className="h-3.5 w-3.5 mr-1" />
                  Sign Out
                </Button>
              </div>
            ) : (
              <div className="pt-3 border-t border-border/80">
                <Link
                  href="/login"
                  onClick={() => setMobileMenuOpen(false)}
                  className="w-full block"
                >
                  <Button
                    variant="default"
                    size="sm"
                    className="w-full justify-center rounded-[6px]"
                  >
                    Sign In
                  </Button>
                </Link>
              </div>
            )}
          </div>
        )}
      </header>

      {/* Global verification banner if user is logged in but unverified */}
      {isAuthenticated && !emailVerified && (
        <div className="border-b border-border/80 bg-pastel-yellow/60 px-4 py-2 text-center text-xs text-pastel-yellowText">
          <div className="mx-auto flex max-w-6xl items-center justify-center gap-2">
            <span>
              <strong>Campus verification pending:</strong> Please verify your institutional .edu
              email address to apply for jobs, post gigs, and upload resumes.
            </span>
            <Link
              href="/verify-email"
              className="font-semibold underline underline-offset-2 hover:opacity-80"
            >
              Verify now
            </Link>
          </div>
        </div>
      )}
    </>
  );
}
