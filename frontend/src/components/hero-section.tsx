"use client";

import { useRef } from "react";
import Link from "next/link";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { ArrowRight, Plus } from "lucide-react";
import { EDITORIAL_EASE } from "@/lib/animations";

export default function HeroSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();

  const { scrollYProgress } = useScroll({
    target: ref,
    offset: ["start start", "end start"],
  });

  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "14%"]);
  const textY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "6%"]);

  return (
    <section
      ref={ref}
      className="relative h-screen min-h-[100dvh] w-full overflow-hidden bg-canvas flex items-center pt-20 pb-12 px-6 lg:px-12"
      aria-label="Hero"
    >
      {/* Background landscape - clearly visible on the right */}
      <motion.div
        style={{ y: imgY }}
        className="absolute inset-0 pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/hero.png"
          alt="Campus landscape"
          fill
          priority
          className="object-cover object-center"
          sizes="100vw"
        />

        {/* Left-weighted scrim for high text contrast while keeping right side open */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to right, rgba(244,241,234,0.94) 0%, rgba(244,241,234,0.85) 38%, rgba(244,241,234,0.3) 70%, rgba(244,241,234,0.05) 100%)",
          }}
        />

        {/* Top fade for navigation readability */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(244,241,234,0.8) 0%, rgba(244,241,234,0.15) 16%, rgba(244,241,234,0) 30%)",
          }}
        />

        {/* Bottom fade into canvas */}
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(244,241,234,0) 75%, rgba(244,241,234,0.85) 92%, rgba(244,241,234,1) 100%)",
          }}
        />
      </motion.div>

      {/* Main Hero Content - Positioned Left, Right side completely open */}
      <motion.div
        style={{ y: textY }}
        className="relative z-10 w-full max-w-7xl mx-auto"
      >
        <div className="max-w-2xl text-left flex flex-col items-start">
          {/* Main Headline */}
          <motion.h1
            initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.55, ease: EDITORIAL_EASE }}
            className="font-display text-[clamp(44px,6.4vw,84px)] font-normal leading-[1.05] tracking-[-0.015em] text-ink [text-wrap:balance]"
          >
            A campus for people who&nbsp;build.
          </motion.h1>

          {/* Supporting Copy - strictly no banned words */}
          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.1, ease: EDITORIAL_EASE }}
            className="mt-6 text-lg sm:text-xl text-ink-secondary leading-relaxed max-w-xl [text-wrap:balance]"
          >
            Connect with university peers, discover student initiatives, and collaborate in lightweight team workspaces.
          </motion.p>

          {/* Actions & Centered Scroll Anchor */}
          <div className="mt-10 flex flex-col items-center">
            <motion.div
              initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.45, delay: 0.18, ease: EDITORIAL_EASE }}
              className="flex flex-wrap items-center gap-4"
            >
              <Link
                href="/projects"
                className="inline-flex items-center gap-2 px-7 py-3.5 bg-ink text-canvas text-sm font-medium tracking-wide rounded hover:bg-ink/90 active:scale-[0.98] transition-all duration-150 shadow-sm"
              >
                <span>Explore projects</span>
                <ArrowRight className="w-4 h-4" />
              </Link>
              <Link
                href="/projects/create"
                className="inline-flex items-center gap-2 px-7 py-3.5 border border-line bg-surface-elevated/80 backdrop-blur-sm text-ink text-sm font-medium tracking-wide rounded hover:border-ink active:scale-[0.98] transition-all duration-150"
              >
                <Plus className="w-4 h-4" />
                <span>Start a project</span>
              </Link>
            </motion.div>

            {/* Scroll anchor - Centered directly below the 2 buttons */}
            <motion.a
              href="#how-it-works"
              initial={prefersReduced ? {} : { opacity: 0 }}
              animate={{ opacity: 1 }}
              transition={{ duration: 0.5, delay: 0.26, ease: EDITORIAL_EASE }}
              className="mt-10 sm:mt-12 inline-flex items-center justify-center gap-1.5 text-xs font-mono uppercase tracking-[0.14em] text-ink-secondary/70 hover:text-ink transition-colors text-center"
            >
              <span>How Lynk works</span>
              <span aria-hidden="true">&darr;</span>
            </motion.a>
          </div>
        </div>
      </motion.div>
    </section>
  );
}
