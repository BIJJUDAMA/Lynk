"use client";

import { useEffect, useRef, useState } from "react";
import { createDebounced, syncSearchDraftFromParent } from "@/lib/job-filters";
import { Search, SlidersHorizontal, X, RotateCcw, ChevronDown, ChevronUp } from "lucide-react";
import { JobStatus } from "@/types/api";
import { cn } from "@/lib/utils";

export interface JobFilterValues {
  search: string;
  department: string;
  skill: string;
  status: "" | JobStatus;
}

export interface JobFilterBarProps {
  filters: JobFilterValues;
  onFilterChange: (filters: JobFilterValues) => void;
  onReset: () => void;
  totalCount?: number | null;
  isLoading?: boolean;
}

export const COMMON_DEPARTMENTS = [
  "Computer Science & Engineering",
  "Data Science & AI",
  "Design & Creative Arts",
  "Business & Marketing",
  "Biology & Life Sciences",
  "Mathematics & Statistics",
  "Writing & Communications",
  "Economics & Finance",
  "Psychology & Social Sciences",
];

export const POPULAR_SKILLS = [
  "React",
  "Python",
  "TypeScript",
  "Figma",
  "Data Analysis",
  "SQL",
  "Content Writing",
  "Machine Learning",
  "UI/UX",
];

