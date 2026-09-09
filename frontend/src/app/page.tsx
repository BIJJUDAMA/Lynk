"use client";

import { useEffect, useState, useRef } from "react";
import Link from "next/link";
import {
  Briefcase,
  ArrowRight,
  Sparkles,
  FileText,
  Star,
  GraduationCap,
  Check,
  ChevronRight,
} from "lucide-react";
import { Job } from "@/types/api";
import { listJobs } from "@/lib/api";
import { JobCard } from "@/components/jobs/JobCard";
import { animateHero, animateStaggerList } from "@/lib/animations";

export default function HomePage() {
  const heroRef = useRef<HTMLDivElement | null>(null);
  const jobsGridRef = useRef<HTMLDivElement | null>(null);
  const [featuredJobs, setFeaturedJobs] = useState<Job[]>([]);
  const [isLoadingJobs, setIsLoadingJobs] = useState(true);

  useEffect(() => {
    let isMounted = true;
    async function loadFeatured() {
      try {
        const jobs = await listJobs({ limit: 3, status: "open" });
        if (isMounted && jobs) {
          setFeaturedJobs(jobs);
          if (jobsGridRef.current && jobs.length > 0) {
            animateStaggerList(jobsGridRef.current);
          }
        }
      } catch {
        // Backend unavailable or empty
      } finally {
        if (isMounted) {
          setIsLoadingJobs(false);
        }
      }
    }
    loadFeatured();
    if (heroRef.current) animateHero(heroRef.current);
    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <div className="flex min-h-screen flex-col selection:bg-emerald-500 selection:text-white">
      <main className="flex-1">
        {/* Hero Section */}
        <section ref={heroRef} className="relative overflow-hidden pt-16 pb-20 sm:pt-24 sm:pb-28">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-3xl text-center">
              <h1 className="text-4xl font-extrabold tracking-tight text-slate-900 sm:text-6xl dark:text-white">
                Campus talent you can trust.{" "}
                <span className="text-emerald-600 dark:text-emerald-400">
                  Real projects, guaranteed.
                </span>
              </h1>

              <p className="mt-6 text-lg leading-8 text-slate-600 dark:text-slate-300">
                Lynk connects verified university students with freelance opportunities,
                clear deliverable contracts, and peer reviews.
              </p>

              <div className="mt-10 flex flex-wrap items-center justify-center gap-4">
                <Link
                  href="/jobs"
                  className="inline-flex items-center gap-2 rounded-[10px] bg-emerald-600 px-6 py-3 text-base font-semibold text-white shadow-lg shadow-emerald-500/25 transition hover:bg-emerald-500 hover:shadow-emerald-500/35"
                >
                  <Briefcase className="h-5 w-5" />
                  Browse Active Jobs
                </Link>
                <Link
                  href="/login"
                  className="inline-flex items-center gap-2 rounded-[10px] border border-slate-300 bg-white px-6 py-3 text-base font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
                >
                  Post a Gig as Employer
                </Link>
              </div>
            </div>
          </div>
        </section>

        {/* Featured Opportunities Section */}
        <section className="border-t border-slate-200/80 bg-slate-50/50 py-16 dark:border-slate-800/80 dark:bg-slate-900/30">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
              <div>
                <div className="inline-flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-emerald-600 dark:text-emerald-400">
                  <Sparkles className="h-3.5 w-3.5" />
                  <span>Featured Opportunities</span>
                </div>
                <h2 className="mt-1 text-2xl font-bold tracking-tight text-slate-900 dark:text-white sm:text-3xl">
                  Recent Campus Gigs
                </h2>
                <p className="mt-1 text-sm text-slate-600 dark:text-slate-400">
                  Discover verified projects ready for skilled student freelancers.
                </p>
              </div>

              <Link
                href="/jobs"
                className="inline-flex items-center gap-1.5 text-sm font-semibold text-emerald-600 transition hover:text-emerald-500 dark:text-emerald-400 dark:hover:text-emerald-300"
              >
                <span>View all active jobs</span>
                <ArrowRight className="h-4 w-4" />
              </Link>
            </div>

            {/* Grid of Featured Job Cards or Empty State */}
            {isLoadingJobs ? (
              <div className="mt-8 grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
                {[1, 2, 3].map((n) => (
                  <div
                    key={n}
                    className="h-64 animate-pulse rounded-[10px] border border-slate-200 bg-white p-6 dark:border-slate-800 dark:bg-slate-900"
                  />
                ))}
              </div>
            ) : featuredJobs.length > 0 ? (
              <div ref={jobsGridRef} className="mt-8 grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
                {featuredJobs.map((job) => (
                  <JobCard key={job.id} job={job} />
                ))}
              </div>
            ) : (
              <div className="mt-8 rounded-[10px] border border-dashed border-slate-300 bg-white/50 p-12 text-center dark:border-slate-800 dark:bg-slate-900/50">
                <Briefcase className="mx-auto h-10 w-10 text-slate-400" />
                <h3 className="mt-3 text-base font-semibold text-slate-900 dark:text-white">
                  No gigs posted yet
                </h3>
                <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
                  Be the first to post a freelance opportunity for university students.
                </p>
                <div className="mt-6">
                  <Link
                    href="/jobs/create"
                    className="inline-flex items-center gap-2 rounded-[10px] bg-emerald-600 px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:bg-emerald-500"
                  >
                    Post an Opportunity
                  </Link>
                </div>
              </div>
            )}

            {featuredJobs.length > 0 && (
              <div className="mt-10 text-center">
                <Link
                  href="/jobs"
                  className="inline-flex items-center gap-2 rounded-[10px] border border-slate-300 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
                >
                  Explore All Opportunities
                  <ChevronRight className="h-4 w-4" />
                </Link>
              </div>
            )}
          </div>
        </section>

        {/* Standards Section */}
        <section className="py-20">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-2xl text-center">
              <span className="text-xs font-semibold uppercase tracking-wider text-emerald-600 dark:text-emerald-400">
                Institutional Safety Architecture
              </span>
              <h2 className="mt-2 text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
                The Lynk Standard
              </h2>
              <p className="mt-4 text-base text-slate-600 dark:text-slate-400">
                Freelance marketplaces often struggle with unverified accounts and payment disputes. Lynk provides verified identity, structured contracts, and authenticated reviews.
              </p>
            </div>

            <div className="mt-16 grid grid-cols-1 gap-8 md:grid-cols-3">
              {/* Feature 1 */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
                  <GraduationCap className="h-6 w-6" />
                </div>
                <h3 className="mt-6 text-lg font-bold text-slate-900 dark:text-white">
                  Institutional Email Verification
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-400">
                  Every student must verify an active university email address before submitting proposals or sharing credentials.
                </p>
                <ul className="mt-4 space-y-2 text-xs text-slate-500 dark:text-slate-400">
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Verified university students only
                  </li>
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Clear department affiliations
                  </li>
                </ul>
              </div>

              {/* Feature 2 */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
                  <FileText className="h-6 w-6" />
                </div>
                <h3 className="mt-6 text-lg font-bold text-slate-900 dark:text-white">
                  Binding Campus Contracts
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-400">
                  Accepted proposals automatically create structured contracts that track project status from active work through completion.
                </p>
                <ul className="mt-4 space-y-2 text-xs text-slate-500 dark:text-slate-400">
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Documented scope and compensation
                  </li>
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Clear delivery deadlines
                  </li>
                </ul>
              </div>

              {/* Feature 3 */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400">
                  <Star className="h-6 w-6" />
                </div>
                <h3 className="mt-6 text-lg font-bold text-slate-900 dark:text-white">
                  Authentic Mutual Feedback
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-400">
                  Ratings and reviews can only be submitted once contracts reach completion, ensuring a genuine record of collaboration.
                </p>
                <ul className="mt-4 space-y-2 text-xs text-slate-500 dark:text-slate-400">
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    1 to 5 star verified reviews
                  </li>
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Gated by completed contract state
                  </li>
                </ul>
              </div>
            </div>
          </div>
        </section>

        {/* How It Works Section */}
        <section id="features" className="border-t border-slate-200 bg-slate-100/60 py-20 dark:border-slate-800 dark:bg-slate-900/40">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-2xl text-center">
              <h2 className="text-3xl font-bold tracking-tight text-slate-900 sm:text-4xl dark:text-white">
                How Lynk Works
              </h2>
              <p className="mt-4 text-base text-slate-600 dark:text-slate-400">
                Streamlined collaboration tailored for university students and campus employers.
              </p>
            </div>

            <div className="mt-16 grid grid-cols-1 gap-12 lg:grid-cols-2">
              {/* Student Workflow */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-emerald-600 text-white">
                    <GraduationCap className="h-5 w-5" />
                  </div>
                  <div>
                    <h3 className="text-lg font-bold text-slate-900 dark:text-white">
                      For University Students
                    </h3>
                    <p className="text-xs text-slate-500">Find vetted opportunities across campus</p>
                  </div>
                </div>

                <ol className="mt-6 space-y-4">
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      1
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Sign up with your university email</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Register with your institutional address to access campus opportunities.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      2
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Build your profile and resume</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Highlight coursework, skills, portfolio links, and your current resume.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      3
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Apply and deliver under contract</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Submit proposals, deliver agreed project scope, and earn authentic feedback.
                      </p>
                    </div>
                  </li>
                </ol>
              </div>

              {/* Employer Workflow */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex items-center gap-3">
                  <div className="flex h-10 w-10 items-center justify-center rounded-[10px] bg-emerald-600 text-white">
                    <Briefcase className="h-5 w-5" />
                  </div>
                  <div>
                    <h3 className="text-lg font-bold text-slate-900 dark:text-white">
                      For Campus Employers & Labs
                    </h3>
                    <p className="text-xs text-slate-500">Connect with motivated student talent</p>
                  </div>
                </div>

                <ol className="mt-6 space-y-4">
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      1
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Post your opportunity</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Define project deliverables, required skills, compensation, and deadline.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      2
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Review student proposals</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Evaluate proposals, inspect student resumes, and review past feedback.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      3
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Coordinate work and provide feedback</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Accept proposals to initiate contracts, monitor deliverables, and leave mutual reviews.
                      </p>
                    </div>
                  </li>
                </ol>
              </div>
            </div>
          </div>
        </section>

        {/* Bottom CTA Banner */}
        <section className="py-16">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="relative overflow-hidden rounded-[10px] bg-emerald-600 px-8 py-12 text-center text-white shadow-xl sm:px-16 sm:py-16">
              <h2 className="text-2xl font-extrabold tracking-tight sm:text-4xl">
                Ready to explore campus opportunities?
              </h2>
              <p className="mx-auto mt-4 max-w-xl text-sm sm:text-base text-emerald-100">
                Join verified students and university teams collaborating across research, engineering, and creative projects.
              </p>
              <div className="mt-8 flex flex-wrap justify-center gap-4">
                <Link
                  href="/jobs"
                  className="rounded-[10px] bg-white px-6 py-3 text-sm font-semibold text-emerald-700 shadow-md transition hover:bg-emerald-50"
                >
                  Explore Active Gigs
                </Link>
                <Link
                  href="/login"
                  className="rounded-[10px] border border-white/40 bg-white/10 px-6 py-3 text-sm font-semibold text-white backdrop-blur transition hover:bg-white/20"
                >
                  Sign In with .edu
                </Link>
              </div>
            </div>
          </div>
        </section>
      </main>

      {/* Footer */}
      <footer className="border-t border-slate-200 bg-white py-8 dark:border-slate-800 dark:bg-slate-900">
        <div className="mx-auto flex max-w-7xl flex-col items-center justify-between gap-4 px-4 sm:flex-row sm:px-6 lg:px-8">
          <p className="text-xs text-slate-500 dark:text-slate-400">
            &copy; {new Date().getFullYear()} Lynk. All rights reserved.
          </p>
          <div className="flex items-center gap-6 text-xs text-slate-500 dark:text-slate-400">
            <Link href="/jobs" className="hover:text-emerald-600 dark:hover:text-emerald-400">
              Browse Jobs
            </Link>
            <Link href="/login" className="hover:text-emerald-600 dark:hover:text-emerald-400">
              Sign In
            </Link>
          </div>
        </div>
      </footer>
    </div>
  );
}
