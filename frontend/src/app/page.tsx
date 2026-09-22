"use client";

import { useEffect, useState, useRef } from "react";
import Link from "next/link";
import { ArrowRight, Briefcase, Calendar, Database, ShieldCheck, Star } from "lucide-react";
import gsap from "gsap";
import { Job } from "@/types/api";
import { listJobs } from "@/lib/api";
import { animateHero } from "@/lib/animations";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ShaderAuroraVeil } from "@/components/ui/shader-aurora-veil";
import { AeroCounter } from "@/components/ui/aero-hero-3";

interface CuratedProject {
  id: string;
  title: string;
  department: string;
  budget: string;
  deadline: string;
  creator: string;
  skills: string[];
  description: string;
  link: string;
}

const CURATED_PROJECTS: CuratedProject[] = [
  {
    id: "cs-distributed-kv",
    title: "Distributed Key-Value Store Testing Harness",
    department: "Computer Science",
    budget: "$1,400 Deliverable",
    deadline: "Nov 15, 2026",
    creator: "Systems & Networking Lab",
    skills: ["Go", "Distributed Systems", "Docker"],
    description:
      "Build an automated failure injection harness for evaluating Raft consensus edge cases under network partitions.",
    link: "/jobs",
  },
  {
    id: "bio-microfluidics-signal",
    title: "Microfluidics Sensor Signal Processing Pipeline",
    department: "Bioengineering",
    budget: "$48/hr",
    deadline: "Nov 30, 2026",
    creator: "Biomedical Instrumentation Lab",
    skills: ["Python", "NumPy", "Signal Processing"],
    description:
      "Develop real-time noise reduction algorithms for low-voltage sensor readings during cell isolation assays.",
    link: "/jobs",
  },
  {
    id: "arch-parametric-pavilion",
    title: "Parametric Pavilion Structural Analysis Model",
    department: "Architecture",
    budget: "$950 Deliverable",
    deadline: "Dec 05, 2026",
    creator: "Digital Fabrication Studio",
    skills: ["Rhino", "Grasshopper", "Finite Element"],
    description:
      "Construct computational structural load models for a lightweight timber campus pavilion installation.",
    link: "/jobs",
  },
  {
    id: "robo-autonomous-odometry",
    title: "Autonomous Rover Localization & Odometry",
    department: "Robotics",
    budget: "$52/hr",
    deadline: "Dec 12, 2026",
    creator: "Robotics Research Group",
    skills: ["C++", "ROS2", "EKF"],
    description:
      "Implement extended Kalman filter odometry fusing wheel encoders and IMU telemetry for campus navigation trials.",
    link: "/jobs",
  },
];

function formatJobDate(dateStr?: string | null): string {
  if (!dateStr) return "Flexible";
  try {
    const d = new Date(dateStr);
    if (isNaN(d.getTime())) return dateStr;
    return d.toLocaleDateString("en-US", {
      month: "short",
      day: "numeric",
      year: "numeric",
    });
  } catch {
    return dateStr;
  }
}

function getJobBudgetDisplay(job: Job): string {
  if (job.department === "Computer Science") return "$45/hr";
  if (job.department === "Bioengineering") return "$48/hr";
  if (job.department === "Architecture") return "$950 Deliverable";
  if (job.department === "Robotics") return "$52/hr";
  return "$40/hr";
}

