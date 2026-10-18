"use client";

import { useRef } from "react";
import Link from "next/link";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { ArrowRight } from "lucide-react";
import { EDITORIAL_EASE } from "@/lib/animations";

export default function FindWorkSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "8%"]);

  return (
    <section
      ref={ref}
      id="find-work"
      className="relative h-screen min-h-[100dvh] max-h-[100dvh] w-full overflow-hidden bg-canvas flex items-center justify-center"
      aria-labelledby="find-work-heading"
    >
      {/* Atmospheric architectural image spanning background */}
      <motion.div
        style={{ y: imgY }}
        className="absolute inset-0 h-full w-full pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/find_the_work.png"
          alt="Vast collegiate architectural hall"
          fill
          className="object-cover object-center"
          sizes="100vw"
          priority
        />
        {/* Background cream gradient overlay to ensure text legibility while artwork pops */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(244,241,234,0.55) 0%, rgba(244,241,234,0.2) 25%, rgba(244,241,234,0.2) 75%, rgba(244,241,234,0.6) 100%)",
          }}
        />
        {/* Luminous soft cream radial wash centered behind text */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "radial-gradient(ellipse 70% 60% at 50% 50%, rgba(244,241,234,0.45) 0%, rgba(244,241,234,0.2) 50%, rgba(244,241,234,0) 80%)",
          }}
        />
      </motion.div>

      {/* Editorial content block - Centered vertically and horizontally */}
      <div className="relative z-10 px-6 lg:px-12 max-w-4xl mx-auto w-full flex flex-col items-center justify-center text-center">
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
          viewport={{ once: true, margin: "-40px" }}
          className="font-mono text-[11px] uppercase tracking-[0.18em] text-ink font-semibold mb-4 text-center drop-shadow-[0_1px_4px_rgba(244,241,234,0.9)]"
        >
          Project Discovery
        </motion.p>

        <motion.h2
          id="find-work-heading"
          initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
          viewport={{ once: true, margin: "0px" }}
          className="font-display text-[clamp(36px,5.2vw,72px)] font-normal leading-[1.06] text-ink [text-wrap:balance] text-center drop-shadow-[0_2px_8px_rgba(244,241,234,0.85)]"
        >
          Projects built to matter.
        </motion.h2>

        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.16, ease: EDITORIAL_EASE }}
          viewport={{ once: true, margin: "0px" }}
          className="mt-6 text-base sm:text-lg md:text-xl text-ink font-medium leading-relaxed [text-wrap:balance] text-center mx-auto max-w-xl drop-shadow-[0_1px_6px_rgba(244,241,234,0.85)]"
        >
          Discover student projects and initiatives across campus looking for collaborators to build together.
        </motion.p>

        <motion.div
          initial={prefersReduced ? {} : { opacity: 0 }}
          whileInView={{ opacity: 1 }}
          transition={{ duration: 0.4, delay: 0.24, ease: EDITORIAL_EASE }}
          viewport={{ once: true, margin: "-40px" }}
          className="mt-8 flex items-center justify-center gap-4"
        >
          <Link
            href="/projects?recruiting=true"
            className="inline-flex items-center gap-2 px-6 py-3 bg-ink text-canvas text-sm font-medium tracking-wide rounded hover:bg-ink/90 active:scale-[0.98] transition-all shadow-sm"
          >
            <span>View open team roles</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </motion.div>
      </div>
    </section>
  );
}
