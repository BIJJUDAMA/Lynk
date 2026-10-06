"use client";

import Link from "next/link";
import { ArrowLeft } from "lucide-react";

export default function NotFound() {
  return (
    <div className="flex min-h-[70vh] flex-col items-center justify-center text-center px-6 bg-canvas">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-3">
        404 // Not Found
      </p>
      <h1 className="font-display text-5xl sm:text-6xl font-normal tracking-[-0.025em] text-ink">
        Page not found.
      </h1>
      <p className="mt-4 text-base text-ink-secondary max-w-md leading-relaxed">
        The archive or opportunity you are looking for does not exist or has been moved.
      </p>
      <Link
        href="/"
        className="mt-8 inline-flex items-center gap-2 px-6 py-3 bg-ink text-canvas text-sm tracking-wide hover:bg-ink/90 transition-colors"
      >
        <ArrowLeft className="h-4 w-4" />
        <span>Return to Campus</span>
      </Link>
    </div>
  );
}
