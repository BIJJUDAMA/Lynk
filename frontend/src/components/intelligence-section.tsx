"use client";

import { useRef } from "react";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { EDITORIAL_EASE } from "@/lib/animations";

const EXAMPLE_MATCHES = [
  "Frontend Engineer -- CS",
  "UI Research -- HCI Lab",
  "Accessibility Specialist -- Design",
];

export default function IntelligenceSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "12%"]);

  return (
    <section
      ref={ref}
      className="relative w-full bg-surface overflow-hidden"
      aria-labelledby="intelligence-heading"
    >
      {/* AI library image bleeds from right */}
      <motion.div
        style={{ y: imgY }}
        className="absolute top-0 right-0 w-full md:w-1/2 h-full pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/ai.png"
          alt="An impossible quiet library representing intelligent search"
          fill
          className="object-cover object-left"
          sizes="(max-width: 768px) 100vw, 50vw"
        />
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to right, rgba(234,230,221,1) 0%, rgba(234,230,221,0.4) 40%, rgba(234,230,221,0) 80%)",
          }}
        />
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(234,230,221,0.6) 0%, transparent 30%, rgba(234,230,221,0.6) 100%)",
          }}
        />
      </motion.div>

      <div className="relative z-10 max-w-7xl mx-auto px-6 lg:px-12 py-20 md:py-28">
        <div className="max-w-2xl">
          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-4"
          >
            Intelligence
          </motion.p>
          <motion.h2
            id="intelligence-heading"
            initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="font-display text-[clamp(28px,4vw,56px)] font-normal leading-[1.1] text-ink [text-wrap:balance]"
          >
            The platform understands what you are looking&nbsp;for.
          </motion.h2>
          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.16, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="mt-6 text-base text-ink-secondary leading-relaxed max-w-xl [text-wrap:balance]"
          >
            Semantic search, skill extraction, and recommendations built into the interface. Not
            announced. Just present.
          </motion.p>

          {/* Quiet search UI demo */}
          <motion.div
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, delay: 0.24, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="mt-10 border border-line bg-surface-elevated p-5 max-w-sm"
          >
            <p className="font-mono text-[11px] text-ink-secondary uppercase tracking-widest mb-3">
              Search
            </p>
            <p className="font-display text-base text-ink italic">
              &quot;React developer for an accessibility project&quot;
            </p>
            <div className="mt-4 space-y-2">
              {EXAMPLE_MATCHES.map((match) => (
                <div key={match} className="flex items-center gap-3">
                  <div className="h-1.5 w-1.5 rounded-full bg-accent-muted shrink-0" />
                  <span className="text-sm text-ink-secondary">{match}</span>
                </div>
              ))}
            </div>
          </motion.div>
        </div>
      </div>
    </section>
  );
}
