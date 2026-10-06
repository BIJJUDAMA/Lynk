"use client";

import Link from "next/link";
import { useState, useEffect } from "react";
import { Menu, X } from "lucide-react";
import { useAuth } from "@/components/auth/AuthProvider";
import { cn } from "@/lib/utils";

export function Navbar() {
  const { isAuthenticated, isLoading, logout } = useAuth();
  const [scrolled, setScrolled] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);

  useEffect(() => {
    const onScroll = () => setScrolled(window.scrollY > 24);
    window.addEventListener("scroll", onScroll, { passive: true });
    return () => window.removeEventListener("scroll", onScroll);
  }, []);

  return (
    <>
      <header
        className={cn(
          "fixed top-0 left-0 right-0 z-50 transition-all duration-500",
          scrolled ? "bg-canvas/90 backdrop-blur-md border-b border-line/60" : "bg-transparent"
        )}
      >
        <div className="mx-auto flex h-16 max-w-7xl items-center justify-between px-6 lg:px-8">
          {/* Wordmark */}
          <Link
            href="/"
            className="font-display text-lg font-normal tracking-[0.12em] text-ink uppercase"
          >
            Lynk
          </Link>

          {/* Desktop nav */}
          <nav className="hidden md:flex items-center gap-8">
            <Link
              href="/jobs"
              className="text-sm text-ink-secondary hover:text-ink transition-colors tracking-wide"
            >
              Explore
            </Link>
            <a
              href="#how-it-works"
              className="text-sm text-ink-secondary hover:text-ink transition-colors tracking-wide"
            >
              How it works
            </a>
            {isAuthenticated ? (
              <>
                <Link
                  href="/activity"
                  className="text-sm text-ink-secondary hover:text-ink transition-colors tracking-wide"
                >
                  Activity
                </Link>
                <button
                  onClick={() => logout()}
                  className="text-sm text-ink-secondary hover:text-ink transition-colors tracking-wide"
                >
                  Sign out
                </button>
              </>
            ) : (
              <>
                {!isLoading && (
                  <Link
                    href="/login"
                    className="text-sm text-ink-secondary hover:text-ink transition-colors tracking-wide"
                  >
                    Sign in
                  </Link>
                )}
                <Link
                  href="/login"
                  className="text-sm px-4 py-2 border border-ink text-ink hover:bg-ink hover:text-canvas transition-colors tracking-wide"
                >
                  Join Lynk
                </Link>
              </>
            )}
          </nav>

          {/* Mobile toggle */}
          <button
            className="md:hidden p-2 text-ink-secondary hover:text-ink transition-colors"
            onClick={() => setMobileOpen(!mobileOpen)}
            aria-label="Toggle menu"
            aria-expanded={mobileOpen}
          >
            {mobileOpen ? <X className="h-5 w-5" /> : <Menu className="h-5 w-5" />}
          </button>
        </div>

        {/* Mobile menu */}
        {mobileOpen && (
          <div className="md:hidden bg-canvas/95 backdrop-blur-md border-t border-line/60 px-6 py-6 space-y-4">
            <Link
              href="/jobs"
              onClick={() => setMobileOpen(false)}
              className="block text-sm text-ink-secondary hover:text-ink transition-colors py-2"
            >
              Explore
            </Link>
            <a
              href="#how-it-works"
              onClick={() => setMobileOpen(false)}
              className="block text-sm text-ink-secondary hover:text-ink transition-colors py-2"
            >
              How it works
            </a>
            {isAuthenticated ? (
              <button
                onClick={() => {
                  setMobileOpen(false);
                  logout();
                }}
                className="block text-sm text-ink-secondary hover:text-ink transition-colors py-2 w-full text-left"
              >
                Sign out
              </button>
            ) : (
              <>
                <Link
                  href="/login"
                  onClick={() => setMobileOpen(false)}
                  className="block text-sm text-ink-secondary hover:text-ink transition-colors py-2"
                >
                  Sign in
                </Link>
                <Link
                  href="/login"
                  onClick={() => setMobileOpen(false)}
                  className="inline-block text-sm px-4 py-2 border border-ink text-ink hover:bg-ink hover:text-canvas transition-colors"
                >
                  Join Lynk
                </Link>
              </>
            )}
          </div>
        )}
      </header>
    </>
  );
}
