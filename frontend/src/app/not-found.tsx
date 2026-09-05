"use client";

import Link from "next/link";
import { ArrowLeft } from "lucide-react";

export default function NotFound() {
  return (
    <div className="flex min-h-[60vh] flex-col items-center justify-center text-center px-4">
      <h1 className="text-6xl font-bold text-slate-900 dark:text-white">404</h1>
      <h2 className="mt-4 text-xl font-semibold text-slate-700 dark:text-slate-300">
        Page Not Found
      </h2>
      <p className="mt-2 text-sm text-slate-500 dark:text-slate-400">
        The page you are looking for does not exist or has been moved.
      </p>
      <Link
        href="/"
        className="mt-6 inline-flex items-center gap-2 rounded-[10px] bg-emerald-600 px-4 py-2.5 text-sm font-semibold text-white shadow-sm hover:bg-emerald-500"
      >
        <ArrowLeft className="h-4 w-4" />
        Return Home
      </Link>
    </div>
  );
}
