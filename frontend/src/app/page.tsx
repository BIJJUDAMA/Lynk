"use client";

import { useEffect, useState, useRef } from "react";
import Link from "next/link";
import {
  Briefcase,
  ShieldCheck,
  ArrowRight,
  Sparkles,
  CheckCircle2,
  FileText,
  Star,
  GraduationCap,
  Users,
  Check,
  ChevronRight,
} from "lucide-react";
import { Job } from "@/types/api";
import { listJobs } from "@/lib/api";
import { JobCard } from "@/components/jobs/JobCard";
import { animateHero, animateStaggerList } from "@/lib/animations";

// High-quality showcase gigs displayed when database is fresh or offline
const SHOWCASE_JOBS: Job[] = [
  {
    id: "showcase-job-1",
    employer_id: "showcase-emp-1",
    title: "Full-Stack Developer for AI Research Portal",
    description:
      "Looking for a skilled CS student to build an interactive dashboard visualizing real-time climate telemetry models. Experience with React and REST APIs required.",
    budget: 1200,
    pay_type: "fixed",
    required_skills: ["React", "TypeScript", "Tailwind CSS", "REST APIs"],
    department: "Computer Science & Engineering",
    deadline: new Date(Date.now() + 14 * 86400000).toISOString(),
    status: "open",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    employer: {
      id: "showcase-emp-1",
      email: "lab-director@university.edu",
      company_or_org: "Cognitive Systems Research Lab",
      contact_name: "Dr. Elena Vance",
      website: "https://lab.university.edu",
    },
  },
  {
    id: "showcase-job-2",
    employer_id: "showcase-emp-2",
    title: "UI/UX Designer for Campus Dining App Redesign",
    description:
      "Redesign user workflows and interactive mobile mockups for the university dining meal plan mobile portal. Need Figma wireframes and design system components.",
    budget: 800,
    pay_type: "fixed",
    required_skills: ["Figma", "UI/UX Design", "Wireframing", "Mobile Design"],
    department: "Design & Creative Arts",
    deadline: new Date(Date.now() + 10 * 86400000).toISOString(),
    status: "open",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    employer: {
      id: "showcase-emp-2",
      email: "aux-services@university.edu",
      company_or_org: "Campus Auxiliary Services",
      contact_name: "Marcus Holloway",
      website: "https://services.university.edu",
    },
  },
  {
    id: "showcase-job-3",
    employer_id: "showcase-emp-3",
    title: "Data Analyst for Biotech Cell Microscopy Dataset",
    description:
      "Perform exploratory statistical data analysis and write automated python preprocessing scripts for high-content fluorescence microscopy image arrays.",
    budget: 35,
    pay_type: "hourly",
    required_skills: ["Python", "Pandas", "Statistical Analysis", "Jupyter"],
    department: "Biology & Life Sciences",
    deadline: new Date(Date.now() + 21 * 86400000).toISOString(),
    status: "open",
    created_at: new Date().toISOString(),
    updated_at: new Date().toISOString(),
    employer: {
      id: "showcase-emp-3",
      email: "bio-informatics@university.edu",
      company_or_org: "Genomic Medicine Initiative",
      contact_name: "Prof. Sarah Chen",
      website: "https://genomics.university.edu",
    },
  },
];

