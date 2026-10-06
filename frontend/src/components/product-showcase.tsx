"use client";

import { useRef } from "react";
import Image from "next/image";
import Link from "next/link";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { ShieldCheck, ArrowRight, Star, FileText } from "lucide-react";

export default function ProductShowcase() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const scale = useTransform(
    scrollYProgress,
    [0, 0.5, 1],
    [prefersReduced ? 1 : 0.96, 1, prefersReduced ? 1 : 1.01]
  );
  const imgOpacity = useTransform(scrollYProgress, [0, 0.2, 0.8, 1], [0.6, 1, 1, 0.7]);

  return (
    <section
      ref={ref}
      className="relative w-full bg-canvas overflow-hidden border-t border-line/60"
      aria-labelledby="product-heading"
    >
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-24 md:py-36">
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7 }}
          viewport={{ once: true }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-4"
        >
          The platform
        </motion.p>
        <motion.h2
          id="product-heading"
          initial={prefersReduced ? {} : { opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.9, delay: 0.1 }}
          viewport={{ once: true }}
          className="font-display text-[clamp(32px,4.5vw,60px)] font-normal tracking-[-0.025em] text-ink mb-6 max-w-2xl"
        >
          The product is
          <br />
          the experience.
        </motion.h2>
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 16 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.2 }}
          viewport={{ once: true }}
          className="text-base text-ink-secondary leading-relaxed max-w-lg mb-14"
        >
          Job discovery, verified campus identities, milestone contracts, and double-blind peer
          reviews. Built for real academic work.
        </motion.p>

        {/* Product atmospheric artwork */}
        <motion.div
          style={{ scale, opacity: imgOpacity }}
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

        {/* Real Product UI Components Layered Beneath */}
        <div className="mt-16 grid grid-cols-1 md:grid-cols-3 gap-6 max-w-5xl mx-auto">
          {/* Card 1: Live Opportunity Primitive */}
          <motion.div
            initial={prefersReduced ? {} : { opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.1 }}
            viewport={{ once: true }}
            className="border border-line bg-surface-elevated p-6 flex flex-col justify-between"
          >
            <div>
              <div className="flex items-center justify-between text-[11px] font-mono text-ink-secondary pb-3 border-b border-line/60">
                <span>OPPORTUNITY #8821</span>
                <span className="text-accent-deep font-semibold">CAMPUS RESEARCH</span>
              </div>
              <h3 className="font-display text-xl text-ink mt-4 leading-snug">
                Distributed Consensus Test Harness
              </h3>
              <p className="mt-2 text-xs text-ink-secondary leading-relaxed line-clamp-3">
                Build an automated failure injection harness for evaluating Raft consensus edge cases
                under network partitions.
              </p>
              <div className="mt-4 flex flex-wrap gap-1.5 font-mono text-[10px]">
                <span className="px-2 py-0.5 bg-surface border border-line text-ink-secondary">
                  Go
                </span>
                <span className="px-2 py-0.5 bg-surface border border-line text-ink-secondary">
                  Raft
                </span>
                <span className="px-2 py-0.5 bg-surface border border-line text-ink-secondary">
                  Docker
                </span>
              </div>
            </div>
            <div className="mt-6 pt-4 border-t border-line/60 flex items-center justify-between text-[11px] font-mono text-ink-secondary">
              <span>Systems & Networks Lab</span>
              <span className="text-ink flex items-center gap-1">
                Apply <ArrowRight className="h-3 w-3" />
              </span>
            </div>
          </motion.div>

          {/* Card 2: Contract State Machine Primitive */}
          <motion.div
            initial={prefersReduced ? {} : { opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.2 }}
            viewport={{ once: true }}
            className="border border-line bg-surface-elevated p-6 flex flex-col justify-between"
          >
            <div>
              <div className="flex items-center justify-between text-[11px] font-mono text-ink-secondary pb-3 border-b border-line/60">
                <span>CONTRACT #LNK-409</span>
                <span className="text-accent-deep font-semibold">MILESTONE 2/2</span>
              </div>
              <h3 className="font-display text-xl text-ink mt-4 leading-snug">
                Milestone Deliverable Agreement
              </h3>
              <div className="mt-4 space-y-2">
                <div className="flex items-center justify-between text-xs font-mono">
                  <span className="text-ink-secondary">Draft</span>
                  <span className="text-ink-secondary line-through">Signed</span>
                </div>
                <div className="flex items-center justify-between text-xs font-mono">
                  <span className="text-ink-secondary">Active</span>
                  <span className="text-accent-deep font-medium">In Progress</span>
                </div>
                <div className="flex items-center justify-between text-xs font-mono">
                  <span className="text-ink-secondary">Completed</span>
                  <span className="text-ink-secondary">Pending Sign-off</span>
                </div>
              </div>
            </div>
            <div className="mt-6 pt-4 border-t border-line/60 flex items-center justify-between text-[11px] font-mono text-ink-secondary">
              <span className="flex items-center gap-1">
                <FileText className="h-3 w-3" /> Milestone Deliverable
              </span>
              <span className="text-ink">Mutual Sign-off</span>
            </div>
          </motion.div>

          {/* Card 3: Verified Campus Profile & Peer Score */}
          <motion.div
            initial={prefersReduced ? {} : { opacity: 0, y: 20 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.8, delay: 0.3 }}
            viewport={{ once: true }}
            className="border border-line bg-surface-elevated p-6 flex flex-col justify-between"
          >
            <div>
              <div className="flex items-center justify-between text-[11px] font-mono text-ink-secondary pb-3 border-b border-line/60">
                <span className="flex items-center gap-1">
                  <ShieldCheck className="h-3.5 w-3.5 text-accent-deep" />
                  VERIFIED .EDU
                </span>
                <span>STANFORD</span>
              </div>
              <h3 className="font-display text-xl text-ink mt-4 leading-snug">
                Student Profile & Ledger
              </h3>
              <p className="mt-2 text-xs text-ink-secondary leading-relaxed">
                Computer Science & Bioengineering // Class of 2026. 6 completed contracts with
                verified faculty laboratories.
              </p>
              <div className="mt-4 p-3 bg-surface border border-line text-xs font-mono space-y-1">
                <div className="flex items-center justify-between text-ink font-semibold">
                  <span className="flex items-center gap-1">
                    <Star className="h-3 w-3 fill-accent-warm text-accent-warm" /> 5.0 Rating
                  </span>
                  <span className="text-ink-secondary text-[10px]">100% ON TIME</span>
                </div>
                <p className="text-[11px] text-ink-secondary italic pt-1">
                  &quot;Production-grade delivery on schedule.&quot;
                </p>
              </div>
            </div>
            <div className="mt-6 pt-4 border-t border-line/60 flex items-center justify-between text-[11px] font-mono text-ink-secondary">
              <span>Resume in MinIO</span>
              <span className="text-ink">View Profile</span>
            </div>
          </motion.div>
        </div>

        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, delay: 0.4 }}
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
