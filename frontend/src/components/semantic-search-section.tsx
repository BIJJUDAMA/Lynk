"use client";

import { useRef, useState, useEffect } from "react";
import Image from "next/image";
import { motion, useScroll, useTransform, useReducedMotion } from "framer-motion";
import { Search, ArrowRight } from "lucide-react";
import Link from "next/link";
import { EDITORIAL_EASE } from "@/lib/animations";

interface DemoScenario {
  query: string;
  category: string;
  matchTitle: string;
  matchDepartment: string;
  matchRole: string;
  matchedTags: string[];
}

const DEMO_SCENARIOS: DemoScenario[] = [
  {
    query: "React engineer for fintech workspace",
    category: "Web & Distributed Systems",
    matchTitle: "Algorithmic Campus Market Engine",
    matchDepartment: "Computer Science & Quantitative Finance",
    matchRole: "Frontend Lead",
    matchedTags: ["React", "TypeScript", "Next.js", "WebSockets"],
  },
  {
    query: "Autonomous drone navigation with ROS2",
    category: "Robotics & Hardware",
    matchTitle: "Autonomous Campus Mapping Drone",
    matchDepartment: "Aerospace & Perception Lab",
    matchRole: "Systems Engineer",
    matchedTags: ["ROS2", "PX4", "C++", "SLAM"],
  },
  {
    query: "PyTorch researcher for LLM benchmark",
    category: "AI & Machine Learning",
    matchTitle: "Campus Reasoning Benchmark Lab",
    matchDepartment: "Language & Intelligence Group",
    matchRole: "ML Researcher",
    matchedTags: ["PyTorch", "Transformers", "Python", "Eval"],
  },
];

