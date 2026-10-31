"use client";

import React from "react";

export interface SkillGapProps {
  readySkills: string[];
  missingSkills: string[];
  readinessPercentage: number;
}

export function SkillGapBadgeList({
  readySkills,
  missingSkills,
  readinessPercentage,
}: SkillGapProps) {
  return (
    <div className="rounded-lg border border-border bg-card p-4 space-y-3">
      <div className="flex items-center justify-between">
        <h4 className="text-sm font-semibold text-foreground">Project Skill Alignment</h4>
        <span className="text-xs font-mono font-medium text-emerald-600 bg-emerald-50 dark:bg-emerald-950/40 px-2 py-0.5 rounded">
          {readinessPercentage}% Ready
        </span>
      </div>

      <div className="space-y-2">
        <div>
          <span className="text-xs text-muted-foreground block mb-1">
            Your Skills Matching Project:
          </span>
          <div className="flex flex-wrap gap-1.5">
            {readySkills.length > 0 ? (
              readySkills.map((sk) => (
                <span
                  key={sk}
                  className="text-xs bg-emerald-100 text-emerald-800 dark:bg-emerald-900/50 dark:text-emerald-200 px-2 py-0.5 rounded font-mono"
                >
                  +{sk}
                </span>
              ))
            ) : (
              <span className="text-xs text-muted-foreground italic">None yet</span>
            )}
          </div>
        </div>

        <div>
          <span className="text-xs text-muted-foreground block mb-1">Skills to Learn:</span>
          <div className="flex flex-wrap gap-1.5">
            {missingSkills.length > 0 ? (
              missingSkills.map((sk) => (
                <span
                  key={sk}
                  className="text-xs bg-amber-100 text-amber-800 dark:bg-amber-900/50 dark:text-amber-200 px-2 py-0.5 rounded font-mono"
                >
                  {sk}
                </span>
              ))
            ) : (
              <span className="text-xs text-emerald-600 font-medium">
                All required skills ready!
              </span>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
