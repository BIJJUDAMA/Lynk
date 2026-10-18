"use client";

import { useRef } from "react";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { EDITORIAL_EASE } from "@/lib/animations";

const STEPS = [
  {
    num: "01",
    label: "Find people",
    body: "Discover students across every discipline. Engineering, design, research, product.",
  },
  {
    num: "02",
    label: "Discover skills",
    body: "See campus profiles with real project history and peer track records.",
  },
  {
    num: "03",
    label: "Build together",
    body: "Form a team, start an initiative, or connect directly. Campus collaboration, structured.",
  },
];

export default function FindPeopleSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["-4%", prefersReduced ? "0%" : "6%"]);

  return (
    <section
      ref={ref}
      className="relative h-screen min-h-[100dvh] max-h-[100dvh] w-full bg-surface overflow-hidden flex items-center py-12 md:py-16"
      aria-labelledby="find-people-heading"
    >
      <div className="max-w-7xl mx-auto px-6 lg:px-12 w-full grid grid-cols-1 lg:grid-cols-2 gap-12 lg:gap-20 items-center">
        {/* Left: parallax image (alternates with Section 2) */}
        <motion.div
          style={{ y: imgY }}
          className="relative h-[45vh] sm:h-[50vh] md:h-[60vh] rounded-lg overflow-hidden border border-line shadow-sm order-2 lg:order-1"
          aria-hidden="true"
        >
          <Image
            src="/find_the_people_warm.jpg"
            alt="Aerial view of connected campus structures"
            fill
            className="object-cover object-center"
            sizes="(max-width: 1024px) 100vw, 50vw"
          />
          {/* Soft edge vignette */}
          <div
            className="absolute inset-0"
            style={{
              background:
                "radial-gradient(ellipse at center, transparent 55%, rgba(234,230,221,0.5) 100%)",
            }}
          />
        </motion.div>

        {/* Right: narrative text & steps (alternates with Section 2) */}
        <div className="space-y-8 md:space-y-10 order-1 lg:order-2">
          <div>
            <motion.p
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-4"
            >
              Find People
            </motion.p>
            <motion.h2
              id="find-people-heading"
              initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="font-display text-[clamp(30px,3.8vw,56px)] font-normal leading-[1.1] text-ink [text-wrap:balance]"
            >
              The right people are closer than you&nbsp;think.
            </motion.h2>
          </div>

          <div className="space-y-6 md:space-y-7">
            {STEPS.map((step, i) => (
              <motion.div
                key={step.num}
                initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
                whileInView={{ opacity: 1, y: 0 }}
                transition={{ duration: 0.45, delay: i * 0.06, ease: EDITORIAL_EASE }}
                viewport={{ once: true, margin: "0px" }}
                className="flex gap-5 sm:gap-6"
              >
                <span className="font-mono text-[11px] text-ink-secondary pt-0.5 shrink-0 tracking-widest">
                  {step.num}
                </span>
                <div>
                  <p className="font-display text-lg sm:text-xl text-ink leading-snug">{step.label}</p>
                  <p className="mt-1 text-xs sm:text-sm text-ink-secondary leading-relaxed">{step.body}</p>
                </div>
              </motion.div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