export default function SemanticSearchSection() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();
  const { scrollYProgress } = useScroll({ target: ref, offset: ["start end", "end start"] });
  const imgY = useTransform(scrollYProgress, [0, 1], ["-2%", prefersReduced ? "0%" : "2%"]);

  // Animated looping simulation state
  const [scenarioIndex, setScenarioIndex] = useState(0);
  const [displayText, setDisplayText] = useState("");
  const [isTyping, setIsTyping] = useState(true);
  const [showResults, setShowResults] = useState(false);

  const currentScenario = DEMO_SCENARIOS[scenarioIndex];

  useEffect(() => {
    let timeoutId: NodeJS.Timeout;
    const targetQuery = currentScenario.query;

    if (isTyping) {
      if (displayText.length < targetQuery.length) {
        // Natural human typing cadence: pauses on spaces, slight rhythm variance
        const nextChar = targetQuery[displayText.length];
        const delay = nextChar === " " ? 95 : 40 + ((displayText.length * 9) % 25);
        timeoutId = setTimeout(() => {
          setDisplayText(targetQuery.slice(0, displayText.length + 1));
        }, delay);
      } else {
        timeoutId = setTimeout(() => {
          setIsTyping(false);
          setShowResults(true);
        }, 450);
      }
    } else {
      timeoutId = setTimeout(() => {
        setShowResults(false);
        setTimeout(() => {
          setDisplayText("");
          setScenarioIndex((prev) => (prev + 1) % DEMO_SCENARIOS.length);
          setIsTyping(true);
        }, 300);
      }, 4000);
    }

    return () => clearTimeout(timeoutId);
  }, [displayText, isTyping, currentScenario]);

  return (
    <section
      ref={ref}
      className="relative h-screen min-h-[100dvh] max-h-[100dvh] w-full bg-surface overflow-hidden flex items-center justify-center border-t border-line/60"
      aria-labelledby="semantic-heading"
    >
      {/* Ambient background library image - oversized to guarantee zero white gap on parallax */}
      <motion.div
        style={{ y: imgY }}
        className="absolute -top-20 -bottom-20 right-0 w-full md:w-1/2 h-[calc(100%+10rem)] pointer-events-none"
        aria-hidden="true"
      >
        <Image
          src="/ai.png"
          alt="Collegiate research library"
          fill
          className="object-cover object-left"
          sizes="(max-width: 768px) 100vw, 50vw"
        />
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to right, rgba(234,230,221,1) 0%, rgba(234,230,221,0.7) 45%, rgba(234,230,221,0.15) 100%)",
          }}
        />
        <div
          className="absolute inset-0"
          style={{
            background:
              "linear-gradient(to bottom, rgba(234,230,221,0.6) 0%, transparent 25%, transparent 75%, rgba(234,230,221,0.6) 100%)",
          }}
        />
      </motion.div>

      <div className="relative z-10 max-w-7xl mx-auto px-6 lg:px-12 w-full grid grid-cols-1 lg:grid-cols-12 gap-10 lg:gap-16 items-center">
        {/* Left Column: Narrative Copy */}
        <div className="lg:col-span-6 flex flex-col items-start">
          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-4"
          >
            Search & Discovery
          </motion.p>

          <motion.h2
            id="semantic-heading"
            initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="font-display text-[clamp(32px,4.2vw,58px)] font-normal leading-[1.08] text-ink [text-wrap:balance]"
          >
            Find teammates by what they actually build.
          </motion.h2>

          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.16, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="mt-5 text-base sm:text-lg text-ink-secondary leading-relaxed max-w-xl [text-wrap:balance]"
          >
            Describe the skills, research topics, or stack you need. Search connects your natural language query with active student projects and team rosters across campus.
          </motion.p>

          <motion.div
            initial={prefersReduced ? {} : { opacity: 0 }}
            whileInView={{ opacity: 1 }}
            transition={{ duration: 0.4, delay: 0.24, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="mt-7 flex flex-wrap gap-4 items-center"
          >
            <Link
              href="/projects"
              className="inline-flex items-center gap-2 px-6 py-3 bg-ink text-canvas text-sm font-medium tracking-wide rounded hover:bg-ink/90 active:scale-[0.98] transition-all shadow-sm"
            >
              <span>Search active projects</span>
              <ArrowRight className="w-4 h-4" />
            </Link>
          </motion.div>
        </div>

        {/* Right Column: High-contrast, authentic search bar simulation */}
        <div className="lg:col-span-6 w-full max-w-lg mx-auto lg:max-w-none">
          <div className="rounded-2xl border border-line bg-canvas/95 backdrop-blur-md p-6 sm:p-7 shadow-2xl transition-all">
            {/* Prominent Real Search Bar */}
            <div className="flex items-center gap-3 rounded-xl border-2 border-line bg-white px-4 py-3.5 shadow-sm hover:border-ink/40 transition-all mb-5">
              <Search className="h-5 w-5 text-ink/75 shrink-0" />
              <div className="flex-1 min-w-0 font-sans text-sm sm:text-base text-ink font-normal flex items-center">
                <span className="whitespace-nowrap">{displayText}</span>
                <span
                  className="inline-block w-0.5 h-4 sm:h-5 bg-ink ml-1 animate-pulse shrink-0"
                  aria-hidden="true"
                />
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <span className="hidden sm:inline-flex items-center px-2 py-0.5 rounded text-[11px] font-mono text-ink-secondary bg-surface border border-line">
                  Enter &#x21B5;
                </span>
                <div className="w-7 h-7 rounded-md bg-ink text-canvas flex items-center justify-center">
                  <ArrowRight className="w-3.5 h-3.5" />
                </div>
              </div>
            </div>

            {/* Matched Project Result Card */}
            <div
              id="match-result-card"
              data-visible={showResults ? "true" : "false"}
              className={`rounded-xl border border-line bg-white p-5 sm:p-6 transition-all duration-300 transform shadow-md ${
                showResults
                  ? "opacity-100 translate-y-0 scale-100"
                  : "opacity-0 translate-y-2 scale-[0.98] pointer-events-none"
              }`}
            >
              <div className="flex items-start justify-between gap-3 mb-2">
                <div>
                  <span className="inline-block px-2.5 py-0.5 rounded text-[10px] font-mono uppercase tracking-wider font-semibold bg-accent-deep/10 text-accent-deep mb-1">
                    {currentScenario.category}
                  </span>
                  <h4 className="font-display text-xl sm:text-2xl font-normal text-ink leading-snug">
                    {currentScenario.matchTitle}
                  </h4>
                </div>
              </div>

              <p className="text-xs sm:text-sm text-ink-secondary font-mono mb-4">
                {currentScenario.matchDepartment} &bull; {currentScenario.matchRole}
              </p>

              {/* Matched Skill Pills in Pure Site Palette */}
              <div className="flex flex-wrap gap-1.5 pt-3 border-t border-line/60">
                {currentScenario.matchedTags.map((tag) => (
                  <span
                    key={tag}
                    className="inline-flex items-center px-2.5 py-1 rounded-md text-xs font-mono font-medium bg-canvas border border-line text-ink"
                  >
                    #{tag}
                  </span>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
