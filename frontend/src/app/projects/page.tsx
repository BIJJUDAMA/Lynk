"use client";

import { useState, useEffect, Suspense, useCallback, useRef } from "react";
import { useSearchParams } from "next/navigation";
import Link from "next/link";
import { FolderGit2, AlertCircle, RotateCcw, PlusCircle, Search, X, Sparkles } from "lucide-react";
import type { Project, RecruitmentStatus } from "@/types/project";
import { listProjects, searchProjectsSemantic } from "@/lib/project-api";
import { useQuery } from "@/lib/useApi";
import { useAuth } from "@/components/auth/AuthProvider";
import { ProjectCard } from "@/components/project/ProjectCard";
import { animateStaggerList } from "@/lib/animations";
import { Badge } from "@/components/ui/badge";
import { Input } from "@/components/ui/input";
import { ProjectCardSkeleton } from "@/components/ui/skeleton";
import { ErrorBoundary } from "@/components/ErrorBoundary";
import { createDebounced } from "@/lib/job-filters";
import { PROJECT_CATEGORIES } from "@/lib/formatters";
import { cn } from "@/lib/utils";

const RECRUITMENT_OPTIONS: Array<{
  id: string;
  label: string;
  value: RecruitmentStatus | "";
}> = [
  { id: "all", label: "All Statuses", value: "" },
  { id: "open", label: "Actively Recruiting", value: "open" },
  { id: "paused", label: "Paused", value: "paused" },
  { id: "closed", label: "Closed", value: "closed" },
];

