"use client";

import { useRef } from "react";
import Image from "next/image";
import Link from "next/link";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";

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

      <div className="relative z-10 max-w-7xl mx-auto px-6 lg:px-12 py-32 md:py-48 text-center w-full">
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7 }}
          viewport={{ once: true }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-6"
        >
          Join Lynk
        </motion.p>
        <motion.h2
          id="cta-heading"
          initial={prefersReduced ? {} : { opacity: 0, y: 28 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 1.1, delay: 0.1, ease: "easeOut" }}
          viewport={{ once: true }}
          className="font-display text-[clamp(48px,7vw,96px)] font-normal tracking-[-0.03em] text-ink leading-[1.0] max-w-3xl mx-auto"
        >
          Build something
          <br />
          real.
        </motion.h2>
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 16 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.9, delay: 0.25 }}
          viewport={{ once: true }}
          className="mt-6 text-base sm:text-lg text-ink-secondary max-w-sm mx-auto leading-relaxed"
        >
          Your campus is full of ideas, skills, and people ready to build.
        </motion.p>
        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.4 }}
          viewport={{ once: true }}
          className="mt-12"
        >
          <Link
            href="/login"
            className="inline-block px-10 py-4 bg-ink text-canvas text-sm tracking-widest uppercase hover:bg-ink/90 transition-colors"
          >
            Join Lynk
          </Link>
        </motion.div>
      </div>
    </section>
  );
}