export default function HomePage() {
  const heroRef = useRef<HTMLDivElement | null>(null);
  const jobsGridRef = useRef<HTMLDivElement | null>(null);
  const [featuredJobs, setFeaturedJobs] = useState<Job[]>(SHOWCASE_JOBS);
  const [isLiveLoaded, setIsLiveLoaded] = useState(false);

  useEffect(() => {
    let isMounted = true;
    async function loadFeatured() {
      try {
        const jobs = await listJobs({ limit: 3, status: "open" });
        if (isMounted && jobs && jobs.length > 0) {
          setFeaturedJobs(jobs);
          if (jobsGridRef.current) animateStaggerList(jobsGridRef.current);
          setIsLiveLoaded(true);
        }
      } catch {
        // Fallback silently to showcase jobs if backend is not yet populated
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
        <section ref={heroRef} className="relative overflow-hidden pt-12 pb-20 sm:pt-20 sm:pb-28">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-3xl text-center">
              <div className="mb-6 inline-flex items-center gap-2 rounded-full border border-emerald-200 bg-emerald-50/70 px-3.5 py-1.5 text-xs font-semibold text-emerald-700 shadow-sm dark:border-emerald-900/50 dark:bg-emerald-950/40 dark:text-emerald-300">
                <Sparkles className="h-3.5 w-3.5" />
                <span>Verified .edu University Freelance Marketplace</span>
              </div>

              <h1 className="text-4xl font-extrabold tracking-tight text-slate-900 sm:text-6xl dark:text-white">
                Campus talent you can trust.{" "}
                <span className="bg-gradient-to-r from-emerald-600 to-emerald-600 bg-clip-text text-transparent">
                  Real projects, guaranteed.
                </span>
              </h1>

              <p className="mt-6 text-lg leading-8 text-slate-600 dark:text-slate-300">
                Lynk bridges verified university students with high-impact freelance jobs,
                structured milestones, automated contracts, and peer ratings. No spam, no scams.
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

              {/* Trust Badges */}
              <div className="mt-12 flex flex-wrap items-center justify-center gap-6 text-xs text-slate-500 dark:text-slate-400 sm:text-sm">
                <div className="flex items-center gap-2">
                  <ShieldCheck className="h-4 w-4 text-emerald-500" />
                  <span>Institutional Email Verification</span>
                </div>
                <div className="flex items-center gap-2">
                  <FileText className="h-4 w-4 text-emerald-500" />
                  <span>Structured Milestone Contracts</span>
                </div>
                <div className="flex items-center gap-2">
                  <Star className="h-4 w-4 text-amber-500" />
                  <span>Authentic Peer Reviews</span>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Featured Live Jobs Preview Section */}
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

            {/* Grid of Featured Job Cards */}
            <div className="mt-8 grid grid-cols-1 gap-6 md:grid-cols-2 lg:grid-cols-3">
              {featuredJobs.map((job) => (
                <JobCard key={job.id} job={job} />
              ))}
            </div>

            <div className="mt-10 text-center">
              <Link
                href="/jobs"
                className="inline-flex items-center gap-2 rounded-[10px] border border-slate-300 bg-white px-5 py-2.5 text-sm font-semibold text-slate-700 shadow-sm transition hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-200 dark:hover:bg-slate-700"
              >
                Explore All Opportunities
                <ChevronRight className="h-4 w-4" />
              </Link>
            </div>
          </div>
        </section>

        {/* Trust Value Proposition Section */}
        <section className="py-20">
          <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8">
            <div className="mx-auto max-w-2xl text-center">
              <span className="text-xs font-semibold uppercase tracking-wider text-emerald-600 dark:text-emerald-400">
                Institutional Safety Architecture
              </span>
              <h2 className="mt-2 text-3xl font-extrabold tracking-tight text-slate-900 dark:text-white sm:text-4xl">
                The Lynk High-Trust Standard
              </h2>
              <p className="mt-4 text-base text-slate-600 dark:text-slate-400">
                Traditional gig networks are flooded with unverified actors, fake proposals, and payment disputes. Lynk replaces ambiguity with strict cryptographic identity and verifiable contracts.
              </p>
            </div>

            <div className="mt-16 grid grid-cols-1 gap-8 md:grid-cols-3">
              {/* Feature 1 */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
                  <GraduationCap className="h-6 w-6" />
                </div>
                <h3 className="mt-6 text-lg font-bold text-slate-900 dark:text-white">
                  Strict .edu Email Verification
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-400">
                  Keycloak OIDC checks ensure only students with active university institutional email addresses can submit proposals or upload resumes.
                </p>
                <ul className="mt-4 space-y-2 text-xs text-slate-500 dark:text-slate-400">
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    No spam proposals from non-students
                  </li>
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Verified department affiliations
                  </li>
                </ul>
              </div>

              {/* Feature 2 */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-emerald-50 text-emerald-600 dark:bg-emerald-950/60 dark:text-emerald-400">
                  <FileText className="h-6 w-6" />
                </div>
                <h3 className="mt-6 text-lg font-bold text-slate-900 dark:text-white">
                  Deterministic Milestone Contracts
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-400">
                  Accepted applications automatically generate binding state-machine contracts (Draft &rarr; Active &rarr; Completed), locking in scope and agreed deliverables.
                </p>
                <ul className="mt-4 space-y-2 text-xs text-slate-500 dark:text-slate-400">
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Agreed scope and compensation
                  </li>
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    Clear completion timestamps
                  </li>
                </ul>
              </div>

              {/* Feature 3 */}
              <div className="rounded-[10px] border border-slate-200 bg-white p-8 shadow-sm dark:border-slate-800 dark:bg-slate-900">
                <div className="flex h-12 w-12 items-center justify-center rounded-[10px] bg-amber-50 text-amber-600 dark:bg-amber-950/60 dark:text-amber-400">
                  <Star className="h-6 w-6" />
                </div>
                <h3 className="mt-6 text-lg font-bold text-slate-900 dark:text-white">
                  Verified Academic Reputation
                </h3>
                <p className="mt-2 text-sm leading-relaxed text-slate-600 dark:text-slate-400">
                  Reviews can only be submitted for completed contracts. Build a permanent, portable track record of on-campus impact that recruiters can trust.
                </p>
                <ul className="mt-4 space-y-2 text-xs text-slate-500 dark:text-slate-400">
                  <li className="flex items-center gap-2">
                    <Check className="h-3.5 w-3.5 text-emerald-500" />
                    1 to 5 star authentic mutual reviews
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
                    <p className="text-xs text-slate-500">Monetize your skills with verified campus jobs</p>
                  </div>
                </div>

                <ol className="mt-6 space-y-4">
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      1
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Sign up with institutional email</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Register with your university .edu email and verify your address.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      2
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Build your verified profile & resume</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Add coursework skills, portfolio links, and upload your resume directly to MinIO storage.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      3
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Apply & complete milestone contracts</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Submit tailored proposals, execute work under active contracts, and earn verified ratings.
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
                    <p className="text-xs text-slate-500">Hire motivated talent with zero administrative friction</p>
                  </div>
                </div>

                <ol className="mt-6 space-y-4">
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      1
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Post your gig in minutes</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Define project deliverables, required skills, budget (fixed or hourly), and deadline.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      2
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Review verified student proposals</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Inspect proposals, view authenticated resumes via presigned URLs, and check prior peer reviews.
                      </p>
                    </div>
                  </li>
                  <li className="flex items-start gap-3 text-sm">
                    <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-emerald-100 text-xs font-bold text-emerald-700 dark:bg-emerald-900 dark:text-emerald-300">
                      3
                    </span>
                    <div>
                      <strong className="text-slate-900 dark:text-white">Activate contract & release peer review</strong>
                      <p className="text-xs text-slate-500 dark:text-slate-400 mt-0.5">
                        Accept applicant to start contract, track milestone deliverables, and submit mutual feedback upon completion.
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
            <div className="relative overflow-hidden rounded-[10px] bg-gradient-to-r from-emerald-600 to-emerald-600 px-8 py-12 text-center text-white shadow-xl sm:px-16 sm:py-16">
              <h2 className="text-2xl font-extrabold tracking-tight sm:text-4xl">
                Ready to explore verified campus opportunities?
              </h2>
              <p className="mx-auto mt-4 max-w-xl text-sm sm:text-base text-emerald-100">
                Join verified students and academic departments collaborating across high-impact research, engineering, and creative gigs.
              </p>
              <div className="mt-8 flex flex-wrap justify-center gap-4">
                <Link
                  href="/jobs"
                  className="rounded-[10px] bg-white px-6 py-3 text-sm font-semibold text-emerald-600 shadow-md transition hover:bg-emerald-50"
                >
                  Explore Active Gigs
                </Link>
                <Link
                  href="/login"
                  className="rounded-[10px] border border-white/30 bg-white/10 px-6 py-3 text-sm font-semibold text-white backdrop-blur transition hover:bg-white/20"
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
            &copy; {new Date().getFullYear()} Lynk Marketplace. High-trust university freelancing.
          </p>
          <div className="flex items-center gap-6 text-xs text-slate-500 dark:text-slate-400">
            <Link href="/jobs" className="hover:text-emerald-600">
              Browse Jobs
            </Link>
            <Link href="/login" className="hover:text-emerald-600">
              Keycloak IAM
            </Link>
            <span>PostgreSQL &amp; MinIO Storage</span>
          </div>
        </div>
      </footer>
    </div>
  );
}
