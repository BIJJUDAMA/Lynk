"use client";

import { useEffect, useRef, useState } from "react";
import { createDebounced, syncSearchDraftFromParent } from "@/lib/job-filters";
import { Search, X, RotateCcw, ChevronDown } from "lucide-react";
import { JobStatus } from "@/types/api";
import { cn } from "@/lib/utils";
import { COMMON_DEPARTMENTS, POPULAR_SKILLS } from "@/lib/formatters";

export { COMMON_DEPARTMENTS, POPULAR_SKILLS };

export type BudgetSort = "" | "high" | "low";

export interface JobFilterValues {
  search: string;
  department: string;
  skill: string;
  status: "" | JobStatus;
  budgetSort?: BudgetSort;
}

export interface JobFilterBarProps {
  filters: JobFilterValues;
  onFilterChange: (filters: JobFilterValues) => void;
  onReset: () => void;
  totalCount?: number | null;
  isLoading?: boolean;
}

export function JobFilterBar({
  filters,
  onFilterChange,
  onReset,
  totalCount,
  isLoading,
}: JobFilterBarProps) {
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

  const hasActiveFilters = Boolean(
    filters.search ||
    filters.department ||
    filters.skill ||
    filters.budgetSort ||
    (filters.status && filters.status !== "open")
  );

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
    <div className="rounded-xl border border-border bg-card p-4 sm:p-5 shadow-none">
      {/* Top Row: Search, Department Dropdown, Budget Sort, and Reset */}
      <div className="flex flex-col md:flex-row items-stretch md:items-center gap-3">
        {/* Search Field */}
        <div className="relative flex-1">
          <Search className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
          <input
            type="text"
            value={searchDraft}
            onChange={(e) => {
              const v = e.target.value;
              setSearchDraft(v);
              searchDebounced.current?.(v);
            }}
            placeholder="Search by title, skill, or keyword..."
            className="w-full rounded-lg border border-border bg-background py-2.5 pl-9 pr-9 text-sm text-foreground placeholder:text-muted-foreground focus:border-foreground/40 focus:outline-none transition-colors"
          />
          {searchDraft && (
            <button
              type="button"
              onClick={clearSearch}
              aria-label="Clear search input"
              className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>

        {/* Academic Department Selector */}
        <div className="relative w-full md:w-64">
          <select
            value={filters.department}
            onChange={(e) => updateField("department", e.target.value)}
            className="w-full appearance-none rounded-lg border border-border bg-background py-2.5 pl-3.5 pr-8 text-sm font-mono text-foreground focus:border-foreground/40 focus:outline-none transition-colors"
          >
            <option value="">All Academic Departments</option>
            {COMMON_DEPARTMENTS.map((dept) => (
              <option key={dept} value={dept}>
                {dept}
              </option>
            ))}
          </select>
          <div className="pointer-events-none absolute inset-y-0 right-0 flex items-center pr-3">
            <ChevronDown className="h-4 w-4 text-muted-foreground" />
          </div>
        </div>

        {/* Budget Sort & Reset Controls */}
        <div className="flex items-center gap-2 self-start md:self-auto">
          {/* Budget Sort Controls */}
          <div className="inline-flex rounded-lg border border-border bg-muted/40 p-0.5">
            <button
              type="button"
              onClick={() => updateField("budgetSort", filters.budgetSort === "high" ? "" : "high")}
              className={cn(
                "rounded-md px-2.5 py-1.5 text-xs font-mono transition-colors",
                filters.budgetSort === "high"
                  ? "bg-background text-foreground font-medium shadow-xs"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              High to Low
            </button>
            <button
              type="button"
              onClick={() => updateField("budgetSort", filters.budgetSort === "low" ? "" : "low")}
              className={cn(
                "rounded-md px-2.5 py-1.5 text-xs font-mono transition-colors",
                filters.budgetSort === "low"
                  ? "bg-background text-foreground font-medium shadow-xs"
                  : "text-muted-foreground hover:text-foreground"
              )}
            >
              Low to High
            </button>
          </div>

          {/* Reset Filters Button */}
          {hasActiveFilters && (
            <button
              type="button"
              onClick={handleReset}
              title="Reset all filters"
              className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-background px-3 py-2 text-xs font-mono text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors"
            >
              <RotateCcw className="h-3.5 w-3.5" />
              <span>Reset</span>
            </button>
          )}
        </div>
      </div>

      {/* Quick Skills Pill Strip */}
      <div className="mt-3 flex flex-wrap items-center gap-1.5 border-t border-border/60 pt-3">
        <span className="text-xs font-mono text-muted-foreground">Popular:</span>
        {POPULAR_SKILLS.map((skill) => {
          const isSelected = filters.skill.toLowerCase() === skill.toLowerCase();
          return (
            <button
              key={skill}
              type="button"
              onClick={() => updateField("skill", isSelected ? "" : skill)}
              className={cn(
                "rounded-md border px-2 py-0.5 text-[11px] font-mono transition-colors",
                isSelected
                  ? "border-foreground bg-foreground text-background font-medium"
                  : "border-border bg-background text-muted-foreground hover:border-foreground/30 hover:text-foreground"
              )}
            >
              {skill}
            </button>
          );
        })}
      </div>

      {/* Active Filter Chips & Results Count */}
      <div className="mt-3 flex flex-wrap items-center justify-between gap-2 border-t border-border/60 pt-3 text-xs font-mono text-muted-foreground">
        <div>
          {isLoading ? (
            <span className="inline-flex items-center gap-1.5">
              <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-foreground" />
              Searching campus opportunities...
            </span>
          ) : typeof totalCount === "number" ? (
            <span>
              Showing <strong className="text-foreground font-semibold">{totalCount}</strong>{" "}
              {totalCount === 1 ? "opportunity" : "opportunities"}
            </span>
          ) : (
            <span>Showing verified campus gigs</span>
          )}
        </div>

        {hasActiveFilters && (
          <div className="flex flex-wrap items-center gap-1.5">
            {filters.search && (
              <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-foreground">
                Search: &quot;{filters.search}&quot;
                <button
                  type="button"
                  onClick={clearSearch}
                  aria-label="Remove search filter"
                  className="hover:text-muted-foreground"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            )}

            {filters.department && (
              <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-foreground">
                Dept: {filters.department}
                <button
                  type="button"
                  onClick={() => updateField("department", "")}
                  aria-label="Remove department filter"
                  className="hover:text-muted-foreground"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            )}

            {filters.skill && (
              <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-foreground">
                Skill: {filters.skill}
                <button
                  type="button"
                  onClick={() => updateField("skill", "")}
                  aria-label="Remove skill filter"
                  className="hover:text-muted-foreground"
                >
                  <X className="h-3 w-3" />
                </button>
              </span>
            )}

            {filters.budgetSort && (
              <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-foreground">
                Sort: {filters.budgetSort === "high" ? "High to Low" : "Low to High"}
                <button
                  type="button"
                  onClick={() => updateField("budgetSort", "")}
                  aria-label="Remove budget sort"
                  className="hover:text-muted-foreground"
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
