"use client";

import { useRef, useState, useEffect, useCallback } from "react";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { EDITORIAL_EASE } from "@/lib/animations";

const STAGES = [
  {
    num: "01",
    label: "Initiate",
    desc: "Turn an idea into a project. Define your vision, outline the scope, and share what you want to build.",
  },
  {
    num: "02",
    label: "Find Your Team",
    desc: "Bring the right people together. Discover collaborators, review applications, and build a team around your vision.",
  },
  {
    num: "03",
    label: "Build Together",
    desc: "Give your team a space to grow. Manage members, connect repositories, share updates, and link live deployments.",
  },
  {
    num: "04",
    label: "Make It Yours",
    desc: "Every project is different. Customize your workspace, choose your tools, and collaborate on your own terms.",
  },
  {
    num: "05",
    label: "Put Your Work Out There",
    desc: "Show what you have built. Showcase projects, contributions, and real work to the campus community.",
  },
];

export default function WorkflowSection() {
  const ref = useRef<HTMLElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const nodeRefs = useRef<(HTMLDivElement | null)[]>([]);

  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["0%", prefersReduced ? "0%" : "6%"]);

  const [coords, setCoords] = useState<{
    p1: { x: number; y: number };
    p2: { x: number; y: number };
    p3: { x: number; y: number };
    p4: { x: number; y: number };
    p5: { x: number; y: number };
    width: number;
    height: number;
  } | null>(null);

  const updateCoords = useCallback(() => {
    if (!containerRef.current) return;
    const cont = containerRef.current.getBoundingClientRect();
    const n = nodeRefs.current;
    if (n[0] && n[1] && n[2] && n[3] && n[4]) {
      const getCenter = (el: HTMLDivElement) => {
        const r = el.getBoundingClientRect();
        return {
          x: r.left + r.width / 2 - cont.left,
          y: r.top + r.height / 2 - cont.top,
        };
      };
      setCoords({
        p1: getCenter(n[0]),
        p2: getCenter(n[1]),
        p3: getCenter(n[2]),
        p4: getCenter(n[3]),
        p5: getCenter(n[4]),
        width: cont.width,
        height: cont.height,
      });
    }
  }, []);

  useEffect(() => {
    updateCoords();
    const ro = new ResizeObserver(updateCoords);
    if (containerRef.current) {
      ro.observe(containerRef.current);
    }
    window.addEventListener("resize", updateCoords);
    return () => {
      ro.disconnect();
      window.removeEventListener("resize", updateCoords);
    };
  }, [updateCoords]);

  // Compute SVG paths when coords are available
  const radius = 16;
  const gap = 7;
  const offset = radius + gap;
  const cornerR = 20;

  let path1to2 = "";
  let arrow1to2 = "";
  let path2to3 = "";
  let arrow2to3 = "";
  let path3to4 = "";
  let arrow3to4 = "";
  let path4to5 = "";
  let arrow4to5 = "";

  if (coords && coords.width > 100) {
    const { p1, p2, p3, p4, p5, width } = coords;

    // 1 -> 2 (left to right)
    path1to2 = `M ${p1.x + offset} ${p1.y} L ${p2.x - offset} ${p2.y}`;
    arrow1to2 = `M ${p2.x - offset - 6} ${p2.y - 4} L ${p2.x - offset} ${p2.y} L ${
      p2.x - offset - 6
    } ${p2.y + 4}`;

    // 2 -> 3 (left to right)
    path2to3 = `M ${p2.x + offset} ${p2.y} L ${p3.x - offset} ${p3.y}`;
    arrow2to3 = `M ${p3.x - offset - 6} ${p3.y - 4} L ${p3.x - offset} ${p3.y} L ${
      p3.x - offset - 6
    } ${p3.y + 4}`;

    // 3 -> 4 (turnaround curve in right margin, completely clear of Step 03's text, entering Step 04 from the right)
    const turnX = width - 10;
    path3to4 = `M ${p3.x + offset} ${p3.y} H ${turnX - cornerR} Q ${turnX} ${p3.y} ${turnX} ${
      p3.y + cornerR
    } V ${p4.y - cornerR} Q ${turnX} ${p4.y} ${turnX - cornerR} ${p4.y} H ${p4.x + offset}`;
    arrow3to4 = `M ${p4.x + offset + 6} ${p4.y - 4} L ${p4.x + offset} ${p4.y} L ${
      p4.x + offset + 6
    } ${p4.y + 4}`;

    // 4 -> 5 (right to left)
    path4to5 = `M ${p4.x - offset} ${p4.y} L ${p5.x + offset} ${p5.y}`;
    arrow4to5 = `M ${p5.x + offset + 6} ${p5.y - 4} L ${p5.x + offset} ${p5.y} L ${
      p5.x + offset + 6
    } ${p5.y + 4}`;
  }

  return (
    <section
      id="how-it-works"
      ref={ref}
      className="relative h-screen min-h-[100dvh] max-h-[100dvh] w-full overflow-hidden bg-surface flex flex-col justify-start pt-13 sm:pt-15 md:pt-16 pb-4"
      aria-labelledby="workflow-heading"
    >
      {/* Background workflow image across bottom: natural height with background cream color overlay */}
      <motion.div
        style={{ y: imgY }}
        className="absolute bottom-0 left-0 right-0 h-[44vh] sm:h-[46vh] md:h-[48vh] pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/workflow.png"
          alt="Minimalist landscape representing the project collaboration journey"
          fill
          className="object-cover object-bottom opacity-100"
          sizes="100vw"
          priority
        />
        {/* Exact background cream color overlay on the image */}
        <div
          className="absolute inset-0 pointer-events-none"
          style={{
            background:
              "linear-gradient(to bottom, rgba(234,230,221,1) 0%, rgba(234,230,221,0.7) 20%, rgba(234,230,221,0.25) 52%, rgba(234,230,221,0) 84%)",
          }}
        />
      </motion.div>

      {/* Main content situated in upper portion, slightly overlapping top of image */}
      <div className="relative z-10 max-w-6xl mx-auto px-6 lg:px-12 w-full">
        {/* Section Header */}
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.5, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-1.5"
        >
          How Lynk Works
        </motion.p>
        <motion.h2
          id="workflow-heading"
          initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.6, delay: 0.1, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="font-display text-[clamp(28px,3.8vw,54px)] font-normal leading-[1.08] text-ink mb-5 sm:mb-6 [text-wrap:balance]"
        >
          From ideas to impact.
        </motion.h2>

        {/* Desktop Connected Flow - Serpentine left-to-right (1,2,3) then right-to-left (4,5) */}
        <div ref={containerRef} className="relative hidden md:block">
          {/* SVG Arrow Lines Canvas */}
          {coords && coords.width > 100 && (
            <svg
              className="absolute inset-0 w-full h-full pointer-events-none overflow-visible text-ink/50"
              aria-hidden="true"
            >
              {/* Line 1 -> 2 */}
              <motion.path
                d={path1to2}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                initial={prefersReduced ? {} : { pathLength: 0 }}
                whileInView={{ pathLength: 1 }}
                transition={{ duration: 0.45, ease: EDITORIAL_EASE, delay: 0.1 }}
                viewport={{ once: true }}
              />
              <motion.path
                d={arrow1to2}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
                initial={prefersReduced ? {} : { opacity: 0 }}
                whileInView={{ opacity: 1 }}
                transition={{ duration: 0.2, delay: 0.5 }}
                viewport={{ once: true }}
              />

              {/* Line 2 -> 3 */}
              <motion.path
                d={path2to3}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                initial={prefersReduced ? {} : { pathLength: 0 }}
                whileInView={{ pathLength: 1 }}
                transition={{ duration: 0.45, ease: EDITORIAL_EASE, delay: 0.55 }}
                viewport={{ once: true }}
              />
              <motion.path
                d={arrow2to3}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
                initial={prefersReduced ? {} : { opacity: 0 }}
                whileInView={{ opacity: 1 }}
                transition={{ duration: 0.2, delay: 0.95 }}
                viewport={{ once: true }}
              />

              {/* Line 3 -> 4 (Serpentine turnaround curve) */}
              <motion.path
                d={path3to4}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
                initial={prefersReduced ? {} : { pathLength: 0 }}
                whileInView={{ pathLength: 1 }}
                transition={{ duration: 0.7, ease: EDITORIAL_EASE, delay: 1.0 }}
                viewport={{ once: true }}
              />
              <motion.path
                d={arrow3to4}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
                initial={prefersReduced ? {} : { opacity: 0 }}
                whileInView={{ opacity: 1 }}
                transition={{ duration: 0.2, delay: 1.65 }}
                viewport={{ once: true }}
              />

              {/* Line 4 -> 5 */}
              <motion.path
                d={path4to5}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                initial={prefersReduced ? {} : { pathLength: 0 }}
                whileInView={{ pathLength: 1 }}
                transition={{ duration: 0.45, ease: EDITORIAL_EASE, delay: 1.7 }}
                viewport={{ once: true }}
              />
              <motion.path
                d={arrow4to5}
                fill="none"
                stroke="currentColor"
                strokeWidth="1.8"
                strokeLinecap="round"
                strokeLinejoin="round"
                initial={prefersReduced ? {} : { opacity: 0 }}
                whileInView={{ opacity: 1 }}
                transition={{ duration: 0.2, delay: 2.1 }}
                viewport={{ once: true }}
              />
            </svg>
          )}

          {/* Row 1: 01, 02, 03 (left to right, comfortable size, clear right margin) */}
          <div className="grid grid-cols-3 gap-6 lg:gap-12 pr-12 lg:pr-16">
            {/* Step 01 */}
            <motion.div
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="flex flex-col items-start"
            >
              <div
                ref={(el) => {
                  nodeRefs.current[0] = el;
                }}
                className="w-8 h-8 rounded-full border border-ink/40 bg-surface flex items-center justify-center font-mono text-xs font-semibold text-ink shadow-sm"
              >
                01
              </div>
              <h3 className="font-display text-lg sm:text-xl font-normal text-ink mt-2.5 mb-1">
                Initiate
              </h3>
              <p className="text-xs sm:text-[13px] md:text-sm text-ink-secondary leading-relaxed max-w-[270px]">
                Turn an idea into a project. Define your vision, outline the scope, and share what
                you want to build.
              </p>
            </motion.div>

            {/* Step 02 */}
            <motion.div
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.25, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="flex flex-col items-start"
            >
              <div
                ref={(el) => {
                  nodeRefs.current[1] = el;
                }}
                className="w-8 h-8 rounded-full border border-ink/40 bg-surface flex items-center justify-center font-mono text-xs font-semibold text-ink shadow-sm"
              >
                02
              </div>
              <h3 className="font-display text-lg sm:text-xl font-normal text-ink mt-2.5 mb-1">
                Find Your Team
              </h3>
              <p className="text-xs sm:text-[13px] md:text-sm text-ink-secondary leading-relaxed max-w-[270px]">
                Bring the right people together. Discover collaborators, review applications, and
                build a team around your vision.
              </p>
            </motion.div>

            {/* Step 03 */}
            <motion.div
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.55, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="flex flex-col items-start"
            >
              <div
                ref={(el) => {
                  nodeRefs.current[2] = el;
                }}
                className="w-8 h-8 rounded-full border border-ink/40 bg-surface flex items-center justify-center font-mono text-xs font-semibold text-ink shadow-sm"
              >
                03
              </div>
              <h3 className="font-display text-lg sm:text-xl font-normal text-ink mt-2.5 mb-1">
                Build Together
              </h3>
              <p className="text-xs sm:text-[13px] md:text-sm text-ink-secondary leading-relaxed max-w-[270px]">
                Give your team a space to grow. Manage members, connect repositories, share updates,
                and link live deployments.
              </p>
            </motion.div>
          </div>

          {/* Row 2: 05, 04 (centered, right-to-left flow, pushed into cream overlay) */}
          <div className="mt-10 sm:mt-12 md:mt-14 flex justify-center gap-12 lg:gap-24 max-w-2xl mx-auto w-full">
            {/* Step 05 (left side of center pair) */}
            <motion.div
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 1.8, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="flex flex-col items-start max-w-[280px]"
            >
              <div
                ref={(el) => {
                  nodeRefs.current[4] = el;
                }}
                className="w-8 h-8 rounded-full border border-ink/40 bg-surface flex items-center justify-center font-mono text-xs font-semibold text-ink shadow-sm"
              >
                05
              </div>
              <h3 className="font-display text-lg sm:text-xl font-normal text-ink mt-2.5 mb-1">
                Put Your Work Out There
              </h3>
              <p className="text-xs sm:text-[13px] md:text-sm text-ink-secondary leading-relaxed">
                Show what you have built. Showcase projects, contributions, and real work to the
                campus community.
              </p>
            </motion.div>

            {/* Step 04 (right side of center pair) */}
            <motion.div
              initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 1.25, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="flex flex-col items-start max-w-[280px]"
            >
              <div
                ref={(el) => {
                  nodeRefs.current[3] = el;
                }}
                className="w-8 h-8 rounded-full border border-ink/40 bg-surface flex items-center justify-center font-mono text-xs font-semibold text-ink shadow-sm"
              >
                04
              </div>
              <h3 className="font-display text-lg sm:text-xl font-normal text-ink mt-2.5 mb-1">
                Make It Yours
              </h3>
              <p className="text-xs sm:text-[13px] md:text-sm text-ink-secondary leading-relaxed">
                Every project is different. Customize your workspace, choose your tools, and
                collaborate on your own terms.
              </p>
            </motion.div>
          </div>
        </div>

        {/* Mobile Responsive Stack (< md) */}
        <ol className="md:hidden space-y-3.5 pt-2">
          {STAGES.map((stage, i) => (
            <motion.li
              key={stage.label}
              initial={prefersReduced ? {} : { opacity: 0, y: 4 }}
              whileInView={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.3, delay: i * 0.05, ease: EDITORIAL_EASE }}
              viewport={{ once: true }}
              className="flex gap-3.5 items-start"
            >
              <div className="w-7 h-7 rounded-full border border-ink/40 bg-surface flex items-center justify-center font-mono text-xs font-semibold text-ink shrink-0 mt-0.5 shadow-sm">
                {stage.num}
              </div>
              <div>
                <h3 className="font-display text-lg font-normal text-ink">{stage.label}</h3>
                <p className="mt-0.5 text-xs sm:text-sm text-ink-secondary leading-snug">
                  {stage.desc}
                </p>
              </div>
            </motion.li>
          ))}
        </ol>
      </div>
    </section>
  );
}
