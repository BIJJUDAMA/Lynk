"use client";

import { useRef } from "react";
import Link from "next/link";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { EDITORIAL_EASE } from "@/lib/animations";

export default function FindWorkSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "12%"]);

  return (
    <section
      ref={ref}
      className="relative w-full overflow-hidden bg-canvas"
      aria-labelledby="find-work-heading"
    >
      {/* Full-bleed image top half */}
      <motion.div
        style={{ y: imgY }}
        className="relative h-[55vh] md:h-[65vh] w-full"
        aria-hidden="true"
      >
        <Image
          src="/find_the_work.png"
          alt="Vast architectural hall representing opportunity"
          fill
          className="object-cover object-center"
          sizes="100vw"
        />
        {/* Bottom fade into canvas */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(244,241,234,0) 40%, rgba(244,241,234,0.8) 80%, rgba(244,241,234,1) 100%)",
          }}
        />
      </motion.div>

      {/* Editorial text block overlapping image bottom */}
      <div className="relative z-10 -mt-24 md:-mt-32 px-6 lg:px-12 pb-24 md:pb-32 max-w-7xl mx-auto">
        <div className="max-w-3xl">
          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
            viewport={{ once: true, margin: "-40px" }}
            className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-5"
          >
            Find work
          </motion.p>

          <motion.h2
            id="find-work-heading"
            initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
            viewport={{ once: true, margin: "0px" }}
            className="font-display text-[clamp(36px,5vw,72px)] font-normal leading-[1.1] text-ink [text-wrap:balance]"
          >
            Find work worth&nbsp;doing.
          </motion.h2>

          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.16, ease: EDITORIAL_EASE }}
            viewport={{ once: true, margin: "0px" }}
            className="mt-6 text-base sm:text-lg text-ink-secondary leading-relaxed max-w-xl [text-wrap:balance]"
          >
            Discover projects and opportunities across your campus that match what you can&nbsp;do.
          </motion.p>

          <motion.div
            initial={prefersReduced ? {} : { opacity: 0 }}
            whileInView={{ opacity: 1 }}
            transition={{ duration: 0.4, delay: 0.24, ease: EDITORIAL_EASE }}
            viewport={{ once: true, margin: "-40px" }}
            className="mt-8"
          >
            <Link
              href="/jobs"
              className="inline-block text-sm text-ink border-b border-ink pb-0.5 hover:text-ink-secondary hover:border-ink-secondary transition-colors tracking-wide"
            >
              Browse open opportunities
            </Link>
          </motion.div>
        </div>
      </div>
    </section>
  );
}
