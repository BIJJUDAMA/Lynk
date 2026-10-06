"use client";

import { useRef } from "react";
import Link from "next/link";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";

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
          initial={prefersReduced ? {} : { opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.2, ease: "easeOut" }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-6"
        >
          A campus for people who build
        </motion.p>

        {/* Main heading */}
        <motion.h1
          initial={prefersReduced ? {} : { opacity: 0, y: 20 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 1.0, delay: 0.35, ease: "easeOut" }}
          className="font-display text-[clamp(42px,7vw,96px)] font-normal leading-[1.0] tracking-[-0.03em] text-ink max-w-4xl"
        >
          A campus for
          <br />
          people who build.
        </motion.h1>

        {/* Supporting copy */}
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 16 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.9, delay: 0.55, ease: "easeOut" }}
          className="mt-6 text-base sm:text-lg text-ink-secondary max-w-md leading-relaxed"
        >
          Find opportunities, discover people, and turn ideas into something real.
        </motion.p>

        {/* CTAs */}
        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 12 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.8, delay: 0.75, ease: "easeOut" }}
          className="mt-10 flex flex-wrap gap-4"
        >
          <Link
            href="/jobs"
            className="inline-block px-6 py-3 bg-ink text-canvas text-sm tracking-wide hover:bg-ink/90 transition-colors"
          >
            Explore opportunities
          </Link>
          <a
            href="#how-it-works"
            className="inline-block px-6 py-3 border border-ink text-ink text-sm tracking-wide hover:bg-ink hover:text-canvas transition-colors"
          >
            How Lynk works
          </a>
        </motion.div>
      </motion.div>
    </section>
  );
}
