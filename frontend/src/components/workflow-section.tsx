"use client";

import { useRef } from "react";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";

const STAGES = [
  { label: "Idea", desc: "A problem worth solving, a project worth building." },
  { label: "Opportunity", desc: "Post a gig or discover one. Verified campus talent, clear scope." },
  { label: "Application", desc: "Propose your approach. Cover letter, portfolio, resume on file." },
  { label: "Contract", desc: "Accepted. A structured agreement, activated when you are ready." },
  { label: "Delivery", desc: "Build it. Ship it. The contract tracks progress and sign-off." },
  { label: "Review", desc: "Mutual verified reviews. A public record of real campus work." },
];

export default function WorkflowSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "10%"]);

  return (
    <section
      id="how-it-works"
      ref={ref}
      className="relative w-full overflow-hidden bg-surface"
      aria-labelledby="workflow-heading"
    >
      {/* Background workflow image bleeds in from bottom */}
      <motion.div
        style={{ y: imgY }}
        className="absolute bottom-0 left-0 right-0 h-80 md:h-96 pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/workflow.png"
          alt="A journey across a minimalist landscape representing the workflow"
          fill
          className="object-cover object-bottom"
          sizes="100vw"
        />
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(234,230,221,1) 0%, rgba(234,230,221,0.4) 60%, rgba(234,230,221,0) 100%)",
          }}
        />
      </motion.div>

      <div className="relative z-10 max-w-7xl mx-auto px-6 lg:px-12 py-24 md:py-36">
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, ease: "easeOut" }}
          viewport={{ once: true }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-4"
        >
          How it works
        </motion.p>
        <motion.h2
          id="workflow-heading"
          initial={prefersReduced ? {} : { opacity: 0, y: 20 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.9, delay: 0.1, ease: "easeOut" }}
          viewport={{ once: true }}
          className="font-display text-[clamp(32px,4.5vw,64px)] font-normal tracking-[-0.025em] text-ink mb-20"
        >
          From idea to
          <br />
          delivered.
        </motion.h2>

        <div className="relative">
          {/* Animated vertical timeline line */}
          <motion.div
            initial={prefersReduced ? {} : { scaleY: 0 }}
            whileInView={{ scaleY: 1 }}
            transition={{ duration: 1.8, ease: "easeInOut", delay: 0.3 }}
            viewport={{ once: true }}
            style={{ transformOrigin: "top" }}
            className="absolute left-[3px] top-0 bottom-0 w-px bg-line hidden md:block"
            aria-hidden="true"
          />

          <ol className="space-y-12 md:space-y-16">
            {STAGES.map((stage, i) => (
              <motion.li
                key={stage.label}
                initial={prefersReduced ? {} : { opacity: 0, x: -20 }}
                whileInView={{ opacity: 1, x: 0 }}
                transition={{ duration: 0.7, delay: i * 0.08, ease: "easeOut" }}
                viewport={{ once: true, margin: "-40px" }}
                className="flex gap-8 md:gap-12 items-start"
              >
                <div
                  className="shrink-0 hidden md:flex flex-col items-center gap-1 pt-1"
                  aria-hidden="true"
                >
                  <div className="h-2 w-2 rounded-full bg-ink-secondary" />
                </div>
                <div>
                  <span className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-secondary">
                    {String(i + 1).padStart(2, "0")}
                  </span>
                  <p className="font-display text-2xl md:text-3xl font-normal text-ink mt-1">
                    {stage.label}
                  </p>
                  <p className="mt-2 text-sm text-ink-secondary leading-relaxed max-w-sm">
                    {stage.desc}
                  </p>
                </div>
              </motion.li>
            ))}
          </ol>
        </div>
      </div>
    </section>
  );
}
