"use client";

import { useRef, useState, useEffect } from "react";
import Link from "next/link";
import { motion, AnimatePresence, useReducedMotion } from "framer-motion";
import { ArrowRight } from "lucide-react";
import { EDITORIAL_EASE } from "@/lib/animations";

interface RosterRow {
  member: string;
  department: string;
  role: string;
  isNew?: boolean;
}

interface ResourceRow {
  resource: string;
  target: string;
  status: string;
  isNew?: boolean;
}

interface MilestoneRow {
  timeline: string;
  deliverable: string;
  status: string;
  isNew?: boolean;
  isPulsing?: boolean;
}

const ROSTER_DATA: RosterRow[] = [
  { member: "Nitansh Shankar", department: "Computer Science", role: "Project Lead" },
  { member: "Frontend Developer", department: "Web Engineering", role: "Next.js / React" },
  { member: "+ AI Engineer", department: "AI / ML", role: "RAG & LLM Systems", isNew: true },
];

const RESOURCE_DATA: ResourceRow[] = [
  { resource: "Source Repository", target: "GitHub", status: "Connected" },
  { resource: "Backend API", target: "Go / REST API", status: "In development" },
  { resource: "+ AI Service", target: "Python / FastAPI", status: "RAG integration", isNew: true },
  { resource: "Database", target: "PostgreSQL", status: "Configured" },
  { resource: "Object Storage", target: "MinIO", status: "Connected" },
];

const MILESTONE_DATA: MilestoneRow[] = [
  { timeline: "Phase 01", deliverable: "Workspace Design", status: "Established" },
  { timeline: "Phase 02", deliverable: "Core Architecture", status: "In development" },
  { timeline: "+ Next", deliverable: "AI-Powered Features", status: "Planned", isNew: true, isPulsing: true },
];

const TABS = [
  { id: 0, label: "Team Roster" },
  { id: 1, label: "Connected Resources" },
  { id: 2, label: "Project Milestones" },
];