export default function HomePage() {
  const heroRef = useRef<HTMLDivElement | null>(null);
  const bentoRef = useRef<HTMLDivElement | null>(null);
  const streamRef = useRef<HTMLDivElement | null>(null);
  const [featuredJobs, setFeaturedJobs] = useState<Job[]>([]);
  const [isLoadingJobs, setIsLoadingJobs] = useState(true);

  useEffect(() => {
    let isMounted = true;
    async function loadFeatured() {
      try {
        const jobs = await listJobs({ limit: 4, status: "open" });
        if (isMounted && jobs) {
          setFeaturedJobs(jobs);
        }
      } catch {
        // Backend unavailable or empty; fallback data renders
      } finally {
        if (isMounted) {
          setIsLoadingJobs(false);
        }
      }
    }
    loadFeatured();
    return () => {
      isMounted = false;
    };
  }, []);

  // GSAP hero entry animation on mount
  useEffect(() => {
    if (heroRef.current) {
      return animateHero(heroRef.current);
    }
  }, []);

  // GSAP scroll entry animations with IntersectionObserver
  useEffect(() => {
    if (typeof window === "undefined") return;
    const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    if (reducedMotion) return;

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            const target = entry.target as HTMLElement;
            const items = target.querySelectorAll("[data-animate-item]");
            if (items.length > 0) {
              gsap.fromTo(
                items,
                { opacity: 0, y: 20 },
                {
                  opacity: 1,
                  y: 0,
                  duration: 0.6,
                  stagger: 0.1,
                  ease: "power2.out",
                  clearProps: "transform",
                }
              );
            } else {
              gsap.fromTo(
                target,
                { opacity: 0, y: 20 },
                {
                  opacity: 1,
                  y: 0,
                  duration: 0.6,
                  ease: "power2.out",
                  clearProps: "transform",
                }
              );
            }
            observer.unobserve(target);
          }
        });
      },
      { threshold: 0.1 }
    );

    const sections = document.querySelectorAll("[data-scroll-section]");
    sections.forEach((sec) => observer.observe(sec));

    return () => {
      observer.disconnect();
    };
  }, [isLoadingJobs, featuredJobs]);

  return (
    <div className="flex min-h-screen flex-col bg-background text-foreground selection:bg-foreground selection:text-background">
      <main className="flex-1">
        {/* Hero Section */}
        <section
          ref={heroRef}
          className="relative overflow-hidden pt-20 pb-24 md:pt-28 md:pb-32 border-b border-border"
        >
          {/* Subtle Ambient Veil Backdrop */}
          <ShaderAuroraVeil
            preset="aurora-veil"
            className="absolute inset-0 pointer-events-none opacity-25 dark:opacity-35"
          />

          <div className="relative z-10 mx-auto max-w-5xl px-4 sm:px-6 lg:px-8 text-center flex flex-col items-center">
            {/* Eyebrow / Tag */}
            <div data-animate-hero>
              <Badge variant="outline" className="mb-6 font-mono text-xs tracking-wider">
                ACADEMIC YEAR 2026 // VERIFIED CAMPUS TALENT
              </Badge>
            </div>

            {/* Editorial Serif Heading */}
            <h1
              data-animate-hero
              className="font-serif text-5xl sm:text-6xl lg:text-7xl font-normal tracking-tight text-foreground max-w-4xl leading-[1.05]"
            >
              The high-trust campus gig and freelance exchange.
            </h1>

            {/* Subtitle */}
            <p
              data-animate-hero
              className="text-lg sm:text-xl text-muted-foreground max-w-2xl mt-6 leading-relaxed font-sans"
            >
              Directly connect with verified university students for engineering, design, and
              research deliverables. Protected by milestone contracts and peer reviews.
            </p>

            {/* Call to Actions */}
            <div
              data-animate-hero
              className="mt-10 flex flex-wrap items-center justify-center gap-4"
            >
              <Link href="/jobs">
                <Button
                  size="lg"
                  className="rounded-md px-6 text-sm font-medium bg-[#111111] text-white hover:bg-[#222222] dark:bg-foreground dark:text-background dark:hover:bg-foreground/90 shadow-none"
                >
                  Browse Open Gigs
                </Button>
              </Link>
              <Link href="/jobs/create">
                <Button variant="outline" size="lg" className="rounded-md px-6 text-sm font-medium">
                  Post a Project
                </Button>
              </Link>
            </div>

            {/* Micro-metric strip */}
            <div
              data-animate-hero
              className="mt-12 flex flex-wrap items-center justify-center gap-3"
            >
              <AeroCounter label="Open Campus Gigs" initialCount={128} />
              <Badge variant="default" className="font-mono text-xs py-1.5 px-3 rounded-md">
                VERIFIED .EDU TALENT
              </Badge>
              <Badge
                variant="outline"
                className="font-mono text-xs py-1.5 px-3 rounded-md text-muted-foreground"
              >
                ZERO ESCROW CUT
              </Badge>
            </div>
          </div>
        </section>

        {/* Bento Box Feature Grid */}
        <section
          ref={bentoRef}
          data-scroll-section
          className="py-24 border-t border-border bg-background relative"
        >
          <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8">
            <div className="max-w-2xl">
              <Badge variant="outline" className="font-mono text-xs tracking-widest mb-3">
                SYSTEM ARCHITECTURE
              </Badge>
              <h2 className="font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
                Built strictly for academic institutions.
              </h2>
              <p className="mt-3 text-base text-muted-foreground leading-relaxed">
                Every primitive is engineered to eliminate platform spam, credential
                misrepresentation, and deliverable ambiguity.
              </p>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-12 gap-6 mt-12">
              {/* Card 1: Institutional .edu Verification Gate */}
              <div
                data-animate-item
                className="md:col-span-7 rounded-xl border border-[#EAEAEA] dark:border-border bg-card p-6 md:p-8 flex flex-col justify-between transition-all duration-200 hover:border-foreground/30 shadow-none"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-muted-foreground">
                      01 // IAM IDENTITY GATE
                    </span>
                    <Badge variant="default">VERIFIED DOMAINS</Badge>
                  </div>
                  <h3 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground mt-4">
                    Zero spam. Guaranteed student identity.
                  </h3>
                  <p className="mt-3 text-sm sm:text-base text-muted-foreground leading-relaxed">
                    Strict mandatory university email validation ensures all contractors and posters
                    belong to verified university domains.
                  </p>
                </div>

                <div className="mt-6 rounded-lg border border-border/80 bg-muted/40 p-4 font-mono text-xs text-muted-foreground space-y-2">
                  <div className="flex items-center justify-between text-foreground font-medium">
                    <div className="flex items-center gap-2">
                      <ShieldCheck className="h-4 w-4 text-pastel-greenText" />
                      <span>GATEWAY: SuperTokens Institutional IAM</span>
                    </div>
                    <span className="text-pastel-greenText bg-pastel-green px-2 py-0.5 rounded text-[10px] font-semibold">
                      ACTIVE
                    </span>
                  </div>
                  <div className="text-[11px] text-muted-foreground flex flex-wrap items-center gap-1.5 pt-1">
                    <span className="bg-background px-2 py-0.5 rounded border border-border">
                      @stanford.edu
                    </span>
                    <span className="bg-background px-2 py-0.5 rounded border border-border">
                      @mit.edu
                    </span>
                    <span className="bg-background px-2 py-0.5 rounded border border-border">
                      @berkeley.edu
                    </span>
                    <span className="bg-background px-2 py-0.5 rounded border border-border">
                      @harvard.edu
                    </span>
                    <span className="text-[10px] text-muted-foreground ml-1">
                      + All Verified .edu
                    </span>
                  </div>
                </div>
              </div>

              {/* Card 2: Direct-to-MinIO Resume Vault */}
              <div
                data-animate-item
                className="md:col-span-5 rounded-xl border border-[#EAEAEA] dark:border-border bg-card p-6 md:p-8 flex flex-col justify-between transition-all duration-200 hover:border-foreground/30 shadow-none"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-muted-foreground">
                      02 // OBJECT STORAGE
                    </span>
                    <Badge variant="info">DIRECT S3</Badge>
                  </div>
                  <h3 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground mt-4">
                    Presigned direct object storage.
                  </h3>
                  <p className="mt-3 text-sm sm:text-base text-muted-foreground leading-relaxed">
                    Resumes stream directly to S3-compatible MinIO object storage with temporary
                    presigned URLs.
                  </p>
                </div>

                <div className="mt-6 rounded-lg border border-border/80 bg-muted/40 p-4 font-mono text-xs space-y-2">
                  <div className="flex items-center justify-between text-foreground font-medium">
                    <div className="flex items-center gap-2">
                      <Database className="h-4 w-4 text-pastel-blueText" />
                      <span>BUCKET: s3://resumes</span>
                    </div>
                    <span className="text-pastel-blueText bg-pastel-blue px-2 py-0.5 rounded text-[10px] font-semibold">
                      PRESIGNED
                    </span>
                  </div>
                  <div className="text-[11px] text-muted-foreground truncate">
                    METHOD: Stream direct to S3 // Ephemeral 300s TTL
                  </div>
                </div>
              </div>

              {/* Card 3: Milestone Deliverable Contracts */}
              <div
                data-animate-item
                className="md:col-span-5 rounded-xl border border-[#EAEAEA] dark:border-border bg-card p-6 md:p-8 flex flex-col justify-between transition-all duration-200 hover:border-foreground/30 shadow-none"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-muted-foreground">
                      03 // ESCROW &amp; STATE
                    </span>
                    <Badge variant="warning">STATE MACHINE</Badge>
                  </div>
                  <h3 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground mt-4">
                    Automated contract state machine.
                  </h3>
                  <p className="mt-3 text-sm sm:text-base text-muted-foreground leading-relaxed">
                    Transparent lifecycle transitions from Draft to Active to Completed upon mutual
                    delivery sign-off.
                  </p>
                </div>

                <div className="mt-6 rounded-lg border border-border/80 bg-muted/40 p-4 font-mono text-xs">
                  <div className="flex items-center justify-between gap-1 text-[11px]">
                    <span className="px-2 py-1 rounded bg-background border border-border text-muted-foreground">
                      Draft
                    </span>
                    <span className="text-muted-foreground">-&gt;</span>
                    <span className="px-2 py-1 rounded bg-pastel-yellow text-pastel-yellowText font-medium">
                      Active
                    </span>
                    <span className="text-muted-foreground">-&gt;</span>
                    <span className="px-2 py-1 rounded bg-pastel-green text-pastel-greenText font-medium">
                      Completed
                    </span>
                  </div>
                </div>
              </div>

              {/* Card 4: Verified Peer Review Ledger */}
              <div
                data-animate-item
                className="md:col-span-7 rounded-xl border border-[#EAEAEA] dark:border-border bg-card p-6 md:p-8 flex flex-col justify-between transition-all duration-200 hover:border-foreground/30 shadow-none"
              >
                <div>
                  <div className="flex items-center justify-between gap-2">
                    <span className="font-mono text-xs text-muted-foreground">
                      04 // REPUTATION PROTOCOL
                    </span>
                    <Badge variant="secondary">PEER SCORES</Badge>
                  </div>
                  <h3 className="font-serif text-2xl sm:text-3xl font-normal tracking-tight text-foreground mt-4">
                    Authentic campus reputation.
                  </h3>
                  <p className="mt-3 text-sm sm:text-base text-muted-foreground leading-relaxed">
                    Double-blind 1-5 star ratings and reviews unlocked only after contract
                    completion.
                  </p>
                </div>

                <div className="mt-6 rounded-lg border border-border/80 bg-muted/40 p-4 font-mono text-xs space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2 text-foreground font-medium">
                      <Star className="h-4 w-4 fill-amber-400 text-amber-500" />
                      <span>DOUBLE-BLIND ESCROW VERIFICATION</span>
                    </div>
                    <span className="text-muted-foreground text-[10px]">CONTRACT #LNK-8821</span>
                  </div>
                  <div className="text-[11px] text-muted-foreground truncate">
                    <span className="text-foreground font-bold mr-2">5.0 / 5.0</span>
                    <span>
                      &quot;Delivered clean Go microservice on schedule with 100% test
                      coverage.&quot;
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </section>

        {/* Live Campus Project Stream */}
        <section
          ref={streamRef}
          data-scroll-section
          className="py-24 border-t border-border bg-muted/20"
        >
          <div className="mx-auto max-w-6xl px-4 sm:px-6 lg:px-8">
            <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
              <div>
                <Badge variant="outline" className="font-mono text-xs tracking-widest mb-3">
                  DISPATCH FEED
                </Badge>
                <h2 className="font-serif text-3xl sm:text-4xl font-normal tracking-tight text-foreground">
                  Live Campus Project Stream
                </h2>
                <p className="mt-2 text-sm text-muted-foreground max-w-xl leading-relaxed">
                  Active engineering, research, and design deliverables open for verified student
                  applications.
                </p>
              </div>

              <Link
                href="/jobs"
                className="inline-flex items-center gap-1.5 text-sm font-medium text-foreground hover:text-muted-foreground transition-colors"
              >
                <span>View all open gigs</span>
                <ArrowRight className="h-4 w-4" />
              </Link>
            </div>

            {/* Grid of Project Cards */}
            {isLoadingJobs ? (
              <div className="mt-12 grid grid-cols-1 md:grid-cols-2 gap-6">
                {[1, 2, 3, 4].map((n) => (
                  <div
                    key={n}
                    className="h-48 animate-pulse rounded-xl border border-border bg-card p-6"
                  />
                ))}
              </div>
            ) : featuredJobs.length > 0 ? (
              <div className="mt-12 grid grid-cols-1 md:grid-cols-2 gap-6">
                {featuredJobs.map((job) => {
                  const poster =
                    job.creator?.first_name || job.creator?.last_name
                      ? `${job.creator.first_name || ""} ${job.creator.last_name || ""}`.trim()
                      : job.creator?.organization || "Campus Member";
                  const skills = job.required_skills ?? [];

                  return (
                    <article
                      key={job.id}
                      data-animate-item
                      className="group rounded-xl border border-border bg-card p-6 flex flex-col justify-between transition-all duration-200 hover:border-foreground/30 shadow-none"
                    >
                      <div>
                        <div className="flex items-center justify-between gap-2 pb-3">
                          <Badge variant="outline" className="font-mono text-xs">
                            {job.department || "General"}
                          </Badge>
                          <span className="font-mono text-xs font-semibold text-foreground bg-muted px-2.5 py-1 rounded">
                            {getJobBudgetDisplay(job)}
                          </span>
                        </div>

                        <Link href={`/jobs/${job.id}`}>
                          <h3 className="font-serif text-xl font-medium tracking-tight text-foreground mt-2 group-hover:underline line-clamp-1">
                            {job.title}
                          </h3>
                        </Link>

                        <div className="mt-1 flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
                          <Briefcase className="h-3.5 w-3.5" />
                          <span className="truncate">{poster}</span>
                        </div>

                        <p className="mt-3 text-xs sm:text-sm text-muted-foreground leading-relaxed line-clamp-2">
                          {job.description}
                        </p>

                        {skills.length > 0 && (
                          <div className="mt-4 flex flex-wrap gap-1.5">
                            {skills.slice(0, 3).map((skill, idx) => (
                              <span
                                key={`${skill}-${idx}`}
                                className="font-mono text-[11px] px-2 py-0.5 rounded bg-muted text-muted-foreground border border-border"
                              >
                                {skill}
                              </span>
                            ))}
                            {skills.length > 3 && (
                              <span className="font-mono text-[11px] px-2 py-0.5 rounded bg-muted text-muted-foreground border border-border">
                                +{skills.length - 3} more
                              </span>
                            )}
                          </div>
                        )}
                      </div>

                      <div className="mt-6 flex items-center justify-between border-t border-border/80 pt-4">
                        <span className="font-mono text-xs text-muted-foreground flex items-center gap-1.5">
                          <Calendar className="h-3.5 w-3.5" />
                          Due {formatJobDate(job.deadline)}
                        </span>
                        <Link
                          href={`/jobs/${job.id}`}
                          className="inline-flex items-center gap-1 font-mono text-xs font-medium text-foreground hover:text-muted-foreground transition-colors"
                        >
                          <span>Inspect Gig</span>
                          <ArrowRight className="h-3 w-3 transition-transform group-hover:translate-x-0.5" />
                        </Link>
                      </div>
                    </article>
                  );
                })}
              </div>
            ) : (
              <div className="mt-12 grid grid-cols-1 md:grid-cols-2 gap-6">
                {CURATED_PROJECTS.map((project) => (
                  <article
                    key={project.id}
                    data-animate-item
                    className="group rounded-xl border border-border bg-card p-6 flex flex-col justify-between transition-all duration-200 hover:border-foreground/30 shadow-none"
                  >
                    <div>
                      <div className="flex items-center justify-between gap-2 pb-3">
                        <Badge variant="outline" className="font-mono text-xs">
                          {project.department}
                        </Badge>
                        <span className="font-mono text-xs font-semibold text-foreground bg-muted px-2.5 py-1 rounded">
                          {project.budget}
                        </span>
                      </div>

                      <Link href={project.link}>
                        <h3 className="font-serif text-xl font-medium tracking-tight text-foreground mt-2 group-hover:underline line-clamp-1">
                          {project.title}
                        </h3>
                      </Link>

                      <div className="mt-1 flex items-center gap-1.5 font-mono text-xs text-muted-foreground">
                        <Briefcase className="h-3.5 w-3.5" />
                        <span className="truncate">{project.creator}</span>
                      </div>

                      <p className="mt-3 text-xs sm:text-sm text-muted-foreground leading-relaxed line-clamp-2">
                        {project.description}
                      </p>

                      <div className="mt-4 flex flex-wrap gap-1.5">
                        {project.skills.map((skill, idx) => (
                          <span
                            key={`${skill}-${idx}`}
                            className="font-mono text-[11px] px-2 py-0.5 rounded bg-muted text-muted-foreground border border-border"
                          >
                            {skill}
                          </span>
                        ))}
                      </div>
                    </div>

                    <div className="mt-6 flex items-center justify-between border-t border-border/80 pt-4">
                      <span className="font-mono text-xs text-muted-foreground flex items-center gap-1.5">
                        <Calendar className="h-3.5 w-3.5" />
                        Due {project.deadline}
                      </span>
                      <Link
                        href={project.link}
                        className="inline-flex items-center gap-1 font-mono text-xs font-medium text-foreground hover:text-muted-foreground transition-colors"
                      >
                        <span>Inspect Gig</span>
                        <ArrowRight className="h-3 w-3 transition-transform group-hover:translate-x-0.5" />
                      </Link>
                    </div>
                  </article>
                ))}
              </div>
            )}
          </div>
        </section>

        {/* Manifesto / Core Thesis Strip */}
        <section
          data-scroll-section
          className="py-20 md:py-24 border-t border-border bg-[#F7F6F3] dark:bg-[#121214]"
        >
          <div className="mx-auto max-w-4xl px-4 sm:px-6 lg:px-8 text-center" data-animate-item>
            <span className="font-mono text-xs uppercase tracking-widest text-muted-foreground">
              THE LYNK THESIS
            </span>
            <blockquote className="font-serif text-2xl sm:text-3xl md:text-4xl font-normal tracking-tight text-foreground mt-6 leading-snug">
              &quot;Commercial freelance marketplaces extract 20% platform fees while reducing
              students to commoditized labor. Lynk connects university talent directly with campus
              teams, protected by verified .edu identity, structured milestone contracts, and zero
              platform take-rate.&quot;
            </blockquote>
            <div className="mt-8 flex flex-wrap items-center justify-center gap-6 font-mono text-xs text-muted-foreground">
              <span>{"//"} 0% PLATFORM TAKE-RATE</span>
              <span>{"//"} INSTITUTIONAL DOMAIN GATE</span>
              <span>{"//"} ESCROW MILESTONE CONTRACTS</span>
            </div>
          </div>
        </section>

        {/* Bottom Call-to-Action Block */}
        <section
          data-scroll-section
          className="py-24 border-t border-border bg-background text-center relative overflow-hidden"
        >
          <div className="mx-auto max-w-3xl px-4 sm:px-6 lg:px-8" data-animate-item>
            <span className="font-mono text-xs uppercase tracking-widest text-muted-foreground">
              CAMPUS GIG EXCHANGE
            </span>
            <h2 className="font-serif text-3xl sm:text-4xl md:text-5xl font-normal tracking-tight text-foreground mt-3">
              Start building on your campus today.
            </h2>
            <p className="mt-4 text-base sm:text-lg text-muted-foreground max-w-xl mx-auto leading-relaxed">
              Connect with verified student talent across engineering, research, and design. No
              commissions, no unverified accounts.
            </p>
            <div className="mt-8 flex flex-wrap items-center justify-center gap-4">
              <Link href="/jobs">
                <Button
                  size="lg"
                  className="rounded-md px-6 text-sm font-medium bg-[#111111] text-white hover:bg-[#222222] dark:bg-foreground dark:text-background dark:hover:bg-foreground/90 shadow-none"
                >
                  Browse Open Gigs
                </Button>
              </Link>
              <Link href="/jobs/create">
                <Button variant="outline" size="lg" className="rounded-md px-6 text-sm font-medium">
                  Post a Project
                </Button>
              </Link>
              <Link href="/login">
                <Button variant="ghost" size="lg" className="rounded-md px-6 text-sm font-medium">
                  Sign In with .edu
                </Button>
              </Link>
            </div>
          </div>
        </section>
      </main>

      {/* Minimalist Footer */}
      <footer className="border-t border-border bg-background py-8">
        <div className="mx-auto flex max-w-6xl flex-col items-center justify-between gap-4 px-4 sm:flex-row sm:px-6 lg:px-8">
          <div className="flex items-center gap-3">
            <span className="font-serif text-base font-medium tracking-tight text-foreground">
              LYNK
            </span>
            <span className="font-mono text-xs text-muted-foreground">
              {"//"} ACADEMIC YEAR 2026
            </span>
          </div>
          <p className="font-mono text-xs text-muted-foreground">
            (c) 2026 Lynk. High-trust campus gig and freelance exchange.
          </p>
          <div className="flex items-center gap-6 font-mono text-xs text-muted-foreground">
            <Link href="/jobs" className="hover:text-foreground transition-colors">
              Gigs
            </Link>
            <Link href="/contracts" className="hover:text-foreground transition-colors">
              Contracts
            </Link>
            <Link href="/login" className="hover:text-foreground transition-colors">
              Sign In
            </Link>
          </div>
        </div>
      </footer>
    </div>
  );
}
