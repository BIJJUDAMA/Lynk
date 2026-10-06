import Link from "next/link";

export default function SiteFooter() {
  return (
    <footer className="w-full bg-surface border-t border-line py-12 md:py-16" role="contentinfo">
      <div className="max-w-7xl mx-auto px-6 lg:px-12">
        <div className="flex flex-col md:flex-row justify-between gap-10">
          {/* Brand */}
          <div>
            <p className="font-display text-lg tracking-[0.12em] text-ink uppercase">Lynk</p>
            <p className="mt-2 text-sm text-ink-secondary max-w-xs leading-relaxed">
              Campus work platform for verified university students.
            </p>
          </div>

          {/* Navigation links */}
          <nav aria-label="Footer navigation" className="flex flex-wrap gap-x-12 gap-y-4">
            <div className="flex flex-col gap-3">
              <span className="font-mono text-[10px] uppercase tracking-widest text-ink-secondary">
                Platform
              </span>
              <Link
                href="/jobs"
                className="text-sm text-ink-secondary hover:text-ink transition-colors"
              >
                Opportunities
              </Link>
              <Link
                href="/jobs/create"
                className="text-sm text-ink-secondary hover:text-ink transition-colors"
              >
                Post a gig
              </Link>
              <Link
                href="/contracts"
                className="text-sm text-ink-secondary hover:text-ink transition-colors"
              >
                Contracts
              </Link>
            </div>
            <div className="flex flex-col gap-3">
              <span className="font-mono text-[10px] uppercase tracking-widest text-ink-secondary">
                Account
              </span>
              <Link
                href="/login"
                className="text-sm text-ink-secondary hover:text-ink transition-colors"
              >
                Sign in
              </Link>
              <Link
                href="/login"
                className="text-sm text-ink-secondary hover:text-ink transition-colors"
              >
                Join Lynk
              </Link>
              <Link
                href="/profile"
                className="text-sm text-ink-secondary hover:text-ink transition-colors"
              >
                Profile
              </Link>
            </div>
          </nav>
        </div>

        <div className="mt-12 pt-6 border-t border-line flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
          <p className="font-mono text-[11px] text-ink-secondary">
            Lynk. Campus work platform. Academic year 2026.
          </p>
          <p className="font-mono text-[11px] text-ink-secondary">
            Verified .edu identity required.
          </p>
        </div>
      </div>
    </footer>
  );
}
