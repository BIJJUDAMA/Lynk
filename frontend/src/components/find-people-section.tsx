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
    body: "See verified profiles with real project history and peer-reviewed track records.",
  },
  {
    num: "03",
    label: "Build together",
    body: "Form a team, publish an opportunity, or connect directly. Campus collaboration, structured.",
  },
];

export default function FindPeopleSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["-5%", prefersReduced ? "0%" : "8%"]);

  return (
    <section
      ref={ref}
      className="relative w-full bg-surface overflow-hidden"
      aria-labelledby="find-people-heading"
    >
      <div className="max-w-7xl mx-auto px-6 lg:px-12 py-20 md:py-28 grid grid-cols-1 lg:grid-cols-2 gap-16 lg:gap-24 items-start">
        {/* Left: sticky narrative text */}
        <div className="lg:sticky lg:top-32 space-y-16">
          <div>
            <motion.p
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-5"
            >
              Find people
            </motion.p>
            <motion.h2
              id="find-people-heading"
              initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="font-display text-[clamp(32px,4vw,60px)] font-normal leading-[1.1] text-ink [text-wrap:balance]"
            >
              The right people are closer than you&nbsp;think.
            </motion.h2>
          </div>

          {STEPS.map((step, i) => (
            <motion.div
              key={step.num}
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.45, delay: i * 0.06, ease: EDITORIAL_EASE }}
              viewport={{ once: true, margin: "0px" }}
              className="flex gap-6"
            >
              <span className="font-mono text-[11px] text-ink-secondary pt-1 shrink-0 tracking-widest">
                {step.num}
              </span>
              <div>
                <p className="font-display text-xl text-ink">{step.label}</p>
                <p className="mt-2 text-sm text-ink-secondary leading-relaxed">{step.body}</p>
              </div>
            </motion.div>
          ))}
        </div>

        {/* Right: parallax image */}
        <motion.div
          style={{ y: imgY }}
          className="relative h-[60vh] md:h-[75vh] overflow-hidden"
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
                "radial-gradient(ellipse at center, transparent 50%, rgba(234,230,221,0.6) 100%)",
            }}
          />
        </motion.div>
      </div>
    </section>
  );
}
