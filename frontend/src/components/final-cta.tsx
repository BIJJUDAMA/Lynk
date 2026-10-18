"use client";

import { useRef } from "react";
import Image from "next/image";
import Link from "next/link";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { EDITORIAL_EASE } from "@/lib/animations";

export default function FinalCta() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "15%"]);

  return (
    <section
      ref={ref}
      className="relative w-full overflow-hidden bg-canvas min-h-[85vh] flex items-center"
      aria-labelledby="cta-heading"
    >
      {/* Final landscape full bleed */}
      <motion.div
        style={{ y: imgY }}
        className="absolute inset-0 pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/final.png"
          alt="A tiny structure beneath an enormous horizon"
          fill
          className="object-cover object-center"
          sizes="100vw"
        />
        {/* Heavy top and bottom fade */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(244,241,234,0.92) 0%, rgba(244,241,234,0.4) 25%, rgba(244,241,234,0.3) 60%, rgba(244,241,234,0.85) 100%)",
          }}
        />
      </motion.div>

      <div className="relative z-10 max-w-7xl mx-auto px-6 lg:px-12 py-24 md:py-32 text-center w-full">
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-6"
        >
          Join Lynk
        </motion.p>
        <motion.h2
          id="cta-heading"
          initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="font-display text-[clamp(44px,6.5vw,88px)] font-normal text-ink leading-[1.1] max-w-5xl mx-auto [text-wrap:balance]"
        >
          Build something&nbsp;real.
        </motion.h2>
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.16, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="mt-6 text-base sm:text-lg text-ink-secondary max-w-xl mx-auto leading-relaxed [text-wrap:balance]"
        >
          Your campus is full of ideas, skills, and people ready to&nbsp;build.
        </motion.p>
        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, delay: 0.24, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="mt-12 flex flex-wrap items-center justify-center gap-4"
        >
          <Link
            href="/login"
            className="inline-block px-8 py-3.5 bg-ink text-canvas text-xs uppercase tracking-widest hover:bg-ink/90 active:scale-[0.98] transition-all duration-150"
          >
            Join with university email
          </Link>
          <Link
            href="/projects"
            className="inline-block px-8 py-3.5 border border-ink text-ink text-xs uppercase tracking-widest hover:bg-ink hover:text-canvas active:scale-[0.98] transition-all duration-150"
          >
            Browse all projects
          </Link>
        </motion.div>
      </div>
    </section>
  );
}