function ProjectsSearchContent() {
  const searchParams = useSearchParams();
  const { isAuthenticated } = useAuth();
  const gridRef = useRef<HTMLDivElement | null>(null);

  const initialSearch = searchParams.get("search") || "";
  const initialCategory = searchParams.get("category") || "";
  const initialRecruitment = (searchParams.get("recruitment") as RecruitmentStatus | null) || "";
  const initialSemantic = searchParams.get("semantic") === "true";

  const [searchDraft, setSearchDraft] = useState(initialSearch);
  const [searchQuery, setSearchQuery] = useState(initialSearch);
  const [selectedCategory, setSelectedCategory] = useState(initialCategory);
  const [selectedRecruitment, setSelectedRecruitment] = useState<RecruitmentStatus | "">(
    initialRecruitment
  );
  const [isSemanticSearch, setIsSemanticSearch] = useState(initialSemantic);

  const searchDebounced = useRef<
    ReturnType<typeof createDebounced<(val: string) => void>> | undefined
  >(undefined);

  useEffect(() => {
    const run = createDebounced((value: string) => {
      setSearchQuery(value);
    }, 350);
    searchDebounced.current = run;
    return () => run.cancel();
  }, []);

  // Fetch projects from Go REST API with active filters & semantic mode
  const {
    data: projects,
    isLoading,
    error,
    refetch,
  } = useQuery<Project[]>(
    useCallback(
      async (client) => {
        const trimmedQuery = searchQuery.trim();

        if (isSemanticSearch && trimmedQuery.length > 0) {
          let results = await searchProjectsSemantic(trimmedQuery, 30, client);
          if (selectedCategory) {
            results = results.filter(
              (p) => p.category.toLowerCase() === selectedCategory.toLowerCase()
            );
          }
          if (selectedRecruitment) {
            results = results.filter((p) => p.recruitment_status === selectedRecruitment);
          }
          return results;
        }

        return listProjects(
          {
            search: trimmedQuery || undefined,
            category: selectedCategory || undefined,
            recruitment_status: (selectedRecruitment as RecruitmentStatus) || undefined,
          },
          client
        );
      },
      [searchQuery, selectedCategory, selectedRecruitment, isSemanticSearch]
    ),
    [searchQuery, selectedCategory, selectedRecruitment, isSemanticSearch]
  );

  useEffect(() => {
    if (gridRef.current && projects && projects.length > 0) {
      return animateStaggerList(gridRef.current);
    }
  }, [projects]);

  const handleResetFilters = () => {
    searchDebounced.current?.cancel();
    setSearchDraft("");
    setSearchQuery("");
    setSelectedCategory("");
    setSelectedRecruitment("");
    setIsSemanticSearch(false);
  };

  const projectList = projects ?? [];
  const hasActiveFilters = Boolean(
    searchQuery || selectedCategory || selectedRecruitment || isSemanticSearch
  );

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:px-6 lg:px-8">
      {/* Editorial Header */}
      <div className="flex flex-col justify-between gap-4 md:flex-row md:items-end">
        <div>
          <Badge variant="outline" className="font-mono text-xs mb-3">
            Campus Collaborative Workspaces
          </Badge>
          <h1 className="font-serif text-4xl sm:text-5xl font-normal leading-[1.15] text-foreground">
            Campus Projects
          </h1>
          <p className="mt-2 max-w-2xl text-sm sm:text-base text-muted-foreground">
            Explore research labs, student startups, and open recruitment across campus.
          </p>
        </div>

        {/* Action Button: + Start a Project */}
        <Link
          href="/projects/create"
          className={cn(
            "inline-flex items-center gap-2 self-start rounded-lg px-4 py-2.5 text-xs font-mono font-medium transition",
            isAuthenticated
              ? "bg-foreground text-background hover:bg-foreground/90 shadow-sm"
              : "border border-border bg-card text-foreground hover:bg-muted"
          )}
        >
          <PlusCircle className="h-4 w-4" />
          Start a Project
        </Link>
      </div>

      {/* Filter & Search Bar */}
      <div className="mt-6 space-y-4">
        <div className="rounded-xl border border-border bg-card p-4 sm:p-5 shadow-none space-y-4">
          {/* Search Field & Semantic Toggle */}
          <div className="flex flex-col sm:flex-row items-stretch sm:items-center gap-3">
            <div className="relative flex-1">
              <Search className="pointer-events-none absolute left-3.5 top-1/2 -translate-y-1/2 h-4 w-4 text-muted-foreground" />
              <Input
                type="text"
                value={searchDraft}
                onChange={(e) => {
                  const val = e.target.value;
                  setSearchDraft(val);
                  searchDebounced.current?.(val);
                }}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    searchDebounced.current?.cancel();
                    setSearchQuery(searchDraft);
                  }
                }}
                placeholder={
                  isSemanticSearch
                    ? "Describe your idea or tech stack (e.g. autonomous rovers, PyTorch, ROS2)..."
                    : "Search projects by title, keywords, or focus area..."
                }
                className="pl-11 pr-9 font-mono text-xs"
              />
              {searchDraft && (
                <button
                  type="button"
                  onClick={() => {
                    searchDebounced.current?.cancel();
                    setSearchDraft("");
                    setSearchQuery("");
                  }}
                  aria-label="Clear search"
                  className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground transition-colors"
                >
                  <X className="h-4 w-4" />
                </button>
              )}
            </div>

            {/* Semantic Search Mode Switch */}
            <button
              type="button"
              onClick={() => setIsSemanticSearch((prev) => !prev)}
              className={cn(
                "inline-flex items-center justify-center gap-2 rounded-lg border px-3.5 py-2 text-xs font-mono transition-colors shrink-0",
                isSemanticSearch
                  ? "border-emerald-500/40 bg-emerald-500/10 text-emerald-600 font-medium"
                  : "border-border bg-background text-muted-foreground hover:text-foreground hover:border-foreground/30"
              )}
            >
              <Sparkles className="h-3.5 w-3.5" />
              <span>Semantic AI Search</span>
              <span
                className={cn(
                  "h-1.5 w-1.5 rounded-full",
                  isSemanticSearch ? "bg-emerald-500 animate-pulse" : "bg-muted-foreground/40"
                )}
              />
            </button>

            {/* Reset Button (when active filters) */}
            {hasActiveFilters && (
              <button
                type="button"
                onClick={handleResetFilters}
                title="Reset all filters"
                className="inline-flex items-center justify-center gap-1.5 rounded-lg border border-border bg-background px-3 py-2 text-xs font-mono text-muted-foreground hover:text-foreground hover:border-foreground/30 transition-colors shrink-0"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                <span>Reset</span>
              </button>
            )}
          </div>

          {/* Category Filter Pills */}
          <div className="flex flex-wrap items-center gap-1.5 border-t border-border/60 pt-3">
            <span className="text-xs font-mono text-muted-foreground mr-1">Track:</span>
            {PROJECT_CATEGORIES.map((cat) => {
              const isSelected = selectedCategory === cat.value;
              return (
                <button
                  key={cat.id}
                  type="button"
                  onClick={() => setSelectedCategory(cat.value)}
                  className={cn(
                    "rounded-md border px-2.5 py-1 text-xs font-mono transition-colors",
                    isSelected
                      ? "border-foreground bg-foreground text-background font-medium"
                      : "border-border bg-background text-muted-foreground hover:border-foreground/40 hover:text-foreground"
                  )}
                >
                  {cat.label}
                </button>
              );
            })}
          </div>

          {/* Recruitment Status Filter Pills */}
          <div className="flex flex-wrap items-center gap-1.5 border-t border-border/60 pt-3">
            <span className="text-xs font-mono text-muted-foreground mr-1">Recruitment:</span>
            {RECRUITMENT_OPTIONS.map((opt) => {
              const isSelected = selectedRecruitment === opt.value;
              return (
                <button
                  key={opt.id}
                  type="button"
                  onClick={() => setSelectedRecruitment(opt.value)}
                  className={cn(
                    "rounded-md border px-2.5 py-1 text-xs font-mono transition-colors",
                    isSelected
                      ? "border-foreground bg-foreground text-background font-medium"
                      : "border-border bg-background text-muted-foreground hover:border-foreground/40 hover:text-foreground"
                  )}
                >
                  {opt.label}
                </button>
              );
            })}
          </div>

          {/* Active status & result count bar */}
          <div className="flex flex-wrap items-center justify-between gap-2 border-t border-border/60 pt-3 text-xs font-mono text-muted-foreground">
            <div>
              {isLoading ? (
                <span className="inline-flex items-center gap-1.5">
                  <span className="h-1.5 w-1.5 animate-pulse rounded-full bg-foreground" />
                  Searching campus workspaces...
                </span>
              ) : (
                <span>
                  Showing{" "}
                  <strong className="text-foreground font-semibold">{projectList.length}</strong>{" "}
                  {projectList.length === 1 ? "workspace" : "workspaces"}
                  {isSemanticSearch && searchQuery.trim() && " (ranked by conceptual similarity)"}
                </span>
              )}
            </div>

            {hasActiveFilters && (
              <div className="flex flex-wrap items-center gap-1.5">
                {searchQuery && (
                  <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-foreground">
                    Search: &quot;{searchQuery}&quot;
                    <button
                      type="button"
                      onClick={() => {
                        searchDebounced.current?.cancel();
                        setSearchDraft("");
                        setSearchQuery("");
                      }}
                      className="hover:text-muted-foreground"
                      aria-label="Remove search filter"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </span>
                )}

                {selectedCategory && (
                  <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-foreground">
                    Track: {selectedCategory}
                    <button
                      type="button"
                      onClick={() => setSelectedCategory("")}
                      className="hover:text-muted-foreground"
                      aria-label="Remove category filter"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </span>
                )}

                {selectedRecruitment && (
                  <span className="inline-flex items-center gap-1 rounded-md border border-border bg-muted/40 px-2 py-0.5 text-[11px] font-mono text-foreground">
                    Status: {selectedRecruitment === "open" ? "Recruiting" : selectedRecruitment}
                    <button
                      type="button"
                      onClick={() => setSelectedRecruitment("")}
                      className="hover:text-muted-foreground"
                      aria-label="Remove recruitment filter"
                    >
                      <X className="h-3 w-3" />
                    </button>
                  </span>
                )}
              </div>
            )}
          </div>
        </div>
      </div>

      {/* Main Content Area */}
      <div className="mt-8">
        {/* Loading Skeletons */}
        {isLoading && (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mt-8">
            {Array.from({ length: 6 }).map((_, i) => (
              <ProjectCardSkeleton key={i} />
            ))}
          </div>
        )}

        {/* Error State */}
        {!isLoading && error && (
          <div className="rounded-xl border border-destructive/20 bg-card p-8 text-center mt-8">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-pastel-red text-pastel-redText">
              <AlertCircle className="h-6 w-6" />
            </div>
            <h2 className="mt-4 font-serif text-xl font-medium text-foreground">
              Unable to load campus projects
            </h2>
            <p className="mt-2 text-sm text-muted-foreground">
              {error.message ||
                "A network or server error occurred while retrieving campus workspaces."}
            </p>
            <div className="mt-6 flex justify-center gap-3">
              <button
                type="button"
                onClick={() => refetch()}
                className="inline-flex items-center gap-2 rounded-lg bg-foreground px-4 py-2 text-xs font-mono font-medium text-background transition hover:bg-foreground/90"
              >
                <RotateCcw className="h-3.5 w-3.5" />
                Retry Search
              </button>
            </div>
          </div>
        )}

        {/* Empty State */}
        {!isLoading && !error && projectList.length === 0 && (
          <div className="rounded-xl border border-dashed border-border bg-card p-12 text-center mt-8">
            <div className="mx-auto flex h-12 w-12 items-center justify-center rounded-lg bg-muted text-muted-foreground">
              <FolderGit2 className="h-6 w-6" />
            </div>
            <h2 className="mt-4 font-serif text-xl font-medium text-foreground">
              No campus projects found
            </h2>
            <p className="mx-auto mt-2 max-w-md text-sm text-muted-foreground">
              {hasActiveFilters
                ? "No campus workspaces match your active filters. Try broadening your search or clear filters."
                : "Be the first to launch a collaborative campus workspace or research lab."}
            </p>
            <div className="mt-6 flex flex-wrap items-center justify-center gap-3">
              {hasActiveFilters && (
                <button
                  type="button"
                  onClick={handleResetFilters}
                  className="inline-flex items-center gap-2 rounded-lg border border-border bg-background px-4 py-2 text-xs font-mono font-medium text-foreground transition hover:border-foreground/40"
                >
                  <RotateCcw className="h-3.5 w-3.5" />
                  Clear Filters
                </button>
              )}
              <Link
                href="/projects/create"
                className="inline-flex items-center gap-2 rounded-lg bg-foreground px-4 py-2 text-xs font-mono font-medium text-background transition hover:bg-foreground/90"
              >
                <PlusCircle className="h-4 w-4" />
                Create the first project in this category
              </Link>
            </div>
          </div>
        )}

        {/* Project Cards Grid */}
        {!isLoading && !error && projectList.length > 0 && (
          <div ref={gridRef} className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6 mt-8">
            {projectList.map((project) => (
              <ProjectCard key={project.id} project={project} />
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

export default function ProjectsPage() {
  return (
    <ErrorBoundary>
      <Suspense
        fallback={
          <div className="mx-auto max-w-7xl px-4 py-12 text-center font-mono text-xs text-muted-foreground">
            Loading campus workspaces...
          </div>
        }
      >
        <ProjectsSearchContent />
      </Suspense>
    </ErrorBoundary>
  );
}
