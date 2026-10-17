"use client";

import { useRef, useState, useEffect } from "react";
import Image from "next/image";
import Link from "next/link";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { ArrowRight, PlusCircle, Compass, Sparkles } from "lucide-react";
import { EDITORIAL_EASE } from "@/lib/animations";
import { listJobs } from "@/lib/api";
import type { Job } from "@/types/api";

export default function ProductShowcase() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgOpacity = useTransform(scrollYProgress, [0, 0.2, 0.8, 1], [0.6, 1, 1, 0.7]);

  const [jobs, setJobs] = useState<Job[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);

  useEffect(() => {
    let isMounted = true;
    listJobs({ limit: 3, status: "open" })
      .then((data) => {
        if (isMounted && Array.isArray(data)) {
          setJobs(data);
        }
      })
      .catch((err) => {
        // Non-blocking fallback if backend is booting
        console.error("Failed to load showcase opportunities:", err);
      })
      .finally(() => {
        if (isMounted) {
          setIsLoading(false);
        }
      });

    return () => {
      isMounted = false;
    };
  }, []);

  return (
    <section
      ref={ref}
      className="relative w-full bg-canvas overflow-hidden border-t border-line/60"
      aria-labelledby="product-heading"
    >
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-24 md:py-36">
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-4"
        >
          The platform
        </motion.p>
        <motion.h2
          id="product-heading"
          initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="font-display text-[clamp(32px,4.5vw,60px)] font-normal leading-[1.1] text-ink mb-6 max-w-4xl"
        >
          The product is the experience.
        </motion.h2>
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.16, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="text-base text-ink-secondary leading-relaxed max-w-2xl mb-14"
        >
          Job discovery, verified campus identities, milestone contracts, and double-blind peer
          reviews. Built for real academic work.
        </motion.p>

        {/* Product atmospheric artwork */}
        <motion.div
          style={{ opacity: imgOpacity }}
          className="relative w-full aspect-[16/9] max-w-5xl mx-auto rounded-sm overflow-hidden border border-line"
          aria-hidden="true"
        >
          <Image
            src="/product.png"
            alt="Abstract representation of the Lynk information architecture"
            fill
            className="object-cover"
            sizes="(max-width: 1024px) 100vw, 80vw"
          />
          <div
            className="absolute inset-0"
            style={{
              background:
                "radial-gradient(ellipse at center, transparent 60%, rgba(244,241,234,0.7) 100%)",
            }}
          />
        </motion.div>

        {/* Dynamic Database Opportunities Layered Beneath */}
        <div className="mt-16 grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl mx-auto">
          {isLoading ? (
            // Skeleton Loading State
            Array.from({ length: 3 }).map((_, idx) => (
              <div
                key={`skeleton-${idx}`}
                className="border border-line bg-surface-elevated p-6 flex flex-col justify-between animate-pulse"
                style={{ minHeight: "260px" }}
              >
                <div>
                  <div className="flex items-center justify-between pb-3 border-b border-line/60">
                    <div className="h-3 w-24 bg-line/60 rounded" />
                    <div className="h-3 w-16 bg-line/60 rounded" />
                  </div>
                  <div className="h-5 w-3/4 bg-line/60 rounded mt-4" />
                  <div className="h-3 w-full bg-line/40 rounded mt-3" />
                  <div className="h-3 w-2/3 bg-line/40 rounded mt-1.5" />
                  <div className="flex gap-2 mt-4">
                    <div className="h-4 w-12 bg-line/50 rounded" />
                    <div className="h-4 w-12 bg-line/50 rounded" />
                  </div>
                </div>
                <div className="pt-4 border-t border-line/60 flex items-center justify-between">
                  <div className="h-3 w-20 bg-line/50 rounded" />
                  <div className="h-3 w-12 bg-line/50 rounded" />
                </div>
              </div>
            ))
          ) : jobs.length > 0 ? (
            // Live Opportunities from PostgreSQL Database
            jobs.map((job, index) => {
              const opportunityNumber = job.id.replace(/-/g, "").slice(0, 6).toUpperCase();
              const creatorLabel =
                job.creator?.organization ||
                (job.creator?.first_name
                  ? `${job.creator.first_name} ${job.creator.last_name || ""}`.trim()
                  : job.department || "Campus Lab");

              return (
                <motion.div
                  key={job.id}
                  initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
                  whileInView={{ opacity: 1, y: 0 }}
                  transition={{ duration: 0.45, delay: index * 0.06, ease: EDITORIAL_EASE }}
                  viewport={{ once: true }}
                  className="border border-line bg-surface-elevated p-6 flex flex-col justify-between hover:border-ink/40 transition-colors"
                >
                  <div>
                    <div className="flex items-center justify-between text-[11px] font-mono text-ink-secondary pb-3 border-b border-line/60">
                      <span>OPPORTUNITY #{opportunityNumber}</span>
                      <span className="text-accent-deep font-semibold uppercase">
                        {job.department || "CAMPUS"}
                      </span>
                    </div>
                    <Link href={`/jobs/${job.id}`}>
                      <h3 className="font-display text-xl text-ink mt-4 leading-snug hover:text-ink/80 transition-colors line-clamp-2">
                        {job.title}
                      </h3>
                    </Link>
                    <p className="mt-2 text-xs text-ink-secondary leading-relaxed line-clamp-3">
                      {job.description}
                    </p>
                    {job.required_skills && job.required_skills.length > 0 && (
                      <div className="mt-4 flex flex-wrap gap-1.5 font-mono text-[10px]">
                        {job.required_skills.slice(0, 3).map((skill) => (
                          <span
                            key={skill}
                            className="px-2 py-0.5 bg-surface border border-line text-ink-secondary"
                          >
                            {skill}
                          </span>
                        ))}
                      </div>
                    )}
                  </div>
                  <div className="mt-6 pt-4 border-t border-line/60 flex items-center justify-between text-[11px] font-mono text-ink-secondary">
                    <span className="truncate max-w-[150px]">{creatorLabel}</span>
                    <Link
                      href={`/jobs/${job.id}`}
                      className="text-ink flex items-center gap-1 hover:text-accent-deep transition-colors"
                    >
                      Apply <ArrowRight className="h-3 w-3" />
                    </Link>
                  </div>
                </motion.div>
              );
            })
          ) : (
            // Clean Fallback / Empty State when database has no open postings
            <>
              <motion.div
                initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.45, delay: 0.06, ease: EDITORIAL_EASE }}
                viewport={{ once: true }}
                className="border border-line bg-surface-elevated p-6 flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-center justify-between text-[11px] font-mono text-ink-secondary pb-3 border-b border-line/60">
                    <span>MARKETPLACE</span>
                    <span className="text-accent-deep font-semibold">CAMPUS DIRECT</span>
                  </div>
                  <h3 className="font-display text-xl text-ink mt-4 leading-snug">
                    Browse All Opportunities
                  </h3>
                  <p className="mt-2 text-xs text-ink-secondary leading-relaxed">
                    Filter open campus research gigs, software contracts, and design projects by
                    academic department.
                  </p>
                </div>
                <div className="mt-6 pt-4 border-t border-line/60 flex items-center justify-between text-[11px] font-mono text-ink-secondary">
                  <span className="flex items-center gap-1">
                    <Compass className="h-3 w-3" /> Open Marketplace
                  </span>
                  <Link href="/jobs" className="text-ink flex items-center gap-1">
                    Explore <ArrowRight className="h-3 w-3" />
                  </Link>
                </div>
              </motion.div>

              <motion.div
                initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.45, delay: 0.12, ease: EDITORIAL_EASE }}
                viewport={{ once: true }}
                className="border border-line bg-surface-elevated p-6 flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-center justify-between text-[11px] font-mono text-ink-secondary pb-3 border-b border-line/60">
                    <span>CREATION</span>
                    <span className="text-accent-deep font-semibold">FACULTY & PEERS</span>
                  </div>
                  <h3 className="font-display text-xl text-ink mt-4 leading-snug">
                    Post an Opportunity
                  </h3>
                  <p className="mt-2 text-xs text-ink-secondary leading-relaxed">
                    Looking for student collaborators or lab assistants? Define deliverables and
                    receive proposals instantly.
                  </p>
                </div>
                <div className="mt-6 pt-4 border-t border-line/60 flex items-center justify-between text-[11px] font-mono text-ink-secondary">
                  <span className="flex items-center gap-1">
                    <PlusCircle className="h-3 w-3" /> Create Posting
                  </span>
                  <Link href="/jobs/create" className="text-ink flex items-center gap-1">
                    Post <ArrowRight className="h-3 w-3" />
                  </Link>
                </div>
              </motion.div>

              <motion.div
                initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.45, delay: 0.18, ease: EDITORIAL_EASE }}
                viewport={{ once: true }}
                className="border border-line bg-surface-elevated p-6 flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-center justify-between text-[11px] font-mono text-ink-secondary pb-3 border-b border-line/60">
                    <span>REPUTATION</span>
                    <span className="text-accent-deep font-semibold">VERIFIED .EDU</span>
                  </div>
                  <h3 className="font-display text-xl text-ink mt-4 leading-snug">
                    Verified Campus Identity
                  </h3>
                  <p className="mt-2 text-xs text-ink-secondary leading-relaxed">
                    Build a tamper-proof portfolio of peer deliverables, lab contracts, and
                    institutional credentials.
                  </p>
                </div>
                <div className="mt-6 pt-4 border-t border-line/60 flex items-center justify-between text-[11px] font-mono text-ink-secondary">
                  <span className="flex items-center gap-1">
                    <Sparkles className="h-3 w-3" /> Peer Reviews
                  </span>
                  <Link href="/login" className="text-ink flex items-center gap-1">
                    Sign in <ArrowRight className="h-3 w-3" />
                  </Link>
                </div>
              </motion.div>
            </>
          )}
        </div>

        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, delay: 0.24, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="mt-14 flex flex-wrap gap-6 justify-center"
        >
          <Link
            href="/jobs"
            className="inline-block text-sm text-ink border-b border-ink pb-0.5 hover:text-ink-secondary hover:border-ink-secondary transition-colors tracking-wide"
          >
            Browse campus opportunities
          </Link>
          <Link
            href="/login"
            className="inline-block text-sm text-ink-secondary border-b border-ink-secondary pb-0.5 hover:text-ink hover:border-ink transition-colors tracking-wide"
          >
            Create student profile
          </Link>
        </motion.div>
      </div>
    </section>
  );
}
