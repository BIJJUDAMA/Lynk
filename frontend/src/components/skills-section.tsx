"use client";

import Link from "next/link";
import { motion, useReducedMotion } from "framer-motion";
import { ArrowUpRight } from "lucide-react";
import { EDITORIAL_EASE } from "@/lib/animations";

const DISCIPLINES = [
  { name: "Design", note: "Visual, Product, Motion", query: "Design" },
  { name: "Engineering", note: "Systems, Backend, Infra", query: "Engineering" },
  { name: "Frontend", note: "React, Next.js, Web", query: "React" },
  { name: "Data & ML", note: "Python, Pipelines, Research", query: "Python" },
  { name: "AI Systems", note: "LLMs, Inference, Agents", query: "AI" },
  { name: "Lab Research", note: "Academic, Studies, Analysis", query: "Research" },
  { name: "Product", note: "Strategy, Roadmap, Launch", query: "Product" },
  { name: "Writing", note: "Technical, Editorial, Grants", query: "Writing" },
];

export default function SkillsSection() {
  const prefersReduced = useReducedMotion();

  return (
    <section className="relative w-full bg-canvas py-20 md:py-28" aria-label="Campus disciplines">
      <div className="px-6 lg:px-12 max-w-7xl mx-auto">
        <div className="flex flex-col sm:flex-row sm:items-end justify-between gap-4 mb-8">
          <div>
            <motion.p
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-2"
            >
              Campus Disciplines
            </motion.p>
            <motion.h2
              initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.05, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="font-display text-2xl sm:text-3xl font-normal text-ink"
            >
              Explore work by academic craft.
            </motion.h2>
          </div>
          <Link
            href="/jobs"
            className="font-mono text-xs text-ink hover:underline underline-offset-4 flex items-center gap-1 shrink-0 self-start sm:self-auto"
          >
            <span>View all opportunities</span>
            <ArrowUpRight className="h-3.5 w-3.5" />
          </Link>
        </div>

        {/* Editorial Disciplines Matrix */}
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-px bg-line border border-line">
          {DISCIPLINES.map((discipline, idx) => (
            <motion.div
              key={discipline.name}
              initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{
                duration: 0.4,
                delay: prefersReduced ? 0 : idx * 0.04,
                ease: EDITORIAL_EASE,
              }}
              viewport={{ once: true }}
            >
              <Link
                href={`/jobs?skill=${encodeURIComponent(discipline.query)}`}
                className="group flex flex-col justify-between h-full bg-canvas p-6 sm:p-7 hover:bg-surface-elevated transition-colors duration-200 min-h-[140px]"
              >
                <div className="flex items-start justify-between gap-3">
                  <span className="font-display text-2xl sm:text-[26px] font-normal text-ink group-hover:text-ink/80 transition-colors leading-snug">
                    {discipline.name}
                  </span>
                  <ArrowUpRight className="h-4 w-4 text-ink/30 group-hover:text-ink group-hover:translate-x-0.5 group-hover:-translate-y-0.5 transition-all shrink-0 mt-1" />
                </div>
                <span className="font-mono text-[11px] text-ink-secondary uppercase tracking-widest mt-6">
                  {discipline.note}
                </span>
              </Link>
            </motion.div>
          ))}
        </div>
      </div>
    </section>
  );
}
