"use client";

import { useRef } from "react";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";

const SKILLS = [
  { name: "Design", note: "Visual, Product, Motion" },
  { name: "Engineering", note: "Systems, Backend, Infra" },
  { name: "React", note: "Frontend, Components, Apps" },
  { name: "Python", note: "Data, Scripting, ML" },
  { name: "AI / ML", note: "Models, Pipelines, Research" },
  { name: "Research", note: "Academic, Lab, Analysis" },
  { name: "Product", note: "Strategy, Roadmap, Growth" },
  { name: "Writing", note: "Technical, Editorial, Copy" },
];

export default function SkillsSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();

  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const x = useTransform(
    scrollYProgress,
    [0, 1],
    [prefersReduced ? "0%" : "8%", prefersReduced ? "0%" : "-35%"]
  );

  return (
    <section
      ref={ref}
      className="relative w-full overflow-hidden bg-canvas py-24 md:py-36"
      aria-label="Campus disciplines"
    >
      <div className="px-6 lg:px-12 max-w-7xl mx-auto mb-12">
        <motion.p
          initial={prefersReduced ? {} : { opacity: 0, y: 10 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.7, ease: "easeOut" }}
          viewport={{ once: true }}
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary"
        >
          Campus disciplines
        </motion.p>
      </div>

      {/* Horizontal scroll track driven by vertical scroll */}
      <div className="overflow-hidden">
        <motion.ul
          style={{ x }}
          className="flex gap-6 pl-6 lg:pl-12 pr-24"
          role="list"
        >
          {SKILLS.map((skill) => (
            <li
              key={skill.name}
              className="shrink-0 w-52 sm:w-64 border border-line bg-surface-elevated p-6 flex flex-col justify-between"
              style={{ minHeight: "160px" }}
            >
              <span className="font-display text-2xl sm:text-3xl font-normal text-ink leading-tight">
                {skill.name}
              </span>
              <span className="font-mono text-[11px] text-ink-secondary uppercase tracking-widest mt-4">
                {skill.note}
              </span>
            </li>
          ))}
        </motion.ul>
      </div>
    </section>
  );
}
