"use client";

import { useRef } from "react";
import Link from "next/link";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { EDITORIAL_EASE } from "@/lib/animations";

export default function HeroSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();

  const { scrollYProgress } = useScroll({
    target: ref,
    offset: ["start start", "end start"],
  });

  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "18%"]);
  const textY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "8%"]);

  return (
    <section
      ref={ref}
      className="relative min-h-screen w-full overflow-hidden bg-canvas flex items-end"
      aria-label="Hero"
    >
      {/* Parallax background image */}
      <motion.div
        style={{ y: imgY }}
        className="absolute inset-0 pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/hero.png"
          alt="Surreal campus landscape"
          fill
          priority
          className="object-cover object-center"
          sizes="100vw"
        />
        {/* Bottom gradient fade into canvas */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(244,241,234,0.1) 0%, rgba(244,241,234,0.0) 30%, rgba(244,241,234,0.55) 70%, rgba(244,241,234,1) 100%)",
          }}
        />
        {/* Top fade for navbar readability */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(244,241,234,0.5) 0%, rgba(244,241,234,0) 15%)",
          }}
        />
      </motion.div>

      {/* Text content pinned to bottom */}
      <motion.div
        style={{ y: textY }}
        className="relative z-10 w-full pb-20 md:pb-28 lg:pb-32 px-6 lg:px-12 max-w-7xl mx-auto"
      >
        {/* Eyebrow */}
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-6"
        >
          Verified Campus Work Platform
        </motion.p>

        {/* Main heading */}
        <motion.h1
          initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.55, delay: 0.1, ease: EDITORIAL_EASE }}
          className="font-display text-[clamp(34px,5.8vw,82px)] font-normal leading-[1.1] text-ink max-w-6xl [text-wrap:balance]"
        >
          A campus for people who&nbsp;build.
        </motion.h1>

        {/* Supporting copy */}
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, delay: 0.2, ease: EDITORIAL_EASE }}
          className="mt-6 text-base sm:text-lg text-ink-secondary max-w-2xl leading-relaxed [text-wrap:balance]"
        >
          Find opportunities, discover people, and turn ideas into something&nbsp;real.
        </motion.p>

        {/* CTAs */}
        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, delay: 0.28, ease: EDITORIAL_EASE }}
          className="mt-10 flex flex-wrap gap-4"
        >
          <Link
            href="/projects"
            className="inline-block px-6 py-3 bg-ink text-canvas text-sm tracking-wide hover:bg-ink/90 active:scale-[0.98] transition-all duration-150"
          >
            Explore projects
          </Link>
          <a
            href="#how-it-works"
            className="inline-block px-6 py-3 border border-ink text-ink text-sm tracking-wide hover:bg-ink hover:text-canvas active:scale-[0.98] transition-all duration-150"
          >
            How Lynk works
          </a>
        </motion.div>
      </motion.div>
    </section>
  );
}