export function JobFilterBar({
  filters,
  onFilterChange,
  onReset,
  totalCount,
  isLoading,
}: JobFilterBarProps) {
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [searchDraft, setSearchDraft] = useState(filters.search);
  const onFilterChangeRef = useRef(onFilterChange);
  onFilterChangeRef.current = onFilterChange;
  const filtersRef = useRef(filters);
  filtersRef.current = filters;
  const searchDebounced = useRef<
    ReturnType<typeof createDebounced<(value: string) => void>> | undefined
  >(undefined);

  useEffect(() => {
    const next = syncSearchDraftFromParent(filters.search, searchDraft);
    if (next.cancelPending) {
      searchDebounced.current?.cancel();
    }
    setSearchDraft(next.draft);
    // eslint-disable-next-line react-hooks/exhaustive-deps -- typing against empty parent must not cancel
  }, [filters.search]);

  useEffect(() => {
    const run = createDebounced((value: string) => {
      onFilterChangeRef.current({ ...filtersRef.current, search: value });
    }, 350);
    searchDebounced.current = run;
    return () => run.cancel();
  }, []);

  // Check if any filter is active
  const hasActiveFilters = Boolean(
    filters.search ||
    filters.department ||
    filters.skill ||
    (filters.status && filters.status !== "open")
  );

  const activeCount = [
    Boolean(filters.search),
    Boolean(filters.department),
    Boolean(filters.skill),
    Boolean(filters.status && filters.status !== "open"),
  ].filter(Boolean).length;

  const updateField = <K extends keyof JobFilterValues>(key: K, value: JobFilterValues[K]) => {
    onFilterChange({
      ...filters,
      [key]: value,
    });
  };

  const clearSearch = () => {
    searchDebounced.current?.cancel();
    setSearchDraft("");
    updateField("search", "");
  };

  const handleReset = () => {
    searchDebounced.current?.cancel();
    setSearchDraft("");
    onReset();
  };

  return (
    <div className="rounded-[10px] border border-slate-200 bg-white p-4 shadow-sm dark:border-slate-800 dark:bg-slate-900/90 sm:p-6">
      {/* Search Input & Quick Controls */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        {/* Search Field */}
        <div className="relative flex-1">
          <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
            <Search className="h-4 w-4 text-slate-400" />
          </div>
          <input
            type="text"
            value={searchDraft}
            onChange={(e) => {
              const v = e.target.value;
              setSearchDraft(v);
              searchDebounced.current?.(v);
            }}
            placeholder="Search gigs by title, keywords, or role..."
            className="w-full rounded-[10px] border border-slate-200 bg-slate-50/50 py-2.5 pl-10 pr-10 text-sm text-slate-900 placeholder:text-slate-400 focus:border-emerald-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-emerald-500/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900"
          />
          {searchDraft && (
            <button
              type="button"
              onClick={clearSearch}
              className="absolute inset-y-0 right-0 flex items-center pr-3 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        {/* Department Quick Dropdown */}
        <div className="w-full md:w-64">
          <div className="relative">
            <select
              value={filters.department}
              onChange={(e) => updateField("department", e.target.value)}
              className="w-full appearance-none rounded-[10px] border border-slate-200 bg-slate-50/50 py-2.5 pl-3.5 pr-8 text-sm text-slate-900 focus:border-emerald-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-emerald-500/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white dark:focus:bg-slate-900"
            >
              <option value="">All Academic Departments</option>
              {COMMON_DEPARTMENTS.map((dept) => (
                <option key={dept} value={dept}>
                  {dept}
                </option>
              ))}
            </select>
            <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
              <ChevronDown className="h-4 w-4 text-slate-400" />
            </div>
          </div>
        </div>

        {/* Advanced Filters Toggle & Reset Button */}
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={() => setShowAdvanced(!showAdvanced)}
            className={cn(
              "inline-flex items-center gap-1.5 rounded-[10px] border px-3.5 py-2.5 text-xs font-semibold transition",
              showAdvanced || activeCount > 0
                ? "border-emerald-200 bg-emerald-50 text-emerald-700 dark:border-emerald-900 dark:bg-emerald-950/50 dark:text-emerald-300"
                : "border-slate-200 bg-white text-slate-700 hover:bg-slate-50 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:bg-slate-700"
            )}
          >
            <SlidersHorizontal className="h-3.5 w-3.5" />
            <span>Filters</span>
            {activeCount > 0 && (
              <span className="flex h-5 w-5 items-center justify-center rounded-full bg-emerald-600 text-[10px] font-bold text-white">
                {activeCount}
              </span>
            )}
            {showAdvanced ? (
              <ChevronUp className="h-3.5 w-3.5" />
            ) : (
              <ChevronDown className="h-3.5 w-3.5" />
            )}
          </button>

          {hasActiveFilters && (
            <button
              type="button"
              onClick={handleReset}
              title="Reset all filters"
              className="inline-flex items-center gap-1 rounded-[10px] border border-slate-200 bg-white px-3 py-2.5 text-xs font-medium text-slate-600 transition hover:border-slate-300 hover:text-slate-900 dark:border-slate-700 dark:bg-slate-800 dark:text-slate-300 dark:hover:text-white"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              <span className="hidden sm:inline">Reset</span>
            </button>
          )}
        </div>
      </div>

      {/* Advanced Filter Drawer */}
      {showAdvanced && (
        <div className="mt-4 border-t border-slate-100 pt-4 dark:border-slate-800/80">
          <div className="max-w-md">
            {/* Skill Filter Input */}
            <div>
              <label className="block text-xs font-semibold text-slate-700 dark:text-slate-300">
                Required Skill
              </label>
              <input
                type="text"
                value={filters.skill}
                onChange={(e) => updateField("skill", e.target.value)}
                placeholder="e.g. React, Python, Figma"
                className="mt-1.5 w-full rounded-[10px] border border-slate-200 bg-slate-50/50 px-3 py-2 text-xs text-slate-900 placeholder:text-slate-400 focus:border-emerald-500 focus:bg-white focus:outline-none focus:ring-2 focus:ring-emerald-500/20 dark:border-slate-700 dark:bg-slate-800/60 dark:text-white"
              />
            </div>
          </div>

          {/* Quick Skill Tags Pills */}
          <div className="mt-4 flex flex-wrap items-center gap-1.5">
            <span className="text-xs text-slate-500 dark:text-slate-400">Popular skills:</span>
            {POPULAR_SKILLS.map((skill) => {
              const isSelected = filters.skill.toLowerCase() === skill.toLowerCase();
              return (
                <button
                  key={skill}
                  type="button"
                  onClick={() => updateField("skill", isSelected ? "" : skill)}
                  className={cn(
                    "rounded-[10px] border px-2 py-0.5 text-[11px] font-medium transition",
                    isSelected
                      ? "border-emerald-500 bg-emerald-50 text-emerald-700 dark:border-emerald-500 dark:bg-emerald-950/60 dark:text-emerald-300"
                      : "border-slate-200 bg-slate-50 text-slate-600 hover:border-slate-300 dark:border-slate-800 dark:bg-slate-800/60 dark:text-slate-400"
                  )}
                >
                  {skill}
                </button>
              );
            })}
          </div>
        </div>
      )}

      {/* Active Filter Chips & Results Count */}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-slate-100 pt-3 text-xs text-slate-500 dark:border-slate-800/80 dark:text-slate-400">
        <div>
          {isLoading ? (
            <span className="inline-flex items-center gap-1.5">
              <span className="h-2 w-2 animate-ping rounded-full bg-emerald-500" />
              Searching available gigs...
            </span>
          ) : typeof totalCount === "number" ? (
            <span>
              Showing <strong className="text-slate-900 dark:text-white">{totalCount}</strong>{" "}
              {totalCount === 1 ? "gig" : "gigs"}
            </span>
          ) : (
            <span>Showing verified opportunities</span>
          )}
        </div>

        {hasActiveFilters && (
          <div className="flex flex-wrap items-center gap-1.5">
            {filters.search && (
              <span className="inline-flex items-center gap-1 rounded-[10px] bg-slate-100 px-2 py-0.5 text-[11px] text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                Keyword: &ldquo;{filters.search}&rdquo;
                <button type="button" onClick={clearSearch} className="hover:text-slate-900">
                  <X className="h-3 w-3" />
                </button>
              </span>
            )}

            {filters.department && (
              <span className="inline-flex items-center gap-1 rounded-[10px] bg-slate-100 px-2 py-0.5 text-[11px] text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                Dept: {filters.department}
                <button
                  type="button"
                  onClick={() => updateField("department", "")}
                  className="hover:text-slate-900"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            )}

            {filters.skill && (
              <span className="inline-flex items-center gap-1 rounded-[10px] bg-slate-100 px-2 py-0.5 text-[11px] text-slate-700 dark:bg-slate-800 dark:text-slate-300">
                Skill: {filters.skill}
                <button
                  type="button"
                  onClick={() => updateField("skill", "")}
                  className="hover:text-slate-900"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            )}
          </div>
        )}
      </div>
    </div>
  );
}