export default function ProductShowcase() {
  const ref = useRef<HTMLElement>(null);
  const prefersReduced = useReducedMotion();

  // 12-second seamless looping cycle (4 seconds per phase)
  const [activeTab, setActiveTab] = useState(0);
  const [revealNewItem, setRevealNewItem] = useState(false);

  useEffect(() => {
    // Reveal the animated item after 1000ms in each phase
    setRevealNewItem(false);
    const itemTimer = setTimeout(() => {
      setRevealNewItem(true);
    }, 1100);

    // Switch to next tab every 4000ms
    const tabTimer = setTimeout(() => {
      setActiveTab((prev) => (prev + 1) % 3);
    }, 4000);

    return () => {
      clearTimeout(itemTimer);
      clearTimeout(tabTimer);
    };
  }, [activeTab]);

  return (
    <section
      ref={ref}
      className="relative h-screen min-h-[100dvh] max-h-[100dvh] w-full bg-canvas overflow-hidden border-t border-line/60 flex flex-col justify-center py-6 sm:py-8 md:py-10"
      aria-labelledby="product-heading"
    >
      <div className="max-w-7xl mx-auto px-6 lg:px-12 w-full">
        {/* Section Header - Centered to ground the page */}
        <div className="max-w-3xl mx-auto text-center mb-6 sm:mb-8">
          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.45, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-secondary mb-3"
          >
            The Platform
          </motion.p>
          <motion.h2
            id="product-heading"
            initial={prefersReduced ? {} : { opacity: 0, y: 8 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.08, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="font-display text-[clamp(28px,3.8vw,56px)] font-normal leading-[1.08] text-ink mb-3.5 [text-wrap:balance]"
          >
            The product is the experience.
          </motion.h2>
          <motion.p
            initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
            whileInView={{ opacity: 1, y: 0 }}
            transition={{ duration: 0.5, delay: 0.16, ease: EDITORIAL_EASE }}
            viewport={{ once: true }}
            className="text-sm sm:text-base text-ink-secondary leading-relaxed max-w-2xl mx-auto [text-wrap:balance]"
          >
            Project discovery, student identities, modular team workspaces, and transparent campus portfolios. Built for real academic collaboration.
          </motion.p>
        </div>

        {/* Responsive Workspace Window Console (100% Pure HTML/CSS/JS, Zero Pills) */}
        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 12 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.55, delay: 0.2, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="w-full max-w-4xl mx-auto rounded-2xl border border-line bg-surface-elevated shadow-xl overflow-hidden"
        >
          {/* Window Chrome Header - Option B: Clean Git & Sync Metadata (Zero AI Fluff) */}
          <div className="flex items-center justify-between px-5 sm:px-6 py-2.5 border-b border-line bg-surface/80">
            <div className="flex items-center gap-2 font-mono text-xs text-ink">
              <span className="font-medium text-ink">market-engine-core</span>
              <span className="text-line-strong text-ink-secondary/50" aria-hidden="true">&bull;</span>
              <span className="text-ink-secondary">main</span>
              <span className="text-line-strong text-ink-secondary/50 hidden sm:inline" aria-hidden="true">&bull;</span>
              <span className="text-ink-secondary text-[11px] hidden sm:inline">synced 14m ago</span>
            </div>

            <div className="flex items-center gap-2 font-mono text-xs text-ink-secondary">
              <span>3 Contributors</span>
              <span className="text-line-strong text-ink-secondary/50" aria-hidden="true">&bull;</span>
              <span className="text-ink text-[11px]">Public Workspace</span>
            </div>
          </div>

          {/* Clean Tab Triggers (No Pills - Pure Minimal Lines) */}
          <div className="flex items-center px-5 sm:px-6 border-b border-line bg-surface-elevated overflow-x-auto scrollbar-none">
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id;
              return (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  className={`relative py-3 px-4 font-mono text-xs uppercase tracking-wider transition-colors whitespace-nowrap ${
                    isActive ? "text-ink font-medium" : "text-ink-secondary hover:text-ink"
                  }`}
                >
                  <span>{tab.label}</span>
                  {isActive && (
                    <motion.div
                      layoutId="workspaceActiveTab"
                      className="absolute bottom-0 left-0 right-0 h-0.5 bg-ink"
                      transition={{ duration: 0.25 }}
                    />
                  )}
                </button>
              );
            })}
          </div>

          {/* Active Phase Display */}
          <div className="p-5 sm:p-7 min-h-[260px] flex flex-col justify-start bg-surface-elevated">
            <AnimatePresence mode="wait">
              {/* PHASE 1: TEAM ROSTER */}
              {activeTab === 0 && (
                <motion.div
                  key="tab-roster"
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.3 }}
                  className="w-full"
                >
                  <div className="grid grid-cols-12 pb-2 border-b border-line font-mono text-[10px] sm:text-[11px] uppercase tracking-wider text-ink-secondary">
                    <div className="col-span-5 sm:col-span-4">Member</div>
                    <div className="col-span-4 sm:col-span-4">Department</div>
                    <div className="col-span-3 sm:col-span-4 text-right sm:text-left">Role</div>
                  </div>

                  <div className="divide-y divide-line/40">
                    {ROSTER_DATA.map((row) => {
                      if (row.isNew && !revealNewItem) return null;
                      return (
                        <motion.div
                          key={row.member}
                          initial={row.isNew ? { opacity: 0, x: -8 } : false}
                          animate={{ opacity: 1, x: 0 }}
                          transition={{ duration: 0.35, ease: EDITORIAL_EASE }}
                          className={`grid grid-cols-12 py-3 items-center text-sm ${
                            row.isNew ? "bg-accent-deep/[0.04]" : ""
                          }`}
                        >
                          <div className="col-span-5 sm:col-span-4 font-medium text-ink flex items-center gap-1.5">
                            {row.isNew && (
                              <span className="text-accent-deep font-mono text-xs shrink-0" aria-hidden="true">
                                &bull;
                              </span>
                            )}
                            <span className="truncate">{row.member}</span>
                          </div>
                          <div className="col-span-4 sm:col-span-4 font-mono text-xs sm:text-sm text-ink-secondary truncate">
                            {row.department}
                          </div>
                          <div className="col-span-3 sm:col-span-4 font-mono text-xs sm:text-sm text-ink text-right sm:text-left truncate">
                            {row.role}
                          </div>
                        </motion.div>
                      );
                    })}
                  </div>
                </motion.div>
              )}

              {/* PHASE 2: CONNECTED RESOURCES */}
              {activeTab === 1 && (
                <motion.div
                  key="tab-resources"
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.3 }}
                  className="w-full"
                >
                  <div className="grid grid-cols-12 pb-2 border-b border-line font-mono text-[10px] sm:text-[11px] uppercase tracking-wider text-ink-secondary">
                    <div className="col-span-5 sm:col-span-4">Resource</div>
                    <div className="col-span-4 sm:col-span-4">Target</div>
                    <div className="col-span-3 sm:col-span-4 text-right sm:text-left">Sync Status</div>
                  </div>

                  <div className="divide-y divide-line/40">
                    {RESOURCE_DATA.map((row) => {
                      if (row.isNew && !revealNewItem) return null;
                      return (
                        <motion.div
                          key={row.resource}
                          initial={row.isNew ? { opacity: 0, x: -8 } : false}
                          animate={{ opacity: 1, x: 0 }}
                          transition={{ duration: 0.35, ease: EDITORIAL_EASE }}
                          className={`grid grid-cols-12 py-2.5 items-center text-sm ${
                            row.isNew ? "bg-accent-deep/[0.04]" : ""
                          }`}
                        >
                          <div className="col-span-5 sm:col-span-4 font-medium text-ink flex items-center gap-1.5">
                            {row.isNew && (
                              <span className="text-accent-deep font-mono text-xs shrink-0" aria-hidden="true">
                                &bull;
                              </span>
                            )}
                            <span className="truncate">{row.resource}</span>
                          </div>
                          <div className="col-span-4 sm:col-span-4 font-mono text-xs sm:text-sm text-ink-secondary truncate">
                            {row.target}
                          </div>
                          <div className="col-span-3 sm:col-span-4 font-mono text-xs sm:text-sm text-ink text-right sm:text-left truncate">
                            {row.status}
                          </div>
                        </motion.div>
                      );
                    })}
                  </div>
                </motion.div>
              )}

              {/* PHASE 3: PROJECT MILESTONES */}
              {activeTab === 2 && (
                <motion.div
                  key="tab-milestones"
                  initial={{ opacity: 0, y: 6 }}
                  animate={{ opacity: 1, y: 0 }}
                  exit={{ opacity: 0, y: -6 }}
                  transition={{ duration: 0.3 }}
                  className="w-full"
                >
                  <div className="grid grid-cols-12 pb-2 border-b border-line font-mono text-[10px] sm:text-[11px] uppercase tracking-wider text-ink-secondary">
                    <div className="col-span-4 sm:col-span-3">Timeline</div>
                    <div className="col-span-5 sm:col-span-5">Deliverable</div>
                    <div className="col-span-3 sm:col-span-4 text-right sm:text-left">Status</div>
                  </div>

                  <div className="divide-y divide-line/40">
                    {MILESTONE_DATA.map((row) => {
                      if (row.isNew && !revealNewItem) return null;
                      return (
                        <motion.div
                          key={row.timeline}
                          initial={row.isNew ? { opacity: 0, x: -8 } : false}
                          animate={{ opacity: 1, x: 0 }}
                          transition={{ duration: 0.35, ease: EDITORIAL_EASE }}
                          className={`grid grid-cols-12 py-3 items-center text-sm ${
                            row.isNew ? "bg-accent-deep/[0.04]" : ""
                          }`}
                        >
                          <div className="col-span-4 sm:col-span-3 font-medium text-ink flex items-center gap-1.5">
                            {row.isPulsing && (
                              <span className="w-1.5 h-1.5 rounded-full bg-accent-deep animate-ping shrink-0" aria-hidden="true" />
                            )}
                            <span className="truncate">{row.timeline}</span>
                          </div>
                          <div className="col-span-5 sm:col-span-5 font-mono text-xs sm:text-sm text-ink truncate">
                            {row.deliverable}
                          </div>
                          <div className="col-span-3 sm:col-span-4 font-mono text-xs sm:text-sm text-ink-secondary text-right sm:text-left truncate">
                            {row.status}
                          </div>
                        </motion.div>
                      );
                    })}
                  </div>
                </motion.div>
              )}
            </AnimatePresence>
          </div>
        </motion.div>

        {/* Action CTA: Single focused button, centered */}
        <motion.div
          initial={prefersReduced ? {} : { opacity: 0, y: 6 }}
          whileInView={{ opacity: 1, y: 0 }}
          transition={{ duration: 0.45, delay: 0.28, ease: EDITORIAL_EASE }}
          viewport={{ once: true }}
          className="mt-8 flex justify-center"
        >
          <Link
            href="/activity"
            className="inline-flex items-center gap-2 px-7 py-3.5 bg-ink text-canvas text-sm font-medium tracking-wide rounded hover:bg-ink/90 active:scale-[0.98] transition-all shadow-sm"
          >
            <span>Enter the workspace</span>
            <ArrowRight className="w-4 h-4" />
          </Link>
        </motion.div>
      </div>
    </section>
  );
}
